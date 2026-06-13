package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	copilotcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// maxResultChars bounds how much tool output is fed back to the model / returned.
const maxResultChars = 4000

// ToolService aggregates the tenant's MCP tools, runs read tools, and drives the
// human-approval flow for write tools. It also implements the copilot ToolBroker
// so the agentic loop can use the same tools.
type ToolService struct {
	servers       repository.ServerRepository
	approvals     repository.ApprovalRepository
	callLogs      repository.CallLogRepository
	conversations convrepo.ConversationRepository
	client        contracts.Client
	publisher     shared.EventPublisher
	clock         shared.Clock
	auditor       shared.Auditor
	ispBridge     contracts.ISPToolBridge // optional: gates + injects ISP config for SMSNET servers
}

// SetISPBridge wires the SMSNET ISP tool bridge: it gates which SMSNET servers a
// conversation may use and injects the ISP config{type+creds} into tool calls
// server-side. Optional; when unset, SMSNET tools behave like any other server.
func (s *ToolService) SetISPBridge(b contracts.ISPToolBridge) { s.ispBridge = b }

// NewToolService builds the service.
func NewToolService(
	servers repository.ServerRepository,
	approvals repository.ApprovalRepository,
	callLogs repository.CallLogRepository,
	conversations convrepo.ConversationRepository,
	client contracts.Client,
	publisher shared.EventPublisher,
	clock shared.Clock,
) *ToolService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if publisher == nil {
		publisher = shared.NoopPublisher{}
	}
	return &ToolService{
		servers: servers, approvals: approvals, callLogs: callLogs, conversations: conversations,
		client: client, publisher: publisher, clock: clock, auditor: shared.NoopAuditor{},
	}
}

// SetAuditor wires the audit trail. Optional.
func (s *ToolService) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// ── discovery ─────────────────────────────────────────────────────────────────

// Tools lists the read/write tools available for a conversation, aggregated
// across the tenant's enabled MCP servers (dynamically discovered).
func (s *ToolService) Tools(ctx context.Context, conversationID string) ([]entity.Tool, error) {
	if _, err := s.loadVisible(ctx, conversationID); err != nil {
		return nil, err
	}
	return s.aggregate(ctx)
}

// ListApprovals returns a conversation's write-action approvals (read). Requires
// the conversation to be visible to the actor; an empty thread yields an empty
// slice (not an error).
func (s *ToolService) ListApprovals(ctx context.Context, conversationID string) ([]*entity.Approval, error) {
	if _, err := s.loadVisible(ctx, conversationID); err != nil {
		return nil, err
	}
	return s.approvals.ListByConversation(ctx, conversationID)
}

// ListCallLogs returns a conversation's payload-free tool-call logs (read), under
// the same visibility rule as ListApprovals.
func (s *ToolService) ListCallLogs(ctx context.Context, conversationID string) ([]*entity.CallLog, error) {
	if _, err := s.loadVisible(ctx, conversationID); err != nil {
		return nil, err
	}
	return s.callLogs.ListByConversation(ctx, conversationID)
}

// aggregate discovers tools from every enabled server, annotating write by kind.
func (s *ToolService) aggregate(ctx context.Context) ([]entity.Tool, error) {
	servers, err := s.servers.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	var tools []entity.Tool
	for _, conn := range servers {
		specs, err := s.client.ListTools(ctx, conn)
		if err != nil {
			continue // a single unreachable server must not break the aggregate
		}
		tools = append(tools, annotate(conn, specs)...)
	}
	return tools, nil
}

// ── manual run (agent) ──────────────────────────────────────────────────────

// Run executes a read tool directly or, for a write tool, records a pending
// approval (never executing it). Write requires integration.execute_action.
func (s *ToolService) Run(ctx context.Context, cmd contracts.RunTool) (contracts.RunResult, error) {
	conv, err := s.loadVisible(ctx, cmd.ConversationID)
	if err != nil {
		return contracts.RunResult{}, err
	}
	ac, _ := authz.FromContext(ctx)
	if !ac.Has(authz.IntegrationRead) {
		return contracts.RunResult{}, apperror.Forbidden("integration.read required")
	}
	conn, err := s.servers.FindByID(ctx, cmd.ServerID)
	if err != nil {
		return contracts.RunResult{}, err
	}
	if !conn.Enabled {
		return contracts.RunResult{}, apperror.Validation("server is disabled")
	}
	write := conn.Kind == entity.KindWrite

	if write {
		if !ac.Has(authz.IntegrationExecuteAction) {
			return contracts.RunResult{}, apperror.Forbidden("integration.execute_action required for write tools")
		}
		approval := s.newApproval(conv, conn, cmd.Tool, cmd.Args, ac.UserID)
		if err := s.approvals.Create(ctx, approval); err != nil {
			return contracts.RunResult{}, err
		}
		s.publishApproval(ctx, conv.TenantID, approval)
		return contracts.RunResult{Executed: false, Approval: approval, Tool: cmd.Tool, Write: true}, nil
	}

	text, err := s.invoke(ctx, conv, conn, cmd.Tool, cmd.Args, ac.UserID, "")
	if err != nil {
		return contracts.RunResult{}, err
	}
	return contracts.RunResult{Executed: true, Result: text, Tool: cmd.Tool, Write: false}, nil
}

// ── approvals ─────────────────────────────────────────────────────────────────

// Decide approves or rejects a pending write action. Approval executes the tool
// and audits it (actor/ip/params, secrets masked); rejection records the refusal.
func (s *ToolService) Decide(ctx context.Context, conversationID, approvalID string, approve bool, reason string) (contracts.RunResult, error) {
	conv, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return contracts.RunResult{}, err
	}
	ac, _ := authz.FromContext(ctx)
	if !ac.Has(authz.IntegrationExecuteAction) {
		return contracts.RunResult{}, apperror.Forbidden("integration.execute_action required")
	}
	approval, err := s.approvals.FindByID(ctx, approvalID)
	if err != nil {
		return contracts.RunResult{}, err
	}
	if approval.ConversationID != conv.ID {
		return contracts.RunResult{}, apperror.NotFound("approval not found")
	}
	if approval.Status != entity.ApprovalPending {
		return contracts.RunResult{}, apperror.Conflict("approval already decided")
	}
	now := s.clock.Now()
	approval.DecidedBy = ac.UserID
	approval.DecidedAt = &now
	approval.Reason = reason

	if !approve {
		approval.Status = entity.ApprovalRejected
		if err := s.approvals.Update(ctx, approval); err != nil {
			return contracts.RunResult{}, err
		}
		_ = s.auditor.Record(ctx, shared.AuditEntry{
			Action: "integration.execute_action.rejected", ResourceType: "mcp_tool", ResourceID: approval.Tool,
			Data: map[string]any{"server": approval.ServerName, "approval_id": approval.ID},
		})
		return contracts.RunResult{Executed: false, Approval: approval, Tool: approval.Tool, Write: true}, nil
	}

	conn, err := s.servers.FindByID(ctx, approval.ServerID)
	if err != nil {
		return contracts.RunResult{}, err
	}
	// Side-effect via approval: forward the approval id as the idempotency key so
	// the SMSNET write can dedup an accidental re-approval/retry.
	text, ierr := s.invoke(ctx, conv, conn, approval.Tool, approval.Args, ac.UserID, approval.ID)
	if ierr != nil {
		approval.Status = entity.ApprovalFailed
		approval.Error = summarizeErr(ierr)
		_ = s.approvals.Update(ctx, approval)
		// The execution attempt is audited regardless of outcome.
		s.auditExecuted(ctx, approval, "failed")
		return contracts.RunResult{}, ierr
	}
	approval.Status = entity.ApprovalExecuted
	approval.Result = truncate(text, maxResultChars)
	if err := s.approvals.Update(ctx, approval); err != nil {
		return contracts.RunResult{}, err
	}
	s.auditExecuted(ctx, approval, "executed")
	return contracts.RunResult{Executed: true, Result: approval.Result, Approval: approval, Tool: approval.Tool, Write: true}, nil
}

// ── copilot ToolBroker ───────────────────────────────────────────────────────

// OpenToolSession implements copilotcontracts.ToolBroker: it discovers the
// tenant's tools and returns a session bound to the conversation. Read tools are
// callable by the model; write tools are offered but only ever proposed.
func (s *ToolService) OpenToolSession(ctx context.Context, conversationID string) (copilotcontracts.ToolSession, error) {
	conv, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	tools, err := s.aggregate(ctx)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]toolRef, len(tools))
	defs := make([]copilotcontracts.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		ref, err := s.servers.FindByID(ctx, t.ServerID)
		if err != nil {
			continue
		}
		// SMSNET tools are gated by the conversation's assistant ISP profile: hide
		// them when no profile is pinned, and hide the write (OPERACOES) tools when
		// the profile supports neither liberacao nor chamado.
		if s.ispBridge != nil && entity.IsSMSNETServer(ref.Name) {
			allowed, aerr := s.ispBridge.AllowServer(ctx, conv.Channel, ref.Name, t.Write)
			if aerr != nil || !allowed {
				continue
			}
		}
		byName[t.Name] = toolRef{conn: ref, write: t.Write}
		defs = append(defs, copilotcontracts.ToolDefinition{
			Name: t.Name, Description: t.Description, Schema: t.Schema, ReadOnly: !t.Write,
		})
	}
	return &toolSession{svc: s, conv: conv, byName: byName, defs: defs}, nil
}

type toolRef struct {
	conn  *entity.ServerConnection
	write bool
}

// toolSession is one agentic run's tool context.
type toolSession struct {
	svc    *ToolService
	conv   *convEntityRef
	byName map[string]toolRef
	defs   []copilotcontracts.ToolDefinition
}

func (t *toolSession) Tools() []copilotcontracts.ToolDefinition { return t.defs }

func (t *toolSession) IsWrite(name string) bool {
	ref, ok := t.byName[name]
	return ok && ref.write
}

// ExecuteRead runs a read tool for the AI loop.
func (t *toolSession) ExecuteRead(ctx context.Context, name, argsJSON string) (string, error) {
	ref, ok := t.byName[name]
	if !ok || ref.write {
		return "", apperror.Validation("unknown read tool")
	}
	return t.svc.invoke(ctx, t.conv, ref.conn, name, parseArgs(argsJSON), "ai", "")
}

// ProposeWrite records a write tool the AI requested as a pending approval; it is
// never executed here.
func (t *toolSession) ProposeWrite(ctx context.Context, name, argsJSON string) (copilotcontracts.ProposedAction, error) {
	ref, ok := t.byName[name]
	if !ok {
		return copilotcontracts.ProposedAction{}, apperror.Validation("unknown tool")
	}
	args := parseArgs(argsJSON)
	approval := t.svc.newApproval(t.conv, ref.conn, name, args, "ai")
	if err := t.svc.approvals.Create(ctx, approval); err != nil {
		return copilotcontracts.ProposedAction{}, err
	}
	t.svc.publishApproval(ctx, t.conv.TenantID, approval)
	return copilotcontracts.ProposedAction{
		ApprovalID: approval.ID, Server: ref.conn.Name, Tool: name, Args: args,
	}, nil
}

// ── shared helpers ───────────────────────────────────────────────────────────

// invoke calls a tool over MCP and records a payload-free call log. This is the
// SINGLE seam where a tool call is dispatched, so it is also where the SMSNET ISP
// config{type+creds} is injected server-side (via the ISP bridge) — the model's
// args are decorated here, after the model has decided to call the tool, and any
// client-supplied "config" is overwritten. Credentials never reach the model.
func (s *ToolService) invoke(ctx context.Context, conv *convEntityRef, conn *entity.ServerConnection, tool string, args map[string]any, userID, idempotencyKey string) (string, error) {
	if s.ispBridge != nil && entity.IsSMSNETServer(conn.Name) {
		decorated, derr := s.ispBridge.Decorate(ctx, contracts.DecorateInput{
			ChannelType:    conv.Channel,
			ServerName:     conn.Name,
			Write:          conn.Kind == entity.KindWrite,
			IdempotencyKey: idempotencyKey,
			Args:           args,
		})
		if derr != nil {
			return "", apperror.Integration("could not resolve the ISP config for this tool").Wrap(derr)
		}
		args = decorated
	}
	start := s.clock.Now()
	res, err := s.client.CallTool(ctx, conn, tool, args)
	latency := s.clock.Now().Sub(start).Milliseconds()
	log := &entity.CallLog{
		ID: shared.NewID(), TenantID: conv.TenantID, UserID: userID, ConversationID: conv.ID,
		ServerID: conn.ID, ServerName: conn.Name, Tool: tool, Write: conn.Kind == entity.KindWrite,
		LatencyMs: latency, CreatedAt: s.clock.Now(),
	}
	if err != nil {
		log.Status = entity.CallError
		log.ErrorSummary = summarizeErr(err)
		_ = s.callLogs.Create(ctx, log)
		return "", apperror.Integration("tool call failed").Wrap(err)
	}
	if res.IsError {
		log.Status = entity.CallError
		log.ErrorSummary = "tool reported an error"
	} else {
		log.Status = entity.CallSuccess
	}
	_ = s.callLogs.Create(ctx, log)
	return truncate(res.Text, maxResultChars), nil
}

func (s *ToolService) newApproval(conv *convEntityRef, conn *entity.ServerConnection, tool string, args map[string]any, proposedBy string) *entity.Approval {
	return &entity.Approval{
		ID: shared.NewID(), TenantID: conv.TenantID, ConversationID: conv.ID,
		ServerID: conn.ID, ServerName: conn.Name, Tool: tool, Args: args,
		Status: entity.ApprovalPending, ProposedBy: proposedBy, CreatedAt: s.clock.Now(),
	}
}

func (s *ToolService) auditExecuted(ctx context.Context, approval *entity.Approval, status string) {
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "integration.execute_action", ResourceType: "mcp_tool", ResourceID: approval.Tool,
		Data: map[string]any{
			"server":      approval.ServerName,
			"approval_id": approval.ID,
			"proposed_by": approval.ProposedBy,
			"args":        maskArgs(approval.Args),
			"status":      status,
		},
	})
}

func (s *ToolService) publishApproval(ctx context.Context, tenantID string, a *entity.Approval) {
	payload := map[string]any{
		"approval_id": a.ID, "conversation_id": a.ConversationID,
		"server": a.ServerName, "tool": a.Tool, "args": a.Args, "proposed_by": a.ProposedBy,
	}
	_ = s.publisher.Publish(ctx, shared.TopicConversation(tenantID, a.ConversationID), contracts.RealtimeApprovalRequested, payload)
}

// convEntityRef is the minimal conversation projection used by the tool flows.
type convEntityRef struct {
	ID         string
	TenantID   string
	Channel    string // channel type, for resolving the conversation's assistant/ISP profile
	SectorID   string
	AssignedTo string
}

// loadVisible loads a conversation tenant-scoped and enforces the actor's
// visibility (sector scope / assignment), mirroring the copilot policy.
func (s *ToolService) loadVisible(ctx context.Context, id string) (*convEntityRef, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return nil, apperror.Unauthorized("authentication required")
	}
	conv, err := s.conversations.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ref := &convEntityRef{ID: conv.ID, TenantID: conv.TenantID, Channel: conv.Channel, SectorID: conv.SectorID, AssignedTo: conv.AssignedTo}
	if ac.SectorScope == authz.ScopeAll {
		return ref, nil
	}
	if ref.AssignedTo != "" && ref.AssignedTo == ac.UserID {
		return ref, nil
	}
	for _, sid := range ac.SectorIDs {
		if sid == ref.SectorID && sid != "" {
			return ref, nil
		}
	}
	return nil, apperror.NotFound("conversation not found")
}

func parseArgs(argsJSON string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(argsJSON) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(argsJSON), &out)
	return out
}

// maskArgs redacts secret-looking argument values for the audit trail.
func maskArgs(args map[string]any) map[string]any {
	if len(args) == 0 {
		return args
	}
	out := make(map[string]any, len(args))
	for k, v := range args {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "password") || strings.Contains(lk, "secret") ||
			strings.Contains(lk, "token") || strings.Contains(lk, "apikey") || strings.Contains(lk, "api_key") {
			out[k] = "***"
			continue
		}
		out[k] = v
	}
	return out
}

func summarizeErr(err error) string {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

var (
	_ copilotcontracts.ToolBroker  = (*ToolService)(nil)
	_ copilotcontracts.ToolSession = (*toolSession)(nil)
)

package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeClient struct {
	tools  map[string][]contracts.ToolSpec // by server id
	calls  []string                        // "<serverID>:<tool>"
	result string
}

func (c *fakeClient) ListTools(_ context.Context, conn *entity.ServerConnection) ([]contracts.ToolSpec, error) {
	return c.tools[conn.ID], nil
}
func (c *fakeClient) CallTool(_ context.Context, conn *entity.ServerConnection, tool string, _ map[string]any) (contracts.CallResult, error) {
	c.calls = append(c.calls, conn.ID+":"+tool)
	return contracts.CallResult{Text: c.result}, nil
}

type fakeServers struct {
	byID map[string]*entity.ServerConnection
}

func (r *fakeServers) Create(_ context.Context, s *entity.ServerConnection) error {
	r.byID[s.ID] = s
	return nil
}
func (r *fakeServers) Update(_ context.Context, s *entity.ServerConnection) error {
	r.byID[s.ID] = s
	return nil
}
func (r *fakeServers) Delete(_ context.Context, id string) error { delete(r.byID, id); return nil }
func (r *fakeServers) FindByID(_ context.Context, id string) (*entity.ServerConnection, error) {
	if s, ok := r.byID[id]; ok {
		return s, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeServers) List(context.Context, shared.PageRequest) ([]*entity.ServerConnection, error) {
	return r.enabled(), nil
}
func (r *fakeServers) ListEnabled(context.Context) ([]*entity.ServerConnection, error) {
	return r.enabled(), nil
}
func (r *fakeServers) enabled() []*entity.ServerConnection {
	var out []*entity.ServerConnection
	for _, s := range r.byID {
		if s.Enabled {
			out = append(out, s)
		}
	}
	return out
}

type fakeApprovals struct{ byID map[string]*entity.Approval }

func (r *fakeApprovals) Create(_ context.Context, a *entity.Approval) error {
	r.byID[a.ID] = a
	return nil
}
func (r *fakeApprovals) Update(_ context.Context, a *entity.Approval) error {
	r.byID[a.ID] = a
	return nil
}
func (r *fakeApprovals) FindByID(_ context.Context, id string) (*entity.Approval, error) {
	if a, ok := r.byID[id]; ok {
		return a, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeApprovals) ListByConversation(_ context.Context, convID string) ([]*entity.Approval, error) {
	out := make([]*entity.Approval, 0)
	for _, a := range r.byID {
		if a.ConversationID == convID {
			out = append(out, a)
		}
	}
	return out, nil
}

type fakeCallLogs struct{ entries []*entity.CallLog }

func (r *fakeCallLogs) Create(_ context.Context, l *entity.CallLog) error {
	r.entries = append(r.entries, l)
	return nil
}
func (r *fakeCallLogs) ListByConversation(_ context.Context, convID string) ([]*entity.CallLog, error) {
	out := make([]*entity.CallLog, 0)
	for _, l := range r.entries {
		if l.ConversationID == convID {
			out = append(out, l)
		}
	}
	return out, nil
}

type fakeConvRepo struct{ conv *conventity.Conversation }

func (r *fakeConvRepo) Create(context.Context, *conventity.Conversation) error { return nil }
func (r *fakeConvRepo) Update(context.Context, *conventity.Conversation) error { return nil }
func (r *fakeConvRepo) FindByID(_ context.Context, id string) (*conventity.Conversation, error) {
	if r.conv != nil && r.conv.ID == id {
		return r.conv, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindByIDs(context.Context, []string) ([]*conventity.Conversation, error) {
	return nil, nil
}
func (r *fakeConvRepo) FindLastByContactChannelID(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindOpenByContactChannelID(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindOpenByContact(context.Context, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindLastByContact(context.Context, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) List(context.Context, convcontracts.ListFilter, convcontracts.Visibility, shared.PageRequest) ([]*conventity.Conversation, error) {
	return nil, nil
}
func (r *fakeConvRepo) ListInactiveOpen(context.Context, time.Time, int) ([]*conventity.Conversation, error) {
	return nil, nil
}

type fakeAuditor struct{ entries []shared.AuditEntry }

func (a *fakeAuditor) Record(_ context.Context, e shared.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}
func (a *fakeAuditor) has(action string) *shared.AuditEntry {
	for i := range a.entries {
		if a.entries[i].Action == action {
			return &a.entries[i]
		}
	}
	return nil
}

// ── fixture ──────────────────────────────────────────────────────────────────

type fixture struct {
	svc       *ToolService
	client    *fakeClient
	servers   *fakeServers
	approvals *fakeApprovals
	logs      *fakeCallLogs
	auditor   *fakeAuditor
}

func newFixture() fixture {
	readSrv := &entity.ServerConnection{ID: "srv-read", TenantID: "t1", Name: "smsnet-queries", Kind: entity.KindRead, Enabled: true}
	writeSrv := &entity.ServerConnection{ID: "srv-write", TenantID: "t1", Name: "smsnet-ops", Kind: entity.KindWrite, Enabled: true}
	servers := &fakeServers{byID: map[string]*entity.ServerConnection{"srv-read": readSrv, "srv-write": writeSrv}}
	client := &fakeClient{
		tools: map[string][]contracts.ToolSpec{
			"srv-read":  {{Name: "consultar_cliente", Description: "lookup"}},
			"srv-write": {{Name: "liberar_acesso", Description: "release"}},
		},
		result: "tool-output",
	}
	approvals := &fakeApprovals{byID: map[string]*entity.Approval{}}
	logs := &fakeCallLogs{}
	conv := &conventity.Conversation{ID: "conv1", TenantID: "t1"}
	auditor := &fakeAuditor{}
	svc := NewToolService(servers, approvals, logs, &fakeConvRepo{conv: conv}, client, shared.NoopPublisher{}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	svc.SetAuditor(auditor)
	return fixture{svc: svc, client: client, servers: servers, approvals: approvals, logs: logs, auditor: auditor}
}

func ctxWith(perms ...authz.Permission) context.Context {
	ctx := shared.WithTenant(context.Background(), "t1")
	return authz.WithAuthContext(ctx, authz.NewAuthContext("t1", "agent1", perms, nil, authz.ScopeAll))
}

// ── tests ────────────────────────────────────────────────────────────────────

// Two servers (a read + a write one, simulating smsnet queries and a future OLT)
// aggregate with no code change — only their registration differs.
func TestTools_AggregatesMultipleServers(t *testing.T) {
	fx := newFixture()
	tools, err := fx.svc.Tools(ctxWith(authz.IntegrationRead), "conv1")
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected tools from both servers, got %d", len(tools))
	}
	byName := map[string]entity.Tool{}
	for _, tl := range tools {
		byName[tl.Name] = tl
	}
	if byName["consultar_cliente"].Write {
		t.Error("read-server tool must be marked read")
	}
	if !byName["liberar_acesso"].Write {
		t.Error("write-server tool must be marked write")
	}
}

func TestRun_ReadExecutesDirectly(t *testing.T) {
	fx := newFixture()
	res, err := fx.svc.Run(ctxWith(authz.IntegrationRead), contracts.RunTool{
		ConversationID: "conv1", ServerID: "srv-read", Tool: "consultar_cliente", Args: map[string]any{"cpf": "123"},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !res.Executed || res.Result != "tool-output" {
		t.Fatalf("read should execute directly: %+v", res)
	}
	if len(fx.logs.entries) != 1 || fx.logs.entries[0].Status != entity.CallSuccess {
		t.Fatalf("a success call log should be recorded: %+v", fx.logs.entries)
	}
}

func TestRun_WriteRequiresExecutePermission(t *testing.T) {
	fx := newFixture()
	_, err := fx.svc.Run(ctxWith(authz.IntegrationRead), contracts.RunTool{
		ConversationID: "conv1", ServerID: "srv-write", Tool: "liberar_acesso",
	})
	if apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("write without execute_action must be forbidden, got %v", err)
	}
}

func TestRun_WriteNeverExecutesWithoutApproval(t *testing.T) {
	fx := newFixture()
	res, err := fx.svc.Run(ctxWith(authz.IntegrationRead, authz.IntegrationExecuteAction), contracts.RunTool{
		ConversationID: "conv1", ServerID: "srv-write", Tool: "liberar_acesso", Args: map[string]any{"id": "c1"},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Executed || res.Approval == nil || res.Approval.Status != entity.ApprovalPending {
		t.Fatalf("write must create a pending approval, not execute: %+v", res)
	}
	if len(fx.client.calls) != 0 {
		t.Fatalf("the tool must NOT have been called: %v", fx.client.calls)
	}
}

func TestDecide_ApprovalExecutesAndAudits(t *testing.T) {
	fx := newFixture()
	// Propose a write (manual).
	run, _ := fx.svc.Run(ctxWith(authz.IntegrationRead, authz.IntegrationExecuteAction), contracts.RunTool{
		ConversationID: "conv1", ServerID: "srv-write", Tool: "liberar_acesso", Args: map[string]any{"id": "c1", "token": "shh"},
	})
	approvalID := run.Approval.ID

	res, err := fx.svc.Decide(ctxWith(authz.IntegrationExecuteAction), "conv1", approvalID, true, "ok")
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if !res.Executed || res.Result != "tool-output" {
		t.Fatalf("approval should execute the tool: %+v", res)
	}
	if len(fx.client.calls) != 1 || fx.client.calls[0] != "srv-write:liberar_acesso" {
		t.Fatalf("the tool must run exactly once on approval: %v", fx.client.calls)
	}
	if fx.approvals.byID[approvalID].Status != entity.ApprovalExecuted {
		t.Errorf("approval status should be executed")
	}
	entry := fx.auditor.has("integration.execute_action")
	if entry == nil {
		t.Fatal("the executed action must be audited")
	}
	args, _ := entry.Data["args"].(map[string]any)
	if args["token"] != "***" {
		t.Errorf("secret args must be masked in the audit, got %v", args["token"])
	}
}

func TestDecide_RejectDoesNotExecute(t *testing.T) {
	fx := newFixture()
	run, _ := fx.svc.Run(ctxWith(authz.IntegrationRead, authz.IntegrationExecuteAction), contracts.RunTool{
		ConversationID: "conv1", ServerID: "srv-write", Tool: "liberar_acesso",
	})
	res, err := fx.svc.Decide(ctxWith(authz.IntegrationExecuteAction), "conv1", run.Approval.ID, false, "not now")
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if res.Executed || len(fx.client.calls) != 0 {
		t.Fatalf("rejected approval must not execute: %+v calls=%v", res, fx.client.calls)
	}
	if fx.approvals.byID[run.Approval.ID].Status != entity.ApprovalRejected {
		t.Errorf("approval should be rejected")
	}
}

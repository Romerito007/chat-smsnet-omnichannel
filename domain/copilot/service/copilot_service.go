package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// friendlyUnavailable is the user-safe message when the provider call fails.
const friendlyUnavailable = "the AI copilot is temporarily unavailable, please try again"

// maxToolIterations bounds the agentic read-tool loop so a model cannot spin.
const maxToolIterations = 6

// Service orchestrates copilot inference: it loads the tenant config, builds a
// policy-filtered context, calls the configured provider, logs the call and
// publishes the realtime result. When a tool broker is wired, suggest_reply runs
// an agentic loop: the model calls read tools (executed via MCP) and proposes
// write tools (never executed — surfaced as approval cards).
type Service struct {
	config        *ConfigService
	logs          repository.LogRepository
	conversations convrepo.ConversationRepository
	builder       *ContextBuilder
	resolver      contracts.ProviderResolver
	publisher     shared.EventPublisher
	clock         shared.Clock
	tools         contracts.ToolBroker
	logger        shared.Logger
	envKeys       map[entity.Provider]string
	assistants    repository.AssistantRepository
}

// SetAssistantResolver wires the assistant repository so the copilot resolves the
// conversation's assistant (by channel_id) to apply its behavior (gates, sampling,
// persona). Optional: when unset, every conversation uses DefaultBehavior.
func (s *Service) SetAssistantResolver(a repository.AssistantRepository) {
	if a != nil {
		s.assistants = a
	}
}

// resolveBehavior resolves the conversation channel's assistant behavior; when no
// assistant serves the channel (or channel_id is empty), the conservative
// DefaultBehavior is used — all data gates OFF, no persona, default sampling.
func (s *Service) resolveBehavior(ctx context.Context, channelID string) entity.Behavior {
	if s.assistants == nil || channelID == "" {
		return entity.DefaultBehavior()
	}
	a, err := s.assistants.FindByChannelID(ctx, channelID)
	if err != nil || a == nil {
		return entity.DefaultBehavior()
	}
	return a.Behavior()
}

// SetToolBroker wires the MCP tool broker enabling the agentic loop. Optional:
// when unset, the copilot runs a plain (tool-less) completion.
func (s *Service) SetToolBroker(b contracts.ToolBroker) {
	if b != nil {
		s.tools = b
	}
}

// SetLogger wires a server-side logger so a provider failure records its real
// cause (HTTP status + provider error body) in the logs, not just the AILog.
func (s *Service) SetLogger(l shared.Logger) {
	if l != nil {
		s.logger = l
	}
}

// SetEnvKeys wires the env-default provider API keys, used as a fallback when a
// tenant has selected a provider but set no key of its own.
func (s *Service) SetEnvKeys(keys map[entity.Provider]string) {
	s.envKeys = keys
}

// NewService builds the copilot service.
func NewService(
	config *ConfigService,
	logs repository.LogRepository,
	conversations convrepo.ConversationRepository,
	builder *ContextBuilder,
	resolver contracts.ProviderResolver,
	publisher shared.EventPublisher,
	clock shared.Clock,
) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if publisher == nil {
		publisher = shared.NoopPublisher{}
	}
	return &Service{
		config: config, logs: logs, conversations: conversations,
		builder: builder, resolver: resolver, publisher: publisher, clock: clock,
	}
}

// SuggestReply drafts a reply for the conversation.
func (s *Service) SuggestReply(ctx context.Context, in contracts.SuggestReplyInput) (contracts.Result, error) {
	return s.run(ctx, in.ConversationID, entity.ActionSuggestReply, in.Instruction, nil)
}

// Summarize summarizes the conversation.
func (s *Service) Summarize(ctx context.Context, in contracts.SummarizeInput) (contracts.Result, error) {
	return s.run(ctx, in.ConversationID, entity.ActionSummarize, "", nil)
}

// Classify classifies the conversation into one of the given categories.
func (s *Service) Classify(ctx context.Context, in contracts.ClassifyInput) (contracts.Result, error) {
	if len(in.Categories) == 0 {
		return contracts.Result{}, apperror.Validation("at least one category is required").
			WithDetails(map[string]any{"categories": "is required"})
	}
	return s.run(ctx, in.ConversationID, entity.ActionClassify, "categories: "+strings.Join(in.Categories, ", "), in.Categories)
}

// NextAction recommends the next action for the conversation.
func (s *Service) NextAction(ctx context.Context, in contracts.NextActionInput) (contracts.Result, error) {
	return s.run(ctx, in.ConversationID, entity.ActionNextAction, "", nil)
}

// run is the shared pipeline for every action.
func (s *Service) run(ctx context.Context, conversationID string, action entity.Action, instruction string, categories []string) (contracts.Result, error) {
	conv, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return contracts.Result{}, err
	}

	cfg, err := s.config.Current(ctx)
	if err != nil {
		return contracts.Result{}, err
	}
	if !cfg.Enabled {
		return contracts.Result{}, apperror.Validation("copilot is disabled for this tenant")
	}

	// Behavior (gates/sampling/persona) comes from the conversation's assistant; no
	// assistant → conservative DefaultBehavior.
	behavior := s.resolveBehavior(ctx, conv.ChannelID)

	provider, err := s.resolver.Resolve(cfg.Provider)
	if err != nil {
		s.log(ctx, conv, cfg, behavior, action, "", entity.StatusError, 0, 0, "provider not available: "+string(cfg.Provider))
		return contracts.Result{}, apperror.Integration(friendlyUnavailable)
	}

	// Key precedence: the tenant config key wins; if it is empty, fall back to the
	// env-default key for the provider. If both are empty, fail with an actionable
	// message instead of letting the provider return a generic auth error.
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = s.envKeys[cfg.Provider]
	}
	if apiKey == "" {
		s.log(ctx, conv, cfg, behavior, action, "", entity.StatusError, 0, 0, "no api key (tenant or env)")
		return contracts.Result{}, apperror.Integration(
			"the copilot API key is not configured — set it in copilot settings or via env").
			WithDetails(map[string]any{"category": "api_key_missing"})
	}

	pc := s.builder.Build(ctx, behavior, conv, instruction)
	base := contracts.Request{
		Action:             action,
		Model:              cfg.Model,
		APIKey:             apiKey,
		BaseURL:            cfg.BaseURL,
		Temperature:        behavior.Temperature,
		MaxTokens:          behavior.MaxTokens,
		SystemInstructions: behavior.SystemInstructions,
		Context:            pc,
	}

	var resp contracts.Response
	var proposed []contracts.ProposedAction
	if action == entity.ActionSuggestReply && s.tools != nil {
		resp, proposed, err = s.agenticLoop(ctx, conv.ID, provider, base)
	} else {
		resp, err = provider.Infer(ctx, base)
	}
	if err != nil {
		s.log(ctx, conv, cfg, behavior, action, "", entity.StatusError, 0, 0, summarize(err))
		// Record the real provider cause server-side (status + error body) so the
		// 502 is diagnosable; the client gets a friendly message + safe category.
		if s.logger != nil {
			tenantID, _ := shared.TenantFrom(ctx)
			s.logger.Error("copilot provider call failed",
				"tenant_id", tenantID, "provider", string(cfg.Provider), "model", cfg.Model,
				"action", string(action), "base_url", cfg.BaseURL, "cause", err.Error())
		}
		category, message := classifyProviderError(err)
		return contracts.Result{}, apperror.Integration(message).
			WithDetails(map[string]any{"category": category})
	}

	cost := estimateCost(cfg.Provider, resp.TokensInput, resp.TokensOutput)
	// A proposed write action always needs approval, regardless of the gate.
	requiresApproval := behavior.HumanApprovalRequired || len(proposed) > 0
	status := entity.StatusSuccess
	if requiresApproval {
		status = entity.StatusPendingApproval
	}

	result := contracts.Result{
		Action:           action,
		Provider:         provider.Name(),
		Model:            cfg.Model,
		Text:             resp.Text,
		Categories:       resp.Categories,
		TokensInput:      resp.TokensInput,
		TokensOutput:     resp.TokensOutput,
		EstimatedCost:    cost,
		RequiresApproval: requiresApproval,
		ProposedActions:  proposed,
	}

	s.log(ctx, conv, cfg, behavior, action, outputSummary(resp), status, resp.TokensInput, resp.TokensOutput, "")
	s.publish(ctx, conv, result)
	return result, nil
}

// agenticLoop runs the read-tool loop: the model calls read tools (executed via
// the broker) until it produces a final answer; write tool calls are proposed as
// approval cards and never executed. Token counts are accumulated across turns.
func (s *Service) agenticLoop(ctx context.Context, conversationID string, provider contracts.AIProvider, base contracts.Request) (contracts.Response, []contracts.ProposedAction, error) {
	session, err := s.tools.OpenToolSession(ctx, conversationID)
	if err != nil || len(session.Tools()) == 0 {
		resp, ierr := provider.Infer(ctx, base) // no tools → plain completion
		return resp, nil, ierr
	}
	base.Tools = session.Tools()

	var (
		history  []contracts.ToolExchange
		proposed []contracts.ProposedAction
		tokIn    int
		tokOut   int
		last     contracts.Response
	)
	for i := 0; i < maxToolIterations; i++ {
		base.ToolHistory = history
		resp, ierr := provider.Infer(ctx, base)
		if ierr != nil {
			return contracts.Response{}, nil, ierr
		}
		tokIn += resp.TokensInput
		tokOut += resp.TokensOutput
		last = resp
		if len(resp.ToolCalls) == 0 {
			last.TokensInput, last.TokensOutput = tokIn, tokOut
			return last, proposed, nil // final answer
		}

		results := make([]contracts.ToolResult, 0, len(resp.ToolCalls))
		sawWrite := false
		for _, call := range resp.ToolCalls {
			if session.IsWrite(call.Name) {
				if pa, perr := session.ProposeWrite(ctx, call.Name, call.Arguments); perr == nil {
					proposed = append(proposed, pa)
				}
				results = append(results, contracts.ToolResult{ID: call.ID, Name: call.Name,
					Content: "This is a write action; it has been proposed and requires explicit human approval before it can run."})
				sawWrite = true
				continue
			}
			out, rerr := session.ExecuteRead(ctx, call.Name, call.Arguments)
			if rerr != nil {
				out = "tool error: " + rerr.Error()
			}
			results = append(results, contracts.ToolResult{ID: call.ID, Name: call.Name, Content: out})
		}
		history = append(history, contracts.ToolExchange{Calls: resp.ToolCalls, Results: results})
		if sawWrite {
			break // present the proposal(s) for approval; do not auto-continue
		}
	}
	last.TokensInput, last.TokensOutput = tokIn, tokOut
	return last, proposed, nil
}

// loadVisible loads a conversation and enforces the actor's tenant + visibility.
func (s *Service) loadVisible(ctx context.Context, id string) (*conventity.Conversation, error) {
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
	if ac.SectorScope == authz.ScopeAll {
		return conv, nil
	}
	if conv.AssignedTo != "" && conv.AssignedTo == ac.UserID {
		return conv, nil
	}
	for _, sid := range ac.SectorIDs {
		if sid == conv.SectorID && sid != "" {
			return conv, nil
		}
	}
	return nil, apperror.NotFound("conversation not found")
}

func (s *Service) log(ctx context.Context, conv *conventity.Conversation, cfg *entity.AIConfig, behavior entity.Behavior, action entity.Action, output string, status entity.LogStatus, tokIn, tokOut int, errMsg string) {
	userID := ""
	if ac, ok := authz.FromContext(ctx); ok {
		userID = ac.UserID
	}
	_ = s.logs.Create(ctx, &entity.AILog{
		ID:             shared.NewID(),
		TenantID:       conv.TenantID,
		UserID:         userID,
		ConversationID: conv.ID,
		Provider:       string(cfg.Provider),
		Model:          cfg.Model,
		Action:         action,
		InputSummary:   inputSummary(behavior, action),
		OutputSummary:  output,
		TokensInput:    tokIn,
		TokensOutput:   tokOut,
		EstimatedCost:  estimateCost(cfg.Provider, tokIn, tokOut),
		Status:         status,
		Error:          errMsg,
		CreatedAt:      s.clock.Now(),
	})
}

func (s *Service) publish(ctx context.Context, conv *conventity.Conversation, result contracts.Result) {
	if ac, ok := authz.FromContext(ctx); ok && ac.UserID != "" {
		_ = s.publisher.Publish(ctx, shared.TopicUser(conv.TenantID, ac.UserID), contracts.RealtimeSuggestionCompleted, result)
	}
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID), contracts.RealtimeSuggestionCompleted, result)
}

// inputSummary records which policy-gated sections were eligible, without any
// raw data — an audit trail of what the model could see.
func inputSummary(beh entity.Behavior, action entity.Action) string {
	return fmt.Sprintf("action=%s customer=%t financial=%t monitoring=%t",
		action, beh.AllowCustomerData, beh.AllowFinancialData, beh.AllowMonitoringData)
}

func outputSummary(resp contracts.Response) string {
	if len(resp.Categories) > 0 {
		return "categories: " + strings.Join(resp.Categories, ", ")
	}
	out := resp.Text
	if len(out) > 200 {
		out = out[:200]
	}
	return out
}

func summarize(err error) string {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}

// classifyProviderError maps a provider error (which carries the HTTP status and
// the provider's error body, e.g. "provider returned 401: ...invalid_api_key...")
// to a safe category and a friendly, actionable message. The raw provider body is
// never returned to the client — only the category.
func classifyProviderError(err error) (category, message string) {
	e := strings.ToLower(err.Error())
	switch {
	case strings.Contains(e, "401") || strings.Contains(e, "invalid_api_key") ||
		strings.Contains(e, "invalid api key") || strings.Contains(e, "not configured") ||
		strings.Contains(e, "no api key") || strings.Contains(e, "incorrect api key"):
		return "api_key", "the copilot API key appears to be invalid or missing — check the copilot settings"
	case strings.Contains(e, "model") && (strings.Contains(e, "404") ||
		strings.Contains(e, "not found") || strings.Contains(e, "does not exist")):
		return "model", "the configured copilot model is not available for this API key"
	case strings.Contains(e, "429") || strings.Contains(e, "insufficient_quota") ||
		strings.Contains(e, "quota") || strings.Contains(e, "rate limit"):
		return "quota", "the AI provider quota/rate limit was reached — try again shortly"
	case strings.Contains(e, "timeout") || strings.Contains(e, "deadline") ||
		strings.Contains(e, "no such host") || strings.Contains(e, "connection refused"):
		return "unreachable", friendlyUnavailable
	default:
		return "unavailable", friendlyUnavailable
	}
}

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

// Service orchestrates copilot inference: it loads the tenant config, builds a
// policy-filtered context, calls the configured provider, logs the call and
// publishes the realtime result.
type Service struct {
	config        *ConfigService
	logs          repository.LogRepository
	conversations convrepo.ConversationRepository
	builder       *ContextBuilder
	resolver      contracts.ProviderResolver
	publisher     shared.EventPublisher
	clock         shared.Clock
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

	provider, err := s.resolver.Resolve(cfg.Provider)
	if err != nil {
		s.log(ctx, conv, cfg, action, "", entity.StatusError, 0, 0, "provider not available: "+string(cfg.Provider))
		return contracts.Result{}, apperror.Integration(friendlyUnavailable)
	}

	pc := s.builder.Build(ctx, cfg, conv, instruction)

	resp, err := provider.Infer(ctx, contracts.Request{
		Action:      action,
		Model:       cfg.Model,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
		Context:     pc,
	})
	if err != nil {
		s.log(ctx, conv, cfg, action, "", entity.StatusError, 0, 0, summarize(err))
		return contracts.Result{}, apperror.Integration(friendlyUnavailable)
	}

	cost := estimateCost(cfg.Provider, resp.TokensInput, resp.TokensOutput)
	status := entity.StatusSuccess
	if cfg.HumanApprovalRequired {
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
		RequiresApproval: cfg.HumanApprovalRequired,
	}

	s.log(ctx, conv, cfg, action, outputSummary(resp), status, resp.TokensInput, resp.TokensOutput, "")
	s.publish(ctx, conv, result)
	return result, nil
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

func (s *Service) log(ctx context.Context, conv *conventity.Conversation, cfg *entity.AIConfig, action entity.Action, output string, status entity.LogStatus, tokIn, tokOut int, errMsg string) {
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
		InputSummary:   inputSummary(cfg, action),
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
func inputSummary(cfg *entity.AIConfig, action entity.Action) string {
	return fmt.Sprintf("action=%s customer=%t financial=%t monitoring=%t",
		action, cfg.AllowCustomerData, cfg.AllowFinancialData, cfg.AllowMonitoringData)
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

package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/entity"
	autorepo "github.com/romerito007/chat-smsnet-omnichannel/domain/automation/repository"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	routingcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/routing/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service implements the automation use cases: start a run, handle the callback,
// apply a flow decision, and handle timeouts. The flow itself is external.
type Service struct {
	integrations    autorepo.IntegrationRepository
	runs            autorepo.RunRepository
	conversations   convrepo.ConversationRepository
	messages        convrepo.MessageRepository
	events          convrepo.EventRepository
	router          contracts.Router
	outbound        convcontracts.OutboundDispatcher
	flow            contracts.FlowClient
	timeouts        contracts.TimeoutScheduler
	publisher       shared.EventPublisher
	clock           shared.Clock
	callbackBaseURL string
	webhooks        shared.WebhookEmitter
	businessHours   shared.BusinessHoursChecker
}

// SetWebhookEmitter wires the outbound webhook emitter. Optional: when unset,
// automation completion/failure events are not forwarded to webhooks.
func (s *Service) SetWebhookEmitter(e shared.WebhookEmitter) {
	if e != nil {
		s.webhooks = e
	}
}

// SetBusinessHoursChecker wires the business-hours checker. When set, the flow
// input carries whether the conversation's sector is within business hours so
// the external flow can route off-hours conversations (we never send an
// automatic message here — that is the flow's job).
func (s *Service) SetBusinessHoursChecker(b shared.BusinessHoursChecker) {
	if b != nil {
		s.businessHours = b
	}
}

// New builds the automation service.
func New(
	integrations autorepo.IntegrationRepository,
	runs autorepo.RunRepository,
	conversations convrepo.ConversationRepository,
	messages convrepo.MessageRepository,
	events convrepo.EventRepository,
	router contracts.Router,
	outbound convcontracts.OutboundDispatcher,
	flow contracts.FlowClient,
	timeouts contracts.TimeoutScheduler,
	publisher shared.EventPublisher,
	clock shared.Clock,
	callbackBaseURL string,
) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if publisher == nil {
		publisher = shared.NoopPublisher{}
	}
	return &Service{
		integrations: integrations, runs: runs, conversations: conversations,
		messages: messages, events: events, router: router, outbound: outbound,
		flow: flow, timeouts: timeouts, publisher: publisher, clock: clock,
		callbackBaseURL: strings.TrimRight(callbackBaseURL, "/"),
		webhooks:        shared.NoopWebhookEmitter{},
		businessHours:   shared.NoopBusinessHoursChecker{},
	}
}

// StartConversationAutomation starts a run for a new conversation by invoking the
// external flow. Without an enabled integration, the conversation is escalated to
// a human. A synchronous decision is applied immediately; otherwise the run waits
// for a callback (with a timeout).
func (s *Service) StartConversationAutomation(ctx context.Context, conversationID, messageID string) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	conv, err := s.conversations.FindByID(ctx, conversationID)
	if err != nil {
		return err
	}

	integration, err := s.integrations.FindEnabled(ctx)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			s.escalate(ctx, conv, "no automation integration configured")
			return nil
		}
		return err
	}

	now := s.clock.Now()
	run := &entity.AutomationRun{
		ID:             shared.NewID(),
		TenantID:       tenantID,
		ConversationID: conversationID,
		MessageID:      messageID,
		Status:         entity.RunStarted,
		Input:          map[string]any{"channel": conv.Channel, "contact_id": conv.ContactID},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.runs.Create(ctx, run); err != nil {
		return err
	}

	input := contracts.FlowInput{
		ConversationID: conv.ID,
		MessageID:      messageID,
		ContactID:      conv.ContactID,
		Channel:        conv.Channel,
		Text:           s.lastInboundText(ctx, conv.ID, messageID),
		CallbackURL:    s.callbackBaseURL + "/v1/automation/callbacks/" + tenantID,
	}
	// Let the external flow route off-hours conversations: pass whether the
	// sector is within business hours. We never send an automatic message here.
	if conv.SectorID != "" {
		if within, herr := s.businessHours.IsWithinBusinessHours(ctx, conv.SectorID, now); herr == nil {
			input.Metadata = map[string]any{"within_business_hours": within}
		}
	}

	result, err := s.flow.Start(ctx, integration, input)
	if err != nil {
		s.markRunFailed(ctx, run, err.Error())
		s.escalate(ctx, conv, "automation start failed")
		return nil
	}

	run.ExternalRunID = result.ExternalRunID
	if result.Decision != nil {
		if derr := s.ApplyDecision(ctx, run, *result.Decision); derr != nil {
			s.markRunFailed(ctx, run, derr.Error())
			s.escalate(ctx, conv, "automation decision failed")
			return nil
		}
		return s.completeRun(ctx, run)
	}

	run.Status = entity.RunWaitingCallback
	run.UpdatedAt = s.clock.Now()
	if err := s.runs.Update(ctx, run); err != nil {
		return err
	}
	if s.timeouts != nil {
		_ = s.timeouts.ScheduleTimeout(contracts.TimeoutTask{TenantID: tenantID, RunID: run.ID}, integration.TimeoutMs)
	}
	return nil
}

// HandleCallback validates the signature and applies the flow's decision to the
// run identified by external_run_id. Idempotent: a terminal run is left as-is.
func (s *Service) HandleCallback(ctx context.Context, tenantID string, rawBody []byte, signature string) error {
	ctx = shared.WithTenant(ctx, tenantID)
	integration, err := s.integrations.FindEnabled(ctx)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return apperror.Unauthorized("no automation integration")
		}
		return err
	}
	if !validSignature(integration.Secret, rawBody, signature) {
		return apperror.Unauthorized("invalid callback signature")
	}

	var cb contracts.Callback
	if err := json.Unmarshal(rawBody, &cb); err != nil {
		return apperror.Validation("invalid callback body").Wrap(err)
	}
	if cb.ExternalRunID == "" {
		return apperror.Validation("external_run_id is required")
	}

	run, err := s.runs.FindByExternalRunID(ctx, cb.ExternalRunID)
	if err != nil {
		return err
	}
	if run.Status.IsTerminal() {
		return nil // already resolved → idempotent
	}

	run.Output = cb.Output
	if cb.Error != "" {
		s.markRunFailed(ctx, run, cb.Error)
		if conv, err := s.conversations.FindByID(ctx, run.ConversationID); err == nil {
			s.escalate(ctx, conv, "automation reported an error")
		}
		return nil
	}
	if cb.Decision != nil {
		if err := s.ApplyDecision(ctx, run, *cb.Decision); err != nil {
			s.markRunFailed(ctx, run, err.Error())
			if conv, cerr := s.conversations.FindByID(ctx, run.ConversationID); cerr == nil {
				s.escalate(ctx, conv, "automation decision failed")
			}
			return nil
		}
	}
	return s.completeRun(ctx, run)
}

// HandleTimeout marks a still-waiting run as timed out and escalates to a human.
func (s *Service) HandleTimeout(ctx context.Context, runID string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	run, err := s.runs.FindByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != entity.RunWaitingCallback {
		return nil // already resolved
	}
	run.Status = entity.RunTimeout
	run.UpdatedAt = s.clock.Now()
	if err := s.runs.Update(ctx, run); err != nil {
		return err
	}
	if conv, err := s.conversations.FindByID(ctx, run.ConversationID); err == nil {
		s.escalate(ctx, conv, "automation timed out")
	}
	return nil
}

// ApplyDecision applies one external-flow decision, recording a ConversationEvent
// and publishing realtime as appropriate.
func (s *Service) ApplyDecision(ctx context.Context, run *entity.AutomationRun, decision contracts.Decision) error {
	conv, err := s.conversations.FindByID(ctx, run.ConversationID)
	if err != nil {
		return err
	}
	s.recordEvent(ctx, conv, conventity.EventAutomationDecision, map[string]any{
		"type": string(decision.Type), "run_id": run.ID,
	})

	sctx := authz.WithAuthContext(ctx, authz.SystemActor(conv.TenantID))

	switch decision.Type {
	case contracts.DecisionSendMessage:
		return s.sendAutomationMessage(ctx, conv, decision.Text)
	case contracts.DecisionAssignSector:
		_, err := s.router.Transfer(sctx, conv.ID, routingcontracts.TransferCommand{SectorID: decision.SectorID})
		return err
	case contracts.DecisionAssignAgent:
		_, err := s.router.Assign(sctx, conv.ID, decision.AgentID)
		return err
	case contracts.DecisionEnqueue, contracts.DecisionRequestHuman:
		return s.enqueueToHuman(sctx, conv, decision.QueueID, decision.SectorID)
	case contracts.DecisionCloseConversation:
		return s.closeConversation(ctx, conv, decision.ReasonID)
	case contracts.DecisionAddTag:
		return s.addTag(ctx, conv, decision.Tag)
	case contracts.DecisionSetPriority:
		return s.setPriority(ctx, conv, decision.Priority)
	case contracts.DecisionCallWebhook, contracts.DecisionNoAction:
		// call_webhook is recorded as an event; outbound webhook delivery is the
		// webhooks domain's responsibility. no_action does nothing further.
		return nil
	default:
		return apperror.Validation("unknown automation decision: " + string(decision.Type))
	}
}

// ── decision implementations ─────────────────────────────────────────────────

func (s *Service) sendAutomationMessage(ctx context.Context, conv *conventity.Conversation, text string) error {
	if strings.TrimSpace(text) == "" {
		return apperror.Validation("send_message requires text")
	}
	now := s.clock.Now()
	msg := &conventity.Message{
		ID:             shared.NewID(),
		TenantID:       conv.TenantID,
		ConversationID: conv.ID,
		SenderType:     conventity.SenderAutomation,
		Direction:      conventity.DirectionOutbound,
		MessageType:    conventity.MessageText,
		Text:           text,
		CreatedAt:      now,
		DeliveryStatus: conventity.DeliveryPending,
	}
	if err := s.messages.Create(ctx, msg); err != nil {
		return err
	}
	conv.LastMessageAt = now
	conv.UpdatedAt = now
	if err := s.conversations.Update(ctx, conv); err != nil {
		return err
	}
	s.recordEvent(ctx, conv, conventity.EventMessageCreated, map[string]any{"message_id": msg.ID})
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID),
		convcontracts.RealtimeMessageCreated, convcontracts.NewMessagePayload(msg))
	s.publishConversation(ctx, conv)
	if s.outbound != nil {
		s.outbound.Dispatch(ctx, conv, msg)
	}
	return nil
}

func (s *Service) enqueueToHuman(sctx context.Context, conv *conventity.Conversation, queueID, sectorID string) error {
	if queueID != "" {
		_, err := s.router.Enqueue(sctx, conv.ID, routingcontracts.EnqueueCommand{QueueID: queueID})
		return err
	}
	if sectorID != "" {
		conv.SectorID = sectorID
	}
	conv.Status = conventity.StatusQueued
	conv.AssignedTo = ""
	conv.UpdatedAt = s.clock.Now()
	if err := s.conversations.Update(sctx, conv); err != nil {
		return err
	}
	s.recordEvent(sctx, conv, conventity.EventConversationEnqueued, map[string]any{"sector_id": conv.SectorID})
	s.publishConversation(sctx, conv)
	return nil
}

func (s *Service) closeConversation(ctx context.Context, conv *conventity.Conversation, reasonID string) error {
	now := s.clock.Now()
	conv.Status = conventity.StatusClosed
	conv.ClosedAt = &now
	conv.UpdatedAt = now
	if err := s.conversations.Update(ctx, conv); err != nil {
		return err
	}
	s.recordEvent(ctx, conv, conventity.EventConversationClosed, map[string]any{"close_reason_id": reasonID})
	s.publishConversation(ctx, conv)
	return nil
}

func (s *Service) addTag(ctx context.Context, conv *conventity.Conversation, tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil
	}
	for _, t := range conv.Tags {
		if t == tag {
			return nil
		}
	}
	conv.Tags = append(conv.Tags, tag)
	conv.UpdatedAt = s.clock.Now()
	if err := s.conversations.Update(ctx, conv); err != nil {
		return err
	}
	s.recordEvent(ctx, conv, conventity.EventConversationUpdated, map[string]any{"added_tag": tag})
	s.publishConversation(ctx, conv)
	return nil
}

func (s *Service) setPriority(ctx context.Context, conv *conventity.Conversation, priority string) error {
	p := conventity.Priority(priority)
	if !p.Valid() {
		return apperror.Validation("invalid priority")
	}
	conv.Priority = p
	conv.UpdatedAt = s.clock.Now()
	if err := s.conversations.Update(ctx, conv); err != nil {
		return err
	}
	s.recordEvent(ctx, conv, conventity.EventConversationUpdated, map[string]any{"priority": priority})
	s.publishConversation(ctx, conv)
	return nil
}

// escalate moves a conversation to a human queue (used on no-integration/failure/
// timeout) so automation never strands a conversation.
func (s *Service) escalate(ctx context.Context, conv *conventity.Conversation, reason string) {
	if conv.Status.IsClosed() {
		return
	}
	conv.Status = conventity.StatusQueued
	conv.AssignedTo = ""
	conv.UpdatedAt = s.clock.Now()
	if err := s.conversations.Update(ctx, conv); err != nil {
		return
	}
	s.recordEvent(ctx, conv, conventity.EventAutomationEscalated, map[string]any{"reason": reason})
	s.publishConversation(ctx, conv)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (s *Service) markRunFailed(ctx context.Context, run *entity.AutomationRun, errMsg string) {
	run.Status = entity.RunFailed
	run.Error = errMsg
	run.UpdatedAt = s.clock.Now()
	_ = s.runs.Update(ctx, run)
	s.webhooks.Emit(ctx, run.TenantID, webhookAutomationFailed, "", runPayload(run))
}

// completeRun marks a run completed and emits the automation.completed webhook.
func (s *Service) completeRun(ctx context.Context, run *entity.AutomationRun) error {
	run.Status = entity.RunCompleted
	run.UpdatedAt = s.clock.Now()
	if err := s.runs.Update(ctx, run); err != nil {
		return err
	}
	s.webhooks.Emit(ctx, run.TenantID, webhookAutomationCompleted, "", runPayload(run))
	return nil
}

// Webhook event names emitted by automation (kept local to avoid importing the
// webhooks domain; values match the canonical webhook event names).
const (
	webhookAutomationCompleted = "automation.completed"
	webhookAutomationFailed    = "automation.failed"
)

// runPayload is the compact webhook body for an automation run outcome.
func runPayload(run *entity.AutomationRun) map[string]any {
	return map[string]any{
		"run_id":          run.ID,
		"conversation_id": run.ConversationID,
		"external_run_id": run.ExternalRunID,
		"status":          string(run.Status),
		"error":           run.Error,
	}
}

func (s *Service) lastInboundText(ctx context.Context, conversationID, messageID string) string {
	if messageID == "" {
		return ""
	}
	if msg, err := s.messages.FindByID(ctx, messageID); err == nil {
		return msg.Text
	}
	return ""
}

func (s *Service) recordEvent(ctx context.Context, conv *conventity.Conversation, eventType string, data map[string]any) {
	_ = s.events.Create(ctx, &conventity.ConversationEvent{
		ID:             shared.NewID(),
		TenantID:       conv.TenantID,
		ConversationID: conv.ID,
		Type:           eventType,
		ActorType:      conventity.ActorAutomation,
		Data:           data,
		CreatedAt:      s.clock.Now(),
	})
}

func (s *Service) publishConversation(ctx context.Context, conv *conventity.Conversation) {
	payload := convcontracts.NewConversationPayload(conv)
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID),
		convcontracts.RealtimeConversationUpdated, payload)
	if conv.SectorID != "" {
		_ = s.publisher.Publish(ctx, shared.TopicInbox(conv.TenantID, conv.SectorID),
			convcontracts.RealtimeConversationUpdated, payload)
	}
}

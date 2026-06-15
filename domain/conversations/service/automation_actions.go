package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// This file holds the automation-facing conversation mutations. They are
// tenant-scoped from ctx and do NOT enforce agent visibility — the caller is the
// trusted automation engine. Each emits its lifecycle event for the webhooks
// pipeline AND the rule sink; because the executor runs them under an
// origin=automation context, the rule event they emit is suppressed (anti-loop).
// Referential integrity is SOFT: a missing referenced entity returns a not_found
// error, which the executor logs as skipped_missing_ref (it never aborts the rule
// or the other actions).

// automationMutate loads the conversation, applies mutate (which returns the
// internal lifecycle event to record/emit), persists and fans the event out.
func (s *Service) automationMutate(ctx context.Context, conversationID string, mutate func(*entity.Conversation) (string, error)) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	conv, err := s.conversations.FindByID(ctx, conversationID)
	if err != nil {
		return err
	}
	event, err := mutate(conv)
	if err != nil {
		return err
	}
	if event == "" {
		return nil // no-op (e.g. open on an already-open conversation)
	}
	conv.UpdatedAt = s.clock.Now()
	if err := s.conversations.Update(ctx, conv); err != nil {
		return err
	}
	s.recordEvent(ctx, conv, event, nil)
	s.publishConversation(ctx, conv)
	s.emitConversationWebhook(ctx, conv, event)
	s.emitRuleEvent(ctx, conv, event, contracts.NewConversationPayload(conv))
	return nil
}

// AutomationAssignAgent assigns the conversation to an agent (must exist).
func (s *Service) AutomationAssignAgent(ctx context.Context, conversationID, agentID string) error {
	return s.automationMutate(ctx, conversationID, func(conv *entity.Conversation) (string, error) {
		if err := s.ensureAgentExists(ctx, agentID); err != nil {
			return "", err
		}
		conv.AssignedTo = agentID
		conv.Status = entity.StatusAssigned
		return entity.EventConversationAssigned, nil
	})
}

// AutomationAssignTeam puts the conversation into a sector ("team", must exist).
func (s *Service) AutomationAssignTeam(ctx context.Context, conversationID, sectorID string) error {
	return s.automationMutate(ctx, conversationID, func(conv *entity.Conversation) (string, error) {
		if _, err := s.sectors.FindByID(ctx, sectorID); err != nil {
			return "", missingRef(err, "sector")
		}
		conv.SectorID = sectorID
		if conv.AssignedTo == "" {
			conv.Status = entity.StatusQueued
		}
		return entity.EventConversationUpdated, nil
	})
}

// AutomationRemoveAgent clears the assignee.
func (s *Service) AutomationRemoveAgent(ctx context.Context, conversationID string) error {
	return s.automationMutate(ctx, conversationID, func(conv *entity.Conversation) (string, error) {
		conv.AssignedTo = ""
		if conv.SectorID != "" {
			conv.Status = entity.StatusQueued
		}
		return entity.EventConversationUpdated, nil
	})
}

// AutomationRemoveTeam clears the sector.
func (s *Service) AutomationRemoveTeam(ctx context.Context, conversationID string) error {
	return s.automationMutate(ctx, conversationID, func(conv *entity.Conversation) (string, error) {
		conv.SectorID = ""
		return entity.EventConversationUpdated, nil
	})
}

// AutomationAddTag adds a tag (resolved/validated to a canonical id).
func (s *Service) AutomationAddTag(ctx context.Context, conversationID, tagID string) error {
	return s.automationMutate(ctx, conversationID, func(conv *entity.Conversation) (string, error) {
		id := tagID
		if s.tags != nil {
			resolved, err := s.tags.ResolveTags(ctx, []string{tagID}, true)
			if err != nil || len(resolved) == 0 {
				return "", apperror.NotFound("tag not found")
			}
			id = resolved[0]
		}
		for _, t := range conv.Tags {
			if t == id {
				return "", nil // already tagged → no-op
			}
		}
		conv.Tags = append(conv.Tags, id)
		return entity.EventConversationUpdated, nil
	})
}

// AutomationRemoveTag strips a tag (lenient: a stale value is still removed).
func (s *Service) AutomationRemoveTag(ctx context.Context, conversationID, tagID string) error {
	return s.automationMutate(ctx, conversationID, func(conv *entity.Conversation) (string, error) {
		id := tagID
		if s.tags != nil {
			if resolved, err := s.tags.ResolveTags(ctx, []string{tagID}, false); err == nil && len(resolved) > 0 {
				id = resolved[0]
			}
		}
		out := conv.Tags[:0]
		removed := false
		for _, t := range conv.Tags {
			if t == id {
				removed = true
				continue
			}
			out = append(out, t)
		}
		conv.Tags = out
		if !removed {
			return "", nil
		}
		return entity.EventConversationUpdated, nil
	})
}

// AutomationChangePriority sets the conversation priority (validated enum).
func (s *Service) AutomationChangePriority(ctx context.Context, conversationID, priority string) error {
	return s.automationMutate(ctx, conversationID, func(conv *entity.Conversation) (string, error) {
		p := entity.Priority(priority)
		if !p.Valid() {
			return "", apperror.Validation("invalid priority")
		}
		conv.Priority = p
		return entity.EventConversationUpdated, nil
	})
}

// AutomationResolve marks the conversation resolved (no-op if already closed).
func (s *Service) AutomationResolve(ctx context.Context, conversationID string) error {
	return s.automationMutate(ctx, conversationID, func(conv *entity.Conversation) (string, error) {
		if conv.Status.IsClosed() {
			return "", nil
		}
		now := s.clock.Now()
		conv.Status = entity.StatusResolved
		conv.ClosedAt = &now
		return entity.EventConversationClosed, nil
	})
}

// AutomationOpen reopens a closed conversation (no-op if already open).
func (s *Service) AutomationOpen(ctx context.Context, conversationID string) error {
	return s.automationMutate(ctx, conversationID, func(conv *entity.Conversation) (string, error) {
		if !conv.Status.IsClosed() {
			return "", nil
		}
		conv.Status = entity.StatusNew
		if conv.AssignedTo != "" {
			conv.Status = entity.StatusAssigned
		} else if conv.SectorID != "" {
			conv.Status = entity.StatusQueued
		}
		conv.ClosedAt = nil
		return entity.EventConversationReopened, nil
	})
}

// AutomationMarkPending sends the conversation back to the queue, waiting to be
// picked up (the Chatwoot "pending" — here status=queued, never a new status).
func (s *Service) AutomationMarkPending(ctx context.Context, conversationID string) error {
	return s.automationMutate(ctx, conversationID, func(conv *entity.Conversation) (string, error) {
		if conv.Status == entity.StatusQueued {
			return "", nil
		}
		conv.Status = entity.StatusQueued
		return entity.EventConversationUpdated, nil
	})
}

// SendAutomationAttachment injects an outbound automation message carrying an
// attachment (must exist/ready), reusing the normal send pipeline.
func (s *Service) SendAutomationAttachment(ctx context.Context, conversationID, ruleID, attachmentID string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(attachmentID) == "" {
		return apperror.Validation("attachment_id is required")
	}
	conv, err := s.conversations.FindByID(ctx, conversationID)
	if err != nil {
		return err
	}
	if s.attachments != nil {
		resolved, herr := s.attachments.HydrateAttachments(ctx, []string{attachmentID})
		if herr != nil || len(resolved) == 0 {
			return apperror.NotFound("attachment not found")
		}
	}
	now := s.clock.Now()
	msg := &entity.Message{
		ID:             shared.NewID(),
		TenantID:       conv.TenantID,
		ConversationID: conv.ID,
		SenderType:     entity.SenderAutomation,
		SenderID:       ruleID,
		Direction:      entity.DirectionOutbound,
		MessageType:    entity.MessageFile,
		Attachments:    []entity.Attachment{{ID: attachmentID}},
		CreatedAt:      now,
		DeliveryStatus: entity.DeliveryPending,
	}
	if _, err := s.persistMessage(ctx, conv, msg, entity.EventMessageCreated); err != nil {
		return err
	}
	return nil
}

// SendAutomationInteractive injects an outbound interactive menu authored by an
// automation rule. The menu is passed as a JSON object (the conversations
// Interactive shape) so it fits the flat action-param model; it is parsed and
// validated against the same WhatsApp limits as a normal interactive send.
func (s *Service) SendAutomationInteractive(ctx context.Context, conversationID, ruleID, interactiveJSON string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(interactiveJSON) == "" {
		return apperror.Validation("interactive is required")
	}
	var iv entity.Interactive
	if err := json.Unmarshal([]byte(interactiveJSON), &iv); err != nil {
		return apperror.Validation("interactive must be a valid JSON object")
	}
	if msg := iv.Validate(); msg != "" {
		return apperror.Validation(msg).WithDetails(map[string]any{"interactive": msg})
	}
	conv, err := s.conversations.FindByID(ctx, conversationID)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	msg := &entity.Message{
		ID:             shared.NewID(),
		TenantID:       conv.TenantID,
		ConversationID: conv.ID,
		SenderType:     entity.SenderAutomation,
		SenderID:       ruleID,
		Direction:      entity.DirectionOutbound,
		MessageType:    entity.MessageInteractive,
		Text:           iv.Body, // mirror the body for inbox preview / search
		Interactive:    &iv,
		CreatedAt:      now,
		DeliveryStatus: entity.DeliveryPending,
	}
	_, err = s.persistMessage(ctx, conv, msg, entity.EventMessageCreated)
	return err
}

// ensureAgentExists returns not_found when the agent id is unknown (soft ref). It
// is a no-op when no agent directory is wired.
func (s *Service) ensureAgentExists(ctx context.Context, agentID string) error {
	if s.agents == nil {
		return nil
	}
	cards, err := s.agents.AgentCards(ctx, []string{agentID})
	if err != nil {
		return err
	}
	if _, ok := cards[agentID]; !ok {
		return apperror.NotFound("agent not found")
	}
	return nil
}

// missingRef maps a not_found error to a not_found "soft ref" error and passes any
// other error through unchanged.
func missingRef(err error, what string) error {
	if apperror.From(err).Code == apperror.CodeNotFound {
		return apperror.NotFound(what + " not found")
	}
	return err
}

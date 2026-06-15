package service

import (
	"context"

	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// transcriptLimit bounds how many recent messages are included in the context.
const transcriptLimit = 30

// ContextBuilder assembles the policy-filtered PromptContext for a conversation.
// The only pre-injected data section is the customer profile, gated by
// allow_customer_data; financial/monitoring data are consulted on demand via ISP
// tools, never pre-injected here.
type ContextBuilder struct {
	messages convrepo.MessageRepository
	customer contracts.CustomerDataSource
}

// NewContextBuilder builds the builder. The customer source is optional (nil means
// the enrichment is unavailable).
func NewContextBuilder(messages convrepo.MessageRepository, customer contracts.CustomerDataSource) *ContextBuilder {
	return &ContextBuilder{messages: messages, customer: customer}
}

// Build assembles the context, enforcing the assistant's customer-data gate (from
// the resolved Behavior).
func (b *ContextBuilder) Build(ctx context.Context, beh entity.Behavior, conv *conventity.Conversation, instruction string) contracts.PromptContext {
	pc := contracts.PromptContext{
		Channel:     conv.Channel,
		Instruction: instruction,
		Transcript:  b.transcript(ctx, conv.ID),
	}

	// Customer profile — only when the gate allows AND a source is wired.
	if beh.AllowCustomerData && b.customer != nil {
		if info, err := b.customer.Customer(ctx, conv.ContactID); err == nil {
			pc.Customer = info
		}
	}
	return pc
}

// transcript loads the recent messages newest-first and returns them in
// chronological order as provider turns. Internal notes are excluded.
func (b *ContextBuilder) transcript(ctx context.Context, conversationID string) []contracts.Turn {
	msgs, err := b.messages.ListByConversation(ctx, conversationID, shared.PageRequest{Limit: transcriptLimit})
	if err != nil || len(msgs) == 0 {
		return nil
	}
	turns := make([]contracts.Turn, 0, len(msgs))
	// ListByConversation returns newest-first; reverse to chronological.
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Direction == conventity.DirectionInternal {
			continue // internal notes are not part of the customer conversation
		}
		turns = append(turns, contracts.Turn{Role: role(m), Text: m.Text})
	}
	return turns
}

func role(m *conventity.Message) string {
	switch m.Direction {
	case conventity.DirectionInbound:
		return "customer"
	case conventity.DirectionOutbound:
		return "agent"
	default:
		return "system"
	}
}

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
// It is the single place the allow_*_data switches are enforced: a data section
// is fetched only when its policy permits, so disallowed data never reaches a
// provider.
type ContextBuilder struct {
	messages   convrepo.MessageRepository
	customer   contracts.CustomerDataSource
	financial  contracts.FinancialDataSource
	monitoring contracts.MonitoringDataSource
}

// NewContextBuilder builds the builder. The data sources are optional (nil means
// the enrichment is unavailable).
func NewContextBuilder(
	messages convrepo.MessageRepository,
	customer contracts.CustomerDataSource,
	financial contracts.FinancialDataSource,
	monitoring contracts.MonitoringDataSource,
) *ContextBuilder {
	return &ContextBuilder{messages: messages, customer: customer, financial: financial, monitoring: monitoring}
}

// Build assembles the context, enforcing the tenant's privacy policies.
func (b *ContextBuilder) Build(ctx context.Context, cfg *entity.AIConfig, conv *conventity.Conversation, instruction string) contracts.PromptContext {
	pc := contracts.PromptContext{
		Channel:     conv.Channel,
		Instruction: instruction,
		Transcript:  b.transcript(ctx, conv.ID),
	}

	// Customer profile — only when the policy allows AND a source is wired.
	if cfg.AllowCustomerData && b.customer != nil {
		if info, err := b.customer.Customer(ctx, conv.ContactID); err == nil {
			pc.Customer = info
		}
	}
	// Financial data — only when the policy allows.
	if cfg.AllowFinancialData && b.financial != nil {
		if info, err := b.financial.Financial(ctx, conv.ContactID); err == nil {
			pc.Financial = info
		}
	}
	// Monitoring/technical status — only when the policy allows.
	if cfg.AllowMonitoringData && b.monitoring != nil {
		if info, err := b.monitoring.Monitoring(ctx, conv.ID); err == nil {
			pc.Monitoring = info
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

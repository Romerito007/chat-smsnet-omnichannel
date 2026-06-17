package factories

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	contactservice "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/service"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
)

// webhookEnricher resolves the outbound-webhook contact + agent blocks over the
// contacts and iam user services. It is invoked LAZILY by the dispatcher (only
// after a matching subscription is confirmed), so the lookups below never run for a
// tenant without a webhook subscribed to the event. Every method is best-effort:
// a failure returns nil so the block is omitted and delivery is never broken.
type webhookEnricher struct {
	contacts *contactservice.Service
	agents   *iamservice.UserService
}

// WebhookContact resolves the recipient block (id, name, phone, channel identities
// and custom_attributes) for a contact id — one contact lookup.
func (e webhookEnricher) WebhookContact(ctx context.Context, contactID string) *convcontracts.WebhookContact {
	c, err := e.contacts.Get(ctx, contactID)
	if err != nil || c == nil {
		return nil
	}
	return &convcontracts.WebhookContact{
		ID:               c.ID,
		Name:             c.Name,
		Phone:            c.Phone,
		IsGroup:          c.IsGroup(),
		Identities:       webhookIdentities(c.Identities),
		CustomAttributes: c.CustomAttributes,
	}
}

// WebhookAgent resolves an agent's id+name block (no PII) — at most one user lookup
// via the existing AgentCards path. Returns nil when the user can't be resolved.
func (e webhookEnricher) WebhookAgent(ctx context.Context, userID string) *convcontracts.WebhookAgent {
	cards, err := e.agents.AgentCards(ctx, []string{userID})
	if err != nil {
		return nil
	}
	card, ok := cards[userID]
	if !ok {
		return nil
	}
	return &convcontracts.WebhookAgent{ID: userID, Name: card.Name}
}

// webhookIdentities maps the contact's channel identities (the routing keys the
// gateway dials, e.g. the WhatsApp JID) to the webhook shape.
func webhookIdentities(in []contactentity.ChannelIdentity) []convcontracts.WebhookIdentity {
	if len(in) == 0 {
		return nil
	}
	out := make([]convcontracts.WebhookIdentity, len(in))
	for i, id := range in {
		out[i] = convcontracts.WebhookIdentity{Channel: id.Channel, ExternalID: id.ExternalID}
	}
	return out
}

// WebhookEnricher builds the contact/agent enricher wired into the conversations,
// routing and inbound services.
func WebhookEnricher(c *container.Container) convcontracts.WebhookEnricher {
	return webhookEnricher{contacts: ContactService(c), agents: UserService(c)}
}

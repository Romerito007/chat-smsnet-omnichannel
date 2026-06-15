package contracts

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// Realtime event names emitted by the conversations service.
const (
	RealtimeMessageCreated          = "message.created"
	RealtimeMessageUpdated          = "message.updated"
	RealtimeMessageDeleted          = "message.deleted"
	RealtimeConversationCreated     = "conversation.created"
	RealtimeConversationUpdated     = "conversation.updated"
	RealtimeConversationClosed      = "conversation.closed"
	RealtimeConversationResolved    = "conversation.resolved"
	RealtimeConversationReopened    = "conversation.reopened"
	RealtimeConversationAssigned    = "conversation.assigned"
	RealtimeConversationTransferred = "conversation.transferred"
	RealtimeConversationTagged      = "conversation.tagged"
	RealtimeTypingStarted           = "typing.started"
	RealtimeTypingStopped           = "typing.stopped"
	RealtimeMessageRead             = "message.read"
	RealtimeMessageSent             = "message.sent"
	RealtimeMessageDelivered        = "message.delivered"
	RealtimeMessageFailed           = "message.failed"
)

// MessageStatusPayload is the payload for message.sent/delivered/read/failed
// delivery-status events.
type MessageStatusPayload struct {
	MessageID      string `json:"message_id"`
	ConversationID string `json:"conversation_id"`
	DeliveryStatus string `json:"delivery_status"`
	Error          string `json:"error,omitempty"`
}

// TypingPayload is the payload of typing.started/stopped events.
type TypingPayload struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
}

// ReadPayload is the payload of the message.read event.
type ReadPayload struct {
	ConversationID string    `json:"conversation_id"`
	UserID         string    `json:"user_id"`
	ReadAt         time.Time `json:"read_at"`
}

// ConversationPayload is the realtime/event representation of a conversation.
type ConversationPayload struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	ContactID     string    `json:"contact_id"`
	Channel       string    `json:"channel"`
	ChannelID     string    `json:"channel_id,omitempty"`
	SectorID      string    `json:"sector_id,omitempty"`
	QueueID       string    `json:"queue_id,omitempty"`
	Status        string    `json:"status"`
	AssignedTo    string    `json:"assigned_to,omitempty"`
	Priority      string    `json:"priority"`
	Protocol      string    `json:"protocol,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	LastMessageAt time.Time `json:"last_message_at"`
	// LastMessage mirrors the denormalized inbox preview so a conversation.updated
	// event refreshes the row's snippet live (no refetch). Nil when no message yet.
	LastMessage *LastMessagePayload `json:"last_message,omitempty"`
	UnreadCount int                 `json:"unread_count"`
	LastReadAt  *time.Time          `json:"last_read_at,omitempty"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// LastMessagePayload is the realtime shape of the conversation's last-message
// preview (matches the inbox REST DTO).
type LastMessagePayload struct {
	Preview     string    `json:"preview"`
	SenderType  string    `json:"sender_type"`
	MessageType string    `json:"message_type"`
	CreatedAt   time.Time `json:"created_at"`
}

// NewConversationPayload builds the payload from a conversation entity.
func NewConversationPayload(c *entity.Conversation) ConversationPayload {
	return ConversationPayload{
		ID:            c.ID,
		TenantID:      c.TenantID,
		ContactID:     c.ContactID,
		Channel:       c.Channel,
		ChannelID:     c.ChannelID,
		SectorID:      c.SectorID,
		QueueID:       c.QueueID,
		Status:        string(c.Status),
		AssignedTo:    c.AssignedTo,
		Priority:      string(c.Priority),
		Protocol:      c.Protocol,
		Tags:          c.Tags,
		LastMessageAt: c.LastMessageAt,
		LastMessage:   newLastMessagePayload(c.LastMessage),
		UnreadCount:   c.UnreadCount,
		LastReadAt:    c.LastReadAt,
		UpdatedAt:     c.UpdatedAt,
	}
}

// newLastMessagePayload maps the denormalized snapshot to the realtime shape.
func newLastMessagePayload(s *entity.LastMessageSnapshot) *LastMessagePayload {
	if s == nil {
		return nil
	}
	return &LastMessagePayload{
		Preview:     s.Preview,
		SenderType:  string(s.SenderType),
		MessageType: string(s.MessageType),
		CreatedAt:   s.CreatedAt,
	}
}

// MessagePayload is the realtime/event representation of a message. Attachments
// carry the full hydrated media metadata (url/content_type/filename/size), the
// same shape as the GET .../messages response.
type MessagePayload struct {
	ID             string              `json:"id"`
	ConversationID string              `json:"conversation_id"`
	SenderType     string              `json:"sender_type"`
	SenderID       string              `json:"sender_id,omitempty"`
	Direction      string              `json:"direction"`
	MessageType    string              `json:"message_type"`
	Text           string              `json:"text"`
	Attachments    []entity.Attachment `json:"attachments,omitempty"`
	// Contacts (message_type=contact) and Location (message_type=location) are the
	// typed structured payloads, mirroring the WhatsApp contacts[]/location blocks.
	Contacts []entity.ContactCard `json:"contacts,omitempty"`
	Location *entity.Location     `json:"location,omitempty"`
	// Template carries the integrator template id + filled params for a template
	// message, so an outbound-webhook receiver can render/send it. Nil otherwise.
	Template       *MessageTemplatePayload `json:"template,omitempty"`
	Internal       bool                    `json:"internal"`
	DeliveryStatus string                  `json:"delivery_status,omitempty"`
	CreatedAt      time.Time               `json:"created_at"`
	EditedAt       *time.Time              `json:"edited_at,omitempty"`
	// Contact/Agent/Conversation are the OUTBOUND-webhook enrichment blocks, set
	// only by the integration builder (see emitMessageWebhook). They stay nil — and
	// thus absent — on the lean realtime/rule payloads. Agent is present only for an
	// agent-authored message.
	Contact      *WebhookContact         `json:"contact,omitempty"`
	Agent        *WebhookAgent           `json:"agent,omitempty"`
	Conversation *WebhookConversationRef `json:"conversation,omitempty"`
}

// MessageTemplatePayload is the template section of a message payload: the opaque
// integrator template id and the filled named params (no resolved text/structure).
type MessageTemplatePayload struct {
	ID     string            `json:"id"`
	Params map[string]string `json:"params,omitempty"`
}

// NewMessagePayload builds the payload from a message entity.
func NewMessagePayload(m *entity.Message) MessagePayload {
	p := MessagePayload{
		ID:             m.ID,
		ConversationID: m.ConversationID,
		SenderType:     string(m.SenderType),
		SenderID:       m.SenderID,
		Direction:      string(m.Direction),
		MessageType:    string(m.MessageType),
		Text:           m.Text,
		Attachments:    m.Attachments,
		Contacts:       m.Contacts,
		Location:       m.Location,
		Internal:       m.Direction == entity.DirectionInternal,
		DeliveryStatus: string(m.DeliveryStatus),
		CreatedAt:      m.CreatedAt,
		EditedAt:       m.EditedAt,
	}
	if m.Template != nil {
		p.Template = &MessageTemplatePayload{ID: m.Template.TemplateID, Params: m.Template.Params}
	}
	return p
}

// NewIntegrationMessagePayload builds the message payload destined for an OUTBOUND
// webhook: a copy of NewMessagePayload with each attachment URL swapped for its
// signed, public channel-media URL (so the integrator can fetch the media without
// a JWT). mediaURLs is keyed by attachment id; an id absent from the map keeps its
// original URL. The template section (id + params) is included as usual.
func NewIntegrationMessagePayload(m *entity.Message, mediaURLs map[string]string) MessagePayload {
	p := NewMessagePayload(m)
	if len(p.Attachments) == 0 || len(mediaURLs) == 0 {
		return p
	}
	out := make([]entity.Attachment, len(p.Attachments))
	for i, a := range p.Attachments {
		if u, ok := mediaURLs[a.ID]; ok && u != "" {
			a.URL = u
		}
		out[i] = a
	}
	p.Attachments = out
	return p
}

// MessageRefPayload is the minimal reference broadcast for a message.deleted
// event — it carries no body, since a deleted message is hidden from listings.
type MessageRefPayload struct {
	MessageID      string `json:"message_id"`
	ConversationID string `json:"conversation_id"`
}

// ── outbound-webhook enrichment ───────────────────────────────────────────────
//
// These blocks are attached ONLY to the integration (outbound-webhook) payload
// variants, never to the realtime/UI or automation-rule payloads. They give the
// channel gateway everything it needs to ROUTE a message without a second call:
// the recipient (contact + its channel identities — e.g. the WhatsApp JID) and,
// for an agent-authored message, who sent it (id + name only, no PII).

// WebhookIdentity is a contact's external identifier on a channel — the routing
// key the gateway dials (e.g. {channel:"whatsapp", external_id:"<JID>"}).
type WebhookIdentity struct {
	Channel    string `json:"channel"`
	ExternalID string `json:"external_id"`
}

// WebhookContact is the recipient block: the contact's id, display name, primary
// phone, channel identities (the routing keys) and tenant custom_attributes. PII
// beyond name/phone is intentionally excluded.
type WebhookContact struct {
	ID               string            `json:"id"`
	Name             string            `json:"name,omitempty"`
	Phone            string            `json:"phone,omitempty"`
	Identities       []WebhookIdentity `json:"identities,omitempty"`
	CustomAttributes map[string]any    `json:"custom_attributes,omitempty"`
}

// WebhookAgent is the sender block for an agent-authored message: id + name only,
// deliberately without email or any other PII.
type WebhookAgent struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// WebhookConversationRef is the lightweight conversation block embedded in a
// message webhook so the integrator can read the conversation's custom_attributes
// without a second call.
type WebhookConversationRef struct {
	CustomAttributes map[string]any `json:"custom_attributes,omitempty"`
}

// WebhookEnricher resolves the contact + agent blocks for an outbound-webhook
// payload. It is invoked LAZILY — only after the dispatcher has confirmed at least
// one subscription exists for the event — so a tenant with no webhook pays zero
// contact/agent lookups on the hot inbound path. Every method is best-effort: a
// nil return omits the block and never breaks delivery. Implemented by an adapter
// over the contacts + iam user services.
type WebhookEnricher interface {
	// WebhookContact resolves the recipient block for a contact id (1 lookup).
	WebhookContact(ctx context.Context, contactID string) *WebhookContact
	// WebhookAgent resolves an agent's id+name block for a user id (0-1 lookup).
	WebhookAgent(ctx context.Context, userID string) *WebhookAgent
}

// IntegrationConversationPayload is the conversation representation destined for an
// OUTBOUND webhook: the lean ConversationPayload plus the enrichment blocks
// (custom_attributes, the recipient contact and the assigned agent). assigned_agent
// is null when the conversation is unassigned (or for inbound, where agents aren't
// resolved).
type IntegrationConversationPayload struct {
	ConversationPayload
	CustomAttributes map[string]any  `json:"custom_attributes,omitempty"`
	Contact          *WebhookContact `json:"contact,omitempty"`
	AssignedAgent    *WebhookAgent   `json:"assigned_agent,omitempty"`
}

// NewIntegrationConversationPayload builds the enriched conversation webhook
// payload. contact/agent are pre-resolved by the caller (lazily) and may be nil.
func NewIntegrationConversationPayload(c *entity.Conversation, contact *WebhookContact, agent *WebhookAgent) IntegrationConversationPayload {
	return IntegrationConversationPayload{
		ConversationPayload: NewConversationPayload(c),
		CustomAttributes:    c.CustomAttributes,
		Contact:             contact,
		AssignedAgent:       agent,
	}
}

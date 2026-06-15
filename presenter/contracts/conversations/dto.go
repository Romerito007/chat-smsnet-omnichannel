// Package conversations holds the request/response DTOs for the conversations
// endpoints.
package conversations

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── requests ─────────────────────────────────────────────────────────────────

// CreateConversationRequest is the body of POST /v1/conversations. channel_id is
// required; the channel TYPE is derived from that connection (the client no longer
// sends a channel type).
type CreateConversationRequest struct {
	ContactID  string   `json:"contact_id"`
	ChannelID  string   `json:"channel_id"`
	SectorID   string   `json:"sector_id"`
	QueueID    string   `json:"queue_id"`
	AssignedTo string   `json:"assigned_to"`
	Priority   string   `json:"priority"`
	Tags       []string `json:"tags"`
}

// ToCommand maps the request to the service command.
func (r CreateConversationRequest) ToCommand() contracts.CreateConversation {
	return contracts.CreateConversation{
		ContactID:  r.ContactID,
		ChannelID:  r.ChannelID,
		SectorID:   r.SectorID,
		QueueID:    r.QueueID,
		AssignedTo: r.AssignedTo,
		Priority:   entity.Priority(r.Priority),
		Tags:       r.Tags,
	}
}

// UpdateConversationRequest is the body of PATCH /v1/conversations/{id}.
type UpdateConversationRequest struct {
	SectorID         *string         `json:"sector_id"`
	QueueID          *string         `json:"queue_id"`
	Status           *string         `json:"status"`
	AssignedTo       *string         `json:"assigned_to"`
	Priority         *string         `json:"priority"`
	Tags             *[]string       `json:"tags"`
	CustomAttributes *map[string]any `json:"custom_attributes"`
}

// ToCommand maps the request to the service command.
func (r UpdateConversationRequest) ToCommand() contracts.UpdateConversation {
	cmd := contracts.UpdateConversation{
		SectorID:         r.SectorID,
		QueueID:          r.QueueID,
		AssignedTo:       r.AssignedTo,
		Tags:             r.Tags,
		CustomAttributes: r.CustomAttributes,
	}
	if r.Status != nil {
		st := entity.Status(*r.Status)
		cmd.Status = &st
	}
	if r.Priority != nil {
		p := entity.Priority(*r.Priority)
		cmd.Priority = &p
	}
	return cmd
}

// AttachmentRequest mirrors entity.Attachment on the wire.
type AttachmentRequest struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
}

// TemplateRequest selects a WhatsApp template + its named params on a template send.
type TemplateRequest struct {
	TemplateID string            `json:"template_id"`
	Params     map[string]string `json:"params"`
}

// SendMessageRequest is the body of POST /v1/conversations/{id}/messages.
type SendMessageRequest struct {
	MessageType string              `json:"message_type"`
	Text        string              `json:"text"`
	Attachments []AttachmentRequest `json:"attachments"`
	Template    *TemplateRequest    `json:"template"`
	Metadata    map[string]any      `json:"metadata"`
}

// ToCommand maps the request to the service command.
func (r SendMessageRequest) ToCommand() contracts.SendMessage {
	atts := make([]entity.Attachment, len(r.Attachments))
	for i, a := range r.Attachments {
		atts[i] = entity.Attachment{ID: a.ID, URL: a.URL, ContentType: a.ContentType, Filename: a.Filename, Size: a.Size}
	}
	cmd := contracts.SendMessage{
		MessageType: entity.MessageType(r.MessageType),
		Text:        r.Text,
		Attachments: atts,
		Metadata:    r.Metadata,
	}
	if r.Template != nil {
		cmd.Template = &contracts.SendTemplate{TemplateID: r.Template.TemplateID, Params: r.Template.Params}
	}
	return cmd
}

// EditMessageRequest is the body of PATCH /v1/conversations/{id}/messages/{mid}.
type EditMessageRequest struct {
	Text string `json:"text"`
}

// ToCommand maps the request to the service command.
func (r EditMessageRequest) ToCommand() contracts.EditMessage {
	return contracts.EditMessage{Text: r.Text}
}

// InternalNoteRequest is the body of POST /v1/conversations/{id}/internal-notes.
type InternalNoteRequest struct {
	Text           string   `json:"text"`
	MentionUserIDs []string `json:"mention_user_ids"`
}

// CloseRequest is the body of POST /v1/conversations/{id}/close.
type CloseRequest struct {
	CloseReasonID string `json:"close_reason_id"`
	Note          string `json:"note"`
}

// ── responses ────────────────────────────────────────────────────────────────

// ConversationResponse is the public representation of a conversation.
type ConversationResponse struct {
	ID               string         `json:"id"`
	TenantID         string         `json:"tenant_id"`
	ContactID        string         `json:"contact_id"`
	Channel          string         `json:"channel"`
	ChannelID        string         `json:"channel_id,omitempty"`
	SectorID         string         `json:"sector_id,omitempty"`
	QueueID          string         `json:"queue_id,omitempty"`
	Status           string         `json:"status"`
	AssignedTo       string         `json:"assigned_to,omitempty"`
	Priority         string         `json:"priority"`
	Protocol         string         `json:"protocol,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	CustomAttributes map[string]any `json:"custom_attributes,omitempty"`
	LastMessageAt    time.Time      `json:"last_message_at"`
	UnreadCount      int            `json:"unread_count"`
	LastReadAt       *time.Time     `json:"last_read_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	ClosedAt         *time.Time     `json:"closed_at,omitempty"`
	LastMessage      *LastMessage   `json:"last_message,omitempty"`
	// ContactName / ContactAvatarURL / AgentName / AgentAvatarURL are read-only,
	// derived display fields, resolved in batch so the inbox renders each row
	// (contact + assignee) without a per-row fetch. Avatar URLs are short-lived
	// signed channel-media URLs (no JWT). Empty when the related entity or its
	// ready avatar is absent.
	ContactName      string `json:"contact_name,omitempty"`
	ContactAvatarURL string `json:"contact_avatar_url,omitempty"`
	AgentName        string `json:"agent_name,omitempty"`
	AgentAvatarURL   string `json:"agent_avatar_url,omitempty"`
}

// LastMessage is a light preview of a conversation's most recent message, used on
// list items so the inbox can render a snippet without a per-row fetch.
type LastMessage struct {
	Preview     string    `json:"preview"`
	SenderType  string    `json:"sender_type"`
	MessageType string    `json:"message_type"`
	CreatedAt   time.Time `json:"created_at"`
}

func newLastMessage(s *entity.LastMessageSnapshot) *LastMessage {
	if s == nil {
		return nil
	}
	return &LastMessage{
		Preview:     s.Preview,
		SenderType:  string(s.SenderType),
		MessageType: string(s.MessageType),
		CreatedAt:   s.CreatedAt,
	}
}

// NewConversationResponsesWithLastMessage maps a page of conversations, attaching
// each one's last-message preview from the denormalized snapshot on the row (no
// aggregation).
func NewConversationResponsesWithLastMessage(items []*entity.Conversation, contactCards, agentCards map[string]shared.DisplayCard) []ConversationResponse {
	out := make([]ConversationResponse, len(items))
	for i, c := range items {
		r := NewConversationResponse(c)
		r.LastMessage = newLastMessage(c.LastMessage)
		applyCards(&r, c, contactCards, agentCards)
		out[i] = r
	}
	return out
}

// applyCards fills the read-only display fields (contact/agent name + avatar)
// from the resolved cards. Empty when the related entity or its avatar is absent.
func applyCards(r *ConversationResponse, c *entity.Conversation, contactCards, agentCards map[string]shared.DisplayCard) {
	if c.ContactID != "" {
		if card, ok := contactCards[c.ContactID]; ok {
			r.ContactName = card.Name
			r.ContactAvatarURL = card.AvatarURL
		}
	}
	if c.AssignedTo != "" {
		if card, ok := agentCards[c.AssignedTo]; ok {
			r.AgentName = card.Name
			r.AgentAvatarURL = card.AvatarURL
		}
	}
}

// NewConversationResponse maps a conversation entity to its DTO.
func NewConversationResponse(c *entity.Conversation) ConversationResponse {
	return ConversationResponse{
		ID:               c.ID,
		TenantID:         c.TenantID,
		ContactID:        c.ContactID,
		Channel:          c.Channel,
		ChannelID:        c.ChannelID,
		SectorID:         c.SectorID,
		QueueID:          c.QueueID,
		Status:           string(c.Status),
		AssignedTo:       c.AssignedTo,
		Priority:         string(c.Priority),
		Protocol:         c.Protocol,
		Tags:             c.Tags,
		CustomAttributes: c.CustomAttributes,
		LastMessageAt:    c.LastMessageAt,
		LastMessage:      newLastMessage(c.LastMessage),
		UnreadCount:      c.UnreadCount,
		LastReadAt:       c.LastReadAt,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
		ClosedAt:         c.ClosedAt,
	}
}

// NewConversationResponseWithCards maps one conversation, attaching the resolved
// contact/agent display cards, so the detail/create responses stay consistent
// with the list. Empty fields when the related entity or its avatar is absent.
func NewConversationResponseWithCards(c *entity.Conversation, contactCards, agentCards map[string]shared.DisplayCard) ConversationResponse {
	r := NewConversationResponse(c)
	applyCards(&r, c, contactCards, agentCards)
	return r
}

// NewConversationResponses maps a slice.
func NewConversationResponses(items []*entity.Conversation) []ConversationResponse {
	out := make([]ConversationResponse, len(items))
	for i, c := range items {
		out[i] = NewConversationResponse(c)
	}
	return out
}

// EventResponse is a conversation timeline event (lifecycle/automation), stored
// separately from chat messages.
type EventResponse struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversation_id"`
	Type           string         `json:"type"`
	ActorType      string         `json:"actor_type,omitempty"`
	ActorID        string         `json:"actor_id,omitempty"`
	Data           map[string]any `json:"data,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// NewEventResponse maps a conversation event entity.
func NewEventResponse(e *entity.ConversationEvent) EventResponse {
	return EventResponse{
		ID:             e.ID,
		ConversationID: e.ConversationID,
		Type:           e.Type,
		ActorType:      string(e.ActorType),
		ActorID:        e.ActorID,
		Data:           e.Data,
		CreatedAt:      e.CreatedAt,
	}
}

// NewEventResponses maps a slice.
func NewEventResponses(items []*entity.ConversationEvent) []EventResponse {
	out := make([]EventResponse, len(items))
	for i, e := range items {
		out[i] = NewEventResponse(e)
	}
	return out
}

// MessageResponse is the public representation of a message.
type MessageResponse struct {
	ID                string              `json:"id"`
	ConversationID    string              `json:"conversation_id"`
	SenderType        string              `json:"sender_type"`
	SenderID          string              `json:"sender_id,omitempty"`
	Direction         string              `json:"direction"`
	MessageType       string              `json:"message_type"`
	Text              string              `json:"text"`
	Attachments       []AttachmentRequest `json:"attachments,omitempty"`
	Template          *TemplateRequest    `json:"template,omitempty"`
	Metadata          map[string]any      `json:"metadata,omitempty"`
	DeliveryStatus    string              `json:"delivery_status,omitempty"`
	ExternalMessageID string              `json:"external_message_id,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
	EditedAt          *time.Time          `json:"edited_at,omitempty"`
}

// NewMessageResponse maps a message entity to its DTO.
func NewMessageResponse(m *entity.Message) MessageResponse {
	atts := make([]AttachmentRequest, len(m.Attachments))
	for i, a := range m.Attachments {
		atts[i] = AttachmentRequest{ID: a.ID, URL: a.URL, ContentType: a.ContentType, Filename: a.Filename, Size: a.Size}
	}
	resp := MessageResponse{
		ID:                m.ID,
		ConversationID:    m.ConversationID,
		SenderType:        string(m.SenderType),
		SenderID:          m.SenderID,
		Direction:         string(m.Direction),
		MessageType:       string(m.MessageType),
		Text:              m.Text,
		Attachments:       atts,
		Metadata:          m.Metadata,
		DeliveryStatus:    string(m.DeliveryStatus),
		ExternalMessageID: m.ExternalMessageID,
		CreatedAt:         m.CreatedAt,
		EditedAt:          m.EditedAt,
	}
	if m.Template != nil {
		resp.Template = &TemplateRequest{TemplateID: m.Template.TemplateID, Params: m.Template.Params}
	}
	return resp
}

// NewMessageResponses maps a slice.
func NewMessageResponses(items []*entity.Message) []MessageResponse {
	out := make([]MessageResponse, len(items))
	for i, m := range items {
		out[i] = NewMessageResponse(m)
	}
	return out
}

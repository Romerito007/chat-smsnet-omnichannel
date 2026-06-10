// Package conversations holds the request/response DTOs for the conversations
// endpoints.
package conversations

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// ── requests ─────────────────────────────────────────────────────────────────

// CreateConversationRequest is the body of POST /v1/conversations.
type CreateConversationRequest struct {
	ContactID  string   `json:"contact_id"`
	Channel    string   `json:"channel"`
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
		Channel:    r.Channel,
		SectorID:   r.SectorID,
		QueueID:    r.QueueID,
		AssignedTo: r.AssignedTo,
		Priority:   entity.Priority(r.Priority),
		Tags:       r.Tags,
	}
}

// UpdateConversationRequest is the body of PATCH /v1/conversations/{id}.
type UpdateConversationRequest struct {
	SectorID   *string   `json:"sector_id"`
	QueueID    *string   `json:"queue_id"`
	Status     *string   `json:"status"`
	AssignedTo *string   `json:"assigned_to"`
	Priority   *string   `json:"priority"`
	Tags       *[]string `json:"tags"`
}

// ToCommand maps the request to the service command.
func (r UpdateConversationRequest) ToCommand() contracts.UpdateConversation {
	cmd := contracts.UpdateConversation{
		SectorID:   r.SectorID,
		QueueID:    r.QueueID,
		AssignedTo: r.AssignedTo,
		Tags:       r.Tags,
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

// SendMessageRequest is the body of POST /v1/conversations/{id}/messages.
type SendMessageRequest struct {
	MessageType string              `json:"message_type"`
	Text        string              `json:"text"`
	Attachments []AttachmentRequest `json:"attachments"`
	Metadata    map[string]any      `json:"metadata"`
}

// ToCommand maps the request to the service command.
func (r SendMessageRequest) ToCommand() contracts.SendMessage {
	atts := make([]entity.Attachment, len(r.Attachments))
	for i, a := range r.Attachments {
		atts[i] = entity.Attachment{ID: a.ID, URL: a.URL, ContentType: a.ContentType, Filename: a.Filename, Size: a.Size}
	}
	return contracts.SendMessage{
		MessageType: entity.MessageType(r.MessageType),
		Text:        r.Text,
		Attachments: atts,
		Metadata:    r.Metadata,
	}
}

// InternalNoteRequest is the body of POST /v1/conversations/{id}/internal-notes.
type InternalNoteRequest struct {
	Text string `json:"text"`
}

// CloseRequest is the body of POST /v1/conversations/{id}/close.
type CloseRequest struct {
	CloseReasonID string `json:"close_reason_id"`
	Note          string `json:"note"`
}

// ── responses ────────────────────────────────────────────────────────────────

// ConversationResponse is the public representation of a conversation.
type ConversationResponse struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenant_id"`
	ContactID     string     `json:"contact_id"`
	Channel       string     `json:"channel"`
	SectorID      string     `json:"sector_id,omitempty"`
	QueueID       string     `json:"queue_id,omitempty"`
	Status        string     `json:"status"`
	AssignedTo    string     `json:"assigned_to,omitempty"`
	Priority      string     `json:"priority"`
	Tags          []string   `json:"tags,omitempty"`
	LastMessageAt time.Time  `json:"last_message_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
}

// NewConversationResponse maps a conversation entity to its DTO.
func NewConversationResponse(c *entity.Conversation) ConversationResponse {
	return ConversationResponse{
		ID:            c.ID,
		TenantID:      c.TenantID,
		ContactID:     c.ContactID,
		Channel:       c.Channel,
		SectorID:      c.SectorID,
		QueueID:       c.QueueID,
		Status:        string(c.Status),
		AssignedTo:    c.AssignedTo,
		Priority:      string(c.Priority),
		Tags:          c.Tags,
		LastMessageAt: c.LastMessageAt,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
		ClosedAt:      c.ClosedAt,
	}
}

// NewConversationResponses maps a slice.
func NewConversationResponses(items []*entity.Conversation) []ConversationResponse {
	out := make([]ConversationResponse, len(items))
	for i, c := range items {
		out[i] = NewConversationResponse(c)
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
	return MessageResponse{
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
}

// NewMessageResponses maps a slice.
func NewMessageResponses(items []*entity.Message) []MessageResponse {
	out := make([]MessageResponse, len(items))
	for i, m := range items {
		out[i] = NewMessageResponse(m)
	}
	return out
}

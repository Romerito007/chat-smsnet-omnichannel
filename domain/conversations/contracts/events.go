package contracts

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// Realtime event names emitted by the conversations service.
const (
	RealtimeMessageCreated          = "message.created"
	RealtimeConversationUpdated     = "conversation.updated"
	RealtimeConversationAssigned    = "conversation.assigned"
	RealtimeConversationTransferred = "conversation.transferred"
	RealtimeTypingStarted           = "typing.started"
	RealtimeTypingStopped           = "typing.stopped"
	RealtimeMessageRead             = "message.read"
)

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
	SectorID      string    `json:"sector_id,omitempty"`
	QueueID       string    `json:"queue_id,omitempty"`
	Status        string    `json:"status"`
	AssignedTo    string    `json:"assigned_to,omitempty"`
	Priority      string    `json:"priority"`
	Tags          []string  `json:"tags,omitempty"`
	LastMessageAt time.Time `json:"last_message_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// NewConversationPayload builds the payload from a conversation entity.
func NewConversationPayload(c *entity.Conversation) ConversationPayload {
	return ConversationPayload{
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
		UpdatedAt:     c.UpdatedAt,
	}
}

// MessagePayload is the realtime/event representation of a message.
type MessagePayload struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderType     string    `json:"sender_type"`
	SenderID       string    `json:"sender_id,omitempty"`
	Direction      string    `json:"direction"`
	MessageType    string    `json:"message_type"`
	Text           string    `json:"text"`
	Internal       bool      `json:"internal"`
	DeliveryStatus string    `json:"delivery_status,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// NewMessagePayload builds the payload from a message entity.
func NewMessagePayload(m *entity.Message) MessagePayload {
	return MessagePayload{
		ID:             m.ID,
		ConversationID: m.ConversationID,
		SenderType:     string(m.SenderType),
		SenderID:       m.SenderID,
		Direction:      string(m.Direction),
		MessageType:    string(m.MessageType),
		Text:           m.Text,
		Internal:       m.Direction == entity.DirectionInternal,
		DeliveryStatus: string(m.DeliveryStatus),
		CreatedAt:      m.CreatedAt,
	}
}

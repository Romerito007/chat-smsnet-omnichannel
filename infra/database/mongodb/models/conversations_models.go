package models

import "time"

// Conversation is the BSON document for a conversation.
type Conversation struct {
	Base          `bson:",inline"`
	ContactID     string     `bson:"contact_id"`
	Channel       string     `bson:"channel"`
	ChannelID     string     `bson:"channel_id,omitempty"`
	SectorID      string     `bson:"sector_id,omitempty"`
	QueueID       string     `bson:"queue_id,omitempty"`
	Status        string     `bson:"status"`
	AssignedTo    string     `bson:"assigned_to,omitempty"`
	Priority      string     `bson:"priority"`
	Protocol      string     `bson:"protocol,omitempty"`
	Tags          []string   `bson:"tags,omitempty"`
	LastMessageAt time.Time  `bson:"last_message_at"`
	UnreadCount   int        `bson:"unread_count,omitempty"`
	LastReadAt    *time.Time `bson:"last_read_at,omitempty"`
	ClosedAt      *time.Time `bson:"closed_at,omitempty"`
}

// Attachment is the BSON sub-document for a message attachment.
type Attachment struct {
	ID          string `bson:"id,omitempty"`
	URL         string `bson:"url,omitempty"`
	ContentType string `bson:"content_type,omitempty"`
	Filename    string `bson:"filename,omitempty"`
	Size        int64  `bson:"size,omitempty"`
}

// Message is the BSON document for a message. Edits/deletes are soft.
type Message struct {
	ID                string         `bson:"_id"`
	TenantID          string         `bson:"tenant_id"`
	ConversationID    string         `bson:"conversation_id"`
	SenderType        string         `bson:"sender_type"`
	SenderID          string         `bson:"sender_id,omitempty"`
	Direction         string         `bson:"direction"`
	MessageType       string         `bson:"message_type"`
	Text              string         `bson:"text"`
	Attachments       []Attachment   `bson:"attachments,omitempty"`
	Metadata          map[string]any `bson:"metadata,omitempty"`
	CreatedAt         time.Time      `bson:"created_at"`
	DeliveryStatus    string         `bson:"delivery_status,omitempty"`
	DeliveryError     string         `bson:"delivery_error,omitempty"`
	ExternalMessageID string         `bson:"external_message_id,omitempty"`
	DeliveredAt       *time.Time     `bson:"delivered_at,omitempty"`
	ReadAt            *time.Time     `bson:"read_at,omitempty"`
	EditedAt          *time.Time     `bson:"edited_at,omitempty"`
	DeletedAt         *time.Time     `bson:"deleted_at,omitempty"`
}

// ConversationEvent is the BSON document for a conversation timeline event.
type ConversationEvent struct {
	ID             string         `bson:"_id"`
	TenantID       string         `bson:"tenant_id"`
	ConversationID string         `bson:"conversation_id"`
	Type           string         `bson:"type"`
	ActorType      string         `bson:"actor_type"`
	ActorID        string         `bson:"actor_id,omitempty"`
	Data           map[string]any `bson:"data,omitempty"`
	CreatedAt      time.Time      `bson:"created_at"`
}

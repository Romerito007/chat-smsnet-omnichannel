package models

import "time"

// Conversation is the BSON document for a conversation.
type Conversation struct {
	Base                  `bson:",inline"`
	ContactID             string         `bson:"contact_id"`
	Channel               string         `bson:"channel"`
	ChannelID             string         `bson:"channel_id,omitempty"`
	SectorID              string         `bson:"sector_id,omitempty"`
	QueueID               string         `bson:"queue_id,omitempty"`
	Status                string         `bson:"status"`
	AssignedTo            string         `bson:"assigned_to,omitempty"`
	Priority              string         `bson:"priority"`
	Protocol              string         `bson:"protocol,omitempty"`
	Tags                  []string       `bson:"tags,omitempty"`
	CustomAttributes      map[string]any `bson:"custom_attributes,omitempty"`
	LastMessageAt         time.Time      `bson:"last_message_at"`
	LastMessage           *LastMessage   `bson:"last_message,omitempty"`
	LastCustomerMessageAt *time.Time     `bson:"last_customer_message_at,omitempty"`
	UnreadCount           int            `bson:"unread_count,omitempty"`
	LastReadAt            *time.Time     `bson:"last_read_at,omitempty"`
	ClosedAt              *time.Time     `bson:"closed_at,omitempty"`
}

// LastMessage is the denormalized preview of a conversation's most recent message,
// kept on the conversation document so the inbox renders without aggregating the
// messages collection.
type LastMessage struct {
	MessageID   string    `bson:"message_id"`
	Preview     string    `bson:"preview,omitempty"`
	SenderType  string    `bson:"sender_type"`
	MessageType string    `bson:"message_type"`
	CreatedAt   time.Time `bson:"created_at"`
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
	ID                string                   `bson:"_id"`
	TenantID          string                   `bson:"tenant_id"`
	ConversationID    string                   `bson:"conversation_id"`
	SenderType        string                   `bson:"sender_type"`
	SenderID          string                   `bson:"sender_id,omitempty"`
	Direction         string                   `bson:"direction"`
	MessageType       string                   `bson:"message_type"`
	Text              string                   `bson:"text"`
	Attachments       []Attachment             `bson:"attachments,omitempty"`
	Template          *MessageTemplate         `bson:"template,omitempty"`
	Contacts          []MessageContact         `bson:"contacts,omitempty"`
	Location          *MessageLocation         `bson:"location,omitempty"`
	Interactive       *MessageInteractive      `bson:"interactive,omitempty"`
	InteractiveReply  *MessageInteractiveReply `bson:"interactive_reply,omitempty"`
	Metadata          map[string]any           `bson:"metadata,omitempty"`
	CreatedAt         time.Time                `bson:"created_at"`
	DeliveryStatus    string                   `bson:"delivery_status,omitempty"`
	DeliveryError     string                   `bson:"delivery_error,omitempty"`
	ExternalMessageID string                   `bson:"external_message_id,omitempty"`
	DeliveredAt       *time.Time               `bson:"delivered_at,omitempty"`
	ReadAt            *time.Time               `bson:"read_at,omitempty"`
	EditedAt          *time.Time               `bson:"edited_at,omitempty"`
	DeletedAt         *time.Time               `bson:"deleted_at,omitempty"`
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

// MessageTemplate is the BSON sub-document for a WhatsApp template message.
type MessageTemplate struct {
	TemplateID string            `bson:"template_id"`
	Params     map[string]string `bson:"params,omitempty"`
}

// MessageContact is the BSON sub-document for one vCard (message_type=contact).
type MessageContact struct {
	Name         MsgContactName    `bson:"name"`
	Phones       []MsgContactPhone `bson:"phones"`
	Emails       []MsgContactEmail `bson:"emails,omitempty"`
	Organization *MsgContactOrg    `bson:"organization,omitempty"`
}

// MsgContactName / MsgContactPhone / MsgContactEmail / MsgContactOrg are the vCard
// sub-documents.
type MsgContactName struct {
	Formatted string `bson:"formatted"`
	First     string `bson:"first,omitempty"`
	Last      string `bson:"last,omitempty"`
}
type MsgContactPhone struct {
	Phone string `bson:"phone"`
	Type  string `bson:"type,omitempty"`
	WaID  string `bson:"wa_id,omitempty"`
}
type MsgContactEmail struct {
	Email string `bson:"email"`
	Type  string `bson:"type,omitempty"`
}
type MsgContactOrg struct {
	Company string `bson:"company,omitempty"`
	Title   string `bson:"title,omitempty"`
}

// MessageLocation is the BSON sub-document for message_type=location.
type MessageLocation struct {
	Latitude  float64 `bson:"latitude"`
	Longitude float64 `bson:"longitude"`
	Name      string  `bson:"name,omitempty"`
	Address   string  `bson:"address,omitempty"`
}

// MessageInteractive is the BSON sub-document for an outbound interactive menu.
type MessageInteractive struct {
	Kind     string          `bson:"kind"`
	Header   string          `bson:"header,omitempty"`
	Body     string          `bson:"body"`
	Footer   string          `bson:"footer,omitempty"`
	Buttons  []MsgIntButton  `bson:"buttons,omitempty"`
	Button   string          `bson:"button,omitempty"`
	Sections []MsgIntSection `bson:"sections,omitempty"`
}
type MsgIntButton struct {
	ID    string `bson:"id"`
	Title string `bson:"title"`
}
type MsgIntSection struct {
	Title string      `bson:"title,omitempty"`
	Rows  []MsgIntRow `bson:"rows"`
}
type MsgIntRow struct {
	ID          string `bson:"id"`
	Title       string `bson:"title"`
	Description string `bson:"description,omitempty"`
}

// MessageInteractiveReply is the BSON sub-document for an inbound interactive reply.
type MessageInteractiveReply struct {
	Kind             string `bson:"kind"`
	ID               string `bson:"id"`
	Title            string `bson:"title"`
	Description      string `bson:"description,omitempty"`
	ContextMessageID string `bson:"context_message_id,omitempty"`
}

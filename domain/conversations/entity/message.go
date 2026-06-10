package entity

import "time"

// SenderType identifies who authored a message.
type SenderType string

const (
	SenderCustomer   SenderType = "customer"
	SenderAgent      SenderType = "agent"
	SenderSystem     SenderType = "system"
	SenderAutomation SenderType = "automation"
	SenderCopilot    SenderType = "copilot"
)

// Direction is the flow of a message relative to the operation.
type Direction string

const (
	DirectionInbound  Direction = "inbound"  // from customer
	DirectionOutbound Direction = "outbound" // to customer
	DirectionInternal Direction = "internal" // internal note, never delivered
)

// MessageType is the payload kind.
type MessageType string

const (
	MessageText     MessageType = "text"
	MessageImage    MessageType = "image"
	MessageFile     MessageType = "file"
	MessageAudio    MessageType = "audio"
	MessageTemplate MessageType = "template"
	MessageSystem   MessageType = "system"
)

// Valid reports whether t is a known message type.
func (t MessageType) Valid() bool {
	switch t {
	case MessageText, MessageImage, MessageFile, MessageAudio, MessageTemplate, MessageSystem:
		return true
	}
	return false
}

// DeliveryStatus tracks outbound delivery, owned by the channels domain.
type DeliveryStatus string

const (
	DeliveryNone      DeliveryStatus = ""        // internal/non-deliverable
	DeliveryPending   DeliveryStatus = "pending" // queued for delivery
	DeliverySent      DeliveryStatus = "sent"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryRead      DeliveryStatus = "read"
	DeliveryFailed    DeliveryStatus = "failed"
)

// Attachment is a media reference carried by a message.
type Attachment struct {
	ID          string `json:"id,omitempty"`
	URL         string `json:"url,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

// Message is a single entry in a conversation. Edits and deletes are soft
// (EditedAt / DeletedAt) so history is preserved.
type Message struct {
	ID                string
	TenantID          string
	ConversationID    string
	SenderType        SenderType
	SenderID          string
	Direction         Direction
	MessageType       MessageType
	Text              string
	Attachments       []Attachment
	Metadata          map[string]any
	CreatedAt         time.Time
	DeliveryStatus    DeliveryStatus
	DeliveryError     string
	ExternalMessageID string
	DeliveredAt       *time.Time
	ReadAt            *time.Time
	EditedAt          *time.Time
	DeletedAt         *time.Time
}

// IsDeleted reports whether the message was soft-deleted.
func (m *Message) IsDeleted() bool { return m.DeletedAt != nil }

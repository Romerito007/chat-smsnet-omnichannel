// Package contracts holds the conversations service inputs/outputs.
package contracts

import "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"

// CreateConversation is the input to open a conversation.
type CreateConversation struct {
	ContactID string
	// ChannelID is the id of the specific ChannelConnection the conversation
	// belongs to (required). The channel TYPE is derived from this connection — the
	// client's channel type is not trusted.
	ChannelID  string
	SectorID   string
	QueueID    string
	AssignedTo string
	Priority   entity.Priority
	Tags       []string
}

// UpdateConversation carries optional fields; nil pointers mean "leave unchanged".
type UpdateConversation struct {
	SectorID   *string
	QueueID    *string
	Status     *entity.Status
	AssignedTo *string
	Priority   *entity.Priority
	Tags       *[]string
	// CustomAttributes, when non-nil, replaces the whole custom-attributes map
	// (omit a key to remove it). Validated against applies_to=conversation defs.
	CustomAttributes *map[string]any
}

// SendMessage is the input to post an outbound message from an agent.
type SendMessage struct {
	MessageType entity.MessageType
	Text        string
	Attachments []entity.Attachment
	// Template is required when MessageType=template (WhatsApp): the opaque
	// integrator template id + filled named params. The display text and the
	// outbound payload are derived from it.
	Template *SendTemplate
	// Contacts is required when MessageType=contact (1..10 vCards); Location is
	// required when MessageType=location.
	Contacts []entity.ContactCard
	Location *entity.Location
	Metadata map[string]any
}

// SendTemplate is the template selection on a template send.
type SendTemplate struct {
	TemplateID string
	Params     map[string]string
}

// EditMessage is the input to edit a message's text (soft edit: edited_at is set
// and the original is preserved as history).
type EditMessage struct {
	Text string
}

// AddInternalNote is the input to add an internal note (never delivered).
type AddInternalNote struct {
	Text string
	// MentionUserIDs are users @-mentioned in the note; each is notified.
	MentionUserIDs []string
}

// CloseConversation is the input to close a conversation.
type CloseConversation struct {
	CloseReasonID string
	Note          string
}

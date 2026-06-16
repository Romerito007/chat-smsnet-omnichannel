// Package entity holds the conversations domain aggregates: Conversation,
// Message and ConversationEvent.
package entity

import "time"

// Status is the conversation lifecycle state.
type Status string

const (
	StatusNew             Status = "new"
	StatusQueued          Status = "queued"
	StatusAssigned        Status = "assigned"
	StatusWaitingCustomer Status = "waiting_customer"
	StatusWaitingAgent    Status = "waiting_agent"
	StatusTransferred     Status = "transferred"
	StatusResolved        Status = "resolved"
	StatusClosed          Status = "closed"
	StatusArchived        Status = "archived"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusQueued, StatusAssigned,
		StatusWaitingCustomer, StatusWaitingAgent, StatusTransferred,
		StatusResolved, StatusClosed, StatusArchived:
		return true
	}
	return false
}

// IsClosed reports whether the conversation is in a terminal state.
func (s Status) IsClosed() bool {
	return s == StatusResolved || s == StatusClosed || s == StatusArchived
}

// Priority ranks a conversation for routing/attention.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// Valid reports whether p is a known priority.
func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}

// Conversation is a thread of messages between a contact and the operation.
type Conversation struct {
	ID         string
	TenantID   string
	ContactID  string
	Channel    string // channel TYPE (e.g. "whatsapp"); type-level logic (adapters, SLA match)
	ChannelID  string // id of the specific ChannelConnection this conversation belongs to
	SectorID   string
	QueueID    string
	Status     Status
	AssignedTo string
	Priority   Priority
	// Protocol is the per-tenant, per-year protocol number ("2026-000123") assigned
	// when the conversation is opened on a channel with UsesProtocol=true. Empty for
	// conversations on channels without protocol numbering.
	Protocol string
	// Tags always stores canonical tag IDs (never names). The service resolves any
	// name supplied on write to its ID, so the array stays ID-only — keeping the
	// front render and tag removal (which match by ID) consistent.
	Tags []string
	// CustomAttributes holds tenant-defined custom attribute values (key→value),
	// validated against definitions with applies_to=conversation.
	CustomAttributes map[string]any
	LastMessageAt    time.Time
	// LastMessage is a denormalized snapshot of the conversation's most recent
	// message (preview/type/sender/time), updated at every message create. It lets
	// the inbox render each row's preview straight from the conversation document —
	// no per-page aggregation over the messages collection. Nil when the
	// conversation has no message yet.
	LastMessage *LastMessageSnapshot
	// LastCustomerMessageAt is the time of the most recent INBOUND (customer) message,
	// denormalized at the inbound chokepoint. It drives the WhatsApp 24h service
	// window. Nil when the customer has never messaged. (LastMessageAt covers ANY
	// message, including outbound, so it cannot be used for the window.)
	LastCustomerMessageAt *time.Time
	// UnreadCount is the number of inbound (customer) messages since an agent last
	// read the conversation. Bumped on each inbound message; zeroed by MarkRead
	// (POST /read).
	UnreadCount int
	// LastReadAt is when an agent last read the conversation (nil if never).
	LastReadAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ClosedAt   *time.Time
}

// ChannelTypeWhatsApp is the channel TYPE for which WhatsApp-only rules (templates,
// interactive messages, the 24h service window) apply. Mirrors channels TypeWhatsApp
// (kept as a literal so the conversations entity need not import the channels domain).
const ChannelTypeWhatsApp = "whatsapp"

// WhatsAppWindowDuration is the WhatsApp service window: free-form (non-template)
// messages are only deliverable within 24h of the customer's last inbound message.
const WhatsAppWindowDuration = 24 * time.Hour

// WhatsAppWindow is the derived 24h-service-window state exposed to the front so it
// can warn "outside the window, use a template". open = the customer messaged in the
// last 24h; expires_at = LastCustomerMessageAt + 24h (absent when none).
type WhatsAppWindow struct {
	Open      bool       `json:"open"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// WhatsAppWindowState returns the 24h window state for a WhatsApp conversation,
// computed against `now` (server clock). Returns nil for non-WhatsApp channels —
// the window does not apply there, so the block is omitted from payloads.
func (c *Conversation) WhatsAppWindowState(now time.Time) *WhatsAppWindow {
	if c.Channel != ChannelTypeWhatsApp {
		return nil
	}
	w := &WhatsAppWindow{}
	if c.LastCustomerMessageAt != nil {
		exp := c.LastCustomerMessageAt.Add(WhatsAppWindowDuration)
		w.Open = now.Before(exp)
		w.ExpiresAt = &exp
	}
	return w
}

// lastMessagePreviewLen bounds the denormalized preview (matches the inbox DTO).
const lastMessagePreviewLen = 280

// LastMessageSnapshot is the denormalized preview of a conversation's most recent
// message, stored on the conversation document for the inbox. MessageID identifies
// which message it mirrors, so an edit/delete of THAT message can refresh or
// recompute the snapshot.
type LastMessageSnapshot struct {
	MessageID   string
	Preview     string
	SenderType  SenderType
	MessageType MessageType
	CreatedAt   time.Time
}

// NewLastMessageSnapshot builds the snapshot from a message: a length-bounded text
// preview plus the fields the inbox row needs (sender/type/time). Returns nil for a
// nil message (e.g. a conversation with no remaining messages).
func NewLastMessageSnapshot(m *Message) *LastMessageSnapshot {
	if m == nil {
		return nil
	}
	return &LastMessageSnapshot{
		MessageID:   m.ID,
		Preview:     truncateRunes(m.Text, lastMessagePreviewLen),
		SenderType:  m.SenderType,
		MessageType: m.MessageType,
		CreatedAt:   m.CreatedAt,
	}
}

// truncateRunes shortens s to at most n runes, appending an ellipsis when cut.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

package entity

import (
	"strings"
	"time"
	"unicode/utf8"
)

// SenderType identifies who authored a message.
type SenderType string

const (
	SenderCustomer SenderType = "customer"
	SenderAgent    SenderType = "agent"
	SenderSystem   SenderType = "system"
	SenderCopilot  SenderType = "copilot"
	// SenderAutomation marks a message authored by an automation rule (shown as
	// "System Automation" in history). It is also the anti-loop signal: a
	// message_created emitted for an automation-authored message carries
	// origin=automation, so it never re-triggers automation rules.
	SenderAutomation SenderType = "automation"
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
	MessageVideo    MessageType = "video"
	MessageContact  MessageType = "contact"  // one or more vCards (Contacts)
	MessageLocation MessageType = "location" // a single geographic point (Location)
	// MessageInteractive is an OUTBOUND menu (reply buttons or list) the business
	// sends; MessageInteractiveReply is the INBOUND choice the customer made.
	MessageInteractive      MessageType = "interactive"
	MessageInteractiveReply MessageType = "interactive_reply"
	MessageTemplate         MessageType = "template"
	MessageSystem           MessageType = "system"
)

// Valid reports whether t is a known message type.
func (t MessageType) Valid() bool {
	switch t {
	case MessageText, MessageImage, MessageFile, MessageAudio, MessageVideo,
		MessageContact, MessageLocation, MessageInteractive, MessageInteractiveReply,
		MessageTemplate, MessageSystem:
		return true
	}
	return false
}

// MaxContactsPerMessage bounds how many vCards a single contact message may carry.
const MaxContactsPerMessage = 10

// ContactCard is one shared vCard (message_type=contact). The JSON shape mirrors the
// WhatsApp contacts[] block the gateway translates to/from Meta.
type ContactCard struct {
	Name         ContactName    `json:"name" bson:"name"`
	Phones       []ContactPhone `json:"phones" bson:"phones"`
	Emails       []ContactEmail `json:"emails,omitempty" bson:"emails,omitempty"`
	Organization *ContactOrg    `json:"organization,omitempty" bson:"organization,omitempty"`
}

// ContactName is a contact's display name (Formatted maps to Meta formatted_name).
type ContactName struct {
	Formatted string `json:"formatted" bson:"formatted"`
	First     string `json:"first,omitempty" bson:"first,omitempty"`
	Last      string `json:"last,omitempty" bson:"last,omitempty"`
}

// ContactPhone is one phone of a contact.
type ContactPhone struct {
	Phone string `json:"phone" bson:"phone"`
	Type  string `json:"type,omitempty" bson:"type,omitempty"`
	WaID  string `json:"wa_id,omitempty" bson:"wa_id,omitempty"`
}

// ContactEmail is one email of a contact.
type ContactEmail struct {
	Email string `json:"email" bson:"email"`
	Type  string `json:"type,omitempty" bson:"type,omitempty"`
}

// ContactOrg is a contact's organization.
type ContactOrg struct {
	Company string `json:"company,omitempty" bson:"company,omitempty"`
	Title   string `json:"title,omitempty" bson:"title,omitempty"`
}

// Location is a single geographic point (message_type=location), mirroring the
// WhatsApp location block.
type Location struct {
	Latitude  float64 `json:"latitude" bson:"latitude"`
	Longitude float64 `json:"longitude" bson:"longitude"`
	Name      string  `json:"name,omitempty" bson:"name,omitempty"`
	Address   string  `json:"address,omitempty" bson:"address,omitempty"`
}

// ValidateContacts checks a message_type=contact payload, returning "" when valid or
// a human-readable reason otherwise (the caller wraps it as a validation error). It
// is shared by the outbound (SendMessage) and inbound paths so both rails enforce
// the same rules.
func ValidateContacts(cs []ContactCard) string {
	if len(cs) == 0 {
		return "contacts are required for message_type=contact"
	}
	if len(cs) > MaxContactsPerMessage {
		return "at most 10 contacts per message"
	}
	for _, c := range cs {
		if strings.TrimSpace(c.Name.Formatted) == "" && strings.TrimSpace(c.Name.First) == "" && strings.TrimSpace(c.Name.Last) == "" {
			return "each contact requires a name"
		}
		if len(c.Phones) == 0 {
			return "each contact requires at least one phone"
		}
		for _, p := range c.Phones {
			if strings.TrimSpace(p.Phone) == "" {
				return "contact phone must not be empty"
			}
		}
	}
	return ""
}

// Validate checks a message_type=location payload, returning "" when valid.
func (l *Location) Validate() string {
	if l == nil {
		return "location is required for message_type=location"
	}
	if l.Latitude < -90 || l.Latitude > 90 {
		return "latitude must be between -90 and 90"
	}
	if l.Longitude < -180 || l.Longitude > 180 {
		return "longitude must be between -180 and 180"
	}
	return ""
}

// Interactive kinds and the WhatsApp limits enforced by ValidateInteractive (the
// gateway can assume a valid payload). Char limits count Unicode runes.
const (
	InteractiveButtons = "buttons" // reply buttons (≤3)
	InteractiveList    = "list"    // single-select list (sections/rows)

	maxReplyButtons      = 3
	maxButtonTitle       = 20
	maxButtonID          = 256
	maxButtonsBody       = 1024
	maxInteractiveHeader = 60
	maxInteractiveFooter = 60
	maxListButton        = 20
	maxListSections      = 10
	maxListRows          = 10
	maxSectionTitle      = 24
	maxRowTitle          = 24
	maxRowDescription    = 72
	maxRowID             = 200
	maxListBody          = 4096
)

// Interactive is an OUTBOUND interactive message (message_type=interactive): reply
// buttons or a single-select list. v1 supports a text-only header. The JSON shape
// mirrors what the gateway translates to the WhatsApp interactive.{button|list}
// block.
type Interactive struct {
	Kind   string `json:"kind" bson:"kind"` // "buttons" | "list"
	Header string `json:"header,omitempty" bson:"header,omitempty"`
	Body   string `json:"body" bson:"body"`
	Footer string `json:"footer,omitempty" bson:"footer,omitempty"`
	// Buttons is set when Kind=buttons. Button/Sections are set when Kind=list
	// (Button is the label that opens the list).
	Buttons  []InteractiveButton  `json:"buttons,omitempty" bson:"buttons,omitempty"`
	Button   string               `json:"button,omitempty" bson:"button,omitempty"`
	Sections []InteractiveSection `json:"sections,omitempty" bson:"sections,omitempty"`
}

// InteractiveButton is one reply button (Kind=buttons).
type InteractiveButton struct {
	ID    string `json:"id" bson:"id"`
	Title string `json:"title" bson:"title"`
}

// InteractiveSection is a list section (Kind=list).
type InteractiveSection struct {
	Title string           `json:"title,omitempty" bson:"title,omitempty"`
	Rows  []InteractiveRow `json:"rows" bson:"rows"`
}

// InteractiveRow is one selectable list row.
type InteractiveRow struct {
	ID          string `json:"id" bson:"id"`
	Title       string `json:"title" bson:"title"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`
}

// InteractiveReply is the INBOUND customer choice (message_type=interactive_reply):
// the id+title of the chosen button/row, plus ContextMessageID linking it back to
// the internal id of the menu message the chat sent.
type InteractiveReply struct {
	Kind             string `json:"kind" bson:"kind"` // "button" | "list"
	ID               string `json:"id" bson:"id"`
	Title            string `json:"title" bson:"title"`
	Description      string `json:"description,omitempty" bson:"description,omitempty"`
	ContextMessageID string `json:"context_message_id,omitempty" bson:"context_message_id,omitempty"`
}

func tooLong(s string, max int) bool { return utf8.RuneCountInString(s) > max }

// Validate checks an outbound interactive payload against the WhatsApp limits,
// returning "" when valid or a human-readable reason. Shared by SendMessage and the
// automation send_interactive action so both rails enforce the same rules.
func (iv *Interactive) Validate() string {
	if iv == nil {
		return "interactive is required for message_type=interactive"
	}
	if tooLong(iv.Header, maxInteractiveHeader) {
		return "interactive header must be at most 60 characters"
	}
	if tooLong(iv.Footer, maxInteractiveFooter) {
		return "interactive footer must be at most 60 characters"
	}
	if strings.TrimSpace(iv.Body) == "" {
		return "interactive body is required"
	}
	switch iv.Kind {
	case InteractiveButtons:
		if tooLong(iv.Body, maxButtonsBody) {
			return "interactive body must be at most 1024 characters"
		}
		if len(iv.Buttons) == 0 {
			return "interactive requires at least one button"
		}
		if len(iv.Buttons) > maxReplyButtons {
			return "interactive supports at most 3 buttons"
		}
		seen := make(map[string]bool, len(iv.Buttons))
		for _, b := range iv.Buttons {
			if strings.TrimSpace(b.ID) == "" {
				return "button id is required"
			}
			if tooLong(b.ID, maxButtonID) {
				return "button id must be at most 256 characters"
			}
			if strings.TrimSpace(b.Title) == "" {
				return "button title is required"
			}
			if tooLong(b.Title, maxButtonTitle) {
				return "button title must be at most 20 characters"
			}
			if seen[b.ID] {
				return "button ids must be unique within the message"
			}
			seen[b.ID] = true
		}
	case InteractiveList:
		if tooLong(iv.Body, maxListBody) {
			return "interactive body must be at most 4096 characters"
		}
		if strings.TrimSpace(iv.Button) == "" {
			return "list button label is required"
		}
		if tooLong(iv.Button, maxListButton) {
			return "list button label must be at most 20 characters"
		}
		if len(iv.Sections) == 0 {
			return "list requires at least one section"
		}
		if len(iv.Sections) > maxListSections {
			return "list supports at most 10 sections"
		}
		seen := map[string]bool{}
		total := 0
		for _, sec := range iv.Sections {
			if tooLong(sec.Title, maxSectionTitle) {
				return "section title must be at most 24 characters"
			}
			if len(sec.Rows) == 0 {
				return "each list section requires at least one row"
			}
			for _, r := range sec.Rows {
				total++
				if strings.TrimSpace(r.ID) == "" {
					return "row id is required"
				}
				if tooLong(r.ID, maxRowID) {
					return "row id must be at most 200 characters"
				}
				if strings.TrimSpace(r.Title) == "" {
					return "row title is required"
				}
				if tooLong(r.Title, maxRowTitle) {
					return "row title must be at most 24 characters"
				}
				if tooLong(r.Description, maxRowDescription) {
					return "row description must be at most 72 characters"
				}
				if seen[r.ID] {
					return "row ids must be unique within the message"
				}
				seen[r.ID] = true
			}
		}
		if total > maxListRows {
			return "list supports at most 10 rows in total"
		}
	default:
		return "interactive.kind must be 'buttons' or 'list'"
	}
	return ""
}

// MessageTypeForContentType derives the media message type from a MIME type:
// image/* -> image, audio/* -> audio, video/* -> video, anything else -> file.
func MessageTypeForContentType(contentType string) MessageType {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return MessageImage
	case strings.HasPrefix(contentType, "audio/"):
		return MessageAudio
	case strings.HasPrefix(contentType, "video/"):
		return MessageVideo
	default:
		return MessageFile
	}
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

// deliveryRank orders the forward-only delivery lifecycle so a receipt can only
// advance a message's status (never regress). failed is off-ladder (-1): it is
// reachable from any non-terminal status but never overwritten by a later
// delivered/read.
func (d DeliveryStatus) deliveryRank() int {
	switch d {
	case DeliveryNone:
		return 0
	case DeliveryPending:
		return 1
	case DeliverySent:
		return 2
	case DeliveryDelivered:
		return 3
	case DeliveryRead:
		return 4
	case DeliveryFailed:
		return -1
	default:
		return 0
	}
}

// DeliveryAdvances reports whether a receipt moving a message from->to is a real
// forward transition (so applying receipts is idempotent and order-tolerant). A
// failed receipt advances unless the message is already read or failed; otherwise
// to must rank strictly above from.
func DeliveryAdvances(from, to DeliveryStatus) bool {
	if to == DeliveryFailed {
		return from != DeliveryRead && from != DeliveryFailed
	}
	if from == DeliveryFailed {
		return false
	}
	return to.deliveryRank() > from.deliveryRank()
}

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
	ID             string
	TenantID       string
	ConversationID string
	SenderType     SenderType
	SenderID       string
	Direction      Direction
	MessageType    MessageType
	Text           string
	Attachments    []Attachment
	// Template is set for message_type=template (WhatsApp). It carries the opaque
	// integrator template id + filled named params sent to the integrator. Text
	// holds the locally-resolved display string (never sent out).
	Template *TemplatePayload
	// Contacts is set for message_type=contact (1..10 vCards); Location for
	// message_type=location. Both bidirectional (inbound + outbound).
	Contacts []ContactCard
	Location *Location
	// Interactive is the OUTBOUND menu (message_type=interactive); InteractiveReply
	// is the INBOUND customer choice (message_type=interactive_reply).
	Interactive       *Interactive
	InteractiveReply  *InteractiveReply
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

// MessageTemplate is the WhatsApp template payload of an outgoing template
// message: the opaque integrator template id and the filled named params. Only
// these go to the integrator; the resolved display text lives in Message.Text.
type TemplatePayload struct {
	TemplateID string
	Params     map[string]string
}

// IsDeleted reports whether the message was soft-deleted.
func (m *Message) IsDeleted() bool { return m.DeletedAt != nil }

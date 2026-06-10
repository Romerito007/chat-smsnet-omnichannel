package entity

import "time"

// DeliveryStatus tracks an outbound delivery's lifecycle. It mirrors the
// message delivery status but is the channel-domain's own record.
type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliverySent      DeliveryStatus = "sent"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryRead      DeliveryStatus = "read"
	DeliveryFailed    DeliveryStatus = "failed"
)

// deliveryRank orders statuses so receipts only advance (never regress),
// making duplicate receipts idempotent.
var deliveryRank = map[DeliveryStatus]int{
	DeliveryPending:   0,
	DeliverySent:      1,
	DeliveryDelivered: 2,
	DeliveryRead:      3,
	DeliveryFailed:    4,
}

// Advances reports whether moving to `to` is a forward transition from `from`.
func Advances(from, to DeliveryStatus) bool {
	return deliveryRank[to] > deliveryRank[from]
}

// OutboundDelivery is the channel-domain record of delivering one outbound
// message to the channel.
type OutboundDelivery struct {
	ID                  string
	TenantID            string
	ChannelConnectionID string
	ConversationID      string
	MessageID           string
	Status              DeliveryStatus
	Attempts            int
	ExternalMessageID   string
	LastError           string
	NextRetryAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

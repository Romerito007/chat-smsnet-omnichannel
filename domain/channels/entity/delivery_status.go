package entity

// DeliveryStatus is the channel-side delivery state reported by an adapter's send
// result. The forward-only message delivery lifecycle (delivered/read/failed) is
// tracked on the conversations message; this type is the adapter contract value.
type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliverySent      DeliveryStatus = "sent"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryRead      DeliveryStatus = "read"
	DeliveryFailed    DeliveryStatus = "failed"
)

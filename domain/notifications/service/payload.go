package service

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/entity"
)

// realtimePayload is the notification.created event body delivered to the
// recipient's WebSocket.
type realtimePayload struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	Link      string    `json:"link,omitempty"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

func newPayload(n *entity.Notification) realtimePayload {
	return realtimePayload{
		ID: n.ID, Type: string(n.Type), Title: n.Title, Body: n.Body,
		Link: n.Link, Read: n.Read, CreatedAt: n.CreatedAt,
	}
}

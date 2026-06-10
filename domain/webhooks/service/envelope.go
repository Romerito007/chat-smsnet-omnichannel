package service

import (
	"encoding/json"
	"time"
)

// Envelope is the canonical JSON body delivered for every webhook event. The
// exact bytes are what gets HMAC-signed and sent, so receivers can verify the
// signature over the raw body.
type Envelope struct {
	ID        string    `json:"id"`
	Event     string    `json:"event"`
	CreatedAt time.Time `json:"created_at"`
	Data      any       `json:"data"`
}

// buildEnvelope serializes an event payload into the canonical body.
func buildEnvelope(id, event string, createdAt time.Time, data any) ([]byte, error) {
	return json.Marshal(Envelope{ID: id, Event: event, CreatedAt: createdAt, Data: data})
}

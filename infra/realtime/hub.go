// Package realtime implements the WebSocket fan-out used for live updates. A
// per-process Hub tracks connected clients by topic; a Redis pub/sub transport
// fans messages across processes so any WS node can deliver to any client.
package realtime

import (
	"sync"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Topic identifies a fan-out channel. Topics are tenant-scoped by convention,
// e.g. "t:<tenant_id>:conversation:<id>".
type Topic = string

// Message is an opaque payload delivered to subscribers of a topic.
type Message struct {
	Topic   Topic  `json:"topic"`
	Payload []byte `json:"payload"`
}

// Client is a single connected WebSocket peer. The transport owns the socket;
// the Hub only needs a buffered send channel and the topics the client follows.
type Client struct {
	ID     shared.ID
	Send   chan Message
	Topics map[Topic]struct{}
}

// Hub maintains the set of clients subscribed to each topic within one process.
type Hub struct {
	mu      sync.RWMutex
	clients map[Topic]map[*Client]struct{}
}

// NewHub builds an empty hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[Topic]map[*Client]struct{})}
}

// Subscribe registers a client for a topic.
func (h *Hub) Subscribe(c *Client, topic Topic) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[topic] == nil {
		h.clients[topic] = make(map[*Client]struct{})
	}
	h.clients[topic][c] = struct{}{}
	if c.Topics == nil {
		c.Topics = make(map[Topic]struct{})
	}
	c.Topics[topic] = struct{}{}
}

// Unsubscribe removes a client from a topic.
func (h *Hub) Unsubscribe(c *Client, topic Topic) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.clients[topic]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.clients, topic)
		}
	}
	delete(c.Topics, topic)
}

// Remove drops a client from every topic it follows (on disconnect).
func (h *Hub) Remove(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for topic := range c.Topics {
		if set := h.clients[topic]; set != nil {
			delete(set, c)
			if len(set) == 0 {
				delete(h.clients, topic)
			}
		}
	}
	c.Topics = nil
}

// Deliver pushes a message to every local client subscribed to its topic.
// Clients with a full send buffer are skipped (best-effort, non-blocking) to
// keep one slow consumer from stalling fan-out.
func (h *Hub) Deliver(msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[msg.Topic] {
		select {
		case c.Send <- msg:
		default:
		}
	}
}

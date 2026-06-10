// Package websocket implements the WS upgrade endpoint and the per-connection
// read/write pumps, bridging sockets to the realtime Hub.
package websocket

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/realtime"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	sendBufferSize = 64
)

// clientCommand is the inbound control frame a client sends to (un)subscribe.
type clientCommand struct {
	Action string         `json:"action"` // "subscribe" | "unsubscribe"
	Topic  realtime.Topic `json:"topic"`
}

// Handler upgrades HTTP requests to WebSocket and wires them into the Hub.
type Handler struct {
	hub      *realtime.Hub
	upgrader websocket.Upgrader
	logger   shared.Logger
}

// NewHandler builds the WS handler. Origin checking is delegated to the CORS
// configuration upstream; here we accept the upgrade.
func NewHandler(hub *realtime.Hub, logger shared.Logger) *Handler {
	return &Handler{
		hub:    hub,
		logger: logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// ServeHTTP performs the upgrade and starts the read/write pumps.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws upgrade failed", "error", err.Error())
		return
	}

	client := &realtime.Client{
		ID:     shared.NewID(),
		Send:   make(chan realtime.Message, sendBufferSize),
		Topics: map[realtime.Topic]struct{}{},
	}

	go h.writePump(conn, client)
	h.readPump(conn, client)
}

// readPump processes inbound control frames and cleans up on disconnect.
func (h *Handler) readPump(conn *websocket.Conn, client *realtime.Client) {
	defer func() {
		h.hub.Remove(client)
		_ = conn.Close()
	}()

	conn.SetReadLimit(4096)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var cmd clientCommand
		if json.Unmarshal(raw, &cmd) != nil || cmd.Topic == "" {
			continue
		}
		switch cmd.Action {
		case "subscribe":
			h.hub.Subscribe(client, cmd.Topic)
		case "unsubscribe":
			h.hub.Unsubscribe(client, cmd.Topic)
		}
	}
}

// writePump delivers Hub messages to the socket and sends periodic pings.
func (h *Handler) writePump(conn *websocket.Conn, client *realtime.Client) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-client.Send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg.Payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

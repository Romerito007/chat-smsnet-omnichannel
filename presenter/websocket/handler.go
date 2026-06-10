// Package websocket implements the authenticated WS endpoint and the
// per-connection read/write pumps, bridging sockets to the realtime Hub.
//
// Authentication: the upgrade requires a valid JWT access token (Authorization:
// Bearer, or ?token= for browsers). The connection is bound to the token's
// tenant and user; it auto-joins the tenant, user, presence and per-sector
// rooms, and may subscribe to conversation rooms on demand (gated by the
// conversation.read permission). Rooms are always tenant-scoped server-side, so
// a client can never address another tenant.
package websocket

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/realtime"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	sendBufferSize = 64
	maxReadBytes   = 4096
)

// clientCommand is the inbound control frame a client sends to (un)subscribe to a
// conversation room.
type clientCommand struct {
	Action         string `json:"action"`          // "subscribe" | "unsubscribe"
	ConversationID string `json:"conversation_id"` // target conversation
}

// Handler upgrades authenticated HTTP requests to WebSocket and wires them into
// the Hub.
type Handler struct {
	hub            *realtime.Hub
	tokens         auth.TokenManager
	logger         shared.Logger
	maxConnPerUser int
	upgrader       websocket.Upgrader
}

// NewHandler builds the WS handler.
func NewHandler(hub *realtime.Hub, tokens auth.TokenManager, logger shared.Logger, maxConnPerUser int) *Handler {
	return &Handler{
		hub:            hub,
		tokens:         tokens,
		logger:         logger,
		maxConnPerUser: maxConnPerUser,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// Origin is enforced by the CORS layer / deployment; the JWT is the
			// real authorization here.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// ServeHTTP authenticates the request, enforces the connection limit, upgrades
// the socket and starts the pumps.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		http.Error(w, "missing access token", http.StatusUnauthorized)
		return
	}
	claims, err := h.tokens.VerifyAccess(token)
	if err != nil {
		http.Error(w, "invalid access token", http.StatusUnauthorized)
		return
	}

	client := &realtime.Client{
		ID:       shared.NewID(),
		TenantID: claims.TenantID,
		UserID:   claims.UserID,
		Send:     make(chan realtime.Message, sendBufferSize),
		Topics:   map[realtime.Topic]struct{}{},
	}

	// Enforce the per-user connection limit before upgrading.
	if !h.hub.Register(client, h.maxConnPerUser) {
		http.Error(w, "connection limit reached", http.StatusTooManyRequests)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.hub.Remove(client) // release the slot reserved by Register
		h.logger.Error("ws upgrade failed", "error", err.Error())
		return
	}

	// Auth context for subscribe-time permission checks.
	ac := authz.NewAuthContext(claims.TenantID, claims.UserID, claims.Permissions, claims.SectorIDs, claims.SectorScope)

	h.joinDefaultRooms(client, claims)
	h.logger.Info("ws connected",
		"client_id", client.ID,
		"tenant_id", client.TenantID,
		"user_id", client.UserID,
		"sectors", len(claims.SectorIDs),
	)

	go h.writePump(conn, client)
	h.readPump(conn, client, ac)
}

// joinDefaultRooms subscribes the client to the rooms it always receives:
// tenant-wide, its own user room, the presence board, and one room per allowed
// sector.
func (h *Handler) joinDefaultRooms(client *realtime.Client, claims auth.AccessClaims) {
	h.hub.Subscribe(client, shared.TopicTenant(claims.TenantID))
	h.hub.Subscribe(client, shared.TopicUser(claims.TenantID, claims.UserID))
	h.hub.Subscribe(client, shared.TopicPresence(claims.TenantID))
	for _, sectorID := range claims.SectorIDs {
		if sectorID != "" {
			h.hub.Subscribe(client, shared.TopicInbox(claims.TenantID, sectorID))
		}
	}
}

// readPump processes inbound control frames and cleans up on disconnect.
func (h *Handler) readPump(conn *websocket.Conn, client *realtime.Client, ac authz.AuthContext) {
	defer func() {
		h.hub.Remove(client)
		_ = conn.Close()
		h.logger.Info("ws disconnected", "client_id", client.ID, "user_id", client.UserID)
	}()

	conn.SetReadLimit(maxReadBytes)
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
		if json.Unmarshal(raw, &cmd) != nil || cmd.ConversationID == "" {
			continue
		}
		// Conversation rooms are built server-side from the connection's tenant,
		// so a client can never subscribe outside its tenant.
		topic := shared.TopicConversation(client.TenantID, cmd.ConversationID)
		switch cmd.Action {
		case "subscribe":
			if !ac.Has(authz.ConversationRead) {
				continue // silently ignore unauthorized subscribe
			}
			h.hub.Subscribe(client, topic)
		case "unsubscribe":
			h.hub.Unsubscribe(client, topic)
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

// extractToken reads the JWT from the Authorization header or the token query
// param (browsers cannot set headers on a WebSocket handshake).
func extractToken(r *http.Request) string {
	const prefix = "Bearer "
	if h := r.Header.Get("Authorization"); len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return r.URL.Query().Get("token")
}

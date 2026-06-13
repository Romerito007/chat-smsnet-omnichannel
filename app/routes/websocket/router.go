// Package websocket mounts the realtime WebSocket endpoint onto a router.
package websocket

import (
	"net/http"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
	ws "github.com/romerito007/chat-smsnet-omnichannel/presenter/websocket"
)

// Handler builds the realtime WS handler, wrapped with recover + request-id so
// the handshake is observable. The handler authenticates the upgrade via JWT
// (Authorization: Bearer, or ?token= for browsers that cannot set headers); the
// tenant comes only from the verified token, never a header. The server wiring
// exposes it at both /realtime/ws (canonical) and /ws (alias).
func Handler(c *container.Container) http.Handler {
	h := ws.NewHandler(c.Realtime.Hub, c.Tokens, c.Logger, c.Config.Realtime.MaxConnPerUser)
	return middleware.Recover(c.Logger)(middleware.RequestID(c.Logger, c.Config.LogRequestBody)(h))
}

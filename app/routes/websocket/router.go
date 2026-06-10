// Package websocket mounts the realtime WebSocket endpoint onto a router.
package websocket

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
	ws "github.com/romerito007/chat-smsnet-omnichannel/presenter/websocket"
)

// NewRouter builds the WS router. The handler authenticates the upgrade via JWT
// and bridges sockets to the realtime Hub owned by the container's manager.
// There is intentionally no tenant header middleware: the tenant comes only from
// the verified token.
func NewRouter(c *container.Container) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recover(c.Logger))
	r.Use(middleware.RequestID(c.Logger))

	handler := ws.NewHandler(c.Realtime.Hub, c.Tokens, c.Logger, c.Config.Realtime.MaxConnPerUser)
	r.Handle("/ws", handler)
	return r
}

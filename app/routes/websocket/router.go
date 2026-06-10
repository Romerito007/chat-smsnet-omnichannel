// Package websocket mounts the realtime WebSocket endpoint onto a router.
package websocket

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
	ws "github.com/romerito007/chat-smsnet-omnichannel/presenter/websocket"
)

// NewRouter builds the WS router. The handler bridges sockets to the realtime
// Hub owned by the container's realtime Manager.
func NewRouter(c *container.Container) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recover(c.Logger))
	r.Use(middleware.RequestID(c.Logger))
	r.Use(middleware.TenantContext)

	handler := ws.NewHandler(c.Realtime.Hub, c.Logger)
	r.Handle("/ws", handler)
	return r
}

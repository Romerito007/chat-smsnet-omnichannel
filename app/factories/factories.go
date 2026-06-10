// Package factories assembles concrete components (repositories, services,
// controllers, adapters) from the container's shared dependencies. Each domain
// gets its own factory function here as it is implemented; the foundation wires
// only the health controller.
package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/health"
	presenterhttp "github.com/romerito007/chat-smsnet-omnichannel/presenter/http"
)

// HealthHandler builds the health handler from the container.
func HealthHandler(c *container.Container) *presenterhttp.HealthHandler {
	checker := health.NewChecker(c.Mongo, c.Redis)
	return presenterhttp.NewHealthHandler(checker)
}

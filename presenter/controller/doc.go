// Package controller is the root for HTTP/WS controllers. Each domain gets its
// own subpackage (presenter/controller/<domain>) that decodes DTOs, calls the
// domain service and renders responses via presenter/middleware helpers. Empty
// in the foundation.
package controller

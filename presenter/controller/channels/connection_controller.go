// Package channels holds the HTTP controllers for channel connection management,
// the inbound endpoint and delivery receipts.
package channels

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	channelservice "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/channels"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// ConnectionController serves CRUD + test for channel connections.
type ConnectionController struct {
	connections *channelservice.ConnectionService
}

// NewConnectionController builds the controller.
func NewConnectionController(connections *channelservice.ConnectionService) *ConnectionController {
	return &ConnectionController{connections: connections}
}

// Create handles POST /v1/channels.
func (c *ConnectionController) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateConnectionRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	conn, err := c.connections.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	// Creation is the only response that reveals the inbound_token and the
	// outbound_secret; both are masked on every subsequent read.
	middleware.WriteJSON(w, http.StatusCreated, dto.NewCreatedConnectionResponse(conn))
}

// List handles GET /v1/channels.
func (c *ConnectionController) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.connections.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewConnectionResponses(items), page.Limit, func(it dto.ConnectionResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: it.CreatedAt.UnixMilli(), ID: it.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Get handles GET /v1/channels/{id}.
func (c *ConnectionController) Get(w http.ResponseWriter, r *http.Request) {
	conn, err := c.connections.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConnectionResponse(conn))
}

// Update handles PATCH /v1/channels/{id}.
func (c *ConnectionController) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateConnectionRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	conn, err := c.connections.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConnectionResponse(conn))
}

// Delete handles DELETE /v1/channels/{id}.
func (c *ConnectionController) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.connections.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RotateInboundToken handles POST /v1/channels/{id}/rotate-inbound-token. It
// issues a fresh integration token (revoking the prior one) and returns it once.
func (c *ConnectionController) RotateInboundToken(w http.ResponseWriter, r *http.Request) {
	conn, err := c.connections.RotateInboundToken(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewRotatedInboundTokenResponse(conn))
}

// RotateOutboundSecret handles POST /v1/channels/{id}/rotate-outbound-secret. It
// issues a fresh outbound HMAC secret (invalidating the previous one) and returns
// it once; the managed webhook is re-synced to sign with the new secret. The
// integrator must switch to the new value.
func (c *ConnectionController) RotateOutboundSecret(w http.ResponseWriter, r *http.Request) {
	conn, err := c.connections.RotateOutboundSecret(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewRotatedOutboundSecretResponse(conn))
}

// RefreshTemplates handles POST /v1/channels/{id}/refresh-templates. It fetches the
// channel's current WhatsApp templates from its gateway (signed with the outbound
// secret) and replaces the stored render-only mirror, returning the updated channel
// so the front can re-render the selector. On a gateway failure the existing
// templates are kept and an error is returned.
func (c *ConnectionController) RefreshTemplates(w http.ResponseWriter, r *http.Request) {
	conn, err := c.connections.RefreshTemplates(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConnectionResponse(conn))
}

// Test handles POST /v1/channels/{id}/test.
func (c *ConnectionController) Test(w http.ResponseWriter, r *http.Request) {
	result, _, err := c.connections.Test(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, result)
}

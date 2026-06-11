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

// Test handles POST /v1/channels/{id}/test.
func (c *ConnectionController) Test(w http.ResponseWriter, r *http.Request) {
	result, _, err := c.connections.Test(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, result)
}

// Package mcp holds the HTTP controllers for MCP server management and the
// conversation-scoped tool/approval endpoints.
package mcp

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	mcpservice "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/mcp"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// ServerController serves CRUD + test for MCP servers.
type ServerController struct {
	servers *mcpservice.ServerService
}

// NewServerController builds the controller.
func NewServerController(servers *mcpservice.ServerService) *ServerController {
	return &ServerController{servers: servers}
}

// Create handles POST /v1/mcp/servers.
func (c *ServerController) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateServerRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	s, err := c.servers.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewServerResponse(s))
}

// List handles GET /v1/mcp/servers.
func (c *ServerController) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.servers.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewServerResponses(items), page.Limit, func(it dto.ServerResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: it.CreatedAt.UnixMilli(), ID: it.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Get handles GET /v1/mcp/servers/{id}.
func (c *ServerController) Get(w http.ResponseWriter, r *http.Request) {
	s, err := c.servers.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewServerResponse(s))
}

// Update handles PATCH /v1/mcp/servers/{id}.
func (c *ServerController) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateServerRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	s, err := c.servers.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewServerResponse(s))
}

// Delete handles DELETE /v1/mcp/servers/{id}.
func (c *ServerController) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.servers.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Test handles POST /v1/mcp/servers/{id}/test — lists the server's tools.
func (c *ServerController) Test(w http.ResponseWriter, r *http.Request) {
	tools, err := c.servers.Test(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"tools": dto.NewToolResponses(tools)})
}

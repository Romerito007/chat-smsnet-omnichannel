// Package providerhub holds the HTTP controllers for the providerhub config and
// the on-demand, by-conversation queries to the smsnet-integrations API.
package providerhub

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	phcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	phservice "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/providerhub"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves config management and on-demand provider queries.
type Controller struct {
	config  *phservice.ConfigService
	queries *phservice.QueryService
}

// NewController builds the controller.
func NewController(config *phservice.ConfigService, queries *phservice.QueryService) *Controller {
	return &Controller{config: config, queries: queries}
}

// ── config ───────────────────────────────────────────────────────────────────

// GetConfig handles GET /v1/providerhub/config.
func (c *Controller) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, source, err := c.config.Resolved(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConfigStatusResponse(cfg, source))
}

// CreateConfig handles POST /v1/providerhub/config.
func (c *Controller) CreateConfig(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateConfigRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	cfg, err := c.config.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewConfigResponse(cfg))
}

// UpdateConfig handles PATCH /v1/providerhub/config.
func (c *Controller) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateConfigRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	cfg, err := c.config.Update(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConfigResponse(cfg))
}

// TestConfig handles POST /v1/providerhub/config/test.
func (c *Controller) TestConfig(w http.ResponseWriter, r *http.Request) {
	result, err := c.config.Test(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, result)
}

// ── on-demand conversation queries ───────────────────────────────────────────

// Cliente handles GET /v1/conversations/{id}/external/cliente
// (query: cpfcnpj|phone|email, id_cliente?).
func (c *Controller) Cliente(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := phcontracts.ConsultaClienteRequest{
		CpfCnpj:   q.Get("cpfcnpj"),
		Phone:     q.Get("phone"),
		Email:     q.Get("email"),
		IDCliente: q.Get("id_cliente"),
	}
	res, err := c.queries.ConsultarCliente(r.Context(), chi.URLParam(r, "id"), req)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

// Planos handles GET /v1/conversations/{id}/external/planos.
func (c *Controller) Planos(w http.ResponseWriter, r *http.Request) {
	res, err := c.queries.ListarPlanos(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"data": res})
}

// Empresa handles GET /v1/conversations/{id}/external/empresa.
func (c *Controller) Empresa(w http.ResponseWriter, r *http.Request) {
	res, err := c.queries.DadosEmpresa(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

// Liberacao handles POST /v1/conversations/{id}/external/liberacao.
func (c *Controller) Liberacao(w http.ResponseWriter, r *http.Request) {
	var req dto.LiberacaoRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.queries.LiberarAcesso(r.Context(), chi.URLParam(r, "id"), req.IDCliente)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

// Chamado handles POST /v1/conversations/{id}/external/chamado.
func (c *Controller) Chamado(w http.ResponseWriter, r *http.Request) {
	var req dto.ChamadoRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.queries.AbrirChamado(r.Context(), chi.URLParam(r, "id"), req.IDCliente, req.Subject, req.Message)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, res)
}

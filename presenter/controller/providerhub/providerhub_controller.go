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

// Controller serves ISP-profile management, gateway status and on-demand provider
// queries.
type Controller struct {
	profiles *phservice.ProfileService
	queries  *phservice.QueryService
}

// NewController builds the controller.
func NewController(profiles *phservice.ProfileService, queries *phservice.QueryService) *Controller {
	return &Controller{profiles: profiles, queries: queries}
}

// ── catalog & gateway status ─────────────────────────────────────────────────

// Catalog handles GET /v1/providerhub/catalog: the static, versioned catalog of
// supported ISPs (per-ISP credential fields + supported actions), so the front
// renders the profile form and shows/hides external actions without hard-coding.
func (c *Controller) Catalog(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, dto.NewCatalogResponse())
}

// GetConfig handles GET /v1/providerhub/config: SMSNET gateway status (infra/env)
// plus a summary of the tenant's ISP profiles.
func (c *Controller) GetConfig(w http.ResponseWriter, r *http.Request) {
	st, err := c.profiles.GatewayStatus(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewGatewayStatusResponse(st))
}

// ── ISP profiles ─────────────────────────────────────────────────────────────

// ListProfiles handles GET /v1/providerhub/profiles.
func (c *Controller) ListProfiles(w http.ResponseWriter, r *http.Request) {
	ps, err := c.profiles.List(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewProfileListResponse(ps))
}

// CreateProfile handles POST /v1/providerhub/profiles.
func (c *Controller) CreateProfile(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateProfileRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.profiles.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewProfileResponse(p))
}

// GetProfile handles GET /v1/providerhub/profiles/{id}.
func (c *Controller) GetProfile(w http.ResponseWriter, r *http.Request) {
	p, err := c.profiles.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewProfileResponse(p))
}

// UpdateProfile handles PATCH /v1/providerhub/profiles/{id}.
func (c *Controller) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateProfileRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.profiles.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewProfileResponse(p))
}

// DeleteProfile handles DELETE /v1/providerhub/profiles/{id}.
func (c *Controller) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	if err := c.profiles.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetDefaultProfile handles POST /v1/providerhub/profiles/{id}/default.
func (c *Controller) SetDefaultProfile(w http.ResponseWriter, r *http.Request) {
	p, err := c.profiles.SetDefault(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewProfileResponse(p))
}

// TestProfile handles POST /v1/providerhub/profiles/{id}/test.
func (c *Controller) TestProfile(w http.ResponseWriter, r *http.Request) {
	result, err := c.profiles.Test(r.Context(), chi.URLParam(r, "id"))
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

// Package providerhub holds the HTTP controllers for the providerhub config and
// the on-demand, by-conversation queries to the smsnet-integrations API.
package providerhub

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	phservice "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
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

// writeExternalError maps the ISP-selection-required sentinel to a 200
// needs_isp_selection response (the agent must pick a profile); every other error
// goes through the standard error envelope.
func (c *Controller) writeExternalError(w http.ResponseWriter, r *http.Request, err error) {
	if sel, ok := phservice.AsISPSelectionRequired(err); ok {
		middleware.WriteJSON(w, http.StatusOK, dto.NewNeedsISPSelectionResponse(sel.Eligible))
		return
	}
	middleware.WriteError(w, r, err)
}

// idempotencyKey returns the request's Idempotency-Key header, or a generated one
// so side-effect calls always forward a key to the gateway.
func idempotencyKey(r *http.Request) string {
	if k := r.Header.Get(middleware.HeaderIdempotencyKey); k != "" {
		return k
	}
	return shared.NewID()
}

// Cliente handles POST /v1/conversations/{id}/external/cliente
// (body: isp_config_id?, cpfcnpj|phone|email, id_cliente?).
func (c *Controller) Cliente(w http.ResponseWriter, r *http.Request) {
	var req dto.ClienteRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.queries.ConsultarCliente(r.Context(), chi.URLParam(r, "id"), req.ToRequest())
	if err != nil {
		c.writeExternalError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

// Planos handles POST /v1/conversations/{id}/external/planos (body: isp_config_id?).
func (c *Controller) Planos(w http.ResponseWriter, r *http.Request) {
	var req dto.ISPSelectorRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.queries.ListarPlanos(r.Context(), chi.URLParam(r, "id"), req.ISPConfigID)
	if err != nil {
		c.writeExternalError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"data": res})
}

// Empresa handles POST /v1/conversations/{id}/external/empresa (body: isp_config_id?).
func (c *Controller) Empresa(w http.ResponseWriter, r *http.Request) {
	var req dto.ISPSelectorRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.queries.DadosEmpresa(r.Context(), chi.URLParam(r, "id"), req.ISPConfigID)
	if err != nil {
		c.writeExternalError(w, r, err)
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
	res, err := c.queries.LiberarAcesso(r.Context(), chi.URLParam(r, "id"), req.ISPConfigID, req.IDCliente, idempotencyKey(r))
	if err != nil {
		c.writeExternalError(w, r, err)
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
	res, err := c.queries.AbrirChamado(r.Context(), chi.URLParam(r, "id"), req.ISPConfigID, req.IDCliente, req.Subject, req.Message, idempotencyKey(r))
	if err != nil {
		c.writeExternalError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, res)
}

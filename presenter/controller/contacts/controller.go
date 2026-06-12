// Package contacts holds the HTTP controller for the contact CRM endpoints.
package contacts

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	contactservice "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/contacts"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves contact reads and writes (CRM). Tenant-scoped via the token.
type Controller struct {
	contacts *contactservice.Service
}

// NewController builds the controller.
func NewController(contacts *contactservice.Service) *Controller {
	return &Controller{contacts: contacts}
}

// List handles GET /v1/contacts (cursor-paginated; ?q= filters).
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.contacts.List(r.Context(), r.URL.Query().Get("q"), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewContactResponses(items), page.Limit, func(it dto.ContactResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: it.CreatedAt.UnixMilli(), ID: it.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Create handles POST /v1/contacts (contact.write).
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateContactRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	contact, err := c.contacts.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewContactResponse(contact))
}

// Get handles GET /v1/contacts/{id} (contact.read). Tenant-scoped.
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	contact, err := c.contacts.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewContactResponse(contact))
}

// Update handles PATCH /v1/contacts/{id} (contact.write, partial).
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateContactRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	contact, err := c.contacts.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewContactResponse(contact))
}

// Package contacts holds the HTTP controller for reading contacts.
package contacts

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	contactservice "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/contacts"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves contact reads.
type Controller struct {
	contacts *contactservice.Service
}

// NewController builds the controller.
func NewController(contacts *contactservice.Service) *Controller {
	return &Controller{contacts: contacts}
}

// Get handles GET /v1/contacts/{id}. Tenant-scoped (the service requires a tenant
// from the token); requires contact.read.
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	contact, err := c.contacts.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewContactResponse(contact))
}

// Package contacts holds the HTTP controller for the contact CRM endpoints.
package contacts

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
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

// List handles GET /v1/contacts (cursor-paginated). Filters: ?q= free-text plus
// ?name=, ?phone= (substring) and ?tag_id= (exact), combinable with AND.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	q := r.URL.Query()
	filter := contracts.ListFilter{
		Query: q.Get("q"),
		Name:  q.Get("name"),
		Phone: q.Get("phone"),
		TagID: q.Get("tag_id"),
	}
	items, err := c.contacts.List(r.Context(), filter, page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	avatars := c.avatarURLs(r, items)
	resp := shared.NewPage(dto.NewContactResponsesWithAvatars(items, avatars), page.Limit, func(it dto.ContactResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: it.CreatedAt.UnixMilli(), ID: it.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// avatarURLs batch-resolves the signed avatar URLs for a page of contacts (keyed
// by avatar attachment id). Best-effort: a resolution hiccup never fails the read.
func (c *Controller) avatarURLs(r *http.Request, items []*entity.Contact) map[string]string {
	ids := make([]string, 0, len(items))
	for _, it := range items {
		if it.AvatarAttachmentID != "" {
			ids = append(ids, it.AvatarAttachmentID)
		}
	}
	urls, _ := c.contacts.AvatarURLs(r.Context(), ids)
	return urls
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
	middleware.WriteJSON(w, http.StatusCreated, dto.NewContactResponseWithAvatar(contact, c.avatarURLs(r, []*entity.Contact{contact})))
}

// Get handles GET /v1/contacts/{id} (contact.read). Tenant-scoped.
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	contact, err := c.contacts.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewContactResponseWithAvatar(contact, c.avatarURLs(r, []*entity.Contact{contact})))
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
	middleware.WriteJSON(w, http.StatusOK, dto.NewContactResponseWithAvatar(contact, c.avatarURLs(r, []*entity.Contact{contact})))
}

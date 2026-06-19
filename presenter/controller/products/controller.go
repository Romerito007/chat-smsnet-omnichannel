// Package products holds the HTTP controller for the product-catalog endpoints.
package products

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/products/contracts"
	productservice "github.com/romerito007/chat-smsnet-omnichannel/domain/products/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/products"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the product-catalog reads/writes. Tenant-scoped via the token.
type Controller struct {
	products *productservice.Service
}

// NewController builds the controller.
func NewController(products *productservice.Service) *Controller {
	return &Controller{products: products}
}

// List handles GET /v1/crm/products?q=&active= (deal.view). Empty when the products
// module is disabled.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	q := r.URL.Query()
	filter := ccontracts.ListFilter{Q: q.Get("q")}
	if v := q.Get("active"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			filter.Active = &b
		}
	}
	items, err := c.products.List(r.Context(), filter, page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	rows := dto.NewProductResponses(items)
	resp := shared.NewPage(rows, page.Limit, func(p dto.ProductResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: p.CreatedAt.UnixMilli(), ID: p.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Create handles POST /v1/crm/products (crm.manage).
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateProductRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.products.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewProductResponse(p))
}

// Update handles PATCH /v1/crm/products/{id} (crm.manage). Set active=false to
// deactivate (preferred over deleting, so deals keep referencing the product).
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateProductRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.products.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewProductResponse(p))
}

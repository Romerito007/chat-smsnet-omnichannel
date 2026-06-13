package copilot

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	cservice "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/copilot"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// AssistantController serves CRUD for copilot assistants (many per tenant).
type AssistantController struct {
	assistants *cservice.AssistantService
}

// NewAssistantController builds the controller.
func NewAssistantController(assistants *cservice.AssistantService) *AssistantController {
	return &AssistantController{assistants: assistants}
}

// List handles GET /v1/copilot/assistants.
func (c *AssistantController) List(w http.ResponseWriter, r *http.Request) {
	as, err := c.assistants.List(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewAssistantListResponse(as))
}

// Create handles POST /v1/copilot/assistants.
func (c *AssistantController) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateAssistantRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	a, err := c.assistants.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewAssistantResponse(a))
}

// Get handles GET /v1/copilot/assistants/{id}.
func (c *AssistantController) Get(w http.ResponseWriter, r *http.Request) {
	a, err := c.assistants.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewAssistantResponse(a))
}

// Update handles PATCH /v1/copilot/assistants/{id}.
func (c *AssistantController) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateAssistantRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	a, err := c.assistants.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewAssistantResponse(a))
}

// Delete handles DELETE /v1/copilot/assistants/{id}.
func (c *AssistantController) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.assistants.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

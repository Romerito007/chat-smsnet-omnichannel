// Package routing holds the HTTP controller for the routing endpoints.
package routing

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	routingservice "github.com/romerito007/chat-smsnet-omnichannel/domain/routing/service"
	convdto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/conversations"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/routing"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the routing endpoints.
type Controller struct {
	svc *routingservice.Service
}

// NewController builds the controller.
func NewController(svc *routingservice.Service) *Controller {
	return &Controller{svc: svc}
}

// Assign handles POST /v1/conversations/{id}/assign.
func (c *Controller) Assign(w http.ResponseWriter, r *http.Request) {
	var req dto.AssignRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	conv, err := c.svc.Assign(r.Context(), chi.URLParam(r, "id"), req.ToCommand().AgentID)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, convdto.NewConversationResponse(conv))
}

// Transfer handles POST /v1/conversations/{id}/transfer.
func (c *Controller) Transfer(w http.ResponseWriter, r *http.Request) {
	var req dto.TransferRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	conv, err := c.svc.Transfer(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, convdto.NewConversationResponse(conv))
}

// Enqueue handles POST /v1/conversations/{id}/enqueue.
func (c *Controller) Enqueue(w http.ResponseWriter, r *http.Request) {
	var req dto.EnqueueRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	conv, err := c.svc.Enqueue(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, convdto.NewConversationResponse(conv))
}

// Run handles POST /v1/routing/run. The body is optional (empty triggers a batch).
func (c *Controller) Run(w http.ResponseWriter, r *http.Request) {
	var req dto.RunRequest
	if err := decodeOptional(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	result, err := c.svc.Run(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, result)
}

// decodeOptional decodes a JSON body but tolerates an empty body.
func decodeOptional(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return apperror.Validation("invalid JSON body").Wrap(err)
	}
	return nil
}

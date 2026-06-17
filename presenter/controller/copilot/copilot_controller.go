// Package copilot holds the HTTP controllers for the copilot config and the
// inference endpoints.
package copilot

import (
	"net/http"

	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	cservice "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/copilot"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves copilot config management and inference.
type Controller struct {
	config  *cservice.ConfigService
	copilot *cservice.Service
}

// NewController builds the controller.
func NewController(config *cservice.ConfigService, copilot *cservice.Service) *Controller {
	return &Controller{config: config, copilot: copilot}
}

// GetConfig handles GET /v1/copilot/config.
func (c *Controller) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := c.config.Current(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConfigResponse(cfg))
}

// SaveConfig handles PATCH /v1/copilot/config (upsert).
func (c *Controller) SaveConfig(w http.ResponseWriter, r *http.Request) {
	var req dto.SaveConfigRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	cfg, err := c.config.Save(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConfigResponse(cfg))
}

// SuggestReply handles POST /v1/copilot/suggest-reply.
func (c *Controller) SuggestReply(w http.ResponseWriter, r *http.Request) {
	var req dto.SuggestReplyRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.copilot.SuggestReply(r.Context(), ccontracts.SuggestReplyInput{
		ConversationID: req.ConversationID,
		Instruction:    req.Instruction,
	})
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

// Ask handles POST /v1/copilot/ask — the agent-facing Q&A chat. The copilot answers
// the agent's question about the customer (using tools), keeping the front-supplied
// agent↔assistant history. Returns the same Result shape (text + proposed_actions).
func (c *Controller) Ask(w http.ResponseWriter, r *http.Request) {
	var req dto.AskRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.copilot.AgentChat(r.Context(), req.ToInput())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

// Summarize handles POST /v1/copilot/summarize.
func (c *Controller) Summarize(w http.ResponseWriter, r *http.Request) {
	var req dto.SummarizeRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.copilot.Summarize(r.Context(), ccontracts.SummarizeInput{ConversationID: req.ConversationID})
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

// Classify handles POST /v1/copilot/classify.
func (c *Controller) Classify(w http.ResponseWriter, r *http.Request) {
	var req dto.ClassifyRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.copilot.Classify(r.Context(), ccontracts.ClassifyInput{
		ConversationID: req.ConversationID,
		Categories:     req.Categories,
	})
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

// NextAction handles POST /v1/copilot/next-action.
func (c *Controller) NextAction(w http.ResponseWriter, r *http.Request) {
	var req dto.NextActionRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.copilot.NextAction(r.Context(), ccontracts.NextActionInput{ConversationID: req.ConversationID})
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

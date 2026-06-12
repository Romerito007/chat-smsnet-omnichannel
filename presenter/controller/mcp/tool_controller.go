package mcp

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	mcpcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/contracts"
	mcpservice "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/mcp"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// ToolController serves the conversation-scoped tool endpoints: discovery, manual
// run, and the approval decision for write actions.
type ToolController struct {
	tools *mcpservice.ToolService
}

// NewToolController builds the controller.
func NewToolController(tools *mcpservice.ToolService) *ToolController {
	return &ToolController{tools: tools}
}

// List handles GET /v1/conversations/{id}/mcp/tools.
func (c *ToolController) List(w http.ResponseWriter, r *http.Request) {
	tools, err := c.tools.Tools(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"tools": dto.NewToolResponses(tools)})
}

// Run handles POST /v1/conversations/{id}/mcp/run. A read tool runs directly; a
// write tool is recorded as a pending approval (never executed here).
func (c *ToolController) Run(w http.ResponseWriter, r *http.Request) {
	var req dto.RunToolRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.tools.Run(r.Context(), mcpcontracts.RunTool{
		ConversationID: chi.URLParam(r, "id"),
		ServerID:       req.ServerID,
		Tool:           req.Tool,
		Args:           req.Args,
	})
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	status := http.StatusOK
	if !res.Executed {
		status = http.StatusAccepted // pending approval
	}
	middleware.WriteJSON(w, status, res)
}

// ListToolCalls handles GET /v1/conversations/{id}/copilot/tool-calls. It returns
// the conversation's payload-free tool-call logs — 200 with an empty list when
// there are none (no 404), matching the SLA read.
func (c *ToolController) ListToolCalls(w http.ResponseWriter, r *http.Request) {
	logs, err := c.tools.ListCallLogs(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"data": dto.NewCallLogResponses(logs)})
}

// ListApprovals handles GET /v1/conversations/{id}/copilot/approvals. It returns
// the conversation's write-action approvals — 200 with an empty list when none.
func (c *ToolController) ListApprovals(w http.ResponseWriter, r *http.Request) {
	items, err := c.tools.ListApprovals(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"data": dto.NewApprovalResponses(items)})
}

// Decide handles POST /v1/conversations/{id}/copilot/approvals/{approvalID}.
// Approval triggers execution; rejection records the refusal. Both are audited.
func (c *ToolController) Decide(w http.ResponseWriter, r *http.Request) {
	var req dto.DecideRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.tools.Decide(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "approvalID"), req.Approve, req.Reason)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

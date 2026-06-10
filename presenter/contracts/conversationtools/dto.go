// Package conversationtools holds the request/response DTOs for the
// conversationtools endpoints (tags, canned responses, close reasons) and the
// conversation tag-apply endpoint.
package conversationtools

import (
	"time"

	ctcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/contracts"
	ctentity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/entity"
)

// ── tags ─────────────────────────────────────────────────────────────────────

type CreateTagRequest struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

func (r CreateTagRequest) ToCommand() ctcontracts.CreateTag {
	return ctcontracts.CreateTag{Name: r.Name, Color: r.Color, Description: r.Description, Enabled: r.Enabled}
}

type UpdateTagRequest struct {
	Name        *string `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
}

func (r UpdateTagRequest) ToCommand() ctcontracts.UpdateTag {
	return ctcontracts.UpdateTag{Name: r.Name, Color: r.Color, Description: r.Description, Enabled: r.Enabled}
}

type TagResponse struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	Color       string    `json:"color,omitempty"`
	Description string    `json:"description,omitempty"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewTagResponse(t *ctentity.Tag) TagResponse {
	return TagResponse{
		ID: t.ID, TenantID: t.TenantID, Name: t.Name, Color: t.Color,
		Description: t.Description, Enabled: t.Enabled, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func NewTagResponses(items []*ctentity.Tag) []TagResponse {
	out := make([]TagResponse, 0, len(items))
	for _, t := range items {
		out = append(out, NewTagResponse(t))
	}
	return out
}

// ── canned responses ─────────────────────────────────────────────────────────

type CreateCannedRequest struct {
	SectorIDs []string `json:"sector_ids"`
	Shortcut  string   `json:"shortcut"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Enabled   *bool    `json:"enabled"`
}

func (r CreateCannedRequest) ToCommand() ctcontracts.CreateCannedResponse {
	return ctcontracts.CreateCannedResponse{SectorIDs: r.SectorIDs, Shortcut: r.Shortcut, Title: r.Title, Body: r.Body, Enabled: r.Enabled}
}

type UpdateCannedRequest struct {
	SectorIDs *[]string `json:"sector_ids"`
	Shortcut  *string   `json:"shortcut"`
	Title     *string   `json:"title"`
	Body      *string   `json:"body"`
	Enabled   *bool     `json:"enabled"`
}

func (r UpdateCannedRequest) ToCommand() ctcontracts.UpdateCannedResponse {
	return ctcontracts.UpdateCannedResponse{SectorIDs: r.SectorIDs, Shortcut: r.Shortcut, Title: r.Title, Body: r.Body, Enabled: r.Enabled}
}

type CannedResponse struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	SectorIDs []string  `json:"sector_ids,omitempty"`
	Global    bool      `json:"global"`
	Shortcut  string    `json:"shortcut"`
	Title     string    `json:"title,omitempty"`
	Body      string    `json:"body"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewCannedResponse(c *ctentity.CannedResponse) CannedResponse {
	return CannedResponse{
		ID: c.ID, TenantID: c.TenantID, SectorIDs: c.SectorIDs, Global: c.Global(),
		Shortcut: c.Shortcut, Title: c.Title, Body: c.Body, Enabled: c.Enabled,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func NewCannedResponses(items []*ctentity.CannedResponse) []CannedResponse {
	out := make([]CannedResponse, 0, len(items))
	for _, c := range items {
		out = append(out, NewCannedResponse(c))
	}
	return out
}

// ── close reasons ────────────────────────────────────────────────────────────

type CreateCloseReasonRequest struct {
	Name         string `json:"name"`
	RequiresNote *bool  `json:"requires_note"`
	Enabled      *bool  `json:"enabled"`
}

func (r CreateCloseReasonRequest) ToCommand() ctcontracts.CreateCloseReason {
	return ctcontracts.CreateCloseReason{Name: r.Name, RequiresNote: r.RequiresNote, Enabled: r.Enabled}
}

type UpdateCloseReasonRequest struct {
	Name         *string `json:"name"`
	RequiresNote *bool   `json:"requires_note"`
	Enabled      *bool   `json:"enabled"`
}

func (r UpdateCloseReasonRequest) ToCommand() ctcontracts.UpdateCloseReason {
	return ctcontracts.UpdateCloseReason{Name: r.Name, RequiresNote: r.RequiresNote, Enabled: r.Enabled}
}

type CloseReasonResponse struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Name         string    `json:"name"`
	RequiresNote bool      `json:"requires_note"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func NewCloseReasonResponse(c *ctentity.CloseReason) CloseReasonResponse {
	return CloseReasonResponse{
		ID: c.ID, TenantID: c.TenantID, Name: c.Name, RequiresNote: c.RequiresNote,
		Enabled: c.Enabled, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func NewCloseReasonResponses(items []*ctentity.CloseReason) []CloseReasonResponse {
	out := make([]CloseReasonResponse, 0, len(items))
	for _, c := range items {
		out = append(out, NewCloseReasonResponse(c))
	}
	return out
}

// ── apply tags ───────────────────────────────────────────────────────────────

// ApplyTagsRequest is the body of POST /v1/conversations/{id}/tags.
type ApplyTagsRequest struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

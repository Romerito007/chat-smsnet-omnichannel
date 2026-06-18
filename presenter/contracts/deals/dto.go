// Package deals holds the request/response DTOs for the sales-deal endpoints.
package deals

import (
	"time"

	dcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/entity"
)

// ── requests ─────────────────────────────────────────────────────────────────

// CreateDealRequest is the body of POST /v1/deals.
type CreateDealRequest struct {
	Title             string     `json:"title"`
	Value             float64    `json:"value"`
	Currency          string     `json:"currency"`
	PipelineID        string     `json:"pipeline_id"`
	StageID           string     `json:"stage_id"`
	ContactID         string     `json:"contact_id"`
	AssignedTo        string     `json:"assigned_to"`
	SectorID          string     `json:"sector_id"`
	Source            string     `json:"source"`
	ExpectedCloseDate *time.Time `json:"expected_close_date"`
}

// ToCommand maps the request to the service command.
func (r CreateDealRequest) ToCommand() dcontracts.CreateDeal {
	return dcontracts.CreateDeal{
		Title: r.Title, Value: r.Value, Currency: r.Currency, PipelineID: r.PipelineID,
		StageID: r.StageID, ContactID: r.ContactID, AssignedTo: r.AssignedTo,
		SectorID: r.SectorID, Source: r.Source, ExpectedCloseDate: r.ExpectedCloseDate,
	}
}

// CreateFromConversationRequest is the body of POST /v1/deals/from-conversation.
type CreateFromConversationRequest struct {
	ConversationID string  `json:"conversation_id"`
	Title          string  `json:"title"`
	Value          float64 `json:"value"`
	Currency       string  `json:"currency"`
}

// ToCommand maps to the service command.
func (r CreateFromConversationRequest) ToCommand() dcontracts.CreateFromConversation {
	return dcontracts.CreateFromConversation{ConversationID: r.ConversationID, Title: r.Title, Value: r.Value, Currency: r.Currency}
}

// UpdateDealRequest is the body of PATCH /v1/deals/{id}. Nil = unchanged.
type UpdateDealRequest struct {
	Title             *string    `json:"title"`
	Value             *float64   `json:"value"`
	Currency          *string    `json:"currency"`
	AssignedTo        *string    `json:"assigned_to"`
	SectorID          *string    `json:"sector_id"`
	Source            *string    `json:"source"`
	ExpectedCloseDate *time.Time `json:"expected_close_date"`
	ClearExpectedDate bool       `json:"clear_expected_close_date"`
}

// ToCommand maps to the service command.
func (r UpdateDealRequest) ToCommand() dcontracts.UpdateDeal {
	return dcontracts.UpdateDeal{
		Title: r.Title, Value: r.Value, Currency: r.Currency, AssignedTo: r.AssignedTo,
		SectorID: r.SectorID, Source: r.Source, ExpectedCloseDate: r.ExpectedCloseDate,
		ClearExpectedDate: r.ClearExpectedDate,
	}
}

// MoveStageRequest is the body of PATCH /v1/deals/{id}/stage.
type MoveStageRequest struct {
	StageID string `json:"stage_id"`
}

// LinkConversationRequest is the body of POST /v1/deals/{id}/conversations.
type LinkConversationRequest struct {
	ConversationID string `json:"conversation_id"`
}

// MarkLostRequest is the body of POST /v1/deals/{id}/lost.
type MarkLostRequest struct {
	Reason string `json:"reason"`
}

// ── responses ────────────────────────────────────────────────────────────────

// DealResponse is the public representation of a deal. The *_name fields are
// read-only/derived, resolved in batch so the Kanban renders names not raw ids.
type DealResponse struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	PipelineID string `json:"pipeline_id"`
	// PipelineName / StageName are resolved in batch from the pipeline.
	PipelineName string  `json:"pipeline_name,omitempty"`
	StageID      string  `json:"stage_id"`
	StageName    string  `json:"stage_name,omitempty"`
	ContactID    string  `json:"contact_id,omitempty"`
	ContactName  string  `json:"contact_name,omitempty"`
	Title        string  `json:"title"`
	Value        float64 `json:"value"`
	Currency     string  `json:"currency"`
	AssignedTo   string  `json:"assigned_to,omitempty"`
	// AssignedToName / AssignedToAvatarURL are resolved in batch from assigned_to.
	AssignedToName      string     `json:"assigned_to_name,omitempty"`
	AssignedToAvatarURL string     `json:"assigned_to_avatar_url,omitempty"`
	SectorID            string     `json:"sector_id,omitempty"`
	ConversationIDs     []string   `json:"conversation_ids"`
	Source              string     `json:"source,omitempty"`
	Status              string     `json:"status"`
	LostReason          string     `json:"lost_reason,omitempty"`
	ExpectedCloseDate   *time.Time `json:"expected_close_date,omitempty"`
	StageChangedAt      time.Time  `json:"stage_changed_at"`
	ClosedAt            *time.Time `json:"closed_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// NewDealResponse maps a deal entity to its DTO (without enrichment).
func NewDealResponse(d *entity.Deal) DealResponse {
	conv := d.ConversationIDs
	if conv == nil {
		conv = []string{}
	}
	return DealResponse{
		ID: d.ID, TenantID: d.TenantID, PipelineID: d.PipelineID, StageID: d.StageID,
		ContactID: d.ContactID, Title: d.Title, Value: d.Value, Currency: d.Currency,
		AssignedTo: d.AssignedTo, SectorID: d.SectorID, ConversationIDs: conv,
		Source: d.Source, Status: string(d.Status), LostReason: d.LostReason,
		ExpectedCloseDate: d.ExpectedCloseDate, StageChangedAt: d.StageChangedAt,
		ClosedAt: d.ClosedAt, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

// NewDealResponses maps a slice.
func NewDealResponses(items []*entity.Deal) []DealResponse {
	out := make([]DealResponse, len(items))
	for i, d := range items {
		out[i] = NewDealResponse(d)
	}
	return out
}

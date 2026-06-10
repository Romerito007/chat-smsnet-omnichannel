// Package csat holds the request/response DTOs for the CSAT endpoints.
package csat

import (
	"time"

	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/csat/contracts"
	centity "github.com/romerito007/chat-smsnet-omnichannel/domain/csat/entity"
)

// ── surveys ──────────────────────────────────────────────────────────────────

type CreateSurveyRequest struct {
	Name         string   `json:"name"`
	Scale        string   `json:"scale"`
	QuestionText string   `json:"question_text"`
	SectorIDs    []string `json:"sector_ids"`
	DelaySeconds int      `json:"delay_seconds"`
	Enabled      *bool    `json:"enabled"`
}

func (r CreateSurveyRequest) ToCommand() ccontracts.CreateSurvey {
	return ccontracts.CreateSurvey{
		Name: r.Name, Scale: r.Scale, QuestionText: r.QuestionText,
		SectorIDs: r.SectorIDs, DelaySeconds: r.DelaySeconds, Enabled: r.Enabled,
	}
}

type UpdateSurveyRequest struct {
	Name         *string   `json:"name"`
	Scale        *string   `json:"scale"`
	QuestionText *string   `json:"question_text"`
	SectorIDs    *[]string `json:"sector_ids"`
	DelaySeconds *int      `json:"delay_seconds"`
	Enabled      *bool     `json:"enabled"`
}

func (r UpdateSurveyRequest) ToCommand() ccontracts.UpdateSurvey {
	return ccontracts.UpdateSurvey{
		Name: r.Name, Scale: r.Scale, QuestionText: r.QuestionText,
		SectorIDs: r.SectorIDs, DelaySeconds: r.DelaySeconds, Enabled: r.Enabled,
	}
}

type SurveyResponse struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Name         string    `json:"name"`
	Scale        string    `json:"scale"`
	QuestionText string    `json:"question_text"`
	SendOn       string    `json:"send_on"`
	SectorIDs    []string  `json:"sector_ids,omitempty"`
	DelaySeconds int       `json:"delay_seconds"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func NewSurveyResponse(s *centity.CSATSurvey) SurveyResponse {
	return SurveyResponse{
		ID: s.ID, TenantID: s.TenantID, Name: s.Name, Scale: string(s.Scale),
		QuestionText: s.QuestionText, SendOn: string(s.SendOn), SectorIDs: s.SectorIDs,
		DelaySeconds: s.DelaySeconds, Enabled: s.Enabled, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func NewSurveyResponses(items []*centity.CSATSurvey) []SurveyResponse {
	out := make([]SurveyResponse, 0, len(items))
	for _, s := range items {
		out = append(out, NewSurveyResponse(s))
	}
	return out
}

// ── responses (reporting) ────────────────────────────────────────────────────

// ResponseResponse is the reporting view of a CSAT response. It deliberately
// omits the public token.
type ResponseResponse struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversation_id"`
	ContactID      string     `json:"contact_id,omitempty"`
	SurveyID       string     `json:"survey_id"`
	AgentID        string     `json:"agent_id,omitempty"`
	Score          *int       `json:"score,omitempty"`
	Comment        string     `json:"comment,omitempty"`
	SentAt         *time.Time `json:"sent_at,omitempty"`
	RespondedAt    *time.Time `json:"responded_at,omitempty"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
}

func NewResponseResponse(r *centity.CSATResponse) ResponseResponse {
	return ResponseResponse{
		ID: r.ID, ConversationID: r.ConversationID, ContactID: r.ContactID,
		SurveyID: r.SurveyID, AgentID: r.AgentID, Score: r.Score, Comment: r.Comment,
		SentAt: r.SentAt, RespondedAt: r.RespondedAt, Status: string(r.Status), CreatedAt: r.CreatedAt,
	}
}

func NewResponseResponses(items []*centity.CSATResponse) []ResponseResponse {
	out := make([]ResponseResponse, 0, len(items))
	for _, r := range items {
		out = append(out, NewResponseResponse(r))
	}
	return out
}

// SubmitRequest is the public answer body for POST /v1/csat/responses/{token}.
type SubmitRequest struct {
	Score   int    `json:"score"`
	Comment string `json:"comment"`
}

func (r SubmitRequest) ToCommand() ccontracts.SubmitResponse {
	return ccontracts.SubmitResponse{Score: r.Score, Comment: r.Comment}
}

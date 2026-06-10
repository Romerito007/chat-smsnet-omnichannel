package models

import "time"

// CSATSurvey is the BSON document for a CSAT survey.
type CSATSurvey struct {
	Base         `bson:",inline"`
	Name         string   `bson:"name"`
	Scale        string   `bson:"scale"`
	QuestionText string   `bson:"question_text"`
	SendOn       string   `bson:"send_on"`
	SectorIDs    []string `bson:"sector_ids,omitempty"`
	DelaySeconds int      `bson:"delay_seconds,omitempty"`
	Enabled      bool     `bson:"enabled"`
}

// CSATResponse is the BSON document for a CSAT response.
type CSATResponse struct {
	ID             string     `bson:"_id"`
	TenantID       string     `bson:"tenant_id"`
	ConversationID string     `bson:"conversation_id"`
	ContactID      string     `bson:"contact_id,omitempty"`
	SurveyID       string     `bson:"survey_id"`
	AgentID        string     `bson:"agent_id,omitempty"`
	Token          string     `bson:"token"`
	Score          *int       `bson:"score,omitempty"`
	Comment        string     `bson:"comment,omitempty"`
	SentAt         *time.Time `bson:"sent_at,omitempty"`
	RespondedAt    *time.Time `bson:"responded_at,omitempty"`
	Status         string     `bson:"status"`
	CreatedAt      time.Time  `bson:"created_at"`
	UpdatedAt      time.Time  `bson:"updated_at"`
}

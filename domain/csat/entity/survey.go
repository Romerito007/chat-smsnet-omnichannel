// Package entity holds the CSAT domain entities: surveys and the per-conversation
// responses.
package entity

import "time"

// Scale is a survey's answer scale.
type Scale string

const (
	ScaleCSAT15 Scale = "csat_1_5" // 1..5
	ScaleNPS010 Scale = "nps_0_10" // 0..10
	ScaleThumbs Scale = "thumbs"   // 0 (down) | 1 (up)
)

// IsValidScale reports whether s is a known scale.
func IsValidScale(s Scale) bool {
	switch s {
	case ScaleCSAT15, ScaleNPS010, ScaleThumbs:
		return true
	default:
		return false
	}
}

// ValidScore reports whether score is valid for the scale.
func (s Scale) ValidScore(score int) bool {
	switch s {
	case ScaleCSAT15:
		return score >= 1 && score <= 5
	case ScaleNPS010:
		return score >= 0 && score <= 10
	case ScaleThumbs:
		return score == 0 || score == 1
	default:
		return false
	}
}

// SendOn enumerates the survey trigger. Only conversation_closed is supported.
type SendOn string

const SendOnConversationClosed SendOn = "conversation_closed"

// CSATSurvey is a tenant's configurable satisfaction survey.
type CSATSurvey struct {
	ID           string
	TenantID     string
	Name         string
	Scale        Scale
	QuestionText string
	SendOn       SendOn
	SectorIDs    []string
	DelaySeconds int
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Matches reports whether the survey applies to a conversation in the sector.
// An empty SectorIDs matches every sector.
func (s *CSATSurvey) Matches(sectorID string) bool {
	if len(s.SectorIDs) == 0 {
		return true
	}
	for _, id := range s.SectorIDs {
		if id == sectorID {
			return true
		}
	}
	return false
}

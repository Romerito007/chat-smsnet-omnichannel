package csat

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/csat"
)

type fakeContactDir struct{}

func (fakeContactDir) ContactCards(_ context.Context, _ []string) (map[string]shared.DisplayCard, error) {
	return map[string]shared.DisplayCard{"ct1": {Name: "Maria Silva"}}, nil
}

type fakeAgentDir struct{}

func (fakeAgentDir) AgentCards(_ context.Context, _ []string) (map[string]shared.DisplayCard, error) {
	return map[string]shared.DisplayCard{"ag1": {Name: "Ana", AvatarURL: "http://api/ana.png"}}, nil
}

type fakeSurveyDir struct{}

func (fakeSurveyDir) SurveyNames(_ context.Context, _ []string) (map[string]string, error) {
	return map[string]string{"sv1": "NPS pós-atendimento"}, nil
}

// Mirrors TestAgentsReport_EnrichesNames: resolved contact/agent/survey ids fill the
// names (+ agent avatar) in batch; unresolved ids keep only the raw id; the original
// ids are preserved.
func TestListResponses_EnrichesNames(t *testing.T) {
	c := NewController(nil, nil).SetDirectories(fakeContactDir{}, fakeAgentDir{}, fakeSurveyDir{})

	rows := []dto.ResponseResponse{
		{ID: "r1", ContactID: "ct1", AgentID: "ag1", SurveyID: "sv1"},
		{ID: "r2", ContactID: "ctX", AgentID: "agX", SurveyID: "svX"}, // unresolved
	}
	c.enrichResponses(context.Background(), rows)

	// r1: fully enriched, raw ids preserved.
	if rows[0].ContactName != "Maria Silva" {
		t.Errorf("contact_name = %q, want Maria Silva", rows[0].ContactName)
	}
	if rows[0].AgentName != "Ana" || rows[0].AgentAvatarURL != "http://api/ana.png" {
		t.Errorf("agent not enriched: name=%q avatar=%q", rows[0].AgentName, rows[0].AgentAvatarURL)
	}
	if rows[0].SurveyName != "NPS pós-atendimento" {
		t.Errorf("survey_name = %q", rows[0].SurveyName)
	}
	if rows[0].ContactID != "ct1" || rows[0].AgentID != "ag1" || rows[0].SurveyID != "sv1" {
		t.Errorf("raw ids must be preserved: %+v", rows[0])
	}

	// r2: unresolved → names stay empty (row carries only the id).
	if rows[1].ContactName != "" || rows[1].AgentName != "" || rows[1].AgentAvatarURL != "" || rows[1].SurveyName != "" {
		t.Errorf("unresolved row must keep only the ids: %+v", rows[1])
	}
}

// With no directories wired the rows pass through untouched (best-effort, no panic).
func TestEnrichResponses_NoDirectoriesIsSafe(t *testing.T) {
	c := NewController(nil, nil)
	rows := []dto.ResponseResponse{{ID: "r1", ContactID: "ct1", AgentID: "ag1", SurveyID: "sv1"}}
	c.enrichResponses(context.Background(), rows)
	if rows[0].ContactName != "" || rows[0].AgentName != "" || rows[0].SurveyName != "" {
		t.Errorf("without directories the rows must carry only ids: %+v", rows[0])
	}
}

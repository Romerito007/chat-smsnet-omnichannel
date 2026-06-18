package deals

import (
	"context"
	"testing"

	pipelineentity "github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/deals"
)

type fakeContactDir struct{}

func (fakeContactDir) ContactCards(_ context.Context, _ []string) (map[string]shared.DisplayCard, error) {
	return map[string]shared.DisplayCard{"ct1": {Name: "Maria Silva"}}, nil
}

type fakeAgentDir struct{ calls int }

func (d *fakeAgentDir) AgentCards(_ context.Context, _ []string) (map[string]shared.DisplayCard, error) {
	d.calls++
	return map[string]shared.DisplayCard{"u9": {Name: "Ana", AvatarURL: "http://cdn/ana.png"}}, nil
}

type fakePipelineDir struct{ calls int }

func (d *fakePipelineDir) List(_ context.Context) ([]*pipelineentity.Pipeline, error) {
	d.calls++
	return []*pipelineentity.Pipeline{{
		ID: "p1", Name: "Vendas",
		Stages: []pipelineentity.Stage{{ID: "s1", Name: "Novo lead"}, {ID: "sw", Name: "Ganho"}},
	}}, nil
}

// Mirrors TestAgentsReport_EnrichesNames: resolved contact/agent/pipeline/stage ids
// fill names (+ avatar); unresolved ids stay id-only; resolution is in batch (one
// call per directory).
func TestEnrichDeals_FillsNamesInBatch(t *testing.T) {
	agentDir := &fakeAgentDir{}
	pipeDir := &fakePipelineDir{}
	c := NewController(nil).SetDirectories(fakeContactDir{}, agentDir, pipeDir)

	rows := []dto.DealResponse{
		{ID: "d1", ContactID: "ct1", AssignedTo: "u9", PipelineID: "p1", StageID: "s1"},
		{ID: "d2", ContactID: "ctX", AssignedTo: "uX", PipelineID: "pX", StageID: "sX"}, // unresolved
	}
	c.enrich(context.Background(), rows)

	if agentDir.calls != 1 || pipeDir.calls != 1 {
		t.Fatalf("each directory must be hit once (batch): agent=%d pipe=%d", agentDir.calls, pipeDir.calls)
	}
	// d1 fully enriched, raw ids preserved.
	if rows[0].ContactName != "Maria Silva" {
		t.Errorf("contact_name = %q", rows[0].ContactName)
	}
	if rows[0].AssignedToName != "Ana" || rows[0].AssignedToAvatarURL != "http://cdn/ana.png" {
		t.Errorf("assigned_to not enriched: %+v", rows[0])
	}
	if rows[0].PipelineName != "Vendas" || rows[0].StageName != "Novo lead" {
		t.Errorf("pipeline/stage not labeled: pipeline=%q stage=%q", rows[0].PipelineName, rows[0].StageName)
	}
	if rows[0].ContactID != "ct1" || rows[0].PipelineID != "p1" || rows[0].StageID != "s1" {
		t.Errorf("raw ids must be preserved: %+v", rows[0])
	}
	// d2 unresolved → all names empty.
	if rows[1].ContactName != "" || rows[1].AssignedToName != "" || rows[1].PipelineName != "" || rows[1].StageName != "" {
		t.Errorf("unresolved row must keep only ids: %+v", rows[1])
	}
}

func TestEnrichDeals_NoDirectoriesIsSafe(t *testing.T) {
	c := NewController(nil)
	rows := []dto.DealResponse{{ID: "d1", ContactID: "ct1", AssignedTo: "u9", PipelineID: "p1", StageID: "s1"}}
	c.enrich(context.Background(), rows)
	if rows[0].ContactName != "" || rows[0].PipelineName != "" {
		t.Errorf("without directories the rows must carry only ids: %+v", rows[0])
	}
}

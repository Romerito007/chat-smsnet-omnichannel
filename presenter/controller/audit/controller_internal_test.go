package audit

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/audit"
)

type fakeAgentDir struct{ calls int }

func (f *fakeAgentDir) AgentCards(_ context.Context, _ []string) (map[string]shared.DisplayCard, error) {
	f.calls++
	return map[string]shared.DisplayCard{"u1": {Name: "Ana"}}, nil
}

// Mirrors TestAgentsReport_EnrichesNames: resolved actor ids fill actor_name in ONE
// batch call; unresolved/non-user actors keep only the id; the raw id is preserved.
func TestEnrichActors_FillsNameInBatch(t *testing.T) {
	dir := &fakeAgentDir{}
	c := NewController(nil).SetDirectories(dir)

	rows := []dto.AuditLogResponse{
		{ID: "l1", ActorID: "u1", ActorType: "user"},
		{ID: "l2", ActorID: "u1", ActorType: "user"},       // duplicate id → deduped
		{ID: "l3", ActorID: "system", ActorType: "system"}, // non-user → unresolved
	}
	c.enrichActors(context.Background(), rows)

	if dir.calls != 1 {
		t.Fatalf("must resolve in ONE batch call, got %d", dir.calls)
	}
	if rows[0].ActorName != "Ana" || rows[1].ActorName != "Ana" {
		t.Errorf("user actor must be named: %+v", rows[:2])
	}
	if rows[0].ActorID != "u1" {
		t.Errorf("raw actor id must be preserved, got %q", rows[0].ActorID)
	}
	if rows[2].ActorName != "" {
		t.Errorf("non-user actor must stay nameless: %+v", rows[2])
	}
}

// Without a directory the rows pass through with only the id (best-effort, no panic).
func TestEnrichActors_NoDirectoryIsSafe(t *testing.T) {
	c := NewController(nil)
	rows := []dto.AuditLogResponse{{ID: "l1", ActorID: "u1"}}
	c.enrichActors(context.Background(), rows)
	if rows[0].ActorName != "" {
		t.Errorf("without a directory actor_name must stay empty: %+v", rows[0])
	}
}

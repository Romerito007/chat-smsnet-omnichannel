package service

import (
	"context"
	"strings"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fakeAgentDir struct{ calls int }

func (d *fakeAgentDir) AgentCards(_ context.Context, ids []string) (map[string]shared.DisplayCard, error) {
	d.calls++
	out := map[string]shared.DisplayCard{}
	for _, id := range ids {
		if id == "u1" {
			out[id] = shared.DisplayCard{Name: "Ana", AvatarURL: "http://cdn/ana.png"}
		}
	}
	return out, nil
}

type fakeNameDir struct {
	calls int
	names map[string]string
}

func (d *fakeNameDir) Names(_ context.Context, ids []string) (map[string]string, error) {
	d.calls++
	out := map[string]string{}
	for _, id := range ids {
		if n, ok := d.names[id]; ok {
			out[id] = n
		}
	}
	return out, nil
}

func enrichService(t *testing.T) (*Service, *fakeRepo, *fakeAgentDir, *fakeNameDir, *fakeNameDir) {
	repo := &fakeRepo{
		agents:    []contracts.AgentStat{{AgentID: "u1", Conversations: 3}, {AgentID: "u2", Conversations: 1}},
		sectors:   []contracts.SectorStat{{SectorID: "s1", Conversations: 4}, {SectorID: "sX", Conversations: 1}},
		byStatus:  []contracts.Bucket{{Key: "open", Count: 5}},
		bySector:  []contracts.Bucket{{Key: "s1", Count: 4}, {Key: "sX", Count: 1}},
		byChannel: []contracts.Bucket{{Key: "whatsapp", Count: 9}},
		closedBy:  []contracts.Bucket{{Key: "r1", Count: 3}, {Key: "rX", Count: 1}},
	}
	agents := &fakeAgentDir{}
	sectors := &fakeNameDir{names: map[string]string{"s1": "Suporte"}}
	reasons := &fakeNameDir{names: map[string]string{"r1": "Resolvido"}}
	svc := NewService(repo, clk(t)).SetDirectories(agents, sectors, reasons)
	return svc, repo, agents, sectors, reasons
}

// Agents resolves the agent name + avatar in ONE batch call; an unresolved agent
// keeps only the id.
func TestAgents_EnrichesNameAndAvatarInBatch(t *testing.T) {
	svc, _, agents, _, _ := enrichService(t)
	rep, err := svc.Agents(ctxT(), contracts.Filter{})
	if err != nil {
		t.Fatalf("agents: %v", err)
	}
	if agents.calls != 1 {
		t.Fatalf("must resolve in ONE batch call, got %d", agents.calls)
	}
	if rep.Agents[0].Name != "Ana" || rep.Agents[0].AvatarURL != "http://cdn/ana.png" {
		t.Errorf("agent not enriched: %+v", rep.Agents[0])
	}
	if rep.Agents[0].AgentID != "u1" {
		t.Errorf("raw agent id must be preserved: %+v", rep.Agents[0])
	}
	if rep.Agents[1].Name != "" {
		t.Errorf("unresolved agent must stay nameless: %+v", rep.Agents[1])
	}
}

func TestSectors_EnrichesName(t *testing.T) {
	svc, _, _, sectors, _ := enrichService(t)
	rep, err := svc.Sectors(ctxT(), contracts.Filter{})
	if err != nil {
		t.Fatalf("sectors: %v", err)
	}
	if sectors.calls != 1 {
		t.Fatalf("must resolve in ONE batch call, got %d", sectors.calls)
	}
	if rep.Sectors[0].Name != "Suporte" || rep.Sectors[1].Name != "" {
		t.Errorf("sector not enriched / unresolved leaked a name: %+v", rep.Sectors)
	}
}

// Conversations labels the id-keyed buckets (by_sector, closed_by_reason); unresolved
// ids stay label-less and the already-readable status/channel buckets are untouched.
func TestConversations_LabelsBuckets(t *testing.T) {
	svc, _, _, _, _ := enrichService(t)
	rep, err := svc.Conversations(ctxT(), contracts.Filter{})
	if err != nil {
		t.Fatalf("conversations: %v", err)
	}
	if rep.BySector[0].Key != "s1" || rep.BySector[0].Label != "Suporte" || rep.BySector[1].Label != "" {
		t.Errorf("by_sector not labeled correctly: %+v", rep.BySector)
	}
	if rep.ClosedByReason[0].Key != "r1" || rep.ClosedByReason[0].Label != "Resolvido" || rep.ClosedByReason[1].Label != "" {
		t.Errorf("closed_by_reason not labeled correctly: %+v", rep.ClosedByReason)
	}
	if rep.ByStatus[0].Label != "" || rep.ByChannel[0].Label != "" {
		t.Errorf("status/channel buckets must not be labeled: %+v %+v", rep.ByStatus, rep.ByChannel)
	}
}

// The EXPORTED file (the key deliverable) carries the resolved names/labels too, in
// both JSON and CSV — consistent with the GET reads.
func TestExport_CarriesResolvedNames(t *testing.T) {
	svc, _, _, _, _ := enrichService(t)
	files := newMemFileStore()
	svc.SetFileStore(files, 0)

	// agents → JSON contains the resolved name.
	if _, err := svc.Export(ctxT(), "agents", "json", contracts.Filter{}); err != nil {
		t.Fatalf("export agents json: %v", err)
	}
	if !savedContains(files, "Ana") {
		t.Errorf("exported agents JSON must contain the resolved agent name")
	}

	// conversations → CSV contains the resolved sector + reason labels.
	files2 := newMemFileStore()
	svc.SetFileStore(files2, 0)
	if _, err := svc.Export(ctxT(), "conversations", "csv", contracts.Filter{}); err != nil {
		t.Fatalf("export conversations csv: %v", err)
	}
	if !savedContains(files2, "Suporte") || !savedContains(files2, "Resolvido") {
		t.Errorf("exported conversations CSV must contain the resolved sector/reason labels")
	}
}

func savedContains(m *memFileStore, want string) bool {
	for _, data := range m.saved {
		if strings.Contains(string(data), want) {
			return true
		}
	}
	return false
}

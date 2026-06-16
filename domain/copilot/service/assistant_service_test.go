package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	centity "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	mcpentity "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
	mcprepo "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/repository"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// fakeChannelsRepo satisfies the channels ConnectionRepository; its methods are
// never called in these tests (assistants are created with no channels).
type fakeChannelsRepo struct{ chrepo.ConnectionRepository }

// fakeMCPServers satisfies the mcp ServerRepository, resolving only known ids.
type fakeMCPServers struct {
	mcprepo.ServerRepository
	known map[string]bool
}

func (r fakeMCPServers) FindByID(_ context.Context, id string) (*mcpentity.ServerConnection, error) {
	if r.known[id] {
		return &mcpentity.ServerConnection{ID: id, Name: "Custom", Kind: mcpentity.KindRead, Enabled: true}, nil
	}
	return nil, apperror.NotFound("nf")
}

func newAssistantSvc(servers map[string]bool) (*AssistantService, *fakeAssistantRepo) {
	arepo := &fakeAssistantRepo{byID: map[string]*centity.Assistant{}}
	svc := NewAssistantService(arepo, &fakeProfileRepo{byID: nil}, fakeChannelsRepo{}, fakeMCPServers{known: servers}, nil)
	return svc, arepo
}

func aCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }

func TestAssistantCreate_BothSourcesRejected(t *testing.T) {
	svc, _ := newAssistantSvc(map[string]bool{"srv1": true})
	_, err := svc.Create(aCtx(), CreateAssistant{Name: "A", ISPProfileID: "isp1", MCPServerID: "srv1"})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("ISP + MCP together must be a validation error, got %v", err)
	}
}

func TestAssistantCreate_UnknownMCPServerRejected(t *testing.T) {
	svc, _ := newAssistantSvc(map[string]bool{}) // nothing exists
	_, err := svc.Create(aCtx(), CreateAssistant{Name: "A", MCPServerID: "ghost"})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("unknown mcp_server_id must be a validation error, got %v", err)
	}
}

func TestAssistantCreate_MCPServerAccepted(t *testing.T) {
	svc, _ := newAssistantSvc(map[string]bool{"srv1": true})
	a, err := svc.Create(aCtx(), CreateAssistant{Name: "A", MCPServerID: "srv1"})
	if err != nil {
		t.Fatalf("a known mcp server must be accepted: %v", err)
	}
	if a.MCPServerID != "srv1" || a.ISPProfileID != "" {
		t.Errorf("expected mcp source only, got %+v", a)
	}
}

func TestAssistantUpdate_SwitchToMCPClearsExclusivity(t *testing.T) {
	svc, repo := newAssistantSvc(map[string]bool{"srv1": true})
	repo.byID["a1"] = &centity.Assistant{ID: "a1", TenantID: "t1", Name: "A", ISPProfileID: "isp1", Enabled: true}
	// PATCH sets mcp_server and clears isp_profile in the same request → valid.
	empty := ""
	srv := "srv1"
	a, err := svc.Update(aCtx(), "a1", UpdateAssistant{ISPProfileID: &empty, MCPServerID: &srv})
	if err != nil {
		t.Fatalf("switching source must be allowed: %v", err)
	}
	if a.MCPServerID != "srv1" || a.ISPProfileID != "" {
		t.Errorf("expected switch to mcp source, got %+v", a)
	}
}

func TestAssistant_IsMCPServerInUse(t *testing.T) {
	svc, repo := newAssistantSvc(map[string]bool{"srv1": true})
	repo.byID["a1"] = &centity.Assistant{ID: "a1", TenantID: "t1", Name: "Atendimento", MCPServerID: "srv1"}
	inUse, usedBy, err := svc.IsMCPServerInUse(aCtx(), "srv1")
	if err != nil {
		t.Fatalf("in-use check: %v", err)
	}
	if !inUse || usedBy != "Atendimento" {
		t.Errorf("expected server in use by Atendimento, got inUse=%v usedBy=%q", inUse, usedBy)
	}
	if free, _, _ := svc.IsMCPServerInUse(aCtx(), "other"); free {
		t.Error("an unreferenced server must not be reported in use")
	}
}

// newAssistantSvcWithProfile builds the service with one pinned ISP profile so the
// per-write-operation mode validation (keys ⊆ enabled write actions) can be tested.
func newAssistantSvcWithProfile(profile *phentity.ISPProfile) (*AssistantService, *fakeAssistantRepo) {
	arepo := &fakeAssistantRepo{byID: map[string]*centity.Assistant{}}
	prepo := &fakeProfileRepo{byID: map[string]*phentity.ISPProfile{}}
	if profile != nil {
		prepo.byID[profile.ID] = profile
	}
	svc := NewAssistantService(arepo, prepo, fakeChannelsRepo{}, fakeMCPServers{known: map[string]bool{}}, nil)
	return svc, arepo
}

func TestAssistantWriteModes_AutomaticOnEnabledWriteAction(t *testing.T) {
	// ixcsoft offers liberacao + chamado; enable both.
	profile := &phentity.ISPProfile{ID: "isp1", TenantID: "t1", ISPType: phentity.ISPIXCSoft, Enabled: true,
		Transports: []string{phentity.TransportMCP}, EnabledActions: []string{"cliente", "planos", "empresa", "liberacao", "chamado"}}
	svc, _ := newAssistantSvcWithProfile(profile)

	a, err := svc.Create(aCtx(), CreateAssistant{Name: "A", ISPProfileID: "isp1", Transport: "mcp",
		WriteModes: map[string]string{"liberacao": centity.WriteModeAuto}})
	if err != nil {
		t.Fatalf("automatic mode on an enabled write action must be accepted: %v", err)
	}
	if a.WriteModes["liberacao"] != centity.WriteModeAuto {
		t.Errorf("write mode not stored: %+v", a.WriteModes)
	}
}

func TestAssistantWriteModes_RejectsActionNotOffered(t *testing.T) {
	// Profile offers only liberacao among writes (chamado unchecked).
	profile := &phentity.ISPProfile{ID: "isp1", TenantID: "t1", ISPType: phentity.ISPIXCSoft, Enabled: true,
		Transports: []string{phentity.TransportMCP}, EnabledActions: []string{"cliente", "liberacao"}}
	svc, _ := newAssistantSvcWithProfile(profile)

	_, err := svc.Create(aCtx(), CreateAssistant{Name: "A", ISPProfileID: "isp1", Transport: "mcp",
		WriteModes: map[string]string{"chamado": centity.WriteModeAuto}})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("a mode for a non-offered action must be a validation error, got %v", err)
	}
}

func TestAssistantWriteModes_RejectsUnknownMode(t *testing.T) {
	profile := &phentity.ISPProfile{ID: "isp1", TenantID: "t1", ISPType: phentity.ISPIXCSoft, Enabled: true,
		Transports: []string{phentity.TransportMCP}, EnabledActions: []string{"liberacao"}}
	svc, _ := newAssistantSvcWithProfile(profile)

	_, err := svc.Create(aCtx(), CreateAssistant{Name: "A", ISPProfileID: "isp1", Transport: "mcp",
		WriteModes: map[string]string{"liberacao": "talvez"}})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("an unknown mode value must be a validation error, got %v", err)
	}
}

func TestAssistantWriteModes_RequireISPProfile(t *testing.T) {
	svc, _ := newAssistantSvcWithProfile(nil)
	_, err := svc.Create(aCtx(), CreateAssistant{Name: "A",
		WriteModes: map[string]string{"liberacao": centity.WriteModeAuto}})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("write_modes without an ISP profile must be a validation error, got %v", err)
	}
}

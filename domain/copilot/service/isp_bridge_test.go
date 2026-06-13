package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	centity "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	mcpcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// fakeAssistantRepo is an in-memory copilot AssistantRepository.
type fakeAssistantRepo struct{ byID map[string]*centity.Assistant }

func (r *fakeAssistantRepo) Create(_ context.Context, a *centity.Assistant) error {
	r.byID[a.ID] = a
	return nil
}
func (r *fakeAssistantRepo) Update(_ context.Context, a *centity.Assistant) error {
	r.byID[a.ID] = a
	return nil
}
func (r *fakeAssistantRepo) Delete(_ context.Context, id string) error {
	delete(r.byID, id)
	return nil
}
func (r *fakeAssistantRepo) FindByID(_ context.Context, id string) (*centity.Assistant, error) {
	if a, ok := r.byID[id]; ok {
		return a, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeAssistantRepo) List(context.Context) ([]*centity.Assistant, error) {
	out := make([]*centity.Assistant, 0, len(r.byID))
	for _, a := range r.byID {
		out = append(out, a)
	}
	return out, nil
}
func (r *fakeAssistantRepo) FindByChannelType(_ context.Context, ct string) (*centity.Assistant, error) {
	for _, a := range r.byID {
		if a.Enabled && a.ServesChannel(ct) {
			return a, nil
		}
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeAssistantRepo) CountByISPProfile(_ context.Context, id string) (int, error) {
	n := 0
	for _, a := range r.byID {
		if a.ISPProfileID == id {
			n++
		}
	}
	return n, nil
}

// fakeProfileRepo is an in-memory providerhub ProfileRepository (read paths only).
type fakeProfileRepo struct {
	byID map[string]*phentity.ISPProfile
}

func (r *fakeProfileRepo) Create(context.Context, *phentity.ISPProfile) error { return nil }
func (r *fakeProfileRepo) Update(context.Context, *phentity.ISPProfile) error { return nil }
func (r *fakeProfileRepo) Delete(context.Context, string) error               { return nil }
func (r *fakeProfileRepo) FindByID(_ context.Context, id string) (*phentity.ISPProfile, error) {
	if p, ok := r.byID[id]; ok {
		return p, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeProfileRepo) FindDefault(context.Context) (*phentity.ISPProfile, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeProfileRepo) List(context.Context) ([]*phentity.ISPProfile, error) { return nil, nil }
func (r *fakeProfileRepo) ClearDefault(context.Context) error                   { return nil }

func bridgeCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }

func newBridge(assistant *centity.Assistant, profile *phentity.ISPProfile) *ISPToolBridge {
	ar := &fakeAssistantRepo{byID: map[string]*centity.Assistant{}}
	if assistant != nil {
		ar.byID[assistant.ID] = assistant
	}
	pr := &fakeProfileRepo{byID: map[string]*phentity.ISPProfile{}}
	if profile != nil {
		pr.byID[profile.ID] = profile
	}
	return NewISPToolBridge(ar, pr)
}

func TestBridge_DecorateInjectsConfigAndOverwritesModelConfig(t *testing.T) {
	prof := &phentity.ISPProfile{
		ID: "p1", TenantID: "t1", ISPType: "ixcsoft", Enabled: true,
		Credentials: map[string]string{"ixcsoft_host": "h", "ixcsoft_token": "real-token"},
	}
	asst := &centity.Assistant{ID: "a1", TenantID: "t1", ChannelTypes: []string{"whatsapp"}, ISPProfileID: "p1", Enabled: true}
	b := newBridge(asst, prof)

	// The "model" tries to smuggle its own config/credentials — must be overwritten.
	args, err := b.Decorate(bridgeCtx(), mcpcontracts.DecorateInput{
		ChannelType: "whatsapp", ServerName: "SMSNET_CONSULTAS", Write: false,
		Args: map[string]any{"cpfcnpj": "123", "config": map[string]any{"type": "evil", "ixcsoft_token": "attacker"}},
	})
	if err != nil {
		t.Fatalf("decorate: %v", err)
	}
	cfg, ok := args["config"].(map[string]any)
	if !ok {
		t.Fatalf("config not injected: %T", args["config"])
	}
	if cfg["type"] != "ixcsoft" {
		t.Errorf("config.type = %v, want ixcsoft (model value must be overwritten)", cfg["type"])
	}
	if cfg["ixcsoft_token"] != "real-token" {
		t.Errorf("real credential not injected: %v", cfg["ixcsoft_token"])
	}
	if args["cpfcnpj"] != "123" {
		t.Errorf("business arg dropped: %v", args["cpfcnpj"])
	}
}

func TestBridge_NoAssistantNoInjection(t *testing.T) {
	b := newBridge(nil, nil)
	args, err := b.Decorate(bridgeCtx(), mcpcontracts.DecorateInput{ChannelType: "whatsapp", Args: map[string]any{"x": 1}})
	if err != nil {
		t.Fatalf("decorate: %v", err)
	}
	if _, ok := args["config"]; ok {
		t.Errorf("no profile → must not inject a config")
	}
}

func TestBridge_WriteAddsIdempotencyKey(t *testing.T) {
	prof := &phentity.ISPProfile{ID: "p1", TenantID: "t1", ISPType: "ixcsoft", Enabled: true, Credentials: map[string]string{"ixcsoft_host": "h"}}
	asst := &centity.Assistant{ID: "a1", TenantID: "t1", ChannelTypes: []string{"whatsapp"}, ISPProfileID: "p1", Enabled: true}
	b := newBridge(asst, prof)
	args, err := b.Decorate(bridgeCtx(), mcpcontracts.DecorateInput{
		ChannelType: "whatsapp", Write: true, IdempotencyKey: "appr-1", Args: map[string]any{},
	})
	if err != nil {
		t.Fatalf("decorate: %v", err)
	}
	if args["idempotency_key"] != "appr-1" {
		t.Errorf("idempotency key not set on write: %v", args["idempotency_key"])
	}
}

func TestBridge_AllowServer(t *testing.T) {
	prof := &phentity.ISPProfile{ID: "p1", TenantID: "t1", ISPType: "ixcsoft", Enabled: true}
	asst := &centity.Assistant{ID: "a1", TenantID: "t1", ChannelTypes: []string{"whatsapp"}, ISPProfileID: "p1", Enabled: true}
	b := newBridge(asst, prof)

	// Read always allowed when a profile is pinned.
	if ok, _ := b.AllowServer(bridgeCtx(), "whatsapp", "SMSNET_CONSULTAS", false); !ok {
		t.Errorf("read server should be allowed with a pinned profile")
	}
	// Write allowed because ixcsoft supports liberacao/chamado.
	if ok, _ := b.AllowServer(bridgeCtx(), "whatsapp", "SMSNET_OPERACOES", true); !ok {
		t.Errorf("write server should be allowed for an ISP with write actions")
	}
	// No assistant for this channel → nothing allowed.
	if ok, _ := b.AllowServer(bridgeCtx(), "telegram", "SMSNET_CONSULTAS", false); ok {
		t.Errorf("no assistant for channel → SMSNET tools must be hidden")
	}
}

func TestBridge_WriteGatedWhenNoWriteAction(t *testing.T) {
	// A profile whose isp_type has no catalog descriptor (legacy) has no actions →
	// write server must be gated out.
	prof := &phentity.ISPProfile{ID: "p1", TenantID: "t1", ISPType: phentity.ISPVoalle, Enabled: true}
	asst := &centity.Assistant{ID: "a1", TenantID: "t1", ChannelTypes: []string{"whatsapp"}, ISPProfileID: "p1", Enabled: true}
	b := newBridge(asst, prof)
	if ok, _ := b.AllowServer(bridgeCtx(), "whatsapp", "SMSNET_OPERACOES", true); ok {
		t.Errorf("write server must be gated when the profile supports no write action")
	}
}

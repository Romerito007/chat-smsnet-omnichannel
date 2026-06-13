package service

import (
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
)

func newConfigSvc(repo *fakeConfigRepo) *ConfigService {
	return NewConfigService(repo, &fakeLogs{}, &fakeGateway{}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
}

func strptr(s string) *string { return &s }

func TestConfigCreate_AcceptsEvery19CatalogSlug(t *testing.T) {
	svc := newConfigSvc(&fakeConfigRepo{})
	for _, d := range phentity.ISPCatalog {
		cfg, err := svc.Create(actorCtx("t1", "u1"), contracts.CreateConfig{
			SMSNetBaseURL: "http://api",
			ISPType:       d.Slug,
		})
		if err != nil {
			t.Errorf("Create rejected catalog slug %q: %v", d.Slug, err)
			continue
		}
		if cfg.ISPType != d.Slug {
			t.Errorf("Create stored isp_type %q, want %q", cfg.ISPType, d.Slug)
		}
	}
}

func TestConfigCreate_AcceptsLegacySlugs(t *testing.T) {
	svc := newConfigSvc(&fakeConfigRepo{})
	for _, legacy := range []string{phentity.ISPVoalle, phentity.ISPSGP} {
		if _, err := svc.Create(actorCtx("t1", "u1"), contracts.CreateConfig{
			SMSNetBaseURL: "http://api", ISPType: legacy,
		}); err != nil {
			t.Errorf("Create rejected legacy slug %q: %v", legacy, err)
		}
	}
}

func TestConfigCreate_RejectsUnknownISPType(t *testing.T) {
	svc := newConfigSvc(&fakeConfigRepo{})
	_, err := svc.Create(actorCtx("t1", "u1"), contracts.CreateConfig{
		SMSNetBaseURL: "http://api", ISPType: "not-a-real-isp",
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation error for unknown isp_type, got %v", err)
	}
}

func TestConfigUpdate_AcceptsCatalogSlugAndRejectsUnknown(t *testing.T) {
	repo := &fakeConfigRepo{cfg: &phentity.ProviderIntegrationConfig{
		ID: "cfg1", TenantID: "t1", Enabled: true, ISPType: phentity.ISPHubsoft, SMSNetBaseURL: "http://api",
	}}
	svc := newConfigSvc(repo)

	cfg, err := svc.Update(actorCtx("t1", "u1"), contracts.UpdateConfig{ISPType: strptr(phentity.ISPIXCSoft)})
	if err != nil {
		t.Fatalf("Update rejected a valid catalog slug: %v", err)
	}
	if cfg.ISPType != phentity.ISPIXCSoft {
		t.Errorf("isp_type = %q, want %q", cfg.ISPType, phentity.ISPIXCSoft)
	}

	if _, err := svc.Update(actorCtx("t1", "u1"), contracts.UpdateConfig{ISPType: strptr("bogus")}); apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation error for unknown isp_type on update, got %v", err)
	}
}

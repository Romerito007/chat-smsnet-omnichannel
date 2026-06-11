package copilot_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/repository"
	cservice "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/service"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/copilot"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/httpharness"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// fakeConfigRepo: FindByTenant returns not_found so the service yields a default
// config; Create/Update succeed.
type fakeConfigRepo struct{ saved *entity.AIConfig }

func (r *fakeConfigRepo) Create(_ context.Context, c *entity.AIConfig) error { r.saved = c; return nil }
func (r *fakeConfigRepo) Update(_ context.Context, c *entity.AIConfig) error { r.saved = c; return nil }
func (r *fakeConfigRepo) FindByTenant(context.Context) (*entity.AIConfig, error) {
	return nil, apperror.NotFound("not found")
}

var _ repository.ConfigRepository = (*fakeConfigRepo)(nil)

func build(t *testing.T) (http.Handler, string) {
	tm := httpharness.Tokens()
	configSvc := cservice.NewConfigService(&fakeConfigRepo{}, nil)
	// The inference service is not exercised by the config endpoints; nil deps are
	// never dereferenced for GetConfig/SaveConfig.
	infer := cservice.NewService(configSvc, nil, nil, nil, nil, nil, nil)
	ctl := copilot.NewController(configSvc, infer)

	r := chi.NewRouter()
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(tm))
		p.With(middleware.RequirePermission(authz.CopilotConfigure)).Get("/copilot/config", ctl.GetConfig)
		p.With(middleware.RequirePermission(authz.CopilotConfigure)).Patch("/copilot/config", ctl.SaveConfig)
	})
	tok := httpharness.Token(t, tm, "t1", "u1", authz.CopilotConfigure)
	return r, tok
}

func TestCopilot_GetConfig_HappyDefault(t *testing.T) {
	r, tok := build(t)
	rec := httpharness.Do(t, r, http.MethodGet, "/copilot/config", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	httpharness.DecodeJSON(t, rec, &resp)
	if resp["provider"] == nil {
		t.Errorf("expected a config response with provider, got %v", resp)
	}
}

func TestCopilot_NoToken_401(t *testing.T) {
	r, _ := build(t)
	rec := httpharness.Do(t, r, http.MethodGet, "/copilot/config", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestCopilot_NoPermission_403(t *testing.T) {
	tm := httpharness.Tokens()
	configSvc := cservice.NewConfigService(&fakeConfigRepo{}, nil)
	infer := cservice.NewService(configSvc, nil, nil, nil, nil, nil, nil)
	ctl := copilot.NewController(configSvc, infer)
	r := chi.NewRouter()
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(tm))
		p.With(middleware.RequirePermission(authz.CopilotConfigure)).Get("/copilot/config", ctl.GetConfig)
	})
	tok := httpharness.Token(t, tm, "t1", "u1", authz.CopilotUse) // wrong permission

	rec := httpharness.Do(t, r, http.MethodGet, "/copilot/config", tok, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeForbidden {
		t.Errorf("code = %q, want forbidden", code)
	}
}

func TestCopilot_InvalidPayload_400(t *testing.T) {
	r, tok := build(t)
	rec := httpharness.Do(t, r, http.MethodPatch, "/copilot/config", tok, "{bad")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeValidation {
		t.Errorf("code = %q, want validation_error", code)
	}
}

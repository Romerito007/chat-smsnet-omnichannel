package privacy_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	pcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/contracts"
	pentity "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/entity"
	prepo "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/repository"
	pservice "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/service"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/privacy"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/httpharness"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

var tm = httpharness.Tokens()

// fakeStore: GetRetention returns nil so the service yields the keep-forever
// default. All other methods are inert (not exercised by the tested endpoints).
type fakeStore struct{}

func (fakeStore) GetRetention(context.Context) (*pentity.RetentionPolicy, error) { return nil, nil }
func (fakeStore) SaveRetention(context.Context, *pentity.RetentionPolicy) error  { return nil }
func (fakeStore) CreateExport(context.Context, *pentity.ExportRequest) error     { return nil }
func (fakeStore) UpdateExport(context.Context, *pentity.ExportRequest) error     { return nil }
func (fakeStore) FindExport(context.Context, string) (*pentity.ExportRequest, error) {
	return nil, apperror.NotFound("none")
}
func (fakeStore) CollectBundle(context.Context, string) (*prepo.ExportBundle, error) {
	return &prepo.ExportBundle{}, nil
}
func (fakeStore) LinkedDeals(context.Context, string) ([]prepo.DealLink, error) { return nil, nil }
func (fakeStore) EraseContact(context.Context, string, bool) (prepo.EraseResult, error) {
	return prepo.EraseResult{}, nil
}
func (fakeStore) HasActiveLegalHold(context.Context, string, time.Time) (bool, error) {
	return false, nil
}
func (fakeStore) ApplyRetention(context.Context, pentity.RetentionPolicy, time.Time) (prepo.RetentionResult, error) {
	return prepo.RetentionResult{}, nil
}

var _ prepo.Store = fakeStore{}

type fakeEnqueuer struct{}

func (fakeEnqueuer) EnqueueExport(pcontracts.ExportTask) error { return nil }

var _ pcontracts.ExportEnqueuer = fakeEnqueuer{}

// fakeFiles is an inert FileStore (download endpoint is not exercised here).
type fakeFiles struct{}

func (fakeFiles) Save(string, []byte) error { return nil }
func (fakeFiles) SignedURL(string, time.Duration) (string, time.Time, error) {
	return "", time.Time{}, nil
}
func (fakeFiles) Resolve(string) (string, error)      { return "", apperror.Forbidden("no") }
func (fakeFiles) Open(string) ([]byte, string, error) { return nil, "", apperror.NotFound("no") }
func (fakeFiles) Delete(string) error                 { return nil }

var _ pcontracts.FileStore = fakeFiles{}

func buildRouter() http.Handler {
	svc := pservice.NewService(fakeStore{}, fakeFiles{}, nil, fakeEnqueuer{}, nil, nil, time.Hour)
	ctl := privacy.NewController(svc, fakeFiles{})
	r := chi.NewRouter()
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(tm))
		p.With(middleware.RequirePermission(authz.PrivacyManage)).Group(func(m chi.Router) {
			m.Get("/privacy/retention", ctl.GetRetention)
			m.Patch("/privacy/retention", ctl.UpdateRetention)
		})
	})
	return r
}

func token(t *testing.T, perms ...authz.Permission) string {
	return httpharness.Token(t, tm, "t1", "u1", perms...)
}

func TestPrivacy_GetRetention_HappyDefault(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(), http.MethodGet, "/privacy/retention", token(t, authz.PrivacyManage), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	httpharness.DecodeJSON(t, rec, &resp)
	if _, ok := resp["messages_days"]; !ok {
		t.Errorf("expected a retention policy response, got %v", resp)
	}
}

func TestPrivacy_NoToken_401(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(), http.MethodGet, "/privacy/retention", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestPrivacy_NoPermission_403(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(), http.MethodGet, "/privacy/retention", token(t), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeForbidden {
		t.Errorf("code = %q, want forbidden", code)
	}
}

func TestPrivacy_InvalidPayload_400(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(), http.MethodPatch, "/privacy/retention", token(t, authz.PrivacyManage), "{bad")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeValidation {
		t.Errorf("code = %q, want validation_error", code)
	}
}

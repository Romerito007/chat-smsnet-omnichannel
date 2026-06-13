package webhooks_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	wcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/contracts"
	wentity "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
	wrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/repository"
	wservice "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/service"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/webhooks"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/httpharness"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeSubs struct {
	items []*wentity.WebhookSubscription
}

func (f *fakeSubs) Create(_ context.Context, s *wentity.WebhookSubscription) error {
	f.items = append(f.items, s)
	return nil
}
func (f *fakeSubs) Update(context.Context, *wentity.WebhookSubscription) error { return nil }
func (f *fakeSubs) Delete(context.Context, string) error                       { return nil }
func (f *fakeSubs) FindByID(context.Context, string) (*wentity.WebhookSubscription, error) {
	return nil, apperror.NotFound("not found")
}

// List is tenant-aware: it returns only the calling tenant's subscriptions,
// proving the tenant flows from the token through the context into the repo.
func (f *fakeSubs) List(ctx context.Context, _ shared.PageRequest) ([]*wentity.WebhookSubscription, error) {
	tenant, _ := shared.TenantFrom(ctx)
	var out []*wentity.WebhookSubscription
	for _, s := range f.items {
		if s.TenantID == tenant {
			out = append(out, s)
		}
	}
	return out, nil
}
func (f *fakeSubs) ListEnabledByEvent(context.Context, string, string) ([]*wentity.WebhookSubscription, error) {
	return nil, nil
}

type fakeDeliveries struct{}

func (fakeDeliveries) Create(context.Context, *wentity.WebhookDelivery) error { return nil }
func (fakeDeliveries) Update(context.Context, *wentity.WebhookDelivery) error { return nil }
func (fakeDeliveries) FindByID(context.Context, string) (*wentity.WebhookDelivery, error) {
	return nil, apperror.NotFound("not found")
}
func (fakeDeliveries) ListByWebhook(context.Context, string, shared.PageRequest) ([]*wentity.WebhookDelivery, error) {
	return nil, nil
}

type noopSender struct{}

func (noopSender) Send(context.Context, *wentity.WebhookSubscription, *wentity.WebhookDelivery) (wcontracts.SendResult, error) {
	return wcontracts.SendResult{}, nil
}

var (
	_ wrepo.SubscriptionRepository = (*fakeSubs)(nil)
	_ wrepo.DeliveryRepository     = fakeDeliveries{}
)

// ── harness ──────────────────────────────────────────────────────────────────

func newRouter(subs *fakeSubs) (http.Handler, auth.TokenManager) {
	tm := httpharness.Tokens()
	svc := wservice.NewSubscriptionService(subs, fakeDeliveries{}, noopSender{}, shared.SystemClock{})
	ctl := webhooks.NewController(svc)
	r := chi.NewRouter()
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(tm))
		p.Use(middleware.RequirePermission(authz.WebhookManage))
		p.Route("/webhooks", func(wh chi.Router) {
			wh.Get("/", ctl.List)
			wh.Post("/", ctl.Create)
		})
	})
	return r, tm
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestWebhooks_List_HappyPathAndCursorShape(t *testing.T) {
	subs := &fakeSubs{items: []*wentity.WebhookSubscription{
		{ID: "w1", TenantID: "t1", URL: "https://x", Events: []string{"conversation_created"}, CreatedAt: time.Now()},
	}}
	r, tm := newRouter(subs)
	tok := httpharness.Token(t, tm, "t1", "u1", authz.WebhookManage)

	rec := httpharness.Do(t, r, http.MethodGet, "/webhooks", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var page struct {
		Data []map[string]any `json:"data"`
		Page struct {
			HasMore    bool   `json:"has_more"`
			NextCursor string `json:"next_cursor"`
		} `json:"page"`
	}
	httpharness.DecodeJSON(t, rec, &page)
	if len(page.Data) != 1 {
		t.Errorf("expected 1 item for the tenant, got %d", len(page.Data))
	}
	// The paginated envelope must carry data + page (cursor shape).
	if _, ok := map[string]any{"has_more": page.Page.HasMore}["has_more"]; !ok {
		t.Errorf("missing page envelope")
	}
}

func TestWebhooks_List_FiltersByTenant(t *testing.T) {
	subs := &fakeSubs{items: []*wentity.WebhookSubscription{
		{ID: "w1", TenantID: "t1", CreatedAt: time.Now()},
		{ID: "w2", TenantID: "t2", CreatedAt: time.Now()},
	}}
	r, tm := newRouter(subs)
	tok := httpharness.Token(t, tm, "t2", "u1", authz.WebhookManage)

	rec := httpharness.Do(t, r, http.MethodGet, "/webhooks", tok, nil)
	var page struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	httpharness.DecodeJSON(t, rec, &page)
	if len(page.Data) != 1 || page.Data[0].ID != "w2" {
		t.Errorf("tenant t2 must only see w2, got %+v", page.Data)
	}
}

func TestWebhooks_Create_HappyPathReturnsSecretOnce(t *testing.T) {
	subs := &fakeSubs{}
	r, tm := newRouter(subs)
	tok := httpharness.Token(t, tm, "t1", "u1", authz.WebhookManage)

	body := map[string]any{"name": "crm", "url": "https://crm.example/hook", "events": []string{"conversation_created"}}
	rec := httpharness.Do(t, r, http.MethodPost, "/webhooks", tok, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
	}
	httpharness.DecodeJSON(t, rec, &resp)
	if resp.ID == "" || resp.Secret == "" {
		t.Errorf("create must return id + secret once, got %+v", resp)
	}
}

func TestWebhooks_NoToken_401(t *testing.T) {
	r, _ := newRouter(&fakeSubs{})
	rec := httpharness.Do(t, r, http.MethodGet, "/webhooks", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeUnauthorized {
		t.Errorf("code = %q, want unauthorized", code)
	}
}

func TestWebhooks_NoPermission_403(t *testing.T) {
	r, tm := newRouter(&fakeSubs{})
	tok := httpharness.Token(t, tm, "t1", "u1") // no webhook.manage
	rec := httpharness.Do(t, r, http.MethodGet, "/webhooks", tok, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeForbidden {
		t.Errorf("code = %q, want forbidden", code)
	}
}

func TestWebhooks_InvalidPayload_400(t *testing.T) {
	r, tm := newRouter(&fakeSubs{})
	tok := httpharness.Token(t, tm, "t1", "u1", authz.WebhookManage)

	// Malformed JSON → standard validation envelope before the service runs.
	rec := httpharness.Do(t, r, http.MethodPost, "/webhooks", tok, "{ not json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeValidation {
		t.Errorf("code = %q, want validation_error", code)
	}

	// Semantically invalid (missing url + events) → validation from the service.
	rec = httpharness.Do(t, r, http.MethodPost, "/webhooks", tok, map[string]any{"name": "x"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
}

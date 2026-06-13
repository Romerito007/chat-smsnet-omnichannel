package conversations_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	convservice "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/service"
	sectorentity "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/conversations"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/httpharness"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// All JWT managers built from httpharness.Tokens() share the same secret, so a
// token minted by one verifies against another. We keep one for the whole file.
var tm = httpharness.Tokens()

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeConvRepo struct{ items []*entity.Conversation }

func (r *fakeConvRepo) Create(_ context.Context, c *entity.Conversation) error {
	r.items = append(r.items, c)
	return nil
}
func (r *fakeConvRepo) Update(context.Context, *entity.Conversation) error { return nil }
func (r *fakeConvRepo) FindByID(_ context.Context, id string) (*entity.Conversation, error) {
	for _, c := range r.items {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, apperror.NotFound("not found")
}
func (r *fakeConvRepo) FindOpenByContactChannel(context.Context, string, string) (*entity.Conversation, error) {
	return nil, apperror.NotFound("none")
}
func (r *fakeConvRepo) List(ctx context.Context, _ convcontracts.ListFilter, _ convcontracts.Visibility, _ shared.PageRequest) ([]*entity.Conversation, error) {
	tenant, _ := shared.TenantFrom(ctx)
	var out []*entity.Conversation
	for _, c := range r.items {
		if c.TenantID == tenant {
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *fakeConvRepo) ListInactiveOpen(context.Context, time.Time, int) ([]*entity.Conversation, error) {
	return nil, nil
}

type fakeMsgRepo struct{}

func (fakeMsgRepo) Create(context.Context, *entity.Message) error { return nil }
func (fakeMsgRepo) Update(context.Context, *entity.Message) error { return nil }
func (fakeMsgRepo) FindByID(context.Context, string) (*entity.Message, error) {
	return nil, apperror.NotFound("none")
}
func (fakeMsgRepo) ListByConversation(context.Context, string, shared.PageRequest) ([]*entity.Message, error) {
	return nil, nil
}
func (fakeMsgRepo) LatestByConversation(context.Context, string) (*entity.Message, error) {
	return nil, apperror.NotFound("none")
}

type fakeEventRepo struct{}

func (fakeEventRepo) Create(context.Context, *entity.ConversationEvent) error { return nil }
func (fakeEventRepo) ListByConversation(context.Context, string, shared.PageRequest) ([]*entity.ConversationEvent, error) {
	return nil, nil
}

type fakeSectorRepo struct{}

func (fakeSectorRepo) Create(context.Context, *sectorentity.Sector) error { return nil }
func (fakeSectorRepo) Update(context.Context, *sectorentity.Sector) error { return nil }
func (fakeSectorRepo) Delete(context.Context, string) error               { return nil }
func (fakeSectorRepo) FindByID(context.Context, string) (*sectorentity.Sector, error) {
	return nil, apperror.NotFound("none")
}
func (fakeSectorRepo) List(context.Context, shared.PageRequest) ([]*sectorentity.Sector, error) {
	return nil, nil
}

var (
	_ convrepo.ConversationRepository = (*fakeConvRepo)(nil)
	_ convrepo.MessageRepository      = fakeMsgRepo{}
	_ convrepo.EventRepository        = fakeEventRepo{}
)

// fakeContactAvatars resolves contact ids to signed avatar URLs, counting calls
// so a test can assert per-page batching.
type fakeContactAvatars struct {
	byContact map[string]string
	calls     int
}

func (f *fakeContactAvatars) ContactAvatarURLs(_ context.Context, contactIDs []string) (map[string]string, error) {
	f.calls++
	out := map[string]string{}
	for _, id := range contactIDs {
		if u, ok := f.byContact[id]; ok {
			out[id] = u
		}
	}
	return out, nil
}

// ── harness ──────────────────────────────────────────────────────────────────

func buildRouter(cr *fakeConvRepo) http.Handler {
	return buildRouterWithAvatars(cr, nil)
}

func buildRouterWithAvatars(cr *fakeConvRepo, av convcontracts.ContactAvatarResolver) http.Handler {
	svc := convservice.New(cr, fakeMsgRepo{}, fakeEventRepo{}, fakeSectorRepo{}, shared.NoopPublisher{}, shared.SystemClock{})
	if av != nil {
		svc.SetContactAvatarResolver(av)
	}
	ctl := conversations.NewController(svc)
	r := chi.NewRouter()
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(tm))
		p.Route("/conversations", func(cv chi.Router) {
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Get("/", ctl.List)
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Post("/", ctl.Create)
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Get("/{id}", ctl.Get)
		})
	})
	return r
}

func token(t *testing.T, tenant string, perms ...authz.Permission) string {
	return httpharness.Token(t, tm, tenant, "u1", perms...)
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestConversations_List_HappyTenantFilteredCursorShape(t *testing.T) {
	cr := &fakeConvRepo{items: []*entity.Conversation{
		{ID: "c1", TenantID: "t1", Status: entity.StatusNew, Priority: entity.PriorityNormal, UpdatedAt: time.Now()},
		{ID: "c2", TenantID: "t2", Status: entity.StatusNew, Priority: entity.PriorityNormal, UpdatedAt: time.Now()},
	}}
	rec := httpharness.Do(t, buildRouter(cr), http.MethodGet, "/conversations", token(t, "t1", authz.ConversationRead), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var page struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Page struct {
			HasMore bool `json:"has_more"`
		} `json:"page"`
	}
	httpharness.DecodeJSON(t, rec, &page)
	if len(page.Data) != 1 || page.Data[0].ID != "c1" {
		t.Errorf("tenant t1 must only see c1 (tenant filter), got %+v", page.Data)
	}
}

// GET /v1/conversations resolves contact_avatar_url per row in one batch: a
// conversation whose contact has a ready avatar carries the signed URL; a contact
// without an avatar omits the field (front falls back to initials).
func TestConversations_List_ResolvesContactAvatarURLInBatch(t *testing.T) {
	now := time.Now()
	cr := &fakeConvRepo{items: []*entity.Conversation{
		{ID: "c1", TenantID: "t1", ContactID: "ct-av", Status: entity.StatusNew, Priority: entity.PriorityNormal, UpdatedAt: now},
		{ID: "c2", TenantID: "t1", ContactID: "ct-none", Status: entity.StatusNew, Priority: entity.PriorityNormal, UpdatedAt: now},
	}}
	av := &fakeContactAvatars{byContact: map[string]string{"ct-av": "http://api/v1/channel-media/tok-ct-av"}}

	rec := httpharness.Do(t, buildRouterWithAvatars(cr, av), http.MethodGet, "/conversations", token(t, "t1", authz.ConversationRead), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var page struct {
		Data []struct {
			ID               string `json:"id"`
			ContactAvatarURL string `json:"contact_avatar_url"`
		} `json:"data"`
	}
	httpharness.DecodeJSON(t, rec, &page)
	byID := map[string]string{}
	for _, d := range page.Data {
		byID[d.ID] = d.ContactAvatarURL
	}
	if byID["c1"] != "http://api/v1/channel-media/tok-ct-av" {
		t.Errorf("c1 (contact with avatar) must carry contact_avatar_url, got %q", byID["c1"])
	}
	if !strings.Contains(byID["c1"], "/v1/channel-media/") {
		t.Errorf("contact_avatar_url must be the JWT-less channel-media URL, got %q", byID["c1"])
	}
	if byID["c2"] != "" {
		t.Errorf("c2 (contact without avatar) must omit contact_avatar_url, got %q", byID["c2"])
	}
	if av.calls != 1 {
		t.Errorf("expected one batch resolution for the page, got %d calls", av.calls)
	}
}

// GET /v1/conversations/{id} (detail) resolves contact_avatar_url consistently
// with the list.
func TestConversations_Detail_ResolvesContactAvatarURL(t *testing.T) {
	cr := &fakeConvRepo{items: []*entity.Conversation{
		{ID: "c1", TenantID: "t1", ContactID: "ct-av", Status: entity.StatusNew, Priority: entity.PriorityNormal, UpdatedAt: time.Now()},
	}}
	av := &fakeContactAvatars{byContact: map[string]string{"ct-av": "http://api/v1/channel-media/tok-ct-av"}}

	rec := httpharness.Do(t, buildRouterWithAvatars(cr, av), http.MethodGet, "/conversations/c1", token(t, "t1", authz.ConversationRead), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		ContactAvatarURL string `json:"contact_avatar_url"`
	}
	httpharness.DecodeJSON(t, rec, &resp)
	if resp.ContactAvatarURL != "http://api/v1/channel-media/tok-ct-av" {
		t.Errorf("detail must resolve contact_avatar_url, got %q", resp.ContactAvatarURL)
	}
}

func TestConversations_Create_Happy(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(&fakeConvRepo{}), http.MethodPost, "/conversations",
		token(t, "t1", authz.ConversationRead), map[string]any{"contact_id": "ct1", "channel": "whatsapp"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	httpharness.DecodeJSON(t, rec, &resp)
	if resp.ID == "" || resp.Status != "new" {
		t.Errorf("unexpected create response: %+v", resp)
	}
}

func TestConversations_NoToken_401(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(&fakeConvRepo{}), http.MethodGet, "/conversations", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestConversations_NoPermission_403(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(&fakeConvRepo{}), http.MethodGet, "/conversations", token(t, "t1"), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeForbidden {
		t.Errorf("code = %q, want forbidden", code)
	}
}

func TestConversations_InvalidPayload_400(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(&fakeConvRepo{}), http.MethodPost, "/conversations",
		token(t, "t1", authz.ConversationRead), "{bad")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeValidation {
		t.Errorf("code = %q, want validation_error", code)
	}
}

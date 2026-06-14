package routing_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	routingservice "github.com/romerito007/chat-smsnet-omnichannel/domain/routing/service"
	sectorentity "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/routing"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/httpharness"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

var tm = httpharness.Tokens()

// Minimal repos for the sector-only transfer happy path. presence/load/users/
// queues are not used by that path and are passed as nil.

type fakeConvRepo struct {
	items map[string]*conventity.Conversation
}

func (r *fakeConvRepo) Create(context.Context, *conventity.Conversation) error { return nil }
func (r *fakeConvRepo) Update(_ context.Context, c *conventity.Conversation) error {
	r.items[c.ID] = c
	return nil
}
func (r *fakeConvRepo) FindByID(_ context.Context, id string) (*conventity.Conversation, error) {
	if c, ok := r.items[id]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("none")
}
func (r *fakeConvRepo) FindByIDs(context.Context, []string) ([]*conventity.Conversation, error) {
	return nil, nil
}
func (r *fakeConvRepo) FindLastByContactChannelID(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindOpenByContactChannelID(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("none")
}
func (r *fakeConvRepo) List(context.Context, convcontracts.ListFilter, convcontracts.Visibility, shared.PageRequest) ([]*conventity.Conversation, error) {
	return nil, nil
}
func (r *fakeConvRepo) ListInactiveOpen(context.Context, time.Time, int) ([]*conventity.Conversation, error) {
	return nil, nil
}

var _ convrepo.ConversationRepository = (*fakeConvRepo)(nil)

type fakeEventRepo struct{}

func (fakeEventRepo) Create(context.Context, *conventity.ConversationEvent) error { return nil }
func (fakeEventRepo) ListByConversation(context.Context, string, shared.PageRequest) ([]*conventity.ConversationEvent, error) {
	return nil, nil
}

type fakeSectorRepo struct{ exists map[string]bool }

func (r fakeSectorRepo) Create(context.Context, *sectorentity.Sector) error { return nil }
func (r fakeSectorRepo) Update(context.Context, *sectorentity.Sector) error { return nil }
func (r fakeSectorRepo) Delete(context.Context, string) error               { return nil }
func (r fakeSectorRepo) FindByID(ctx context.Context, id string) (*sectorentity.Sector, error) {
	if r.exists[id] {
		tenant, _ := shared.TenantFrom(ctx)
		return &sectorentity.Sector{ID: id, TenantID: tenant}, nil
	}
	return nil, apperror.NotFound("none")
}
func (r fakeSectorRepo) List(context.Context, shared.PageRequest) ([]*sectorentity.Sector, error) {
	return nil, nil
}

func buildRouter(convs map[string]*conventity.Conversation, sectors map[string]bool) http.Handler {
	svc := routingservice.New(
		&fakeConvRepo{items: convs}, fakeEventRepo{},
		nil, nil, nil, // presence, load, users (unused by sector-only transfer)
		fakeSectorRepo{exists: sectors},
		nil, // queues (unused)
		shared.NoopLocker{}, shared.NoopPublisher{}, shared.SystemClock{},
	)
	ctl := routing.NewController(svc)
	r := chi.NewRouter()
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(tm))
		p.With(middleware.RequirePermission(authz.ConversationTransfer)).Post("/conversations/{id}/transfer", ctl.Transfer)
	})
	return r
}

func token(t *testing.T, perms ...authz.Permission) string {
	return httpharness.Token(t, tm, "t1", "u1", perms...)
}

func TestRouting_Transfer_HappyToSector(t *testing.T) {
	convs := map[string]*conventity.Conversation{
		"cv1": {ID: "cv1", TenantID: "t1", SectorID: "s1", Status: conventity.StatusAssigned, Priority: conventity.PriorityNormal},
	}
	r := buildRouter(convs, map[string]bool{"s2": true})
	rec := httpharness.Do(t, r, http.MethodPost, "/conversations/cv1/transfer",
		token(t, authz.ConversationTransfer), map[string]any{"sector_id": "s2"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID       string `json:"id"`
		SectorID string `json:"sector_id"`
		Status   string `json:"status"`
	}
	httpharness.DecodeJSON(t, rec, &resp)
	if resp.SectorID != "s2" || resp.Status != "transferred" {
		t.Errorf("unexpected transfer result: %+v", resp)
	}
}

func TestRouting_NoToken_401(t *testing.T) {
	r := buildRouter(map[string]*conventity.Conversation{}, nil)
	rec := httpharness.Do(t, r, http.MethodPost, "/conversations/cv1/transfer", "", map[string]any{"sector_id": "s2"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRouting_NoPermission_403(t *testing.T) {
	r := buildRouter(map[string]*conventity.Conversation{}, nil)
	rec := httpharness.Do(t, r, http.MethodPost, "/conversations/cv1/transfer", token(t), map[string]any{"sector_id": "s2"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeForbidden {
		t.Errorf("code = %q, want forbidden", code)
	}
}

func TestRouting_InvalidPayload_400(t *testing.T) {
	r := buildRouter(map[string]*conventity.Conversation{}, nil)
	rec := httpharness.Do(t, r, http.MethodPost, "/conversations/cv1/transfer", token(t, authz.ConversationTransfer), "{bad")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeValidation {
		t.Errorf("code = %q, want validation_error", code)
	}
}

package channels_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	chrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	chservice "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/channels"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/httpharness"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

var tm = httpharness.Tokens()

type fakeConnRepo struct{ items []*entity.ChannelConnection }

func (r *fakeConnRepo) Create(_ context.Context, c *entity.ChannelConnection) error {
	r.items = append(r.items, c)
	return nil
}
func (r *fakeConnRepo) Update(context.Context, *entity.ChannelConnection) error { return nil }
func (r *fakeConnRepo) Delete(context.Context, string) error                    { return nil }
func (r *fakeConnRepo) FindByID(context.Context, string) (*entity.ChannelConnection, error) {
	return nil, apperror.NotFound("none")
}
func (r *fakeConnRepo) List(ctx context.Context, _ shared.PageRequest) ([]*entity.ChannelConnection, error) {
	tenant, _ := shared.TenantFrom(ctx)
	var out []*entity.ChannelConnection
	for _, c := range r.items {
		if c.TenantID == tenant {
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *fakeConnRepo) FindEnabledByType(context.Context, entity.Type) (*entity.ChannelConnection, error) {
	return nil, apperror.NotFound("none")
}
func (r *fakeConnRepo) FindByInboundTokenHash(context.Context, string) (*entity.ChannelConnection, error) {
	return nil, apperror.NotFound("none")
}

var _ chrepo.ConnectionRepository = (*fakeConnRepo)(nil)

func buildRouter(repo *fakeConnRepo) http.Handler {
	// The adapter registry is only needed for create/test; List does not use it.
	svc := chservice.NewConnectionService(repo, nil, shared.SystemClock{})
	ctl := channels.NewConnectionController(svc)
	r := chi.NewRouter()
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(tm))
		p.Use(middleware.RequirePermission(authz.ChannelManage))
		p.Route("/channels", func(ch chi.Router) {
			ch.Get("/", ctl.List)
			ch.Post("/", ctl.Create)
		})
	})
	return r
}

func token(t *testing.T, tenant string, perms ...authz.Permission) string {
	return httpharness.Token(t, tm, tenant, "u1", perms...)
}

func TestChannels_List_HappyTenantFilteredCursorShape(t *testing.T) {
	repo := &fakeConnRepo{items: []*entity.ChannelConnection{
		{ID: "k1", TenantID: "t1", CreatedAt: time.Now()},
		{ID: "k2", TenantID: "t2", CreatedAt: time.Now()},
	}}
	rec := httpharness.Do(t, buildRouter(repo), http.MethodGet, "/channels", token(t, "t1", authz.ChannelManage), nil)
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
	if len(page.Data) != 1 || page.Data[0].ID != "k1" {
		t.Errorf("tenant t1 must only see k1, got %+v", page.Data)
	}
}

func TestChannels_NoToken_401(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(&fakeConnRepo{}), http.MethodGet, "/channels", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestChannels_NoPermission_403(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(&fakeConnRepo{}), http.MethodGet, "/channels", token(t, "t1"), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeForbidden {
		t.Errorf("code = %q, want forbidden", code)
	}
}

func TestChannels_InvalidPayload_400(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(&fakeConnRepo{}), http.MethodPost, "/channels", token(t, "t1", authz.ChannelManage), "{bad")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeValidation {
		t.Errorf("code = %q, want validation_error", code)
	}
}

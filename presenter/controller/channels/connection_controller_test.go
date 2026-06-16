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

// seededUpdRepo holds ONE connection so a PATCH round-trip (FindByID → Update) can
// be exercised end-to-end.
type seededUpdRepo struct{ conn *entity.ChannelConnection }

func (r *seededUpdRepo) Create(context.Context, *entity.ChannelConnection) error { return nil }
func (r *seededUpdRepo) Update(_ context.Context, c *entity.ChannelConnection) error {
	r.conn = c
	return nil
}
func (r *seededUpdRepo) Delete(context.Context, string) error { return nil }
func (r *seededUpdRepo) FindByID(context.Context, string) (*entity.ChannelConnection, error) {
	cp := *r.conn
	return &cp, nil
}
func (r *seededUpdRepo) List(context.Context, shared.PageRequest) ([]*entity.ChannelConnection, error) {
	return nil, nil
}
func (r *seededUpdRepo) FindByInboundTokenHash(context.Context, string) (*entity.ChannelConnection, error) {
	return nil, apperror.NotFound("none")
}

// TestChannels_Update_PersistsAndEchoesOutOfHoursMessage proves the backend accepts
// out_of_hours_message on PATCH /v1/channels/{id}, persists it, and returns it. (A
// PATCH that OMITS the key leaves it unchanged — the field is a pointer.)
func TestChannels_Update_PersistsAndEchoesOutOfHoursMessage(t *testing.T) {
	repo := &seededUpdRepo{conn: &entity.ChannelConnection{ID: "c1", TenantID: "t1", Type: entity.TypeAPI, Enabled: true}}
	svc := chservice.NewConnectionService(repo, nil, shared.SystemClock{})
	ctl := channels.NewConnectionController(svc)
	r := chi.NewRouter()
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(tm))
		p.Use(middleware.RequirePermission(authz.ChannelManage))
		p.Patch("/channels/{id}", ctl.Update)
	})

	tok := token(t, "t1", authz.ChannelManage)
	rec := httpharness.Do(t, r, http.MethodPatch, "/channels/c1", tok,
		map[string]any{"out_of_hours_message": "Estamos fora do Horário."})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		OutOfHoursMessage string `json:"out_of_hours_message"`
	}
	httpharness.DecodeJSON(t, rec, &resp)
	if resp.OutOfHoursMessage != "Estamos fora do Horário." {
		t.Errorf("response out_of_hours_message = %q, want the saved text", resp.OutOfHoursMessage)
	}
	if repo.conn.OutOfHoursMessage != "Estamos fora do Horário." {
		t.Errorf("persisted out_of_hours_message = %q, want the saved text", repo.conn.OutOfHoursMessage)
	}
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

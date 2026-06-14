package businesshours

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	bhentity "github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/entity"
	bhservice "github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/service"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── minimal fakes for the business-status endpoint ───────────────────────────

type fakeConnRepo struct {
	conns map[string]*chentity.ChannelConnection
}

func (r *fakeConnRepo) Create(context.Context, *chentity.ChannelConnection) error { return nil }
func (r *fakeConnRepo) Update(context.Context, *chentity.ChannelConnection) error { return nil }
func (r *fakeConnRepo) Delete(context.Context, string) error                      { return nil }
func (r *fakeConnRepo) FindByID(_ context.Context, id string) (*chentity.ChannelConnection, error) {
	if c, ok := r.conns[id]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeConnRepo) List(context.Context, shared.PageRequest) ([]*chentity.ChannelConnection, error) {
	return nil, nil
}
func (r *fakeConnRepo) FindByInboundTokenHash(context.Context, string) (*chentity.ChannelConnection, error) {
	return nil, apperror.NotFound("nf")
}

type fakeHolidayRepo struct{}

func (fakeHolidayRepo) Create(context.Context, *bhentity.Holiday) error { return nil }
func (fakeHolidayRepo) Update(context.Context, *bhentity.Holiday) error { return nil }
func (fakeHolidayRepo) Delete(context.Context, string) error            { return nil }
func (fakeHolidayRepo) FindByID(context.Context, string) (*bhentity.Holiday, error) {
	return nil, apperror.NotFound("nf")
}
func (fakeHolidayRepo) List(context.Context, shared.PageRequest) ([]*bhentity.Holiday, error) {
	return nil, nil
}
func (fakeHolidayRepo) ListAll(context.Context) ([]*bhentity.Holiday, error) { return nil, nil }

func newCtl() *Controller {
	conns := &fakeConnRepo{conns: map[string]*chentity.ChannelConnection{
		"ch1": {ID: "ch1", TenantID: "t1", BusinessHours: map[string]any{
			"timezone": "UTC",
			"weekly":   []any{map[string]any{"day": 1, "intervals": []any{map[string]any{"start": "09:00", "end": "18:00"}}}},
		}},
	}}
	hr := fakeHolidayRepo{}
	hours := bhservice.NewBusinessHoursService(conns, hr, nil)
	holidays := bhservice.NewHolidayService(hr, nil)
	return NewController(holidays, hours)
}

func doStatus(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Get("/v1/channels/{id}/business-status", newCtl().BusinessStatus)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req = req.WithContext(shared.WithTenant(req.Context(), "t1"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestBusinessStatus_OpenAtInstant(t *testing.T) {
	// 2025-01-06 is a Monday; 13:00 UTC is inside the 09:00–18:00 window.
	rec := doStatus(t, "/v1/channels/ch1/business-status?at=2025-01-06T13:00:00Z")
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		ChannelID string `json:"channel_id"`
		Open      bool   `json:"open"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ChannelID != "ch1" || !body.Open || body.Reason != "open" {
		t.Errorf("got %+v, want channel_id=ch1 open=true reason=open", body)
	}
}

func TestBusinessStatus_ClosedOutsideHours(t *testing.T) {
	// Monday 21:00 UTC is after close → closed/outside_hours.
	rec := doStatus(t, "/v1/channels/ch1/business-status?at=2025-01-06T21:00:00Z")
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d", rec.Code)
	}
	var body struct {
		Open   bool   `json:"open"`
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Open || body.Reason != "outside_hours" {
		t.Errorf("got open=%v reason=%s, want closed/outside_hours", body.Open, body.Reason)
	}
}

func TestBusinessStatus_BadAtIs400(t *testing.T) {
	rec := doStatus(t, "/v1/channels/ch1/business-status?at=not-a-timestamp")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want 400", rec.Code)
	}
}

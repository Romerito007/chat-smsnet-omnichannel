package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/entity"
	sectorentity "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeSectorRepo struct {
	sectors map[string]*sectorentity.Sector
}

func (r *fakeSectorRepo) Create(context.Context, *sectorentity.Sector) error { return nil }
func (r *fakeSectorRepo) Update(context.Context, *sectorentity.Sector) error { return nil }
func (r *fakeSectorRepo) Delete(context.Context, string) error               { return nil }
func (r *fakeSectorRepo) FindByID(_ context.Context, id string) (*sectorentity.Sector, error) {
	if s, ok := r.sectors[id]; ok {
		return s, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeSectorRepo) List(context.Context, shared.PageRequest) ([]*sectorentity.Sector, error) {
	return nil, nil
}

type fakeHolidayRepo struct{ all []*entity.Holiday }

func (r *fakeHolidayRepo) Create(context.Context, *entity.Holiday) error { return nil }
func (r *fakeHolidayRepo) Update(context.Context, *entity.Holiday) error { return nil }
func (r *fakeHolidayRepo) Delete(context.Context, string) error          { return nil }
func (r *fakeHolidayRepo) FindByID(context.Context, string) (*entity.Holiday, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeHolidayRepo) List(context.Context, shared.PageRequest) ([]*entity.Holiday, error) {
	return r.all, nil
}
func (r *fakeHolidayRepo) ListAll(context.Context) ([]*entity.Holiday, error) { return r.all, nil }

// ── helpers ──────────────────────────────────────────────────────────────────

// schedule builds a sector business_hours document with the given timezone and a
// single mon-fri 09:00-18:00 window.
func weekdaysDoc(tz string) map[string]any {
	window := []any{map[string]any{"start": "09:00", "end": "18:00"}}
	return map[string]any{
		"timezone": tz,
		"weekly": map[string]any{
			"monday": window, "tuesday": window, "wednesday": window,
			"thursday": window, "friday": window,
		},
	}
}

func newSvc(sectorDoc map[string]any, holidays ...*entity.Holiday) *BusinessHoursService {
	sectors := &fakeSectorRepo{sectors: map[string]*sectorentity.Sector{
		"s1": {ID: "s1", TenantID: "t1", BusinessHours: sectorDoc},
		"s2": {ID: "s2", TenantID: "t1", BusinessHours: weekdaysDoc("UTC")},
	}}
	return NewBusinessHoursService(sectors, &fakeHolidayRepo{all: holidays}, nil)
}

func ctxT() context.Context { return shared.WithTenant(context.Background(), "t1") }

func mustUTC(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return tm
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestStatus_InsideAndOutsideHours(t *testing.T) {
	svc := newSvc(weekdaysDoc("UTC"))
	// 2025-01-06 is a Monday.
	cases := []struct {
		at   string
		open bool
	}{
		{"2025-01-06T09:00:00Z", true},  // exactly opening (inclusive)
		{"2025-01-06T13:00:00Z", true},  // midday
		{"2025-01-06T17:59:00Z", true},  // just before close
		{"2025-01-06T18:00:00Z", false}, // exactly closing (exclusive)
		{"2025-01-06T08:59:00Z", false}, // just before open
		{"2025-01-06T21:00:00Z", false}, // evening
	}
	for _, c := range cases {
		st, err := svc.Status(ctxT(), "s1", mustUTC(t, c.at))
		if err != nil {
			t.Fatalf("status %s: %v", c.at, err)
		}
		if st.Open != c.open {
			t.Errorf("at %s: open=%v, want %v (reason=%s)", c.at, st.Open, c.open, st.Reason)
		}
	}
}

func TestStatus_WeekendClosed(t *testing.T) {
	svc := newSvc(weekdaysDoc("UTC"))
	// 2025-01-04 is a Saturday (no window configured).
	st, err := svc.Status(ctxT(), "s1", mustUTC(t, "2025-01-04T13:00:00Z"))
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Open || st.Reason != contracts.ReasonOutsideHours {
		t.Errorf("expected closed/outside_hours on Saturday, got open=%v reason=%s", st.Open, st.Reason)
	}
}

func TestStatus_TimezoneRespected(t *testing.T) {
	// Same UTC instant, different sector timezones → different local wall time.
	// 2025-01-06T13:00:00Z is Monday 10:00 in São Paulo (UTC-3): open.
	spSvc := newSvc(weekdaysDoc("America/Sao_Paulo"))
	st, err := spSvc.Status(ctxT(), "s1", mustUTC(t, "2025-01-06T13:00:00Z"))
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !st.Open {
		t.Errorf("expected open at 10:00 BRT, got closed (local=%s)", st.LocalTime)
	}
	// 2025-01-06T21:00:00Z is 18:00 BRT → exactly closing → closed.
	st, _ = spSvc.Status(ctxT(), "s1", mustUTC(t, "2025-01-06T21:00:00Z"))
	if st.Open {
		t.Errorf("expected closed at 18:00 BRT (exclusive)")
	}
	// 2025-01-06T11:00:00Z is 08:00 BRT → before open → closed.
	st, _ = spSvc.Status(ctxT(), "s1", mustUTC(t, "2025-01-06T11:00:00Z"))
	if st.Open {
		t.Errorf("expected closed at 08:00 BRT")
	}
}

func TestStatus_HolidayBlocksDay(t *testing.T) {
	holiday := &entity.Holiday{
		ID: "h1", TenantID: "t1", Date: "2025-01-06", Name: "Company Day",
		Scope: entity.ScopeAllSectors,
	}
	svc := newSvc(weekdaysDoc("UTC"), holiday)
	// Monday 13:00 would normally be open, but it is a holiday.
	st, err := svc.Status(ctxT(), "s1", mustUTC(t, "2025-01-06T13:00:00Z"))
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Open || st.Reason != contracts.ReasonHoliday || st.HolidayName != "Company Day" {
		t.Errorf("expected closed/holiday, got open=%v reason=%s name=%q", st.Open, st.Reason, st.HolidayName)
	}
}

func TestStatus_RecurringHoliday(t *testing.T) {
	// Christmas, recurring, defined in 2020 but applies every year.
	xmas := &entity.Holiday{
		ID: "h2", TenantID: "t1", Date: "2020-12-25", Name: "Christmas",
		Scope: entity.ScopeAllSectors, Recurring: true,
	}
	svc := newSvc(weekdaysDoc("UTC"), xmas)
	// 2025-12-25 is a Thursday (normally open) → blocked by recurring holiday.
	st, _ := svc.Status(ctxT(), "s1", mustUTC(t, "2025-12-25T13:00:00Z"))
	if st.Open || st.Reason != contracts.ReasonHoliday {
		t.Errorf("expected recurring holiday to block 2025-12-25, got open=%v reason=%s", st.Open, st.Reason)
	}
	// A non-recurring holiday on the same month/day but a different year must NOT
	// match other years.
	oneOff := &entity.Holiday{ID: "h3", TenantID: "t1", Date: "2020-12-25", Name: "One-off", Scope: entity.ScopeAllSectors}
	svc2 := newSvc(weekdaysDoc("UTC"), oneOff)
	st, _ = svc2.Status(ctxT(), "s1", mustUTC(t, "2025-12-25T13:00:00Z"))
	if !st.Open {
		t.Errorf("expected one-off 2020 holiday NOT to affect 2025")
	}
}

func TestStatus_SectorScopedHoliday(t *testing.T) {
	// Holiday only for sector s2 must not close s1.
	holiday := &entity.Holiday{
		ID: "h4", TenantID: "t1", Date: "2025-01-06", Name: "S2 only",
		Scope: entity.ScopeSectors, SectorIDs: []string{"s2"},
	}
	svc := newSvc(weekdaysDoc("UTC"), holiday)
	st, _ := svc.Status(ctxT(), "s1", mustUTC(t, "2025-01-06T13:00:00Z"))
	if !st.Open {
		t.Errorf("sector-scoped holiday for s2 must not close s1")
	}
	st, _ = svc.Status(ctxT(), "s2", mustUTC(t, "2025-01-06T13:00:00Z"))
	if st.Open || st.Reason != contracts.ReasonHoliday {
		t.Errorf("expected s2 closed by its scoped holiday, got open=%v", st.Open)
	}
}

func TestStatus_Unconfigured_IsOpen(t *testing.T) {
	svc := newSvc(nil) // no business hours document
	st, err := svc.Status(ctxT(), "s1", mustUTC(t, "2025-01-04T03:00:00Z"))
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !st.Open || st.Reason != contracts.ReasonUnconfigured {
		t.Errorf("expected open/unconfigured with no hours set, got open=%v reason=%s", st.Open, st.Reason)
	}
}

func TestIsWithinBusinessHours_Port(t *testing.T) {
	svc := newSvc(weekdaysDoc("UTC"))
	within, err := svc.IsWithinBusinessHours(ctxT(), "s1", mustUTC(t, "2025-01-06T13:00:00Z"))
	if err != nil {
		t.Fatalf("within: %v", err)
	}
	if !within {
		t.Errorf("expected within business hours")
	}
	var _ shared.BusinessHoursChecker = svc
}

func TestStatus_InvalidTimezoneFallsBackToUTC(t *testing.T) {
	svc := newSvc(weekdaysDoc("Mars/Phobos"))
	st, err := svc.Status(ctxT(), "s1", mustUTC(t, "2025-01-06T13:00:00Z"))
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Timezone != "UTC" || !st.Open {
		t.Errorf("expected UTC fallback and open at 13:00 UTC, got tz=%s open=%v", st.Timezone, st.Open)
	}
}

// ── business-clock arithmetic (used by SLA for "horário útil") ───────────────

func TestAddBusinessDuration_WalksOpenTime(t *testing.T) {
	svc := newSvc(weekdaysDoc("UTC")) // mon-fri 09:00-18:00 UTC
	// Monday 17:00 + 2 business hours → 1h Mon (17-18) + 1h Tue (09-10) = Tue 10:00.
	from := mustUTC(t, "2025-01-06T17:00:00Z")
	due, err := svc.AddBusinessDuration(ctxT(), "s1", from, 2*time.Hour)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	want := mustUTC(t, "2025-01-07T10:00:00Z")
	if !due.Equal(want) {
		t.Errorf("due = %s, want %s", due.UTC().Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestAddBusinessDuration_SkipsWeekendAndHoliday(t *testing.T) {
	// Friday 17:00 + 2h → 1h Fri, then skip Sat/Sun, 1h Mon 09-10 = Mon 10:00.
	svc := newSvc(weekdaysDoc("UTC"))
	from := mustUTC(t, "2025-01-10T17:00:00Z") // 2025-01-10 is a Friday
	due, _ := svc.AddBusinessDuration(ctxT(), "s1", from, 2*time.Hour)
	want := mustUTC(t, "2025-01-13T10:00:00Z") // Monday
	if !due.Equal(want) {
		t.Errorf("due = %s, want %s", due.UTC().Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestBusinessDurationBetween_CountsOpenTimeOnly(t *testing.T) {
	svc := newSvc(weekdaysDoc("UTC"))
	// Mon 17:00 → Tue 10:00 spans 17h wall, but only 2h business (1h Mon + 1h Tue).
	d, err := svc.BusinessDurationBetween(ctxT(), "s1",
		mustUTC(t, "2025-01-06T17:00:00Z"), mustUTC(t, "2025-01-07T10:00:00Z"))
	if err != nil {
		t.Fatalf("between: %v", err)
	}
	if d != 2*time.Hour {
		t.Errorf("business duration = %s, want 2h", d)
	}
}

func TestAddBusinessDuration_Unconfigured_IsWallClock(t *testing.T) {
	svc := newSvc(nil) // no business hours → 24/7
	from := mustUTC(t, "2025-01-04T03:00:00Z")
	due, _ := svc.AddBusinessDuration(ctxT(), "s1", from, 90*time.Minute)
	if !due.Equal(from.Add(90 * time.Minute)) {
		t.Errorf("unconfigured should be wall-clock add, got %s", due)
	}
}

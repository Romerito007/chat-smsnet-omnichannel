package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sla/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sla/entity"
)

// ── clocks & fakes ───────────────────────────────────────────────────────────

type mutableClock struct{ t time.Time }

func (c *mutableClock) Now() time.Time  { return c.t }
func (c *mutableClock) set(t time.Time) { c.t = t }

type fakePolicyRepo struct{ policies []*entity.SLAPolicy }

func (r *fakePolicyRepo) Create(context.Context, *entity.SLAPolicy) error { return nil }
func (r *fakePolicyRepo) Update(context.Context, *entity.SLAPolicy) error { return nil }
func (r *fakePolicyRepo) Delete(context.Context, string) error            { return nil }
func (r *fakePolicyRepo) FindByID(context.Context, string) (*entity.SLAPolicy, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakePolicyRepo) List(context.Context, shared.PageRequest) ([]*entity.SLAPolicy, error) {
	return r.policies, nil
}
func (r *fakePolicyRepo) ListEnabled(context.Context) ([]*entity.SLAPolicy, error) {
	var out []*entity.SLAPolicy
	for _, p := range r.policies {
		if p.Enabled {
			out = append(out, p)
		}
	}
	return out, nil
}

type fakeTrackingRepo struct {
	byConv map[string]*entity.SLATracking
}

func newTrackingRepo() *fakeTrackingRepo {
	return &fakeTrackingRepo{byConv: map[string]*entity.SLATracking{}}
}
func (r *fakeTrackingRepo) Create(_ context.Context, t *entity.SLATracking) error {
	cp := *t
	r.byConv[t.ConversationID] = &cp
	return nil
}
func (r *fakeTrackingRepo) Update(_ context.Context, t *entity.SLATracking) error {
	cp := *t
	r.byConv[t.ConversationID] = &cp
	return nil
}
func (r *fakeTrackingRepo) FindByConversation(_ context.Context, id string) (*entity.SLATracking, error) {
	if t, ok := r.byConv[id]; ok {
		cp := *t
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeTrackingRepo) ListAtRisk(_ context.Context, _ shared.PageRequest) ([]*entity.SLATracking, error) {
	var out []*entity.SLATracking
	for _, t := range r.byConv {
		if t.Status == entity.StatusRunning && t.AtRisk() {
			out = append(out, t)
		}
	}
	return out, nil
}
func (r *fakeTrackingRepo) ListRunningAcrossTenants(_ context.Context, _ int) ([]*entity.SLATracking, error) {
	var out []*entity.SLATracking
	for _, t := range r.byConv {
		if t.Status == entity.StatusRunning {
			cp := *t
			out = append(out, &cp)
		}
	}
	return out, nil
}

type fakeConvRepo struct {
	items      map[string]*conventity.Conversation
	findByIDN  int // calls to the per-item FindByID
	findByIDsN int // calls to the batch FindByIDs
}

func (r *fakeConvRepo) Create(context.Context, *conventity.Conversation) error { return nil }
func (r *fakeConvRepo) Update(context.Context, *conventity.Conversation) error { return nil }
func (r *fakeConvRepo) FindByID(_ context.Context, id string) (*conventity.Conversation, error) {
	r.findByIDN++
	if c, ok := r.items[id]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindByIDs(_ context.Context, ids []string) ([]*conventity.Conversation, error) {
	r.findByIDsN++
	var out []*conventity.Conversation
	for _, id := range ids {
		if c, ok := r.items[id]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *fakeConvRepo) FindLastByContactChannelID(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindOpenByContactChannelID(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) ListInactiveOpen(context.Context, time.Time, int) ([]*conventity.Conversation, error) {
	return nil, nil
}
func (r *fakeConvRepo) List(context.Context, convcontracts.ListFilter, convcontracts.Visibility, shared.PageRequest) ([]*conventity.Conversation, error) {
	return nil, nil
}

type captured struct{ topic, event string }
type fakePublisher struct{ events []captured }

func (p *fakePublisher) Publish(_ context.Context, topic, event string, _ any) error {
	p.events = append(p.events, captured{topic, event})
	return nil
}
func (p *fakePublisher) has(event string) bool {
	for _, e := range p.events {
		if e.event == event {
			return true
		}
	}
	return false
}

type fakeWebhooks struct{ events []string }

func (w *fakeWebhooks) Emit(_ context.Context, _, event, _ string, _ any) {
	w.events = append(w.events, event)
}
func (w *fakeWebhooks) has(event string) bool {
	for _, e := range w.events {
		if e == event {
			return true
		}
	}
	return false
}

// recordingBizClock reports that business mode was used and applies a fixed
// offset so tests can prove the SLA service delegated to it.
type recordingBizClock struct{ calls int }

func (b *recordingBizClock) AddBusinessDuration(_ context.Context, _ string, from time.Time, d time.Duration) (time.Time, error) {
	b.calls++
	return from.Add(d).Add(time.Hour), nil // sentinel: +1h marker
}
func (b *recordingBizClock) BusinessDurationBetween(_ context.Context, _ string, from, to time.Time) (time.Duration, error) {
	return to.Sub(from), nil
}

// ── fixtures ─────────────────────────────────────────────────────────────────

const t0 = "2025-01-06T12:00:00Z"

func ctxT() context.Context { return shared.WithTenant(context.Background(), "t1") }

func mustT(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("time: %v", err)
	}
	return tm
}

func policy() *entity.SLAPolicy {
	return &entity.SLAPolicy{
		ID: "p1", TenantID: "t1", Name: "Default", Enabled: true,
		FirstResponseTargetSec: 60, ResolutionTargetSec: 600, WarningThresholdPct: 80,
	}
}

func conv(start time.Time) *conventity.Conversation {
	return &conventity.Conversation{
		ID: "cv1", TenantID: "t1", Channel: "whatsapp", SectorID: "s1",
		Priority: conventity.PriorityNormal, Status: conventity.StatusNew, CreatedAt: start,
	}
}

type fixture struct {
	svc      *Service
	tracking *fakeTrackingRepo
	convs    *fakeConvRepo
	pub      *fakePublisher
	wh       *fakeWebhooks
	clock    *mutableClock
	biz      *recordingBizClock
}

func newFixture(p *entity.SLAPolicy, c *conventity.Conversation) fixture {
	tr := newTrackingRepo()
	convs := &fakeConvRepo{items: map[string]*conventity.Conversation{c.ID: c}}
	pub := &fakePublisher{}
	wh := &fakeWebhooks{}
	clk := &mutableClock{t: c.CreatedAt}
	biz := &recordingBizClock{}
	svc := NewService(&fakePolicyRepo{policies: []*entity.SLAPolicy{p}}, tr, convs, biz, pub, wh, clk)
	return fixture{svc: svc, tracking: tr, convs: convs, pub: pub, wh: wh, clock: clk, biz: biz}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestSelectPolicy_MostSpecific(t *testing.T) {
	generic := &entity.SLAPolicy{ID: "g", Enabled: true, FirstResponseTargetSec: 60}
	bySector := &entity.SLAPolicy{ID: "s", Enabled: true, SectorIDs: []string{"s1"}, FirstResponseTargetSec: 30}
	byChannel := &entity.SLAPolicy{ID: "c", Enabled: true, Channel: "whatsapp", FirstResponseTargetSec: 45}
	got := selectPolicy([]*entity.SLAPolicy{generic, byChannel, bySector}, conv(time.Now()))
	if got == nil || got.ID != "s" {
		t.Errorf("expected the sector policy (most specific), got %+v", got)
	}
}

func TestOnCreated_NoBusinessHours_WallClockDue(t *testing.T) {
	start := mustT(t, t0)
	fx := newFixture(policy(), conv(start))
	fx.svc.OnConversationCreated(ctxT(), conv(start))

	tr := fx.tracking.byConv["cv1"]
	if tr == nil {
		t.Fatal("tracking not created")
	}
	if !tr.FirstResponseDueAt.Equal(start.Add(60 * time.Second)) {
		t.Errorf("first_response_due = %s, want +60s", tr.FirstResponseDueAt)
	}
	if !tr.FirstResponseWarnAt.Equal(start.Add(48 * time.Second)) { // 80% of 60
		t.Errorf("first_response_warn = %s, want +48s", tr.FirstResponseWarnAt)
	}
	if !tr.ResolutionDueAt.Equal(start.Add(600 * time.Second)) {
		t.Errorf("resolution_due = %s, want +600s", tr.ResolutionDueAt)
	}
	if fx.biz.calls != 0 {
		t.Errorf("business clock should not be used when business_hours_only=false")
	}
}

func TestOnCreated_BusinessHoursOnly_UsesBusinessClock(t *testing.T) {
	p := policy()
	p.BusinessHoursOnly = true
	start := mustT(t, t0)
	fx := newFixture(p, conv(start))
	fx.svc.OnConversationCreated(ctxT(), conv(start))

	tr := fx.tracking.byConv["cv1"]
	// recordingBizClock adds a +1h sentinel, proving delegation.
	if !tr.FirstResponseDueAt.Equal(start.Add(60*time.Second + time.Hour)) {
		t.Errorf("expected business-clock due (+1h sentinel), got %s", tr.FirstResponseDueAt)
	}
	if fx.biz.calls == 0 {
		t.Errorf("business clock should be consulted when business_hours_only=true")
	}
}

func TestFirstResponse_RecordedAndBreach(t *testing.T) {
	start := mustT(t, t0)
	fx := newFixture(policy(), conv(start))
	fx.svc.OnConversationCreated(ctxT(), conv(start))

	// On-time response.
	fx.svc.OnFirstResponse(ctxT(), conv(start), start.Add(30*time.Second))
	tr := fx.tracking.byConv["cv1"]
	if tr.FirstResponseAt == nil || tr.FirstResponseBreached {
		t.Errorf("expected on-time first response, got %+v", tr)
	}
	// A second call is ignored (idempotent).
	fx.svc.OnFirstResponse(ctxT(), conv(start), start.Add(90*time.Second))
	if !fx.tracking.byConv["cv1"].FirstResponseAt.Equal(start.Add(30 * time.Second)) {
		t.Errorf("first response time should not change on second call")
	}
}

func TestFirstResponse_LateBreaches(t *testing.T) {
	start := mustT(t, t0)
	fx := newFixture(policy(), conv(start))
	fx.svc.OnConversationCreated(ctxT(), conv(start))
	fx.svc.OnFirstResponse(ctxT(), conv(start), start.Add(120*time.Second)) // > 60s
	if !fx.tracking.byConv["cv1"].FirstResponseBreached {
		t.Errorf("late first response should breach")
	}
}

func TestResolved_FinalizesStatus(t *testing.T) {
	start := mustT(t, t0)
	fx := newFixture(policy(), conv(start))
	fx.svc.OnConversationCreated(ctxT(), conv(start))
	fx.svc.OnFirstResponse(ctxT(), conv(start), start.Add(10*time.Second))
	fx.svc.OnResolved(ctxT(), conv(start), start.Add(300*time.Second)) // < 600s
	tr := fx.tracking.byConv["cv1"]
	if tr.ResolvedAt == nil || tr.Status != entity.StatusMet {
		t.Errorf("expected met status, got %+v", tr.Status)
	}
}

// RunCheck hydrates conversations in batch ($in per tenant), never one FindByID
// per tracking.
func TestRunCheck_BatchHydratesNoFindByIDPerItem(t *testing.T) {
	start := mustT(t, t0)
	fx := newFixture(policy(), conv(start))
	fx.svc.OnConversationCreated(ctxT(), conv(start))
	fx.convs.findByIDN, fx.convs.findByIDsN = 0, 0

	fx.clock.set(start.Add(61 * time.Second))
	if err := fx.svc.RunCheck(ctxT()); err != nil {
		t.Fatalf("check: %v", err)
	}
	if fx.convs.findByIDN != 0 {
		t.Errorf("RunCheck must not FindByID per tracking, got %d calls", fx.convs.findByIDN)
	}
	if fx.convs.findByIDsN != 1 {
		t.Errorf("RunCheck must batch-hydrate with one FindByIDs per tenant, got %d", fx.convs.findByIDsN)
	}
}

func TestRunCheck_WarningThenBreach(t *testing.T) {
	start := mustT(t, t0)
	fx := newFixture(policy(), conv(start))
	fx.svc.OnConversationCreated(ctxT(), conv(start))

	// At +50s: first-response warning (warn at 48s, due at 60s), no breach/webhook.
	fx.clock.set(start.Add(50 * time.Second))
	if err := fx.svc.RunCheck(ctxT()); err != nil {
		t.Fatalf("check: %v", err)
	}
	tr := fx.tracking.byConv["cv1"]
	if !tr.FirstResponseWarned || tr.FirstResponseBreached {
		t.Errorf("expected warned, not breached at +50s: %+v", tr)
	}
	if !fx.pub.has(contracts.RealtimeSLAWarning) {
		t.Errorf("expected realtime sla.warning")
	}
	if fx.wh.has(contracts.RealtimeSLABreached) {
		t.Errorf("no webhook should fire for a warning")
	}

	// At +61s: first-response breach → realtime + webhook sla.breached.
	fx.clock.set(start.Add(61 * time.Second))
	if err := fx.svc.RunCheck(ctxT()); err != nil {
		t.Fatalf("check: %v", err)
	}
	tr = fx.tracking.byConv["cv1"]
	if !tr.FirstResponseBreached {
		t.Errorf("expected first-response breach at +61s")
	}
	if !fx.pub.has(contracts.RealtimeSLABreached) {
		t.Errorf("expected realtime sla.breached")
	}
	if !fx.wh.has(contracts.RealtimeSLABreached) {
		t.Errorf("expected sla.breached webhook")
	}
	// Status stays running until the conversation resolves.
	if tr.Status != entity.StatusRunning {
		t.Errorf("status should remain running while open, got %s", tr.Status)
	}
}

func TestRunCheck_PauseOnWaitingCustomer(t *testing.T) {
	p := policy()
	p.PauseOnWaitingCustomer = true
	start := mustT(t, t0)
	waiting := conv(start)
	waiting.Status = conventity.StatusWaitingCustomer
	fx := newFixture(p, waiting)
	fx.svc.OnConversationCreated(ctxT(), waiting)

	// Well past due, but paused → no breach, no events.
	fx.clock.set(start.Add(1 * time.Hour))
	if err := fx.svc.RunCheck(ctxT()); err != nil {
		t.Fatalf("check: %v", err)
	}
	tr := fx.tracking.byConv["cv1"]
	if tr.FirstResponseBreached || tr.ResolutionBreached {
		t.Errorf("paused tracking must not breach while waiting_customer")
	}
	if fx.wh.has(contracts.RealtimeSLABreached) {
		t.Errorf("paused tracking must not emit a webhook")
	}
}

func TestAtRisk_ListsWarnedOrBreached(t *testing.T) {
	start := mustT(t, t0)
	fx := newFixture(policy(), conv(start))
	fx.svc.OnConversationCreated(ctxT(), conv(start))

	// Not at risk yet.
	if items, _ := fx.svc.AtRisk(ctxT(), shared.PageRequest{}); len(items) != 0 {
		t.Errorf("expected no at-risk trackings initially, got %d", len(items))
	}
	// Trigger a warning.
	fx.clock.set(start.Add(50 * time.Second))
	_ = fx.svc.RunCheck(ctxT())
	items, _ := fx.svc.AtRisk(ctxT(), shared.PageRequest{})
	if len(items) != 1 {
		t.Errorf("expected 1 at-risk tracking after warning, got %d", len(items))
	}
}

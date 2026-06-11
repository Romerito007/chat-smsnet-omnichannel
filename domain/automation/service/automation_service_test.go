package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/entity"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	routingcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/routing/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeIntegrations struct{ enabled *entity.AutomationIntegration }

func (r *fakeIntegrations) Create(context.Context, *entity.AutomationIntegration) error { return nil }
func (r *fakeIntegrations) Update(context.Context, *entity.AutomationIntegration) error { return nil }
func (r *fakeIntegrations) Delete(context.Context, string) error                        { return nil }
func (r *fakeIntegrations) FindByID(context.Context, string) (*entity.AutomationIntegration, error) {
	return r.enabled, nil
}
func (r *fakeIntegrations) List(context.Context, shared.PageRequest) ([]*entity.AutomationIntegration, error) {
	return nil, nil
}
func (r *fakeIntegrations) FindEnabled(context.Context) (*entity.AutomationIntegration, error) {
	if r.enabled == nil {
		return nil, apperror.NotFound("nf")
	}
	return r.enabled, nil
}

type fakeRuns struct {
	byID  map[string]*entity.AutomationRun
	byExt map[string]*entity.AutomationRun
}

func newFakeRuns() *fakeRuns {
	return &fakeRuns{byID: map[string]*entity.AutomationRun{}, byExt: map[string]*entity.AutomationRun{}}
}
func (r *fakeRuns) put(run *entity.AutomationRun) {
	cp := *run
	r.byID[run.ID] = &cp
	if run.ExternalRunID != "" {
		r.byExt[run.ExternalRunID] = &cp
	}
}
func (r *fakeRuns) Create(_ context.Context, run *entity.AutomationRun) error { r.put(run); return nil }
func (r *fakeRuns) Update(_ context.Context, run *entity.AutomationRun) error { r.put(run); return nil }
func (r *fakeRuns) FindByID(_ context.Context, id string) (*entity.AutomationRun, error) {
	if run, ok := r.byID[id]; ok {
		cp := *run
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRuns) FindByExternalRunID(_ context.Context, ext string) (*entity.AutomationRun, error) {
	if run, ok := r.byExt[ext]; ok {
		cp := *run
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRuns) List(context.Context, shared.PageRequest) ([]*entity.AutomationRun, error) {
	return nil, nil
}

type fakeConvRepo struct {
	items map[string]*conventity.Conversation
}

func (r *fakeConvRepo) Create(_ context.Context, c *conventity.Conversation) error {
	cp := *c
	r.items[c.ID] = &cp
	return nil
}
func (r *fakeConvRepo) Update(_ context.Context, c *conventity.Conversation) error {
	cp := *c
	r.items[c.ID] = &cp
	return nil
}
func (r *fakeConvRepo) FindByID(_ context.Context, id string) (*conventity.Conversation, error) {
	if c, ok := r.items[id]; ok {
		cp := *c
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindOpenByContactChannel(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) ListInactiveOpen(context.Context, time.Time, int) ([]*conventity.Conversation, error) {
	return nil, nil
}
func (r *fakeConvRepo) List(context.Context, convcontracts.ListFilter, convcontracts.Visibility, shared.PageRequest) ([]*conventity.Conversation, error) {
	return nil, nil
}

type fakeMsgRepo struct{ items []*conventity.Message }

func (r *fakeMsgRepo) Create(_ context.Context, m *conventity.Message) error {
	r.items = append(r.items, m)
	return nil
}
func (r *fakeMsgRepo) Update(context.Context, *conventity.Message) error { return nil }
func (r *fakeMsgRepo) FindByID(_ context.Context, id string) (*conventity.Message, error) {
	for _, m := range r.items {
		if m.ID == id {
			return m, nil
		}
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeMsgRepo) ListByConversation(context.Context, string, shared.PageRequest) ([]*conventity.Message, error) {
	return nil, nil
}

type fakeEventRepo struct {
	items []*conventity.ConversationEvent
}

func (r *fakeEventRepo) Create(_ context.Context, e *conventity.ConversationEvent) error {
	r.items = append(r.items, e)
	return nil
}
func (r *fakeEventRepo) ListByConversation(context.Context, string, shared.PageRequest) ([]*conventity.ConversationEvent, error) {
	return r.items, nil
}
func (r *fakeEventRepo) count(eventType string) int {
	n := 0
	for _, e := range r.items {
		if e.Type == eventType {
			n++
		}
	}
	return n
}

type fakeRouter struct{ assigns, transfers, enqueues int }

func (r *fakeRouter) Assign(_ context.Context, _, _ string) (*conventity.Conversation, error) {
	r.assigns++
	return &conventity.Conversation{}, nil
}
func (r *fakeRouter) Transfer(_ context.Context, _ string, _ routingcontracts.TransferCommand) (*conventity.Conversation, error) {
	r.transfers++
	return &conventity.Conversation{}, nil
}
func (r *fakeRouter) Enqueue(_ context.Context, _ string, _ routingcontracts.EnqueueCommand) (*conventity.Conversation, error) {
	r.enqueues++
	return &conventity.Conversation{}, nil
}

type fakeFlow struct {
	result contracts.FlowStartResult
	err    error
}

func (f fakeFlow) Start(context.Context, *entity.AutomationIntegration, contracts.FlowInput) (contracts.FlowStartResult, error) {
	return f.result, f.err
}

type fakeOutbound struct{ dispatched int }

func (o *fakeOutbound) Dispatch(context.Context, *conventity.Conversation, *conventity.Message) {
	o.dispatched++
}

type fakeTimeouts struct{ scheduled int }

func (t *fakeTimeouts) ScheduleTimeout(contracts.TimeoutTask, int) error { t.scheduled++; return nil }

// ── fixture ──────────────────────────────────────────────────────────────────

type fixture struct {
	svc      *Service
	runs     *fakeRuns
	convs    *fakeConvRepo
	msgs     *fakeMsgRepo
	events   *fakeEventRepo
	router   *fakeRouter
	outbound *fakeOutbound
	timeouts *fakeTimeouts
}

const secret = "s3cr3t"

func newFixture(flow fakeFlow) fixture {
	integ := &entity.AutomationIntegration{ID: "i1", TenantID: "t1", Secret: secret, Enabled: true, TimeoutMs: 5000, BaseURL: "http://flow"}
	runs := newFakeRuns()
	convs := &fakeConvRepo{items: map[string]*conventity.Conversation{
		"conv1": {ID: "conv1", TenantID: "t1", Channel: "whatsapp", Status: conventity.StatusAutomation, ContactID: "c1"},
	}}
	msgs := &fakeMsgRepo{}
	events := &fakeEventRepo{}
	router := &fakeRouter{}
	outbound := &fakeOutbound{}
	timeouts := &fakeTimeouts{}
	svc := New(&fakeIntegrations{enabled: integ}, runs, convs, msgs, events, router, outbound,
		flow, timeouts, shared.NoopPublisher{}, fixedClock{t: time.Unix(1700000000, 0).UTC()}, "http://host")
	return fixture{svc: svc, runs: runs, convs: convs, msgs: msgs, events: events, router: router, outbound: outbound, timeouts: timeouts}
}

func tctx() context.Context { return shared.WithTenant(context.Background(), "t1") }

func sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestStart_AsyncWaitsForCallbackAndSchedulesTimeout(t *testing.T) {
	fx := newFixture(fakeFlow{result: contracts.FlowStartResult{ExternalRunID: "run-ext-1"}})
	if err := fx.svc.StartConversationAutomation(tctx(), "conv1", "msg1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if len(fx.runs.byExt) != 1 {
		t.Fatal("expected a run keyed by external id")
	}
	run := fx.runs.byExt["run-ext-1"]
	if run.Status != entity.RunWaitingCallback {
		t.Errorf("status = %q, want waiting_callback", run.Status)
	}
	if fx.timeouts.scheduled != 1 {
		t.Errorf("expected a timeout scheduled, got %d", fx.timeouts.scheduled)
	}
}

func TestStart_SyncDecisionAppliedImmediately(t *testing.T) {
	fx := newFixture(fakeFlow{result: contracts.FlowStartResult{
		ExternalRunID: "run-ext-2",
		Decision:      &contracts.Decision{Type: contracts.DecisionSendMessage, Text: "hi from bot"},
	}})
	if err := fx.svc.StartConversationAutomation(tctx(), "conv1", "msg1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	run := fx.runs.byExt["run-ext-2"]
	if run.Status != entity.RunCompleted {
		t.Errorf("status = %q, want completed", run.Status)
	}
	if len(fx.msgs.items) != 1 || fx.msgs.items[0].SenderType != conventity.SenderAutomation {
		t.Errorf("send_message should create an automation message")
	}
	if fx.outbound.dispatched != 1 {
		t.Errorf("automation message should be dispatched for delivery")
	}
}

func TestStart_NoIntegrationEscalates(t *testing.T) {
	fx := newFixture(fakeFlow{})
	fx.svc = New(&fakeIntegrations{enabled: nil}, fx.runs, fx.convs, fx.msgs, fx.events, fx.router, fx.outbound,
		fakeFlow{}, fx.timeouts, shared.NoopPublisher{}, fixedClock{t: time.Unix(1700000000, 0).UTC()}, "http://host")
	if err := fx.svc.StartConversationAutomation(tctx(), "conv1", "msg1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if fx.convs.items["conv1"].Status != conventity.StatusQueued {
		t.Errorf("expected escalation to queued, got %s", fx.convs.items["conv1"].Status)
	}
	if fx.events.count(conventity.EventAutomationEscalated) != 1 {
		t.Error("expected an automation.escalated event")
	}
}

func TestStart_FlowErrorRecordsFailedAndEscalates(t *testing.T) {
	fx := newFixture(fakeFlow{err: apperror.Integration("flow down")})
	if err := fx.svc.StartConversationAutomation(tctx(), "conv1", "msg1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	var run *entity.AutomationRun
	for _, r := range fx.runs.byID {
		run = r
	}
	if run.Status != entity.RunFailed {
		t.Errorf("run status = %q, want failed", run.Status)
	}
	if fx.convs.items["conv1"].Status != conventity.StatusQueued {
		t.Error("flow failure should escalate the conversation")
	}
}

func TestCallback_AppliesDecisionAndCompletes(t *testing.T) {
	fx := newFixture(fakeFlow{result: contracts.FlowStartResult{ExternalRunID: "run-cb-1"}})
	_ = fx.svc.StartConversationAutomation(tctx(), "conv1", "msg1") // waiting_callback

	cb := contracts.Callback{ExternalRunID: "run-cb-1", Decision: &contracts.Decision{Type: contracts.DecisionAssignAgent, AgentID: "agent9"}}
	body, _ := json.Marshal(cb)
	if err := fx.svc.HandleCallback(context.Background(), "t1", body, "sha256="+sign(body)); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if fx.runs.byExt["run-cb-1"].Status != entity.RunCompleted {
		t.Errorf("run should be completed")
	}
	if fx.router.assigns != 1 {
		t.Errorf("assign_agent decision should call routing Assign")
	}
}

func TestCallback_InvalidSignatureRejected(t *testing.T) {
	fx := newFixture(fakeFlow{result: contracts.FlowStartResult{ExternalRunID: "run-cb-2"}})
	_ = fx.svc.StartConversationAutomation(tctx(), "conv1", "msg1")
	body, _ := json.Marshal(contracts.Callback{ExternalRunID: "run-cb-2"})
	if err := fx.svc.HandleCallback(context.Background(), "t1", body, "sha256=deadbeef"); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Errorf("expected unauthorized, got %v", err)
	}
}

func TestCallback_Idempotent(t *testing.T) {
	fx := newFixture(fakeFlow{result: contracts.FlowStartResult{ExternalRunID: "run-cb-3"}})
	_ = fx.svc.StartConversationAutomation(tctx(), "conv1", "msg1")
	cb := contracts.Callback{ExternalRunID: "run-cb-3", Decision: &contracts.Decision{Type: contracts.DecisionNoAction}}
	body, _ := json.Marshal(cb)
	sig := "sha256=" + sign(body)
	if err := fx.svc.HandleCallback(context.Background(), "t1", body, sig); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Second callback on a now-terminal run is a no-op.
	if err := fx.svc.HandleCallback(context.Background(), "t1", body, sig); err != nil {
		t.Fatalf("second: %v", err)
	}
	if fx.runs.byExt["run-cb-3"].Status != entity.RunCompleted {
		t.Errorf("run should remain completed")
	}
}

func TestCallback_ErrorEscalates(t *testing.T) {
	fx := newFixture(fakeFlow{result: contracts.FlowStartResult{ExternalRunID: "run-cb-4"}})
	_ = fx.svc.StartConversationAutomation(tctx(), "conv1", "msg1")
	body, _ := json.Marshal(contracts.Callback{ExternalRunID: "run-cb-4", Error: "flow failed"})
	if err := fx.svc.HandleCallback(context.Background(), "t1", body, "sha256="+sign(body)); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if fx.runs.byExt["run-cb-4"].Status != entity.RunFailed {
		t.Errorf("run should be failed")
	}
	if fx.convs.items["conv1"].Status != conventity.StatusQueued {
		t.Error("error callback should escalate to human")
	}
}

func TestTimeout_MarksTimeoutAndEscalates(t *testing.T) {
	fx := newFixture(fakeFlow{result: contracts.FlowStartResult{ExternalRunID: "run-to-1"}})
	_ = fx.svc.StartConversationAutomation(tctx(), "conv1", "msg1") // waiting_callback
	var runID string
	for id := range fx.runs.byID {
		runID = id
	}
	if err := fx.svc.HandleTimeout(tctx(), runID); err != nil {
		t.Fatalf("timeout: %v", err)
	}
	if fx.runs.byID[runID].Status != entity.RunTimeout {
		t.Errorf("run should be timeout")
	}
	if fx.convs.items["conv1"].Status != conventity.StatusQueued {
		t.Error("timeout should escalate to human")
	}
}

func TestApplyDecision_RecordsEvent(t *testing.T) {
	fx := newFixture(fakeFlow{})
	run := &entity.AutomationRun{ID: "r1", TenantID: "t1", ConversationID: "conv1"}
	if err := fx.svc.ApplyDecision(tctx(), run, contracts.Decision{Type: contracts.DecisionAddTag, Tag: "vip"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if fx.events.count(conventity.EventAutomationDecision) != 1 {
		t.Error("every decision must record an automation.decision event")
	}
	found := false
	for _, tag := range fx.convs.items["conv1"].Tags {
		if tag == "vip" {
			found = true
		}
	}
	if !found {
		t.Error("add_tag decision should add the tag")
	}
}

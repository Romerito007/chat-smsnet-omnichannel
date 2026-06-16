package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeSurveyRepo struct{ items map[string]*entity.CSATSurvey }

func (r *fakeSurveyRepo) Create(context.Context, *entity.CSATSurvey) error { return nil }
func (r *fakeSurveyRepo) Update(context.Context, *entity.CSATSurvey) error { return nil }
func (r *fakeSurveyRepo) Delete(context.Context, string) error             { return nil }
func (r *fakeSurveyRepo) FindByID(_ context.Context, id string) (*entity.CSATSurvey, error) {
	if s, ok := r.items[id]; ok {
		return s, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeSurveyRepo) List(context.Context, shared.PageRequest) ([]*entity.CSATSurvey, error) {
	return nil, nil
}
func (r *fakeSurveyRepo) ListEnabled(context.Context) ([]*entity.CSATSurvey, error) {
	var out []*entity.CSATSurvey
	for _, s := range r.items {
		if s.Enabled {
			out = append(out, s)
		}
	}
	return out, nil
}

type fakeResponseRepo struct {
	byID    map[string]*entity.CSATResponse
	byConv  map[string]*entity.CSATResponse
	byToken map[string]*entity.CSATResponse
}

func newResponseRepo() *fakeResponseRepo {
	return &fakeResponseRepo{
		byID:    map[string]*entity.CSATResponse{},
		byConv:  map[string]*entity.CSATResponse{},
		byToken: map[string]*entity.CSATResponse{},
	}
}
func (r *fakeResponseRepo) put(resp *entity.CSATResponse) {
	cp := *resp
	r.byID[resp.ID] = &cp
	r.byConv[resp.ConversationID] = &cp
	r.byToken[resp.Token] = &cp
}
func (r *fakeResponseRepo) Create(_ context.Context, resp *entity.CSATResponse) error {
	r.put(resp)
	return nil
}
func (r *fakeResponseRepo) Update(_ context.Context, resp *entity.CSATResponse) error {
	r.put(resp)
	return nil
}
func (r *fakeResponseRepo) FindByID(_ context.Context, id string) (*entity.CSATResponse, error) {
	if x, ok := r.byID[id]; ok {
		cp := *x
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeResponseRepo) FindByToken(_ context.Context, token string) (*entity.CSATResponse, error) {
	if x, ok := r.byToken[token]; ok {
		cp := *x
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeResponseRepo) FindByConversation(_ context.Context, convID string) (*entity.CSATResponse, error) {
	if x, ok := r.byConv[convID]; ok {
		cp := *x
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeResponseRepo) List(context.Context, shared.PageRequest) ([]*entity.CSATResponse, error) {
	return nil, nil
}

type fakeSender struct {
	calls int
	last  string
	err   error
}

func (s *fakeSender) SendToConversation(_ context.Context, _, text string) error {
	s.calls++
	s.last = text
	return s.err
}

type enqueued struct {
	kind  string
	delay int
	id    string
}
type fakeEnqueuer struct{ items []enqueued }

func (e *fakeEnqueuer) EnqueueSend(t contracts.SendTask, delay int) error {
	e.items = append(e.items, enqueued{"send", delay, t.ResponseID})
	return nil
}
func (e *fakeEnqueuer) EnqueueExpire(t contracts.ExpireTask, delay int) error {
	e.items = append(e.items, enqueued{"expire", delay, t.ResponseID})
	return nil
}

// ── fixture ──────────────────────────────────────────────────────────────────

func ctxT() context.Context { return shared.WithTenant(context.Background(), "t1") }

type fixture struct {
	svc      *Service
	surveys  *fakeSurveyRepo
	respo    *fakeResponseRepo
	sender   *fakeSender
	enqueuer *fakeEnqueuer
}

func newFixture(surveys ...*entity.CSATSurvey) fixture {
	sr := &fakeSurveyRepo{items: map[string]*entity.CSATSurvey{}}
	for _, s := range surveys {
		sr.items[s.ID] = s
	}
	rr := newResponseRepo()
	sender := &fakeSender{}
	enq := &fakeEnqueuer{}
	svc := NewService(sr, rr, sender, enq, fixedClock{t: time.Unix(1700000000, 0).UTC()}, 3600, "https://app.example.com")
	return fixture{svc: svc, surveys: sr, respo: rr, sender: sender, enqueuer: enq}
}

func survey(id string, scale entity.Scale, sectors ...string) *entity.CSATSurvey {
	return &entity.CSATSurvey{
		ID: id, TenantID: "t1", Name: id, Scale: scale, QuestionText: "How did we do?",
		SendOn: entity.SendOnConversationClosed, SectorIDs: sectors, Enabled: true,
	}
}

func conv() *conventity.Conversation {
	return &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1", ContactID: "ct1", AssignedTo: "u1"}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestOnClosed_CreatesResponseAndEnqueuesSend(t *testing.T) {
	fx := newFixture(survey("sv1", entity.ScaleCSAT15))
	fx.svc.OnConversationClosed(ctxT(), conv())

	resp := fx.respo.byConv["cv1"]
	if resp == nil {
		t.Fatal("expected a response created on close")
	}
	if resp.Status != entity.StatusSent || resp.SurveyID != "sv1" || resp.Token == "" {
		t.Errorf("unexpected response: %+v", resp)
	}
	if resp.AgentID != "u1" || resp.ContactID != "ct1" {
		t.Errorf("response should capture agent/contact: %+v", resp)
	}
	if len(fx.enqueuer.items) != 1 || fx.enqueuer.items[0].kind != "send" {
		t.Errorf("expected a send enqueue, got %+v", fx.enqueuer.items)
	}
}

func TestOnClosed_NoResendSameConversation(t *testing.T) {
	fx := newFixture(survey("sv1", entity.ScaleCSAT15))
	fx.svc.OnConversationClosed(ctxT(), conv())
	fx.svc.OnConversationClosed(ctxT(), conv()) // second close
	if len(fx.enqueuer.items) != 1 {
		t.Errorf("must not re-send for the same conversation, got %d enqueues", len(fx.enqueuer.items))
	}
}

func TestOnClosed_SectorSurveyBeatsGlobal(t *testing.T) {
	fx := newFixture(survey("global", entity.ScaleNPS010), survey("sector", entity.ScaleCSAT15, "s1"))
	fx.svc.OnConversationClosed(ctxT(), conv())
	if got := fx.respo.byConv["cv1"].SurveyID; got != "sector" {
		t.Errorf("expected the sector-specific survey, got %q", got)
	}
}

func TestOnClosed_NoMatchingSurvey(t *testing.T) {
	fx := newFixture(survey("other", entity.ScaleCSAT15, "s2")) // different sector
	fx.svc.OnConversationClosed(ctxT(), conv())
	if _, ok := fx.respo.byConv["cv1"]; ok {
		t.Errorf("no survey should be created when none matches the sector")
	}
}

func TestSend_DeliversAndSchedulesExpire(t *testing.T) {
	fx := newFixture(survey("sv1", entity.ScaleCSAT15))
	fx.svc.OnConversationClosed(ctxT(), conv())
	resp := fx.respo.byConv["cv1"]

	if err := fx.svc.Send(ctxT(), contracts.SendTask{TenantID: "t1", ResponseID: resp.ID}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if fx.sender.calls != 1 {
		t.Errorf("expected the survey to be sent via the channel")
	}
	if fx.respo.byID[resp.ID].SentAt == nil {
		t.Errorf("sent_at should be set after delivery")
	}
	// The delivered text carries the public token link, not the conversation.
	if want := "https://app.example.com/csat/" + resp.Token; !contains(fx.sender.last, want) {
		t.Errorf("delivered text missing token link %q in %q", want, fx.sender.last)
	}
	// expire scheduled.
	expireScheduled := false
	for _, e := range fx.enqueuer.items {
		if e.kind == "expire" && e.delay == 3600 {
			expireScheduled = true
		}
	}
	if !expireScheduled {
		t.Errorf("expected an expire enqueue, got %+v", fx.enqueuer.items)
	}
}

func TestSubmitByToken_ValidScore(t *testing.T) {
	fx := newFixture(survey("sv1", entity.ScaleCSAT15))
	fx.svc.OnConversationClosed(ctxT(), conv())
	token := fx.respo.byConv["cv1"].Token

	// Public ctx has NO tenant — only the token validates.
	if err := fx.svc.SubmitByToken(context.Background(), token, contracts.SubmitResponse{Score: 5, Comment: "Great!"}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	resp := fx.respo.byToken[token]
	if resp.Status != entity.StatusResponded || resp.Score == nil || *resp.Score != 5 || resp.Comment != "Great!" {
		t.Errorf("unexpected response after submit: %+v", resp)
	}
	if resp.RespondedAt == nil {
		t.Errorf("responded_at should be set")
	}
}

func TestSubmitByToken_InvalidScorePerScale(t *testing.T) {
	fx := newFixture(survey("sv1", entity.ScaleCSAT15))
	fx.svc.OnConversationClosed(ctxT(), conv())
	token := fx.respo.byConv["cv1"].Token
	// 6 is out of range for csat_1_5.
	if err := fx.svc.SubmitByToken(context.Background(), token, contracts.SubmitResponse{Score: 6}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation error for out-of-range score, got %v", err)
	}
}

func TestSubmitByToken_UnknownToken(t *testing.T) {
	fx := newFixture(survey("sv1", entity.ScaleCSAT15))
	if err := fx.svc.SubmitByToken(context.Background(), "ghost", contracts.SubmitResponse{Score: 3}); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("expected not_found for unknown token, got %v", err)
	}
}

func TestSubmitByToken_AlreadyAnswered(t *testing.T) {
	fx := newFixture(survey("sv1", entity.ScaleCSAT15))
	fx.svc.OnConversationClosed(ctxT(), conv())
	token := fx.respo.byConv["cv1"].Token
	_ = fx.svc.SubmitByToken(context.Background(), token, contracts.SubmitResponse{Score: 4})
	if err := fx.svc.SubmitByToken(context.Background(), token, contracts.SubmitResponse{Score: 5}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("expected conflict on second answer, got %v", err)
	}
}

func TestExpire_OnlyWhenStillSent(t *testing.T) {
	fx := newFixture(survey("sv1", entity.ScaleCSAT15))
	fx.svc.OnConversationClosed(ctxT(), conv())
	resp := fx.respo.byConv["cv1"]

	// Expire an unanswered survey → expired.
	if err := fx.svc.Expire(ctxT(), contracts.ExpireTask{TenantID: "t1", ResponseID: resp.ID}); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if fx.respo.byID[resp.ID].Status != entity.StatusExpired {
		t.Errorf("expected expired status")
	}

	// Answered surveys are not expired.
	fx2 := newFixture(survey("sv1", entity.ScaleCSAT15))
	fx2.svc.OnConversationClosed(ctxT(), conv())
	r2 := fx2.respo.byConv["cv1"]
	_ = fx2.svc.SubmitByToken(context.Background(), r2.Token, contracts.SubmitResponse{Score: 5})
	_ = fx2.svc.Expire(ctxT(), contracts.ExpireTask{TenantID: "t1", ResponseID: r2.ID})
	if fx2.respo.byID[r2.ID].Status != entity.StatusResponded {
		t.Errorf("answered survey must not be expired, got %s", fx2.respo.byID[r2.ID].Status)
	}
}

// Orphan tasks (response gone after a DB reset / deletion) must succeed as a
// no-op, not error — otherwise Asynq retries to exhaustion and floods the logs.
func TestExpireAndSend_OrphanTaskIsNoOp(t *testing.T) {
	fx := newFixture()

	if err := fx.svc.Expire(ctxT(), contracts.ExpireTask{TenantID: "t1", ResponseID: "ghost"}); err != nil {
		t.Errorf("expire of a missing response must be a no-op, got %v", err)
	}
	if err := fx.svc.Send(ctxT(), contracts.SendTask{TenantID: "t1", ResponseID: "ghost"}); err != nil {
		t.Errorf("send of a missing response must be a no-op, got %v", err)
	}
	if fx.sender.calls != 0 {
		t.Errorf("orphan send must not deliver anything, got %d calls", fx.sender.calls)
	}
}

func TestScale_ValidScore(t *testing.T) {
	cases := []struct {
		scale entity.Scale
		score int
		ok    bool
	}{
		{entity.ScaleCSAT15, 1, true}, {entity.ScaleCSAT15, 5, true}, {entity.ScaleCSAT15, 0, false}, {entity.ScaleCSAT15, 6, false},
		{entity.ScaleNPS010, 0, true}, {entity.ScaleNPS010, 10, true}, {entity.ScaleNPS010, 11, false}, {entity.ScaleNPS010, -1, false},
		{entity.ScaleThumbs, 0, true}, {entity.ScaleThumbs, 1, true}, {entity.ScaleThumbs, 2, false},
	}
	for _, c := range cases {
		if got := c.scale.ValidScore(c.score); got != c.ok {
			t.Errorf("%s ValidScore(%d) = %v, want %v", c.scale, c.score, got, c.ok)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

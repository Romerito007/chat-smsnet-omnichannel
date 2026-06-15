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
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	channelrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
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
func (r *fakeConvRepo) FindByIDs(_ context.Context, ids []string) ([]*entity.Conversation, error) {
	var out []*entity.Conversation
	for _, id := range ids {
		for _, c := range r.items {
			if c.ID == id {
				out = append(out, c)
			}
		}
	}
	return out, nil
}
func (r *fakeConvRepo) FindLastByContactChannelID(context.Context, string, string) (*entity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindOpenByContactChannelID(context.Context, string, string) (*entity.Conversation, error) {
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

// fakeChannelConnRepo resolves "ch1" to a whatsapp connection so Create can derive
// the channel type; any other id is NotFound.
type fakeChannelConnRepo struct {
	channelrepo.ConnectionRepository
}

func (fakeChannelConnRepo) FindByID(ctx context.Context, id string) (*chentity.ChannelConnection, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if id == "ch1" {
		return &chentity.ChannelConnection{ID: id, TenantID: tenant, Type: chentity.TypeWhatsApp, Enabled: true}, nil
	}
	return nil, apperror.NotFound("nf")
}

type fakeMsgRepo struct{ latestBatchCalls *int }

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
func (r fakeMsgRepo) LatestByConversations(context.Context, []string) (map[string]*entity.Message, error) {
	if r.latestBatchCalls != nil {
		*r.latestBatchCalls++
	}
	return nil, nil
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

// fakeContactDir / fakeAgentDir resolve display cards, counting calls so a test
// can assert per-page batching (one call per page, not per row).
type fakeContactDir struct {
	cards map[string]shared.DisplayCard
	calls int
}

func (f *fakeContactDir) ContactCards(_ context.Context, ids []string) (map[string]shared.DisplayCard, error) {
	f.calls++
	out := map[string]shared.DisplayCard{}
	for _, id := range ids {
		if c, ok := f.cards[id]; ok {
			out[id] = c
		}
	}
	return out, nil
}

type fakeAgentDir struct {
	cards map[string]shared.DisplayCard
	calls int
}

func (f *fakeAgentDir) AgentCards(_ context.Context, ids []string) (map[string]shared.DisplayCard, error) {
	f.calls++
	out := map[string]shared.DisplayCard{}
	for _, id := range ids {
		if c, ok := f.cards[id]; ok {
			out[id] = c
		}
	}
	return out, nil
}

// ── harness ──────────────────────────────────────────────────────────────────

func buildRouter(cr *fakeConvRepo) http.Handler {
	return buildRouterFull(cr, nil, nil, nil)
}

func buildRouterFull(cr *fakeConvRepo, cd convcontracts.ContactDirectory, ad convcontracts.AgentDirectory, latestCalls *int) http.Handler {
	svc := convservice.New(cr, fakeMsgRepo{latestBatchCalls: latestCalls}, fakeEventRepo{}, fakeSectorRepo{}, fakeChannelConnRepo{}, shared.NoopPublisher{}, shared.SystemClock{})
	if cd != nil {
		svc.SetContactDirectory(cd)
	}
	if ad != nil {
		svc.SetAgentDirectory(ad)
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

// GET /v1/conversations embeds contact + agent display fields resolved in a
// CONSTANT number of batch calls (contact cards = 1, agent cards = 1), and reads
// the last-message preview from the denormalized snapshot on each row — NO
// aggregation over the messages collection. Contacts/agents without a record (or
// avatar) yield empty fields.
func TestConversations_List_EmbedsContactAndAgentInConstantQueries(t *testing.T) {
	now := time.Now()
	cr := &fakeConvRepo{items: []*entity.Conversation{
		{ID: "c1", TenantID: "t1", ContactID: "ct-av", AssignedTo: "u-agent", Status: entity.StatusAssigned, Priority: entity.PriorityNormal, UpdatedAt: now,
			LastMessage: &entity.LastMessageSnapshot{MessageID: "m1", Preview: "última mensagem", SenderType: entity.SenderCustomer, MessageType: entity.MessageText, CreatedAt: now}},
		{ID: "c2", TenantID: "t1", ContactID: "ct-none", Status: entity.StatusNew, Priority: entity.PriorityNormal, UpdatedAt: now},
		{ID: "c3", TenantID: "t1", ContactID: "ct-av", AssignedTo: "u-agent", Status: entity.StatusAssigned, Priority: entity.PriorityNormal, UpdatedAt: now},
	}}
	cd := &fakeContactDir{cards: map[string]shared.DisplayCard{
		"ct-av":   {Name: "Ana Cliente", AvatarURL: "http://api/v1/channel-media/tok-ct-av"},
		"ct-none": {Name: "Bob SemAvatar"}, // name resolves, no avatar
	}}
	ad := &fakeAgentDir{cards: map[string]shared.DisplayCard{
		"u-agent": {Name: "Diego Agente", AvatarURL: "http://api/v1/channel-media/tok-agent"},
	}}
	var latestCalls int

	rec := httpharness.Do(t, buildRouterFull(cr, cd, ad, &latestCalls), http.MethodGet, "/conversations", token(t, "t1", authz.ConversationRead), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var page struct {
		Data []struct {
			ID               string `json:"id"`
			ContactName      string `json:"contact_name"`
			ContactAvatarURL string `json:"contact_avatar_url"`
			AgentName        string `json:"agent_name"`
			AgentAvatarURL   string `json:"agent_avatar_url"`
			LastMessage      *struct {
				Preview     string `json:"preview"`
				MessageType string `json:"message_type"`
			} `json:"last_message"`
		} `json:"data"`
	}
	httpharness.DecodeJSON(t, rec, &page)
	got := map[string]struct {
		cn, ca, an, aa string
	}{}
	for _, d := range page.Data {
		got[d.ID] = struct{ cn, ca, an, aa string }{d.ContactName, d.ContactAvatarURL, d.AgentName, d.AgentAvatarURL}
	}
	if got["c1"].cn != "Ana Cliente" || got["c1"].ca == "" {
		t.Errorf("c1 must carry contact_name + contact_avatar_url, got %+v", got["c1"])
	}
	if got["c1"].an != "Diego Agente" || got["c1"].aa == "" {
		t.Errorf("c1 must carry agent_name + agent_avatar_url, got %+v", got["c1"])
	}
	if !strings.Contains(got["c1"].ca, "/v1/channel-media/") || !strings.Contains(got["c1"].aa, "/v1/channel-media/") {
		t.Errorf("avatar URLs must be JWT-less channel-media URLs, got %+v", got["c1"])
	}
	if got["c2"].cn != "Bob SemAvatar" || got["c2"].ca != "" {
		t.Errorf("c2 must carry name with empty avatar, got %+v", got["c2"])
	}
	if got["c2"].an != "" || got["c2"].aa != "" {
		t.Errorf("c2 (unassigned) must have empty agent fields, got %+v", got["c2"])
	}
	// The inbox preview is denormalized on the conversation row now, so the list
	// must NOT aggregate the messages collection at all (the gargalo we removed),
	// and the row carries the snapshot preview straight from the document.
	if latestCalls != 0 {
		t.Errorf("inbox must not aggregate last-messages anymore, got %d calls", latestCalls)
	}
	for _, d := range page.Data {
		if d.ID == "c1" {
			if d.LastMessage == nil || d.LastMessage.Preview != "última mensagem" {
				t.Errorf("c1 must carry the denormalized last_message preview, got %+v", d.LastMessage)
			}
		}
	}
	if cd.calls != 1 {
		t.Errorf("contact cards must resolve in one batch, got %d calls", cd.calls)
	}
	if ad.calls != 1 {
		t.Errorf("agent cards must resolve in one batch, got %d calls", ad.calls)
	}
}

// GET /v1/conversations/{id} (detail) resolves the same display fields as the list.
func TestConversations_Detail_ResolvesContactAndAgent(t *testing.T) {
	cr := &fakeConvRepo{items: []*entity.Conversation{
		{ID: "c1", TenantID: "t1", ContactID: "ct-av", AssignedTo: "u-agent", Status: entity.StatusAssigned, Priority: entity.PriorityNormal, UpdatedAt: time.Now()},
	}}
	cd := &fakeContactDir{cards: map[string]shared.DisplayCard{"ct-av": {Name: "Ana Cliente", AvatarURL: "http://api/v1/channel-media/tok-ct-av"}}}
	ad := &fakeAgentDir{cards: map[string]shared.DisplayCard{"u-agent": {Name: "Diego Agente", AvatarURL: "http://api/v1/channel-media/tok-agent"}}}

	rec := httpharness.Do(t, buildRouterFull(cr, cd, ad, nil), http.MethodGet, "/conversations/c1", token(t, "t1", authz.ConversationRead), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		ContactName      string `json:"contact_name"`
		ContactAvatarURL string `json:"contact_avatar_url"`
		AgentName        string `json:"agent_name"`
		AgentAvatarURL   string `json:"agent_avatar_url"`
	}
	httpharness.DecodeJSON(t, rec, &resp)
	if resp.ContactName != "Ana Cliente" || resp.ContactAvatarURL == "" || resp.AgentName != "Diego Agente" || resp.AgentAvatarURL == "" {
		t.Errorf("detail must resolve contact+agent fields, got %+v", resp)
	}
}

func TestConversations_Create_Happy(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(&fakeConvRepo{}), http.MethodPost, "/conversations",
		token(t, "t1", authz.ConversationRead), map[string]any{"contact_id": "ct1", "channel_id": "ch1"})
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

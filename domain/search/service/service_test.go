package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/search/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── fake index ───────────────────────────────────────────────────────────────

type fakeIndex struct {
	convs       []*conventity.Conversation
	contacts    []*contactentity.Contact
	messages    []*conventity.Message
	convByID    map[string]*conventity.Conversation
	visibleCtct map[string]bool // contactID -> has a visible conversation
	gotVis      contracts.Visibility
}

func (f *fakeIndex) SearchConversations(_ context.Context, _ contracts.ConversationFilter, vis contracts.Visibility, page shared.PageRequest) ([]*conventity.Conversation, error) {
	f.gotVis = vis
	// Echo up to limit+1 (the index would apply vis itself; here we just return).
	n := page.Limit + 1
	if n > len(f.convs) {
		n = len(f.convs)
	}
	return f.convs[:n], nil
}
func (f *fakeIndex) SearchContactsText(_ context.Context, _ string, _ shared.Cursor, scan int) ([]*contactentity.Contact, error) {
	if scan > len(f.contacts) {
		scan = len(f.contacts)
	}
	return f.contacts[:scan], nil
}
func (f *fakeIndex) SearchMessagesText(_ context.Context, _, _ string, _ shared.Cursor, scan int) ([]*conventity.Message, error) {
	if scan > len(f.messages) {
		scan = len(f.messages)
	}
	return f.messages[:scan], nil
}
func (f *fakeIndex) FindConversation(_ context.Context, id string) (*conventity.Conversation, error) {
	if c, ok := f.convByID[id]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (f *fakeIndex) HasVisibleConversationForContact(_ context.Context, contactID string, _ contracts.Visibility) (bool, error) {
	return f.visibleCtct[contactID], nil
}

// ── ctx helpers ──────────────────────────────────────────────────────────────

func allCtx() context.Context {
	ctx := shared.WithTenant(context.Background(), "t1")
	return authz.WithAuthContext(ctx, authz.NewAuthContext("t1", "u1", nil, nil, authz.ScopeAll))
}
func scopedCtx(sectors ...string) context.Context {
	ctx := shared.WithTenant(context.Background(), "t1")
	return authz.WithAuthContext(ctx, authz.NewAuthContext("t1", "bob", nil, sectors, authz.ScopeOwn))
}

func msg(id, convID string) *conventity.Message {
	return &conventity.Message{ID: id, TenantID: "t1", ConversationID: convID, Text: "hello", CreatedAt: time.Unix(1700000000, 0)}
}
func conv(id, sector, assigned string) *conventity.Conversation {
	return &conventity.Conversation{ID: id, TenantID: "t1", SectorID: sector, AssignedTo: assigned, UpdatedAt: time.Unix(1700000000, 0)}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestVisibility_BuiltFromAuth(t *testing.T) {
	idx := &fakeIndex{convs: []*conventity.Conversation{conv("a", "s1", "")}}
	svc := NewService(idx)
	if _, err := svc.SearchConversations(scopedCtx("s1", "s2"), contracts.ConversationFilter{}, shared.PageRequest{Limit: 10}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if idx.gotVis.All || idx.gotVis.UserID != "bob" || len(idx.gotVis.SectorIDs) != 2 {
		t.Errorf("visibility not built from auth: %+v", idx.gotVis)
	}

	idx.gotVis = contracts.Visibility{}
	_, _ = svc.SearchConversations(allCtx(), contracts.ConversationFilter{}, shared.PageRequest{Limit: 10})
	if !idx.gotVis.All {
		t.Errorf("all-scope actor should yield All visibility")
	}
}

func TestSearch_RequiresAuth(t *testing.T) {
	svc := NewService(&fakeIndex{})
	// Tenant but no auth context.
	ctx := shared.WithTenant(context.Background(), "t1")
	if _, err := svc.SearchContacts(ctx, "x", shared.PageRequest{Limit: 10}); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Errorf("expected unauthorized without auth context, got %v", err)
	}
}

func TestSearchMessages_ScopeFiltersInvisibleConversations(t *testing.T) {
	// Conversation A is in the actor's sector (visible); B is not.
	a := conv("A", "s1", "")
	b := conv("B", "s2", "")
	idx := &fakeIndex{
		messages: []*conventity.Message{msg("m1", "A"), msg("m2", "B"), msg("m3", "A")},
		convByID: map[string]*conventity.Conversation{"A": a, "B": b},
	}
	svc := NewService(idx)

	res, err := svc.SearchMessages(scopedCtx("s1"), contracts.MessageFilter{Query: "hello"}, shared.PageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("expected 2 visible messages (conv A), got %d", len(res.Items))
	}
	for _, m := range res.Items {
		if m.ConversationID != "A" {
			t.Errorf("leaked a message from an invisible conversation: %+v", m)
		}
	}

	// All-scope sees every match.
	resAll, _ := svc.SearchMessages(allCtx(), contracts.MessageFilter{Query: "hello"}, shared.PageRequest{Limit: 10})
	if len(resAll.Items) != 3 {
		t.Errorf("all-scope should see all 3 messages, got %d", len(resAll.Items))
	}
}

func TestSearchContacts_ScopeFiltersToVisibleContacts(t *testing.T) {
	idx := &fakeIndex{
		contacts: []*contactentity.Contact{
			{ID: "c1", TenantID: "t1", Name: "Alice"},
			{ID: "c2", TenantID: "t1", Name: "Bob"},
			{ID: "c3", TenantID: "t1", Name: "Carol"},
		},
		visibleCtct: map[string]bool{"c1": true, "c3": true}, // c2 not visible to this actor
	}
	svc := NewService(idx)

	res, err := svc.SearchContacts(scopedCtx("s1"), "a", shared.PageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("expected 2 visible contacts, got %d", len(res.Items))
	}
	for _, c := range res.Items {
		if c.ID == "c2" {
			t.Errorf("leaked an out-of-scope contact")
		}
	}

	resAll, _ := svc.SearchContacts(allCtx(), "a", shared.PageRequest{Limit: 10})
	if len(resAll.Items) != 3 {
		t.Errorf("all-scope should see all 3 contacts, got %d", len(resAll.Items))
	}
}

func TestSearchConversations_CursorPagination(t *testing.T) {
	// 3 conversations, limit 2 → page 1 returns 2 with a next cursor.
	convs := []*conventity.Conversation{
		conv("c1", "s1", ""), conv("c2", "s1", ""), conv("c3", "s1", ""),
	}
	svc := NewService(&fakeIndex{convs: convs})
	res, err := svc.SearchConversations(allCtx(), contracts.ConversationFilter{}, shared.PageRequest{Limit: 2})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Items) != 2 {
		t.Errorf("expected 2 items on the first page, got %d", len(res.Items))
	}
	if res.NextCursor == "" {
		t.Errorf("expected a next cursor when more results exist")
	}
}

func TestFilteredGeneric_ProgressesPastInvisible(t *testing.T) {
	// 5 raw items, only odd indices visible, limit 2. The page should keep the
	// visible ones and set a cursor that makes progress.
	raw := []int{0, 1, 2, 3, 4}
	visible := func(n int) bool { return n%2 == 1 }
	cursorOf := func(n int) shared.Cursor { return shared.Cursor{CreatedAt: int64(n), ID: "x"} }
	kept, next, err := filteredGeneric[int](shared.PageRequest{Limit: 2}, false,
		func(scan int) ([]int, error) {
			if scan > len(raw) {
				scan = len(raw)
			}
			return raw[:scan], nil
		}, visible, cursorOf)
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if len(kept) != 2 || kept[0] != 1 || kept[1] != 3 {
		t.Errorf("expected [1,3], got %v", kept)
	}
	if next == "" {
		t.Errorf("expected a next cursor (more candidates remain)")
	}
}

func TestVisibility_CanSee(t *testing.T) {
	all := contracts.Visibility{All: true}
	if !all.CanSee(conv("x", "s9", "")) {
		t.Errorf("all-scope sees everything")
	}
	scoped := contracts.Visibility{SectorIDs: []string{"s1"}, UserID: "bob"}
	if !scoped.CanSee(conv("x", "s1", "")) {
		t.Errorf("should see own-sector conversation")
	}
	if !scoped.CanSee(conv("x", "s9", "bob")) {
		t.Errorf("should see assigned-to-me conversation")
	}
	if scoped.CanSee(conv("x", "s9", "alice")) {
		t.Errorf("must not see out-of-scope conversation")
	}
}

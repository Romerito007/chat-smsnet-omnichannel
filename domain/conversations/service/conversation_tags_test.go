package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

type fakeTagCatalog struct{ err error }

func (f fakeTagCatalog) ValidateTags(context.Context, []string) error { return f.err }
func (f fakeTagCatalog) ResolveTags(_ context.Context, refs []string, _ bool) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return refs, nil // passthrough: the test treats its inputs as canonical ids
}

type fakeCloseReasonPolicy struct {
	requiresNote bool
	err          error
}

func (f fakeCloseReasonPolicy) RequiresNote(context.Context, string) (bool, error) {
	return f.requiresNote, f.err
}

// openConv creates a conversation and returns its id.
func openConv(t *testing.T, svc *Service) string {
	t.Helper()
	conv, err := svc.Create(adminCtx(), contracts.CreateConversation{ContactID: "c1", Channel: "whatsapp", SectorID: "s1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return conv.ID
}

func TestClose_RequiresNote_Enforced(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	svc.SetCloseReasonPolicy(fakeCloseReasonPolicy{requiresNote: true})
	id := openConv(t, svc)

	// Closing with a note-requiring reason but no note is rejected.
	_, err := svc.Close(adminCtx(), id, contracts.CloseConversation{CloseReasonID: "r1"})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation error without note, got %v", err)
	}

	// With a note it succeeds.
	conv, err := svc.Close(adminCtx(), id, contracts.CloseConversation{CloseReasonID: "r1", Note: "resolved by phone"})
	if err != nil {
		t.Fatalf("close with note: %v", err)
	}
	if conv.Status != entity.StatusClosed {
		t.Errorf("status = %q, want closed", conv.Status)
	}
}

func TestClose_NoteNotRequired(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	svc.SetCloseReasonPolicy(fakeCloseReasonPolicy{requiresNote: false})
	id := openConv(t, svc)
	if _, err := svc.Close(adminCtx(), id, contracts.CloseConversation{CloseReasonID: "r1"}); err != nil {
		t.Fatalf("close without note should succeed when reason does not require one: %v", err)
	}
}

func TestApplyTags_AddsValidatesAndEmitsRealtime(t *testing.T) {
	svc, cr, _, er, pub := newService(map[string]string{"s1": "t1"})
	svc.SetTagCatalog(fakeTagCatalog{})
	id := openConv(t, svc)

	conv, err := svc.ApplyTags(adminCtx(), id, []string{"urgent", "urgent"}, nil)
	if err != nil {
		t.Fatalf("apply tags: %v", err)
	}
	if len(conv.Tags) != 1 || conv.Tags[0] != "urgent" {
		t.Errorf("expected deduped [urgent], got %v", conv.Tags)
	}
	// persisted
	if got := cr.items[id]; got == nil || len(got.Tags) != 1 {
		t.Errorf("tags not persisted: %+v", got)
	}
	// timeline event
	tagged := false
	for _, e := range er.items {
		if e.Type == entity.EventConversationTagged {
			tagged = true
		}
	}
	if !tagged {
		t.Errorf("expected a conversation.tagged timeline event")
	}
	// realtime
	realtime := false
	for _, e := range pub.events {
		if e.event == contracts.RealtimeConversationTagged {
			realtime = true
		}
	}
	if !realtime {
		t.Errorf("expected a conversation.tagged realtime event, got %+v", pub.events)
	}
}

func TestApplyTags_RemoveAndReject(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	svc.SetTagCatalog(fakeTagCatalog{})
	id := openConv(t, svc)

	if _, err := svc.ApplyTags(adminCtx(), id, []string{"a", "b"}, nil); err != nil {
		t.Fatalf("add: %v", err)
	}
	conv, err := svc.ApplyTags(adminCtx(), id, nil, []string{"a"})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(conv.Tags) != 1 || conv.Tags[0] != "b" {
		t.Errorf("expected [b] after removing a, got %v", conv.Tags)
	}

	// An invalid tag from the catalog is rejected.
	svc.SetTagCatalog(fakeTagCatalog{err: apperror.Validation("unknown tag")})
	if _, err := svc.ApplyTags(adminCtx(), id, []string{"ghost"}, nil); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation error for unknown tag, got %v", err)
	}
}

// mappingTagCatalog resolves names to canonical ids (and ids pass through), so
// the stored array is always id-only — mirroring the real TagService.
type mappingTagCatalog struct{ nameToID map[string]string }

func (mappingTagCatalog) ValidateTags(context.Context, []string) error { return nil }
func (c mappingTagCatalog) ResolveTags(_ context.Context, refs []string, _ bool) ([]string, error) {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if id, ok := c.nameToID[r]; ok {
			out = append(out, id)
			continue
		}
		out = append(out, r) // already an id
	}
	return out, nil
}

// Applying a tag by NAME stores its ID; removing by that same ID empties the
// array — the mixed name/id bug that broke front removal.
func TestApplyTags_NameResolvesToIDAndRemovesByID(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	svc.SetTagCatalog(mappingTagCatalog{nameToID: map[string]string{"vip": "id-vip"}})
	id := openConv(t, svc)

	conv, err := svc.ApplyTags(adminCtx(), id, []string{"vip"}, nil)
	if err != nil {
		t.Fatalf("add by name: %v", err)
	}
	if len(conv.Tags) != 1 || conv.Tags[0] != "id-vip" {
		t.Fatalf("expected [id-vip] stored, got %v", conv.Tags)
	}

	conv, err = svc.ApplyTags(adminCtx(), id, nil, []string{"id-vip"})
	if err != nil {
		t.Fatalf("remove by id: %v", err)
	}
	if len(conv.Tags) != 0 {
		t.Errorf("expected empty tags after removal, got %v", conv.Tags)
	}
}

func TestApplyTags_RequiresAtLeastOne(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)
	if _, err := svc.ApplyTags(adminCtx(), id, nil, nil); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation error when no tags provided, got %v", err)
	}
}

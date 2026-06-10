package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeRepo struct {
	byIdentity map[string]*entity.Contact
	byID       map[string]*entity.Contact
	creates    int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byIdentity: map[string]*entity.Contact{}, byID: map[string]*entity.Contact{}}
}
func key(ch, ext string) string { return ch + "|" + ext }

func (r *fakeRepo) Create(_ context.Context, c *entity.Contact) error {
	r.creates++
	cp := *c
	r.byID[c.ID] = &cp
	for _, id := range c.Identities {
		r.byIdentity[key(id.Channel, id.ExternalID)] = &cp
	}
	return nil
}
func (r *fakeRepo) Update(_ context.Context, c *entity.Contact) error {
	cp := *c
	r.byID[c.ID] = &cp
	for _, id := range c.Identities {
		r.byIdentity[key(id.Channel, id.ExternalID)] = &cp
	}
	return nil
}
func (r *fakeRepo) FindByID(_ context.Context, id string) (*entity.Contact, error) {
	if c, ok := r.byID[id]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRepo) FindByChannelIdentity(_ context.Context, ch, ext string) (*entity.Contact, error) {
	if c, ok := r.byIdentity[key(ch, ext)]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRepo) List(context.Context, shared.PageRequest) ([]*entity.Contact, error) {
	return nil, nil
}

func tenantCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }

func TestUpsert_CreatesThenReuses(t *testing.T) {
	repo := newFakeRepo()
	svc := New(repo, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	ctx := tenantCtx()

	c1, err := svc.UpsertFromInbound(ctx, contracts.UpsertFromInbound{
		Channel: "whatsapp", ExternalID: "5511999", Name: "Jane", Phone: "+55 (11) 99999-0000",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if c1.Phone != "+5511999990000" {
		t.Errorf("phone not normalized to digits: %q", c1.Phone)
	}

	// Same identity → reuse, no new contact.
	c2, err := svc.UpsertFromInbound(ctx, contracts.UpsertFromInbound{
		Channel: "whatsapp", ExternalID: "5511999", Name: "Jane Doe",
	})
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	if c2.ID != c1.ID {
		t.Errorf("expected same contact, got different id")
	}
	if repo.creates != 1 {
		t.Errorf("expected exactly one create, got %d", repo.creates)
	}
	if c2.Name != "Jane Doe" {
		t.Errorf("name should update to new non-empty value, got %q", c2.Name)
	}
}

func TestUpsert_RequiresIdentity(t *testing.T) {
	svc := New(newFakeRepo(), fixedClock{t: time.Unix(1700000000, 0).UTC()})
	if _, err := svc.UpsertFromInbound(tenantCtx(), contracts.UpsertFromInbound{Channel: "whatsapp"}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error without external id, got %v", err)
	}
}

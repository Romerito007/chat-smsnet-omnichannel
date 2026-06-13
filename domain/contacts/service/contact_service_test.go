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
func (r *fakeRepo) FindByDocument(_ context.Context, document string) (*entity.Contact, error) {
	for _, c := range r.byID {
		if c.Document != "" && c.Document == document {
			return c, nil
		}
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRepo) FindByPhone(_ context.Context, phone string) (*entity.Contact, error) {
	for _, c := range r.byID {
		if c.Phone == phone {
			return c, nil
		}
		for _, p := range c.Phones {
			if p == phone {
				return c, nil
			}
		}
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRepo) List(context.Context, contracts.ListFilter, shared.PageRequest) ([]*entity.Contact, error) {
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

func newSvc() *Service {
	return New(newFakeRepo(), fixedClock{t: time.Unix(1700000000, 0).UTC()})
}

func TestCreate_NormalizesAndPersists(t *testing.T) {
	svc := newSvc()
	c, err := svc.Create(tenantCtx(), contracts.CreateContact{
		Name: "  Jane  ", Phones: []string{"+55 (11) 99999-0000", "11 98888 1111"},
		Document: "12345678900", Email: "  JANE@x.com ", Tags: []string{"vip", " "},
		ExternalIDs: []contracts.ExternalIdentity{{Channel: "whatsapp", ExternalID: "5511"}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.Name != "Jane" || c.Email != "jane@x.com" {
		t.Errorf("trim/lowercase failed: %+v", c)
	}
	if len(c.Phones) != 2 || c.Phones[0] != "+5511999990000" || c.Phone != "+5511999990000" {
		t.Errorf("phones not normalized/primary set: %+v", c.Phones)
	}
	if len(c.Tags) != 1 || c.Tags[0] != "vip" {
		t.Errorf("tags not cleaned: %+v", c.Tags)
	}
	if len(c.Identities) != 1 {
		t.Errorf("external id not mapped: %+v", c.Identities)
	}
}

func TestCreate_RequiresName(t *testing.T) {
	if _, err := newSvc().Create(tenantCtx(), contracts.CreateContact{Name: "  "}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error without name, got %v", err)
	}
}

func TestCreate_DedupByDocumentAndPhone(t *testing.T) {
	svc := newSvc()
	ctx := tenantCtx()
	if _, err := svc.Create(ctx, contracts.CreateContact{Name: "A", Document: "999", Phones: []string{"5511"}}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	// Same document → conflict.
	if _, err := svc.Create(ctx, contracts.CreateContact{Name: "B", Document: "999"}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("duplicate document must conflict, got %v", err)
	}
	// Same phone → conflict.
	if _, err := svc.Create(ctx, contracts.CreateContact{Name: "C", Phones: []string{"55 11"}}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("duplicate phone must conflict, got %v", err)
	}
}

func TestUpdate_PartialAndDedupExcludesSelf(t *testing.T) {
	svc := newSvc()
	ctx := tenantCtx()
	c, _ := svc.Create(ctx, contracts.CreateContact{Name: "A", Document: "111", Phones: []string{"5511"}, Notes: "old"})

	// Re-saving the same contact's own phone/document must NOT conflict with itself.
	name := "A2"
	got, err := svc.Update(ctx, c.ID, contracts.UpdateContact{Name: &name, Phones: &[]string{"5511"}})
	if err != nil {
		t.Fatalf("update self: %v", err)
	}
	if got.Name != "A2" || got.Notes != "old" {
		t.Errorf("partial update wrong: %+v", got)
	}

	// A second contact, then updating it to the first's document → conflict.
	c2, _ := svc.Create(ctx, contracts.CreateContact{Name: "B", Document: "222"})
	doc := "111"
	if _, err := svc.Update(ctx, c2.ID, contracts.UpdateContact{Document: &doc}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("update to an existing document must conflict, got %v", err)
	}
}

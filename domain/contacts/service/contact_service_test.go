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
func (r *fakeRepo) FindByIDs(_ context.Context, ids []string) ([]*entity.Contact, error) {
	var out []*entity.Contact
	for _, id := range ids {
		if c, ok := r.byID[id]; ok {
			out = append(out, c)
		}
	}
	return out, nil
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

// countingAvatarResolver records how many times it is called, to prove batching.
type countingAvatarResolver struct{ calls int }

func (c *countingAvatarResolver) SignedAvatarURLs(_ context.Context, ids []string) (map[string]string, error) {
	c.calls++
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		out[id] = "url-" + id
	}
	return out, nil
}

// A page of N avatar ids resolves in ONE signing call (no per-item round-trip).
func TestAvatarURLs_BatchSingleCall(t *testing.T) {
	svc := newSvc()
	res := &countingAvatarResolver{}
	svc.SetAvatarURLResolver(res)

	urls, err := svc.AvatarURLs(tenantCtx(), []string{"a1", "a2", "a3"})
	if err != nil {
		t.Fatalf("avatar urls: %v", err)
	}
	if res.calls != 1 {
		t.Errorf("expected a single batch call, got %d", res.calls)
	}
	if len(urls) != 3 {
		t.Errorf("expected 3 resolved urls, got %d", len(urls))
	}
}

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
		Document: "111.444.777-35", Email: "  JANE@x.com ", Tags: []string{"vip", " "},
		ExternalIDs: []contracts.ExternalIdentity{{Channel: "whatsapp", ExternalID: "5511"}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.Name != "Jane" || c.Email != "jane@x.com" {
		t.Errorf("trim/lowercase failed: %+v", c)
	}
	if c.Document != "11144477735" {
		t.Errorf("document not stored digits-only: %q", c.Document)
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

func TestCreate_AcceptsMessengerIdentity(t *testing.T) {
	svc := newSvc()
	c, err := svc.Create(tenantCtx(), contracts.CreateContact{
		Name:        "Meg",
		ExternalIDs: []contracts.ExternalIdentity{{Channel: "messenger", ExternalID: "fb-1234"}},
	})
	if err != nil {
		t.Fatalf("create with messenger identity: %v", err)
	}
	if len(c.Identities) != 1 || c.Identities[0].Channel != "messenger" {
		t.Errorf("messenger identity not persisted: %+v", c.Identities)
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
	if _, err := svc.Create(ctx, contracts.CreateContact{Name: "A", Document: "111.444.777-35", Phones: []string{"+5511999990000"}}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	// Same document → conflict.
	if _, err := svc.Create(ctx, contracts.CreateContact{Name: "B", Document: "11144477735"}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("duplicate document must conflict, got %v", err)
	}
	// Same phone (different representation, same E.164) → conflict.
	if _, err := svc.Create(ctx, contracts.CreateContact{Name: "C", Phones: []string{"11 99999-0000"}}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("duplicate phone must conflict, got %v", err)
	}
}

func TestCreate_RejectsInvalidPhoneDocumentEmailChannel(t *testing.T) {
	svc := newSvc()
	ctx := tenantCtx()
	_, err := svc.Create(ctx, contracts.CreateContact{
		Name:        "Bad",
		Phones:      []string{"abc123"},
		Document:    "12345678900", // invalid CPF check digits
		Email:       "not-an-email",
		ExternalIDs: []contracts.ExternalIdentity{{Channel: "carrier-pigeon", ExternalID: "x"}},
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation_error, got %v", err)
	}
	details := apperror.From(err).Details
	for _, want := range []string{"phones[0]", "document", "email", "external_ids[0].channel"} {
		if _, ok := details[want]; !ok {
			t.Errorf("missing detail for %q in %v", want, details)
		}
	}
}

func TestCreate_RejectsDuplicateChannelIdentity(t *testing.T) {
	svc := newSvc()
	ctx := tenantCtx()
	if _, err := svc.Create(ctx, contracts.CreateContact{
		Name: "A", ExternalIDs: []contracts.ExternalIdentity{{Channel: "whatsapp", ExternalID: "5544999"}},
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	// Another contact claiming the same (channel, external_id) → conflict.
	if _, err := svc.Create(ctx, contracts.CreateContact{
		Name: "B", ExternalIDs: []contracts.ExternalIdentity{{Channel: "whatsapp", ExternalID: "5544999"}},
	}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("duplicate channel identity must conflict, got %v", err)
	}
}

func TestUpdate_PartialAndDedupExcludesSelf(t *testing.T) {
	svc := newSvc()
	ctx := tenantCtx()
	c, _ := svc.Create(ctx, contracts.CreateContact{Name: "A", Document: "111.444.777-35", Phones: []string{"+5511999990000"}, Notes: "old"})

	// Re-saving the same contact's own phone/document must NOT conflict with itself.
	name := "A2"
	got, err := svc.Update(ctx, c.ID, contracts.UpdateContact{Name: &name, Phones: &[]string{"11 99999-0000"}})
	if err != nil {
		t.Fatalf("update self: %v", err)
	}
	if got.Name != "A2" || got.Notes != "old" {
		t.Errorf("partial update wrong: %+v", got)
	}

	// A second contact, then updating it to the first's document → conflict.
	c2, _ := svc.Create(ctx, contracts.CreateContact{Name: "B", Document: "529.982.247-25"})
	doc := "11144477735"
	if _, err := svc.Update(ctx, c2.ID, contracts.UpdateContact{Document: &doc}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("update to an existing document must conflict, got %v", err)
	}
}

func TestAddChannelIdentity(t *testing.T) {
	svc := newSvc()
	ctx := tenantCtx()
	// Seed a contact with no whatsapp identity.
	base, err := svc.Create(ctx, contracts.CreateContact{Name: "Pedro", Phones: []string{"+5544999088478"}})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	jid := "554499088478@s.whatsapp.net"

	// 1) New identity → applied=true and stored.
	applied, err := svc.AddChannelIdentity(ctx, base.ID, "whatsapp", jid)
	if err != nil || !applied {
		t.Fatalf("first add: applied=%v err=%v", applied, err)
	}
	got, _ := svc.Get(ctx, base.ID)
	if !got.HasIdentity("whatsapp", jid) {
		t.Errorf("identity not persisted: %+v", got.Identities)
	}

	// 2) Idempotent → applied=false, no duplicate.
	applied, err = svc.AddChannelIdentity(ctx, base.ID, "whatsapp", jid)
	if err != nil || applied {
		t.Errorf("repeat must be a no-op: applied=%v err=%v", applied, err)
	}
	if got, _ := svc.Get(ctx, base.ID); len(got.Identities) != 1 {
		t.Errorf("identity must not duplicate, got %d", len(got.Identities))
	}

	// 3) JID owned by ANOTHER contact → not stolen (applied=false, no error).
	other, _ := svc.Create(ctx, contracts.CreateContact{Name: "Maria",
		ExternalIDs: []contracts.ExternalIdentity{{Channel: "whatsapp", ExternalID: "999@s.whatsapp.net"}}})
	applied, err = svc.AddChannelIdentity(ctx, base.ID, "whatsapp", "999@s.whatsapp.net")
	if err != nil {
		t.Fatalf("in-use must not error: %v", err)
	}
	if applied {
		t.Error("must NOT steal an identity owned by another contact")
	}
	if got, _ := svc.Get(ctx, base.ID); got.HasIdentity("whatsapp", "999@s.whatsapp.net") {
		t.Error("the other contact's identity must not be added to this contact")
	}
	if om, _ := svc.Get(ctx, other.ID); !om.HasIdentity("whatsapp", "999@s.whatsapp.net") {
		t.Error("the owner must keep its identity")
	}

	// 4) Validation: missing external_id / unsupported channel.
	if _, err := svc.AddChannelIdentity(ctx, base.ID, "whatsapp", "  "); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("empty external_id must be validation, got %v", err)
	}
	if _, err := svc.AddChannelIdentity(ctx, "ghost", "whatsapp", jid); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("unknown contact must be not_found, got %v", err)
	}
}

// UpsertGroupContact creates ONE group contact keyed by the JID (no phone), seeds
// name/notes from the registry, and is idempotent: a second call dedups by identity
// and only refreshes the name when it changed — never creating a second contact.
func TestUpsertGroupContact_DedupByJIDNoPhone(t *testing.T) {
	svc := newSvc()
	ctx := tenantCtx()
	const jid = "120363000000000000@g.us"

	g1, err := svc.UpsertGroupContact(ctx, "whatsapp", jid, "Cliente A", "Suporte")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if !g1.IsGroup() {
		t.Errorf("kind must be group, got %q", g1.Kind)
	}
	if g1.Phone != "" || len(g1.Phones) != 0 {
		t.Errorf("a group contact must have no phone, got %q / %v", g1.Phone, g1.Phones)
	}
	if !g1.HasIdentity("whatsapp", jid) {
		t.Errorf("identity must be {whatsapp, %s}, got %+v", jid, g1.Identities)
	}
	if g1.Notes != "Suporte" {
		t.Errorf("description should seed notes, got %q", g1.Notes)
	}

	// Re-sync with a renamed group → same contact id, refreshed name.
	g2, err := svc.UpsertGroupContact(ctx, "whatsapp", jid, "Cliente A Renomeado", "ignored on update")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if g2.ID != g1.ID {
		t.Errorf("re-upsert must dedup by JID (same id), got %s vs %s", g2.ID, g1.ID)
	}
	if g2.Name != "Cliente A Renomeado" {
		t.Errorf("name should refresh from the registry, got %q", g2.Name)
	}
}

// The registry description seeds Notes only when empty: an agent's edited note is
// never clobbered by a re-sync, but an empty note gets the description back.
func TestUpsertGroupContact_DescriptionSeedsNotesWithoutClobber(t *testing.T) {
	svc := newSvc()
	ctx := tenantCtx()
	const jid = "120363000000000099@g.us"

	// Created with no registry description → Notes empty.
	g1, err := svc.UpsertGroupContact(ctx, "whatsapp", jid, "Grupo", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if g1.Notes != "" {
		t.Errorf("no description → notes should be empty, got %q", g1.Notes)
	}

	// A later sync that now has a description seeds the empty Notes.
	g2, err := svc.UpsertGroupContact(ctx, "whatsapp", jid, "Grupo", "Cliente Acme — financeiro")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if g2.Notes != "Cliente Acme — financeiro" {
		t.Errorf("empty notes must be seeded from the registry description, got %q", g2.Notes)
	}

	// Agent edits the note; a subsequent sync must NOT overwrite it.
	edited := "Falar só com o Sr. João"
	if _, err := svc.Update(ctx, g2.ID, contracts.UpdateContact{Notes: &edited}); err != nil {
		t.Fatalf("agent edit: %v", err)
	}
	g3, err := svc.UpsertGroupContact(ctx, "whatsapp", jid, "Grupo", "Cliente Acme — financeiro")
	if err != nil {
		t.Fatalf("resync: %v", err)
	}
	if g3.Notes != "Falar só com o Sr. João" {
		t.Errorf("a re-sync must not clobber the agent's note, got %q", g3.Notes)
	}
}

// Self-heal: a group contact that somehow lacks its JID identity gets it back on the
// next sync — so the outbound webhook can always route to the group.
func TestUpsertGroupContact_HealsMissingIdentity(t *testing.T) {
	repo := newFakeRepo()
	svc := New(repo, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	ctx := tenantCtx()
	const jid = "120363000000000077@g.us"

	// Seed a broken group contact: right kind, but NO identity (pre-fix shape).
	broken := &entity.Contact{ID: "broken1", TenantID: "t1", Kind: entity.KindGroup, Name: "Grupo"}
	_ = repo.Create(ctx, broken)
	// Make it findable by JID anyway (simulating a record located by another means).
	repo.byIdentity[key("whatsapp", jid)] = broken

	healed, err := svc.UpsertGroupContact(ctx, "whatsapp", jid, "Grupo", "")
	if err != nil {
		t.Fatalf("heal: %v", err)
	}
	if !healed.HasIdentity("whatsapp", jid) {
		t.Errorf("the JID identity must be healed onto the group contact, got %+v", healed.Identities)
	}
}

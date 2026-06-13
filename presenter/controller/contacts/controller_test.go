package contacts_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	domainauth "github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	contactcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/contracts"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	contactservice "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/contacts"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/httpharness"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// memRepo is an in-memory ContactRepository for the controller stack test.
type memRepo struct {
	byID map[string]*contactentity.Contact
}

func newMemRepo() *memRepo { return &memRepo{byID: map[string]*contactentity.Contact{}} }

func (r *memRepo) Create(_ context.Context, c *contactentity.Contact) error {
	cp := *c
	r.byID[c.ID] = &cp
	return nil
}
func (r *memRepo) Update(_ context.Context, c *contactentity.Contact) error {
	cp := *c
	r.byID[c.ID] = &cp
	return nil
}
func (r *memRepo) FindByID(_ context.Context, id string) (*contactentity.Contact, error) {
	if c, ok := r.byID[id]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *memRepo) FindByChannelIdentity(context.Context, string, string) (*contactentity.Contact, error) {
	return nil, apperror.NotFound("nf")
}
func (r *memRepo) FindByDocument(_ context.Context, document string) (*contactentity.Contact, error) {
	for _, c := range r.byID {
		if c.Document != "" && c.Document == document {
			return c, nil
		}
	}
	return nil, apperror.NotFound("nf")
}
func (r *memRepo) FindByPhone(_ context.Context, phone string) (*contactentity.Contact, error) {
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
func (r *memRepo) List(_ context.Context, f contactcontracts.ListFilter, _ shared.PageRequest) ([]*contactentity.Contact, error) {
	out := make([]*contactentity.Contact, 0, len(r.byID))
	for _, c := range r.byID {
		if f.TagID != "" && !hasTag(c.Tags, f.TagID) {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func hasTag(tags []string, id string) bool {
	for _, t := range tags {
		if t == id {
			return true
		}
	}
	return false
}

func build(t *testing.T) (http.Handler, domainauth.TokenManager) {
	t.Helper()
	tm := httpharness.Tokens()
	svc := contactservice.New(newMemRepo(), nil)
	ctl := contacts.NewController(svc)

	r := chi.NewRouter()
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(tm))
		p.With(middleware.RequirePermission(authz.ContactRead)).Get("/contacts", ctl.List)
		p.With(middleware.RequirePermission(authz.ContactRead)).Get("/contacts/{id}", ctl.Get)
		p.With(middleware.RequirePermission(authz.ContactWrite)).Post("/contacts", ctl.Create)
		p.With(middleware.RequirePermission(authz.ContactWrite)).Patch("/contacts/{id}", ctl.Update)
	})
	return r, tm
}

func TestContacts_CreateGetUpdate_RoundTrip(t *testing.T) {
	r, tm := build(t)
	write := httpharness.Token(t, tm, "t1", "u1", authz.ContactWrite, authz.ContactRead)

	// Create.
	rec := httpharness.Do(t, r, http.MethodPost, "/contacts", write,
		map[string]any{"name": "Jane", "phones": []string{"+55 11 99999-0000"}, "document": "111.444.777-35", "email": "Jane@x.com"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d (%s)", rec.Code, rec.Body.String())
	}
	var created struct {
		ID     string   `json:"id"`
		Phones []string `json:"phones"`
		Email  string   `json:"email"`
	}
	httpharness.DecodeJSON(t, rec, &created)
	if created.ID == "" || len(created.Phones) != 1 || created.Phones[0] != "+5511999990000" || created.Email != "jane@x.com" {
		t.Fatalf("unexpected created contact: %+v", created)
	}

	// Get.
	if g := httpharness.Do(t, r, http.MethodGet, "/contacts/"+created.ID, write, nil); g.Code != http.StatusOK {
		t.Fatalf("get status = %d", g.Code)
	}

	// Update (partial).
	up := httpharness.Do(t, r, http.MethodPatch, "/contacts/"+created.ID, write, map[string]any{"notes": "vip"})
	if up.Code != http.StatusOK {
		t.Fatalf("update status = %d (%s)", up.Code, up.Body.String())
	}
}

func TestContacts_ListFilterByTag(t *testing.T) {
	r, tm := build(t)
	write := httpharness.Token(t, tm, "t1", "u1", authz.ContactWrite, authz.ContactRead)

	// Two contacts, only one carries the "vip" tag.
	_ = httpharness.Do(t, r, http.MethodPost, "/contacts", write,
		map[string]any{"name": "Ana", "tags": []string{"vip"}})
	_ = httpharness.Do(t, r, http.MethodPost, "/contacts", write,
		map[string]any{"name": "Bob"})

	rec := httpharness.Do(t, r, http.MethodGet, "/contacts?tag_id=vip", write, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d (%s)", rec.Code, rec.Body.String())
	}
	var page struct {
		Data []struct {
			Name string   `json:"name"`
			Tags []string `json:"tags"`
		} `json:"data"`
	}
	httpharness.DecodeJSON(t, rec, &page)
	if len(page.Data) != 1 || page.Data[0].Name != "Ana" {
		t.Fatalf("expected only the vip-tagged contact, got %+v", page.Data)
	}
}

func TestContacts_DuplicateDocument_409(t *testing.T) {
	r, tm := build(t)
	write := httpharness.Token(t, tm, "t1", "u1", authz.ContactWrite)
	_ = httpharness.Do(t, r, http.MethodPost, "/contacts", write, map[string]any{"name": "A", "document": "111.444.777-35"})
	rec := httpharness.Do(t, r, http.MethodPost, "/contacts", write, map[string]any{"name": "B", "document": "11144477735"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate document status = %d, want 409", rec.Code)
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeConflict {
		t.Errorf("code = %q, want conflict", code)
	}
}

func TestContacts_WriteRequiresContactWrite_403(t *testing.T) {
	r, tm := build(t)
	readOnly := httpharness.Token(t, tm, "t1", "u1", authz.ContactRead) // no contact.write
	rec := httpharness.Do(t, r, http.MethodPost, "/contacts", readOnly, map[string]any{"name": "Jane"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("create without contact.write status = %d, want 403", rec.Code)
	}
}

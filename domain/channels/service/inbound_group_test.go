package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	contactservice "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/service"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	groupentity "github.com/romerito007/chat-smsnet-omnichannel/domain/groups/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// fakeGroupGate stands in for the groups registry (Domain 1) in the attend gate.
type fakeGroupGate struct {
	byJID map[string]*groupentity.Group
}

func (g *fakeGroupGate) FindByJID(_ context.Context, jid string) (*groupentity.Group, error) {
	if grp, ok := g.byJID[jid]; ok {
		return grp, nil
	}
	return nil, apperror.NotFound("nf")
}

// newGroupFixture builds an inbound fixture with the group gate wired and exposes
// the contact repo so tests can assert exactly one group contact is created.
func newGroupFixture(gate GroupGate) (inboundFixture, *fakeContactRepo) {
	cr := newFakeConvRepo()
	mr := newFakeMsgRepo()
	er := &fakeEventRepo{}
	rules := &fakeRuleSink{}
	pub := &fakePublisher{}
	contactRepo := newFakeContactRepo()
	contacts := contactservice.New(contactRepo, clockNow())
	svc := NewInboundService(contacts, cr, mr, er, &fakeProtocolCounter{}, newFakeInbound(),
		shared.NoopLocker{}, pub, clockNow())
	svc.SetRuleSink(rules)
	if gate != nil {
		svc.SetGroupGate(gate)
	}
	return inboundFixture{svc: svc, convs: cr, msgs: mr, events: er, rules: rules, pub: pub}, contactRepo
}

func groupMsg(ext, groupJID, senderJID, senderName string) chcontracts.InboundMessage {
	return chcontracts.InboundMessage{
		ExternalMessageID: ext, Channel: "whatsapp", Text: "oi pessoal",
		GroupJID: groupJID, SenderJID: senderJID, SenderName: senderName,
	}
}

func attendedGate(jid, name string) *fakeGroupGate {
	return &fakeGroupGate{byJID: map[string]*groupentity.Group{
		jid: {ID: "grp1", TenantID: "t1", GroupJID: jid, Name: name, Description: "desc", Attend: true},
	}}
}

// A group message that IS attended creates exactly ONE group contact and ONE
// conversation — even across DIFFERENT members — instead of one contact per sender
// (the bug this domain fixes).
func TestInbound_Group_Attended_OneContactOneConversation(t *testing.T) {
	const jid = "120363000000000000@g.us"
	fx, contacts := newGroupFixture(attendedGate(jid, "Cliente A"))
	ctx := tenantCtx()

	first, err := fx.svc.Handle(ctx, conn(""), groupMsg("g-1", jid, "5544100@s.whatsapp.net", "João"))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	// A different member sends the next message in the SAME group.
	second, err := fx.svc.Handle(ctx, conn(""), groupMsg("g-2", jid, "5544200@s.whatsapp.net", "Maria"))
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if second.ConversationID != first.ConversationID {
		t.Errorf("a group is ONE conversation; got two (%s, %s)", first.ConversationID, second.ConversationID)
	}
	if len(fx.convs.items) != 1 {
		t.Errorf("expected 1 conversation, got %d", len(fx.convs.items))
	}
	if fx.msgs.count() != 2 {
		t.Errorf("expected 2 messages, got %d", fx.msgs.count())
	}
	if len(contacts.byID) != 1 {
		t.Fatalf("expected exactly 1 GROUP contact (not one per member), got %d", len(contacts.byID))
	}
	// The single contact is a group, keyed by the group JID — that identity is what
	// the outbound webhook dials, so a reply already routes to the group.
	var c *contactentity.Contact
	for _, v := range contacts.byID {
		c = v
	}
	if c.Kind != contactentity.KindGroup {
		t.Errorf("contact kind = %q, want group", c.Kind)
	}
	if c.Name != "Cliente A" {
		t.Errorf("group contact name should come from the registry, got %q", c.Name)
	}
	if !c.HasIdentity("whatsapp", jid) {
		t.Errorf("group contact identity must be {whatsapp, %s} (the outbound routing key); got %+v", jid, c.Identities)
	}
	if c.Phone != "" || len(c.Phones) != 0 {
		t.Errorf("a group contact must have no phone, got phone=%q phones=%v", c.Phone, c.Phones)
	}
	// The registry description is seeded into the contact's Notes for the agent.
	if c.Notes != "desc" {
		t.Errorf("group description should seed the contact's notes, got %q", c.Notes)
	}
}

// A group message records WHICH member sent it as display metadata only — the member
// is never turned into a contact, and the message stays a normal customer message.
func TestInbound_Group_SetsGroupSenderMetadata(t *testing.T) {
	const jid = "120363000000000001@g.us"
	fx, _ := newGroupFixture(attendedGate(jid, "Grupo X"))

	res, err := fx.svc.Handle(tenantCtx(), conn(""), groupMsg("g-meta", jid, "5544300@s.whatsapp.net", "Carlos"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	m := fx.msgs.items[res.MessageID]
	if m.GroupSender == nil {
		t.Fatal("a group message must carry GroupSender metadata")
	}
	if m.GroupSender.JID != "5544300@s.whatsapp.net" || m.GroupSender.Name != "Carlos" {
		t.Errorf("group_sender mismatch: %+v", m.GroupSender)
	}
	if m.SenderType != conventity.SenderCustomer {
		t.Errorf("a group message is still a customer message, got sender_type=%q", m.SenderType)
	}
	if m.SenderID != res.ContactID {
		t.Errorf("sender_id must be the GROUP contact (%s), got %q", res.ContactID, m.SenderID)
	}
}

// A 1:1 message is completely untouched: it still creates a per-person contact and
// carries NO group_sender. (Guards the bifurcation from leaking into the 1:1 path.)
func TestInbound_OneToOne_Unchanged_NoGroupSender(t *testing.T) {
	fx, contacts := newGroupFixture(attendedGate("unused@g.us", "X"))

	res, err := fx.svc.Handle(tenantCtx(), conn(""), inMsg("solo-1"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	m := fx.msgs.items[res.MessageID]
	if m.GroupSender != nil {
		t.Errorf("a 1:1 message must not carry group_sender, got %+v", m.GroupSender)
	}
	if len(contacts.byID) != 1 {
		t.Fatalf("1:1 must create the per-person contact, got %d contacts", len(contacts.byID))
	}
	var c *contactentity.Contact
	for _, v := range contacts.byID {
		c = v
	}
	if c.IsGroup() {
		t.Error("a 1:1 contact must not be a group")
	}
}

// A shared CONTACT in an attended group materializes a message (message_type=contact)
// in the group conversation — not silently dropped just because there is no file.
func TestInbound_Group_Contact_CreatesMessage(t *testing.T) {
	const jid = "120363000000000010@g.us"
	fx, _ := newGroupFixture(attendedGate(jid, "Grupo"))
	msg := groupMsg("g-contact", jid, "5544@s.whatsapp.net", "João")
	msg.Text = "compartilhou um contato"
	msg.Contacts = []conventity.ContactCard{{
		Name:   conventity.ContactName{Formatted: "Maria"},
		Phones: []conventity.ContactPhone{{Phone: "5544111222"}},
	}}

	res, err := fx.svc.Handle(tenantCtx(), conn(""), msg)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if res.Discarded || fx.msgs.count() != 1 {
		t.Fatalf("a group contact must create one message, got discarded=%v count=%d", res.Discarded, fx.msgs.count())
	}
	m := fx.msgs.items[res.MessageID]
	if m.MessageType != conventity.MessageContact {
		t.Errorf("message_type = %q, want contact", m.MessageType)
	}
	if len(m.Contacts) != 1 || m.GroupSender == nil {
		t.Errorf("contact + group_sender must be persisted: contacts=%d group_sender=%v", len(m.Contacts), m.GroupSender)
	}
}

// A shared LOCATION in an attended group materializes a message (message_type=location).
func TestInbound_Group_Location_CreatesMessage(t *testing.T) {
	const jid = "120363000000000011@g.us"
	fx, _ := newGroupFixture(attendedGate(jid, "Grupo"))
	msg := groupMsg("g-loc", jid, "5544@s.whatsapp.net", "João")
	msg.Text = "estou aqui"
	msg.Location = &conventity.Location{Latitude: -23.55, Longitude: -46.63, Name: "SP"}

	res, err := fx.svc.Handle(tenantCtx(), conn(""), msg)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if res.Discarded || fx.msgs.count() != 1 {
		t.Fatalf("a group location must create one message, got discarded=%v count=%d", res.Discarded, fx.msgs.count())
	}
	m := fx.msgs.items[res.MessageID]
	if m.MessageType != conventity.MessageLocation || m.Location == nil {
		t.Errorf("message_type/location mismatch: type=%q location=%v", m.MessageType, m.Location)
	}
}

// A group that is NOT attended (attend=false) is discarded: 200 + nothing persisted,
// so the gateway does not retry and no junk contact/conversation is created.
func TestInbound_Group_NotAttended_Discarded(t *testing.T) {
	const jid = "120363000000000002@g.us"
	gate := &fakeGroupGate{byJID: map[string]*groupentity.Group{
		jid: {ID: "grp", TenantID: "t1", GroupJID: jid, Attend: false},
	}}
	fx, contacts := newGroupFixture(gate)

	res, err := fx.svc.Handle(tenantCtx(), conn(""), groupMsg("g-na", jid, "5544@s.whatsapp.net", "Z"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !res.Discarded || res.Status != "discarded" {
		t.Errorf("a non-attended group must be discarded, got %+v", res)
	}
	if fx.msgs.count() != 0 || len(fx.convs.items) != 0 || len(contacts.byID) != 0 {
		t.Errorf("discard must persist nothing (msgs=%d convs=%d contacts=%d)",
			fx.msgs.count(), len(fx.convs.items), len(contacts.byID))
	}
}

// A group that was NEVER synced (absent from the registry) is discarded too — same
// effect as attend=false. The operator must sync it (Domain 1) first.
func TestInbound_Group_NotSynced_Discarded(t *testing.T) {
	fx, contacts := newGroupFixture(&fakeGroupGate{byJID: map[string]*groupentity.Group{}})

	res, err := fx.svc.Handle(tenantCtx(), conn(""), groupMsg("g-ns", "999@g.us", "5544@s.whatsapp.net", "Z"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !res.Discarded {
		t.Errorf("an unsynced group must be discarded, got %+v", res)
	}
	if fx.msgs.count() != 0 || len(contacts.byID) != 0 {
		t.Errorf("discard must persist nothing")
	}
}

// With no group gate wired, a group message is discarded (fail-safe) rather than
// falling through to create a per-member contact.
func TestInbound_Group_GateNotConfigured_Discarded(t *testing.T) {
	fx, contacts := newGroupFixture(nil)

	res, err := fx.svc.Handle(tenantCtx(), conn(""), groupMsg("g-nogate", "1@g.us", "5544@s.whatsapp.net", "Z"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !res.Discarded {
		t.Errorf("with no gate, a group message must be discarded, got %+v", res)
	}
	if len(contacts.byID) != 0 {
		t.Errorf("must not create any contact when the gate is unconfigured, got %d", len(contacts.byID))
	}
}

// A re-delivery of a discarded group message re-evaluates the gate (no idempotency is
// spent on a discard): once the operator turns attend on, the NEXT delivery enters.
func TestInbound_Group_DiscardThenAttend_Enters(t *testing.T) {
	const jid = "120363000000000003@g.us"
	grp := &groupentity.Group{ID: "grp", TenantID: "t1", GroupJID: jid, Name: "Late", Attend: false}
	gate := &fakeGroupGate{byJID: map[string]*groupentity.Group{jid: grp}}
	fx, contacts := newGroupFixture(gate)
	ctx := tenantCtx()

	if res, _ := fx.svc.Handle(ctx, conn(""), groupMsg("g-late-1", jid, "5544@s.whatsapp.net", "Z")); !res.Discarded {
		t.Fatalf("first delivery must be discarded while attend=false")
	}
	// Operator enables attendance; a NEW message now enters normally.
	grp.Attend = true
	res, err := fx.svc.Handle(ctx, conn(""), groupMsg("g-late-2", jid, "5544@s.whatsapp.net", "Z"))
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if res.Discarded {
		t.Errorf("after attend=true the message must enter, got discarded")
	}
	if fx.msgs.count() != 1 || len(contacts.byID) != 1 {
		t.Errorf("expected exactly the post-attend message+contact, got msgs=%d contacts=%d", fx.msgs.count(), len(contacts.byID))
	}
}

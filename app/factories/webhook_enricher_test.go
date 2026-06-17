package factories

import (
	"testing"

	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
)

// A GROUP contact maps to an outbound-webhook recipient that carries the group JID as
// a routable identity (contact.identities[].external_id) AND is_group=true — exactly
// what the gateway needs to reply into the group. This guards the Domain-2 outbound
// path the WhatsApp gateway consumes.
func TestNewWebhookContact_GroupCarriesJIDIdentityAndIsGroup(t *testing.T) {
	const jid = "120363025246125486@g.us"
	c := &contactentity.Contact{
		ID:   "ct_grp",
		Kind: contactentity.KindGroup,
		Name: "Cliente Acme",
		Identities: []contactentity.ChannelIdentity{
			{Channel: "whatsapp", ExternalID: jid},
		},
	}

	wc := newWebhookContact(c)
	if wc == nil {
		t.Fatal("expected a webhook contact")
	}
	if !wc.IsGroup {
		t.Error("a group contact must serialize is_group=true so the gateway knows it is a group")
	}
	if len(wc.Identities) != 1 {
		t.Fatalf("expected the group JID identity to be carried, got %+v", wc.Identities)
	}
	if wc.Identities[0].ExternalID != jid {
		t.Errorf("external_id must be the group JID (the routing key), got %q", wc.Identities[0].ExternalID)
	}
	if wc.Identities[0].Channel != "whatsapp" {
		t.Errorf("identity channel mismatch: %q", wc.Identities[0].Channel)
	}
	if wc.Phone != "" {
		t.Errorf("a group has no phone, got %q", wc.Phone)
	}
}

// A normal person contact must NOT leak is_group, and still carries its identity.
func TestNewWebhookContact_PersonHasNoIsGroup(t *testing.T) {
	c := &contactentity.Contact{
		ID:         "ct_p",
		Name:       "Maria",
		Phone:      "5544999990000",
		Identities: []contactentity.ChannelIdentity{{Channel: "whatsapp", ExternalID: "5544999990000@s.whatsapp.net"}},
	}
	wc := newWebhookContact(c)
	if wc.IsGroup {
		t.Error("a person contact must not set is_group")
	}
	if wc.Phone != "5544999990000" {
		t.Errorf("person phone must be carried, got %q", wc.Phone)
	}
}

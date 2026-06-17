package channels

import (
	"encoding/json"
	"testing"
)

// A gateway group message in JSON (group_jid + sender_* + a shared contact + text)
// must map onto the domain InboundMessage with the group context AND the structured
// contact preserved — so the service materializes a group message, not a silent 200.
func TestInboundRequest_ToMessage_GroupContact(t *testing.T) {
	body := `{
		"external_message_id": "wamid.1",
		"group_jid": "120363025246125486@g.us",
		"sender_jid": "5544999990000@s.whatsapp.net",
		"sender_name": "João",
		"sender_phone": "5544999990000",
		"text": "compartilhou um contato",
		"contacts": [
			{ "name": { "formatted": "Maria Silva" }, "phones": [ { "phone": "5544111222" } ] }
		]
	}`
	var req InboundRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	msg := req.ToMessage("whatsapp")

	if msg.GroupJID != "120363025246125486@g.us" {
		t.Errorf("group_jid not mapped: %q", msg.GroupJID)
	}
	if msg.SenderJID != "5544999990000@s.whatsapp.net" || msg.SenderName != "João" {
		t.Errorf("sender not mapped: jid=%q name=%q", msg.SenderJID, msg.SenderName)
	}
	if len(msg.Contacts) != 1 || msg.Contacts[0].Name.Formatted != "Maria Silva" {
		t.Fatalf("contacts not mapped: %+v", msg.Contacts)
	}
	if msg.Text != "compartilhou um contato" {
		t.Errorf("text not mapped: %q", msg.Text)
	}
}

// A gateway group LOCATION message in JSON must map the location block + group context.
func TestInboundRequest_ToMessage_GroupLocation(t *testing.T) {
	body := `{
		"external_message_id": "wamid.2",
		"group_jid": "120363025246125486@g.us",
		"sender_jid": "5544999990000@s.whatsapp.net",
		"text": "estou aqui",
		"location": { "latitude": -23.55, "longitude": -46.63, "name": "SP", "address": "Centro" }
	}`
	var req InboundRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	msg := req.ToMessage("whatsapp")

	if msg.GroupJID == "" {
		t.Error("group_jid not mapped")
	}
	if msg.Location == nil {
		t.Fatal("location not mapped")
	}
	if msg.Location.Latitude != -23.55 || msg.Location.Name != "SP" {
		t.Errorf("location fields not mapped: %+v", msg.Location)
	}
}

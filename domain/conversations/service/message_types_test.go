package service

import (
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

func validContact() entity.ContactCard {
	return entity.ContactCard{
		Name:   entity.ContactName{Formatted: "Maria Silva"},
		Phones: []entity.ContactPhone{{Phone: "+5511999998888", Type: "CELL"}},
	}
}

// TestSendMessage_Contact: a valid contact send stores message_type=contact with the
// typed contacts[], and surfaces them on the realtime payload.
func TestSendMessage_Contact(t *testing.T) {
	svc, _, mr, _, pub := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)

	msg, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{
		MessageType: entity.MessageContact,
		Contacts:    []entity.ContactCard{validContact()},
	})
	if err != nil {
		t.Fatalf("send contact: %v", err)
	}
	if msg.MessageType != entity.MessageContact || len(msg.Contacts) != 1 || msg.Contacts[0].Name.Formatted != "Maria Silva" {
		t.Fatalf("contact not stored: %+v", msg)
	}
	stored := mr.items[len(mr.items)-1]
	if len(stored.Contacts) != 1 {
		t.Errorf("persisted message must carry contacts, got %+v", stored)
	}
	p, ok := pub.lastPayload(contracts.RealtimeMessageCreated).(contracts.MessagePayload)
	if !ok || len(p.Contacts) != 1 || p.MessageType != string(entity.MessageContact) {
		t.Errorf("realtime payload must carry contacts + type, got %+v", p)
	}
}

func TestSendMessage_Contact_Invalid(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)

	cases := map[string][]entity.ContactCard{
		"no contacts": nil,
		"no phone":    {{Name: entity.ContactName{Formatted: "X"}}},
		"no name":     {{Phones: []entity.ContactPhone{{Phone: "+551199"}}}},
	}
	for name, cs := range cases {
		_, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{MessageType: entity.MessageContact, Contacts: cs})
		if apperror.From(err).Code != apperror.CodeValidation {
			t.Errorf("%s: expected validation error, got %v", name, err)
		}
	}
}

// TestSendMessage_Location: valid location stored; out-of-range rejected.
func TestSendMessage_Location(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)

	msg, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{
		MessageType: entity.MessageLocation,
		Location:    &entity.Location{Latitude: -27.59, Longitude: -48.54, Name: "Loja"},
	})
	if err != nil {
		t.Fatalf("send location: %v", err)
	}
	if msg.MessageType != entity.MessageLocation || msg.Location == nil || msg.Location.Name != "Loja" {
		t.Fatalf("location not stored: %+v", msg)
	}

	for _, bad := range []*entity.Location{nil, {Latitude: 200, Longitude: 0}, {Latitude: 0, Longitude: 999}} {
		_, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{MessageType: entity.MessageLocation, Location: bad})
		if apperror.From(err).Code != apperror.CodeValidation {
			t.Errorf("location %+v: expected validation error, got %v", bad, err)
		}
	}
}

func buttonsMenu() *entity.Interactive {
	return &entity.Interactive{Kind: entity.InteractiveButtons, Body: "Pick one", Buttons: []entity.InteractiveButton{
		{ID: "a", Title: "Option A"}, {ID: "b", Title: "Option B"},
	}}
}

// TestSendMessage_Interactive: a valid buttons menu stores message_type=interactive,
// mirrors body to text, and surfaces the menu on the realtime payload.
func TestSendMessage_Interactive(t *testing.T) {
	svc, _, _, _, pub := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)

	msg, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{
		MessageType: entity.MessageInteractive, Interactive: buttonsMenu(),
	})
	if err != nil {
		t.Fatalf("send interactive: %v", err)
	}
	if msg.MessageType != entity.MessageInteractive || msg.Interactive == nil || len(msg.Interactive.Buttons) != 2 {
		t.Fatalf("interactive not stored: %+v", msg)
	}
	if msg.Text != "Pick one" {
		t.Errorf("body must mirror to text, got %q", msg.Text)
	}
	if p, ok := pub.lastPayload(contracts.RealtimeMessageCreated).(contracts.MessagePayload); !ok || p.Interactive == nil {
		t.Errorf("realtime payload must carry interactive")
	}
}

func TestSendMessage_Interactive_Invalid(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)

	long := ""
	for i := 0; i < 21; i++ {
		long += "x"
	}
	cases := map[string]*entity.Interactive{
		"no body":      {Kind: entity.InteractiveButtons, Buttons: []entity.InteractiveButton{{ID: "a", Title: "A"}}},
		"4 buttons":    {Kind: entity.InteractiveButtons, Body: "b", Buttons: []entity.InteractiveButton{{ID: "1", Title: "A"}, {ID: "2", Title: "B"}, {ID: "3", Title: "C"}, {ID: "4", Title: "D"}}},
		"title>20":     {Kind: entity.InteractiveButtons, Body: "b", Buttons: []entity.InteractiveButton{{ID: "a", Title: long}}},
		"dup id":       {Kind: entity.InteractiveButtons, Body: "b", Buttons: []entity.InteractiveButton{{ID: "a", Title: "A"}, {ID: "a", Title: "B"}}},
		"unknown kind": {Kind: "carousel", Body: "b"},
		"list no btn":  {Kind: entity.InteractiveList, Body: "b", Sections: []entity.InteractiveSection{{Rows: []entity.InteractiveRow{{ID: "r", Title: "R"}}}}},
	}
	for name, iv := range cases {
		_, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{MessageType: entity.MessageInteractive, Interactive: iv})
		if apperror.From(err).Code != apperror.CodeValidation {
			t.Errorf("%s: expected validation error, got %v", name, err)
		}
	}
}

// TestSendMessage_Interactive_List validates a list with >10 rows is rejected.
func TestSendMessage_Interactive_List(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)

	rows := make([]entity.InteractiveRow, 11)
	for i := range rows {
		rows[i] = entity.InteractiveRow{ID: string(rune('a' + i)), Title: "Row"}
	}
	_, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{
		MessageType: entity.MessageInteractive,
		Interactive: &entity.Interactive{Kind: entity.InteractiveList, Body: "b", Button: "Open", Sections: []entity.InteractiveSection{{Rows: rows}}},
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf(">10 rows must be rejected, got %v", err)
	}
}

// TestSendAutomationInteractive: a rule sends an interactive menu via JSON param.
func TestSendAutomationInteractive(t *testing.T) {
	svc, cr, mr, _, _ := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)
	_ = cr

	good := `{"kind":"buttons","body":"Menu","buttons":[{"id":"x","title":"X"}]}`
	if err := svc.SendAutomationInteractive(adminCtx(), id, "rule1", good); err != nil {
		t.Fatalf("automation interactive: %v", err)
	}
	last := mr.items[len(mr.items)-1]
	if last.MessageType != entity.MessageInteractive || last.SenderType != entity.SenderAutomation || last.Interactive == nil {
		t.Fatalf("automation interactive not stored: %+v", last)
	}
	// Bad JSON and limit violations are rejected.
	if err := svc.SendAutomationInteractive(adminCtx(), id, "rule1", "{not json"); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("bad json must be a validation error, got %v", err)
	}
	if err := svc.SendAutomationInteractive(adminCtx(), id, "rule1", `{"kind":"buttons","body":"b"}`); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("no buttons must be a validation error, got %v", err)
	}
}

// TestSendMessage_InteractiveTemplateGatedToWhatsApp: interactive and template are
// WhatsApp-only — rejected on a non-whatsapp channel; the other kinds are fine.
func TestSendMessage_InteractiveTemplateGatedToWhatsApp(t *testing.T) {
	svc, cr, _, _, _ := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)
	cr.items[id].Channel = "api" // a generic API channel (not whatsapp)

	if _, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{
		MessageType: entity.MessageInteractive, Interactive: buttonsMenu(),
	}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("interactive on a non-whatsapp channel must be 422, got %v", err)
	}
	if _, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{
		MessageType: entity.MessageTemplate,
	}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("template on a non-whatsapp channel must be 422, got %v", err)
	}
	// A plain text message is unaffected on the same channel.
	if _, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{Text: "oi"}); err != nil {
		t.Errorf("text must still work on a non-whatsapp channel: %v", err)
	}
}

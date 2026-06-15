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

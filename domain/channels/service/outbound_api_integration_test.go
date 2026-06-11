package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/api"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/sign"
)

// apiRegistry returns the real generic API channel adapter for any type.
type apiRegistry struct{ a chcontracts.Adapter }

func (r apiRegistry) For(chentity.Type) chcontracts.Adapter { return r.a }

const apiSecret = "integration-secret"

// TestDeliver_RealAPIAdapter_EndToEnd drives the OutboundService through the real
// API channel adapter against an httptest server simulating the external system:
// the agent's message is delivered by a signed POST (delivery_status pending→sent),
// then delivery/read receipts advance the status.
func TestDeliver_RealAPIAdapter_EndToEnd(t *testing.T) {
	var gotSigned bool
	var gotEnvelope map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ts := r.Header.Get("Timestamp")
		if r.Header.Get("Signature") == sign.Payload(apiSecret, ts, body) && r.Header.Get("X-Chat-Event") != "" {
			gotSigned = true
		}
		_ = json.Unmarshal(body, &gotEnvelope)
		_, _ = w.Write([]byte(`{"external_message_id":"EXT-1"}`))
	}))
	defer srv.Close()

	conns := newFakeConnRepo()
	conns.put(&chentity.ChannelConnection{
		ID: "conn-api", TenantID: "t1", Type: chentity.TypeAPI,
		BaseURL: srv.URL, Secret: apiSecret, Enabled: true,
	})
	deliveries := newFakeDeliveryRepo()
	convs := newFakeConvRepo()
	convs.Create(context.Background(), &conventity.Conversation{ID: "conv1", TenantID: "t1", ContactID: "cont1", Channel: "api", AssignedTo: "agent1"})
	msgs := newFakeMsgRepo()
	contacts := newFakeContactRepo()
	contacts.byID["cont1"] = &contactentity.Contact{ID: "cont1", TenantID: "t1", Name: "Alice", Phone: "5511", Identities: []contactentity.ChannelIdentity{{Channel: "api", ExternalID: "ext-c"}}}
	pub := &fakePublisher{}
	svc := NewOutboundService(conns, deliveries, convs, msgs, contacts, apiRegistry{api.New(chentity.TypeAPI)}, &fakeEnqueuer{}, pub, clockNow())

	msgs.Create(context.Background(), &conventity.Message{ID: "msg1", TenantID: "t1", ConversationID: "conv1", Direction: conventity.DirectionOutbound, Text: "olá", DeliveryStatus: conventity.DeliveryPending})
	deliveries.Create(context.Background(), &chentity.OutboundDelivery{ID: "del1", TenantID: "t1", ChannelConnectionID: "conn-api", ConversationID: "conv1", MessageID: "msg1", Status: chentity.DeliveryPending})

	// pending → sent via a signed POST to outbound_url.
	if err := svc.Deliver(tenantCtx(), "del1"); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if !gotSigned {
		t.Fatal("external system did not receive a properly signed POST")
	}
	if gotEnvelope["delivery_id"] != "del1" || gotEnvelope["conversation_id"] != "conv1" {
		t.Errorf("envelope ids not delivered: %+v", gotEnvelope)
	}
	d := deliveries.byID["del1"]
	if d.Status != chentity.DeliverySent || d.ExternalMessageID != "EXT-1" {
		t.Fatalf("delivery should be sent with external id EXT-1, got %+v", d)
	}
	if msgs.items["msg1"].DeliveryStatus != conventity.DeliverySent {
		t.Errorf("message should be sent, got %s", msgs.items["msg1"].DeliveryStatus)
	}

	// sent → delivered → read via receipts (parsed by the same adapter).
	applied, err := svc.ProcessReceipts(tenantCtx(), conns.byID["conn-api"], []byte(`{"external_message_id":"EXT-1","status":"delivered"}`))
	if err != nil || applied != 1 {
		t.Fatalf("process delivered receipt: applied=%d err=%v", applied, err)
	}
	if msgs.items["msg1"].DeliveryStatus != conventity.DeliveryDelivered {
		t.Errorf("message should be delivered, got %s", msgs.items["msg1"].DeliveryStatus)
	}
	if _, err := svc.ProcessReceipts(tenantCtx(), conns.byID["conn-api"], []byte(`{"external_message_id":"EXT-1","status":"read"}`)); err != nil {
		t.Fatalf("process read receipt: %v", err)
	}
	if msgs.items["msg1"].DeliveryStatus != conventity.DeliveryRead {
		t.Errorf("message should be read, got %s", msgs.items["msg1"].DeliveryStatus)
	}
}

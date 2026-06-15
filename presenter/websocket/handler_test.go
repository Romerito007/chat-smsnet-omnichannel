package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/realtime"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/security"
)

func testManager() *security.JWTManager {
	return security.NewJWTManager("ws-secret", "chat-backend", 15*time.Minute, time.Hour)
}

func issueToken(t *testing.T, m *security.JWTManager, perms []authz.Permission, sectors []string) string {
	t.Helper()
	tok, _, err := m.IssueAccess(auth.AccessClaims{
		TenantID:    "t1",
		UserID:      "u1",
		Permissions: perms,
		SectorIDs:   sectors,
		SectorScope: authz.ScopeOwn,
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	return tok
}

func newServer(t *testing.T, hub *realtime.Hub, m *security.JWTManager, maxConn int) (*httptest.Server, string) {
	t.Helper()
	h := NewHandler(hub, m, shared.NewLogger("error"), maxConn, 0)
	srv := httptest.NewServer(h)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return srv, wsURL
}

// waitForSubscription polls until the topic has a local subscriber, so delivery
// is deterministic (no racing the handshake/subscribe).
func waitForSubscription(t *testing.T, hub *realtime.Hub, topic string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hub.HasSubscribers(topic) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no subscriber appeared for %q", topic)
}

// expectDelivery waits for the subscription, delivers once, and reads one frame.
func expectDelivery(t *testing.T, hub *realtime.Hub, topic string, conn *websocket.Conn) []byte {
	t.Helper()
	waitForSubscription(t, hub, topic)
	hub.Deliver(realtime.Message{Topic: topic, Payload: []byte(`{"event":"test"}`)})
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read on %q: %v", topic, err)
	}
	return msg
}

func TestWS_RejectsWithoutToken(t *testing.T) {
	hub := realtime.NewHub()
	srv, wsURL := newServer(t, hub, testManager(), 0)
	defer srv.Close()

	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected handshake to fail without a token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

func TestWS_RejectsInvalidToken(t *testing.T) {
	hub := realtime.NewHub()
	srv, wsURL := newServer(t, hub, testManager(), 0)
	defer srv.Close()

	_, resp, err := websocket.DefaultDialer.Dial(wsURL+"?token=garbage", nil)
	if err == nil {
		t.Fatal("expected handshake to fail with an invalid token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

func TestWS_ConnectsAndReceivesUserRoomEvent(t *testing.T) {
	hub := realtime.NewHub()
	m := testManager()
	srv, wsURL := newServer(t, hub, m, 0)
	defer srv.Close()

	token := issueToken(t, m, []authz.Permission{authz.ConversationRead}, []string{"s1"})
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL+"?token="+token, nil)
	if err != nil {
		t.Fatalf("dial: %v (resp %v)", err, resp)
	}
	defer conn.Close()

	// Auto-joined the user room: a delivery there reaches the client.
	got := expectDelivery(t, hub, shared.TopicUser("t1", "u1"), conn)
	if !strings.Contains(string(got), "test") {
		t.Fatalf("unexpected payload %q", got)
	}
}

func TestWS_AutoJoinsSectorRoom(t *testing.T) {
	hub := realtime.NewHub()
	m := testManager()
	srv, wsURL := newServer(t, hub, m, 0)
	defer srv.Close()

	token := issueToken(t, m, []authz.Permission{authz.ConversationRead}, []string{"s1"})
	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?token="+token, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	expectDelivery(t, hub, shared.TopicInbox("t1", "s1"), conn)
}

func TestWS_SubscribeConversationRoom(t *testing.T) {
	hub := realtime.NewHub()
	m := testManager()
	srv, wsURL := newServer(t, hub, m, 0)
	defer srv.Close()

	token := issueToken(t, m, []authz.Permission{authz.ConversationRead}, nil)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?token="+token, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]string{"action": "subscribe", "conversation_id": "conv1"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	expectDelivery(t, hub, shared.TopicConversation("t1", "conv1"), conn)
}

func TestWS_EnforcesConnectionLimit(t *testing.T) {
	hub := realtime.NewHub()
	m := testManager()
	srv, wsURL := newServer(t, hub, m, 1) // max 1 per user
	defer srv.Close()

	token := issueToken(t, m, nil, nil)
	c1, _, err := websocket.DefaultDialer.Dial(wsURL+"?token="+token, nil)
	if err != nil {
		t.Fatalf("first dial: %v", err)
	}
	defer c1.Close()

	_, resp, err := websocket.DefaultDialer.Dial(wsURL+"?token="+token, nil)
	if err == nil {
		t.Fatal("expected the second connection to be rejected")
	}
	if resp == nil || resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %v", resp)
	}
}

// TestResyncFrame_WireContract pins the frame the server sends on a dropped event,
// which the frontend must handle exactly like a reconnect (refetch its state).
func TestResyncFrame_WireContract(t *testing.T) {
	var env struct {
		Event string         `json:"event"`
		Ts    int64          `json:"ts"`
		Data  map[string]any `json:"data"`
	}
	if err := json.Unmarshal(resyncFrame(), &env); err != nil {
		t.Fatalf("resync frame must be valid JSON: %v", err)
	}
	if env.Event != "realtime.resync" {
		t.Errorf("event = %q, want realtime.resync", env.Event)
	}
	if env.Data["reason"] != "slow_consumer" {
		t.Errorf("reason = %v, want slow_consumer", env.Data["reason"])
	}
	if env.Ts == 0 {
		t.Error("ts must be set")
	}
}

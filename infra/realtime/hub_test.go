package realtime

import "testing"

func newClient(tenant, user string) *Client {
	return &Client{
		ID:       "c-" + user,
		TenantID: tenant,
		UserID:   user,
		Send:     make(chan Message, 4),
		Topics:   map[Topic]struct{}{},
	}
}

func TestHub_DeliverOnlyToSubscribers(t *testing.T) {
	h := NewHub()
	a := newClient("t1", "a")
	b := newClient("t1", "b")
	h.Subscribe(a, "room:1")
	// b is not subscribed to room:1.

	h.Deliver(Message{Topic: "room:1", Payload: []byte("hi")})

	select {
	case m := <-a.Send:
		if string(m.Payload) != "hi" {
			t.Fatalf("payload = %q", m.Payload)
		}
	default:
		t.Fatal("subscriber a did not receive the message")
	}
	select {
	case <-b.Send:
		t.Fatal("non-subscriber b should not receive the message")
	default:
	}
}

func TestHub_RemoveCleansTopicsAndCount(t *testing.T) {
	h := NewHub()
	a := newClient("t1", "a")
	h.Register(a, 0)
	h.Subscribe(a, "room:1")
	h.Subscribe(a, "room:2")

	if h.ConnectionsFor("t1", "a") != 1 {
		t.Fatalf("expected 1 connection, got %d", h.ConnectionsFor("t1", "a"))
	}
	h.Remove(a)
	if h.ConnectionsFor("t1", "a") != 0 {
		t.Errorf("connection count not released")
	}
	// Delivering to the old rooms reaches nobody (no panic, no send).
	h.Deliver(Message{Topic: "room:1", Payload: []byte("x")})
	if len(a.Topics) != 0 {
		t.Errorf("client topics not cleared")
	}
}

func TestHub_RegisterEnforcesPerUserLimit(t *testing.T) {
	h := NewHub()
	c1 := newClient("t1", "a")
	c2 := newClient("t1", "a")
	c3 := newClient("t1", "a")

	if !h.Register(c1, 2) {
		t.Fatal("first connection should be allowed")
	}
	if !h.Register(c2, 2) {
		t.Fatal("second connection should be allowed")
	}
	if h.Register(c3, 2) {
		t.Fatal("third connection should be rejected by the limit")
	}

	// Freeing one slot allows a new connection.
	h.Remove(c1)
	if !h.Register(c3, 2) {
		t.Fatal("connection should be allowed after one is removed")
	}
}

func TestHub_RegisterUnlimitedWhenMaxZero(t *testing.T) {
	h := NewHub()
	for i := 0; i < 100; i++ {
		if !h.Register(newClient("t1", "a"), 0) {
			t.Fatalf("max=0 should be unlimited, rejected at %d", i)
		}
	}
}

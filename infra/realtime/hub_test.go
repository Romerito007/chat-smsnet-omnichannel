package realtime

import "testing"

func newClient(tenant, user string) *Client {
	return &Client{
		ID:       "c-" + user,
		TenantID: tenant,
		UserID:   user,
		Send:     make(chan Message, 4),
		Topics:   map[Topic]struct{}{},
		Resync:   make(chan struct{}, 1),
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

// TestHub_DeliverDropSignalsResync: when a subscriber's send buffer is full, the
// message is dropped (non-blocking, so fan-out never stalls) BUT the client is
// flagged for resync — so it learns it missed events instead of silently showing a
// stale thread until a manual refresh (the "só vejo no F5" bug).
func TestHub_DeliverDropSignalsResync(t *testing.T) {
	h := NewHub()
	c := &Client{ID: "c1", TenantID: "t1", UserID: "u1", Send: make(chan Message, 1), Topics: map[Topic]struct{}{}, Resync: make(chan struct{}, 1)}
	h.Subscribe(c, "room:1")

	// First delivery fills the 1-slot buffer; no resync yet.
	h.Deliver(Message{Topic: "room:1", Payload: []byte("m1")})
	select {
	case <-c.Resync:
		t.Fatal("no resync expected while the buffer still had room")
	default:
	}
	// Buffer is now full: the next delivery is dropped and must raise resync.
	h.Deliver(Message{Topic: "room:1", Payload: []byte("m2")})
	select {
	case <-c.Resync:
	default:
		t.Fatal("a dropped delivery must signal the client to resync")
	}
}

// TestHub_ResyncCoalesces: many drops collapse into a single pending resync (one
// refetch recovers all missed events), so the signal never piles up.
func TestHub_ResyncCoalesces(t *testing.T) {
	h := NewHub()
	c := &Client{ID: "c1", TenantID: "t1", UserID: "u1", Send: make(chan Message, 1), Topics: map[Topic]struct{}{}, Resync: make(chan struct{}, 1)}
	h.Subscribe(c, "room:1")
	for i := 0; i < 50; i++ { // far more drops than the 1-slot buffer
		h.Deliver(Message{Topic: "room:1", Payload: []byte("x")})
	}
	// Exactly one pending resync (capacity 1, coalesced).
	select {
	case <-c.Resync:
	default:
		t.Fatal("expected one pending resync")
	}
	select {
	case <-c.Resync:
		t.Fatal("resync must coalesce into a single pending signal")
	default:
	}
}

// TestHub_ResyncRaisedEvenWhenSendFull confirms the resync signal is out-of-band:
// it is raised even though Send is saturated (it does not depend on buffer space).
func TestHub_ResyncRaisedEvenWhenSendFull(t *testing.T) {
	h := NewHub()
	c := &Client{ID: "c1", TenantID: "t1", UserID: "u1", Send: make(chan Message, 1), Topics: map[Topic]struct{}{}, Resync: make(chan struct{}, 1)}
	h.Subscribe(c, "room:1")
	h.Deliver(Message{Topic: "room:1", Payload: []byte("fill")}) // Send now full
	h.Deliver(Message{Topic: "room:1", Payload: []byte("drop")}) // dropped
	if len(c.Send) != 1 {
		t.Fatalf("send buffer should stay at capacity 1, got %d", len(c.Send))
	}
	select {
	case <-c.Resync:
	default:
		t.Fatal("resync must be raised out-of-band while Send is full")
	}
}

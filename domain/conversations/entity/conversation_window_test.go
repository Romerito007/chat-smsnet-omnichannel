package entity

import (
	"testing"
	"time"
)

func TestWhatsAppWindowState(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-2 * time.Hour) // within 24h
	stale := now.Add(-25 * time.Hour) // outside 24h

	// Non-WhatsApp channel → no window block at all.
	if w := (&Conversation{Channel: "api", LastCustomerMessageAt: &recent}).WhatsAppWindowState(now); w != nil {
		t.Errorf("non-whatsapp channel must have no window, got %+v", w)
	}

	// WhatsApp + recent inbound → open, expires_at = inbound+24h.
	w := (&Conversation{Channel: ChannelTypeWhatsApp, LastCustomerMessageAt: &recent}).WhatsAppWindowState(now)
	if w == nil || !w.Open {
		t.Fatalf("recent inbound must be open, got %+v", w)
	}
	if w.ExpiresAt == nil || !w.ExpiresAt.Equal(recent.Add(24*time.Hour)) {
		t.Errorf("expires_at = %v, want %v", w.ExpiresAt, recent.Add(24*time.Hour))
	}

	// WhatsApp + stale inbound → closed, expires_at in the past.
	w = (&Conversation{Channel: ChannelTypeWhatsApp, LastCustomerMessageAt: &stale}).WhatsAppWindowState(now)
	if w == nil || w.Open {
		t.Fatalf("stale inbound must be closed, got %+v", w)
	}

	// WhatsApp + never inbound → block present, closed, no expires_at.
	w = (&Conversation{Channel: ChannelTypeWhatsApp}).WhatsAppWindowState(now)
	if w == nil || w.Open || w.ExpiresAt != nil {
		t.Errorf("no inbound must be closed with no expires_at, got %+v", w)
	}
}

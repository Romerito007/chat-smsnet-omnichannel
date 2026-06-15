package shared

import (
	"strings"
	"testing"
)

// TestInboxTopicsFor mirrors the REST inbox visibility (visibleTo):
//   - sectored conversation → its sector room only;
//   - queued/sector-less → the unassigned room (all-scope agents) + assignee.
func TestInboxTopicsFor(t *testing.T) {
	cases := []struct {
		name, sector, assignee string
		want                   []string
	}{
		{"sectored", "s1", "", []string{"t:t1:inbox:s1"}},
		{"sectored+assigned", "s1", "u9", []string{"t:t1:inbox:s1"}}, // sector room covers it
		{"queued unassigned", "", "", []string{"t:t1:unassigned"}},
		{"queued assigned", "", "u9", []string{"t:t1:unassigned", "t:t1:user:u9"}},
	}
	for _, c := range cases {
		got := InboxTopicsFor("t1", c.sector, c.assignee)
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// TestUnassignedRoomNeverLeaksToSector documents that a sector-less conversation's
// inbox update targets ONLY the unassigned room (joined by all-scope agents) — never
// a sector room — so sector-scoped agents cannot receive a conversation the REST
// inbox would hide from them.
func TestUnassignedRoomNeverLeaksToSector(t *testing.T) {
	for _, topic := range InboxTopicsFor("t1", "", "") {
		if strings.HasPrefix(topic, "t:t1:inbox:") {
			t.Errorf("queued conversation must not route to a sector inbox room, got %q", topic)
		}
	}
}

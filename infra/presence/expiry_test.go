package presence

import "testing"

func TestParsePresenceKey(t *testing.T) {
	cases := []struct {
		key              string
		wantTenant, want string
		ok               bool
	}{
		{"presence:conns:t1:u1", "t1", "u1", true},
		{"presence:t1:u1", "", "", false},             // the status hash carries no TTL
		{"presence:agents:t1", "", "", false},         // roster set, not a record
		{"presence:conns:t1", "", "", false},          // missing user
		{"presence:conns:t1:u1:extra", "", "", false}, // unexpected shape
		{"presence:conns::u1", "", "", false},         // blank tenant
		{"presence:conns:t1:", "", "", false},         // blank user
		{"cache:t1:u1", "", "", false},                // wrong prefix
		{"", "", "", false},
	}
	for _, c := range cases {
		tenant, user, ok := parsePresenceKey(c.key)
		if ok != c.ok || tenant != c.wantTenant || user != c.want {
			t.Errorf("parsePresenceKey(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.key, tenant, user, ok, c.wantTenant, c.want, c.ok)
		}
	}
}

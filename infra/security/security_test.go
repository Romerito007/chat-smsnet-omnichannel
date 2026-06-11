package security

import (
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
)

func TestBcryptHasher_RoundTrip(t *testing.T) {
	h := NewBcryptHasher(4) // low cost for test speed
	hash, err := h.Hash("correct-horse")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := h.Compare(hash, "correct-horse"); err != nil {
		t.Errorf("compare valid: %v", err)
	}
	if err := h.Compare(hash, "wrong"); err == nil {
		t.Error("compare should fail for wrong password")
	}
}

func TestJWTManager_IssueVerify(t *testing.T) {
	m := NewJWTManager("test-secret", "chat-backend", 15*time.Minute, 24*time.Hour)

	token, exp, err := m.IssueAccess(auth.AccessClaims{
		TenantID:    "t1",
		UserID:      "u1",
		Permissions: []authz.Permission{authz.ConversationRead, authz.UserManage},
		SectorIDs:   []string{"s1"},
		SectorScope: authz.ScopeAll,
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !exp.After(time.Now()) {
		t.Error("expiry should be in the future")
	}

	claims, err := m.VerifyAccess(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.TenantID != "t1" || claims.UserID != "u1" {
		t.Errorf("identity mismatch: %+v", claims)
	}
	if claims.SectorScope != authz.ScopeAll {
		t.Errorf("scope mismatch: %q", claims.SectorScope)
	}
	if len(claims.Permissions) != 2 {
		t.Errorf("permissions = %d, want 2", len(claims.Permissions))
	}
}

func TestJWTManager_RejectsTamperedToken(t *testing.T) {
	m := NewJWTManager("secret-a", "chat-backend", time.Minute, time.Hour)
	other := NewJWTManager("secret-b", "chat-backend", time.Minute, time.Hour)

	token, _, _ := m.IssueAccess(auth.AccessClaims{TenantID: "t1", UserID: "u1"})
	if _, err := other.VerifyAccess(token); err == nil {
		t.Error("token signed with a different secret must be rejected")
	}
}

func TestJWTManager_RefreshHashStable(t *testing.T) {
	m := NewJWTManager("secret", "iss", time.Minute, time.Hour)
	plain, _, err := m.GenerateRefresh()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Hashing the same token twice must yield the same digest (deterministic).
	first := m.HashRefresh(plain)
	second := m.HashRefresh(plain)
	if first != second {
		t.Error("hash must be deterministic")
	}
	if first == plain {
		t.Error("stored hash must not equal the plaintext")
	}
}

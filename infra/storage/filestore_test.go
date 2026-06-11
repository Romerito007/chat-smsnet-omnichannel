package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
)

func newStore(t *testing.T) *LocalFileStore {
	t.Helper()
	return NewLocalFileStore(t.TempDir(), "test-secret", "http://localhost:8080")
}

func TestSaveOpenRoundTrip(t *testing.T) {
	s := newStore(t)
	key := "exports/t1/c1/r1.json"
	if err := s.Save(key, []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("save: %v", err)
	}
	data, name, err := s.Open(key)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Errorf("data mismatch: %s", data)
	}
	if name != "r1.json" {
		t.Errorf("filename = %s", name)
	}
}

func TestSignedURLResolves(t *testing.T) {
	s := newStore(t)
	key := "exports/t1/c1/r1.json"
	url, exp, err := s.SignedURL(key, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if exp.Before(time.Now()) {
		t.Errorf("expiry in the past")
	}
	token := filepath.Base(url)
	got, err := s.Resolve(token)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != key {
		t.Errorf("resolved key = %q, want %q", got, key)
	}
}

func TestResolveRejectsTamperedToken(t *testing.T) {
	s := newStore(t)
	url, _, _ := s.SignedURL("exports/t1/c1/r1.json", time.Hour)
	token := filepath.Base(url) + "x" // tamper with the signature
	if _, err := s.Resolve(token); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("tampered token must be forbidden, got %v", err)
	}
}

func TestResolveRejectsExpiredToken(t *testing.T) {
	s := newStore(t)
	url, _, _ := s.SignedURL("exports/t1/c1/r1.json", -time.Minute) // already expired
	token := filepath.Base(url)
	if _, err := s.Resolve(token); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expired token must be forbidden, got %v", err)
	}
}

func TestSignedBySecretMismatch(t *testing.T) {
	a := newStore(t)
	url, _, _ := a.SignedURL("exports/t1/c1/r1.json", time.Hour)
	token := filepath.Base(url)
	other := NewLocalFileStore(t.TempDir(), "different-secret", "http://localhost:8080")
	if _, err := other.Resolve(token); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("token signed by another secret must be forbidden, got %v", err)
	}
}

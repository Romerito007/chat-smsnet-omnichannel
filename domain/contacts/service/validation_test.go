package service

import (
	"math/rand"
	"testing"
)

// GenerateValidCPF must always produce a document the create/update validation
// accepts (same NormalizeDocument), stored digits-only.
func TestGenerateValidCPF_AlwaysValid(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 5000; i++ {
		cpf := GenerateValidCPF(rng)
		if len(cpf) != 11 {
			t.Fatalf("CPF %q is not 11 digits", cpf)
		}
		got, ok := NormalizeDocument(cpf)
		if !ok || got != cpf {
			t.Fatalf("generated CPF %q rejected by NormalizeDocument (ok=%v got=%q)", cpf, ok, got)
		}
	}
}

// The seed's phone shape (+55 + real DDD + 9 + 8 digits) is accepted and returned
// as E.164 — so a seeded phone is exactly what an edit would accept.
func TestSeedPhoneShape_NormalizesToE164(t *testing.T) {
	for _, raw := range []string{"+5544912345678", "+5511987654321", "+5521991112222"} {
		e164, ok := NormalizePhoneE164(raw)
		if !ok {
			t.Errorf("expected %q to be a valid BR mobile", raw)
		}
		if e164 != raw {
			t.Errorf("expected idempotent E.164 for %q, got %q", raw, e164)
		}
	}
	// A number without DDI/invalid length must be rejected (the old seed bug).
	if _, ok := NormalizePhoneE164("64503181111"); ok {
		t.Errorf("expected the DDI-less 64503181111 to be rejected")
	}
}

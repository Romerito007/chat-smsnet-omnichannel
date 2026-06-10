// Package security implements the auth/iam security ports: password hashing
// (bcrypt) and token management (JWT access + opaque refresh).
package security

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam"
)

// BcryptHasher implements iam.PasswordHasher using bcrypt.
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher builds a hasher. A cost <= 0 falls back to a safe default.
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = 12
	}
	return &BcryptHasher{cost: cost}
}

// Hash returns the bcrypt hash of plain.
func (h *BcryptHasher) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Compare returns nil when plain matches the stored hash.
func (h *BcryptHasher) Compare(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}

// Ensure the interface is satisfied at compile time.
var _ iam.PasswordHasher = (*BcryptHasher)(nil)

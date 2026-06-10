// Package iam declares cross-cutting ports owned by the IAM domain. The user and
// role aggregates live in the entity/ subpackage; this root file holds the
// password-hashing port, implemented in infra/security and consumed by both the
// iam and auth services.
package iam

// PasswordHasher hashes and verifies user passwords. The implementation
// (infra/security) chooses the algorithm and cost.
type PasswordHasher interface {
	// Hash returns a self-describing hash of the plaintext password.
	Hash(plain string) (string, error)
	// Compare returns nil when plain matches hash, or an error otherwise.
	Compare(hash, plain string) error
}

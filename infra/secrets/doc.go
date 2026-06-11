// Package secrets handles encryption/decryption of sensitive values (channel
// credentials, webhook signing keys) before they are persisted. The plaintext
// never leaves this package's boundary. The AES-256-GCM cipher is in cipher.go.
package secrets

package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/contracts"
)

// LocalFileStore is a filesystem-backed contracts.FileStore. It writes objects
// under a base directory and mints HMAC-signed, expiring download URLs validated
// by the public download endpoint. An object-store backend can replace it
// without touching the consuming domain.
type LocalFileStore struct {
	baseDir      string
	secret       []byte
	baseURL      string // e.g. "http://localhost:8080" — download path is appended
	downloadPath string // e.g. "/v1/privacy/downloads/" — the signed token is appended
}

// NewLocalFileStore builds the store for the privacy download endpoint. baseDir
// is created on demand; secret signs download tokens; baseURL is the public API
// origin the signed link points at.
func NewLocalFileStore(baseDir, secret, baseURL string) *LocalFileStore {
	return NewLocalFileStoreAt(baseDir, secret, baseURL, "/v1/privacy/downloads/")
}

// NewLocalFileStoreAt builds the store with an explicit download path, so the
// same backend can serve different domains (e.g. /v1/reports/downloads/).
func NewLocalFileStoreAt(baseDir, secret, baseURL, downloadPath string) *LocalFileStore {
	if !strings.HasSuffix(downloadPath, "/") {
		downloadPath += "/"
	}
	return &LocalFileStore{
		baseDir:      baseDir,
		secret:       []byte(secret),
		baseURL:      strings.TrimRight(baseURL, "/"),
		downloadPath: downloadPath,
	}
}

// Save writes data under key, creating parent directories as needed.
func (s *LocalFileStore) Save(key string, data []byte) error {
	path, err := s.pathFor(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o640)
}

// Open returns the stored bytes and a suggested download filename for key.
func (s *LocalFileStore) Open(key string) ([]byte, string, error) {
	path, err := s.pathFor(key)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", apperror.NotFound("export file not found")
		}
		return nil, "", err
	}
	return data, filepath.Base(key), nil
}

// SignedURL builds a temporary download URL for key valid for ttl.
func (s *LocalFileStore) SignedURL(key string, ttl time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(ttl).UTC()
	token := s.sign(key, expiresAt)
	return s.baseURL + s.downloadPath + token, expiresAt, nil
}

// Resolve validates a signed token and returns the object key. It fails for
// tampered or expired tokens.
func (s *LocalFileStore) Resolve(token string) (string, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", apperror.Forbidden("invalid download token")
	}
	payloadB64, sig := parts[0], parts[1]
	if !hmac.Equal([]byte(sig), []byte(s.hmacHex(payloadB64))) {
		return "", apperror.Forbidden("invalid download token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return "", apperror.Forbidden("invalid download token")
	}
	// payload = "<expiryUnixMillis>:<key>"
	payload := string(raw)
	idx := strings.IndexByte(payload, ':')
	if idx < 0 {
		return "", apperror.Forbidden("invalid download token")
	}
	expMillis, err := strconv.ParseInt(payload[:idx], 10, 64)
	if err != nil {
		return "", apperror.Forbidden("invalid download token")
	}
	if time.Now().After(time.UnixMilli(expMillis)) {
		return "", apperror.Forbidden("download link has expired")
	}
	return payload[idx+1:], nil
}

func (s *LocalFileStore) sign(key string, expiresAt time.Time) string {
	payload := fmt.Sprintf("%d:%s", expiresAt.UnixMilli(), key)
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return payloadB64 + "." + s.hmacHex(payloadB64)
}

func (s *LocalFileStore) hmacHex(payloadB64 string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payloadB64))
	return hex.EncodeToString(mac.Sum(nil))
}

// pathFor maps an object key to a filesystem path, rejecting traversal.
func (s *LocalFileStore) pathFor(key string) (string, error) {
	clean := filepath.Clean("/" + key) // anchor so ".." cannot escape
	path := filepath.Join(s.baseDir, clean)
	if !strings.HasPrefix(path, filepath.Clean(s.baseDir)+string(os.PathSeparator)) {
		return "", apperror.Validation("invalid storage key")
	}
	return path, nil
}

var _ contracts.FileStore = (*LocalFileStore)(nil)

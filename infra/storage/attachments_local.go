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
	"github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/contracts"
)

// LocalAttachmentStorage is a filesystem-backed attachments backend. Direct
// "upload to storage" is modelled as an HMAC-signed PUT to our own blob endpoint
// (PUT /v1/attachments/blobs/{token}); downloads are streamed by the API after
// the conversation-access check.
type LocalAttachmentStorage struct {
	baseDir string
	secret  []byte
	baseURL string
}

// NewLocalAttachmentStorage builds the backend.
func NewLocalAttachmentStorage(baseDir, secret, baseURL string) *LocalAttachmentStorage {
	return &LocalAttachmentStorage{baseDir: baseDir, secret: []byte(secret), baseURL: strings.TrimRight(baseURL, "/")}
}

// Provider implements contracts.Storage.
func (s *LocalAttachmentStorage) Provider() string { return "local" }

// SignUpload returns a PUT target pointing at our blob endpoint, with a signed
// token binding the key, content-type and size.
func (s *LocalAttachmentStorage) SignUpload(key, contentType string, size int64, ttl time.Duration) (contracts.UploadTarget, error) {
	expiresAt := time.Now().Add(ttl).UTC()
	token := s.signUpload(key, contentType, size, expiresAt)
	return contracts.UploadTarget{
		URL:       fmt.Sprintf("%s/v1/attachments/blobs/%s", s.baseURL, token),
		Method:    "PUT",
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: expiresAt,
	}, nil
}

// Download reads the stored bytes for serving by the API.
func (s *LocalAttachmentStorage) Download(key, filename string, _ time.Duration) (contracts.DownloadResult, error) {
	data, err := s.read(key)
	if err != nil {
		return contracts.DownloadResult{}, err
	}
	return contracts.DownloadResult{Data: data, Filename: filename}, nil
}

// Put stores bytes under key.
func (s *LocalAttachmentStorage) Put(key, _ string, data []byte) error {
	path, err := s.pathFor(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o640)
}

// ResolveUpload validates a signed upload token and returns the bound key,
// content-type and max size. Used by the blob PUT endpoint (local only).
func (s *LocalAttachmentStorage) ResolveUpload(token string) (key, contentType string, maxSize int64, err error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", "", 0, apperror.Forbidden("invalid upload token")
	}
	payloadB64, sig := parts[0], parts[1]
	if !hmac.Equal([]byte(sig), []byte(s.hmacHex(payloadB64))) {
		return "", "", 0, apperror.Forbidden("invalid upload token")
	}
	raw, derr := base64.RawURLEncoding.DecodeString(payloadB64)
	if derr != nil {
		return "", "", 0, apperror.Forbidden("invalid upload token")
	}
	// payload = "<expMillis>|<size>|<contentType>|<key>"
	fields := strings.SplitN(string(raw), "|", 4)
	if len(fields) != 4 {
		return "", "", 0, apperror.Forbidden("invalid upload token")
	}
	expMillis, perr := strconv.ParseInt(fields[0], 10, 64)
	if perr != nil {
		return "", "", 0, apperror.Forbidden("invalid upload token")
	}
	if time.Now().After(time.UnixMilli(expMillis)) {
		return "", "", 0, apperror.Forbidden("upload link has expired")
	}
	size, _ := strconv.ParseInt(fields[1], 10, 64)
	return fields[3], fields[2], size, nil
}

func (s *LocalAttachmentStorage) signUpload(key, contentType string, size int64, expiresAt time.Time) string {
	payload := fmt.Sprintf("%d|%d|%s|%s", expiresAt.UnixMilli(), size, contentType, key)
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return payloadB64 + "." + s.hmacHex(payloadB64)
}

func (s *LocalAttachmentStorage) hmacHex(payloadB64 string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payloadB64))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *LocalAttachmentStorage) read(key string) ([]byte, error) {
	path, err := s.pathFor(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperror.NotFound("attachment file not found")
		}
		return nil, err
	}
	return data, nil
}

// pathFor maps an object key to a filesystem path, rejecting traversal.
func (s *LocalAttachmentStorage) pathFor(key string) (string, error) {
	clean := filepath.Clean("/" + key)
	path := filepath.Join(s.baseDir, clean)
	if !strings.HasPrefix(path, filepath.Clean(s.baseDir)+string(os.PathSeparator)) {
		return "", apperror.Validation("invalid storage key")
	}
	return path, nil
}

var _ contracts.Storage = (*LocalAttachmentStorage)(nil)

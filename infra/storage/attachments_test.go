package storage

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
)

func TestLocalAttachment_UploadTokenRoundTrip(t *testing.T) {
	s := NewLocalAttachmentStorage(t.TempDir(), "secret", "http://localhost:8080")
	key := "attachments/t1/cv1/a1/pic.png"
	target, err := s.SignUpload(key, "image/png", 1234, 10*time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if target.Method != "PUT" || !strings.Contains(target.URL, "/v1/attachments/blobs/") {
		t.Fatalf("bad target: %+v", target)
	}
	token := target.URL[strings.LastIndex(target.URL, "/")+1:]
	gotKey, ct, size, err := s.ResolveUpload(token)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if gotKey != key || ct != "image/png" || size != 1234 {
		t.Errorf("resolved mismatch: %q %q %d", gotKey, ct, size)
	}
}

func TestLocalAttachment_PutDownloadRoundTrip(t *testing.T) {
	s := NewLocalAttachmentStorage(t.TempDir(), "secret", "http://localhost:8080")
	key := "attachments/t1/cv1/a1/doc.txt"
	if err := s.Put(key, "text/plain", []byte("hello")); err != nil {
		t.Fatalf("put: %v", err)
	}
	res, err := s.Download(key, "doc.txt", time.Minute)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if string(res.Data) != "hello" || res.RedirectURL != "" {
		t.Errorf("unexpected download: %+v", res)
	}
}

func TestLocalAttachment_RejectsTamperedToken(t *testing.T) {
	s := NewLocalAttachmentStorage(t.TempDir(), "secret", "http://localhost:8080")
	target, _ := s.SignUpload("attachments/t1/cv1/a1/p.png", "image/png", 1, time.Minute)
	token := target.URL[strings.LastIndex(target.URL, "/")+1:] + "x"
	if _, _, _, err := s.ResolveUpload(token); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("tampered token must be forbidden, got %v", err)
	}
}

func TestLocalAttachment_TraversalContained(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalAttachmentStorage(dir, "secret", "http://localhost:8080")
	// A traversal key is anchored and collapsed so it cannot escape the base dir.
	if err := s.Put("../../etc/passwd", "text/plain", []byte("x")); err != nil {
		t.Fatalf("put: %v", err)
	}
	// The bytes are reachable only via the same (contained) key, never outside.
	res, err := s.Download("../../etc/passwd", "passwd", time.Minute)
	if err != nil || string(res.Data) != "x" {
		t.Fatalf("contained read failed: %v %q", err, res.Data)
	}
	p, err := s.pathFor("../../etc/passwd")
	if err != nil {
		t.Fatalf("pathFor: %v", err)
	}
	if !strings.HasPrefix(p, dir) {
		t.Errorf("resolved path escaped base dir: %s", p)
	}
}

func TestS3Attachment_PresignsUploadAndDownload(t *testing.T) {
	s, err := NewS3AttachmentStorage(S3Config{
		Endpoint: "https://s3.eu-west-1.amazonaws.com", Region: "eu-west-1",
		Bucket: "bucket", AccessKey: "AKID", SecretKey: "secret",
	})
	if err != nil {
		t.Fatalf("new s3: %v", err)
	}
	target, err := s.SignUpload("attachments/t1/cv1/a1/p.png", "image/png", 10, 5*time.Minute)
	if err != nil {
		t.Fatalf("sign upload: %v", err)
	}
	u, err := url.Parse(target.URL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := u.Query()
	for _, p := range []string{"X-Amz-Algorithm", "X-Amz-Credential", "X-Amz-Date", "X-Amz-Expires", "X-Amz-SignedHeaders", "X-Amz-Signature"} {
		if q.Get(p) == "" {
			t.Errorf("presigned upload missing %s", p)
		}
	}
	if !strings.HasPrefix(u.Path, "/bucket/") {
		t.Errorf("path-style bucket missing: %s", u.Path)
	}

	res, err := s.Download("attachments/t1/cv1/a1/p.png", "p.png", time.Minute)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if res.RedirectURL == "" || !strings.Contains(res.RedirectURL, "X-Amz-Signature=") {
		t.Errorf("expected presigned redirect, got %+v", res)
	}
}

func TestS3Attachment_DeterministicSignature(t *testing.T) {
	cfg := S3Config{Endpoint: "https://s3.amazonaws.com", Region: "us-east-1", Bucket: "b", AccessKey: "AKID", SecretKey: "secret"}
	a, _ := NewS3AttachmentStorage(cfg)
	b, _ := NewS3AttachmentStorage(cfg)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if a.presign("GET", "k/obj.txt", time.Minute, now) != b.presign("GET", "k/obj.txt", time.Minute, now) {
		t.Errorf("presign must be deterministic for identical inputs")
	}
}

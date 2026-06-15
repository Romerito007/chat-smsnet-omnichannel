package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestS3Attachment_PresignShapes(t *testing.T) {
	s, err := NewS3AttachmentStorage(S3Config{
		Endpoint: "https://s3.eu-west-1.amazonaws.com", Region: "eu-west-1",
		Bucket: "bucket", AccessKey: "AKID", SecretKey: "secret",
		ForcePathStyle: true, PresignExpiry: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("new s3: %v", err)
	}
	target, err := s.SignUpload("attachments/t1/cv1/a1/p.png", "image/png", 10, 5*time.Minute)
	if err != nil {
		t.Fatalf("sign upload: %v", err)
	}
	if target.Method != "PUT" || target.Headers["Content-Type"] != "image/png" {
		t.Errorf("upload target should be a PUT carrying the signed Content-Type: %+v", target)
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

// mockS3 is a tiny in-memory, path-style S3 emulator for the end-to-end test. It
// ignores the signature (the SDK still signs every request).
func TestS3Attachment_EnsureCORS(t *testing.T) {
	var gotMethod, gotQuery, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotQuery = r.URL.RawQuery
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s, err := NewS3AttachmentStorage(S3Config{
		Endpoint: srv.URL, Region: "us-east-1", Bucket: "bucket",
		AccessKey: "test", SecretKey: "test", ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("new s3: %v", err)
	}

	// Empty origins is a no-op (no request issued).
	if err := s.EnsureCORS(context.Background(), nil); err != nil {
		t.Fatalf("empty origins should be a no-op, got %v", err)
	}
	if gotMethod != "" {
		t.Fatalf("empty origins must not call the bucket, got %s", gotMethod)
	}

	// With origins it issues a PutBucketCors carrying the policy.
	if err := s.EnsureCORS(context.Background(), []string{"http://localhost:3000"}); err != nil {
		t.Fatalf("ensure cors: %v", err)
	}
	if gotMethod != http.MethodPut || !strings.Contains(gotQuery, "cors") {
		t.Errorf("expected PUT ...?cors, got %s ?%s", gotMethod, gotQuery)
	}
	for _, want := range []string{"http://localhost:3000", "<AllowedMethod>PUT</AllowedMethod>", "ETag"} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("cors body missing %q; body=%s", want, gotBody)
		}
	}
}

func mockS3(t *testing.T) (*httptest.Server, map[string][]byte) {
	t.Helper()
	objects := map[string][]byte{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path // "/bucket/key..."
		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			objects[key] = body
			w.WriteHeader(http.StatusOK)
		case http.MethodHead:
			if _, ok := objects[key]; ok {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case http.MethodGet:
			if data, ok := objects[key]; ok {
				_, _ = w.Write(data)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, objects
}

// End-to-end against a mock S3: presigned PUT → Exists (HEAD) → presigned GET.
func TestS3Attachment_EndToEnd_MockServer(t *testing.T) {
	srv, _ := mockS3(t)
	s, err := NewS3AttachmentStorage(S3Config{
		Endpoint: srv.URL, Region: "us-east-1", Bucket: "bucket",
		AccessKey: "test", SecretKey: "test", ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("new s3: %v", err)
	}
	key := "attachments/t1/cv1/a1/doc.txt"

	// Not uploaded yet.
	if ok, err := s.Exists(key); err != nil || ok {
		t.Fatalf("exists before upload = %v, %v; want false", ok, err)
	}

	// Presign + upload the bytes directly to the (mock) bucket.
	target, err := s.SignUpload(key, "text/plain", 5, time.Minute)
	if err != nil {
		t.Fatalf("sign upload: %v", err)
	}
	req, _ := http.NewRequest(target.Method, target.URL, strings.NewReader("hello"))
	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload PUT: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}

	// Confirm-time existence check now passes.
	if ok, err := s.Exists(key); err != nil || !ok {
		t.Fatalf("exists after upload = %v, %v; want true", ok, err)
	}

	// Download presigns a GET; following it returns the bytes.
	dl, err := s.Download(key, "doc.txt", time.Minute)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if dl.RedirectURL == "" {
		t.Fatal("expected a presigned redirect URL")
	}
	getResp, err := http.Get(dl.RedirectURL)
	if err != nil {
		t.Fatalf("download GET: %v", err)
	}
	defer func() { _ = getResp.Body.Close() }()
	got, _ := io.ReadAll(getResp.Body)
	if string(got) != "hello" {
		t.Errorf("downloaded %q, want %q", got, "hello")
	}
}

// TestS3Attachment_PutUploadsServerSide is the regression for the inbound media 500:
// the S3 backend's Put used to be a stub that always errored ("direct put is not
// supported"), so every inbound multipart attachment (gateway → API → S3) failed
// with "could not store inbound attachment". Put must now upload the bytes
// server-side (there is no client to presign for on the inbound rail).
func TestS3Attachment_PutUploadsServerSide(t *testing.T) {
	srv, objects := mockS3(t)
	s, err := NewS3AttachmentStorage(S3Config{
		Endpoint: srv.URL, Region: "us-east-1", Bucket: "bucket",
		AccessKey: "test", SecretKey: "test", ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("new s3: %v", err)
	}
	key := "attachments/t1/cv1/a1/photo.jpg"
	if err := s.Put(key, "image/jpeg", []byte("\xff\xd8\xff\xe0jpegbytes")); err != nil {
		t.Fatalf("Put must upload server-side, got: %v", err)
	}
	if got, ok := objects["/bucket/"+key]; !ok || string(got) != "\xff\xd8\xff\xe0jpegbytes" {
		t.Fatalf("object not stored in the bucket: ok=%v bytes=%q", ok, got)
	}
	// The object is now visible to the confirm-time existence check.
	if ok, err := s.Exists(key); err != nil || !ok {
		t.Fatalf("exists after Put = %v, %v; want true", ok, err)
	}
}

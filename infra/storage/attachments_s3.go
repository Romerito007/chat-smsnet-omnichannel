package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/contracts"
)

// S3Config configures the S3-compatible backend (AWS S3, MinIO, etc.).
type S3Config struct {
	Endpoint  string // e.g. https://s3.eu-west-1.amazonaws.com or http://minio:9000
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

// S3AttachmentStorage is an S3-compatible attachments backend. Uploads and
// downloads use AWS Signature V4 presigned URLs (path-style, works with AWS S3
// and MinIO), so bytes flow directly between the client and the object store.
// The implementation is self-contained (no AWS SDK dependency).
type S3AttachmentStorage struct {
	cfg  S3Config
	host string
}

// NewS3AttachmentStorage builds the backend.
func NewS3AttachmentStorage(cfg S3Config) (*S3AttachmentStorage, error) {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid S3 endpoint %q", cfg.Endpoint)
	}
	return &S3AttachmentStorage{cfg: cfg, host: u.Host}, nil
}

// Provider implements contracts.Storage.
func (s *S3AttachmentStorage) Provider() string { return "s3" }

// SignUpload returns a presigned PUT URL the client uploads the bytes to.
func (s *S3AttachmentStorage) SignUpload(key, contentType string, _ int64, ttl time.Duration) (contracts.UploadTarget, error) {
	signed := s.presign("PUT", key, ttl, time.Now().UTC())
	headers := map[string]string{}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	return contracts.UploadTarget{
		URL:       signed,
		Method:    "PUT",
		Headers:   headers,
		ExpiresAt: time.Now().Add(ttl).UTC(),
	}, nil
}

// Download returns a short-lived presigned GET URL for the client to redirect to.
func (s *S3AttachmentStorage) Download(key, _ string, ttl time.Duration) (contracts.DownloadResult, error) {
	return contracts.DownloadResult{RedirectURL: s.presign("GET", key, ttl, time.Now().UTC())}, nil
}

// Put is not supported: S3 uploads go directly from the client to the bucket.
func (s *S3AttachmentStorage) Put(string, string, []byte) error {
	return apperror.Internal("direct put is not supported by the s3 backend")
}

// presign builds an AWS SigV4 query-string-authenticated URL (path-style).
func (s *S3AttachmentStorage) presign(method, key string, ttl time.Duration, now time.Time) string {
	const algorithm = "AWS4-HMAC-SHA256"
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	scope := strings.Join([]string{dateStamp, s.cfg.Region, "s3", "aws4_request"}, "/")

	// Path-style canonical URI: /bucket/key (each segment URL-encoded, '/' kept).
	canonicalURI := "/" + s.cfg.Bucket + "/" + encodePath(key)

	q := url.Values{}
	q.Set("X-Amz-Algorithm", algorithm)
	q.Set("X-Amz-Credential", s.cfg.AccessKey+"/"+scope)
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", strconv.Itoa(int(ttl.Seconds())))
	q.Set("X-Amz-SignedHeaders", "host")
	canonicalQuery := encodeQuery(q)

	canonicalHeaders := "host:" + s.host + "\n"
	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		"host",
		"UNSIGNED-PAYLOAD",
	}, "\n")

	stringToSign := strings.Join([]string{
		algorithm,
		amzDate,
		scope,
		hexSHA256([]byte(canonicalRequest)),
	}, "\n")

	signature := hex.EncodeToString(hmacSHA256(s.signingKey(dateStamp), []byte(stringToSign)))
	return s.cfg.Endpoint + canonicalURI + "?" + canonicalQuery + "&X-Amz-Signature=" + signature
}

func (s *S3AttachmentStorage) signingKey(dateStamp string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+s.cfg.SecretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(s.cfg.Region))
	kService := hmacSHA256(kRegion, []byte("s3"))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

func hexSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// encodePath URI-encodes each path segment per AWS rules, preserving '/'.
func encodePath(key string) string {
	segments := strings.Split(key, "/")
	for i, seg := range segments {
		segments[i] = awsURIEncode(seg, false)
	}
	return strings.Join(segments, "/")
}

// encodeQuery renders the query in the canonical, sorted, AWS-encoded form.
func encodeQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, awsURIEncode(k, true)+"="+awsURIEncode(q.Get(k), true))
	}
	return strings.Join(parts, "&")
}

// awsURIEncode encodes per RFC 3986 as AWS SigV4 requires (unreserved chars are
// left as-is; everything else is %-encoded). When encodeSlash is false, '/' is
// preserved (for path segments handled separately).
func awsURIEncode(s string, encodeSlash bool) string {
	const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.IndexByte(unreserved, c) >= 0 {
			b.WriteByte(c)
		} else if c == '/' && !encodeSlash {
			b.WriteByte(c)
		} else {
			b.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return b.String()
}

var _ contracts.Storage = (*S3AttachmentStorage)(nil)

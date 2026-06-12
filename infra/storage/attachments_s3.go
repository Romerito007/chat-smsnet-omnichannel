package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/contracts"
)

// S3Config configures the S3-compatible backend (AWS S3, MinIO, Cloudflare R2).
// When AccessKey/SecretKey are empty the AWS default credential chain is used
// (env, shared config, or an IAM role on EC2/ECS) — credentials are never logged.
type S3Config struct {
	Endpoint       string // optional, for S3-compatible stores (MinIO/R2); empty = AWS
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool          // path-style addressing (required by MinIO and most R2 setups)
	PresignExpiry  time.Duration // upper bound on a presigned URL's lifetime
}

// S3AttachmentStorage is an S3-compatible attachments backend built on the AWS
// SDK for Go v2. Uploads and downloads use presigned URLs (SigV4), so bytes flow
// directly between the client and the object store; the API never proxies them.
type S3AttachmentStorage struct {
	bucket  string
	client  *s3.Client
	presign *s3.PresignClient
	expiry  time.Duration
}

// NewS3AttachmentStorage builds the backend.
func NewS3AttachmentStorage(cfg S3Config) (*S3AttachmentStorage, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	// Static credentials only when both are provided; otherwise the SDK's default
	// chain (env / shared config / IAM role) resolves them.
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
	})

	expiry := cfg.PresignExpiry
	if expiry <= 0 {
		expiry = 5 * time.Minute
	}
	return &S3AttachmentStorage{
		bucket:  cfg.Bucket,
		client:  client,
		presign: s3.NewPresignClient(client),
		expiry:  expiry,
	}, nil
}

// Provider implements contracts.Storage.
func (s *S3AttachmentStorage) Provider() string { return "s3" }

// SignUpload returns a presigned PUT URL the client uploads the bytes to. The
// returned headers (e.g. Content-Type) MUST be replayed by the client, since they
// are part of the signature.
func (s *S3AttachmentStorage) SignUpload(key, contentType string, _ int64, ttl time.Duration) (contracts.UploadTarget, error) {
	exp := s.effective(ttl)
	in := &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	req, err := s.presign.PresignPutObject(context.Background(), in, s3.WithPresignExpires(exp))
	if err != nil {
		return contracts.UploadTarget{}, apperror.Integration("could not presign upload").Wrap(err)
	}
	headers := map[string]string{}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	return contracts.UploadTarget{
		URL:       req.URL,
		Method:    req.Method,
		Headers:   headers,
		ExpiresAt: time.Now().Add(exp).UTC(),
	}, nil
}

// Download returns a short-lived presigned GET URL for the client to redirect to.
// The filename drives a Content-Disposition so browsers download with the right
// name.
func (s *S3AttachmentStorage) Download(key, filename string, ttl time.Duration) (contracts.DownloadResult, error) {
	exp := s.effective(ttl)
	in := &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}
	if filename != "" {
		in.ResponseContentDisposition = aws.String(`attachment; filename="` + filename + `"`)
	}
	req, err := s.presign.PresignGetObject(context.Background(), in, s3.WithPresignExpires(exp))
	if err != nil {
		return contracts.DownloadResult{}, apperror.Integration("could not presign download").Wrap(err)
	}
	return contracts.DownloadResult{RedirectURL: req.URL}, nil
}

// Put is not supported: S3 uploads go directly from the client to the bucket.
func (s *S3AttachmentStorage) Put(string, string, []byte) error {
	return apperror.Internal("direct put is not supported by the s3 backend")
}

// Exists reports whether the object was actually uploaded (HeadObject). A missing
// object is (false, nil); any other error propagates.
func (s *S3AttachmentStorage) Exists(key string) (bool, error) {
	_, err := s.client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, err
}

// isNotFound recognizes S3's "object missing" responses across SDK error shapes.
func isNotFound(err error) bool {
	var nf *s3types.NotFound
	var nsk *s3types.NoSuchKey
	if errors.As(err, &nf) || errors.As(err, &nsk) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return true
		}
	}
	// HeadObject 404 may surface only as a transport-level response error.
	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) && respErr.HTTPStatusCode() == 404 {
		return true
	}
	return false
}

// effective bounds the presign lifetime by the configured PresignExpiry so a URL
// never lives longer than allowed.
func (s *S3AttachmentStorage) effective(ttl time.Duration) time.Duration {
	exp := ttl
	if exp <= 0 || (s.expiry > 0 && s.expiry < exp) {
		exp = s.expiry
	}
	if exp <= 0 {
		exp = 5 * time.Minute
	}
	return exp
}

var _ contracts.Storage = (*S3AttachmentStorage)(nil)

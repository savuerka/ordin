package framework

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Storage is the small ORDIN abstraction for object/file storage.
//
// The default implementation is S3-compatible, so the same interface works with
// MinIO, SeaweedFS S3, AWS S3 and similar backends.
type Storage interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, options ...PutOption) error
	PutBytes(ctx context.Context, key string, data []byte, options ...PutOption) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	GetBytes(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	URL(ctx context.Context, key string, expiry time.Duration) (string, error)
}

// PutOptions configures uploaded object metadata.
type PutOptions struct {
	ContentType  string
	CacheControl string
	Metadata     map[string]string
}

// PutOption updates PutOptions.
type PutOption func(*PutOptions)

// WithContentType sets the object content type.
func WithContentType(contentType string) PutOption {
	return func(options *PutOptions) {
		options.ContentType = strings.TrimSpace(contentType)
	}
}

// WithCacheControl sets the object Cache-Control metadata.
func WithCacheControl(value string) PutOption {
	return func(options *PutOptions) {
		options.CacheControl = strings.TrimSpace(value)
	}
}

// WithObjectMetadata adds user metadata to the object.
func WithObjectMetadata(metadata map[string]string) PutOption {
	return func(options *PutOptions) {
		if options.Metadata == nil {
			options.Metadata = map[string]string{}
		}
		for key, value := range metadata {
			options.Metadata[key] = value
		}
	}
}

// S3Config configures an S3-compatible storage backend.
type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Bucket          string
	Region          string
	Secure          bool
	CreateBucket    bool
}

// S3ConfigFromEnv reads an S3-compatible configuration from environment.
//
// For prefix "S3", it reads S3_ENDPOINT, S3_ACCESS_KEY_ID,
// S3_SECRET_ACCESS_KEY, S3_SESSION_TOKEN, S3_BUCKET, S3_REGION, S3_SECURE and
// S3_CREATE_BUCKET.
func S3ConfigFromEnv(prefix string) S3Config {
	prefix = strings.Trim(strings.ToUpper(prefix), "_")
	if prefix == "" {
		prefix = "S3"
	}
	key := func(name string) string { return prefix + "_" + name }

	return S3Config{
		Endpoint:        getenv(key("ENDPOINT"), "localhost:9000"),
		AccessKeyID:     getenv(key("ACCESS_KEY_ID"), getenv(key("ACCESS_KEY"), "minioadmin")),
		SecretAccessKey: getenv(key("SECRET_ACCESS_KEY"), getenv(key("SECRET_KEY"), "minioadmin")),
		SessionToken:    getenv(key("SESSION_TOKEN"), ""),
		Bucket:          getenv(key("BUCKET"), "ordin"),
		Region:          getenv(key("REGION"), "us-east-1"),
		Secure:          getenvBool(key("SECURE"), false),
		CreateBucket:    getenvBool(key("CREATE_BUCKET"), true),
	}
}

// S3Storage is the default ORDIN Storage implementation.
type S3Storage struct {
	client *minio.Client
	bucket string
}

// NewS3Storage creates an S3-compatible storage backend.
func NewS3Storage(config S3Config) (*S3Storage, error) {
	config.Endpoint = normalizeS3Endpoint(config.Endpoint)
	if config.Endpoint == "" {
		return nil, errors.New("s3 endpoint is empty")
	}
	if strings.TrimSpace(config.Bucket) == "" {
		return nil, errors.New("s3 bucket is empty")
	}

	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, config.SessionToken),
		Secure: config.Secure,
		Region: config.Region,
	})
	if err != nil {
		return nil, err
	}

	storage := &S3Storage{client: client, bucket: strings.TrimSpace(config.Bucket)}
	if config.CreateBucket {
		if err := storage.EnsureBucket(context.Background(), config.Region); err != nil {
			return nil, err
		}
	}

	return storage, nil
}

// MustS3Storage creates an S3 storage or panics with a readable message.
func MustS3Storage(config S3Config) *S3Storage {
	storage, err := NewS3Storage(config)
	if err != nil {
		panic(err)
	}
	return storage
}

// Client exposes the underlying MinIO client for advanced S3 operations.
func (s *S3Storage) Client() *minio.Client {
	return s.client
}

// Bucket returns the configured bucket name.
func (s *S3Storage) Bucket() string {
	return s.bucket
}

// EnsureBucket creates the bucket when it does not exist.
func (s *S3Storage) EnsureBucket(ctx context.Context, region string) error {
	if s == nil || s.client == nil {
		return errors.New("s3 storage is not configured")
	}

	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{Region: region})
}

// Put uploads an object.
func (s *S3Storage) Put(ctx context.Context, key string, body io.Reader, size int64, options ...PutOption) error {
	if s == nil || s.client == nil {
		return errors.New("s3 storage is not configured")
	}
	key = cleanObjectKey(key)
	if key == "" {
		return errors.New("object key is empty")
	}
	if body == nil {
		return errors.New("object body is nil")
	}

	put := PutOptions{ContentType: detectContentType(key)}
	for _, option := range options {
		if option != nil {
			option(&put)
		}
	}

	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{
		ContentType:  put.ContentType,
		CacheControl: put.CacheControl,
		UserMetadata: put.Metadata,
	})
	return err
}

// PutBytes uploads a byte slice.
func (s *S3Storage) PutBytes(ctx context.Context, key string, data []byte, options ...PutOption) error {
	return s.Put(ctx, key, bytes.NewReader(data), int64(len(data)), options...)
}

// Get opens an object for streaming.
func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("s3 storage is not configured")
	}
	key = cleanObjectKey(key)
	if key == "" {
		return nil, errors.New("object key is empty")
	}
	return s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
}

// GetBytes downloads an object into memory.
func (s *S3Storage) GetBytes(ctx context.Context, key string) ([]byte, error) {
	reader, err := s.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// Delete removes an object.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	if s == nil || s.client == nil {
		return errors.New("s3 storage is not configured")
	}
	key = cleanObjectKey(key)
	if key == "" {
		return errors.New("object key is empty")
	}
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

// Exists checks if an object is available.
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	if s == nil || s.client == nil {
		return false, errors.New("s3 storage is not configured")
	}
	key = cleanObjectKey(key)
	if key == "" {
		return false, errors.New("object key is empty")
	}
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}
	response := minio.ToErrorResponse(err)
	if response.Code == "NoSuchKey" || response.StatusCode == 404 {
		return false, nil
	}
	return false, err
}

// URL returns a temporary presigned GET URL.
func (s *S3Storage) URL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if s == nil || s.client == nil {
		return "", errors.New("s3 storage is not configured")
	}
	key = cleanObjectKey(key)
	if key == "" {
		return "", errors.New("object key is empty")
	}
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	presigned, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
	if err != nil {
		return "", err
	}
	return presigned.String(), nil
}

func cleanObjectKey(key string) string {
	key = strings.TrimSpace(strings.ReplaceAll(key, "\\", "/"))
	key = strings.TrimPrefix(key, "/")
	clean := filepath.ToSlash(filepath.Clean(key))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return ""
	}
	return clean
}

func detectContentType(key string) string {
	if ext := filepath.Ext(key); ext != "" {
		if contentType := mime.TypeByExtension(ext); contentType != "" {
			return contentType
		}
	}
	return "application/octet-stream"
}

func normalizeS3Endpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://"), "/")
}

func getenvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(getenv(key, ""))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func storageNotConfiguredError() error {
	return errors.New("storage is not configured")
}

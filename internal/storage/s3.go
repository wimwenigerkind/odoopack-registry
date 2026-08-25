package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var _ Storage = (*S3Storage)(nil)

type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	UsePathStyle    bool
	Prefix          string
	PresignTTL      time.Duration
}

type S3Storage struct {
	client     *minio.Client
	bucket     string
	prefix     string
	presignTTL time.Duration
}

func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("storage: s3 bucket is required")
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}

	var creds *credentials.Credentials
	if cfg.AccessKeyID != "" {
		creds = credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, "")
	} else {
		creds = credentials.NewChainCredentials([]credentials.Provider{
			&credentials.EnvAWS{},
			&credentials.EnvMinio{},
			&credentials.IAM{Client: &http.Client{Timeout: 10 * time.Second}},
		})
	}

	opts := &minio.Options{
		Creds:  creds,
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	}
	if cfg.UsePathStyle {
		opts.BucketLookup = minio.BucketLookupPath
	}

	client, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("storage: s3 client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("storage: s3 bucket check: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("storage: s3 bucket %q does not exist", cfg.Bucket)
	}

	ttl := cfg.PresignTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	return &S3Storage{
		client:     client,
		bucket:     cfg.Bucket,
		prefix:     strings.Trim(cfg.Prefix, "/"),
		presignTTL: ttl,
	}, nil
}

func (s *S3Storage) objectKey(key string) string {
	if s.prefix == "" {
		return key
	}
	return s.prefix + "/" + key
}

func (s *S3Storage) Put(ctx context.Context, key string, r io.Reader, opts PutOptions) error {
	size := int64(-1)
	if f, ok := r.(*os.File); ok {
		if info, err := f.Stat(); err == nil {
			size = info.Size()
		}
	}
	_, err := s.client.PutObject(ctx, s.bucket, s.objectKey(key), r, size, minio.PutObjectOptions{
		ContentType: opts.ContentType,
	})
	if err != nil {
		return fmt.Errorf("storage: s3 put: %w", err)
	}
	return nil
}

func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, s.objectKey(key), minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("storage: s3 get: %w", err)
	}
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		if isS3NotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage: s3 stat: %w", err)
	}
	return obj, nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, s.objectKey(key), minio.RemoveObjectOptions{})
	if err != nil {
		if isS3NotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("storage: s3 delete: %w", err)
	}
	return nil
}

func isS3NotFound(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.StatusCode == http.StatusNotFound || resp.Code == "NoSuchKey"
}

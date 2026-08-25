package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func TestS3StorageRoundTrip(t *testing.T) {
	if os.Getenv("ODOOPACK_S3_TEST") != "1" {
		t.Skip("set ODOOPACK_S3_TEST=1 to run against a MinIO/S3 endpoint")
	}

	endpoint := envOr("ODOOPACK_S3_ENDPOINT", "localhost:9100")
	access := envOr("ODOOPACK_S3_ACCESS", "minioadmin")
	secret := envOr("ODOOPACK_S3_SECRET", "minioadmin")
	bucket := envOr("ODOOPACK_S3_BUCKET", "odoopack-test")

	ctx := context.Background()

	admin, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(access, secret, ""),
		Secure:       false,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		t.Fatalf("admin client: %v", err)
	}
	if exists, _ := admin.BucketExists(ctx, bucket); !exists {
		if err := admin.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			t.Fatalf("make bucket: %v", err)
		}
	}

	s, err := NewS3Storage(S3Config{
		Endpoint:        endpoint,
		Region:          "us-east-1",
		Bucket:          bucket,
		AccessKeyID:     access,
		SecretAccessKey: secret,
		UseSSL:          false,
		UsePathStyle:    true,
		Prefix:          "test-prefix",
	})
	if err != nil {
		t.Fatalf("NewS3Storage: %v", err)
	}

	payload := []byte("hello s3 world \x00\x01\x02 odoopack")

	fileKey := "packages/abc/1.0.0-deadbeef.zip"
	tmp := filepath.Join(t.TempDir(), "artifact.zip")
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := s.Put(ctx, fileKey, f, PutOptions{ContentType: "application/zip"}); err != nil {
		t.Fatalf("put (file): %v", err)
	}

	readerKey := "packages/abc/dev-main.zip"
	if err := s.Put(ctx, readerKey, bytes.NewReader(payload), PutOptions{ContentType: "application/zip"}); err != nil {
		t.Fatalf("put (reader): %v", err)
	}

	for _, key := range []string{fileKey, readerKey} {
		rc, err := s.Get(ctx, key)
		if err != nil {
			t.Fatalf("get %s: %v", key, err)
		}
		got, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", key, err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("round-trip mismatch for %s: got %q want %q", key, got, payload)
		}
	}

	if _, err := s.Get(ctx, "packages/does-not-exist.zip"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing key, got %v", err)
	}

	if err := s.Delete(ctx, fileKey); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, fileKey); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	if err := s.Delete(ctx, "packages/does-not-exist.zip"); err != nil {
		t.Fatalf("delete of missing key should be nil (idempotent), got %v", err)
	}

	_ = s.Delete(ctx, readerKey)
}

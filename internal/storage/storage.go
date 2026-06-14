package storage

import (
	"context"
	"errors"
	"io"
)

var ErrNotFound = errors.New("storage: object not found")

type PutOptions struct {
	ContentType string
}

type Storage interface {
	Put(ctx context.Context, key string, r io.Reader, opts PutOptions) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrNotFound = errors.New("file not found")

type Store interface {
	List(ctx context.Context, prefix string) ([]ObjectMetadata, error)
	Open(ctx context.Context, path string) (*FileStream, error)
	Write(ctx context.Context, path string, contentType string, body io.Reader) (*ObjectMetadata, error)
	SignedURL(ctx context.Context, path string, method string, expires time.Duration) (string, error)
}

type ObjectMetadata struct {
	Path        string
	ContentType string
	Size        int64
	UpdatedAt   time.Time
}

type FileStream struct {
	Path        string
	ContentType string
	Size        int64
	Body        io.ReadCloser
}

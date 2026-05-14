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

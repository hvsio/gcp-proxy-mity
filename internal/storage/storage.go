package storage

import (
	"context"
	"errors"
	"io"
)

var ErrNotFound = errors.New("file not found")

type Store interface {
	Open(ctx context.Context, path string) (*FileStream, error)
}

type FileStream struct {
	Path        string
	ContentType string
	Size        int64
	Body        io.ReadCloser
}

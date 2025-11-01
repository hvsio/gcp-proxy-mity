package storage

import (
	"context"
	"io"
)

type FileMetadata struct {
	Name        string
	ContentType string
	Size        int64
}

type WriteRequest struct {
	Path        string
	Content     io.Reader
	ContentType string
}

type WriteResponse struct {
	FilesWritten []FileMetadata
	Errors       []WriteError
}

type WriteError struct {
	FilePath string
	Error    string
}

type ReadResponse struct {
	Files  []FileData
	Errors []ReadError
}

type FileData struct {
	Metadata FileMetadata
	Content  []byte
}

type ReadError struct {
	FilePath string
	Error    string
}

type Storage interface {
	WriteFiles(ctx context.Context, requests []WriteRequest) (*WriteResponse, error)
	ReadFiles(ctx context.Context, filePaths []string) (*ReadResponse, error)
	ReadFile(ctx context.Context, filePath string) (*FileData, error)
}

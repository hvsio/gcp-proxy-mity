package service

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound is returned when a requested file or object does not exist.
var ErrNotFound = errors.New("object not found")

// FileMetadata holds metadata for a stored file.
type FileMetadata struct {
	Name        string
	ContentType string
	Size        int64
}

// WriteRequest represents a request to write a file.
type WriteRequest struct {
	Path        string
	Content     io.Reader
	ContentType string
}

// WriteResponse is the result of a batch write operation.
type WriteResponse struct {
	FilesWritten []FileMetadata
	Errors       []WriteError
}

// WriteError represents an error for a single file in a batch write.
type WriteError struct {
	FilePath string
	Error    string
}

// ReadResponse is the result of a batch read operation.
type ReadResponse struct {
	Files  []FileData
	Errors []ReadError
}

// FileData holds file metadata and content.
type FileData struct {
	Metadata FileMetadata
	Content  []byte
}

// ReadError represents an error for a single file in a batch read.
type ReadError struct {
	FilePath string
	Error    string
}

// DeleteResponse is the result of a batch delete operation.
type DeleteResponse struct {
	Deleted []string     `json:"deleted"`
	Errors  []DeleteError `json:"errors"`
}

// DeleteError represents an error for a single file in a batch delete.
type DeleteError struct {
	FilePath string `json:"file_path"`
	Error    string `json:"error"`
}

// Storage is the interface for storage backends used by StorageService.
type Storage interface {
	WriteFiles(ctx context.Context, requests []WriteRequest) (*WriteResponse, error)
	ReadFiles(ctx context.Context, filePaths []string) (*ReadResponse, error)
	ReadFile(ctx context.Context, filePath string) (*FileData, error)
	DeleteFile(ctx context.Context, filePath string) error
	DeleteFiles(ctx context.Context, filePaths []string) (*DeleteResponse, error)
}

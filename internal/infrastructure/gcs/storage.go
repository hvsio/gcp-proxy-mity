package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"

	"gcp-proxy-mity/internal/service"
	gcsclient "gcp-proxy-mity/pkg/storage/gcs"

	cloudstorage "cloud.google.com/go/storage"
)

// Storage implements service.Storage using Google Cloud Storage.
type Storage struct {
	client *gcsclient.Client
}

// NewStorage creates a new GCS storage implementation.
func NewStorage(client *gcsclient.Client) *Storage {
	return &Storage{
		client: client,
	}
}

// WriteFiles implements service.Storage.
func (s *Storage) WriteFiles(ctx context.Context, requests []service.WriteRequest) (*service.WriteResponse, error) {
	response := &service.WriteResponse{
		FilesWritten: make([]service.FileMetadata, 0),
		Errors:       make([]service.WriteError, 0),
	}

	bucket := s.client.GetBucket()

	for _, req := range requests {
		obj := bucket.Object(req.Path)
		writer := obj.NewWriter(ctx)

		if req.ContentType != "" {
			writer.ContentType = req.ContentType
		} else {
			writer.ContentType = mime.TypeByExtension(getExtension(req.Path))
		}

		written, err := io.Copy(writer, req.Content)
		if err != nil {
			writer.Close()
			response.Errors = append(response.Errors, service.WriteError{
				FilePath: req.Path,
				Error:    err.Error(),
			})
			continue
		}

		if err := writer.Close(); err != nil {
			response.Errors = append(response.Errors, service.WriteError{
				FilePath: req.Path,
				Error:    err.Error(),
			})
			continue
		}

		attrs, err := obj.Attrs(ctx)
		if err != nil {
			response.Errors = append(response.Errors, service.WriteError{
				FilePath: req.Path,
				Error:    fmt.Sprintf("failed to get file attributes: %v", err),
			})
			continue
		}

		response.FilesWritten = append(response.FilesWritten, service.FileMetadata{
			Name:        req.Path,
			ContentType: attrs.ContentType,
			Size:        written,
		})
	}

	return response, nil
}

// ReadFiles implements service.Storage.
func (s *Storage) ReadFiles(ctx context.Context, filePaths []string) (*service.ReadResponse, error) {
	response := &service.ReadResponse{
		Files:  make([]service.FileData, 0),
		Errors: make([]service.ReadError, 0),
	}

	bucket := s.client.GetBucket()

	for _, filePath := range filePaths {
		fileData, err := s.readSingleFile(ctx, bucket, filePath)
		if err != nil {
			response.Errors = append(response.Errors, service.ReadError{
				FilePath: filePath,
				Error:    err.Error(),
			})
			continue
		}

		response.Files = append(response.Files, *fileData)
	}

	return response, nil
}

// ReadFile implements service.Storage.
func (s *Storage) ReadFile(ctx context.Context, filePath string) (*service.FileData, error) {
	bucket := s.client.GetBucket()
	return s.readSingleFile(ctx, bucket, filePath)
}

func (s *Storage) readSingleFile(ctx context.Context, bucket *cloudstorage.BucketHandle, filePath string) (*service.FileData, error) {
	obj := bucket.Object(filePath)

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get object attributes: %w", err)
	}

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	return &service.FileData{
		Metadata: service.FileMetadata{
			Name:        filePath,
			ContentType: attrs.ContentType,
			Size:        attrs.Size,
		},
		Content: content,
	}, nil
}

// DeleteFile implements service.Storage.
func (s *Storage) DeleteFile(ctx context.Context, filePath string) error {
	bucket := s.client.GetBucket()
	obj := bucket.Object(filePath)
	if err := obj.Delete(ctx); err != nil {
		if errors.Is(err, cloudstorage.ErrObjectNotExist) {
			return service.ErrNotFound
		}
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// DeleteFiles implements service.Storage.
func (s *Storage) DeleteFiles(ctx context.Context, filePaths []string) (*service.DeleteResponse, error) {
	response := &service.DeleteResponse{
		Deleted: make([]string, 0),
		Errors:  make([]service.DeleteError, 0),
	}

	bucket := s.client.GetBucket()

	for _, filePath := range filePaths {
		obj := bucket.Object(filePath)
		if err := obj.Delete(ctx); err != nil {
			errMsg := err.Error()
			if errors.Is(err, cloudstorage.ErrObjectNotExist) {
				errMsg = service.ErrNotFound.Error()
			}
			response.Errors = append(response.Errors, service.DeleteError{
				FilePath: filePath,
				Error:    errMsg,
			})
			continue
		}
		response.Deleted = append(response.Deleted, filePath)
	}

	return response, nil
}

func getExtension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' {
			break
		}
	}
	return ""
}

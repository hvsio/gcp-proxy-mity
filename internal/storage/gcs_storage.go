package storage

import (
	"context"
	"fmt"
	"io"
	"mime"

	"gcp-proxy-mity/pkg/storage/gcs"

	"cloud.google.com/go/storage"
)

type GCSStorage struct {
	client *gcs.Client
}

func NewGCSStorage(client *gcs.Client) *GCSStorage {
	return &GCSStorage{
		client: client,
	}
}

func (s *GCSStorage) WriteFiles(ctx context.Context, requests []WriteRequest) (*WriteResponse, error) {
	response := &WriteResponse{
		FilesWritten: make([]FileMetadata, 0),
		Errors:       make([]WriteError, 0),
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
			response.Errors = append(response.Errors, WriteError{
				FilePath: req.Path,
				Error:    err.Error(),
			})
			continue
		}

		if err := writer.Close(); err != nil {
			response.Errors = append(response.Errors, WriteError{
				FilePath: req.Path,
				Error:    err.Error(),
			})
			continue
		}

		attrs, err := obj.Attrs(ctx)
		if err != nil {
			response.Errors = append(response.Errors, WriteError{
				FilePath: req.Path,
				Error:    fmt.Sprintf("failed to get file attributes: %v", err),
			})
			continue
		}

		response.FilesWritten = append(response.FilesWritten, FileMetadata{
			Name:        req.Path,
			ContentType: attrs.ContentType,
			Size:        written,
		})
	}

	return response, nil
}

func (s *GCSStorage) ReadFiles(ctx context.Context, filePaths []string) (*ReadResponse, error) {
	response := &ReadResponse{
		Files:  make([]FileData, 0),
		Errors: make([]ReadError, 0),
	}

	bucket := s.client.GetBucket()

	for _, filePath := range filePaths {
		fileData, err := s.readSingleFile(ctx, bucket, filePath)
		if err != nil {
			response.Errors = append(response.Errors, ReadError{
				FilePath: filePath,
				Error:    err.Error(),
			})
			continue
		}

		response.Files = append(response.Files, *fileData)
	}

	return response, nil
}

func (s *GCSStorage) ReadFile(ctx context.Context, filePath string) (*FileData, error) {
	bucket := s.client.GetBucket()
	return s.readSingleFile(ctx, bucket, filePath)
}

func (s *GCSStorage) readSingleFile(ctx context.Context, bucket *storage.BucketHandle, filePath string) (*FileData, error) {
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

	return &FileData{
		Metadata: FileMetadata{
			Name:        filePath,
			ContentType: attrs.ContentType,
			Size:        attrs.Size,
		},
		Content: content,
	}, nil
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

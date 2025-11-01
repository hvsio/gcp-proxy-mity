package service

import (
	"context"
	"gcp-proxy-mity/internal/storage"
)

// StorageService provides business logic for storage operations
type StorageService struct {
	storage storage.Storage
}

// NewStorageService creates a new storage service
func NewStorageService(storage storage.Storage) *StorageService {
	return &StorageService{
		storage: storage,
	}
}

// WriteFiles writes multiple files to storage
func (s *StorageService) WriteFiles(ctx context.Context, requests []storage.WriteRequest) (*storage.WriteResponse, error) {
	return s.storage.WriteFiles(ctx, requests)
}

// ReadFiles reads multiple files from storage
func (s *StorageService) ReadFiles(ctx context.Context, filePaths []string) (*storage.ReadResponse, error) {
	return s.storage.ReadFiles(ctx, filePaths)
}

// ReadFile reads a single file from storage
func (s *StorageService) ReadFile(ctx context.Context, filePath string) (*storage.FileData, error) {
	return s.storage.ReadFile(ctx, filePath)
}
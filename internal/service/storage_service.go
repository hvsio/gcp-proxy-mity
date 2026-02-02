package service

import (
	"context"
)

// StorageService provides business logic for storage operations
type StorageService struct {
	storage Storage
}

// NewStorageService creates a new storage service
func NewStorageService(storage Storage) *StorageService {
	return &StorageService{
		storage: storage,
	}
}

// WriteFiles writes multiple files to storage
func (s *StorageService) WriteFiles(ctx context.Context, requests []WriteRequest) (*WriteResponse, error) {
	return s.storage.WriteFiles(ctx, requests)
}

// ReadFiles reads multiple files from storage
func (s *StorageService) ReadFiles(ctx context.Context, filePaths []string) (*ReadResponse, error) {
	return s.storage.ReadFiles(ctx, filePaths)
}

// ReadFile reads a single file from storage
func (s *StorageService) ReadFile(ctx context.Context, filePath string) (*FileData, error) {
	return s.storage.ReadFile(ctx, filePath)
}

// DeleteFile deletes a single file from storage
func (s *StorageService) DeleteFile(ctx context.Context, filePath string) error {
	return s.storage.DeleteFile(ctx, filePath)
}

// DeleteFiles deletes multiple files from storage
func (s *StorageService) DeleteFiles(ctx context.Context, filePaths []string) (*DeleteResponse, error) {
	return s.storage.DeleteFiles(ctx, filePaths)
}
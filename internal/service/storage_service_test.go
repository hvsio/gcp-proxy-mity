package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gcp-proxy-mity/internal/storage"
)

// mockStorage is a mock implementation of storage.Storage
type mockStorage struct {
	writeFilesResponse *storage.WriteResponse
	writeFilesError    error
	readFilesResponse  *storage.ReadResponse
	readFilesError     error
	readFileData       *storage.FileData
	readFileError      error
}

func (m *mockStorage) WriteFiles(ctx context.Context, requests []storage.WriteRequest) (*storage.WriteResponse, error) {
	return m.writeFilesResponse, m.writeFilesError
}

func (m *mockStorage) ReadFiles(ctx context.Context, filePaths []string) (*storage.ReadResponse, error) {
	return m.readFilesResponse, m.readFilesError
}

func (m *mockStorage) ReadFile(ctx context.Context, filePath string) (*storage.FileData, error) {
	return m.readFileData, m.readFileError
}

func TestStorageService_WriteFiles(t *testing.T) {
	tests := []struct {
		name           string
		mockStorage    *mockStorage
		requests       []storage.WriteRequest
		expectError    bool
		expectedFiles  int
		expectedErrors int
	}{
		{
			name: "successful write",
			mockStorage: &mockStorage{
				writeFilesResponse: &storage.WriteResponse{
					FilesWritten: []storage.FileMetadata{
						{Name: "test1.mp4", ContentType: "video/mp4", Size: 100},
						{Name: "test2.mp4", ContentType: "video/mp4", Size: 200},
					},
					Errors: []storage.WriteError{},
				},
			},
			requests: []storage.WriteRequest{
				{Path: "test1.mp4", Content: strings.NewReader("content1"), ContentType: "video/mp4"},
				{Path: "test2.mp4", Content: strings.NewReader("content2"), ContentType: "video/mp4"},
			},
			expectError:    false,
			expectedFiles:  2,
			expectedErrors: 0,
		},
		{
			name: "storage error",
			mockStorage: &mockStorage{
				writeFilesError: errors.New("storage error"),
			},
			requests: []storage.WriteRequest{
				{Path: "test.mp4", Content: strings.NewReader("content"), ContentType: "video/mp4"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewStorageService(tt.mockStorage)
			response, err := service.WriteFiles(context.Background(), tt.requests)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(response.FilesWritten) != tt.expectedFiles {
				t.Errorf("Expected %d files written, got %d", tt.expectedFiles, len(response.FilesWritten))
			}

			if len(response.Errors) != tt.expectedErrors {
				t.Errorf("Expected %d errors, got %d", tt.expectedErrors, len(response.Errors))
			}
		})
	}
}

func TestStorageService_ReadFiles(t *testing.T) {
	tests := []struct {
		name          string
		mockStorage   *mockStorage
		filePaths     []string
		expectError   bool
		expectedFiles int
		expectedErrs  int
	}{
		{
			name: "successful read",
			mockStorage: &mockStorage{
				readFilesResponse: &storage.ReadResponse{
					Files: []storage.FileData{
						{
							Metadata: storage.FileMetadata{Name: "test1.mp4", ContentType: "video/mp4", Size: 100},
							Content:  []byte("content1"),
						},
						{
							Metadata: storage.FileMetadata{Name: "test2.mp4", ContentType: "video/mp4", Size: 200},
							Content:  []byte("content2"),
						},
					},
					Errors: []storage.ReadError{},
				},
			},
			filePaths:     []string{"test1.mp4", "test2.mp4"},
			expectError:   false,
			expectedFiles: 2,
			expectedErrs:  0,
		},
		{
			name: "storage error",
			mockStorage: &mockStorage{
				readFilesError: errors.New("storage error"),
			},
			filePaths:   []string{"test.mp4"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewStorageService(tt.mockStorage)
			response, err := service.ReadFiles(context.Background(), tt.filePaths)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(response.Files) != tt.expectedFiles {
				t.Errorf("Expected %d files, got %d", tt.expectedFiles, len(response.Files))
			}

			if len(response.Errors) != tt.expectedErrs {
				t.Errorf("Expected %d errors, got %d", tt.expectedErrs, len(response.Errors))
			}
		})
	}
}

func TestStorageService_ReadFile(t *testing.T) {
	tests := []struct {
		name        string
		mockStorage *mockStorage
		filePath    string
		expectError bool
		expectedName string
	}{
		{
			name: "successful read",
			mockStorage: &mockStorage{
				readFileData: &storage.FileData{
					Metadata: storage.FileMetadata{Name: "test.mp4", ContentType: "video/mp4", Size: 100},
					Content:  []byte("content"),
				},
			},
			filePath:     "test.mp4",
			expectError:  false,
			expectedName: "test.mp4",
		},
		{
			name: "file not found",
			mockStorage: &mockStorage{
				readFileError: errors.New("file not found"),
			},
			filePath:    "nonexistent.mp4",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewStorageService(tt.mockStorage)
			fileData, err := service.ReadFile(context.Background(), tt.filePath)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if fileData.Metadata.Name != tt.expectedName {
				t.Errorf("Expected name '%s', got '%s'", tt.expectedName, fileData.Metadata.Name)
			}
		})
	}
}
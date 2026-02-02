package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// mockStorage is a mock implementation of Storage
type mockStorage struct {
	writeFilesResponse  *WriteResponse
	writeFilesError     error
	readFilesResponse   *ReadResponse
	readFilesError      error
	readFileData        *FileData
	readFileError       error
	deleteFileError     error
	deleteFilesResponse *DeleteResponse
	deleteFilesError    error
}

func (m *mockStorage) WriteFiles(ctx context.Context, requests []WriteRequest) (*WriteResponse, error) {
	return m.writeFilesResponse, m.writeFilesError
}

func (m *mockStorage) ReadFiles(ctx context.Context, filePaths []string) (*ReadResponse, error) {
	return m.readFilesResponse, m.readFilesError
}

func (m *mockStorage) ReadFile(ctx context.Context, filePath string) (*FileData, error) {
	return m.readFileData, m.readFileError
}

func (m *mockStorage) DeleteFile(ctx context.Context, filePath string) error {
	return m.deleteFileError
}

func (m *mockStorage) DeleteFiles(ctx context.Context, filePaths []string) (*DeleteResponse, error) {
	return m.deleteFilesResponse, m.deleteFilesError
}

func TestStorageService_WriteFiles(t *testing.T) {
	tests := []struct {
		name           string
		mockStorage    *mockStorage
		requests       []WriteRequest
		expectError    bool
		expectedFiles  int
		expectedErrors int
	}{
		{
			name: "successful write",
			mockStorage: &mockStorage{
				writeFilesResponse: &WriteResponse{
					FilesWritten: []FileMetadata{
						{Name: "test1.mp4", ContentType: "video/mp4", Size: 100},
						{Name: "test2.mp4", ContentType: "video/mp4", Size: 200},
					},
					Errors: []WriteError{},
				},
			},
			requests: []WriteRequest{
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
			requests: []WriteRequest{
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
				readFilesResponse: &ReadResponse{
					Files: []FileData{
						{
							Metadata: FileMetadata{Name: "test1.mp4", ContentType: "video/mp4", Size: 100},
							Content:  []byte("content1"),
						},
						{
							Metadata: FileMetadata{Name: "test2.mp4", ContentType: "video/mp4", Size: 200},
							Content:  []byte("content2"),
						},
					},
					Errors: []ReadError{},
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
				readFileData: &FileData{
					Metadata: FileMetadata{Name: "test.mp4", ContentType: "video/mp4", Size: 100},
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

func TestStorageService_DeleteFile(t *testing.T) {
	tests := []struct {
		name        string
		mockStorage *mockStorage
		filePath    string
		expectError bool
	}{
		{
			name: "successful delete",
			mockStorage: &mockStorage{
				deleteFileError: nil,
			},
			filePath:    "photos/photo.jpg",
			expectError: false,
		},
		{
			name: "file not found",
			mockStorage: &mockStorage{
				deleteFileError: ErrNotFound,
			},
			filePath:    "nonexistent.jpg",
			expectError: true,
		},
		{
			name: "storage error",
			mockStorage: &mockStorage{
				deleteFileError: errors.New("storage error"),
			},
			filePath:    "photo.jpg",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewStorageService(tt.mockStorage)
			err := service.DeleteFile(context.Background(), tt.filePath)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

func TestStorageService_DeleteFiles(t *testing.T) {
	tests := []struct {
		name            string
		mockStorage     *mockStorage
		filePaths       []string
		expectError     bool
		expectedDeleted int
		expectedErrors  int
	}{
		{
			name: "successful delete all",
			mockStorage: &mockStorage{
				deleteFilesResponse: &DeleteResponse{
					Deleted: []string{"a.jpg", "b.jpg"},
					Errors:  []DeleteError{},
				},
			},
			filePaths:       []string{"a.jpg", "b.jpg"},
			expectError:     false,
			expectedDeleted: 2,
			expectedErrors:  0,
		},
		{
			name: "partial success",
			mockStorage: &mockStorage{
				deleteFilesResponse: &DeleteResponse{
					Deleted: []string{"a.jpg"},
					Errors:  []DeleteError{{FilePath: "b.jpg", Error: "not found"}},
				},
			},
			filePaths:       []string{"a.jpg", "b.jpg"},
			expectError:     false,
			expectedDeleted: 1,
			expectedErrors:  1,
		},
		{
			name: "storage error",
			mockStorage: &mockStorage{
				deleteFilesError: errors.New("storage error"),
			},
			filePaths:   []string{"a.jpg"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewStorageService(tt.mockStorage)
			response, err := service.DeleteFiles(context.Background(), tt.filePaths)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(response.Deleted) != tt.expectedDeleted {
				t.Errorf("Expected %d deleted, got %d", tt.expectedDeleted, len(response.Deleted))
			}

			if len(response.Errors) != tt.expectedErrors {
				t.Errorf("Expected %d errors, got %d", tt.expectedErrors, len(response.Errors))
			}
		})
	}
}
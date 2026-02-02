package service

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

// MockStorage is a func-based mock used to test the Storage interface contract.
type MockStorage struct {
	writeFilesFunc  func(ctx context.Context, requests []WriteRequest) (*WriteResponse, error)
	readFilesFunc   func(ctx context.Context, filePaths []string) (*ReadResponse, error)
	readFileFunc    func(ctx context.Context, filePath string) (*FileData, error)
	deleteFileFunc  func(ctx context.Context, filePath string) error
	deleteFilesFunc func(ctx context.Context, filePaths []string) (*DeleteResponse, error)
}

func (m *MockStorage) WriteFiles(ctx context.Context, requests []WriteRequest) (*WriteResponse, error) {
	if m.writeFilesFunc != nil {
		return m.writeFilesFunc(ctx, requests)
	}
	return nil, nil
}

func (m *MockStorage) ReadFiles(ctx context.Context, filePaths []string) (*ReadResponse, error) {
	if m.readFilesFunc != nil {
		return m.readFilesFunc(ctx, filePaths)
	}
	return nil, nil
}

func (m *MockStorage) ReadFile(ctx context.Context, filePath string) (*FileData, error) {
	if m.readFileFunc != nil {
		return m.readFileFunc(ctx, filePath)
	}
	return nil, nil
}

func (m *MockStorage) DeleteFile(ctx context.Context, filePath string) error {
	if m.deleteFileFunc != nil {
		return m.deleteFileFunc(ctx, filePath)
	}
	return nil
}

func (m *MockStorage) DeleteFiles(ctx context.Context, filePaths []string) (*DeleteResponse, error) {
	if m.deleteFilesFunc != nil {
		return m.deleteFilesFunc(ctx, filePaths)
	}
	return nil, nil
}

func TestStorageContract_WriteFiles_Success(t *testing.T) {
	mock := &MockStorage{
		writeFilesFunc: func(ctx context.Context, requests []WriteRequest) (*WriteResponse, error) {
			var filesWritten []FileMetadata
			for _, req := range requests {
				content, _ := io.ReadAll(req.Content)
				filesWritten = append(filesWritten, FileMetadata{
					Name:        req.Path,
					ContentType: req.ContentType,
					Size:        int64(len(content)),
				})
			}
			return &WriteResponse{
				FilesWritten: filesWritten,
				Errors:       []WriteError{},
			}, nil
		},
	}

	requests := []WriteRequest{
		{Path: "test1.mp4", Content: strings.NewReader("test content 1"), ContentType: "video/mp4"},
		{Path: "test2.mp4", Content: strings.NewReader("test content 2"), ContentType: "video/mp4"},
	}

	response, err := mock.WriteFiles(context.Background(), requests)
	if err != nil {
		t.Fatalf("WriteFiles failed: %v", err)
	}
	if len(response.FilesWritten) != 2 {
		t.Errorf("Expected 2 files written, got %d", len(response.FilesWritten))
	}
	if len(response.Errors) != 0 {
		t.Errorf("Expected no errors, got %d", len(response.Errors))
	}
}

func TestStorageContract_WriteFiles_PartialFailure(t *testing.T) {
	mock := &MockStorage{
		writeFilesFunc: func(ctx context.Context, requests []WriteRequest) (*WriteResponse, error) {
			var filesWritten []FileMetadata
			var errs []WriteError
			for i, req := range requests {
				if i == 0 {
					errs = append(errs, WriteError{FilePath: req.Path, Error: "simulated error"})
					continue
				}
				content, _ := io.ReadAll(req.Content)
				filesWritten = append(filesWritten, FileMetadata{
					Name: req.Path, ContentType: req.ContentType, Size: int64(len(content)),
				})
			}
			return &WriteResponse{FilesWritten: filesWritten, Errors: errs}, nil
		},
	}

	requests := []WriteRequest{
		{Path: "test1.mp4", Content: strings.NewReader("test content 1"), ContentType: "video/mp4"},
		{Path: "test2.mp4", Content: strings.NewReader("test content 2"), ContentType: "video/mp4"},
	}

	response, err := mock.WriteFiles(context.Background(), requests)
	if err != nil {
		t.Fatalf("WriteFiles failed: %v", err)
	}
	if len(response.FilesWritten) != 1 {
		t.Errorf("Expected 1 file written, got %d", len(response.FilesWritten))
	}
	if len(response.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(response.Errors))
	}
}

func TestStorageContract_ReadFiles_Success(t *testing.T) {
	mock := &MockStorage{
		readFilesFunc: func(ctx context.Context, filePaths []string) (*ReadResponse, error) {
			var files []FileData
			for _, path := range filePaths {
				files = append(files, FileData{
					Metadata: FileMetadata{Name: path, ContentType: "video/mp4", Size: 100},
					Content:  []byte("file content for " + path),
				})
			}
			return &ReadResponse{Files: files, Errors: []ReadError{}}, nil
		},
	}

	response, err := mock.ReadFiles(context.Background(), []string{"test1.mp4", "test2.mp4"})
	if err != nil {
		t.Fatalf("ReadFiles failed: %v", err)
	}
	if len(response.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(response.Files))
	}
	if len(response.Errors) != 0 {
		t.Errorf("Expected no errors, got %d", len(response.Errors))
	}
}

func TestStorageContract_ReadFile_Success(t *testing.T) {
	expectedContent := []byte("file content")
	mock := &MockStorage{
		readFileFunc: func(ctx context.Context, filePath string) (*FileData, error) {
			return &FileData{
				Metadata: FileMetadata{Name: filePath, ContentType: "video/mp4", Size: int64(len(expectedContent))},
				Content:  expectedContent,
			}, nil
		},
	}

	fileData, err := mock.ReadFile(context.Background(), "test.mp4")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if fileData.Metadata.Name != "test.mp4" {
		t.Errorf("Expected name 'test.mp4', got '%s'", fileData.Metadata.Name)
	}
	if !bytes.Equal(fileData.Content, expectedContent) {
		t.Errorf("Content mismatch")
	}
}

func TestStorageContract_ReadFile_NotFound(t *testing.T) {
	mock := &MockStorage{
		readFileFunc: func(ctx context.Context, filePath string) (*FileData, error) {
			return nil, &contractMockError{message: "file not found", isNotFound: true}
		},
	}

	_, err := mock.ReadFile(context.Background(), "nonexistent.mp4")
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
}

func TestStorageContract_DeleteFile_Success(t *testing.T) {
	mock := &MockStorage{
		deleteFileFunc: func(ctx context.Context, filePath string) error { return nil },
	}

	err := mock.DeleteFile(context.Background(), "photos/photo.jpg")
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}
}

func TestStorageContract_DeleteFile_NotFound(t *testing.T) {
	mock := &MockStorage{
		deleteFileFunc: func(ctx context.Context, filePath string) error { return ErrNotFound },
	}

	err := mock.DeleteFile(context.Background(), "nonexistent.jpg")
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestStorageContract_DeleteFiles_Success(t *testing.T) {
	mock := &MockStorage{
		deleteFilesFunc: func(ctx context.Context, filePaths []string) (*DeleteResponse, error) {
			return &DeleteResponse{Deleted: filePaths, Errors: []DeleteError{}}, nil
		},
	}

	response, err := mock.DeleteFiles(context.Background(), []string{"a.jpg", "b.jpg"})
	if err != nil {
		t.Fatalf("DeleteFiles failed: %v", err)
	}
	if len(response.Deleted) != 2 {
		t.Errorf("Expected 2 deleted, got %d", len(response.Deleted))
	}
	if len(response.Errors) != 0 {
		t.Errorf("Expected no errors, got %d", len(response.Errors))
	}
}

func TestStorageContract_DeleteFiles_PartialFailure(t *testing.T) {
	mock := &MockStorage{
		deleteFilesFunc: func(ctx context.Context, filePaths []string) (*DeleteResponse, error) {
			return &DeleteResponse{
				Deleted: []string{filePaths[0]},
				Errors:  []DeleteError{{FilePath: filePaths[1], Error: "not found"}},
			}, nil
		},
	}

	response, err := mock.DeleteFiles(context.Background(), []string{"a.jpg", "nonexistent.jpg"})
	if err != nil {
		t.Fatalf("DeleteFiles failed: %v", err)
	}
	if len(response.Deleted) != 1 {
		t.Errorf("Expected 1 deleted, got %d", len(response.Deleted))
	}
	if len(response.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(response.Errors))
	}
	if response.Errors[0].FilePath != "nonexistent.jpg" {
		t.Errorf("Expected error for nonexistent.jpg, got %s", response.Errors[0].FilePath)
	}
}

type contractMockError struct {
	message   string
	isNotFound bool
}

func (e *contractMockError) Error() string {
	return e.message
}

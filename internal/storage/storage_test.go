package storage

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

// mockStorage is a mock implementation of Storage for testing
type mockStorage struct {
	writeFilesFunc func(ctx context.Context, requests []WriteRequest) (*WriteResponse, error)
	readFilesFunc  func(ctx context.Context, filePaths []string) (*ReadResponse, error)
	readFileFunc   func(ctx context.Context, filePath string) (*FileData, error)
}

func (m *mockStorage) WriteFiles(ctx context.Context, requests []WriteRequest) (*WriteResponse, error) {
	if m.writeFilesFunc != nil {
		return m.writeFilesFunc(ctx, requests)
	}
	return nil, nil
}

func (m *mockStorage) ReadFiles(ctx context.Context, filePaths []string) (*ReadResponse, error) {
	if m.readFilesFunc != nil {
		return m.readFilesFunc(ctx, filePaths)
	}
	return nil, nil
}

func (m *mockStorage) ReadFile(ctx context.Context, filePath string) (*FileData, error) {
	if m.readFileFunc != nil {
		return m.readFileFunc(ctx, filePath)
	}
	return nil, nil
}

func TestStorage_WriteFiles_Success(t *testing.T) {
	mock := &mockStorage{
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
		{
			Path:        "test1.mp4",
			Content:     strings.NewReader("test content 1"),
			ContentType: "video/mp4",
		},
		{
			Path:        "test2.mp4",
			Content:     strings.NewReader("test content 2"),
			ContentType: "video/mp4",
		},
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

func TestStorage_WriteFiles_PartialFailure(t *testing.T) {
	mock := &mockStorage{
		writeFilesFunc: func(ctx context.Context, requests []WriteRequest) (*WriteResponse, error) {
			var filesWritten []FileMetadata
			var errors []WriteError

			for i, req := range requests {
				if i == 0 {
					// Simulate failure for first file
					errors = append(errors, WriteError{
						FilePath: req.Path,
						Error:    "simulated error",
					})
					continue
				}
				content, _ := io.ReadAll(req.Content)
				filesWritten = append(filesWritten, FileMetadata{
					Name:        req.Path,
					ContentType: req.ContentType,
					Size:        int64(len(content)),
				})
			}
			return &WriteResponse{
				FilesWritten: filesWritten,
				Errors:       errors,
			}, nil
		},
	}

	requests := []WriteRequest{
		{
			Path:        "test1.mp4",
			Content:     strings.NewReader("test content 1"),
			ContentType: "video/mp4",
		},
		{
			Path:        "test2.mp4",
			Content:     strings.NewReader("test content 2"),
			ContentType: "video/mp4",
		},
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

func TestStorage_ReadFiles_Success(t *testing.T) {
	mock := &mockStorage{
		readFilesFunc: func(ctx context.Context, filePaths []string) (*ReadResponse, error) {
			var files []FileData
			for _, path := range filePaths {
				files = append(files, FileData{
					Metadata: FileMetadata{
						Name:        path,
						ContentType: "video/mp4",
						Size:        100,
					},
					Content: []byte("file content for " + path),
				})
			}
			return &ReadResponse{
				Files:  files,
				Errors: []ReadError{},
			}, nil
		},
	}

	filePaths := []string{"test1.mp4", "test2.mp4"}
	response, err := mock.ReadFiles(context.Background(), filePaths)
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

func TestStorage_ReadFile_Success(t *testing.T) {
	expectedContent := []byte("file content")
	mock := &mockStorage{
		readFileFunc: func(ctx context.Context, filePath string) (*FileData, error) {
			return &FileData{
				Metadata: FileMetadata{
					Name:        filePath,
					ContentType: "video/mp4",
					Size:        int64(len(expectedContent)),
				},
				Content: expectedContent,
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

func TestStorage_ReadFile_NotFound(t *testing.T) {
	mock := &mockStorage{
		readFileFunc: func(ctx context.Context, filePath string) (*FileData, error) {
			return nil, &mockError{message: "file not found", isNotFound: true}
		},
	}

	_, err := mock.ReadFile(context.Background(), "nonexistent.mp4")
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
}

type mockError struct {
	message   string
	isNotFound bool
}

func (e *mockError) Error() string {
	return e.message
}
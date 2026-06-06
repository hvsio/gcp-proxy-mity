package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gcp-proxy-mity/internal/storage"
)

type fakeStore struct {
	files   map[string]*storage.FileStream
	list    []storage.ObjectMetadata
	errs    map[string]error
	listErr error
}

func (s fakeStore) List(ctx context.Context, prefix string) ([]storage.ObjectMetadata, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}

	files := make([]storage.ObjectMetadata, 0, len(s.list))
	for _, file := range s.list {
		if strings.HasPrefix(file.Path, prefix) {
			files = append(files, file)
		}
	}
	return files, nil
}

func (s fakeStore) Open(ctx context.Context, path string) (*storage.FileStream, error) {
	if err := s.errs[path]; err != nil {
		return nil, err
	}
	file := s.files[path]
	if file == nil {
		return nil, storage.ErrNotFound
	}
	return &storage.FileStream{
		Path:        file.Path,
		ContentType: file.ContentType,
		Size:        file.Size,
		Body:        io.NopCloser(bytes.NewReader(mustRead(file.Body))),
	}, nil
}

func (s fakeStore) Write(ctx context.Context, path string, contentType string, body io.Reader) (*storage.ObjectMetadata, error) {
	content, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return &storage.ObjectMetadata{Path: path, ContentType: contentType, Size: int64(len(content)), UpdatedAt: time.Now()}, nil
}

func (s fakeStore) SignedURL(ctx context.Context, path string, method string, expires time.Duration) (string, error) {
	if err := s.errs[path]; err != nil {
		return "", err
	}
	return "https://signed.example/" + path, nil
}

func TestListFilesReturnsMetadata(t *testing.T) {
	updatedAt := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	handler := NewStorageHandler(fakeStore{
		list: []storage.ObjectMetadata{
			{
				Path:        "photos/a.jpg",
				ContentType: "image/jpeg",
				Size:        42,
				UpdatedAt:   updatedAt,
			},
			{
				Path:        "videos/ignored.mp4",
				ContentType: "video/mp4",
				Size:        100,
				UpdatedAt:   updatedAt,
			},
		},
	})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/files?prefix=photos/", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got listResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Files) != 1 {
		t.Fatalf("expected one file, got %+v", got.Files)
	}
	if got.Files[0].Name != "photos/a.jpg" {
		t.Fatalf("expected photos/a.jpg, got %q", got.Files[0].Name)
	}
	if got.Files[0].ContentType != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %q", got.Files[0].ContentType)
	}
	if got.Files[0].Size != 42 {
		t.Fatalf("expected size 42, got %d", got.Files[0].Size)
	}
	if got.Files[0].UpdatedAt != updatedAt.Format(time.RFC3339) {
		t.Fatalf("expected updated_at %q, got %q", updatedAt.Format(time.RFC3339), got.Files[0].UpdatedAt)
	}
}

func TestListFilesMapsErrors(t *testing.T) {
	handler := NewStorageHandler(fakeStore{listErr: context.DeadlineExceeded})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/files", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestReadFileStreamsObject(t *testing.T) {
	handler := NewStorageHandler(fakeStore{
		files: map[string]*storage.FileStream{
			"photos/a.jpg": {
				Path:        "photos/a.jpg",
				ContentType: "image/jpeg",
				Size:        5,
				Body:        io.NopCloser(bytes.NewReader([]byte("hello"))),
			},
		},
	})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/files/photos/a.jpg", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Content-Length") != "5" {
		t.Fatalf("expected content length 5, got %q", rec.Header().Get("Content-Length"))
	}
	if rec.Body.String() != "hello" {
		t.Fatalf("expected body hello, got %q", rec.Body.String())
	}
}

func TestReadFileSupportsHead(t *testing.T) {
	handler := NewStorageHandler(fakeStore{
		files: map[string]*storage.FileStream{
			"photos/a.jpg": {
				Path:        "photos/a.jpg",
				ContentType: "image/jpeg",
				Size:        5,
				Body:        io.NopCloser(bytes.NewReader([]byte("hello"))),
			},
		},
	})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	req := httptest.NewRequest(http.MethodHead, "/api/v1/storage/files/photos/a.jpg", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty HEAD body, got %q", rec.Body.String())
	}
}

func TestReadFileMapsErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "not found", err: storage.ErrNotFound, want: http.StatusNotFound},
		{name: "timeout", err: context.DeadlineExceeded, want: http.StatusGatewayTimeout},
		{name: "other", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewStorageHandler(fakeStore{errs: map[string]error{"missing": tt.err}})
			mux := http.NewServeMux()
			handler.SetupRoutes(mux)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/files/missing", nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("expected %d, got %d: %s", tt.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestReadFilesReturnsPartialBatch(t *testing.T) {
	handler := NewStorageHandler(fakeStore{
		files: map[string]*storage.FileStream{
			"ok.txt": {
				Path:        "ok.txt",
				ContentType: "text/plain",
				Size:        2,
				Body:        io.NopCloser(bytes.NewReader([]byte("ok"))),
			},
		},
	})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	body := bytes.NewBufferString(`{"file_paths":["ok.txt","missing.txt"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/files/read", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got readResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Files) != 1 || string(got.Files[0].Content) != "ok" {
		t.Fatalf("expected one ok file, got %+v", got.Files)
	}
	if len(got.Errors) != 1 || got.Errors[0].FilePath != "missing.txt" {
		t.Fatalf("expected one missing error, got %+v", got.Errors)
	}
}

func TestReadFilesReadsAllImagesByPrefix(t *testing.T) {
	handler := NewStorageHandler(fakeStore{
		list: []storage.ObjectMetadata{
			{Path: "photos/a.jpg", ContentType: "image/jpeg", Size: 1},
			{Path: "photos/b.png", ContentType: "image/png", Size: 2},
			{Path: "photos/ignored.txt", ContentType: "text/plain", Size: 3},
			{Path: "other/c.jpg", ContentType: "image/jpeg", Size: 4},
		},
		files: map[string]*storage.FileStream{
			"photos/a.jpg": {
				Path:        "photos/a.jpg",
				ContentType: "image/jpeg",
				Size:        1,
				Body:        io.NopCloser(bytes.NewReader([]byte("a"))),
			},
			"photos/b.png": {
				Path:        "photos/b.png",
				ContentType: "image/png",
				Size:        2,
				Body:        io.NopCloser(bytes.NewReader([]byte("bb"))),
			},
			"photos/ignored.txt": {
				Path:        "photos/ignored.txt",
				ContentType: "text/plain",
				Size:        3,
				Body:        io.NopCloser(bytes.NewReader([]byte("txt"))),
			},
			"other/c.jpg": {
				Path:        "other/c.jpg",
				ContentType: "image/jpeg",
				Size:        4,
				Body:        io.NopCloser(bytes.NewReader([]byte("cccc"))),
			},
		},
	})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	body := bytes.NewBufferString(`{"prefix":"photos/"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/files/read", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got readResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Errors) != 0 {
		t.Fatalf("expected no errors, got %+v", got.Errors)
	}
	if len(got.Files) != 2 {
		t.Fatalf("expected two images, got %+v", got.Files)
	}
	if got.Files[0].Metadata.Name != "photos/a.jpg" || string(got.Files[0].Content) != "a" {
		t.Fatalf("expected first image, got %+v", got.Files[0])
	}
	if got.Files[1].Metadata.Name != "photos/b.png" || string(got.Files[1].Content) != "bb" {
		t.Fatalf("expected second image, got %+v", got.Files[1])
	}
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := CORS(CORSConfig{AllowedOrigins: []string{"https://example.com"}})(next)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/storage/files/read", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Fatalf("missing allowed origin header")
	}
}

func mustRead(reader io.ReadCloser) []byte {
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		panic(err)
	}
	return content
}

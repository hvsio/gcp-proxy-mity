package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gcp-proxy-mity/internal/storage"
)

type fakeStore struct {
	files map[string]*storage.FileStream
	errs  map[string]error
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

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gcp-proxy-mity/internal/storage"
)

type StorageHandler struct {
	store storage.Store
}

func NewStorageHandler(store storage.Store) *StorageHandler {
	return &StorageHandler{store: store}
}

func (h *StorageHandler) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/storage/files/read", h.ReadFiles)
	mux.HandleFunc("/api/v1/storage/files/", h.ReadFile)
	mux.HandleFunc("/api/v1/storage/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.ListFiles(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})
}

func (h *StorageHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	files, err := h.store.List(r.Context(), r.URL.Query().Get("prefix"))
	if err != nil {
		writeStorageError(w, err)
		return
	}

	response := listResponse{Files: make([]fileMetadata, 0, len(files))}
	for _, file := range files {
		response.Files = append(response.Files, fileMetadata{
			Name:        file.Path,
			ContentType: file.ContentType,
			Size:        file.Size,
			UpdatedAt:   formatTime(file.UpdatedAt),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *StorageHandler) ReadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filePath := strings.TrimPrefix(r.URL.Path, "/api/v1/storage/files/")
	if filePath == "" {
		http.Error(w, "File path is required", http.StatusBadRequest)
		return
	}

	file, err := h.store.Open(r.Context(), filePath)
	if err != nil {
		writeStorageError(w, err)
		return
	}
	defer file.Body.Close()

	contentType := file.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	if file.Size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	}
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return
	}
	if _, err := io.Copy(w, file.Body); err != nil {
		return
	}
}

func (h *StorageHandler) ReadFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		FilePaths []string `json:"file_paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(request.FilePaths) == 0 {
		http.Error(w, "No file paths provided", http.StatusBadRequest)
		return
	}

	response := readResponse{
		Files:  make([]fileData, 0, len(request.FilePaths)),
		Errors: make([]readError, 0),
	}
	for _, filePath := range request.FilePaths {
		file, err := h.store.Open(r.Context(), filePath)
		if err != nil {
			response.Errors = append(response.Errors, readError{FilePath: filePath, Error: err.Error()})
			continue
		}

		content, err := io.ReadAll(file.Body)
		closeErr := file.Body.Close()
		if err != nil {
			response.Errors = append(response.Errors, readError{FilePath: filePath, Error: err.Error()})
			continue
		}
		if closeErr != nil {
			response.Errors = append(response.Errors, readError{FilePath: filePath, Error: closeErr.Error()})
			continue
		}

		response.Files = append(response.Files, fileData{
			Metadata: fileMetadata{
				Name:        file.Path,
				ContentType: file.ContentType,
				Size:        file.Size,
			},
			Content: content,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func writeStorageError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		http.Error(w, "File not found", http.StatusNotFound)
	case errors.Is(err, context.DeadlineExceeded):
		http.Error(w, "Storage timeout", http.StatusGatewayTimeout)
	case errors.Is(err, context.Canceled):
		return
	default:
		http.Error(w, "Failed to read file: "+err.Error(), http.StatusInternalServerError)
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

type fileMetadata struct {
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type fileData struct {
	Metadata fileMetadata `json:"metadata"`
	Content  []byte       `json:"content"`
}

type readResponse struct {
	Files  []fileData  `json:"files"`
	Errors []readError `json:"errors"`
}

type listResponse struct {
	Files []fileMetadata `json:"files"`
}

type readError struct {
	FilePath string `json:"file_path"`
	Error    string `json:"error"`
}

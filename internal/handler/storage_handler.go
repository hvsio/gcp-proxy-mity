package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"gcp-proxy-mity/internal/service"
	"gcp-proxy-mity/internal/storage"
)

type StorageHandler struct {
	service *service.StorageService
}

func NewStorageHandler(service *service.StorageService) *StorageHandler {
	return &StorageHandler{
		service: service,
	}
}

func (h *StorageHandler) WriteFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	var requests []storage.WriteRequest

	for key, files := range r.MultipartForm.File {
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, "Failed to open file: "+err.Error(), http.StatusBadRequest)
				return
			}

			filePath := key
			if filePath == "" {
				filePath = fileHeader.Filename
			}

			requests = append(requests, storage.WriteRequest{
				Path:        filePath,
				Content:     file,
				ContentType: fileHeader.Header.Get("Content-Type"),
			})

		}
	}

	if len(requests) == 0 {
		http.Error(w, "No files provided", http.StatusBadRequest)
		return
	}

	defer func() {
		for _, req := range requests {
			if closer, ok := req.Content.(io.Closer); ok {
				closer.Close()
			}
		}
	}()

	response, err := h.service.WriteFiles(r.Context(), requests)
	if err != nil {
		http.Error(w, "Failed to write files: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
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

	response, err := h.service.ReadFiles(r.Context(), request.FilePaths)
	if err != nil {
		http.Error(w, "Failed to read files: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *StorageHandler) ReadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	prefix := "/api/v1/storage/files/"

	if !strings.HasPrefix(path, prefix) {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	filePath := strings.TrimPrefix(path, prefix)
	if filePath == "" {
		http.Error(w, "File path is required", http.StatusBadRequest)
		return
	}

	fileData, err := h.service.ReadFile(r.Context(), filePath)
	if err != nil {
		http.Error(w, "Failed to read file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", fileData.Metadata.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileData.Metadata.Size, 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileData.Metadata.Name))

	w.WriteHeader(http.StatusOK)
	w.Write(fileData.Content)
}

// WriteFileRaw handles raw binary media data upload
// PUT /api/v1/storage/files/{filePath}
// Accepts raw binary data in request body with file path in URL
func (h *StorageHandler) WriteFileRaw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract file path from URL
	path := r.URL.Path
	prefix := "/api/v1/storage/files/"
	
	if !strings.HasPrefix(path, prefix) {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	filePath := strings.TrimPrefix(path, prefix)
	// Filter out reserved paths
	if filePath == "" || filePath == "read" || filePath == "raw" {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// Get content type from header or detect from file extension
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = detectContentType(filePath)
	}

	// Limit request body size (e.g., 100MB)
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	// Create write request with raw body data
	request := storage.WriteRequest{
		Path:        filePath,
		Content:     r.Body,
		ContentType: contentType,
	}

	response, err := h.service.WriteFiles(r.Context(), []storage.WriteRequest{request})
	if err != nil {
		http.Error(w, "Failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(response.FilesWritten) == 0 {
		if len(response.Errors) > 0 {
			http.Error(w, "Failed to write file: "+response.Errors[0].Error, http.StatusInternalServerError)
			return
		}
		http.Error(w, "No file was written", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response.FilesWritten[0])
}

// WriteFileRawFromBody handles raw binary media data upload with path in header/query
// POST /api/v1/storage/files/raw
// Accepts raw binary data in request body, file path in X-File-Path header or query parameter
func (h *StorageHandler) WriteFileRawFromBody(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get file path from header or query parameter
	filePath := r.Header.Get("X-File-Path")
	if filePath == "" {
		filePath = r.URL.Query().Get("path")
	}
	if filePath == "" {
		http.Error(w, "File path required in X-File-Path header or 'path' query parameter", http.StatusBadRequest)
		return
	}

	// Get content type from header or detect from file extension
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = detectContentType(filePath)
	}

	// Limit request body size (e.g., 100MB)
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	// Create write request with raw body data
	request := storage.WriteRequest{
		Path:        filePath,
		Content:     r.Body,
		ContentType: contentType,
	}

	response, err := h.service.WriteFiles(r.Context(), []storage.WriteRequest{request})
	if err != nil {
		http.Error(w, "Failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(response.FilesWritten) == 0 {
		if len(response.Errors) > 0 {
			http.Error(w, "Failed to write file: "+response.Errors[0].Error, http.StatusInternalServerError)
			return
		}
		http.Error(w, "No file was written", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response.FilesWritten[0])
}

// detectContentType detects content type from file extension
func detectContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	// Map common media file extensions to MIME types
	mediaTypes := map[string]string{
		".mp4":  "video/mp4",
		".m4v":  "video/mp4",
		".mov":  "video/quicktime",
		".avi":  "video/x-msvideo",
		".webm": "video/webm",
		".heim": "image/heic",
		".heic": "image/heic",
		".heif": "image/heif",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".bmp":  "image/bmp",
	}

	if mimeType, ok := mediaTypes[ext]; ok {
		return mimeType
	}

	// Try system MIME type detection
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}

	// Default to binary if unknown
	return "application/octet-stream"
}

func (h *StorageHandler) SetupRoutes(mux *http.ServeMux) {
	// Multipart file upload (existing, for backward compatibility)
	mux.HandleFunc("/api/v1/storage/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Check if it's multipart or raw
			contentType := r.Header.Get("Content-Type")
			if strings.HasPrefix(contentType, "multipart/form-data") {
				h.WriteFiles(w, r)
			} else {
				// For POST without multipart, use raw endpoint logic
				h.WriteFileRawFromBody(w, r)
			}
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Raw binary upload with path in header/query
	mux.HandleFunc("/api/v1/storage/files/raw", h.WriteFileRawFromBody)

	// Raw binary upload with path in URL (PUT)
	// This must be registered before the generic "/api/v1/storage/files/" handler
	// to avoid conflicts with ReadFile
	mux.HandleFunc("/api/v1/storage/files/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/storage/files/")
		
		// Reserved paths
		if path == "read" || path == "raw" {
			if path == "read" && r.Method == http.MethodPost {
				h.ReadFiles(w, r)
				return
			}
			if path == "raw" && r.Method == http.MethodPost {
				h.WriteFileRawFromBody(w, r)
				return
			}
		}
		
		// PUT = write raw file, GET = read file
		if r.Method == http.MethodPut {
			h.WriteFileRaw(w, r)
		} else if r.Method == http.MethodGet {
			h.ReadFile(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Explicit read endpoint
	mux.HandleFunc("/api/v1/storage/files/read", h.ReadFiles)
}

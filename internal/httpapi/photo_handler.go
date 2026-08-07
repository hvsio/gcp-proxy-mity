package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gcp-proxy-mity/internal/domain/photo"
	"gcp-proxy-mity/internal/storage"

	"github.com/google/uuid"
)

const (
	defaultAssetPageSize  = 100
	signedURLTTL          = 15 * time.Minute
	albumAssetIDLimit     = 100
	tagMutationValueLimit = 20
	assetTagRuneLimit     = 64
)

type PhotoHandler struct {
	assets photo.AssetRepository
	albums photo.AlbumRepository
	jobs   photo.JobRepository
	health photo.HealthChecker
	store  storage.Store
}

func NewPhotoHandler(
	assets photo.AssetRepository,
	albums photo.AlbumRepository,
	jobs photo.JobRepository,
	health photo.HealthChecker,
	store storage.Store,
) *PhotoHandler {
	return &PhotoHandler{
		assets: assets,
		albums: albums,
		jobs:   jobs,
		health: health,
		store:  store,
	}
}

func (h *PhotoHandler) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/assets/upload", h.UploadAssets)
	mux.HandleFunc("/api/v1/assets/", h.AssetByID)
	mux.HandleFunc("/api/v1/assets", h.Assets)
	mux.HandleFunc("/api/v1/albums/", h.AlbumByID)
	mux.HandleFunc("/api/v1/albums", h.Albums)
	mux.HandleFunc("/api/v1/jobs", h.Jobs)
	mux.HandleFunc("/api/v1/status", h.Status)
}

func (h *PhotoHandler) SetupSessionRoute(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/auth/session", h.Session)
}

func (h *PhotoHandler) Session(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"email":         sessionEmail(r),
	})
}

func sessionEmail(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-IAP-Email"))
}

func (h *PhotoHandler) Assets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	filter, ok := decodeAssetFilter(w, r)
	if !ok {
		return
	}
	limit := queryInt(r, "limit", defaultAssetPageSize)
	page, err := h.assets.ListAssets(r.Context(), limit, r.URL.Query().Get("cursor"), filter)
	if err != nil {
		writePhotoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *PhotoHandler) UploadAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "Invalid multipart upload: "+err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		// Keep compatibility with browser FormData that appends arbitrary field names.
		for _, values := range r.MultipartForm.File {
			files = append(files, values...)
		}
	}
	if len(files) == 0 {
		http.Error(w, "At least one file is required", http.StatusBadRequest)
		return
	}

	created := make([]*photo.Asset, 0, len(files))
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			writePhotoError(w, err)
			return
		}

		assetID := uuid.NewString()
		mimeType := header.Header.Get("Content-Type")
		if mimeType == "" || mimeType == "application/octet-stream" {
			mimeType = mimeTypeFromName(header.Filename)
		}
		mediaType := mediaTypeFromMime(mimeType)
		if mediaType == "" {
			file.Close()
			http.Error(w, "Unsupported media type: "+mimeType, http.StatusBadRequest)
			return
		}

		objectKey := fmt.Sprintf("originals/%s/%s_%s", time.Now().UTC().Format("2006/01/02"), assetID, sanitizeFilename(header.Filename))
		written, err := h.store.Write(r.Context(), objectKey, mimeType, file)
		closeErr := file.Close()
		if err != nil {
			writePhotoError(w, err)
			return
		}
		if closeErr != nil {
			writePhotoError(w, closeErr)
			return
		}

		now := time.Now().UTC()
		asset := &photo.Asset{
			ID:                assetID,
			Filename:          header.Filename,
			Type:              mediaType,
			MimeType:          mimeType,
			Size:              written.Size,
			OriginalObjectKey: written.Path,
			UploadedAt:        now,
			Metadata: map[string]any{
				"source": "upload",
			},
			Favorite: false,
			Tags:     []string{},
		}
		if err := h.assets.CreateAsset(r.Context(), asset); err != nil {
			writePhotoError(w, err)
			return
		}

		jobID := uuid.NewString()
		job := &photo.Job{
			ID:        jobID,
			Type:      "metadata_and_previews",
			AssetID:   &assetID,
			State:     "queued",
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := h.jobs.CreateJob(r.Context(), job); err != nil {
			writePhotoError(w, err)
			return
		}
		created = append(created, asset)
	}

	writeJSON(w, http.StatusCreated, map[string]any{"items": created})
}

func (h *PhotoHandler) AssetByID(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/assets/"), "/")
	if path == "tags" {
		if r.Method != http.MethodPatch {
			methodNotAllowed(w)
			return
		}
		assetIDs, add, remove, ok := decodeAssetTagMutationRequest(w, r)
		if !ok {
			return
		}
		if err := h.assets.MutateAssetTags(r.Context(), assetIDs, add, remove); err != nil {
			writePhotoError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Asset id is required", http.StatusBadRequest)
		return
	}
	assetID := parts[0]

	if len(parts) == 1 && r.Method == http.MethodGet {
		asset, err := h.assets.GetAsset(r.Context(), assetID)
		if err != nil {
			writePhotoError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, asset)
		return
	}

	if len(parts) == 2 && parts[1] == "favorite" && r.Method == http.MethodPatch {
		var body struct {
			Favorite bool `json:"favorite"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		asset, err := h.assets.SetAssetFavorite(r.Context(), assetID, body.Favorite)
		if err != nil {
			writePhotoError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, asset)
		return
	}

	if len(parts) == 2 && parts[1] == "urls" && r.Method == http.MethodGet {
		asset, err := h.assets.GetAsset(r.Context(), assetID)
		if err != nil {
			writePhotoError(w, err)
			return
		}
		originalURL, err := h.store.SignedURL(r.Context(), asset.OriginalObjectKey, http.MethodGet, signedURLTTL)
		if err != nil {
			writePhotoError(w, err)
			return
		}
		response := map[string]any{"originalUrl": originalURL}
		if asset.PreviewObjectKey != nil && *asset.PreviewObjectKey != "" {
			previewURL, err := h.store.SignedURL(r.Context(), *asset.PreviewObjectKey, http.MethodGet, signedURLTTL)
			if err != nil {
				writePhotoError(w, err)
				return
			}
			response["previewUrl"] = previewURL
		}
		writeJSON(w, http.StatusOK, response)
		return
	}

	methodNotAllowed(w)
}

func (h *PhotoHandler) Albums(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		albums, err := h.albums.ListAlbums(r.Context())
		if err != nil {
			writePhotoError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": albums})
	case http.MethodPost:
		var body struct {
			Name       string `json:"name"`
			CoverEmoji string `json:"coverEmoji"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		body.Name = strings.TrimSpace(body.Name)
		if body.Name == "" {
			http.Error(w, "Album name is required", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.CoverEmoji) == "" {
			body.CoverEmoji = "📷"
		}
		now := time.Now().UTC()
		album := &photo.Album{ID: uuid.NewString(), Name: body.Name, CoverEmoji: body.CoverEmoji, CreatedAt: now, UpdatedAt: now}
		if err := h.albums.CreateAlbum(r.Context(), album); err != nil {
			writePhotoError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, album)
	default:
		methodNotAllowed(w)
	}
}

func (h *PhotoHandler) AlbumByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/albums/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Album id is required", http.StatusBadRequest)
		return
	}
	albumID := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPatch:
			var body struct {
				Name       string `json:"name"`
				CoverEmoji string `json:"coverEmoji"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
				return
			}
			album := &photo.Album{ID: albumID, Name: strings.TrimSpace(body.Name), CoverEmoji: body.CoverEmoji}
			if album.Name == "" {
				http.Error(w, "Album name is required", http.StatusBadRequest)
				return
			}
			if album.CoverEmoji == "" {
				album.CoverEmoji = "📷"
			}
			if err := h.albums.UpdateAlbum(r.Context(), album); err != nil {
				writePhotoError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			if err := h.albums.DeleteAlbum(r.Context(), albumID); err != nil {
				writePhotoError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w)
		}
		return
	}

	if len(parts) == 2 && parts[1] == "assets" {
		switch r.Method {
		case http.MethodPost:
			assetIDs, ok := decodeAlbumMembershipAssetIDs(w, r)
			if !ok {
				return
			}
			if err := h.albums.AddAssetsToAlbum(r.Context(), albumID, assetIDs); err != nil {
				writePhotoError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			assetIDs, ok := decodeAlbumMembershipAssetIDs(w, r)
			if !ok {
				return
			}
			if err := h.albums.RemoveAssetsFromAlbum(r.Context(), albumID, assetIDs); err != nil {
				writePhotoError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w)
		}
		return
	}

	http.NotFound(w, r)
}

func (h *PhotoHandler) Jobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	jobs, err := h.jobs.ListJobs(r.Context(), queryInt(r, "limit", 50))
	if err != nil {
		writePhotoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": jobs})
}

func (h *PhotoHandler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	status := map[string]any{"healthy": true, "database": "ok"}
	if err := h.health.HealthCheck(r.Context()); err != nil {
		status["healthy"] = false
		status["database"] = err.Error()
		writeJSON(w, http.StatusServiceUnavailable, status)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func writePhotoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, photo.ErrAssetTagLimitExceeded):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, photo.ErrNotFound), errors.Is(err, storage.ErrNotFound):
		http.Error(w, "Not found", http.StatusNotFound)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func decodeAlbumMembershipAssetIDs(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	var body struct {
		AssetIDs []string `json:"assetIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return nil, false
	}
	if len(body.AssetIDs) == 0 || len(body.AssetIDs) > albumAssetIDLimit {
		http.Error(w, "assetIds must contain between 1 and 100 items", http.StatusBadRequest)
		return nil, false
	}
	assetIDs := make([]string, 0, len(body.AssetIDs))
	for _, assetID := range body.AssetIDs {
		trimmed := strings.TrimSpace(assetID)
		if trimmed == "" {
			http.Error(w, "assetIds must contain non-empty ids", http.StatusBadRequest)
			return nil, false
		}
		assetIDs = append(assetIDs, trimmed)
	}
	return assetIDs, true
}

func decodeAssetTagMutationRequest(w http.ResponseWriter, r *http.Request) ([]string, []string, []string, bool) {
	var body struct {
		AssetIDs []string `json:"assetIds"`
		Add      []string `json:"add"`
		Remove   []string `json:"remove"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return nil, nil, nil, false
	}
	if len(body.Add) == 0 && len(body.Remove) == 0 {
		http.Error(w, "at least one tag mutation is required", http.StatusBadRequest)
		return nil, nil, nil, false
	}
	assetIDs, ok := normalizeTagMutationAssetIDs(body.AssetIDs, w)
	if !ok {
		return nil, nil, nil, false
	}
	add, ok := normalizeTagMutationValues(body.Add, "add", w)
	if !ok {
		return nil, nil, nil, false
	}
	remove, ok := normalizeTagMutationValues(body.Remove, "remove", w)
	if !ok {
		return nil, nil, nil, false
	}
	if hasOverlappingStrings(add, remove) {
		http.Error(w, "add and remove must not contain the same tag", http.StatusBadRequest)
		return nil, nil, nil, false
	}
	return assetIDs, add, remove, true
}

func normalizeTagMutationAssetIDs(values []string, w http.ResponseWriter) ([]string, bool) {
	if len(values) == 0 || len(values) > albumAssetIDLimit {
		http.Error(w, "assetIds must contain between 1 and 100 items", http.StatusBadRequest)
		return nil, false
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			http.Error(w, "assetIds must contain non-empty ids", http.StatusBadRequest)
			return nil, false
		}
		normalized = append(normalized, trimmed)
	}
	return dedupeStrings(normalized), true
}

func normalizeTagMutationValues(values []string, field string, w http.ResponseWriter) ([]string, bool) {
	if len(values) > tagMutationValueLimit {
		http.Error(w, field+" must contain at most 20 tags", http.StatusBadRequest)
		return nil, false
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			http.Error(w, field+" must contain non-empty tags", http.StatusBadRequest)
			return nil, false
		}
		if utf8.RuneCountInString(trimmed) > assetTagRuneLimit {
			http.Error(w, field+" tags must be between 1 and 64 characters", http.StatusBadRequest)
			return nil, false
		}
		normalized = append(normalized, trimmed)
	}
	return dedupeStrings(normalized), true
}

func hasOverlappingStrings(left []string, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(left))
	for _, value := range left {
		seen[value] = struct{}{}
	}
	for _, value := range right {
		if _, ok := seen[value]; ok {
			return true
		}
	}
	return false
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func decodeAssetFilter(w http.ResponseWriter, r *http.Request) (photo.AssetFilter, bool) {
	query := r.URL.Query()
	filter := photo.AssetFilter{
		AlbumID: strings.TrimSpace(query.Get("albumId")),
	}
	activeFilters := 0
	if filter.AlbumID != "" {
		activeFilters++
	}

	if values, ok := query["favorite"]; ok {
		favorite := ""
		if len(values) > 0 {
			favorite = values[0]
		}
		if favorite != "true" {
			http.Error(w, "favorite must be true when provided", http.StatusBadRequest)
			return photo.AssetFilter{}, false
		}
		filter.Favorite = true
		activeFilters++
	}

	if values, ok := query["tag"]; ok {
		tag := ""
		if len(values) > 0 {
			tag = strings.TrimSpace(values[0])
		}
		if tag == "" {
			http.Error(w, "tag must be non-empty when provided", http.StatusBadRequest)
			return photo.AssetFilter{}, false
		}
		filter.Tag = tag
		activeFilters++
	}

	if activeFilters > 1 {
		http.Error(w, "only one of albumId, favorite, or tag may be provided", http.StatusBadRequest)
		return photo.AssetFilter{}, false
	}

	return filter, true
}

func queryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func mediaTypeFromMime(mimeType string) string {
	if strings.HasPrefix(mimeType, "image/") {
		return "photo"
	}
	if strings.HasPrefix(mimeType, "video/") {
		return "video"
	}
	return ""
}

func mimeTypeFromName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".heic", ".heif":
		return "image/heic"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	default:
		return "application/octet-stream"
	}
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('_')
	}
	cleaned := strings.Trim(b.String(), "._-")
	if cleaned == "" {
		return "asset"
	}
	return cleaned
}

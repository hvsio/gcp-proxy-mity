package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"gcp-proxy-mity/internal/domain/photo"
	"gcp-proxy-mity/internal/storage"
)

type fakePhotoRepo struct {
	assets                map[string]*photo.Asset
	albums                map[string]*photo.Album
	memberships           map[string]map[string]struct{}
	jobs                  []*photo.Job
	addAssetsToAlbumCalls int
	removeAssetsFromCalls int
}

func newFakePhotoRepo() *fakePhotoRepo {
	return &fakePhotoRepo{
		assets:      map[string]*photo.Asset{},
		albums:      map[string]*photo.Album{},
		memberships: map[string]map[string]struct{}{},
	}
}

func (r *fakePhotoRepo) CreateAsset(ctx context.Context, asset *photo.Asset) error {
	r.assets[asset.ID] = asset
	return nil
}

func (r *fakePhotoRepo) GetAsset(ctx context.Context, id string) (*photo.Asset, error) {
	asset := r.assets[id]
	if asset == nil {
		return nil, photo.ErrNotFound
	}
	return asset, nil
}

func (r *fakePhotoRepo) ListAssets(ctx context.Context, limit int, cursor string, albumID string) (*photo.AssetPage, error) {
	items := make([]*photo.Asset, 0, len(r.assets))
	for _, asset := range r.assets {
		items = append(items, asset)
	}
	return &photo.AssetPage{Items: items, HasMore: false}, nil
}

func (r *fakePhotoRepo) SetAssetFavorite(ctx context.Context, id string, favorite bool) (*photo.Asset, error) {
	asset, err := r.GetAsset(ctx, id)
	if err != nil {
		return nil, err
	}
	asset.Favorite = favorite
	return asset, nil
}

func (r *fakePhotoRepo) CreateAlbum(ctx context.Context, album *photo.Album) error {
	r.albums[album.ID] = album
	return nil
}

func (r *fakePhotoRepo) ListAlbums(ctx context.Context) ([]*photo.Album, error) {
	albums := make([]*photo.Album, 0, len(r.albums))
	for _, album := range r.albums {
		albums = append(albums, album)
	}
	return albums, nil
}

func (r *fakePhotoRepo) UpdateAlbum(ctx context.Context, album *photo.Album) error {
	if r.albums[album.ID] == nil {
		return photo.ErrNotFound
	}
	r.albums[album.ID].Name = album.Name
	r.albums[album.ID].CoverEmoji = album.CoverEmoji
	return nil
}

func (r *fakePhotoRepo) DeleteAlbum(ctx context.Context, id string) error {
	if r.albums[id] == nil {
		return photo.ErrNotFound
	}
	delete(r.albums, id)
	delete(r.memberships, id)
	return nil
}

func (r *fakePhotoRepo) AddAssetsToAlbum(ctx context.Context, albumID string, assetIDs []string) error {
	r.addAssetsToAlbumCalls++
	album := r.albums[albumID]
	if album == nil {
		return photo.ErrNotFound
	}
	unique := uniqueAssetIDs(assetIDs)
	for _, assetID := range unique {
		if _, ok := r.assets[assetID]; !ok {
			return photo.ErrNotFound
		}
	}
	if r.memberships[albumID] == nil {
		r.memberships[albumID] = map[string]struct{}{}
	}
	for _, assetID := range unique {
		if _, exists := r.memberships[albumID][assetID]; exists {
			continue
		}
		r.memberships[albumID][assetID] = struct{}{}
		album.AssetCount++
	}
	return nil
}

func (r *fakePhotoRepo) RemoveAssetsFromAlbum(ctx context.Context, albumID string, assetIDs []string) error {
	r.removeAssetsFromCalls++
	album := r.albums[albumID]
	if album == nil {
		return nil
	}
	for _, assetID := range uniqueAssetIDs(assetIDs) {
		if _, exists := r.memberships[albumID][assetID]; !exists {
			continue
		}
		delete(r.memberships[albumID], assetID)
		if album.AssetCount > 0 {
			album.AssetCount--
		}
	}
	return nil
}

func (r *fakePhotoRepo) CreateJob(ctx context.Context, job *photo.Job) error {
	r.jobs = append(r.jobs, job)
	return nil
}

func (r *fakePhotoRepo) ListJobs(ctx context.Context, limit int) ([]*photo.Job, error) {
	return r.jobs, nil
}

func (r *fakePhotoRepo) HealthCheck(ctx context.Context) error { return nil }

func TestSessionReturnsTrustedIAPEmail(t *testing.T) {
	handler := NewPhotoHandler(newFakePhotoRepo(), nil, nil, nil, fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	req.Header.Set("X-IAP-Email", "user@example.com")
	rec := httptest.NewRecorder()

	handler.Session(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["email"] != "user@example.com" {
		t.Fatalf("expected IAP email, got %q", got["email"])
	}
}

func TestSessionIgnoresUnsignedAuthenticatedUserEmail(t *testing.T) {
	handler := NewPhotoHandler(newFakePhotoRepo(), nil, nil, nil, fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	req.Header.Set("X-Goog-Authenticated-User-Email", "accounts.google.com:user@example.com")
	rec := httptest.NewRecorder()

	handler.Session(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["email"] != "" {
		t.Fatalf("expected unsigned forwarded email to be ignored, got %q", got["email"])
	}
}

func TestSessionRouteCanBeRegisteredWithoutPhotoRepositories(t *testing.T) {
	handler := NewPhotoHandler(nil, nil, nil, nil, nil)
	mux := http.NewServeMux()
	handler.SetupSessionRoute(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUploadAssetsStoresObjectCreatesAssetAndQueuesJob(t *testing.T) {
	repo := newFakePhotoRepo()
	handler := NewPhotoHandler(repo, repo, repo, repo, fakeStore{})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("files", "IMG 001.HEIC")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write([]byte("heic-bytes"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(repo.assets) != 1 {
		t.Fatalf("expected one asset, got %d", len(repo.assets))
	}
	if len(repo.jobs) != 1 || repo.jobs[0].State != "queued" {
		t.Fatalf("expected queued job, got %+v", repo.jobs)
	}
	for _, asset := range repo.assets {
		if asset.MimeType != "image/heic" {
			t.Fatalf("expected image/heic, got %q", asset.MimeType)
		}
		if !strings.HasPrefix(asset.OriginalObjectKey, "originals/") {
			t.Fatalf("expected original object key, got %q", asset.OriginalObjectKey)
		}
	}
}

func TestAssetURLsReturnSignedOriginalAndPreviewURLs(t *testing.T) {
	previewKey := "previews/a.jpg"
	repo := newFakePhotoRepo()
	repo.assets["asset-1"] = &photo.Asset{
		ID:                "asset-1",
		Filename:          "a.heic",
		Type:              "photo",
		MimeType:          "image/heic",
		Size:              10,
		OriginalObjectKey: "originals/a.heic",
		PreviewObjectKey:  &previewKey,
		UploadedAt:        time.Now(),
		Metadata:          map[string]any{},
	}
	handler := NewPhotoHandler(repo, repo, repo, repo, fakeStore{})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets/asset-1/urls", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["originalUrl"] == "" || got["previewUrl"] == "" {
		t.Fatalf("expected signed urls, got %+v", got)
	}
}

func TestFavoriteEndpointUpdatesAsset(t *testing.T) {
	repo := newFakePhotoRepo()
	repo.assets["asset-1"] = &photo.Asset{ID: "asset-1", Metadata: map[string]any{}}
	handler := NewPhotoHandler(repo, repo, repo, repo, fakeStore{})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/assets/asset-1/favorite", strings.NewReader(`{"favorite":true}`))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !repo.assets["asset-1"].Favorite {
		t.Fatalf("expected favorite true")
	}
}

func TestPhotoStoreErrorsMapToNotFound(t *testing.T) {
	repo := newFakePhotoRepo()
	repo.assets["asset-1"] = &photo.Asset{ID: "asset-1", OriginalObjectKey: "missing", Metadata: map[string]any{}}
	handler := NewPhotoHandler(repo, repo, repo, repo, fakeStore{errs: map[string]error{"missing": storage.ErrNotFound}})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets/asset-1/urls", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAlbumsCreatePatchDeletePreservesAssets(t *testing.T) {
	repo := newFakePhotoRepo()
	repo.assets["asset-1"] = &photo.Asset{ID: "asset-1", OriginalObjectKey: "originals/asset-1", Metadata: map[string]any{}}
	handler := NewPhotoHandler(repo, repo, repo, repo, fakeStore{})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/albums", strings.NewReader(`{"name":"Vacation","coverEmoji":" "}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()

	mux.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created photo.Album
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.CoverEmoji != "📷" {
		t.Fatalf("expected default cover emoji, got %q", created.CoverEmoji)
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/albums/"+created.ID, strings.NewReader(`{"name":"Family","coverEmoji":"🌊"}`))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()

	mux.ServeHTTP(patchRec, patchReq)

	if patchRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", patchRec.Code, patchRec.Body.String())
	}
	updated := repo.albums[created.ID]
	if updated.Name != "Family" || updated.CoverEmoji != "🌊" {
		t.Fatalf("expected authoritative rename and cover update, got %+v", updated)
	}

	if err := repo.AddAssetsToAlbum(context.Background(), created.ID, []string{"asset-1"}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/albums/"+created.ID, nil)
	deleteRec := httptest.NewRecorder()

	mux.ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
	if _, ok := repo.albums[created.ID]; ok {
		t.Fatalf("expected album to be deleted")
	}
	if _, ok := repo.assets["asset-1"]; !ok {
		t.Fatalf("expected asset to survive album deletion")
	}
	if len(repo.memberships[created.ID]) != 0 {
		t.Fatalf("expected memberships to be removed")
	}
}

func TestAlbumMembershipRoutesAddAndRemoveAreIdempotent(t *testing.T) {
	repo := newFakePhotoRepo()
	repo.assets["asset-1"] = &photo.Asset{ID: "asset-1", Metadata: map[string]any{}}
	repo.assets["asset-2"] = &photo.Asset{ID: "asset-2", Metadata: map[string]any{}}
	repo.albums["album-1"] = &photo.Album{ID: "album-1", Name: "Album", CoverEmoji: "📷"}
	handler := NewPhotoHandler(repo, repo, repo, repo, fakeStore{})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/albums/album-1/assets", strings.NewReader(`{"assetIds":["asset-1","asset-1","asset-2"]}`))
	addReq.Header.Set("Content-Type", "application/json")
	addRec := httptest.NewRecorder()

	mux.ServeHTTP(addRec, addReq)

	if addRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", addRec.Code, addRec.Body.String())
	}
	if repo.albums["album-1"].AssetCount != 2 {
		t.Fatalf("expected asset count 2 after duplicate add, got %d", repo.albums["album-1"].AssetCount)
	}
	if len(repo.memberships["album-1"]) != 2 {
		t.Fatalf("expected 2 memberships after duplicate add, got %d", len(repo.memberships["album-1"]))
	}

	removeReq := httptest.NewRequest(http.MethodDelete, "/api/v1/albums/album-1/assets", strings.NewReader(`{"assetIds":["asset-2","asset-2","missing"]}`))
	removeReq.Header.Set("Content-Type", "application/json")
	removeRec := httptest.NewRecorder()

	mux.ServeHTTP(removeRec, removeReq)

	if removeRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", removeRec.Code, removeRec.Body.String())
	}
	if repo.albums["album-1"].AssetCount != 1 {
		t.Fatalf("expected asset count 1 after duplicate remove, got %d", repo.albums["album-1"].AssetCount)
	}
	if _, ok := repo.memberships["album-1"]["asset-1"]; !ok {
		t.Fatalf("expected asset-1 membership to remain")
	}
	if _, ok := repo.memberships["album-1"]["asset-2"]; ok {
		t.Fatalf("expected asset-2 membership to be removed")
	}
}

func TestAlbumMembershipRoutesRejectInvalidPayloadsBeforeRepositoryCalls(t *testing.T) {
	tests := []struct {
		name   string
		method string
		body   string
	}{
		{name: "missing asset ids", method: http.MethodPost, body: `{}`},
		{name: "blank asset id", method: http.MethodPost, body: `{"assetIds":["asset-1","   "]}`},
		{name: "malformed body", method: http.MethodDelete, body: `{"assetIds":"asset-1"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakePhotoRepo()
			repo.albums["album-1"] = &photo.Album{ID: "album-1", Name: "Album", CoverEmoji: "📷"}
			handler := NewPhotoHandler(repo, repo, repo, repo, fakeStore{})
			mux := http.NewServeMux()
			handler.SetupRoutes(mux)

			req := httptest.NewRequest(tt.method, "/api/v1/albums/album-1/assets", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
			if repo.addAssetsToAlbumCalls != 0 {
				t.Fatalf("expected add repository calls 0, got %d", repo.addAssetsToAlbumCalls)
			}
			if repo.removeAssetsFromCalls != 0 {
				t.Fatalf("expected remove repository calls 0, got %d", repo.removeAssetsFromCalls)
			}
		})
	}
}

func TestAlbumMembershipRoutesRejectOversizedPayloadBeforeRepositoryCalls(t *testing.T) {
	repo := newFakePhotoRepo()
	repo.albums["album-1"] = &photo.Album{ID: "album-1", Name: "Album", CoverEmoji: "📷"}
	handler := NewPhotoHandler(repo, repo, repo, repo, fakeStore{})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	assetIDs := make([]string, 0, 101)
	for i := 0; i < 101; i++ {
		assetIDs = append(assetIDs, "asset-"+strconv.Itoa(i))
	}
	body, err := json.Marshal(map[string]any{"assetIds": assetIDs})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/albums/album-1/assets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if repo.addAssetsToAlbumCalls != 0 {
		t.Fatalf("expected add repository calls 0, got %d", repo.addAssetsToAlbumCalls)
	}
}

func TestAlbumMembershipAddMissingAssetRollsBack(t *testing.T) {
	repo := newFakePhotoRepo()
	repo.assets["asset-1"] = &photo.Asset{ID: "asset-1", Metadata: map[string]any{}}
	repo.albums["album-1"] = &photo.Album{ID: "album-1", Name: "Album", CoverEmoji: "📷"}
	handler := NewPhotoHandler(repo, repo, repo, repo, fakeStore{})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/albums/album-1/assets", strings.NewReader(`{"assetIds":["asset-1","missing"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if repo.albums["album-1"].AssetCount != 0 {
		t.Fatalf("expected asset count rollback to 0, got %d", repo.albums["album-1"].AssetCount)
	}
	if len(repo.memberships["album-1"]) != 0 {
		t.Fatalf("expected no memberships after rollback, got %d", len(repo.memberships["album-1"]))
	}
}

func TestAlbumMembershipRemoveMissingAlbumIsNoOp(t *testing.T) {
	repo := newFakePhotoRepo()
	handler := NewPhotoHandler(repo, repo, repo, repo, fakeStore{})
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/albums/missing/assets", strings.NewReader(`{"assetIds":["asset-1"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func uniqueAssetIDs(assetIDs []string) []string {
	seen := make(map[string]struct{}, len(assetIDs))
	out := make([]string, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		if _, ok := seen[assetID]; ok {
			continue
		}
		seen[assetID] = struct{}{}
		out = append(out, assetID)
	}
	return out
}

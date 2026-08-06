package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gcp-proxy-mity/internal/domain/photo"
	"gcp-proxy-mity/internal/storage"
)

type fakePhotoRepo struct {
	assets map[string]*photo.Asset
	albums map[string]*photo.Album
	jobs   []*photo.Job
}

func newFakePhotoRepo() *fakePhotoRepo {
	return &fakePhotoRepo{
		assets: map[string]*photo.Asset{},
		albums: map[string]*photo.Album{},
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
	return nil
}

func (r *fakePhotoRepo) AddAssetsToAlbum(ctx context.Context, albumID string, assetIDs []string) error {
	return nil
}

func (r *fakePhotoRepo) RemoveAssetsFromAlbum(ctx context.Context, albumID string, assetIDs []string) error {
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

func TestSessionReturnsIAPEmail(t *testing.T) {
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

package database

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"testing"
	"time"

	"gcp-proxy-mity/internal/domain/photo"
)

func TestFirestorePhotoStoreListAssetsPaginatesDeterministically(t *testing.T) {
	store := newFirestorePhotoStoreWithDocumentStore(newFakePhotoDocumentStore())
	ctx := context.Background()

	for _, asset := range []*photo.Asset{
		{ID: "asset-c", Filename: "c.jpg", Type: "photo", MimeType: "image/jpeg", Size: 3, OriginalObjectKey: "c", UploadedAt: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}},
		{ID: "asset-b", Filename: "b.jpg", Type: "photo", MimeType: "image/jpeg", Size: 2, OriginalObjectKey: "b", UploadedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}},
		{ID: "asset-a", Filename: "a.jpg", Type: "photo", MimeType: "image/jpeg", Size: 1, OriginalObjectKey: "a", UploadedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}},
	} {
		if err := store.CreateAsset(ctx, asset); err != nil {
			t.Fatalf("CreateAsset() error = %v", err)
		}
	}

	firstPage, err := store.ListAssets(ctx, 2, "", "")
	if err != nil {
		t.Fatalf("ListAssets() error = %v", err)
	}
	if !firstPage.HasMore {
		t.Fatalf("expected first page to have more items")
	}
	if got := []string{firstPage.Items[0].ID, firstPage.Items[1].ID}; !equalStrings(got, []string{"asset-c", "asset-b"}) {
		t.Fatalf("first page ids = %v", got)
	}
	if firstPage.NextCursor == "" {
		t.Fatalf("expected next cursor")
	}

	secondPage, err := store.ListAssets(ctx, 2, firstPage.NextCursor, "")
	if err != nil {
		t.Fatalf("ListAssets() second page error = %v", err)
	}
	if secondPage.HasMore {
		t.Fatalf("expected second page to be terminal")
	}
	if got := []string{secondPage.Items[0].ID}; !equalStrings(got, []string{"asset-a"}) {
		t.Fatalf("second page ids = %v", got)
	}
}

func TestFirestorePhotoStoreListAssetsFiltersByAlbum(t *testing.T) {
	fake := newFakePhotoDocumentStore()
	store := newFirestorePhotoStoreWithDocumentStore(fake)
	ctx := context.Background()

	for _, asset := range []*photo.Asset{
		{ID: "asset-c", Filename: "c.jpg", Type: "photo", MimeType: "image/jpeg", Size: 3, OriginalObjectKey: "c", UploadedAt: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}},
		{ID: "asset-b", Filename: "b.jpg", Type: "photo", MimeType: "image/jpeg", Size: 2, OriginalObjectKey: "b", UploadedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}},
		{ID: "asset-a", Filename: "a.jpg", Type: "photo", MimeType: "image/jpeg", Size: 1, OriginalObjectKey: "a", UploadedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}},
	} {
		if err := store.CreateAsset(ctx, asset); err != nil {
			t.Fatalf("CreateAsset() error = %v", err)
		}
	}
	if err := store.CreateAlbum(ctx, &photo.Album{ID: "album-1", Name: "Album 1", CoverEmoji: "x", CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("CreateAlbum() error = %v", err)
	}
	if err := store.AddAssetsToAlbum(ctx, "album-1", []string{"asset-a", "asset-c"}); err != nil {
		t.Fatalf("AddAssetsToAlbum() error = %v", err)
	}

	page, err := store.ListAssets(ctx, 10, "", "album-1")
	if err != nil {
		t.Fatalf("ListAssets() error = %v", err)
	}
	if got := []string{page.Items[0].ID, page.Items[1].ID}; !equalStrings(got, []string{"asset-c", "asset-a"}) {
		t.Fatalf("filtered ids = %v", got)
	}

	if len(fake.memberships) != 2 {
		t.Fatalf("expected persisted memberships, got %d", len(fake.memberships))
	}
}

func TestFirestorePhotoStoreAddAndRemoveAssetsToAlbumUpdateAssetCount(t *testing.T) {
	store := newFirestorePhotoStoreWithDocumentStore(newFakePhotoDocumentStore())
	ctx := context.Background()

	for _, asset := range []*photo.Asset{
		{ID: "asset-1", Filename: "1.jpg", Type: "photo", MimeType: "image/jpeg", Size: 1, OriginalObjectKey: "1", UploadedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}},
		{ID: "asset-2", Filename: "2.jpg", Type: "photo", MimeType: "image/jpeg", Size: 2, OriginalObjectKey: "2", UploadedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}},
	} {
		if err := store.CreateAsset(ctx, asset); err != nil {
			t.Fatalf("CreateAsset() error = %v", err)
		}
	}
	if err := store.CreateAlbum(ctx, &photo.Album{ID: "album-1", Name: "Album 1", CoverEmoji: "x", CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("CreateAlbum() error = %v", err)
	}

	if err := store.AddAssetsToAlbum(ctx, "album-1", []string{"asset-1", "asset-1", "asset-2"}); err != nil {
		t.Fatalf("AddAssetsToAlbum() error = %v", err)
	}
	if got := len(store.store.(*fakePhotoDocumentStore).memberships); got != 2 {
		t.Fatalf("membership count after add = %d", got)
	}
	albums, err := store.ListAlbums(ctx)
	if err != nil {
		t.Fatalf("ListAlbums() error = %v", err)
	}
	if albums[0].AssetCount != 2 {
		t.Fatalf("asset count after add = %d", albums[0].AssetCount)
	}

	if err := store.RemoveAssetsFromAlbum(ctx, "album-1", []string{"asset-2", "asset-2", "missing"}); err != nil {
		t.Fatalf("RemoveAssetsFromAlbum() error = %v", err)
	}
	if got := len(store.store.(*fakePhotoDocumentStore).memberships); got != 1 {
		t.Fatalf("membership count after remove = %d", got)
	}
	albums, err = store.ListAlbums(ctx)
	if err != nil {
		t.Fatalf("ListAlbums() error = %v", err)
	}
	if albums[0].AssetCount != 1 {
		t.Fatalf("asset count after remove = %d", albums[0].AssetCount)
	}
}

func TestFirestorePhotoStoreDeleteAlbumRemovesMemberships(t *testing.T) {
	fake := newFakePhotoDocumentStore()
	store := newFirestorePhotoStoreWithDocumentStore(fake)
	ctx := context.Background()

	if err := store.CreateAsset(ctx, &photo.Asset{ID: "asset-1", Filename: "1.jpg", Type: "photo", MimeType: "image/jpeg", Size: 1, OriginalObjectKey: "1", UploadedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}}); err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	if err := store.CreateAlbum(ctx, &photo.Album{ID: "album-1", Name: "Album 1", CoverEmoji: "x", CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("CreateAlbum() error = %v", err)
	}
	if err := store.AddAssetsToAlbum(ctx, "album-1", []string{"asset-1"}); err != nil {
		t.Fatalf("AddAssetsToAlbum() error = %v", err)
	}

	if err := store.DeleteAlbum(ctx, "album-1"); err != nil {
		t.Fatalf("DeleteAlbum() error = %v", err)
	}
	if len(fake.memberships) != 0 {
		t.Fatalf("expected memberships to be deleted, got %d", len(fake.memberships))
	}
	if _, err := fake.GetAlbum(ctx, "album-1"); !errors.Is(err, photo.ErrNotFound) {
		t.Fatalf("expected deleted album to be missing, got %v", err)
	}
	if _, err := store.GetAsset(ctx, "asset-1"); err != nil {
		t.Fatalf("expected asset to survive album deletion, got %v", err)
	}
	if len(fake.assets) != 1 {
		t.Fatalf("expected asset document to remain, got %d", len(fake.assets))
	}
}

func TestFirestorePhotoStoreAddAssetsToAlbumRollsBackWhenAssetMissing(t *testing.T) {
	fake := newFakePhotoDocumentStore()
	store := newFirestorePhotoStoreWithDocumentStore(fake)
	ctx := context.Background()

	if err := store.CreateAsset(ctx, &photo.Asset{ID: "asset-1", Filename: "1.jpg", Type: "photo", MimeType: "image/jpeg", Size: 1, OriginalObjectKey: "1", UploadedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}}); err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	if err := store.CreateAlbum(ctx, &photo.Album{ID: "album-1", Name: "Album 1", CoverEmoji: "x", CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("CreateAlbum() error = %v", err)
	}

	err := store.AddAssetsToAlbum(ctx, "album-1", []string{"asset-1", "missing"})
	if !errors.Is(err, photo.ErrNotFound) {
		t.Fatalf("AddAssetsToAlbum() error = %v, want %v", err, photo.ErrNotFound)
	}
	if got := len(fake.memberships); got != 0 {
		t.Fatalf("expected rollback to leave 0 memberships, got %d", got)
	}
	albums, err := store.ListAlbums(ctx)
	if err != nil {
		t.Fatalf("ListAlbums() error = %v", err)
	}
	if albums[0].AssetCount != 0 {
		t.Fatalf("expected rollback to preserve asset count 0, got %d", albums[0].AssetCount)
	}
}

func TestFirestorePhotoStoreRemoveAssetsFromMissingAlbumIsNoOp(t *testing.T) {
	store := newFirestorePhotoStoreWithDocumentStore(newFakePhotoDocumentStore())
	ctx := context.Background()

	if err := store.RemoveAssetsFromAlbum(ctx, "missing-album", []string{"asset-1"}); err != nil {
		t.Fatalf("RemoveAssetsFromAlbum() error = %v", err)
	}
}

func TestFirestorePhotoStoreSetAssetFavoritePersistsBoolean(t *testing.T) {
	store := newFirestorePhotoStoreWithDocumentStore(newFakePhotoDocumentStore())
	ctx := context.Background()

	if err := store.CreateAsset(ctx, &photo.Asset{ID: "asset-1", Filename: "1.jpg", Type: "photo", MimeType: "image/jpeg", Size: 1, OriginalObjectKey: "1", UploadedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}, Favorite: false}); err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}

	asset, err := store.SetAssetFavorite(ctx, "asset-1", true)
	if err != nil {
		t.Fatalf("SetAssetFavorite() error = %v", err)
	}
	if !asset.Favorite {
		t.Fatalf("expected favorite to be true")
	}
}

func TestFirestorePhotoStoreGetAssetNormalizesMissingTagsWithoutUsingLegacyMetadata(t *testing.T) {
	fake := newFakePhotoDocumentStore()
	fake.assets["asset-1"] = &firestoreAssetDocument{
		ID:                "asset-1",
		Filename:          "1.jpg",
		Type:              "photo",
		MimeType:          "image/jpeg",
		Size:              1,
		OriginalObjectKey: "1",
		UploadedAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Metadata:          map[string]any{"tags": []string{"legacy"}},
		Favorite:          false,
	}
	store := newFirestorePhotoStoreWithDocumentStore(fake)

	asset, err := store.GetAsset(context.Background(), "asset-1")
	if err != nil {
		t.Fatalf("GetAsset() error = %v", err)
	}
	if asset.Tags == nil || len(asset.Tags) != 0 {
		t.Fatalf("expected tags to normalize to empty slice, got %v", asset.Tags)
	}
}

func TestFirestorePhotoStoreMutateAssetTagsIsIdempotent(t *testing.T) {
	store := newFirestorePhotoStoreWithDocumentStore(newFakePhotoDocumentStore())
	ctx := context.Background()

	if err := store.CreateAsset(ctx, &photo.Asset{ID: "asset-1", Filename: "1.jpg", Type: "photo", MimeType: "image/jpeg", Size: 1, OriginalObjectKey: "1", UploadedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}, Tags: []string{"Family", "Trip"}}); err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	if err := store.CreateAsset(ctx, &photo.Asset{ID: "asset-2", Filename: "2.jpg", Type: "photo", MimeType: "image/jpeg", Size: 2, OriginalObjectKey: "2", UploadedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}, Tags: []string{"Trip"}}); err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}

	for i := 0; i < 2; i++ {
		if err := store.MutateAssetTags(ctx, []string{"asset-1", "asset-1", "asset-2"}, []string{"Summer", "Family"}, []string{"Trip"}); err != nil {
			t.Fatalf("MutateAssetTags() error = %v", err)
		}
	}

	asset1, err := store.GetAsset(ctx, "asset-1")
	if err != nil {
		t.Fatalf("GetAsset(asset-1) error = %v", err)
	}
	if got := asset1.Tags; !equalStrings(got, []string{"Family", "Summer"}) {
		t.Fatalf("asset-1 tags = %v", got)
	}
	asset2, err := store.GetAsset(ctx, "asset-2")
	if err != nil {
		t.Fatalf("GetAsset(asset-2) error = %v", err)
	}
	if got := asset2.Tags; !equalStrings(got, []string{"Summer", "Family"}) {
		t.Fatalf("asset-2 tags = %v", got)
	}
}

func TestFirestorePhotoStoreMutateAssetTagsRollsBackWhenAssetMissing(t *testing.T) {
	store := newFirestorePhotoStoreWithDocumentStore(newFakePhotoDocumentStore())
	ctx := context.Background()

	if err := store.CreateAsset(ctx, &photo.Asset{ID: "asset-1", Filename: "1.jpg", Type: "photo", MimeType: "image/jpeg", Size: 1, OriginalObjectKey: "1", UploadedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}, Tags: []string{"existing"}}); err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}

	err := store.MutateAssetTags(ctx, []string{"asset-1", "missing"}, []string{"new"}, nil)
	if !errors.Is(err, photo.ErrNotFound) {
		t.Fatalf("MutateAssetTags() error = %v, want %v", err, photo.ErrNotFound)
	}
	asset, getErr := store.GetAsset(ctx, "asset-1")
	if getErr != nil {
		t.Fatalf("GetAsset() error = %v", getErr)
	}
	if got := asset.Tags; !equalStrings(got, []string{"existing"}) {
		t.Fatalf("expected rollback to preserve tags, got %v", got)
	}
}

func TestFirestorePhotoStoreMutateAssetTagsRollsBackOnTagLimit(t *testing.T) {
	store := newFirestorePhotoStoreWithDocumentStore(newFakePhotoDocumentStore())
	ctx := context.Background()

	if err := store.CreateAsset(ctx, &photo.Asset{ID: "asset-1", Filename: "1.jpg", Type: "photo", MimeType: "image/jpeg", Size: 1, OriginalObjectKey: "1", UploadedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Metadata: map[string]any{}, Tags: makeTagValues(49)}); err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}

	err := store.MutateAssetTags(ctx, []string{"asset-1"}, []string{"tag-49", "tag-50"}, nil)
	if !errors.Is(err, photo.ErrAssetTagLimitExceeded) {
		t.Fatalf("MutateAssetTags() error = %v, want %v", err, photo.ErrAssetTagLimitExceeded)
	}
	asset, getErr := store.GetAsset(ctx, "asset-1")
	if getErr != nil {
		t.Fatalf("GetAsset() error = %v", getErr)
	}
	if got := asset.Tags; !equalStrings(got, makeTagValues(49)) {
		t.Fatalf("expected rollback to preserve tags, got %v", got)
	}
}

func TestFirestorePhotoStoreCreateAndListJobs(t *testing.T) {
	store := newFirestorePhotoStoreWithDocumentStore(newFakePhotoDocumentStore())
	ctx := context.Background()

	for _, job := range []*photo.Job{
		{ID: "job-a", Type: "metadata", State: "queued", Attempts: 0, CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "job-b", Type: "metadata", State: "running", Attempts: 1, CreatedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)},
	} {
		if err := store.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}
	}

	jobs, err := store.ListJobs(ctx, 10)
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if got := []string{jobs[0].ID, jobs[1].ID}; !equalStrings(got, []string{"job-b", "job-a"}) {
		t.Fatalf("job order = %v", got)
	}
}

type fakePhotoDocumentStore struct {
	assets      map[string]*firestoreAssetDocument
	albums      map[string]*firestoreAlbumDocument
	memberships map[string]*firestoreAlbumMembershipDocument
	jobs        map[string]*firestoreJobDocument
	healthErr   error
}

func newFakePhotoDocumentStore() *fakePhotoDocumentStore {
	return &fakePhotoDocumentStore{
		assets:      map[string]*firestoreAssetDocument{},
		albums:      map[string]*firestoreAlbumDocument{},
		memberships: map[string]*firestoreAlbumMembershipDocument{},
		jobs:        map[string]*firestoreJobDocument{},
	}
}

func (s *fakePhotoDocumentStore) CreateAsset(ctx context.Context, asset *firestoreAssetDocument) error {
	if _, ok := s.assets[asset.ID]; ok {
		return errors.New("asset exists")
	}
	s.assets[asset.ID] = cloneAssetDocument(asset)
	return nil
}

func (s *fakePhotoDocumentStore) GetAsset(ctx context.Context, id string) (*firestoreAssetDocument, error) {
	asset, ok := s.assets[id]
	if !ok {
		return nil, photo.ErrNotFound
	}
	return cloneAssetDocument(asset), nil
}

func (s *fakePhotoDocumentStore) ListAssets(ctx context.Context, limit int, cursor *assetPageCursor) ([]*firestoreAssetDocument, error) {
	assets := make([]*firestoreAssetDocument, 0, len(s.assets))
	for _, asset := range s.assets {
		assets = append(assets, cloneAssetDocument(asset))
	}
	sort.Slice(assets, func(i int, j int) bool {
		if assets[i].UploadedAt.Equal(assets[j].UploadedAt) {
			return assets[i].ID > assets[j].ID
		}
		return assets[i].UploadedAt.After(assets[j].UploadedAt)
	})

	start := 0
	if cursor != nil {
		for idx, asset := range assets {
			if asset.UploadedAt.Before(cursor.UploadedAt) || asset.UploadedAt.Equal(cursor.UploadedAt) && asset.ID < cursor.ID {
				start = idx
				break
			}
			if idx == len(assets)-1 {
				start = len(assets)
			}
		}
	}
	end := start + limit
	if end > len(assets) {
		end = len(assets)
	}
	return assets[start:end], nil
}

func (s *fakePhotoDocumentStore) UpdateAssetFavorite(ctx context.Context, id string, favorite bool) error {
	asset, ok := s.assets[id]
	if !ok {
		return photo.ErrNotFound
	}
	asset.Favorite = favorite
	return nil
}

func (s *fakePhotoDocumentStore) CreateAlbum(ctx context.Context, album *firestoreAlbumDocument) error {
	if _, ok := s.albums[album.ID]; ok {
		return errors.New("album exists")
	}
	s.albums[album.ID] = cloneAlbumDocument(album)
	return nil
}

func (s *fakePhotoDocumentStore) GetAlbum(ctx context.Context, id string) (*firestoreAlbumDocument, error) {
	album, ok := s.albums[id]
	if !ok {
		return nil, photo.ErrNotFound
	}
	return cloneAlbumDocument(album), nil
}

func (s *fakePhotoDocumentStore) ListAlbums(ctx context.Context) ([]*firestoreAlbumDocument, error) {
	albums := make([]*firestoreAlbumDocument, 0, len(s.albums))
	for _, album := range s.albums {
		albums = append(albums, cloneAlbumDocument(album))
	}
	sort.Slice(albums, func(i int, j int) bool {
		if albums[i].CreatedAt.Equal(albums[j].CreatedAt) {
			return albums[i].ID < albums[j].ID
		}
		return albums[i].CreatedAt.Before(albums[j].CreatedAt)
	})
	return albums, nil
}

func (s *fakePhotoDocumentStore) UpdateAlbumDetails(ctx context.Context, id string, name string, coverEmoji string, updatedAt time.Time) error {
	album, ok := s.albums[id]
	if !ok {
		return photo.ErrNotFound
	}
	album.Name = name
	album.CoverEmoji = coverEmoji
	album.UpdatedAt = updatedAt
	return nil
}

func (s *fakePhotoDocumentStore) DeleteAlbum(ctx context.Context, id string) error {
	if _, ok := s.albums[id]; !ok {
		return photo.ErrNotFound
	}
	delete(s.albums, id)
	return nil
}

func (s *fakePhotoDocumentStore) ListAlbumMemberships(ctx context.Context, albumID string, limit int, cursor *assetPageCursor) ([]*firestoreAlbumMembershipDocument, error) {
	memberships := make([]*firestoreAlbumMembershipDocument, 0, len(s.memberships))
	for _, membership := range s.memberships {
		if membership.AlbumID == albumID {
			memberships = append(memberships, cloneMembershipDocument(membership))
		}
	}
	sort.Slice(memberships, func(i int, j int) bool {
		if memberships[i].AssetUploadedAt.Equal(memberships[j].AssetUploadedAt) {
			return memberships[i].AssetID > memberships[j].AssetID
		}
		return memberships[i].AssetUploadedAt.After(memberships[j].AssetUploadedAt)
	})

	start := 0
	if cursor != nil {
		for idx, membership := range memberships {
			if membership.AssetUploadedAt.Before(cursor.UploadedAt) || membership.AssetUploadedAt.Equal(cursor.UploadedAt) && membership.AssetID < cursor.ID {
				start = idx
				break
			}
			if idx == len(memberships)-1 {
				start = len(memberships)
			}
		}
	}
	end := start + limit
	if end > len(memberships) {
		end = len(memberships)
	}
	return memberships[start:end], nil
}

func (s *fakePhotoDocumentStore) ListAllAlbumMemberships(ctx context.Context, albumID string) ([]*firestoreAlbumMembershipDocument, error) {
	memberships := make([]*firestoreAlbumMembershipDocument, 0, len(s.memberships))
	for _, membership := range s.memberships {
		if membership.AlbumID == albumID {
			memberships = append(memberships, cloneMembershipDocument(membership))
		}
	}
	return memberships, nil
}

func (s *fakePhotoDocumentStore) DeleteAlbumMemberships(ctx context.Context, albumID string, assetIDs []string) error {
	for _, assetID := range assetIDs {
		delete(s.memberships, albumMembershipDocumentID(albumID, assetID))
	}
	return nil
}

func (s *fakePhotoDocumentStore) RunTransaction(ctx context.Context, fn func(tx photoDocumentTx) error) error {
	cloned := &fakePhotoDocumentStore{
		assets:      cloneAssetDocuments(s.assets),
		albums:      cloneAlbumDocuments(s.albums),
		memberships: cloneMembershipDocuments(s.memberships),
		jobs:        cloneJobDocuments(s.jobs),
		healthErr:   s.healthErr,
	}
	if err := fn(&fakePhotoDocumentTx{store: cloned}); err != nil {
		return err
	}
	s.assets = cloned.assets
	s.albums = cloned.albums
	s.memberships = cloned.memberships
	s.jobs = cloned.jobs
	return nil
}

func (s *fakePhotoDocumentStore) CreateJob(ctx context.Context, job *firestoreJobDocument) error {
	if _, ok := s.jobs[job.ID]; ok {
		return errors.New("job exists")
	}
	s.jobs[job.ID] = cloneJobDocument(job)
	return nil
}

func (s *fakePhotoDocumentStore) ListJobs(ctx context.Context, limit int) ([]*firestoreJobDocument, error) {
	jobs := make([]*firestoreJobDocument, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, cloneJobDocument(job))
	}
	sort.Slice(jobs, func(i int, j int) bool {
		if jobs[i].CreatedAt.Equal(jobs[j].CreatedAt) {
			return jobs[i].ID > jobs[j].ID
		}
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	if limit < len(jobs) {
		jobs = jobs[:limit]
	}
	return jobs, nil
}

func (s *fakePhotoDocumentStore) HealthCheck(ctx context.Context) error {
	return s.healthErr
}

func (s *fakePhotoDocumentStore) Close() error {
	return nil
}

type fakePhotoDocumentTx struct {
	store *fakePhotoDocumentStore
}

func (t *fakePhotoDocumentTx) GetAsset(ctx context.Context, id string) (*firestoreAssetDocument, error) {
	return t.store.GetAsset(ctx, id)
}

func (t *fakePhotoDocumentTx) PutAsset(ctx context.Context, asset *firestoreAssetDocument) error {
	if _, ok := t.store.assets[asset.ID]; !ok {
		return photo.ErrNotFound
	}
	t.store.assets[asset.ID] = cloneAssetDocument(asset)
	return nil
}

func (t *fakePhotoDocumentTx) GetAlbum(ctx context.Context, id string) (*firestoreAlbumDocument, error) {
	return t.store.GetAlbum(ctx, id)
}

func (t *fakePhotoDocumentTx) PutAlbum(ctx context.Context, album *firestoreAlbumDocument) error {
	if _, ok := t.store.albums[album.ID]; !ok {
		return photo.ErrNotFound
	}
	t.store.albums[album.ID] = cloneAlbumDocument(album)
	return nil
}

func (t *fakePhotoDocumentTx) AlbumMembershipExists(ctx context.Context, albumID string, assetID string) (bool, error) {
	_, ok := t.store.memberships[albumMembershipDocumentID(albumID, assetID)]
	return ok, nil
}

func (t *fakePhotoDocumentTx) CreateAlbumMembership(ctx context.Context, membership *firestoreAlbumMembershipDocument) error {
	key := albumMembershipDocumentID(membership.AlbumID, membership.AssetID)
	if _, ok := t.store.memberships[key]; ok {
		return errors.New("membership exists")
	}
	t.store.memberships[key] = cloneMembershipDocument(membership)
	return nil
}

func (t *fakePhotoDocumentTx) DeleteAlbumMembership(ctx context.Context, albumID string, assetID string) error {
	delete(t.store.memberships, albumMembershipDocumentID(albumID, assetID))
	return nil
}

func cloneAssetDocuments(in map[string]*firestoreAssetDocument) map[string]*firestoreAssetDocument {
	out := make(map[string]*firestoreAssetDocument, len(in))
	for key, value := range in {
		out[key] = cloneAssetDocument(value)
	}
	return out
}

func cloneAlbumDocuments(in map[string]*firestoreAlbumDocument) map[string]*firestoreAlbumDocument {
	out := make(map[string]*firestoreAlbumDocument, len(in))
	for key, value := range in {
		out[key] = cloneAlbumDocument(value)
	}
	return out
}

func cloneMembershipDocuments(in map[string]*firestoreAlbumMembershipDocument) map[string]*firestoreAlbumMembershipDocument {
	out := make(map[string]*firestoreAlbumMembershipDocument, len(in))
	for key, value := range in {
		out[key] = cloneMembershipDocument(value)
	}
	return out
}

func cloneJobDocuments(in map[string]*firestoreJobDocument) map[string]*firestoreJobDocument {
	out := make(map[string]*firestoreJobDocument, len(in))
	for key, value := range in {
		out[key] = cloneJobDocument(value)
	}
	return out
}

func cloneAssetDocument(in *firestoreAssetDocument) *firestoreAssetDocument {
	if in == nil {
		return nil
	}
	clone := *in
	if in.Metadata != nil {
		clone.Metadata = map[string]any{}
		for key, value := range in.Metadata {
			clone.Metadata[key] = value
		}
	}
	if in.Tags != nil {
		clone.Tags = append([]string(nil), in.Tags...)
	}
	return &clone
}

func cloneAlbumDocument(in *firestoreAlbumDocument) *firestoreAlbumDocument {
	if in == nil {
		return nil
	}
	clone := *in
	return &clone
}

func cloneMembershipDocument(in *firestoreAlbumMembershipDocument) *firestoreAlbumMembershipDocument {
	if in == nil {
		return nil
	}
	clone := *in
	return &clone
}

func cloneJobDocument(in *firestoreJobDocument) *firestoreJobDocument {
	if in == nil {
		return nil
	}
	clone := *in
	return &clone
}

func equalStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for idx := range got {
		if got[idx] != want[idx] {
			return false
		}
	}
	return true
}

func makeTagValues(count int) []string {
	tags := make([]string, 0, count)
	for i := 0; i < count; i++ {
		tags = append(tags, "tag-"+strconv.Itoa(i))
	}
	return tags
}

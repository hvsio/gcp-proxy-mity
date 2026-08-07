package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"gcp-proxy-mity/internal/domain/photo"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	firestoreDefaultDatabase       = "(default)"
	firestoreAssetsCollection      = "photo_assets"
	firestoreAlbumsCollection      = "photo_albums"
	firestoreAlbumAssetsCollection = "photo_album_assets"
	firestoreJobsCollection        = "photo_jobs"
)

type FirestorePhotoStore struct {
	store photoDocumentStore
}

type photoDocumentStore interface {
	CreateAsset(ctx context.Context, asset *firestoreAssetDocument) error
	GetAsset(ctx context.Context, id string) (*firestoreAssetDocument, error)
	ListAssets(ctx context.Context, limit int, cursor *assetPageCursor) ([]*firestoreAssetDocument, error)
	ListFavoriteAssets(ctx context.Context, limit int, cursor *assetPageCursor) ([]*firestoreAssetDocument, error)
	ListTaggedAssets(ctx context.Context, limit int, cursor *assetPageCursor, tag string) ([]*firestoreAssetDocument, error)
	UpdateAssetFavorite(ctx context.Context, id string, favorite bool) error
	CreateAlbum(ctx context.Context, album *firestoreAlbumDocument) error
	GetAlbum(ctx context.Context, id string) (*firestoreAlbumDocument, error)
	ListAlbums(ctx context.Context) ([]*firestoreAlbumDocument, error)
	UpdateAlbumDetails(ctx context.Context, id string, name string, coverEmoji string, updatedAt time.Time) error
	DeleteAlbum(ctx context.Context, id string) error
	ListAlbumMemberships(ctx context.Context, albumID string, limit int, cursor *assetPageCursor) ([]*firestoreAlbumMembershipDocument, error)
	ListAllAlbumMemberships(ctx context.Context, albumID string) ([]*firestoreAlbumMembershipDocument, error)
	DeleteAlbumMemberships(ctx context.Context, albumID string, assetIDs []string) error
	RunTransaction(ctx context.Context, fn func(tx photoDocumentTx) error) error
	CreateJob(ctx context.Context, job *firestoreJobDocument) error
	ListJobs(ctx context.Context, limit int) ([]*firestoreJobDocument, error)
	HealthCheck(ctx context.Context) error
	Close() error
}

type photoDocumentTx interface {
	GetAsset(ctx context.Context, id string) (*firestoreAssetDocument, error)
	PutAsset(ctx context.Context, asset *firestoreAssetDocument) error
	GetAlbum(ctx context.Context, id string) (*firestoreAlbumDocument, error)
	PutAlbum(ctx context.Context, album *firestoreAlbumDocument) error
	AlbumMembershipExists(ctx context.Context, albumID string, assetID string) (bool, error)
	CreateAlbumMembership(ctx context.Context, membership *firestoreAlbumMembershipDocument) error
	DeleteAlbumMembership(ctx context.Context, albumID string, assetID string) error
}

type firestoreAssetDocument struct {
	ID                string         `firestore:"id"`
	Filename          string         `firestore:"filename"`
	Type              string         `firestore:"type"`
	MimeType          string         `firestore:"mimeType"`
	Size              int64          `firestore:"size"`
	OriginalObjectKey string         `firestore:"originalObjectKey"`
	PreviewObjectKey  *string        `firestore:"previewObjectKey,omitempty"`
	UploadedAt        time.Time      `firestore:"uploadedAt"`
	Metadata          map[string]any `firestore:"metadata"`
	Favorite          bool           `firestore:"favorite"`
	Tags              []string       `firestore:"tags"`
}

type firestoreAlbumDocument struct {
	ID         string    `firestore:"id"`
	Name       string    `firestore:"name"`
	CoverEmoji string    `firestore:"coverEmoji"`
	CreatedAt  time.Time `firestore:"createdAt"`
	UpdatedAt  time.Time `firestore:"updatedAt"`
	AssetCount int64     `firestore:"assetCount"`
}

type firestoreAlbumMembershipDocument struct {
	AlbumID         string    `firestore:"albumId"`
	AssetID         string    `firestore:"assetId"`
	AssetUploadedAt time.Time `firestore:"assetUploadedAt"`
	CreatedAt       time.Time `firestore:"createdAt"`
}

type firestoreJobDocument struct {
	ID        string    `firestore:"id"`
	Type      string    `firestore:"type"`
	AssetID   *string   `firestore:"assetId,omitempty"`
	State     string    `firestore:"state"`
	Attempts  int       `firestore:"attempts"`
	Error     *string   `firestore:"error,omitempty"`
	CreatedAt time.Time `firestore:"createdAt"`
	UpdatedAt time.Time `firestore:"updatedAt"`
}

type assetPageCursor struct {
	UploadedAt time.Time `json:"uploadedAt"`
	ID         string    `json:"id"`
}

func NewFirestorePhotoStore(ctx context.Context, projectID string, databaseID string) (*FirestorePhotoStore, error) {
	store, err := newFirestorePhotoDocumentStore(ctx, projectID, databaseID)
	if err != nil {
		return nil, err
	}
	return &FirestorePhotoStore{store: store}, nil
}

func newFirestorePhotoStoreWithDocumentStore(store photoDocumentStore) *FirestorePhotoStore {
	return &FirestorePhotoStore{store: store}
}

func (s *FirestorePhotoStore) Close() error {
	return s.store.Close()
}

func (s *FirestorePhotoStore) HealthCheck(ctx context.Context) error {
	return s.store.HealthCheck(ctx)
}

func (s *FirestorePhotoStore) CreateAsset(ctx context.Context, asset *photo.Asset) error {
	if err := s.store.CreateAsset(ctx, assetToFirestoreDocument(asset)); err != nil {
		return fmt.Errorf("failed to create asset: %w", err)
	}
	return nil
}

func (s *FirestorePhotoStore) GetAsset(ctx context.Context, id string) (*photo.Asset, error) {
	asset, err := s.store.GetAsset(ctx, id)
	if err != nil {
		return nil, err
	}
	return firestoreDocumentToAsset(asset), nil
}

func (s *FirestorePhotoStore) ListAssets(ctx context.Context, limit int, cursor string, filter photo.AssetFilter) (*photo.AssetPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	decodedCursor := decodeAssetPageCursor(cursor)
	pageSize := limit + 1

	var items []*photo.Asset
	switch {
	case filter.AlbumID == "" && !filter.Favorite && filter.Tag == "":
		assets, err := s.store.ListAssets(ctx, pageSize, decodedCursor)
		if err != nil {
			return nil, fmt.Errorf("failed to list assets: %w", err)
		}
		items = make([]*photo.Asset, 0, len(assets))
		for _, asset := range assets {
			items = append(items, firestoreDocumentToAsset(asset))
		}
	case filter.AlbumID != "":
		memberships, err := s.store.ListAlbumMemberships(ctx, filter.AlbumID, pageSize, decodedCursor)
		if err != nil {
			return nil, fmt.Errorf("failed to list assets: %w", err)
		}
		items = make([]*photo.Asset, 0, len(memberships))
		for _, membership := range memberships {
			asset, err := s.store.GetAsset(ctx, membership.AssetID)
			if err != nil {
				return nil, fmt.Errorf("failed to list assets: %w", err)
			}
			items = append(items, firestoreDocumentToAsset(asset))
		}
	case filter.Favorite:
		assets, err := s.store.ListFavoriteAssets(ctx, pageSize, decodedCursor)
		if err != nil {
			return nil, fmt.Errorf("failed to list assets: %w", err)
		}
		items = make([]*photo.Asset, 0, len(assets))
		for _, asset := range assets {
			items = append(items, firestoreDocumentToAsset(asset))
		}
	default:
		assets, err := s.store.ListTaggedAssets(ctx, pageSize, decodedCursor, filter.Tag)
		if err != nil {
			return nil, fmt.Errorf("failed to list assets: %w", err)
		}
		items = make([]*photo.Asset, 0, len(assets))
		for _, asset := range assets {
			items = append(items, firestoreDocumentToAsset(asset))
		}
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	nextCursor := ""
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		nextCursor = encodeAssetPageCursor(&assetPageCursor{
			UploadedAt: last.UploadedAt.UTC(),
			ID:         last.ID,
		})
	}

	return &photo.AssetPage{
		Items:      items,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

func (s *FirestorePhotoStore) SetAssetFavorite(ctx context.Context, id string, favorite bool) (*photo.Asset, error) {
	asset, err := s.store.GetAsset(ctx, id)
	if err != nil {
		return nil, err
	}
	asset.Favorite = favorite
	if err := s.store.UpdateAssetFavorite(ctx, id, favorite); err != nil {
		return nil, fmt.Errorf("failed to set asset favorite: %w", err)
	}
	return firestoreDocumentToAsset(asset), nil
}

func (s *FirestorePhotoStore) MutateAssetTags(ctx context.Context, assetIDs []string, add []string, remove []string) error {
	uniqueIDs := uniqueStrings(assetIDs)
	if len(uniqueIDs) == 0 {
		return nil
	}

	if err := s.store.RunTransaction(ctx, func(tx photoDocumentTx) error {
		assets := make([]*firestoreAssetDocument, 0, len(uniqueIDs))
		for _, assetID := range uniqueIDs {
			asset, err := tx.GetAsset(ctx, assetID)
			if err != nil {
				return err
			}
			tags, err := applyAssetTagMutation(asset.Tags, add, remove)
			if err != nil {
				return err
			}
			asset.Tags = tags
			assets = append(assets, asset)
		}

		for _, asset := range assets {
			if err := tx.PutAsset(ctx, asset); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to mutate asset tags: %w", err)
	}
	return nil
}

func (s *FirestorePhotoStore) CreateAlbum(ctx context.Context, album *photo.Album) error {
	if err := s.store.CreateAlbum(ctx, albumToFirestoreDocument(album)); err != nil {
		return fmt.Errorf("failed to create album: %w", err)
	}
	return nil
}

func (s *FirestorePhotoStore) ListAlbums(ctx context.Context) ([]*photo.Album, error) {
	albums, err := s.store.ListAlbums(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list albums: %w", err)
	}

	items := make([]*photo.Album, 0, len(albums))
	for _, album := range albums {
		items = append(items, firestoreDocumentToAlbum(album))
	}
	return items, nil
}

func (s *FirestorePhotoStore) UpdateAlbum(ctx context.Context, album *photo.Album) error {
	_, err := s.store.GetAlbum(ctx, album.ID)
	if err != nil {
		return err
	}

	updatedAt := time.Now().UTC()
	if err := s.store.UpdateAlbumDetails(ctx, album.ID, album.Name, album.CoverEmoji, updatedAt); err != nil {
		return fmt.Errorf("failed to update album: %w", err)
	}
	return nil
}

func (s *FirestorePhotoStore) DeleteAlbum(ctx context.Context, id string) error {
	if _, err := s.store.GetAlbum(ctx, id); err != nil {
		return err
	}

	memberships, err := s.store.ListAllAlbumMemberships(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete album: %w", err)
	}
	if len(memberships) > 0 {
		assetIDs := make([]string, 0, len(memberships))
		for _, membership := range memberships {
			assetIDs = append(assetIDs, membership.AssetID)
		}
		if err := s.store.DeleteAlbumMemberships(ctx, id, assetIDs); err != nil {
			return fmt.Errorf("failed to delete album: %w", err)
		}
	}
	if err := s.store.DeleteAlbum(ctx, id); err != nil {
		return fmt.Errorf("failed to delete album: %w", err)
	}
	return nil
}

func (s *FirestorePhotoStore) AddAssetsToAlbum(ctx context.Context, albumID string, assetIDs []string) error {
	uniqueIDs := uniqueStrings(assetIDs)
	if len(uniqueIDs) == 0 {
		return nil
	}

	if err := s.store.RunTransaction(ctx, func(tx photoDocumentTx) error {
		album, err := tx.GetAlbum(ctx, albumID)
		if err != nil {
			return err
		}

		pending := make([]*firestoreAlbumMembershipDocument, 0, len(uniqueIDs))
		for _, assetID := range uniqueIDs {
			asset, err := tx.GetAsset(ctx, assetID)
			if err != nil {
				return err
			}
			exists, err := tx.AlbumMembershipExists(ctx, albumID, assetID)
			if err != nil {
				return err
			}
			if exists {
				continue
			}
			pending = append(pending, &firestoreAlbumMembershipDocument{
				AlbumID:         albumID,
				AssetID:         assetID,
				AssetUploadedAt: asset.UploadedAt.UTC(),
				CreatedAt:       time.Now().UTC(),
			})
		}

		for _, membership := range pending {
			if err := tx.CreateAlbumMembership(ctx, membership); err != nil {
				return err
			}
			album.AssetCount++
		}

		if len(pending) > 0 {
			return tx.PutAlbum(ctx, album)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to add asset to album: %w", err)
	}
	return nil
}

func (s *FirestorePhotoStore) RemoveAssetsFromAlbum(ctx context.Context, albumID string, assetIDs []string) error {
	uniqueIDs := uniqueStrings(assetIDs)
	if len(uniqueIDs) == 0 {
		return nil
	}

	if err := s.store.RunTransaction(ctx, func(tx photoDocumentTx) error {
		album, err := tx.GetAlbum(ctx, albumID)
		if err != nil {
			if err == photo.ErrNotFound {
				return nil
			}
			return err
		}

		toDelete := make([]string, 0, len(uniqueIDs))
		for _, assetID := range uniqueIDs {
			exists, err := tx.AlbumMembershipExists(ctx, albumID, assetID)
			if err != nil {
				return err
			}
			if !exists {
				continue
			}
			toDelete = append(toDelete, assetID)
		}

		for _, assetID := range toDelete {
			if err := tx.DeleteAlbumMembership(ctx, albumID, assetID); err != nil {
				return err
			}
			if album.AssetCount > 0 {
				album.AssetCount--
			}
		}

		if len(toDelete) > 0 {
			return tx.PutAlbum(ctx, album)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to remove asset from album: %w", err)
	}
	return nil
}

func (s *FirestorePhotoStore) CreateJob(ctx context.Context, job *photo.Job) error {
	if err := s.store.CreateJob(ctx, jobToFirestoreDocument(job)); err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	return nil
}

func (s *FirestorePhotoStore) ListJobs(ctx context.Context, limit int) ([]*photo.Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	jobs, err := s.store.ListJobs(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	items := make([]*photo.Job, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, firestoreDocumentToJob(job))
	}
	return items, nil
}

type firestorePhotoDocumentStore struct {
	client *firestore.Client
}

func newFirestorePhotoDocumentStore(ctx context.Context, projectID string, databaseID string) (*firestorePhotoDocumentStore, error) {
	if databaseID == "" {
		databaseID = firestoreDefaultDatabase
	}
	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client: %w", err)
	}
	return &firestorePhotoDocumentStore{client: client}, nil
}

func (s *firestorePhotoDocumentStore) CreateAsset(ctx context.Context, asset *firestoreAssetDocument) error {
	_, err := s.client.Collection(firestoreAssetsCollection).Doc(asset.ID).Create(ctx, asset)
	return err
}

func (s *firestorePhotoDocumentStore) GetAsset(ctx context.Context, id string) (*firestoreAssetDocument, error) {
	snap, err := s.client.Collection(firestoreAssetsCollection).Doc(id).Get(ctx)
	if err != nil {
		if isFirestoreNotFound(err) {
			return nil, photo.ErrNotFound
		}
		return nil, err
	}

	var asset firestoreAssetDocument
	if err := snap.DataTo(&asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

func (s *firestorePhotoDocumentStore) ListAssets(ctx context.Context, limit int, cursor *assetPageCursor) ([]*firestoreAssetDocument, error) {
	query := s.client.Collection(firestoreAssetsCollection).
		OrderBy("uploadedAt", firestore.Desc).
		OrderBy("id", firestore.Desc)
	if cursor != nil {
		query = query.StartAfter(cursor.UploadedAt, cursor.ID)
	}
	docs, err := query.Limit(limit).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	assets := make([]*firestoreAssetDocument, 0, len(docs))
	for _, doc := range docs {
		var asset firestoreAssetDocument
		if err := doc.DataTo(&asset); err != nil {
			return nil, err
		}
		assets = append(assets, &asset)
	}
	return assets, nil
}

func (s *firestorePhotoDocumentStore) ListFavoriteAssets(ctx context.Context, limit int, cursor *assetPageCursor) ([]*firestoreAssetDocument, error) {
	query := s.client.Collection(firestoreAssetsCollection).
		Where("favorite", "==", true).
		OrderBy("uploadedAt", firestore.Desc).
		OrderBy("id", firestore.Desc)
	if cursor != nil {
		query = query.StartAfter(cursor.UploadedAt, cursor.ID)
	}
	docs, err := query.Limit(limit).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	assets := make([]*firestoreAssetDocument, 0, len(docs))
	for _, doc := range docs {
		var asset firestoreAssetDocument
		if err := doc.DataTo(&asset); err != nil {
			return nil, err
		}
		assets = append(assets, &asset)
	}
	return assets, nil
}

func (s *firestorePhotoDocumentStore) ListTaggedAssets(ctx context.Context, limit int, cursor *assetPageCursor, tag string) ([]*firestoreAssetDocument, error) {
	query := s.client.Collection(firestoreAssetsCollection).
		Where("tags", "array-contains", tag).
		OrderBy("uploadedAt", firestore.Desc).
		OrderBy("id", firestore.Desc)
	if cursor != nil {
		query = query.StartAfter(cursor.UploadedAt, cursor.ID)
	}
	docs, err := query.Limit(limit).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	assets := make([]*firestoreAssetDocument, 0, len(docs))
	for _, doc := range docs {
		var asset firestoreAssetDocument
		if err := doc.DataTo(&asset); err != nil {
			return nil, err
		}
		assets = append(assets, &asset)
	}
	return assets, nil
}

func (s *firestorePhotoDocumentStore) UpdateAssetFavorite(ctx context.Context, id string, favorite bool) error {
	_, err := s.client.Collection(firestoreAssetsCollection).Doc(id).Update(ctx, []firestore.Update{
		{Path: "favorite", Value: favorite},
	})
	return err
}

func (s *firestorePhotoDocumentStore) CreateAlbum(ctx context.Context, album *firestoreAlbumDocument) error {
	_, err := s.client.Collection(firestoreAlbumsCollection).Doc(album.ID).Create(ctx, album)
	return err
}

func (s *firestorePhotoDocumentStore) GetAlbum(ctx context.Context, id string) (*firestoreAlbumDocument, error) {
	snap, err := s.client.Collection(firestoreAlbumsCollection).Doc(id).Get(ctx)
	if err != nil {
		if isFirestoreNotFound(err) {
			return nil, photo.ErrNotFound
		}
		return nil, err
	}

	var album firestoreAlbumDocument
	if err := snap.DataTo(&album); err != nil {
		return nil, err
	}
	return &album, nil
}

func (s *firestorePhotoDocumentStore) ListAlbums(ctx context.Context) ([]*firestoreAlbumDocument, error) {
	docs, err := s.client.Collection(firestoreAlbumsCollection).
		OrderBy("createdAt", firestore.Asc).
		OrderBy("id", firestore.Asc).
		Documents(ctx).
		GetAll()
	if err != nil {
		return nil, err
	}

	albums := make([]*firestoreAlbumDocument, 0, len(docs))
	for _, doc := range docs {
		var album firestoreAlbumDocument
		if err := doc.DataTo(&album); err != nil {
			return nil, err
		}
		albums = append(albums, &album)
	}
	return albums, nil
}

func (s *firestorePhotoDocumentStore) UpdateAlbumDetails(ctx context.Context, id string, name string, coverEmoji string, updatedAt time.Time) error {
	_, err := s.client.Collection(firestoreAlbumsCollection).Doc(id).Update(ctx, []firestore.Update{
		{Path: "name", Value: name},
		{Path: "coverEmoji", Value: coverEmoji},
		{Path: "updatedAt", Value: updatedAt},
	})
	return err
}

func (s *firestorePhotoDocumentStore) DeleteAlbum(ctx context.Context, id string) error {
	_, err := s.client.Collection(firestoreAlbumsCollection).Doc(id).Delete(ctx)
	return err
}

func (s *firestorePhotoDocumentStore) ListAlbumMemberships(ctx context.Context, albumID string, limit int, cursor *assetPageCursor) ([]*firestoreAlbumMembershipDocument, error) {
	query := s.client.Collection(firestoreAlbumAssetsCollection).
		Where("albumId", "==", albumID).
		OrderBy("assetUploadedAt", firestore.Desc).
		OrderBy("assetId", firestore.Desc)
	if cursor != nil {
		query = query.StartAfter(cursor.UploadedAt, cursor.ID)
	}
	docs, err := query.Limit(limit).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	memberships := make([]*firestoreAlbumMembershipDocument, 0, len(docs))
	for _, doc := range docs {
		var membership firestoreAlbumMembershipDocument
		if err := doc.DataTo(&membership); err != nil {
			return nil, err
		}
		memberships = append(memberships, &membership)
	}
	return memberships, nil
}

func (s *firestorePhotoDocumentStore) ListAllAlbumMemberships(ctx context.Context, albumID string) ([]*firestoreAlbumMembershipDocument, error) {
	docs, err := s.client.Collection(firestoreAlbumAssetsCollection).
		Where("albumId", "==", albumID).
		Documents(ctx).
		GetAll()
	if err != nil {
		return nil, err
	}

	memberships := make([]*firestoreAlbumMembershipDocument, 0, len(docs))
	for _, doc := range docs {
		var membership firestoreAlbumMembershipDocument
		if err := doc.DataTo(&membership); err != nil {
			return nil, err
		}
		memberships = append(memberships, &membership)
	}
	return memberships, nil
}

func (s *firestorePhotoDocumentStore) DeleteAlbumMemberships(ctx context.Context, albumID string, assetIDs []string) error {
	for _, chunk := range chunkStrings(uniqueStrings(assetIDs), 400) {
		batch := s.client.Batch()
		for _, assetID := range chunk {
			batch.Delete(s.client.Collection(firestoreAlbumAssetsCollection).Doc(albumMembershipDocumentID(albumID, assetID)))
		}
		if _, err := batch.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *firestorePhotoDocumentStore) RunTransaction(ctx context.Context, fn func(tx photoDocumentTx) error) error {
	return s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		return fn(&firestorePhotoTransaction{
			assets:      s.client.Collection(firestoreAssetsCollection),
			albums:      s.client.Collection(firestoreAlbumsCollection),
			memberships: s.client.Collection(firestoreAlbumAssetsCollection),
			tx:          tx,
		})
	})
}

func (s *firestorePhotoDocumentStore) CreateJob(ctx context.Context, job *firestoreJobDocument) error {
	_, err := s.client.Collection(firestoreJobsCollection).Doc(job.ID).Create(ctx, job)
	return err
}

func (s *firestorePhotoDocumentStore) ListJobs(ctx context.Context, limit int) ([]*firestoreJobDocument, error) {
	docs, err := s.client.Collection(firestoreJobsCollection).
		OrderBy("createdAt", firestore.Desc).
		OrderBy("id", firestore.Desc).
		Limit(limit).
		Documents(ctx).
		GetAll()
	if err != nil {
		return nil, err
	}

	jobs := make([]*firestoreJobDocument, 0, len(docs))
	for _, doc := range docs {
		var job firestoreJobDocument
		if err := doc.DataTo(&job); err != nil {
			return nil, err
		}
		jobs = append(jobs, &job)
	}
	return jobs, nil
}

func (s *firestorePhotoDocumentStore) HealthCheck(ctx context.Context) error {
	_, err := s.client.Collection(firestoreAssetsCollection).Limit(1).Documents(ctx).GetAll()
	return err
}

func (s *firestorePhotoDocumentStore) Close() error {
	return s.client.Close()
}

type firestorePhotoTransaction struct {
	assets      *firestore.CollectionRef
	albums      *firestore.CollectionRef
	memberships *firestore.CollectionRef
	tx          *firestore.Transaction
}

func (t *firestorePhotoTransaction) GetAsset(ctx context.Context, id string) (*firestoreAssetDocument, error) {
	snap, err := t.tx.Get(t.assets.Doc(id))
	if err != nil {
		if isFirestoreNotFound(err) {
			return nil, photo.ErrNotFound
		}
		return nil, err
	}

	var asset firestoreAssetDocument
	if err := snap.DataTo(&asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

func (t *firestorePhotoTransaction) PutAsset(ctx context.Context, asset *firestoreAssetDocument) error {
	return t.tx.Set(t.assets.Doc(asset.ID), asset)
}

func (t *firestorePhotoTransaction) GetAlbum(ctx context.Context, id string) (*firestoreAlbumDocument, error) {
	snap, err := t.tx.Get(t.albums.Doc(id))
	if err != nil {
		if isFirestoreNotFound(err) {
			return nil, photo.ErrNotFound
		}
		return nil, err
	}

	var album firestoreAlbumDocument
	if err := snap.DataTo(&album); err != nil {
		return nil, err
	}
	return &album, nil
}

func (t *firestorePhotoTransaction) PutAlbum(ctx context.Context, album *firestoreAlbumDocument) error {
	return t.tx.Set(t.albums.Doc(album.ID), album)
}

func (t *firestorePhotoTransaction) AlbumMembershipExists(ctx context.Context, albumID string, assetID string) (bool, error) {
	_, err := t.tx.Get(t.memberships.Doc(albumMembershipDocumentID(albumID, assetID)))
	if err != nil {
		if isFirestoreNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (t *firestorePhotoTransaction) CreateAlbumMembership(ctx context.Context, membership *firestoreAlbumMembershipDocument) error {
	return t.tx.Create(t.memberships.Doc(albumMembershipDocumentID(membership.AlbumID, membership.AssetID)), membership)
}

func (t *firestorePhotoTransaction) DeleteAlbumMembership(ctx context.Context, albumID string, assetID string) error {
	return t.tx.Delete(t.memberships.Doc(albumMembershipDocumentID(albumID, assetID)))
}

func assetToFirestoreDocument(asset *photo.Asset) *firestoreAssetDocument {
	metadata := asset.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	return &firestoreAssetDocument{
		ID:                asset.ID,
		Filename:          asset.Filename,
		Type:              asset.Type,
		MimeType:          asset.MimeType,
		Size:              asset.Size,
		OriginalObjectKey: asset.OriginalObjectKey,
		PreviewObjectKey:  asset.PreviewObjectKey,
		UploadedAt:        asset.UploadedAt.UTC(),
		Metadata:          metadata,
		Favorite:          asset.Favorite,
		Tags:              normalizeStoredTags(asset.Tags),
	}
}

func firestoreDocumentToAsset(asset *firestoreAssetDocument) *photo.Asset {
	metadata := asset.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	return &photo.Asset{
		ID:                asset.ID,
		Filename:          asset.Filename,
		Type:              asset.Type,
		MimeType:          asset.MimeType,
		Size:              asset.Size,
		OriginalObjectKey: asset.OriginalObjectKey,
		PreviewObjectKey:  asset.PreviewObjectKey,
		UploadedAt:        asset.UploadedAt.UTC(),
		Metadata:          metadata,
		Favorite:          asset.Favorite,
		Tags:              normalizeStoredTags(asset.Tags),
	}
}

func albumToFirestoreDocument(album *photo.Album) *firestoreAlbumDocument {
	return &firestoreAlbumDocument{
		ID:         album.ID,
		Name:       album.Name,
		CoverEmoji: album.CoverEmoji,
		CreatedAt:  album.CreatedAt.UTC(),
		UpdatedAt:  album.UpdatedAt.UTC(),
		AssetCount: int64(album.AssetCount),
	}
}

func firestoreDocumentToAlbum(album *firestoreAlbumDocument) *photo.Album {
	return &photo.Album{
		ID:         album.ID,
		Name:       album.Name,
		CoverEmoji: album.CoverEmoji,
		CreatedAt:  album.CreatedAt.UTC(),
		UpdatedAt:  album.UpdatedAt.UTC(),
		AssetCount: int(album.AssetCount),
	}
}

func jobToFirestoreDocument(job *photo.Job) *firestoreJobDocument {
	return &firestoreJobDocument{
		ID:        job.ID,
		Type:      job.Type,
		AssetID:   job.AssetID,
		State:     job.State,
		Attempts:  job.Attempts,
		Error:     job.Error,
		CreatedAt: job.CreatedAt.UTC(),
		UpdatedAt: job.UpdatedAt.UTC(),
	}
}

func firestoreDocumentToJob(job *firestoreJobDocument) *photo.Job {
	return &photo.Job{
		ID:        job.ID,
		Type:      job.Type,
		AssetID:   job.AssetID,
		State:     job.State,
		Attempts:  job.Attempts,
		Error:     job.Error,
		CreatedAt: job.CreatedAt.UTC(),
		UpdatedAt: job.UpdatedAt.UTC(),
	}
}

func encodeAssetPageCursor(cursor *assetPageCursor) string {
	if cursor == nil || cursor.UploadedAt.IsZero() || cursor.ID == "" {
		return ""
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeAssetPageCursor(value string) *assetPageCursor {
	if value == "" {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil
	}
	var cursor assetPageCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return nil
	}
	if cursor.UploadedAt.IsZero() || cursor.ID == "" {
		return nil
	}
	return &cursor
}

func albumMembershipDocumentID(albumID string, assetID string) string {
	return albumID + ":" + assetID
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeStoredTags(tags []string) []string {
	normalized := uniqueStrings(tags)
	if len(normalized) == 0 {
		return []string{}
	}
	return normalized
}

func applyAssetTagMutation(existing []string, add []string, remove []string) ([]string, error) {
	removeSet := make(map[string]struct{}, len(remove))
	for _, value := range remove {
		removeSet[value] = struct{}{}
	}

	current := normalizeStoredTags(existing)
	tags := make([]string, 0, len(current)+len(add))
	seen := make(map[string]struct{}, len(current)+len(add))
	for _, value := range current {
		if _, ok := removeSet[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		tags = append(tags, value)
	}
	for _, value := range add {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		tags = append(tags, value)
	}
	if len(tags) > 50 {
		return nil, photo.ErrAssetTagLimitExceeded
	}
	return tags, nil
}

func chunkStrings(values []string, size int) [][]string {
	if len(values) == 0 {
		return nil
	}
	if size <= 0 {
		size = len(values)
	}
	chunks := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
	}
	return chunks
}

func isFirestoreNotFound(err error) bool {
	return status.Code(err) == codes.NotFound
}

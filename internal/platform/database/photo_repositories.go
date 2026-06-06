package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"gcp-proxy-mity/internal/platform/database/generated/dbq"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresPhotoStore struct {
	*PostgresAssetRepository
	*PostgresAlbumRepository
	*PostgresJobRepository
	pool *pgxpool.Pool
}

type PostgresAssetRepository struct {
	q *dbq.Queries
}

type PostgresAlbumRepository struct {
	q *dbq.Queries
}

type PostgresJobRepository struct {
	q *dbq.Queries
}

func NewPostgresPhotoStore(pool *pgxpool.Pool) *PostgresPhotoStore {
	q := dbq.New(pool)
	return &PostgresPhotoStore{
		PostgresAssetRepository: &PostgresAssetRepository{q: q},
		PostgresAlbumRepository: &PostgresAlbumRepository{q: q},
		PostgresJobRepository:   &PostgresJobRepository{q: q},
		pool:                    pool,
	}
}

func (s *PostgresPhotoStore) HealthCheck(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (r *PostgresAssetRepository) CreateAsset(ctx context.Context, asset *Asset) error {
	metadataJSON, err := json.Marshal(asset.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal asset metadata: %w", err)
	}

	err = r.q.CreatePhotoAsset(ctx, dbq.CreatePhotoAssetParams{
		ID:                asset.ID,
		Filename:          asset.Filename,
		MediaType:         asset.Type,
		MimeType:          asset.MimeType,
		Size:              asset.Size,
		OriginalObjectKey: asset.OriginalObjectKey,
		PreviewObjectKey:  asset.PreviewObjectKey,
		UploadedAt:        timestamptz(asset.UploadedAt),
		Metadata:          metadataJSON,
		Favorite:          asset.Favorite,
	})
	if err != nil {
		return fmt.Errorf("failed to create asset: %w", err)
	}
	return nil
}

func (r *PostgresAssetRepository) GetAsset(ctx context.Context, id string) (*Asset, error) {
	row, err := r.q.GetPhotoAsset(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}
	return assetFromRow(row)
}

func (r *PostgresAssetRepository) ListAssets(ctx context.Context, limit int, cursor string, albumID string) (*AssetPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	offset, _ := strconv.Atoi(cursor)

	rows, err := r.q.ListPhotoAssets(ctx, dbq.ListPhotoAssetsParams{
		AlbumID:     albumID,
		AssetOffset: int32(offset),
		AssetLimit:  int32(limit + 1),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list assets: %w", err)
	}

	assets := make([]*Asset, 0, limit)
	for _, row := range rows {
		asset, err := assetFromRow(row)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}

	hasMore := len(assets) > limit
	if hasMore {
		assets = assets[:limit]
	}
	nextCursor := ""
	if hasMore {
		nextCursor = strconv.Itoa(offset + limit)
	}
	return &AssetPage{Items: assets, HasMore: hasMore, NextCursor: nextCursor}, nil
}

func (r *PostgresAssetRepository) SetAssetFavorite(ctx context.Context, id string, favorite bool) (*Asset, error) {
	row, err := r.q.SetPhotoAssetFavorite(ctx, dbq.SetPhotoAssetFavoriteParams{
		ID:       id,
		Favorite: favorite,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to set asset favorite: %w", err)
	}
	return assetFromRow(row)
}

func (r *PostgresAlbumRepository) CreateAlbum(ctx context.Context, album *Album) error {
	err := r.q.CreatePhotoAlbum(ctx, dbq.CreatePhotoAlbumParams{
		ID:         album.ID,
		Name:       album.Name,
		CoverEmoji: album.CoverEmoji,
		CreatedAt:  timestamptz(album.CreatedAt),
		UpdatedAt:  timestamptz(album.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("failed to create album: %w", err)
	}
	return nil
}

func (r *PostgresAlbumRepository) ListAlbums(ctx context.Context) ([]*Album, error) {
	rows, err := r.q.ListPhotoAlbums(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list albums: %w", err)
	}

	albums := make([]*Album, 0, len(rows))
	for _, row := range rows {
		albums = append(albums, &Album{
			ID:         row.ID,
			Name:       row.Name,
			CoverEmoji: row.CoverEmoji,
			CreatedAt:  row.CreatedAt.Time,
			UpdatedAt:  row.UpdatedAt.Time,
			AssetCount: int(row.AssetCount),
		})
	}
	return albums, nil
}

func (r *PostgresAlbumRepository) UpdateAlbum(ctx context.Context, album *Album) error {
	affected, err := r.q.UpdatePhotoAlbum(ctx, dbq.UpdatePhotoAlbumParams{
		ID:         album.ID,
		Name:       album.Name,
		CoverEmoji: album.CoverEmoji,
		UpdatedAt:  timestamptz(time.Now().UTC()),
	})
	if err != nil {
		return fmt.Errorf("failed to update album: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresAlbumRepository) DeleteAlbum(ctx context.Context, id string) error {
	affected, err := r.q.DeletePhotoAlbum(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete album: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresAlbumRepository) AddAssetsToAlbum(ctx context.Context, albumID string, assetIDs []string) error {
	for _, assetID := range assetIDs {
		if err := r.q.AddPhotoAssetToAlbum(ctx, dbq.AddPhotoAssetToAlbumParams{AlbumID: albumID, AssetID: assetID}); err != nil {
			return fmt.Errorf("failed to add asset to album: %w", err)
		}
	}
	return nil
}

func (r *PostgresAlbumRepository) RemoveAssetsFromAlbum(ctx context.Context, albumID string, assetIDs []string) error {
	for _, assetID := range assetIDs {
		if err := r.q.RemovePhotoAssetFromAlbum(ctx, dbq.RemovePhotoAssetFromAlbumParams{AlbumID: albumID, AssetID: assetID}); err != nil {
			return fmt.Errorf("failed to remove asset from album: %w", err)
		}
	}
	return nil
}

func (r *PostgresJobRepository) CreateJob(ctx context.Context, job *Job) error {
	err := r.q.CreatePhotoJob(ctx, dbq.CreatePhotoJobParams{
		ID:        job.ID,
		Type:      job.Type,
		AssetID:   job.AssetID,
		State:     job.State,
		Attempts:  int32(job.Attempts),
		Error:     job.Error,
		CreatedAt: timestamptz(job.CreatedAt),
		UpdatedAt: timestamptz(job.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	return nil
}

func (r *PostgresJobRepository) ListJobs(ctx context.Context, limit int) ([]*Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.q.ListPhotoJobs(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	jobs := make([]*Job, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, &Job{
			ID:        row.ID,
			Type:      row.Type,
			AssetID:   row.AssetID,
			State:     row.State,
			Attempts:  int(row.Attempts),
			Error:     row.Error,
			CreatedAt: row.CreatedAt.Time,
			UpdatedAt: row.UpdatedAt.Time,
		})
	}
	return jobs, nil
}

func assetFromRow(row dbq.PhotoAsset) (*Asset, error) {
	asset := &Asset{
		ID:                row.ID,
		Filename:          row.Filename,
		Type:              row.MediaType,
		MimeType:          row.MimeType,
		Size:              row.Size,
		OriginalObjectKey: row.OriginalObjectKey,
		PreviewObjectKey:  row.PreviewObjectKey,
		UploadedAt:        row.UploadedAt.Time,
		Metadata:          map[string]any{},
		Favorite:          row.Favorite,
	}
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &asset.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal asset metadata: %w", err)
		}
	}
	return asset, nil
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

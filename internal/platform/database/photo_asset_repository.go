package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"gcp-proxy-mity/internal/domain/photo"
	"gcp-proxy-mity/internal/platform/database/generated/dbq"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresAssetRepository struct {
	q *dbq.Queries
}

func NewPostgresAssetRepository(pool *pgxpool.Pool) *PostgresAssetRepository {
	return &PostgresAssetRepository{q: dbq.New(pool)}
}

func (r *PostgresAssetRepository) CreateAsset(ctx context.Context, asset *photo.Asset) error {
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

func (r *PostgresAssetRepository) GetAsset(ctx context.Context, id string) (*photo.Asset, error) {
	row, err := r.q.GetPhotoAsset(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, photo.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}
	return assetFromRow(row)
}

func (r *PostgresAssetRepository) ListAssets(ctx context.Context, limit int, cursor string, albumID string) (*photo.AssetPage, error) {
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

	assets := make([]*photo.Asset, 0, limit)
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
	return &photo.AssetPage{Items: assets, HasMore: hasMore, NextCursor: nextCursor}, nil
}

func (r *PostgresAssetRepository) SetAssetFavorite(ctx context.Context, id string, favorite bool) (*photo.Asset, error) {
	row, err := r.q.SetPhotoAssetFavorite(ctx, dbq.SetPhotoAssetFavoriteParams{
		ID:       id,
		Favorite: favorite,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, photo.ErrNotFound
		}
		return nil, fmt.Errorf("failed to set asset favorite: %w", err)
	}
	return assetFromRow(row)
}

func assetFromRow(row dbq.PhotoAsset) (*photo.Asset, error) {
	asset := &photo.Asset{
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

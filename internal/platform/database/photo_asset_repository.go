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
	pool *pgxpool.Pool
	q    *dbq.Queries
}

func NewPostgresAssetRepository(pool *pgxpool.Pool) *PostgresAssetRepository {
	return &PostgresAssetRepository{pool: pool, q: dbq.New(pool)}
}

func (r *PostgresAssetRepository) CreateAsset(ctx context.Context, asset *photo.Asset) error {
	metadataJSON, err := json.Marshal(asset.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal asset metadata: %w", err)
	}
	tagsJSON, err := json.Marshal(normalizeStoredTags(asset.Tags))
	if err != nil {
		return fmt.Errorf("failed to marshal asset tags: %w", err)
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
		Tags:              tagsJSON,
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

func (r *PostgresAssetRepository) ListAssets(ctx context.Context, limit int, cursor string, filter photo.AssetFilter) (*photo.AssetPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	offset, _ := strconv.Atoi(cursor)

	rows, err := r.q.ListPhotoAssets(ctx, dbq.ListPhotoAssetsParams{
		AlbumID:      filter.AlbumID,
		FavoriteOnly: filter.Favorite,
		Tag:          filter.Tag,
		AssetOffset:  int32(offset),
		AssetLimit:   int32(limit + 1),
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

func (r *PostgresAssetRepository) MutateAssetTags(ctx context.Context, assetIDs []string, add []string, remove []string) error {
	uniqueIDs := uniqueStrings(assetIDs)
	if len(uniqueIDs) == 0 {
		return nil
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to start asset tag transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := r.q.WithTx(tx)
	rows, err := qtx.GetPhotoAssetsByIDs(ctx, uniqueIDs)
	if err != nil {
		return fmt.Errorf("failed to load assets for tag mutation: %w", err)
	}

	rowsByID := make(map[string]dbq.PhotoAsset, len(rows))
	for _, row := range rows {
		rowsByID[row.ID] = row
	}

	for _, assetID := range uniqueIDs {
		row, ok := rowsByID[assetID]
		if !ok {
			return photo.ErrNotFound
		}
		var existing []string
		if len(row.Tags) > 0 {
			if err := json.Unmarshal(row.Tags, &existing); err != nil {
				return fmt.Errorf("failed to unmarshal asset tags: %w", err)
			}
		}
		tags, err := applyAssetTagMutation(existing, add, remove)
		if err != nil {
			return err
		}
		tagsJSON, err := json.Marshal(tags)
		if err != nil {
			return fmt.Errorf("failed to marshal asset tags: %w", err)
		}
		affected, err := qtx.UpdatePhotoAssetTags(ctx, dbq.UpdatePhotoAssetTagsParams{
			ID:   assetID,
			Tags: tagsJSON,
		})
		if err != nil {
			return fmt.Errorf("failed to update asset tags: %w", err)
		}
		if affected != 1 {
			return photo.ErrNotFound
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit asset tag transaction: %w", err)
	}
	return nil
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
		Tags:              []string{},
	}
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &asset.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal asset metadata: %w", err)
		}
	}
	if len(row.Tags) > 0 {
		if err := json.Unmarshal(row.Tags, &asset.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal asset tags: %w", err)
		}
	}
	asset.Tags = normalizeStoredTags(asset.Tags)
	return asset, nil
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

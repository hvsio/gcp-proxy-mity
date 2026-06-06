package database

import (
	"context"
	"fmt"
	"time"

	"gcp-proxy-mity/internal/domain/photo"
	"gcp-proxy-mity/internal/platform/database/generated/dbq"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresAlbumRepository struct {
	q *dbq.Queries
}

func NewPostgresAlbumRepository(pool *pgxpool.Pool) *PostgresAlbumRepository {
	return &PostgresAlbumRepository{q: dbq.New(pool)}
}

func (r *PostgresAlbumRepository) CreateAlbum(ctx context.Context, album *photo.Album) error {
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

func (r *PostgresAlbumRepository) ListAlbums(ctx context.Context) ([]*photo.Album, error) {
	rows, err := r.q.ListPhotoAlbums(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list albums: %w", err)
	}

	albums := make([]*photo.Album, 0, len(rows))
	for _, row := range rows {
		albums = append(albums, &photo.Album{
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

func (r *PostgresAlbumRepository) UpdateAlbum(ctx context.Context, album *photo.Album) error {
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
		return photo.ErrNotFound
	}
	return nil
}

func (r *PostgresAlbumRepository) DeleteAlbum(ctx context.Context, id string) error {
	affected, err := r.q.DeletePhotoAlbum(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete album: %w", err)
	}
	if affected == 0 {
		return photo.ErrNotFound
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

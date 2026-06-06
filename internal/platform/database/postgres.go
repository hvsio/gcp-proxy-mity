package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresService implements the DatabaseService interface using PostgreSQL
type PostgresService struct {
	pool *pgxpool.Pool
}

func (p *PostgresService) CreateAsset(ctx context.Context, asset *Asset) error {
	metadataJSON, err := json.Marshal(asset.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal asset metadata: %w", err)
	}

	query := `
		INSERT INTO photo_assets (id, filename, media_type, mime_type, size, original_object_key, preview_object_key, uploaded_at, metadata, favorite)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = p.pool.Exec(ctx, query,
		asset.ID, asset.Filename, asset.Type, asset.MimeType, asset.Size,
		asset.OriginalObjectKey, asset.PreviewObjectKey, asset.UploadedAt, metadataJSON, asset.Favorite)
	if err != nil {
		return fmt.Errorf("failed to create asset: %w", err)
	}
	return nil
}

func (p *PostgresService) GetAsset(ctx context.Context, id string) (*Asset, error) {
	query := `
		SELECT id, filename, media_type, mime_type, size, original_object_key, preview_object_key, uploaded_at, metadata, favorite
		FROM photo_assets
		WHERE id = $1
	`
	row := p.pool.QueryRow(ctx, query, id)
	asset, err := scanAsset(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}
	return asset, nil
}

func (p *PostgresService) ListAssets(ctx context.Context, limit int, cursor string, albumID string) (*AssetPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	offset, _ := strconv.Atoi(cursor)

	query := `
		SELECT a.id, a.filename, a.media_type, a.mime_type, a.size, a.original_object_key, a.preview_object_key, a.uploaded_at, a.metadata, a.favorite
		FROM photo_assets a
		WHERE ($3 = '' OR EXISTS (
			SELECT 1 FROM photo_album_assets aa WHERE aa.album_id = $3 AND aa.asset_id = a.id
		))
		ORDER BY a.uploaded_at DESC, a.id DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := p.pool.Query(ctx, query, limit+1, offset, albumID)
	if err != nil {
		return nil, fmt.Errorf("failed to list assets: %w", err)
	}
	defer rows.Close()

	assets := make([]*Asset, 0, limit)
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan asset: %w", err)
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate assets: %w", err)
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

func (p *PostgresService) SetAssetFavorite(ctx context.Context, id string, favorite bool) (*Asset, error) {
	query := `
		UPDATE photo_assets
		SET favorite = $2
		WHERE id = $1
		RETURNING id, filename, media_type, mime_type, size, original_object_key, preview_object_key, uploaded_at, metadata, favorite
	`
	asset, err := scanAsset(p.pool.QueryRow(ctx, query, id, favorite))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to set asset favorite: %w", err)
	}
	return asset, nil
}

func (p *PostgresService) CreateAlbum(ctx context.Context, album *Album) error {
	query := `INSERT INTO photo_albums (id, name, cover_emoji, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := p.pool.Exec(ctx, query, album.ID, album.Name, album.CoverEmoji, album.CreatedAt, album.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create album: %w", err)
	}
	return nil
}

func (p *PostgresService) ListAlbums(ctx context.Context) ([]*Album, error) {
	query := `
		SELECT a.id, a.name, a.cover_emoji, a.created_at, a.updated_at, COUNT(aa.asset_id)::int AS asset_count
		FROM photo_albums a
		LEFT JOIN photo_album_assets aa ON aa.album_id = a.id
		GROUP BY a.id
		ORDER BY a.created_at ASC
	`
	rows, err := p.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list albums: %w", err)
	}
	defer rows.Close()

	albums := make([]*Album, 0)
	for rows.Next() {
		var album Album
		if err := rows.Scan(&album.ID, &album.Name, &album.CoverEmoji, &album.CreatedAt, &album.UpdatedAt, &album.AssetCount); err != nil {
			return nil, fmt.Errorf("failed to scan album: %w", err)
		}
		albums = append(albums, &album)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate albums: %w", err)
	}
	return albums, nil
}

func (p *PostgresService) UpdateAlbum(ctx context.Context, album *Album) error {
	query := `UPDATE photo_albums SET name = $2, cover_emoji = $3, updated_at = $4 WHERE id = $1`
	result, err := p.pool.Exec(ctx, query, album.ID, album.Name, album.CoverEmoji, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update album: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresService) DeleteAlbum(ctx context.Context, id string) error {
	result, err := p.pool.Exec(ctx, `DELETE FROM photo_albums WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete album: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresService) AddAssetsToAlbum(ctx context.Context, albumID string, assetIDs []string) error {
	batch := &pgx.Batch{}
	for _, assetID := range assetIDs {
		batch.Queue(`INSERT INTO photo_album_assets (album_id, asset_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, albumID, assetID)
	}
	return p.pool.SendBatch(ctx, batch).Close()
}

func (p *PostgresService) RemoveAssetsFromAlbum(ctx context.Context, albumID string, assetIDs []string) error {
	batch := &pgx.Batch{}
	for _, assetID := range assetIDs {
		batch.Queue(`DELETE FROM photo_album_assets WHERE album_id = $1 AND asset_id = $2`, albumID, assetID)
	}
	return p.pool.SendBatch(ctx, batch).Close()
}

func (p *PostgresService) CreateJob(ctx context.Context, job *Job) error {
	query := `INSERT INTO photo_jobs (id, type, asset_id, state, attempts, error, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := p.pool.Exec(ctx, query, job.ID, job.Type, job.AssetID, job.State, job.Attempts, job.Error, job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	return nil
}

func (p *PostgresService) ListJobs(ctx context.Context, limit int) ([]*Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := p.pool.Query(ctx, `SELECT id, type, asset_id, state, attempts, error, created_at, updated_at FROM photo_jobs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]*Job, 0)
	for rows.Next() {
		var job Job
		if err := rows.Scan(&job.ID, &job.Type, &job.AssetID, &job.State, &job.Attempts, &job.Error, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, &job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate jobs: %w", err)
	}
	return jobs, nil
}

type assetScanner interface {
	Scan(dest ...any) error
}

func scanAsset(row assetScanner) (*Asset, error) {
	var asset Asset
	var metadataJSON []byte
	if err := row.Scan(
		&asset.ID, &asset.Filename, &asset.Type, &asset.MimeType, &asset.Size,
		&asset.OriginalObjectKey, &asset.PreviewObjectKey, &asset.UploadedAt, &metadataJSON, &asset.Favorite,
	); err != nil {
		return nil, err
	}
	asset.Metadata = map[string]any{}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &asset.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal asset metadata: %w", err)
		}
	}
	return &asset, nil
}

// NewPostgresService creates a new PostgreSQL database service
func NewPostgresService(pool *pgxpool.Pool) *PostgresService {
	return &PostgresService{
		pool: pool,
	}
}

// CreateMediaMetadata inserts a new media metadata record
func (p *PostgresService) CreateMediaMetadata(ctx context.Context, metadata *MediaMetadata) error {
	exifJSON, err := json.Marshal(metadata.EXIFData)
	if err != nil {
		return fmt.Errorf("failed to marshal EXIF data: %w", err)
	}

	tagsJSON, err := json.Marshal(metadata.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	query := `
		INSERT INTO media_metadata (id, file_path, file_name, content_type, size, created_at, updated_at, exif_data, tags, user_id, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err = p.pool.Exec(ctx, query,
		metadata.ID, metadata.FilePath, metadata.FileName, metadata.ContentType,
		metadata.Size, metadata.CreatedAt, metadata.UpdatedAt, exifJSON, tagsJSON,
		metadata.UserID, metadata.IsDeleted)

	if err != nil {
		return fmt.Errorf("failed to create media metadata: %w", err)
	}

	return nil
}

// GetMediaMetadata retrieves media metadata by file path and user ID
func (p *PostgresService) GetMediaMetadata(ctx context.Context, filePath string, userID string) (*MediaMetadata, error) {
	query := `
		SELECT id, file_path, file_name, content_type, size, created_at, updated_at, exif_data, tags, user_id, is_deleted
		FROM media_metadata
		WHERE file_path = $1 AND user_id = $2 AND is_deleted = false
	`

	row := p.pool.QueryRow(ctx, query, filePath, userID)

	var metadata MediaMetadata
	var exifJSON, tagsJSON []byte

	err := row.Scan(
		&metadata.ID, &metadata.FilePath, &metadata.FileName, &metadata.ContentType,
		&metadata.Size, &metadata.CreatedAt, &metadata.UpdatedAt, &exifJSON, &tagsJSON,
		&metadata.UserID, &metadata.IsDeleted,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get media metadata: %w", err)
	}

	if len(exifJSON) > 0 {
		if err := json.Unmarshal(exifJSON, &metadata.EXIFData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EXIF data: %w", err)
		}
	}

	if len(tagsJSON) > 0 {
		if err := json.Unmarshal(tagsJSON, &metadata.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}
	}

	return &metadata, nil
}

// UpdateMediaMetadata updates existing media metadata
func (p *PostgresService) UpdateMediaMetadata(ctx context.Context, metadata *MediaMetadata) error {
	exifJSON, err := json.Marshal(metadata.EXIFData)
	if err != nil {
		return fmt.Errorf("failed to marshal EXIF data: %w", err)
	}

	tagsJSON, err := json.Marshal(metadata.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	query := `
		UPDATE media_metadata 
		SET file_name = $2, content_type = $3, size = $4, updated_at = $5, exif_data = $6, tags = $7
		WHERE id = $1 AND user_id = $8 AND is_deleted = false
	`

	result, err := p.pool.Exec(ctx, query,
		metadata.ID, metadata.FileName, metadata.ContentType, metadata.Size,
		time.Now(), exifJSON, tagsJSON, metadata.UserID)

	if err != nil {
		return fmt.Errorf("failed to update media metadata: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteMediaMetadata soft deletes media metadata
func (p *PostgresService) DeleteMediaMetadata(ctx context.Context, filePath string, userID string) error {
	query := `
		UPDATE media_metadata 
		SET is_deleted = true, updated_at = $3
		WHERE file_path = $1 AND user_id = $2 AND is_deleted = false
	`

	result, err := p.pool.Exec(ctx, query, filePath, userID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to delete media metadata: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// ListMediaMetadata retrieves paginated list of media metadata for a user
func (p *PostgresService) ListMediaMetadata(ctx context.Context, userID string, limit, offset int) ([]*MediaMetadata, error) {
	query := `
		SELECT id, file_path, file_name, content_type, size, created_at, updated_at, exif_data, tags, user_id, is_deleted
		FROM media_metadata
		WHERE user_id = $1 AND is_deleted = false
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := p.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list media metadata: %w", err)
	}
	defer rows.Close()

	var results []*MediaMetadata
	for rows.Next() {
		var metadata MediaMetadata
		var exifJSON, tagsJSON []byte

		err := rows.Scan(
			&metadata.ID, &metadata.FilePath, &metadata.FileName, &metadata.ContentType,
			&metadata.Size, &metadata.CreatedAt, &metadata.UpdatedAt, &exifJSON, &tagsJSON,
			&metadata.UserID, &metadata.IsDeleted,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan media metadata: %w", err)
		}

		if len(exifJSON) > 0 {
			if err := json.Unmarshal(exifJSON, &metadata.EXIFData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal EXIF data: %w", err)
			}
		}

		if len(tagsJSON) > 0 {
			if err := json.Unmarshal(tagsJSON, &metadata.Tags); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
			}
		}

		results = append(results, &metadata)
	}

	return results, nil
}

// SearchMediaMetadata searches media metadata by query
func (p *PostgresService) SearchMediaMetadata(ctx context.Context, userID string, query string, limit, offset int) ([]*MediaMetadata, error) {
	searchQuery := `
		SELECT id, file_path, file_name, content_type, size, created_at, updated_at, exif_data, tags, user_id, is_deleted
		FROM media_metadata
		WHERE user_id = $1 AND is_deleted = false 
		AND (file_name ILIKE $2 OR tags::text ILIKE $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	searchTerm := "%" + query + "%"
	rows, err := p.pool.Query(ctx, searchQuery, userID, searchTerm, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search media metadata: %w", err)
	}
	defer rows.Close()

	var results []*MediaMetadata
	for rows.Next() {
		var metadata MediaMetadata
		var exifJSON, tagsJSON []byte

		err := rows.Scan(
			&metadata.ID, &metadata.FilePath, &metadata.FileName, &metadata.ContentType,
			&metadata.Size, &metadata.CreatedAt, &metadata.UpdatedAt, &exifJSON, &tagsJSON,
			&metadata.UserID, &metadata.IsDeleted,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan media metadata: %w", err)
		}

		if len(exifJSON) > 0 {
			if err := json.Unmarshal(exifJSON, &metadata.EXIFData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal EXIF data: %w", err)
			}
		}

		if len(tagsJSON) > 0 {
			if err := json.Unmarshal(tagsJSON, &metadata.Tags); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
			}
		}

		results = append(results, &metadata)
	}

	return results, nil
}

// CreateSignedURL creates a new signed URL record
func (p *PostgresService) CreateSignedURL(ctx context.Context, record *SignedURLRecord) error {
	query := `
		INSERT INTO signed_urls (id, file_path, url, expires_at, created_at, user_id, purpose)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := p.pool.Exec(ctx, query,
		record.ID, record.FilePath, record.URL, record.ExpiresAt,
		record.CreatedAt, record.UserID, record.Purpose)

	if err != nil {
		return fmt.Errorf("failed to create signed URL: %w", err)
	}

	return nil
}

// GetSignedURL retrieves a signed URL by ID
func (p *PostgresService) GetSignedURL(ctx context.Context, id string) (*SignedURLRecord, error) {
	query := `
		SELECT id, file_path, url, expires_at, created_at, user_id, purpose
		FROM signed_urls
		WHERE id = $1 AND expires_at > NOW()
	`

	row := p.pool.QueryRow(ctx, query, id)

	var record SignedURLRecord
	err := row.Scan(
		&record.ID, &record.FilePath, &record.URL, &record.ExpiresAt,
		&record.CreatedAt, &record.UserID, &record.Purpose,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get signed URL: %w", err)
	}

	return &record, nil
}

// CleanupExpiredURLs removes expired signed URLs
func (p *PostgresService) CleanupExpiredURLs(ctx context.Context) error {
	query := `DELETE FROM signed_urls WHERE expires_at < NOW()`

	_, err := p.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired URLs: %w", err)
	}

	return nil
}

// HealthCheck verifies database connectivity
func (p *PostgresService) HealthCheck(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// Close closes the database connection pool
func (p *PostgresService) Close() error {
	p.pool.Close()
	return nil
}

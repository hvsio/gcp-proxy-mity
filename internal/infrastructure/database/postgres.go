package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gcp-proxy-mity/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresService implements the DatabaseService interface using PostgreSQL
type PostgresService struct {
	pool *pgxpool.Pool
}

// NewPostgresService creates a new PostgreSQL database service
func NewPostgresService(pool *pgxpool.Pool) *PostgresService {
	return &PostgresService{
		pool: pool,
	}
}

// CreateMediaMetadata inserts a new media metadata record
func (p *PostgresService) CreateMediaMetadata(ctx context.Context, metadata *service.MediaMetadata) error {
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
func (p *PostgresService) GetMediaMetadata(ctx context.Context, filePath string, userID string) (*service.MediaMetadata, error) {
	query := `
		SELECT id, file_path, file_name, content_type, size, created_at, updated_at, exif_data, tags, user_id, is_deleted
		FROM media_metadata
		WHERE file_path = $1 AND user_id = $2 AND is_deleted = false
	`

	row := p.pool.QueryRow(ctx, query, filePath, userID)

	var metadata service.MediaMetadata
	var exifJSON, tagsJSON []byte

	err := row.Scan(
		&metadata.ID, &metadata.FilePath, &metadata.FileName, &metadata.ContentType,
		&metadata.Size, &metadata.CreatedAt, &metadata.UpdatedAt, &exifJSON, &tagsJSON,
		&metadata.UserID, &metadata.IsDeleted,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, service.ErrNotFound
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
func (p *PostgresService) UpdateMediaMetadata(ctx context.Context, metadata *service.MediaMetadata) error {
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
		return service.ErrNotFound
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
		return service.ErrNotFound
	}

	return nil
}

// ListMediaMetadata retrieves paginated list of media metadata for a user
func (p *PostgresService) ListMediaMetadata(ctx context.Context, userID string, limit, offset int) ([]*service.MediaMetadata, error) {
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

	var results []*service.MediaMetadata
	for rows.Next() {
		var metadata service.MediaMetadata
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
func (p *PostgresService) SearchMediaMetadata(ctx context.Context, userID string, query string, limit, offset int) ([]*service.MediaMetadata, error) {
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

	var results []*service.MediaMetadata
	for rows.Next() {
		var metadata service.MediaMetadata
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
func (p *PostgresService) CreateSignedURL(ctx context.Context, record *service.SignedURLRecord) error {
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
func (p *PostgresService) GetSignedURL(ctx context.Context, id string) (*service.SignedURLRecord, error) {
	query := `
		SELECT id, file_path, url, expires_at, created_at, user_id, purpose
		FROM signed_urls
		WHERE id = $1 AND expires_at > NOW()
	`

	row := p.pool.QueryRow(ctx, query, id)

	var record service.SignedURLRecord
	err := row.Scan(
		&record.ID, &record.FilePath, &record.URL, &record.ExpiresAt,
		&record.CreatedAt, &record.UserID, &record.Purpose,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, service.ErrNotFound
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

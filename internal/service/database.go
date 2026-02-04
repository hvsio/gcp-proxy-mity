package service

import (
	"context"
	"time"
)

// MediaMetadata represents metadata for media files stored in the database
type MediaMetadata struct {
	ID          string            `json:"id"`
	FilePath    string            `json:"file_path"`
	FileName    string            `json:"file_name"`
	ContentType string            `json:"content_type"`
	Size        int64             `json:"size"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	EXIFData    map[string]string `json:"exif_data,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	UserID      string            `json:"user_id"`
	IsDeleted   bool              `json:"is_deleted"`
}

// SignedURLRecord represents a signed URL for temporary file access
type SignedURLRecord struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UserID    string    `json:"user_id"`
	Purpose   string    `json:"purpose"` // "read", "write", "delete"
}

// DatabaseService defines the interface for database operations
type DatabaseService interface {
	// Media Metadata operations
	CreateMediaMetadata(ctx context.Context, metadata *MediaMetadata) error
	GetMediaMetadata(ctx context.Context, filePath string, userID string) (*MediaMetadata, error)
	UpdateMediaMetadata(ctx context.Context, metadata *MediaMetadata) error
	DeleteMediaMetadata(ctx context.Context, filePath string, userID string) error
	ListMediaMetadata(ctx context.Context, userID string, limit, offset int) ([]*MediaMetadata, error)
	SearchMediaMetadata(ctx context.Context, userID string, query string, limit, offset int) ([]*MediaMetadata, error)

	// Signed URL operations
	CreateSignedURL(ctx context.Context, record *SignedURLRecord) error
	GetSignedURL(ctx context.Context, id string) (*SignedURLRecord, error)
	CleanupExpiredURLs(ctx context.Context) error

	// Utility operations
	HealthCheck(ctx context.Context) error
	Close() error
}

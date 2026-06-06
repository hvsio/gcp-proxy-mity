package database

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("record not found")

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

type SignedURLRecord struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UserID    string    `json:"user_id"`
	Purpose   string    `json:"purpose"`
}

type Service interface {
	CreateMediaMetadata(ctx context.Context, metadata *MediaMetadata) error
	GetMediaMetadata(ctx context.Context, filePath string, userID string) (*MediaMetadata, error)
	UpdateMediaMetadata(ctx context.Context, metadata *MediaMetadata) error
	DeleteMediaMetadata(ctx context.Context, filePath string, userID string) error
	ListMediaMetadata(ctx context.Context, userID string, limit, offset int) ([]*MediaMetadata, error)
	SearchMediaMetadata(ctx context.Context, userID string, query string, limit, offset int) ([]*MediaMetadata, error)
	CreateSignedURL(ctx context.Context, record *SignedURLRecord) error
	GetSignedURL(ctx context.Context, id string) (*SignedURLRecord, error)
	CleanupExpiredURLs(ctx context.Context) error
	HealthCheck(ctx context.Context) error
	Close() error
}

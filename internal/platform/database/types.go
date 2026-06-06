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

type Asset struct {
	ID                string         `json:"id"`
	Filename          string         `json:"filename"`
	Type              string         `json:"type"`
	MimeType          string         `json:"mimeType"`
	Size              int64          `json:"size"`
	OriginalObjectKey string         `json:"originalObjectKey"`
	PreviewObjectKey  *string        `json:"previewObjectKey,omitempty"`
	UploadedAt        time.Time      `json:"uploadedAt"`
	Metadata          map[string]any `json:"metadata"`
	Favorite          bool           `json:"favorite"`
}

type AssetPage struct {
	Items      []*Asset `json:"items"`
	NextCursor string   `json:"nextCursor,omitempty"`
	HasMore    bool     `json:"hasMore"`
}

type Album struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CoverEmoji string    `json:"coverEmoji"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	AssetCount int       `json:"assetCount"`
}

type Job struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	AssetID   *string   `json:"assetId,omitempty"`
	State     string    `json:"state"`
	Attempts  int       `json:"attempts"`
	Error     *string   `json:"error,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
	CreateAsset(ctx context.Context, asset *Asset) error
	GetAsset(ctx context.Context, id string) (*Asset, error)
	ListAssets(ctx context.Context, limit int, cursor string, albumID string) (*AssetPage, error)
	SetAssetFavorite(ctx context.Context, id string, favorite bool) (*Asset, error)
	CreateAlbum(ctx context.Context, album *Album) error
	ListAlbums(ctx context.Context) ([]*Album, error)
	UpdateAlbum(ctx context.Context, album *Album) error
	DeleteAlbum(ctx context.Context, id string) error
	AddAssetsToAlbum(ctx context.Context, albumID string, assetIDs []string) error
	RemoveAssetsFromAlbum(ctx context.Context, albumID string, assetIDs []string) error
	CreateJob(ctx context.Context, job *Job) error
	ListJobs(ctx context.Context, limit int) ([]*Job, error)
	HealthCheck(ctx context.Context) error
	Close() error
}

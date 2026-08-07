package photo

import (
	"context"
	"errors"
	"time"
)

var ErrAssetTagLimitExceeded = errors.New("asset tag limit exceeded")

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
	Tags              []string       `json:"tags"`
}

type AssetFilter struct {
	AlbumID  string `json:"albumId,omitempty"`
	Favorite bool   `json:"favorite,omitempty"`
	Tag      string `json:"tag,omitempty"`
}

type AssetPage struct {
	Items      []*Asset `json:"items"`
	NextCursor string   `json:"nextCursor,omitempty"`
	HasMore    bool     `json:"hasMore"`
}

type AssetRepository interface {
	CreateAsset(ctx context.Context, asset *Asset) error
	GetAsset(ctx context.Context, id string) (*Asset, error)
	ListAssets(ctx context.Context, limit int, cursor string, filter AssetFilter) (*AssetPage, error)
	SetAssetFavorite(ctx context.Context, id string, favorite bool) (*Asset, error)
	MutateAssetTags(ctx context.Context, assetIDs []string, add []string, remove []string) error
}

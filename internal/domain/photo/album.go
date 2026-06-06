package photo

import (
	"context"
	"time"
)

type Album struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CoverEmoji string    `json:"coverEmoji"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	AssetCount int       `json:"assetCount"`
}

type AlbumRepository interface {
	CreateAlbum(ctx context.Context, album *Album) error
	ListAlbums(ctx context.Context) ([]*Album, error)
	UpdateAlbum(ctx context.Context, album *Album) error
	DeleteAlbum(ctx context.Context, id string) error
	AddAssetsToAlbum(ctx context.Context, albumID string, assetIDs []string) error
	RemoveAssetsFromAlbum(ctx context.Context, albumID string, assetIDs []string) error
}

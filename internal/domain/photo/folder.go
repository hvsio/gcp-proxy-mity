package photo

import (
	"context"
	"time"
)

type Folder struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ParentID  *string   `json:"parentId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type FolderRepository interface {
	CreateFolder(ctx context.Context, folder *Folder) error
	GetFolder(ctx context.Context, id string) (*Folder, error)
	ListFolders(ctx context.Context) ([]*Folder, error)
	UpdateFolder(ctx context.Context, folder *Folder) error
	DeleteFolder(ctx context.Context, id string) error
}

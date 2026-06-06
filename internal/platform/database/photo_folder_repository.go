package database

import (
	"context"
	"fmt"
	"time"

	"gcp-proxy-mity/internal/domain/photo"
	"gcp-proxy-mity/internal/platform/database/generated/dbq"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresFolderRepository struct {
	q *dbq.Queries
}

func NewPostgresFolderRepository(pool *pgxpool.Pool) *PostgresFolderRepository {
	return &PostgresFolderRepository{q: dbq.New(pool)}
}

func (r *PostgresFolderRepository) CreateFolder(ctx context.Context, folder *photo.Folder) error {
	err := r.q.CreatePhotoFolder(ctx, dbq.CreatePhotoFolderParams{
		ID:        folder.ID,
		Name:      folder.Name,
		ParentID:  folder.ParentID,
		CreatedAt: timestamptz(folder.CreatedAt),
		UpdatedAt: timestamptz(folder.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}
	return nil
}

func (r *PostgresFolderRepository) GetFolder(ctx context.Context, id string) (*photo.Folder, error) {
	row, err := r.q.GetPhotoFolder(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, photo.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}
	return folderFromRow(row), nil
}

func (r *PostgresFolderRepository) ListFolders(ctx context.Context) ([]*photo.Folder, error) {
	rows, err := r.q.ListPhotoFolders(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list folders: %w", err)
	}

	folders := make([]*photo.Folder, 0, len(rows))
	for _, row := range rows {
		folders = append(folders, folderFromRow(row))
	}
	return folders, nil
}

func (r *PostgresFolderRepository) UpdateFolder(ctx context.Context, folder *photo.Folder) error {
	affected, err := r.q.UpdatePhotoFolder(ctx, dbq.UpdatePhotoFolderParams{
		ID:        folder.ID,
		Name:      folder.Name,
		ParentID:  folder.ParentID,
		UpdatedAt: timestamptz(time.Now().UTC()),
	})
	if err != nil {
		return fmt.Errorf("failed to update folder: %w", err)
	}
	if affected == 0 {
		return photo.ErrNotFound
	}
	return nil
}

func (r *PostgresFolderRepository) DeleteFolder(ctx context.Context, id string) error {
	affected, err := r.q.DeletePhotoFolder(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
	}
	if affected == 0 {
		return photo.ErrNotFound
	}
	return nil
}

func folderFromRow(row dbq.PhotoFolder) *photo.Folder {
	return &photo.Folder{
		ID:        row.ID,
		Name:      row.Name,
		ParentID:  row.ParentID,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

package database

import (
	"context"
	"fmt"

	"gcp-proxy-mity/internal/domain/photo"
	"gcp-proxy-mity/internal/platform/database/generated/dbq"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresJobRepository struct {
	q *dbq.Queries
}

func NewPostgresJobRepository(pool *pgxpool.Pool) *PostgresJobRepository {
	return &PostgresJobRepository{q: dbq.New(pool)}
}

func (r *PostgresJobRepository) CreateJob(ctx context.Context, job *photo.Job) error {
	err := r.q.CreatePhotoJob(ctx, dbq.CreatePhotoJobParams{
		ID:        job.ID,
		Type:      job.Type,
		AssetID:   job.AssetID,
		State:     job.State,
		Attempts:  int32(job.Attempts),
		Error:     job.Error,
		CreatedAt: timestamptz(job.CreatedAt),
		UpdatedAt: timestamptz(job.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	return nil
}

func (r *PostgresJobRepository) ListJobs(ctx context.Context, limit int) ([]*photo.Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.q.ListPhotoJobs(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	jobs := make([]*photo.Job, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, &photo.Job{
			ID:        row.ID,
			Type:      row.Type,
			AssetID:   row.AssetID,
			State:     row.State,
			Attempts:  int(row.Attempts),
			Error:     row.Error,
			CreatedAt: row.CreatedAt.Time,
			UpdatedAt: row.UpdatedAt.Time,
		})
	}
	return jobs, nil
}

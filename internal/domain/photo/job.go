package photo

import (
	"context"
	"time"
)

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

type JobRepository interface {
	CreateJob(ctx context.Context, job *Job) error
	ListJobs(ctx context.Context, limit int) ([]*Job, error)
}

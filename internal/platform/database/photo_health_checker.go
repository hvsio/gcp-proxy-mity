package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresHealthChecker struct {
	pool *pgxpool.Pool
}

func NewPostgresHealthChecker(pool *pgxpool.Pool) *PostgresHealthChecker {
	return &PostgresHealthChecker{pool: pool}
}

func (h *PostgresHealthChecker) HealthCheck(ctx context.Context) error {
	return h.pool.Ping(ctx)
}

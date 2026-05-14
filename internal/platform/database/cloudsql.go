package database

import (
	"context"
	"fmt"
	"net"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CloudSQLConfig contains configuration for Google Cloud SQL connection
type CloudSQLConfig struct {
	InstanceConnectionName string // projects/PROJECT_ID/regions/REGION/instances/INSTANCE_ID
	DatabaseName           string
	Username               string
	Password               string
	MaxConnections         int32
	MaxIdleTime            time.Duration
	MaxLifetime            time.Duration
}

// NewCloudSQLPool creates a new connection pool for Google Cloud SQL
func NewCloudSQLPool(ctx context.Context, config CloudSQLConfig) (*pgxpool.Pool, error) {
	// Create a dialer for Cloud SQL
	dialer, err := cloudsqlconn.NewDialer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloud SQL dialer: %w", err)
	}

	// Create a custom dial function for Cloud SQL
	dialFunc := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.Dial(ctx, config.InstanceConnectionName)
	}

	// Build the connection string
	dsn := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable",
		config.Username, config.Password, config.DatabaseName)

	// Parse the connection string and configure the pool
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Set connection pool parameters
	poolConfig.MaxConns = config.MaxConnections
	if config.MaxConnections == 0 {
		poolConfig.MaxConns = 10 // Default value
	}

	poolConfig.MaxConnIdleTime = config.MaxIdleTime
	if config.MaxIdleTime == 0 {
		poolConfig.MaxConnIdleTime = 30 * time.Minute // Default value
	}

	poolConfig.MaxConnLifetime = config.MaxLifetime
	if config.MaxLifetime == 0 {
		poolConfig.MaxConnLifetime = time.Hour // Default value
	}

	// Set the custom dialer
	poolConfig.ConnConfig.DialFunc = dialFunc

	// Create the connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// NewStandardPostgresPool creates a connection pool for standard PostgreSQL (non-Cloud SQL)
func NewStandardPostgresPool(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	if maxConns > 0 {
		poolConfig.MaxConns = maxConns
	} else {
		poolConfig.MaxConns = 10 // Default value
	}

	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// RunMigrations executes the database migrations
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationSQL string) error {
	_, err := pool.Exec(ctx, migrationSQL)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

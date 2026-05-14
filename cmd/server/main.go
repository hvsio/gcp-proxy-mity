package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"gcp-proxy-mity/internal/auth"
	"gcp-proxy-mity/internal/config"
	"gcp-proxy-mity/internal/httpapi"
	"gcp-proxy-mity/internal/platform/database"
	"gcp-proxy-mity/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ready atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if ready.Load() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("READY"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("NOT READY"))
		}
	})

	iapValidator, err := auth.NewIAPValidator(cfg)
	if err != nil {
		log.Fatalf("IAP validator: %v", err)
	}
	if iapValidator != nil {
		log.Println("IAP JWT validation enabled; backend will reject requests without valid X-Goog-IAP-JWT-Assertion")
	}

	iapHandler := auth.WrapWithIAP(iapValidator, mux, []string{"/health", "/ready"})

	var rootHandler http.Handler = iapHandler
	if len(cfg.CORS.AllowedOrigins) > 0 {
		rootHandler = httpapi.CORS(httpapi.CORSConfig{
			AllowedOrigins: cfg.CORS.AllowedOrigins,
		})(iapHandler)
	}

	server := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: rootHandler,
	}

	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Initialize dependencies after the server is listening
	if cfg.Database.Enabled {
		dbPool, err := initializeDatabaseWithRetry(ctx, cfg.Database)
		if err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}
		defer dbPool.Close()

		if err := database.RunMigrations(ctx, dbPool, database.MigrationsSQL); err != nil {
			log.Fatalf("Failed to run database migrations: %v", err)
		}

		_ = database.NewPostgresService(dbPool)
	} else {
		log.Println("Database disabled; skipping database initialization")
	}

	store, err := storage.NewGCSStore(ctx, cfg.Storage.GCSBucketName, cfg.Storage.GoogleCredentials)
	if err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}
	defer store.Close()

	storageHandler := httpapi.NewStorageHandler(store)
	storageHandler.SetupRoutes(mux)

	ready.Store(true)
	log.Println("All dependencies initialized, service is ready")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func initializeDatabaseWithRetry(ctx context.Context, dbConfig config.DatabaseConfig) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error
	for attempt := 1; attempt <= 5; attempt++ {
		pool, err = initializeDatabase(ctx, dbConfig)
		if err == nil {
			return pool, nil
		}
		log.Printf("Database connection attempt %d/5 failed: %v", attempt, err)
		if attempt < 5 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}
	return nil, fmt.Errorf("failed after 5 attempts: %w", err)
}

func initializeDatabase(ctx context.Context, dbConfig config.DatabaseConfig) (*pgxpool.Pool, error) {
	switch dbConfig.Type {
	case "cloudsql":
		config := database.CloudSQLConfig{
			InstanceConnectionName: dbConfig.InstanceConnectionName,
			DatabaseName:           dbConfig.DatabaseName,
			Username:               dbConfig.Username,
			Password:               dbConfig.Password,
			MaxConnections:         dbConfig.MaxConnections,
			MaxIdleTime:            dbConfig.MaxIdleTime,
			MaxLifetime:            dbConfig.MaxLifetime,
		}
		return database.NewCloudSQLPool(ctx, config)

	case "postgres":
		dsn := dbConfig.DSN
		if dsn == "" {
			dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
				dbConfig.Host, dbConfig.Port, dbConfig.Username, dbConfig.Password,
				dbConfig.DatabaseName, dbConfig.SSLMode)
		}
		return database.NewStandardPostgresPool(ctx, dsn, dbConfig.MaxConnections)

	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbConfig.Type)
	}
}

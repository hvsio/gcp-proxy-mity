package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gcp-proxy-mity/internal/config"
	"gcp-proxy-mity/internal/handler"
	"gcp-proxy-mity/internal/infrastructure/database"
	"gcp-proxy-mity/internal/infrastructure/gcs"
	"gcp-proxy-mity/internal/service"
	gcsclient "gcp-proxy-mity/pkg/storage/gcs"
	
	"github.com/jackc/pgx/v5/pgxpool"

	_ "embed"
)

//go:embed migrations.sql
var migrationSQL string

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database
	dbPool, err := initializeDatabase(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dbPool.Close()

	// Run database migrations
	if err := database.RunMigrations(ctx, dbPool, migrationSQL); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Initialize database service
	_ = database.NewPostgresService(dbPool) // TODO: Wire up database service to handlers

	// Initialize GCS client
	gcsClient, err := gcsclient.NewClient(ctx, cfg.Storage.GCPProjectID, cfg.Storage.GCSBucketName, cfg.Storage.GoogleCredentials)
	if err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}
	defer gcsClient.Close()

	gcsStorage := gcs.NewStorage(gcsClient)
	storageService := service.NewStorageService(gcsStorage)
	storageHandler := handler.NewStorageHandler(storageService)

	// Setup routes
	mux := http.NewServeMux()
	storageHandler.SetupRoutes(mux)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: mux,
	}

	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
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

// initializeDatabase creates a database connection pool based on configuration
func initializeDatabase(ctx context.Context, dbConfig config.DatabaseConfig) (*pgxpool.Pool, error) {
	switch dbConfig.Type {
	case "cloudsql":
		config := database.CloudSQLConfig{
			InstanceConnectionName: dbConfig.InstanceConnectionName,
			DatabaseName:           dbConfig.DatabaseName,
			Username:               dbConfig.Username,
			Password:               dbConfig.Password,
			MaxConnections:         dbConfig.MaxConnections,
			MaxIdleTime:           dbConfig.MaxIdleTime,
			MaxLifetime:           dbConfig.MaxLifetime,
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

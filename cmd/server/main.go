package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gcp-proxy-mity/internal/config"
	"gcp-proxy-mity/internal/handler"
	"gcp-proxy-mity/internal/service"
	"gcp-proxy-mity/internal/storage"
	"gcp-proxy-mity/pkg/storage/gcs"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize GCS client
	gcsClient, err := gcs.NewClient(ctx, cfg.GCPProjectID, cfg.GCSBucketName, cfg.GoogleCredentials)
	if err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}
	defer gcsClient.Close()

	gcsStorage := storage.NewGCSStorage(gcsClient)
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
		Addr:    "0.0.0.0:" + cfg.Port,
		Handler: mux,
	}

	go func() {
		log.Printf("Server starting on port %s", cfg.Port)
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

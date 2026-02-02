//go:build acceptance
// +build acceptance

package acceptance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gcp-proxy-mity/internal/config"
	"gcp-proxy-mity/internal/handler"
	"gcp-proxy-mity/internal/infrastructure/gcs"
	"gcp-proxy-mity/internal/service"
	gcsclient "gcp-proxy-mity/pkg/storage/gcs"
)

// harness holds the Given state: app server, client, test file path and content, and cleanup.
type harness struct {
	BaseURL  string
	Client   *http.Client
	FilePath string
	Content  []byte
	Cleanup  func()
}

// givenAppWithRealGCS initializes config, GCS client, and the full app (handler + service + storage),
// starts a test server, and returns the harness. Skips the test if credentials are missing.
func givenAppWithRealGCS(t *testing.T) harness {
	t.Helper()

	cfg := config.Load()
	if cfg.GCPProjectID == "" || cfg.GCSBucketName == "" || cfg.GoogleCredentials == "" {
		t.Skip("Skipping acceptance test: GCP credentials not set (GCP_PROJECT_ID, GCS_BUCKET_NAME, STORAGE_GOOGLE_APPLICATION_CREDENTIALS)")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Given: invalid config: %v", err)
	}

	ctx := context.Background()
	gcsClient, err := gcsclient.NewClient(ctx, cfg.GCPProjectID, cfg.GCSBucketName, cfg.GoogleCredentials)
	if err != nil {
		t.Fatalf("Given: failed to create GCS client: %v", err)
	}

	storage := gcs.NewStorage(gcsClient)
	storageService := service.NewStorageService(storage)
	storageHandler := handler.NewStorageHandler(storageService)
	mux := http.NewServeMux()
	storageHandler.SetupRoutes(mux)
	server := httptest.NewServer(mux)

	filePath := "acceptance-test/sample-file.txt"
	content := []byte("Hello, acceptance test!\nThis is the file content.")

	cleanup := func() {
		server.Close()
		gcsClient.Close()
	}

	return harness{
		BaseURL:  server.URL,
		Client:   server.Client(),
		FilePath: filePath,
		Content:  content,
		Cleanup:  cleanup,
	}
}

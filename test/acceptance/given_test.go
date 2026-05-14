//go:build acceptance
// +build acceptance

package acceptance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"gcp-proxy-mity/internal/config"
	"gcp-proxy-mity/internal/httpapi"
	"gcp-proxy-mity/internal/storage"
)

// harness holds the Given state: app server, client, test file path and content, and cleanup.
type harness struct {
	BaseURL  string
	Client   *http.Client
	FilePath string
	Expected []byte
	Cleanup  func()
}

// givenAppWithRealGCS initializes config, GCS client, and the full app (handler + service + storage),
// starts a test server, and returns the harness. Skips the test if credentials are missing.
func givenAppWithRealGCS(t *testing.T) harness {
	t.Helper()

	cfg := config.Load()
	filePath := os.Getenv("ACCEPTANCE_READ_FILE_PATH")
	if filePath == "" {
		t.Skip("Skipping acceptance test: ACCEPTANCE_READ_FILE_PATH is not set")
	}
	if cfg.Storage.GCPProjectID == "" || cfg.Storage.GCSBucketName == "" || cfg.Storage.GoogleCredentials == "" {
		t.Skip("Skipping acceptance test: GCP credentials not set (GCP_PROJECT_ID, GCS_BUCKET_NAME, GOOGLE_APPLICATION_CREDENTIALS)")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Given: invalid config: %v", err)
	}

	ctx := context.Background()
	store, err := storage.NewGCSStore(ctx, cfg.Storage.GCSBucketName, cfg.Storage.GoogleCredentials)
	if err != nil {
		t.Fatalf("Given: failed to create GCS client: %v", err)
	}

	storageHandler := httpapi.NewStorageHandler(store)
	mux := http.NewServeMux()
	storageHandler.SetupRoutes(mux)
	server := httptest.NewServer(mux)

	cleanup := func() {
		server.Close()
		store.Close()
	}

	return harness{
		BaseURL:  server.URL,
		Client:   server.Client(),
		FilePath: filePath,
		Expected: []byte(os.Getenv("ACCEPTANCE_EXPECTED_CONTENT")),
		Cleanup:  cleanup,
	}
}

//go:build acceptance
// +build acceptance

// Acceptance test that uploads a real file to GCP and verifies it can be retrieved.
// Run with prod credentials:
//   go test -tags=acceptance -v ./test/acceptance/
// Required env (or .env): GCP_PROJECT_ID, GCS_BUCKET_NAME, STORAGE_GOOGLE_APPLICATION_CREDENTIALS (base64-encoded JSON).
package acceptance

import (
	"context"
	"net/http"
	"testing"
)

// deleteTestFile removes the file at the given path from GCP (best-effort cleanup).
func deleteTestFile(baseURL string, client *http.Client, filePath string) {
	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+"/api/v1/storage/files/"+filePath, nil)
	if resp, err := client.Do(req); err == nil {
		resp.Body.Close()
	}
}

func TestUploadAndRetrieveFile_ContentMatches(t *testing.T) {
	// Given: production credentials and application wired with real GCP
	h := givenAppWithRealGCS(t)
	defer h.Cleanup()

	// When: we upload a file and retrieve it via the API
	retrieved := whenFileIsUploadedAndRetrieved(t, h.BaseURL, h.Client, h.FilePath, h.Content)

	// Then: the retrieved content is identical to what was sent
	thenRetrievedContentMatchesOriginal(t, h.Content, retrieved)

	deleteTestFile(h.BaseURL, h.Client, h.FilePath)
}

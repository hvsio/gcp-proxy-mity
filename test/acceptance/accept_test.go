//go:build acceptance
// +build acceptance

// Acceptance test that retrieves a real file from GCP through the read-only API.
// Run with prod credentials:
//
//	go test -tags=acceptance -v ./test/acceptance/
//
// Required env (or .env): GCP_PROJECT_ID, GCS_BUCKET_NAME, GOOGLE_APPLICATION_CREDENTIALS, ACCEPTANCE_READ_FILE_PATH.
package acceptance

import "testing"

func TestRetrieveFile_ContentMatches(t *testing.T) {
	// Given: production credentials and application wired with real GCP
	h := givenAppWithRealGCS(t)
	defer h.Cleanup()

	// When: we retrieve a file via the API
	retrieved := whenFileIsRetrieved(t, h.BaseURL, h.Client, h.FilePath)

	// Then: the retrieved content is identical to the optional expected value
	thenRetrievedContentMatchesExpected(t, h.Expected, retrieved)
}

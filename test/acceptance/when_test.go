//go:build acceptance
// +build acceptance

package acceptance

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
)

// whenFileIsUploadedAndRetrieved uploads the given content to the path via PUT, then retrieves it via GET.
// Returns the retrieved body; fails the test on request or status errors.
func whenFileIsUploadedAndRetrieved(t *testing.T, baseURL string, client *http.Client, filePath string, content []byte) []byte {
	t.Helper()

	ctx := context.Background()

	putURL := baseURL + "/api/v1/storage/files/" + filePath
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("When (upload): failed to create request: %v", err)
	}
	putReq.Header.Set("Content-Type", "text/plain")

	putResp, err := client.Do(putReq)
	if err != nil {
		t.Fatalf("When (upload): request failed: %v", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(putResp.Body)
		t.Fatalf("When (upload): expected 200, got %d: %s", putResp.StatusCode, string(body))
	}

	getURL := baseURL + "/api/v1/storage/files/" + filePath
	getResp, err := client.Get(getURL)
	if err != nil {
		t.Fatalf("When (retrieve): request failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		t.Fatalf("When (retrieve): expected 200, got %d: %s", getResp.StatusCode, string(body))
	}

	gotContent, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("When (retrieve): failed to read body: %v", err)
	}

	return gotContent
}

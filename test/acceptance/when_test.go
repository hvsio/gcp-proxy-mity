//go:build acceptance
// +build acceptance

package acceptance

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func whenFileIsRetrieved(t *testing.T, baseURL string, client *http.Client, filePath string) []byte {
	t.Helper()

	ctx := context.Background()

	getURL := baseURL + "/api/v1/storage/files/" + filePath
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		t.Fatalf("When (retrieve): failed to create request: %v", err)
	}
	getResp, err := client.Do(getReq)
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

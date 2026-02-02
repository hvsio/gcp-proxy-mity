//go:build acceptance
// +build acceptance

package acceptance

import (
	"bytes"
	"testing"
)

// thenRetrievedContentMatchesOriginal asserts that the retrieved bytes equal the original content.
func thenRetrievedContentMatchesOriginal(t *testing.T, original, retrieved []byte) {
	t.Helper()

	if !bytes.Equal(retrieved, original) {
		t.Errorf("Then: content mismatch: sent %d bytes %q, got %d bytes %q",
			len(original), string(original), len(retrieved), string(retrieved))
	}
}

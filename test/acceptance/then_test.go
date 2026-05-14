//go:build acceptance
// +build acceptance

package acceptance

import (
	"bytes"
	"testing"
)

func thenRetrievedContentMatchesExpected(t *testing.T, expected, retrieved []byte) {
	t.Helper()

	if len(expected) == 0 {
		return
	}
	if !bytes.Equal(retrieved, expected) {
		t.Errorf("Then: content mismatch: expected %d bytes %q, got %d bytes %q",
			len(expected), string(expected), len(retrieved), string(retrieved))
	}
}

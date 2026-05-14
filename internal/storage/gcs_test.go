package storage

import (
	"errors"
	"testing"

	cloudstorage "cloud.google.com/go/storage"
)

func TestMapGCSErrorMapsMissingObject(t *testing.T) {
	err := mapGCSError("open", cloudstorage.ErrObjectNotExist)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMapGCSErrorWrapsOtherErrors(t *testing.T) {
	source := errors.New("boom")
	err := mapGCSError("open", source)

	if !errors.Is(err, source) {
		t.Fatalf("expected source error to be wrapped, got %v", err)
	}
}

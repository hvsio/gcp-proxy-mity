package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrapWithIAPSkipsConfiguredPaths(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := WrapWithIAP(&IAPValidator{}, next, []string{"/health"})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected skipped path to pass, got %d", rec.Code)
	}
}

func TestWrapWithIAPRejectsUnskippedPaths(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := WrapWithIAP(&IAPValidator{}, next, []string{"/health"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/files/a.txt", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", rec.Code)
	}
}

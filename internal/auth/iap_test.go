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
	if got := rec.Header().Get("X-IAP-Reject-Reason"); got != "missing_jwt" {
		t.Fatalf("expected missing_jwt reject reason, got %q", got)
	}
}

func TestIAPRejectReasonClassifiesValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "missing JWT", err: errMissingJWT, want: "missing_jwt"},
		{name: "claims validation", err: errValidateClaims, want: "validate_claims"},
		{name: "email not allowed", err: errEmailNotAllowed, want: "email_not_allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := iapRejectReason(tt.err); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

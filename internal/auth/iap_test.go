package auth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gcp-proxy-mity/internal/config"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
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

func TestNewIAPValidatorRejectsHalfConfiguredIAP(t *testing.T) {
	tests := []struct {
		name string
		iap  config.IAPConfig
		want error
	}{
		{
			name: "missing audience",
			iap: config.IAPConfig{
				AllowedEmails: []string{"owner@example.com"},
			},
			want: config.ErrMissingIAPAudience,
		},
		{
			name: "missing allowlist",
			iap: config.IAPConfig{
				Audience: "/projects/123/global/backendServices/456",
			},
			want: config.ErrMissingIAPAllowedEmails,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewIAPValidator(&config.Config{IAP: tt.iap})
			if err != tt.want {
				t.Fatalf("NewIAPValidator() error = %v, want %v", err, tt.want)
			}
			if validator != nil {
				t.Fatalf("expected nil validator, got %#v", validator)
			}
		})
	}
}

func TestIAPValidatorAcceptsSignedAllowedOwner(t *testing.T) {
	key := testIAPPrivateKey()
	jwksURL := "https://iap.test/jwks"

	validator, err := NewIAPValidator(&config.Config{
		IAP: config.IAPConfig{
			Audience:      "/projects/123/global/backendServices/456",
			AllowedEmails: []string{" Mock.Owner@Example.com "},
		},
	})
	if err != nil {
		t.Fatalf("NewIAPValidator() error = %v", err)
	}
	validator.jwksURL = jwksURL
	validator.httpClient = newTestJWKSClient(t, key, jwksURL)

	rawJWT := signedTestJWT(t, key, signedTestJWTOptions{
		audience: validator.audience,
		email:    "MOCK.OWNER@EXAMPLE.COM",
		issuer:   iapIssuer,
	})

	email, err := validator.Validate(context.Background(), rawJWT)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if email != "mock.owner@example.com" {
		t.Fatalf("Validate() email = %q, want %q", email, "mock.owner@example.com")
	}
}

func TestIAPValidatorRejectsStableIdentityFailures(t *testing.T) {
	key := testIAPPrivateKey()
	jwksURL := "https://iap.test/jwks"

	validator, err := NewIAPValidator(&config.Config{
		IAP: config.IAPConfig{
			Audience:      "/projects/123/global/backendServices/456",
			AllowedEmails: []string{"mock.owner@example.com"},
		},
	})
	if err != nil {
		t.Fatalf("NewIAPValidator() error = %v", err)
	}
	validator.jwksURL = jwksURL
	validator.httpClient = newTestJWKSClient(t, key, jwksURL)

	tests := []struct {
		name   string
		rawJWT string
		want   error
		reason string
	}{
		{
			name:   "missing jwt",
			rawJWT: "",
			want:   errMissingJWT,
			reason: "missing_jwt",
		},
		{
			name:   "malformed jwt",
			rawJWT: "not-a-jwt",
			want:   errParseJWT,
			reason: "parse_jwt",
		},
		{
			name: "wrong issuer",
			rawJWT: signedTestJWT(t, key, signedTestJWTOptions{
				audience: validator.audience,
				email:    "mock.owner@example.com",
				issuer:   "https://example.com/not-iap",
			}),
			want:   errValidateClaims,
			reason: "validate_claims",
		},
		{
			name: "wrong audience",
			rawJWT: signedTestJWT(t, key, signedTestJWTOptions{
				audience: "/projects/123/global/backendServices/not-this-one",
				email:    "mock.owner@example.com",
				issuer:   iapIssuer,
			}),
			want:   errValidateClaims,
			reason: "validate_claims",
		},
		{
			name: "missing email",
			rawJWT: signedTestJWT(t, key, signedTestJWTOptions{
				audience: validator.audience,
				issuer:   iapIssuer,
			}),
			want:   errMissingEmail,
			reason: "missing_email",
		},
		{
			name: "disallowed email",
			rawJWT: signedTestJWT(t, key, signedTestJWTOptions{
				audience: validator.audience,
				email:    "someone.else@example.com",
				issuer:   iapIssuer,
			}),
			want:   errEmailNotAllowed,
			reason: "email_not_allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.Validate(context.Background(), tt.rawJWT)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.want)
			}
			if got := iapRejectReason(err); got != tt.reason {
				t.Fatalf("iapRejectReason() = %q, want %q", got, tt.reason)
			}
		})
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

type signedTestJWTOptions struct {
	audience string
	email    string
	issuer   string
}

type signedTestJWTExtendedClaims struct {
	Email string `json:"email,omitempty"`
}

func signedTestJWT(t *testing.T, key *ecdsa.PrivateKey, opts signedTestJWTOptions) string {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.ES256,
			Key: jose.JSONWebKey{
				Key:       key,
				KeyID:     "test-iap-key",
				Use:       "sig",
				Algorithm: string(jose.ES256),
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("NewSigner() error = %v", err)
	}

	issuedAt := time.Date(2024, time.July, 3, 0, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC)
	builder := jwt.Signed(signer).Claims(jwt.Claims{
		Issuer:    opts.issuer,
		Audience:  jwt.Audience{opts.audience},
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		NotBefore: jwt.NewNumericDate(issuedAt.Add(-time.Minute)),
		Expiry:    jwt.NewNumericDate(expiresAt),
	})
	if opts.email != "" {
		builder = builder.Claims(signedTestJWTExtendedClaims{Email: opts.email})
	}

	rawJWT, err := builder.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}
	return rawJWT
}

func newTestJWKSClient(t *testing.T, key *ecdsa.PrivateKey, jwksURL string) *http.Client {
	t.Helper()

	body, err := json.Marshal(jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       &key.PublicKey,
				KeyID:     "test-iap-key",
				Use:       "sig",
				Algorithm: string(jose.ES256),
			},
		},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("unexpected method %q", req.Method)
			}
			if req.URL.String() != jwksURL {
				t.Fatalf("unexpected url %q", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body:       io.NopCloser(bytes.NewReader(body)),
				Request:    req,
				Status:     "200 OK",
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
			}, nil
		}),
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testIAPPrivateKey() *ecdsa.PrivateKey {
	curve := elliptic.P256()
	d := new(big.Int).SetBytes([]byte{
		0x14, 0x7d, 0x4c, 0xc4, 0x37, 0xbb, 0x9f, 0x68,
		0x5d, 0x34, 0x2d, 0x96, 0x84, 0x73, 0xd8, 0x11,
		0x33, 0x91, 0x56, 0x25, 0x57, 0x4a, 0x0f, 0x42,
		0x18, 0x6a, 0x74, 0xe5, 0xb8, 0xf1, 0x2c, 0x19,
	})
	x, y := curve.ScalarBaseMult(d.Bytes())
	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		},
		D: d,
	}
}

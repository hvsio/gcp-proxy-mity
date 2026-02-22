// Package middleware provides IAP JWT validation so the backend only accepts
// requests that carry a valid Identity-Aware Proxy assertion (e.g. forwarded
// by the frontend). Rejects requests without valid IAP identity.
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"gcp-proxy-mity/internal/config"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

var errBadJWKS = errors.New("failed to fetch or parse IAP JWKS")

const (
	iapJWTHeader   = "X-Goog-IAP-JWT-Assertion"
	iapJWKSURL     = "https://www.gstatic.com/iap/verify/public_key-jwk"
	iapIssuer      = "https://cloud.google.com/iap"
	iapJWKSCacheTTL = 5 * time.Minute
)

// IAPValidator validates IAP JWTs and enforces allowed emails.
type IAPValidator struct {
	audience      string
	allowedEmails map[string]struct{}
	jwksURL      string
	issuer       string
	mu           sync.RWMutex
	jwks         *jose.JSONWebKeySet
	jwksExpiry   time.Time
}

// NewIAPValidator builds an IAP validator from config. If cfg.IAP.Audience is
// empty, validation is disabled (returns nil and no middleware should be applied).
func NewIAPValidator(cfg *config.Config) (*IAPValidator, error) {
	if cfg.IAP.Audience == "" || len(cfg.IAP.AllowedEmails) == 0 {
		return nil, nil
	}
	allowed := make(map[string]struct{})
	for _, e := range cfg.IAP.AllowedEmails {
		allowed[strings.TrimSpace(strings.ToLower(e))] = struct{}{}
	}
	return &IAPValidator{
		audience:      cfg.IAP.Audience,
		allowedEmails: allowed,
		jwksURL:      iapJWKSURL,
		issuer:       iapIssuer,
	}, nil
}

func (v *IAPValidator) fetchJWKS(ctx context.Context) (*jose.JSONWebKeySet, error) {
	v.mu.RLock()
	if v.jwks != nil && time.Now().Before(v.jwksExpiry) {
		ks := v.jwks
		v.mu.RUnlock()
		return ks, nil
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.jwks != nil && time.Now().Before(v.jwksExpiry) {
		return v.jwks, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errBadJWKS
	}

	var raw struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	keySet := &jose.JSONWebKeySet{}
	for _, k := range raw.Keys {
		var jwk jose.JSONWebKey
		if err := jwk.UnmarshalJSON(k); err != nil {
			continue
		}
		keySet.Keys = append(keySet.Keys, jwk)
	}
	v.jwks = keySet
	v.jwksExpiry = time.Now().Add(iapJWKSCacheTTL)
	return v.jwks, nil
}

// Validate extracts X-Goog-IAP-JWT-Assertion, verifies signature and claims
// (issuer, audience, exp), and ensures the email claim is in the allowed list.
// Returns the verified email and nil on success; otherwise an error and empty string.
func (v *IAPValidator) Validate(ctx context.Context, rawJWT string) (email string, err error) {
	if v == nil || rawJWT == "" {
		return "", http.ErrNoCookie
	}

	keySet, err := v.fetchJWKS(ctx)
	if err != nil {
		return "", err
	}

	tok, err := jwt.ParseSigned(rawJWT, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		return "", err
	}

	var claims jwt.Claims
	var extra struct {
		Email string `json:"email"`
	}
	if err := tok.Claims(keySet, &claims, &extra); err != nil {
		return "", err
	}

	if err := claims.Validate(jwt.Expected{
		Issuer:       v.issuer,
		AnyAudience:  jwt.Audience{v.audience},
		Time:         time.Now(),
	}); err != nil {
		return "", err
	}

	email = strings.TrimSpace(strings.ToLower(extra.Email))
	if email == "" {
		return "", http.ErrNoCookie
	}
	if _, ok := v.allowedEmails[email]; !ok {
		return "", http.ErrNoCookie
	}
	return email, nil
}

// RequireIAP returns an http.Handler that only calls next if the request has a
// valid IAP JWT (X-Goog-IAP-JWT-Assertion) with an allowed email. Otherwise
// responds with 401. If validator is nil, next is always called (IAP disabled).
func RequireIAP(validator *IAPValidator, next http.Handler) http.Handler {
	if validator == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get(iapJWTHeader)
		email, err := validator.Validate(r.Context(), raw)
		if err != nil {
			log.Printf("iap: reject %s %s: %v", r.Method, r.URL.Path, err)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Missing or invalid IAP identity"))
			return
		}
		// Optional: set header for downstream (e.g. logging)
		r.Header.Set("X-IAP-Email", email)
		next.ServeHTTP(w, r)
	})
}

// WrapWithIAP wraps next so that IAP is required for all requests except paths in skipPaths.
// Use skipPaths like []string{"/health", "/ready"} so probes and LB health checks are not blocked.
func WrapWithIAP(validator *IAPValidator, next http.Handler, skipPaths []string) http.Handler {
	skip := make(map[string]struct{})
	for _, p := range skipPaths {
		skip[p] = struct{}{}
	}
	if validator == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := skip[r.URL.Path]; ok {
			next.ServeHTTP(w, r)
			return
		}
		RequireIAP(validator, next).ServeHTTP(w, r)
	})
}

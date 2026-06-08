package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"gcp-proxy-mity/internal/config"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

var (
	errBadJWKS            = errors.New("failed to fetch or parse IAP JWKS")
	errMissingJWT         = errors.New("missing_iap_jwt")
	errFetchJWKS          = errors.New("fetch_iap_jwks")
	errParseJWT           = errors.New("parse_iap_jwt")
	errReadClaims         = errors.New("read_iap_claims")
	errValidateClaims     = errors.New("validate_iap_claims")
	errMissingEmail       = errors.New("missing_iap_email")
	errEmailNotAllowed    = errors.New("iap_email_not_allowed")
	errValidatorUnenabled = errors.New("iap_validator_unenabled")
)

const (
	iapJWTHeader    = "X-Goog-IAP-JWT-Assertion"
	iapJWKSURL      = "https://www.gstatic.com/iap/verify/public_key-jwk"
	iapIssuer       = "https://cloud.google.com/iap"
	iapJWKSCacheTTL = 5 * time.Minute
)

type IAPValidator struct {
	audience      string
	allowedEmails map[string]struct{}
	jwksURL       string
	issuer        string
	mu            sync.RWMutex
	jwks          *jose.JSONWebKeySet
	jwksExpiry    time.Time
}

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
		jwksURL:       iapJWKSURL,
		issuer:        iapIssuer,
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

func (v *IAPValidator) Validate(ctx context.Context, rawJWT string) (email string, err error) {
	if v == nil {
		return "", errValidatorUnenabled
	}
	if rawJWT == "" {
		return "", errMissingJWT
	}

	keySet, err := v.fetchJWKS(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errFetchJWKS, err)
	}

	tok, err := jwt.ParseSigned(rawJWT, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		return "", fmt.Errorf("%w: %v", errParseJWT, err)
	}

	var claims jwt.Claims
	var extra struct {
		Email string `json:"email"`
	}
	if err := tok.Claims(keySet, &claims, &extra); err != nil {
		return "", fmt.Errorf("%w: %v", errReadClaims, err)
	}

	if err := claims.Validate(jwt.Expected{
		Issuer:      v.issuer,
		AnyAudience: jwt.Audience{v.audience},
		Time:        time.Now(),
	}); err != nil {
		return "", fmt.Errorf("%w: %v", errValidateClaims, err)
	}

	email = strings.TrimSpace(strings.ToLower(extra.Email))
	if email == "" {
		return "", errMissingEmail
	}
	if _, ok := v.allowedEmails[email]; !ok {
		return "", errEmailNotAllowed
	}
	return email, nil
}

func iapRejectReason(err error) string {
	switch {
	case errors.Is(err, errMissingJWT):
		return "missing_jwt"
	case errors.Is(err, errFetchJWKS):
		return "fetch_jwks"
	case errors.Is(err, errParseJWT):
		return "parse_jwt"
	case errors.Is(err, errReadClaims):
		return "read_claims"
	case errors.Is(err, errValidateClaims):
		return "validate_claims"
	case errors.Is(err, errMissingEmail):
		return "missing_email"
	case errors.Is(err, errEmailNotAllowed):
		return "email_not_allowed"
	case errors.Is(err, errValidatorUnenabled):
		return "validator_unenabled"
	default:
		return "unknown"
	}
}

func RequireIAP(validator *IAPValidator, next http.Handler) http.Handler {
	if validator == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get(iapJWTHeader)
		email, err := validator.Validate(r.Context(), raw)
		if err != nil {
			reason := iapRejectReason(err)
			log.Printf(
				"iap: reject method=%s path=%s reason=%s jwt_present=%t authenticated_user_email_present=%t error=%v",
				r.Method,
				r.URL.Path,
				reason,
				raw != "",
				r.Header.Get("X-Goog-Authenticated-User-Email") != "",
				err,
			)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-IAP-Reject-Reason", reason)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Missing or invalid IAP identity"))
			return
		}
		r.Header.Set("X-IAP-Email", email)
		next.ServeHTTP(w, r)
	})
}

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

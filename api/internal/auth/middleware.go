// Package auth verifies OIDC bearer tokens and exposes caller identity.
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

type contextKey struct{}

// Authenticator validates bearer tokens against an OIDC issuer.
type Authenticator struct {
	verifier *oidc.IDTokenVerifier
	clientID string
}

// New discovers the issuer and prepares a token verifier.
// Audience is checked manually (aud contains clientID, or azp == clientID)
// because Keycloak access tokens often carry azp instead of aud.
func New(ctx context.Context, issuerURL, clientID string) (*Authenticator, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, err
	}
	verifier := provider.Verifier(&oidc.Config{SkipClientIDCheck: true})
	return &Authenticator{verifier: verifier, clientID: clientID}, nil
}

// Middleware rejects requests without a valid bearer token and stores the
// caller's Claims in the request context.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := a.authenticate(r)
		if err != nil {
			writeUnauthorized(w, err.Error())
			return
		}
		next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
	})
}

func (a *Authenticator) authenticate(r *http.Request) (*Claims, error) {
	rawToken, err := bearerToken(r)
	if err != nil {
		return nil, err
	}
	token, err := a.verifier.Verify(r.Context(), rawToken)
	if err != nil {
		return nil, errInvalidToken
	}
	if !a.audienceAllowed(token) {
		return nil, errInvalidAudience
	}
	var payload json.RawMessage
	if err := token.Claims(&payload); err != nil {
		return nil, errInvalidToken
	}
	return ParseClaims(payload)
}

func (a *Authenticator) audienceAllowed(token *oidc.IDToken) bool {
	if slices.Contains(token.Audience, a.clientID) {
		return true
	}
	var extra struct {
		Azp string `json:"azp"`
	}
	if err := token.Claims(&extra); err != nil {
		return false
	}
	return extra.Azp == a.clientID
}

type authError string

func (e authError) Error() string { return string(e) }

const (
	errMissingToken    = authError("missing bearer token")
	errInvalidToken    = authError("invalid token")
	errInvalidAudience = authError("token not issued for this client")
)

func bearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", errMissingToken
	}
	return strings.TrimSpace(token), nil
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// WithClaims stores claims in a context.
func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

// FromContext retrieves the caller's claims, if present.
func FromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(contextKey{}).(*Claims)
	return c, ok
}

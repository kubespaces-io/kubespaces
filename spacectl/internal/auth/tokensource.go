package auth

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"
)

// ErrNotLoggedIn is returned when no usable credentials exist.
var ErrNotLoggedIn = errors.New("not logged in — run 'spacectl login'")

// ErrSessionExpired is returned when the cached tokens can no longer be
// refreshed.
var ErrSessionExpired = errors.New("session expired — run 'spacectl login'")

// expirySkew renews access tokens slightly before their actual expiry.
const expirySkew = 30 * time.Second

// TokenSource yields a valid access token from the on-disk cache, silently
// refreshing it via the refresh_token grant when it is (about to be) expired.
type TokenSource struct {
	CredentialsPath string
	Issuer          string
	ClientID        string
	HTTP            *http.Client
	Now             func() time.Time
}

func (ts *TokenSource) now() time.Time {
	if ts.Now != nil {
		return ts.Now()
	}
	return time.Now()
}

// Token returns a valid access token or ErrNotLoggedIn / ErrSessionExpired.
func (ts *TokenSource) Token(ctx context.Context) (string, error) {
	creds, err := LoadCredentials(ts.CredentialsPath)
	if os.IsNotExist(err) {
		return "", ErrNotLoggedIn
	}
	if err != nil {
		return "", err
	}
	if creds.AccessToken != "" && ts.now().Before(creds.Expiry.Add(-expirySkew)) {
		return creds.AccessToken, nil
	}
	return ts.refresh(ctx, creds)
}

func (ts *TokenSource) refresh(ctx context.Context, creds *Credentials) (string, error) {
	if creds.RefreshToken == "" || ts.Issuer == "" {
		return "", ErrSessionExpired
	}
	eps, err := Discover(ctx, ts.HTTP, ts.Issuer)
	if err != nil {
		return "", err
	}
	flow := &Flow{HTTP: ts.HTTP, ClientID: ts.ClientID, Now: ts.Now}
	tok, err := flow.Refresh(ctx, eps.TokenEndpoint, creds.RefreshToken)
	if err != nil {
		return "", ErrSessionExpired
	}
	fresh := flow.Credentials(tok)
	if fresh.RefreshToken == "" {
		fresh.RefreshToken = creds.RefreshToken
	}
	if err := SaveCredentials(ts.CredentialsPath, fresh); err != nil {
		return "", err
	}
	return fresh.AccessToken, nil
}

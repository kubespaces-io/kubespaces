package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeIssuer serves OIDC discovery plus a scripted token endpoint.
func fakeIssuer(t *testing.T, tokenHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"device_authorization_endpoint": srv.URL + "/device",
			"token_endpoint":                srv.URL + "/token",
		})
	})
	mux.HandleFunc("/token", tokenHandler)
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestTokenSourceReturnsValidCachedToken(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "credentials.json")
	creds := &Credentials{AccessToken: "cached", Expiry: time.Now().Add(time.Hour)}
	if err := SaveCredentials(path, creds); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	ts := &TokenSource{CredentialsPath: path}

	// Act
	tok, err := ts.Token(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if tok != "cached" {
		t.Errorf("Token() = %q, want cached", tok)
	}
}

func TestTokenSourceRefreshesExpiredToken(t *testing.T) {
	// Arrange
	srv := fakeIssuer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing form: %v", err)
		}
		if r.PostForm.Get("refresh_token") != "rt" {
			t.Errorf("refresh_token = %q", r.PostForm.Get("refresh_token"))
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "fresh", "refresh_token": "rt2", "expires_in": 300})
	})
	path := filepath.Join(t.TempDir(), "credentials.json")
	expired := &Credentials{AccessToken: "stale", RefreshToken: "rt", Expiry: time.Now().Add(-time.Minute)}
	if err := SaveCredentials(path, expired); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	ts := &TokenSource{CredentialsPath: path, Issuer: srv.URL, ClientID: "kubespaces", HTTP: srv.Client()}

	// Act
	tok, err := ts.Token(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if tok != "fresh" {
		t.Errorf("Token() = %q, want fresh", tok)
	}
	saved, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if saved.AccessToken != "fresh" || saved.RefreshToken != "rt2" {
		t.Errorf("saved credentials = %+v", saved)
	}
}

func TestTokenSourceNotLoggedIn(t *testing.T) {
	// Arrange
	ts := &TokenSource{CredentialsPath: filepath.Join(t.TempDir(), "missing.json")}

	// Act
	_, err := ts.Token(context.Background())

	// Assert
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Errorf("Token() error = %v, want ErrNotLoggedIn", err)
	}
}

func TestTokenSourceRefreshFailure(t *testing.T) {
	// Arrange
	srv := fakeIssuer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
	})
	path := filepath.Join(t.TempDir(), "credentials.json")
	expired := &Credentials{AccessToken: "stale", RefreshToken: "rt", Expiry: time.Now().Add(-time.Minute)}
	if err := SaveCredentials(path, expired); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	ts := &TokenSource{CredentialsPath: path, Issuer: srv.URL, ClientID: "kubespaces", HTTP: srv.Client()}

	// Act
	_, err := ts.Token(context.Background())

	// Assert
	if !errors.Is(err, ErrSessionExpired) {
		t.Errorf("Token() error = %v, want ErrSessionExpired", err)
	}
}

func TestSaveCredentialsPermissions(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "credentials.json")

	// Act
	err := SaveCredentials(path, &Credentials{AccessToken: "at"})

	// Assert
	if err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials mode = %o, want 600", perm)
	}
}

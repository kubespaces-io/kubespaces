package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	deviceGrantType     = "urn:ietf:params:oauth:grant-type:device_code"
	refreshGrantType    = "refresh_token"
	defaultScope        = "openid email profile"
	defaultPollInterval = 5 * time.Second
	slowDownIncrement   = 5 * time.Second
)

// DeviceAuthorization is the response of the device authorization endpoint
// (RFC 8628 section 3.2).
type DeviceAuthorization struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// Token is a token endpoint response.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// Flow drives the OIDC device authorization grant. Sleep and Now are
// injectable for tests; zero values fall back to the real clock.
type Flow struct {
	HTTP     *http.Client
	ClientID string
	Sleep    func(time.Duration)
	Now      func() time.Time
}

func (f *Flow) sleep(d time.Duration) {
	if f.Sleep != nil {
		f.Sleep(d)
		return
	}
	time.Sleep(d)
}

func (f *Flow) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
}

// Start requests a device code from the device authorization endpoint.
func (f *Flow) Start(ctx context.Context, deviceEndpoint string) (*DeviceAuthorization, error) {
	form := url.Values{
		"client_id": {f.ClientID},
		"scope":     {defaultScope},
	}
	body, err := postForm(ctx, f.HTTP, deviceEndpoint, form)
	if err != nil {
		return nil, fmt.Errorf("device authorization request: %w", err)
	}
	da := &DeviceAuthorization{}
	if err := json.Unmarshal(body, da); err != nil {
		return nil, fmt.Errorf("parsing device authorization response: %w", err)
	}
	if da.DeviceCode == "" {
		return nil, fmt.Errorf("device authorization response missing device_code")
	}
	return da, nil
}

// Poll polls the token endpoint per RFC 8628 until the user approves the
// device, the code expires, or ctx is cancelled.
func (f *Flow) Poll(ctx context.Context, tokenEndpoint string, da *DeviceAuthorization) (*Token, error) {
	interval := defaultPollInterval
	if da.Interval > 0 {
		interval = time.Duration(da.Interval) * time.Second
	}
	deadline := f.now().Add(time.Duration(da.ExpiresIn) * time.Second)

	form := url.Values{
		"grant_type":  {deviceGrantType},
		"device_code": {da.DeviceCode},
		"client_id":   {f.ClientID},
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if da.ExpiresIn > 0 && f.now().After(deadline) {
			return nil, fmt.Errorf("device code expired before the login was approved")
		}
		tok, oidcErr, err := requestToken(ctx, f.HTTP, tokenEndpoint, form)
		if err != nil {
			return nil, err
		}
		if tok != nil {
			return tok, nil
		}
		switch oidcErr.Code {
		case "authorization_pending":
			// keep polling at the current interval
		case "slow_down":
			interval += slowDownIncrement
		case "expired_token":
			return nil, fmt.Errorf("device code expired before the login was approved")
		default:
			return nil, fmt.Errorf("token endpoint error: %s", oidcErr.message())
		}
		f.sleep(interval)
	}
}

// Refresh exchanges a refresh token for a new token set.
func (f *Flow) Refresh(ctx context.Context, tokenEndpoint, refreshToken string) (*Token, error) {
	form := url.Values{
		"grant_type":    {refreshGrantType},
		"refresh_token": {refreshToken},
		"client_id":     {f.ClientID},
	}
	tok, oidcErr, err := requestToken(ctx, f.HTTP, tokenEndpoint, form)
	if err != nil {
		return nil, err
	}
	if tok == nil {
		return nil, fmt.Errorf("refresh failed: %s", oidcErr.message())
	}
	return tok, nil
}

// Credentials converts a Token into cacheable Credentials.
func (f *Flow) Credentials(tok *Token) *Credentials {
	return &Credentials{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       f.now().Add(time.Duration(tok.ExpiresIn) * time.Second),
	}
}

type oidcError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *oidcError) message() string {
	if e.Description != "" {
		return fmt.Sprintf("%s (%s)", e.Code, e.Description)
	}
	return e.Code
}

// requestToken posts to the token endpoint. A 2xx yields a Token; a 4xx with
// an OAuth error body yields an oidcError; anything else is a hard error.
func requestToken(ctx context.Context, httpClient *http.Client, endpoint string, form url.Values) (*Token, *oidcError, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, fmt.Errorf("building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("reading token response: %w", err)
	}
	if resp.StatusCode == http.StatusOK {
		tok := &Token{}
		if err := json.Unmarshal(body, tok); err != nil {
			return nil, nil, fmt.Errorf("parsing token response: %w", err)
		}
		return tok, nil, nil
	}
	oe := &oidcError{}
	if err := json.Unmarshal(body, oe); err != nil || oe.Code == "" {
		return nil, nil, fmt.Errorf("token endpoint returned %s", resp.Status)
	}
	return nil, oe, nil
}

func postForm(ctx context.Context, httpClient *http.Client, endpoint string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		oe := &oidcError{}
		if jsonErr := json.Unmarshal(body, oe); jsonErr == nil && oe.Code != "" {
			return nil, fmt.Errorf("%s", oe.message())
		}
		return nil, fmt.Errorf("endpoint returned %s", resp.Status)
	}
	return body, nil
}

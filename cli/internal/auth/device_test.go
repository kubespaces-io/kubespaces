package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeTokenEndpoint serves a scripted sequence of token endpoint responses.
type fakeTokenEndpoint struct {
	t         *testing.T
	responses []tokenResponse
	calls     int
	gotForms  []map[string]string
}

type tokenResponse struct {
	status int
	body   map[string]any
}

func (f *fakeTokenEndpoint) handler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		f.t.Fatalf("parsing form: %v", err)
	}
	form := map[string]string{}
	for k := range r.PostForm {
		form[k] = r.PostForm.Get(k)
	}
	f.gotForms = append(f.gotForms, form)
	if f.calls >= len(f.responses) {
		f.t.Fatalf("unexpected extra token request #%d", f.calls+1)
	}
	resp := f.responses[f.calls]
	f.calls++
	w.WriteHeader(resp.status)
	json.NewEncoder(w).Encode(resp.body)
}

func TestPollPendingSlowDownSuccess(t *testing.T) {
	// Arrange
	fake := &fakeTokenEndpoint{t: t, responses: []tokenResponse{
		{http.StatusBadRequest, map[string]any{"error": "authorization_pending"}},
		{http.StatusBadRequest, map[string]any{"error": "slow_down"}},
		{http.StatusOK, map[string]any{"access_token": "at", "refresh_token": "rt", "expires_in": 300}},
	}}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	var sleeps []time.Duration
	flow := &Flow{
		HTTP:     srv.Client(),
		ClientID: "kubespaces",
		Sleep:    func(d time.Duration) { sleeps = append(sleeps, d) },
	}
	da := &DeviceAuthorization{DeviceCode: "dc", Interval: 1, ExpiresIn: 600}

	// Act
	tok, err := flow.Poll(context.Background(), srv.URL, da)

	// Assert
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if tok.AccessToken != "at" || tok.RefreshToken != "rt" || tok.ExpiresIn != 300 {
		t.Errorf("Poll() token = %+v", tok)
	}
	if fake.calls != 3 {
		t.Errorf("token endpoint calls = %d, want 3", fake.calls)
	}
	// interval starts at 1s; slow_down adds 5s for the second sleep.
	wantSleeps := []time.Duration{1 * time.Second, 6 * time.Second}
	if len(sleeps) != len(wantSleeps) {
		t.Fatalf("sleeps = %v, want %v", sleeps, wantSleeps)
	}
	for i, want := range wantSleeps {
		if sleeps[i] != want {
			t.Errorf("sleep[%d] = %v, want %v", i, sleeps[i], want)
		}
	}
	form := fake.gotForms[0]
	if form["grant_type"] != deviceGrantType || form["device_code"] != "dc" || form["client_id"] != "kubespaces" {
		t.Errorf("poll form = %v", form)
	}
}

func TestPollExpiredToken(t *testing.T) {
	// Arrange
	fake := &fakeTokenEndpoint{t: t, responses: []tokenResponse{
		{http.StatusBadRequest, map[string]any{"error": "expired_token"}},
	}}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	flow := &Flow{HTTP: srv.Client(), ClientID: "kubespaces", Sleep: func(time.Duration) {}}
	da := &DeviceAuthorization{DeviceCode: "dc", Interval: 1, ExpiresIn: 600}

	// Act
	_, err := flow.Poll(context.Background(), srv.URL, da)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("Poll() error = %v, want expiry error", err)
	}
}

func TestPollAccessDenied(t *testing.T) {
	// Arrange
	fake := &fakeTokenEndpoint{t: t, responses: []tokenResponse{
		{http.StatusBadRequest, map[string]any{"error": "access_denied", "error_description": "user rejected"}},
	}}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	flow := &Flow{HTTP: srv.Client(), ClientID: "kubespaces", Sleep: func(time.Duration) {}}
	da := &DeviceAuthorization{DeviceCode: "dc", Interval: 1, ExpiresIn: 600}

	// Act
	_, err := flow.Poll(context.Background(), srv.URL, da)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("Poll() error = %v, want access_denied", err)
	}
}

func TestPollDeadlineFromExpiresIn(t *testing.T) {
	// Arrange: clock jumps past the device-code deadline after the first poll.
	fake := &fakeTokenEndpoint{t: t, responses: []tokenResponse{
		{http.StatusBadRequest, map[string]any{"error": "authorization_pending"}},
	}}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	now := time.Now()
	calls := 0
	flow := &Flow{
		HTTP:     srv.Client(),
		ClientID: "kubespaces",
		Sleep:    func(time.Duration) {},
		Now: func() time.Time {
			calls++
			if calls > 1 {
				return now.Add(time.Hour)
			}
			return now
		},
	}
	da := &DeviceAuthorization{DeviceCode: "dc", Interval: 1, ExpiresIn: 30}

	// Act
	_, err := flow.Poll(context.Background(), srv.URL, da)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("Poll() error = %v, want expiry error", err)
	}
}

func TestStart(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing form: %v", err)
		}
		if r.PostForm.Get("client_id") != "kubespaces" {
			t.Errorf("client_id = %q", r.PostForm.Get("client_id"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"device_code":               "dc",
			"user_code":                 "ABCD-EFGH",
			"verification_uri":          "https://idp/device",
			"verification_uri_complete": "https://idp/device?user_code=ABCD-EFGH",
			"expires_in":                600,
			"interval":                  5,
		})
	}))
	defer srv.Close()
	flow := &Flow{HTTP: srv.Client(), ClientID: "kubespaces"}

	// Act
	da, err := flow.Start(context.Background(), srv.URL)

	// Assert
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if da.DeviceCode != "dc" || da.UserCode != "ABCD-EFGH" || da.Interval != 5 {
		t.Errorf("Start() = %+v", da)
	}
}

func TestRefresh(t *testing.T) {
	// Arrange
	fake := &fakeTokenEndpoint{t: t, responses: []tokenResponse{
		{http.StatusOK, map[string]any{"access_token": "new-at", "refresh_token": "new-rt", "expires_in": 300}},
	}}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	flow := &Flow{HTTP: srv.Client(), ClientID: "kubespaces"}

	// Act
	tok, err := flow.Refresh(context.Background(), srv.URL, "old-rt")

	// Assert
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if tok.AccessToken != "new-at" || tok.RefreshToken != "new-rt" {
		t.Errorf("Refresh() = %+v", tok)
	}
	form := fake.gotForms[0]
	if form["grant_type"] != refreshGrantType || form["refresh_token"] != "old-rt" {
		t.Errorf("refresh form = %v", form)
	}
}

func TestRefreshFailure(t *testing.T) {
	// Arrange
	fake := &fakeTokenEndpoint{t: t, responses: []tokenResponse{
		{http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "token expired"}},
	}}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	flow := &Flow{HTTP: srv.Client(), ClientID: "kubespaces"}

	// Act
	_, err := flow.Refresh(context.Background(), srv.URL, "old-rt")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("Refresh() error = %v, want invalid_grant", err)
	}
}

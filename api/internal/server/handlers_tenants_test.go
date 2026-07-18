package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kubespaces-io/kubespaces/api/internal/auth"
	"github.com/kubespaces-io/kubespaces/api/internal/k8s"
	"github.com/kubespaces-io/kubespaces/api/internal/store"
)

var (
	adminClaims = &auth.Claims{
		Subject: "admin-sub",
		Email:   "admin@example.com",
		Roles:   []string{auth.RoleAdmin},
	}
	memberClaims = &auth.Claims{
		Subject: "member-sub",
		Email:   "member@example.com",
		Roles:   []string{auth.RoleMember},
	}
)

func seedTenant(fs *fakeStore, fc *fakeCluster, name, owner, phase string) {
	fs.tenants[name] = store.TenantRecord{
		Name:      name,
		Owner:     owner,
		Spec:      json.RawMessage(`{"resources":{"cpu":"4"}}`),
		CreatedAt: time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC),
	}
	fc.states[name] = k8s.TenantState{Phase: phase}
}

func doRequest(t *testing.T, claims *auth.Claims, fs *fakeStore, fc *fakeCluster, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	srv := New(fs, fc, fakeAuth(claims))
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	return rec
}

func decodeTenants(t *testing.T, rec *httptest.ResponseRecorder) []TenantResponse {
	t.Helper()
	var tenants []TenantResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &tenants); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return tenants
}

func TestListTenants(t *testing.T) {
	tests := []struct {
		name      string
		claims    *auth.Claims
		wantNames map[string]bool
	}{
		{
			name:      "admin sees all tenants",
			claims:    adminClaims,
			wantNames: map[string]bool{"alpha": true, "beta": true},
		},
		{
			name:      "member sees only own tenants",
			claims:    memberClaims,
			wantNames: map[string]bool{"beta": true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			fs, fc := newFakeStore(), newFakeCluster()
			seedTenant(fs, fc, "alpha", "someone-else@example.com", "Ready")
			seedTenant(fs, fc, "beta", "member@example.com", "Provisioning")

			// Act
			rec := doRequest(t, tt.claims, fs, fc, http.MethodGet, "/api/v1/tenants", "")

			// Assert
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
			}
			tenants := decodeTenants(t, rec)
			if len(tenants) != len(tt.wantNames) {
				t.Fatalf("got %d tenants, want %d: %+v", len(tenants), len(tt.wantNames), tenants)
			}
			for _, tenant := range tenants {
				if !tt.wantNames[tenant.Name] {
					t.Errorf("unexpected tenant %q in response", tenant.Name)
				}
			}
		})
	}
}

func TestListTenantsPhaseFromCluster(t *testing.T) {
	// Arrange: record exists in DB but CR is gone.
	fs, fc := newFakeStore(), newFakeCluster()
	seedTenant(fs, fc, "alpha", "member@example.com", "Ready")
	seedTenant(fs, fc, "beta", "member@example.com", "Ready")
	delete(fc.states, "beta")

	// Act
	rec := doRequest(t, memberClaims, fs, fc, http.MethodGet, "/api/v1/tenants", "")

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	phases := map[string]string{}
	for _, tenant := range decodeTenants(t, rec) {
		phases[tenant.Name] = tenant.Phase
	}
	if phases["alpha"] != "Ready" {
		t.Errorf("alpha phase = %q, want Ready", phases["alpha"])
	}
	if phases["beta"] != k8s.PhaseUnknown {
		t.Errorf("beta phase = %q, want %q (CR missing)", phases["beta"], k8s.PhaseUnknown)
	}
}

func TestCreateTenant(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		arrange    func(fs *fakeStore, fc *fakeCluster)
		wantStatus int
	}{
		{
			name:       "valid create",
			body:       `{"name":"team-a","displayName":"Team A","resources":{"cpu":"4","memory":"8Gi"}}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid name uppercase",
			body:       `{"name":"Team-A"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "name too long",
			body:       `{"name":"` + strings.Repeat("a", 41) + `"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed json",
			body:       `{"name":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate name conflicts",
			body: `{"name":"team-a"}`,
			arrange: func(fs *fakeStore, fc *fakeCluster) {
				seedTenant(fs, fc, "team-a", "member@example.com", "Ready")
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "cluster failure rolls back and reports 500",
			body: `{"name":"team-a"}`,
			arrange: func(_ *fakeStore, fc *fakeCluster) {
				fc.createErr = errors.New("apiserver down")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			fs, fc := newFakeStore(), newFakeCluster()
			if tt.arrange != nil {
				tt.arrange(fs, fc)
			}

			// Act
			rec := doRequest(t, memberClaims, fs, fc, http.MethodPost, "/api/v1/tenants", tt.body)

			// Assert
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestCreateTenantPersistsOwnerAndAudits(t *testing.T) {
	// Arrange
	fs, fc := newFakeStore(), newFakeCluster()

	// Act
	rec := doRequest(t, memberClaims, fs, fc, http.MethodPost, "/api/v1/tenants",
		`{"name":"team-a","displayName":"Team A","resources":{"cpu":"4"}}`)

	// Assert
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var resp TenantResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Owner != "member@example.com" {
		t.Errorf("owner = %q, want member email", resp.Owner)
	}
	if resp.Phase != "Pending" {
		t.Errorf("phase = %q, want Pending", resp.Phase)
	}
	if resp.Resources.CPU != "4" {
		t.Errorf("resources.cpu = %q, want 4", resp.Resources.CPU)
	}
	record, ok := fs.tenants["team-a"]
	if !ok {
		t.Fatal("tenant record not persisted")
	}
	if record.Owner != "member@example.com" {
		t.Errorf("persisted owner = %q, want member email", record.Owner)
	}
	if _, ok := fc.states["team-a"]; !ok {
		t.Error("tenant CR not created")
	}
	if len(fs.auditEntries) != 1 || fs.auditEntries[0] != "tenant.create" {
		t.Errorf("audit entries = %v, want [tenant.create]", fs.auditEntries)
	}
}

func TestCreateTenantRollsBackRecordOnClusterFailure(t *testing.T) {
	// Arrange
	fs, fc := newFakeStore(), newFakeCluster()
	fc.createErr = errors.New("apiserver down")

	// Act
	rec := doRequest(t, memberClaims, fs, fc, http.MethodPost, "/api/v1/tenants", `{"name":"team-a"}`)

	// Assert
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if _, exists := fs.tenants["team-a"]; exists {
		t.Error("tenant record should have been rolled back")
	}
}

func TestGetTenantOwnership(t *testing.T) {
	tests := []struct {
		name       string
		claims     *auth.Claims
		tenant     string
		wantStatus int
	}{
		{name: "owner reads own tenant", claims: memberClaims, tenant: "mine", wantStatus: http.StatusOK},
		{name: "member cannot read others", claims: memberClaims, tenant: "theirs", wantStatus: http.StatusNotFound},
		{name: "admin reads any tenant", claims: adminClaims, tenant: "theirs", wantStatus: http.StatusOK},
		{name: "unknown tenant is 404", claims: adminClaims, tenant: "nope", wantStatus: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			fs, fc := newFakeStore(), newFakeCluster()
			seedTenant(fs, fc, "mine", "member@example.com", "Ready")
			seedTenant(fs, fc, "theirs", "other@example.com", "Ready")

			// Act
			rec := doRequest(t, tt.claims, fs, fc, http.MethodGet, "/api/v1/tenants/"+tt.tenant, "")

			// Assert
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestDeleteTenant(t *testing.T) {
	// Arrange
	fs, fc := newFakeStore(), newFakeCluster()
	seedTenant(fs, fc, "mine", "member@example.com", "Ready")

	// Act
	rec := doRequest(t, memberClaims, fs, fc, http.MethodDelete, "/api/v1/tenants/mine", "")

	// Assert
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body %s", rec.Code, rec.Body.String())
	}
	if _, exists := fc.states["mine"]; exists {
		t.Error("tenant CR should have been deleted")
	}
	if _, exists := fs.tenants["mine"]; exists {
		t.Error("tenant record should have been soft-deleted")
	}
	if len(fs.auditEntries) != 1 || fs.auditEntries[0] != "tenant.delete" {
		t.Errorf("audit entries = %v, want [tenant.delete]", fs.auditEntries)
	}
}

func TestKubeconfig(t *testing.T) {
	tests := []struct {
		name       string
		arrange    func(fs *fakeStore, fc *fakeCluster)
		wantStatus int
		wantBody   string
	}{
		{
			name: "ready kubeconfig is served as yaml",
			arrange: func(fs *fakeStore, fc *fakeCluster) {
				seedTenant(fs, fc, "mine", "member@example.com", "Ready")
				fc.kubeconfigs["mine"] = []byte("apiVersion: v1\nkind: Config\n")
			},
			wantStatus: http.StatusOK,
			wantBody:   "apiVersion: v1\nkind: Config\n",
		},
		{
			name: "not ready is 409",
			arrange: func(fs *fakeStore, fc *fakeCluster) {
				seedTenant(fs, fc, "mine", "member@example.com", "Provisioning")
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "missing CR is 404",
			arrange: func(fs *fakeStore, fc *fakeCluster) {
				seedTenant(fs, fc, "mine", "member@example.com", "Ready")
				delete(fc.states, "mine")
			},
			wantStatus: http.StatusNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			fs, fc := newFakeStore(), newFakeCluster()
			tt.arrange(fs, fc)

			// Act
			rec := doRequest(t, memberClaims, fs, fc, http.MethodGet, "/api/v1/tenants/mine/kubeconfig", "")

			// Assert
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody == "" {
				return
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "text/yaml" {
				t.Errorf("content-type = %q, want text/yaml", ct)
			}
		})
	}
}

func TestHealthzNoAuth(t *testing.T) {
	// Arrange: auth middleware that always rejects, healthz must bypass it.
	reject := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
	}
	srv := New(newFakeStore(), newFakeCluster(), reject)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", rec.Code)
	}
}

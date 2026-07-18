package server

import (
	"context"
	"net/http"

	"github.com/kubespaces-io/kubespaces/api/internal/auth"
	"github.com/kubespaces-io/kubespaces/api/internal/k8s"
	"github.com/kubespaces-io/kubespaces/api/internal/store"
)

// fakeStore is an in-memory TenantStore.
type fakeStore struct {
	tenants      map[string]store.TenantRecord
	auditEntries []string

	createErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{tenants: map[string]store.TenantRecord{}}
}

func (f *fakeStore) CreateTenant(_ context.Context, t store.TenantRecord) error {
	if f.createErr != nil {
		return f.createErr
	}
	if _, exists := f.tenants[t.Name]; exists {
		return store.ErrConflict
	}
	f.tenants[t.Name] = t
	return nil
}

func (f *fakeStore) GetTenant(_ context.Context, name string) (*store.TenantRecord, error) {
	t, ok := f.tenants[name]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &t, nil
}

func (f *fakeStore) ListTenants(_ context.Context) ([]store.TenantRecord, error) {
	var out []store.TenantRecord
	for _, t := range f.tenants {
		out = append(out, t)
	}
	return out, nil
}

func (f *fakeStore) ListTenantsByOwner(_ context.Context, owner string) ([]store.TenantRecord, error) {
	var out []store.TenantRecord
	for _, t := range f.tenants {
		if t.Owner == owner {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeStore) SoftDeleteTenant(_ context.Context, name string) error {
	if _, ok := f.tenants[name]; !ok {
		return store.ErrNotFound
	}
	delete(f.tenants, name)
	return nil
}

func (f *fakeStore) HardDeleteTenant(_ context.Context, name string) error {
	delete(f.tenants, name)
	return nil
}

func (f *fakeStore) Audit(_ context.Context, _, action string, _ map[string]any) error {
	f.auditEntries = append(f.auditEntries, action)
	return nil
}

// fakeCluster is an in-memory TenantCluster.
type fakeCluster struct {
	states      map[string]k8s.TenantState
	kubeconfigs map[string][]byte

	createErr error
}

func newFakeCluster() *fakeCluster {
	return &fakeCluster{
		states:      map[string]k8s.TenantState{},
		kubeconfigs: map[string][]byte{},
	}
}

func (f *fakeCluster) CreateTenant(_ context.Context, spec k8s.TenantSpec) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.states[spec.Name] = k8s.TenantState{Phase: "Pending"}
	return nil
}

func (f *fakeCluster) GetTenantState(_ context.Context, name string) (*k8s.TenantState, error) {
	state, ok := f.states[name]
	if !ok {
		return nil, k8s.ErrNotFound
	}
	return &state, nil
}

func (f *fakeCluster) ListTenantStates(_ context.Context) (map[string]k8s.TenantState, error) {
	out := make(map[string]k8s.TenantState, len(f.states))
	for name, state := range f.states {
		out[name] = state
	}
	return out, nil
}

func (f *fakeCluster) DeleteTenant(_ context.Context, name string) error {
	delete(f.states, name)
	return nil
}

func (f *fakeCluster) Kubeconfig(_ context.Context, name string) ([]byte, error) {
	if _, ok := f.states[name]; !ok {
		return nil, k8s.ErrNotFound
	}
	data, ok := f.kubeconfigs[name]
	if !ok {
		return nil, k8s.ErrNotReady
	}
	return data, nil
}

// fakeAuth injects fixed claims, standing in for the OIDC middleware.
func fakeAuth(claims *auth.Claims) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithClaims(r.Context(), claims)))
		})
	}
}

// Package server wires the HTTP API: routing, handlers, authorization.
package server

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kubespaces-io/kubespaces/api/internal/k8s"
	"github.com/kubespaces-io/kubespaces/api/internal/store"
)

// TenantStore is the narrow persistence interface the handlers need.
type TenantStore interface {
	CreateTenant(ctx context.Context, t store.TenantRecord) error
	GetTenant(ctx context.Context, name string) (*store.TenantRecord, error)
	ListTenants(ctx context.Context) ([]store.TenantRecord, error)
	ListTenantsByOwner(ctx context.Context, owner string) ([]store.TenantRecord, error)
	SoftDeleteTenant(ctx context.Context, name string) error
	HardDeleteTenant(ctx context.Context, name string) error
	Audit(ctx context.Context, actor, action string, detail map[string]any) error
}

// TenantCluster is the narrow Kubernetes interface the handlers need.
type TenantCluster interface {
	CreateTenant(ctx context.Context, spec k8s.TenantSpec) error
	GetTenantState(ctx context.Context, name string) (*k8s.TenantState, error)
	ListTenantStates(ctx context.Context) (map[string]k8s.TenantState, error)
	DeleteTenant(ctx context.Context, name string) error
	Kubeconfig(ctx context.Context, name string) ([]byte, error)
}

// Middleware is an http middleware; auth is injected so tests can fake it.
type Middleware func(http.Handler) http.Handler

// Server holds handler dependencies.
type Server struct {
	store   TenantStore
	cluster TenantCluster
	auth    Middleware
}

// New builds a Server.
func New(st TenantStore, cluster TenantCluster, auth Middleware) *Server {
	return &Server{store: st, cluster: cluster, auth: auth}
}

// Router builds the chi router with all routes mounted.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", handleHealthz)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.auth)
		r.Get("/me", handleMe)
		r.Route("/tenants", func(r chi.Router) {
			r.Get("/", s.handleListTenants)
			r.Post("/", s.handleCreateTenant)
			r.Get("/{name}", s.handleGetTenant)
			r.Delete("/{name}", s.handleDeleteTenant)
			r.Get("/{name}/kubeconfig", s.handleKubeconfig)
		})
	})
	return r
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

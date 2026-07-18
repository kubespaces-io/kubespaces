package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kubespaces-io/kubespaces/api/internal/auth"
	"github.com/kubespaces-io/kubespaces/api/internal/k8s"
	"github.com/kubespaces-io/kubespaces/api/internal/store"
)

const maxRequestBody = 1 << 20 // 1 MiB

func (s *Server) handleListTenants(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "missing authentication")
		return
	}

	records, err := s.listRecordsFor(r, claims)
	if err != nil {
		respondInternal(w, "list tenants", err)
		return
	}
	states, err := s.cluster.ListTenantStates(r.Context())
	if err != nil {
		respondInternal(w, "list tenant resources", err)
		return
	}

	responses := make([]TenantResponse, 0, len(records))
	for i := range records {
		record := &records[i]
		var state *k8s.TenantState
		if st, found := states[record.Name]; found {
			state = &st
		}
		responses = append(responses, tenantResponse(record, state))
	}
	respondJSON(w, http.StatusOK, responses)
}

func (s *Server) listRecordsFor(r *http.Request, claims *auth.Claims) ([]store.TenantRecord, error) {
	if claims.IsAdmin() {
		return s.store.ListTenants(r.Context())
	}
	return s.store.ListTenantsByOwner(r.Context(), claims.Identity())
}

func (s *Server) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "missing authentication")
		return
	}

	var req createTenantRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := ValidateTenantName(req.Name); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	spec, err := json.Marshal(tenantSpec{Resources: req.Resources, VCluster: req.VCluster})
	if err != nil {
		respondInternal(w, "marshal tenant spec", err)
		return
	}
	record := store.TenantRecord{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Owner:       claims.Identity(),
		Spec:        spec,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.store.CreateTenant(r.Context(), record); err != nil {
		if errors.Is(err, store.ErrConflict) {
			respondError(w, http.StatusConflict, "tenant already exists")
			return
		}
		respondInternal(w, "create tenant record", err)
		return
	}

	if err := s.cluster.CreateTenant(r.Context(), crSpec(req, claims.Identity())); err != nil {
		// Roll back the DB row so the name is not left orphaned.
		if delErr := s.store.HardDeleteTenant(r.Context(), req.Name); delErr != nil {
			slog.Error("rollback tenant record", "tenant", req.Name, "error", delErr)
		}
		respondInternal(w, "create tenant resource", err)
		return
	}

	s.audit(r, claims, "tenant.create", map[string]any{"tenant": req.Name})

	state := &k8s.TenantState{Phase: "Pending"}
	respondJSON(w, http.StatusCreated, tenantResponse(&record, state))
}

func crSpec(req createTenantRequest, owner string) k8s.TenantSpec {
	spec := k8s.TenantSpec{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Owner:       owner,
	}
	if req.Resources != nil {
		spec.Resources = map[string]string{}
		setIfNotEmpty(spec.Resources, "cpu", req.Resources.CPU)
		setIfNotEmpty(spec.Resources, "memory", req.Resources.Memory)
		setIfNotEmpty(spec.Resources, "storage", req.Resources.Storage)
	}
	if req.VCluster != nil {
		vc := map[string]any{}
		if req.VCluster.Version != "" {
			vc["version"] = req.VCluster.Version
		}
		if req.VCluster.KubernetesVersion != "" {
			vc["kubernetesVersion"] = req.VCluster.KubernetesVersion
		}
		if len(req.VCluster.ValuesOverrides) > 0 {
			vc["valuesOverrides"] = req.VCluster.ValuesOverrides
		}
		spec.VCluster = vc
	}
	return spec
}

func setIfNotEmpty(m map[string]string, key, value string) {
	if value != "" {
		m[key] = value
	}
}

func (s *Server) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	record, _, done := s.authorizedTenant(w, r)
	if done {
		return
	}

	state, err := s.cluster.GetTenantState(r.Context(), record.Name)
	if err != nil && !errors.Is(err, k8s.ErrNotFound) {
		respondInternal(w, "get tenant resource", err)
		return
	}
	respondJSON(w, http.StatusOK, tenantResponse(record, state))
}

func (s *Server) handleDeleteTenant(w http.ResponseWriter, r *http.Request) {
	record, claims, done := s.authorizedTenant(w, r)
	if done {
		return
	}

	if err := s.cluster.DeleteTenant(r.Context(), record.Name); err != nil {
		respondInternal(w, "delete tenant resource", err)
		return
	}
	if err := s.store.SoftDeleteTenant(r.Context(), record.Name); err != nil && !errors.Is(err, store.ErrNotFound) {
		respondInternal(w, "soft-delete tenant record", err)
		return
	}

	s.audit(r, claims, "tenant.delete", map[string]any{"tenant": record.Name})
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleKubeconfig(w http.ResponseWriter, r *http.Request) {
	record, _, done := s.authorizedTenant(w, r)
	if done {
		return
	}

	kubeconfig, err := s.cluster.Kubeconfig(r.Context(), record.Name)
	if errors.Is(err, k8s.ErrNotFound) {
		respondError(w, http.StatusNotFound, "tenant resource not found")
		return
	}
	if errors.Is(err, k8s.ErrNotReady) {
		respondError(w, http.StatusConflict, "tenant kubeconfig not ready")
		return
	}
	if err != nil {
		respondInternal(w, "read tenant kubeconfig", err)
		return
	}
	w.Header().Set("Content-Type", "text/yaml")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(kubeconfig); err != nil {
		slog.Error("write kubeconfig response", "error", err)
	}
}

// authorizedTenant loads the tenant record and enforces ownership. Members
// get 404 (not 403) for tenants they do not own, to avoid name enumeration.
// Returns done=true if a response has already been written.
func (s *Server) authorizedTenant(w http.ResponseWriter, r *http.Request) (*store.TenantRecord, *auth.Claims, bool) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "missing authentication")
		return nil, nil, true
	}
	name := chi.URLParam(r, "name")
	if err := ValidateTenantName(name); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return nil, nil, true
	}

	record, err := s.store.GetTenant(r.Context(), name)
	if errors.Is(err, store.ErrNotFound) {
		respondError(w, http.StatusNotFound, "tenant not found")
		return nil, nil, true
	}
	if err != nil {
		respondInternal(w, "get tenant record", err)
		return nil, nil, true
	}
	if !claims.IsAdmin() && record.Owner != claims.Identity() {
		respondError(w, http.StatusNotFound, "tenant not found")
		return nil, nil, true
	}
	return record, claims, false
}

func (s *Server) audit(r *http.Request, claims *auth.Claims, action string, detail map[string]any) {
	if err := s.store.Audit(r.Context(), claims.Identity(), action, detail); err != nil {
		slog.Error("write audit entry", "action", action, "error", err)
	}
}

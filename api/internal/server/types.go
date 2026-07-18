package server

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/kubespaces-io/kubespaces/api/internal/k8s"
	"github.com/kubespaces-io/kubespaces/api/internal/store"
)

// TenantResources mirrors the CRD spec.resources quota block.
type TenantResources struct {
	CPU     string `json:"cpu,omitempty"`
	Memory  string `json:"memory,omitempty"`
	Storage string `json:"storage,omitempty"`
}

// TenantVCluster mirrors the CRD spec.vcluster options block.
type TenantVCluster struct {
	Version           string         `json:"version,omitempty"`
	KubernetesVersion string         `json:"kubernetesVersion,omitempty"`
	ValuesOverrides   map[string]any `json:"valuesOverrides,omitempty"`
}

// createTenantRequest is the POST /tenants body.
type createTenantRequest struct {
	Name        string           `json:"name"`
	DisplayName string           `json:"displayName"`
	Resources   *TenantResources `json:"resources"`
	VCluster    *TenantVCluster  `json:"vcluster"`
}

// tenantSpec is what gets persisted in the tenants.spec JSONB column.
type tenantSpec struct {
	Resources *TenantResources `json:"resources,omitempty"`
	VCluster  *TenantVCluster  `json:"vcluster,omitempty"`
}

// TenantResponse is the Tenant JSON shape from the contract.
type TenantResponse struct {
	Name        string          `json:"name"`
	DisplayName string          `json:"displayName"`
	Owner       string          `json:"owner"`
	Phase       string          `json:"phase"`
	Message     string          `json:"message"`
	Resources   TenantResources `json:"resources"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// tenantResponse merges the DB record with live CR state (nil = CR missing).
func tenantResponse(record *store.TenantRecord, state *k8s.TenantState) TenantResponse {
	resp := TenantResponse{
		Name:        record.Name,
		DisplayName: record.DisplayName,
		Owner:       record.Owner,
		Phase:       k8s.PhaseUnknown,
		Resources:   resourcesFromSpec(record),
		CreatedAt:   record.CreatedAt,
	}
	if state != nil {
		resp.Phase = state.Phase
		resp.Message = state.Message
	}
	return resp
}

func resourcesFromSpec(record *store.TenantRecord) TenantResources {
	if len(record.Spec) == 0 {
		return TenantResources{}
	}
	var spec tenantSpec
	if err := json.Unmarshal(record.Spec, &spec); err != nil {
		slog.Warn("tenant has malformed spec JSON", "tenant", record.Name, "error", err)
		return TenantResources{}
	}
	if spec.Resources == nil {
		return TenantResources{}
	}
	return *spec.Resources
}

package v1alpha1

import (
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TenantFinalizer is the finalizer the operator places on every Tenant it
// manages so that vCluster and namespace teardown happens before the CR
// disappears.
const TenantFinalizer = "kubespaces.io/finalizer"

// TenantNamespacePrefix is the prefix used when spec.targetNamespace is empty.
const TenantNamespacePrefix = "kubespaces-tenant-"

// KubeconfigSecretKey is the key inside the vCluster kubeconfig Secret.
const KubeconfigSecretKey = "config"

// TenantPhase describes the lifecycle phase of a Tenant.
type TenantPhase string

const (
	// TenantPhasePending means the Tenant has been accepted but reconciliation
	// has not materialized anything yet.
	TenantPhasePending TenantPhase = "Pending"
	// TenantPhaseProvisioning means namespace/quota/vCluster are being created
	// and the virtual cluster is not ready yet.
	TenantPhaseProvisioning TenantPhase = "Provisioning"
	// TenantPhaseReady means the virtual cluster is up and its kubeconfig
	// Secret is available.
	TenantPhaseReady TenantPhase = "Ready"
	// TenantPhaseDeleting means the Tenant is being torn down.
	TenantPhaseDeleting TenantPhase = "Deleting"
	// TenantPhaseFailed means the last reconciliation attempt failed; the
	// operator keeps retrying with backoff.
	TenantPhaseFailed TenantPhase = "Failed"
)

// ConditionReady is the condition type tracking overall tenant readiness.
const ConditionReady = "Ready"

// TenantResources is the resource quota applied to the tenant namespace.
type TenantResources struct {
	// CPU quota (Kubernetes quantity, e.g. "4").
	// +optional
	CPU string `json:"cpu,omitempty"`
	// Memory quota (e.g. "8Gi").
	// +optional
	Memory string `json:"memory,omitempty"`
	// Storage quota (e.g. "20Gi").
	// +optional
	Storage string `json:"storage,omitempty"`
}

// TenantVCluster holds options passed to the vCluster provisioner.
type TenantVCluster struct {
	// Version is the vCluster chart/app version to deploy.
	// +optional
	Version string `json:"version,omitempty"`
	// KubernetesVersion is the Kubernetes version inside the virtual cluster.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
	// ValuesOverrides are raw vCluster chart values overrides (escape hatch).
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	ValuesOverrides *apiextensionsv1.JSON `json:"valuesOverrides,omitempty"`
}

// TenantSpec defines the desired state of a Tenant.
type TenantSpec struct {
	// DisplayName is the human-friendly name shown in the portal.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Owner is the OIDC subject or email of the tenant owner. Authorization is
	// enforced by the API/portal; the operator treats it as metadata.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Owner string `json:"owner"`

	// TargetNamespace is the host-cluster namespace where the virtual cluster
	// is provisioned. Defaults to kubespaces-tenant-<name>.
	// +optional
	TargetNamespace string `json:"targetNamespace,omitempty"`

	// Resources is the resource quota applied to the tenant namespace.
	// +optional
	Resources *TenantResources `json:"resources,omitempty"`

	// VCluster holds options passed to the vCluster provisioner.
	// +optional
	VCluster *TenantVCluster `json:"vcluster,omitempty"`
}

// SecretKeyRef points at a key inside a Secret in the tenant namespace.
type SecretKeyRef struct {
	// Name of the Secret.
	// +optional
	Name string `json:"name,omitempty"`
	// Key inside the Secret.
	// +optional
	Key string `json:"key,omitempty"`
}

// TenantStatus defines the observed state of a Tenant.
type TenantStatus struct {
	// Phase is the coarse-grained lifecycle phase of the tenant.
	// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Deleting;Failed
	// +optional
	Phase TenantPhase `json:"phase,omitempty"`

	// Message is a human-readable explanation of the current phase.
	// +optional
	Message string `json:"message,omitempty"`

	// ObservedGeneration is the last spec generation the operator acted on.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// KubeconfigSecretRef points at the Secret (in targetNamespace) holding
	// the tenant kubeconfig.
	// +optional
	KubeconfigSecretRef *SecretKeyRef `json:"kubeconfigSecretRef,omitempty"`

	// APIServerURL is the public endpoint of the tenant's API server, set
	// when the operator exposes it through the platform Gateway.
	// +optional
	APIServerURL string `json:"apiServerUrl,omitempty"`

	// AppsDomain is the wildcard hostname (*.{tenant}.apps.{domain}) under
	// which the tenant's workloads are exposed, set when app exposure is
	// configured.
	// +optional
	AppsDomain string `json:"appsDomain,omitempty"`

	// Conditions represent the latest available observations of the tenant state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=ten
// +kubebuilder:printcolumn:name="Owner",type=string,JSONPath=`.spec.owner`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="API",type=string,JSONPath=`.status.apiServerUrl`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Tenant is a KubeSpaces tenant: a namespace, a resource quota and a virtual
// cluster provisioned on the host cluster.
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec"`
	Status TenantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TenantList contains a list of Tenant.
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}

// TargetNamespace returns spec.targetNamespace, defaulting to
// kubespaces-tenant-<name> when unset.
func (t *Tenant) TargetNamespace() string {
	if t.Spec.TargetNamespace != "" {
		return t.Spec.TargetNamespace
	}
	return TenantNamespacePrefix + t.Name
}

// KubeconfigSecretName returns the name of the Secret vCluster writes the
// virtual-cluster kubeconfig into (vc-<release>, release name = tenant name).
func (t *Tenant) KubeconfigSecretName() string {
	return fmt.Sprintf("vc-%s", t.Name)
}

func init() {
	SchemeBuilder.Register(&Tenant{}, &TenantList{})
}

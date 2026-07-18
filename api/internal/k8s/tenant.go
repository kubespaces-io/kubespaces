package k8s

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TenantSpec is what the API sets when creating a Tenant CR.
type TenantSpec struct {
	Name        string
	DisplayName string
	Owner       string
	Resources   map[string]string
	VCluster    map[string]any
}

// TenantState is the live state read from a Tenant CR.
type TenantState struct {
	Phase               string
	Message             string
	TargetNamespace     string
	KubeconfigSecretRef SecretRef
}

// SecretRef points at the kubeconfig secret in the tenant namespace.
type SecretRef struct {
	Name string
	Key  string
}

// PhaseUnknown is reported when the CR is missing.
const PhaseUnknown = "Unknown"

// phasePending is reported for CRs the operator has not touched yet.
const phasePending = "Pending"

// CreateTenant creates the Tenant CR for the given spec.
func (c *Client) CreateTenant(ctx context.Context, spec TenantSpec) error {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": TenantGVR.Group + "/" + TenantGVR.Version,
		"kind":       "Tenant",
		"metadata": map[string]any{
			"name":   spec.Name,
			"labels": map[string]any{ManagedByLabelKey: ManagedByLabelValue},
		},
		"spec": tenantSpecFields(spec),
	}}
	_, err := c.dynamic.Resource(TenantGVR).Create(ctx, obj, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("tenant resource %q already exists", spec.Name)
	}
	if err != nil {
		return fmt.Errorf("create tenant resource: %w", err)
	}
	return nil
}

func tenantSpecFields(spec TenantSpec) map[string]any {
	fields := map[string]any{"owner": spec.Owner}
	if spec.DisplayName != "" {
		fields["displayName"] = spec.DisplayName
	}
	if len(spec.Resources) > 0 {
		resources := map[string]any{}
		for k, v := range spec.Resources {
			resources[k] = v
		}
		fields["resources"] = resources
	}
	if len(spec.VCluster) > 0 {
		fields["vcluster"] = spec.VCluster
	}
	return fields
}

// GetTenantState reads live status from a Tenant CR; ErrNotFound if absent.
func (c *Client) GetTenantState(ctx context.Context, name string) (*TenantState, error) {
	obj, err := c.dynamic.Resource(TenantGVR).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant resource: %w", err)
	}
	state := stateFromObject(obj)
	return &state, nil
}

// ListTenantStates returns live state for all Tenant CRs, keyed by name.
func (c *Client) ListTenantStates(ctx context.Context) (map[string]TenantState, error) {
	list, err := c.dynamic.Resource(TenantGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list tenant resources: %w", err)
	}
	states := make(map[string]TenantState, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		states[item.GetName()] = stateFromObject(item)
	}
	return states, nil
}

// DeleteTenant removes the Tenant CR; missing CRs are not an error.
func (c *Client) DeleteTenant(ctx context.Context, name string) error {
	err := c.dynamic.Resource(TenantGVR).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete tenant resource: %w", err)
	}
	return nil
}

func stateFromObject(obj *unstructured.Unstructured) TenantState {
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
	if phase == "" {
		phase = phasePending
	}
	message, _, _ := unstructured.NestedString(obj.Object, "status", "message")
	targetNamespace, _, _ := unstructured.NestedString(obj.Object, "spec", "targetNamespace")
	if targetNamespace == "" {
		targetNamespace = "kubespaces-tenant-" + obj.GetName()
	}
	refName, _, _ := unstructured.NestedString(obj.Object, "status", "kubeconfigSecretRef", "name")
	refKey, _, _ := unstructured.NestedString(obj.Object, "status", "kubeconfigSecretRef", "key")
	return TenantState{
		Phase:               phase,
		Message:             message,
		TargetNamespace:     targetNamespace,
		KubeconfigSecretRef: SecretRef{Name: refName, Key: refKey},
	}
}

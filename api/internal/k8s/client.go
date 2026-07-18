// Package k8s manages Tenant custom resources and kubeconfig secrets
// via the dynamic client (no operator type imports).
package k8s

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// TenantGVR identifies the cluster-scoped Tenant custom resource.
var TenantGVR = schema.GroupVersionResource{
	Group:    "kubespaces.io",
	Version:  "v1alpha1",
	Resource: "tenants",
}

// Sentinel errors surfaced to handlers.
var (
	ErrNotFound = errors.New("tenant resource not found")
	ErrNotReady = errors.New("tenant kubeconfig not ready")
)

// ManagedByLabel marks CRs created by this API.
const (
	ManagedByLabelKey   = "app.kubernetes.io/managed-by"
	ManagedByLabelValue = "kubespaces-api"
)

// Client talks to the cluster for Tenant CRs and kubeconfig secrets.
type Client struct {
	dynamic dynamic.Interface
	core    kubernetes.Interface
}

// NewClient builds a Client from in-cluster config, falling back to the
// local kubeconfig for development.
func NewClient() (*Client, error) {
	cfg, err := restConfig()
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}
	core, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create core client: %w", err)
	}
	return &Client{dynamic: dyn, core: core}, nil
}

func restConfig() (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubernetes config (in-cluster and kubeconfig failed): %w", err)
	}
	return cfg, nil
}

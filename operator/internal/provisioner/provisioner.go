// Package provisioner isolates the vCluster provisioning logic (Helm SDK)
// behind a small interface so the reconciler can be tested without Helm.
package provisioner

import "context"

// Request describes one desired vCluster installation.
type Request struct {
	// ReleaseName is the Helm release name (= tenant name).
	ReleaseName string
	// Namespace is the host-cluster namespace to install into.
	Namespace string
	// ChartVersion is the vCluster chart version ("" = latest).
	ChartVersion string
	// KubernetesVersion is the Kubernetes version inside the virtual cluster
	// ("" = chart default).
	KubernetesVersion string
	// ValuesOverrides are raw chart values merged on top of the generated
	// values (overrides win).
	ValuesOverrides map[string]any
	// PublicAPIHost, when set, is added to the vCluster proxy certificate
	// SANs so the public endpoint presents a valid cert.
	PublicAPIHost string
	// PublicAPIURL, when set, becomes the server URL in the exported
	// kubeconfig Secret, so downloaded kubeconfigs point at the public
	// endpoint instead of the in-cluster Service.
	PublicAPIURL string
	// SyncGatewayAPI enables vCluster's native Gateway API toHost sync so
	// HTTPRoutes created inside the virtual cluster materialize in the
	// tenant host namespace.
	SyncGatewayAPI bool
	// AppsGatewayNamespace/AppsGatewayName identify the shared apps Gateway.
	// It is projected into the virtual cluster (fromHost sync, same
	// namespace/name) because the syncer only exports HTTPRoutes whose
	// parentRef resolves to a Gateway it can see.
	AppsGatewayNamespace string
	AppsGatewayName      string
}

// Provisioner installs, removes and inspects vCluster releases.
type Provisioner interface {
	// Install installs the vCluster chart, or upgrades it if the release
	// already exists. It is idempotent.
	Install(ctx context.Context, req Request) error
	// Uninstall removes the release. Missing releases are not an error.
	Uninstall(ctx context.Context, releaseName, namespace string) error
	// Status returns the Helm release status (e.g. "deployed") or
	// ErrReleaseNotFound if the release does not exist.
	Status(ctx context.Context, releaseName, namespace string) (string, error)
}

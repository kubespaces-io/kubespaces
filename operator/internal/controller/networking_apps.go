// Tenant app exposure (roadmap #16, decision D18): a per-tenant HTTPS
// listener on the shared apps Gateway (hostname *.{tenant}.apps.{domain},
// per-tenant cert-manager wildcard Certificate, allowedRoutes locked to the
// tenant namespace) plus vCluster's native Gateway API toHost sync. Isolation
// is structural: a tenant's synced routes can only attach to its own listener.
package controller

import (
	"context"
	"fmt"
	"os"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	kubespacesv1alpha1 "github.com/kubespaces-io/kubespaces/operator/api/v1alpha1"
)

// Environment variables configuring tenant app exposure. Gateway namespace,
// gateway name, domain and cluster issuer must all be set to activate.
const (
	// EnvAppsGatewayNamespace is the namespace of the shared apps Gateway.
	EnvAppsGatewayNamespace = "KUBESPACES_APPS_GATEWAY_NAMESPACE"
	// EnvAppsGatewayName is the name of that Gateway.
	EnvAppsGatewayName = "KUBESPACES_APPS_GATEWAY_NAME"
	// EnvAppsDomain is the base domain: tenant apps are exposed at
	// *.{tenant}.apps.<domain>.
	EnvAppsDomain = "KUBESPACES_APPS_DOMAIN"
	// EnvAppsClusterIssuer is the cert-manager ClusterIssuer that signs the
	// per-tenant wildcard certificates.
	EnvAppsClusterIssuer = "KUBESPACES_APPS_CLUSTER_ISSUER"
)

const (
	// tenantListenerPrefix names the per-tenant listener on the apps Gateway.
	tenantListenerPrefix = "t-"
	// tenantAppsCertSuffix names the per-tenant Certificate and its Secret.
	tenantAppsCertSuffix = "-apps-tls"
	appsListenerPort     = 443
)

// certificateGVK is cert-manager's Certificate (managed as unstructured to
// avoid importing the cert-manager API module for a single object type).
var certificateGVK = schema.GroupVersionKind{
	Group: "cert-manager.io", Version: "v1", Kind: "Certificate",
}

// AppsConfig configures public exposure of tenant workloads.
type AppsConfig struct {
	GatewayNamespace string
	GatewayName      string
	Domain           string
	ClusterIssuer    string
}

// AppsConfigFromEnv reads the KUBESPACES_APPS_* environment variables.
func AppsConfigFromEnv() AppsConfig {
	return AppsConfig{
		GatewayNamespace: os.Getenv(EnvAppsGatewayNamespace),
		GatewayName:      os.Getenv(EnvAppsGatewayName),
		Domain:           os.Getenv(EnvAppsDomain),
		ClusterIssuer:    os.Getenv(EnvAppsClusterIssuer),
	}
}

// Enabled reports whether tenant app exposure is configured.
func (a AppsConfig) Enabled() bool {
	return a.GatewayNamespace != "" && a.GatewayName != "" && a.Domain != "" && a.ClusterIssuer != ""
}

// WildcardFor returns the per-tenant apps wildcard hostname.
func (a AppsConfig) WildcardFor(tenant *kubespacesv1alpha1.Tenant) string {
	return fmt.Sprintf("*.%s.apps.%s", tenant.Name, a.Domain)
}

func tenantListenerName(tenant *kubespacesv1alpha1.Tenant) gatewayv1.SectionName {
	return gatewayv1.SectionName(tenantListenerPrefix + tenant.Name)
}

func tenantAppsCertName(tenant *kubespacesv1alpha1.Tenant) string {
	return tenant.Name + tenantAppsCertSuffix
}

// ensureTenantApps creates/updates the per-tenant Certificate and the
// per-tenant listener on the shared apps Gateway.
func (r *TenantReconciler) ensureTenantApps(ctx context.Context, tenant *kubespacesv1alpha1.Tenant) error {
	if err := r.ensureAppsCertificate(ctx, tenant); err != nil {
		return fmt.Errorf("ensuring apps certificate: %w", err)
	}
	if err := r.ensureAppsListener(ctx, tenant); err != nil {
		return fmt.Errorf("ensuring apps listener: %w", err)
	}
	return nil
}

// ensureAppsCertificate creates/updates the cert-manager Certificate for
// *.{tenant}.apps.{domain} in the apps gateway namespace (same namespace as
// the Gateway, so no ReferenceGrant is needed for certificateRefs).
func (r *TenantReconciler) ensureAppsCertificate(ctx context.Context, tenant *kubespacesv1alpha1.Tenant) error {
	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certificateGVK)
	cert.SetName(tenantAppsCertName(tenant))
	cert.SetNamespace(r.Apps.GatewayNamespace)

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cert, func() error {
		labels := cert.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[managedByLabel] = managedByValue
		labels[tenantLabel] = tenant.Name
		cert.SetLabels(labels)
		return unstructured.SetNestedMap(cert.Object, map[string]any{
			"secretName": tenantAppsCertName(tenant),
			"dnsNames":   []any{r.Apps.WildcardFor(tenant)},
			"issuerRef": map[string]any{
				"kind": "ClusterIssuer",
				"name": r.Apps.ClusterIssuer,
			},
		}, "spec")
	})
	return err
}

// ensureAppsListener upserts the tenant's listener on the shared apps
// Gateway: wildcard hostname, TLS terminate with the tenant certificate, and
// allowedRoutes restricted to namespaces labeled with this tenant — which is
// what makes cross-tenant hostname theft structurally impossible.
func (r *TenantReconciler) ensureAppsListener(ctx context.Context, tenant *kubespacesv1alpha1.Tenant) error {
	gateway := &gatewayv1.Gateway{}
	key := types.NamespacedName{Name: r.Apps.GatewayName, Namespace: r.Apps.GatewayNamespace}
	if err := r.Get(ctx, key, gateway); err != nil {
		return fmt.Errorf("getting apps gateway %s: %w", key, err)
	}

	listener := gatewayv1.Listener{
		Name:     tenantListenerName(tenant),
		Hostname: ptrTo(gatewayv1.Hostname(r.Apps.WildcardFor(tenant))),
		Port:     appsListenerPort,
		Protocol: gatewayv1.HTTPSProtocolType,
		TLS: &gatewayv1.GatewayTLSConfig{
			Mode: ptrTo(gatewayv1.TLSModeTerminate),
			CertificateRefs: []gatewayv1.SecretObjectReference{{
				Name: gatewayv1.ObjectName(tenantAppsCertName(tenant)),
			}},
		},
		AllowedRoutes: &gatewayv1.AllowedRoutes{
			Namespaces: &gatewayv1.RouteNamespaces{
				From: ptrTo(gatewayv1.NamespacesFromSelector),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenant.Name},
				},
			},
		},
	}

	for i, existing := range gateway.Spec.Listeners {
		if existing.Name == listener.Name {
			gateway.Spec.Listeners[i] = listener
			return r.Update(ctx, gateway)
		}
	}
	gateway.Spec.Listeners = append(gateway.Spec.Listeners, listener)
	return r.Update(ctx, gateway)
}

// deleteTenantApps removes the tenant listener from the apps Gateway and
// deletes the Certificate + its Secret (cert-manager does not garbage-collect
// issued Secrets by default).
func (r *TenantReconciler) deleteTenantApps(ctx context.Context, tenant *kubespacesv1alpha1.Tenant) error {
	gateway := &gatewayv1.Gateway{}
	key := types.NamespacedName{Name: r.Apps.GatewayName, Namespace: r.Apps.GatewayNamespace}
	err := r.Get(ctx, key, gateway)
	switch {
	case apierrors.IsNotFound(err):
		// Gateway gone: nothing to prune from it.
	case err != nil:
		return fmt.Errorf("getting apps gateway %s: %w", key, err)
	default:
		name := tenantListenerName(tenant)
		kept := gateway.Spec.Listeners[:0]
		for _, l := range gateway.Spec.Listeners {
			if l.Name != name {
				kept = append(kept, l)
			}
		}
		if len(kept) != len(gateway.Spec.Listeners) {
			gateway.Spec.Listeners = kept
			if err := r.Update(ctx, gateway); err != nil {
				return fmt.Errorf("removing listener %s from apps gateway: %w", name, err)
			}
		}
	}

	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certificateGVK)
	cert.SetName(tenantAppsCertName(tenant))
	cert.SetNamespace(r.Apps.GatewayNamespace)
	// Tolerate a missing cert-manager CRD (uninstalled before the tenant).
	if err := r.Delete(ctx, cert); err != nil && !apierrors.IsNotFound(err) && !apimeta.IsNoMatchError(err) {
		return fmt.Errorf("deleting apps certificate: %w", err)
	}

	secret := &unstructured.Unstructured{}
	secret.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Secret"})
	secret.SetName(tenantAppsCertName(tenant))
	secret.SetNamespace(r.Apps.GatewayNamespace)
	if err := r.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting apps certificate secret: %w", err)
	}
	return nil
}

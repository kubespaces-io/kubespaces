// Tenant API exposure (roadmap #15, decision D17): a per-tenant TLSRoute (SNI
// passthrough) on a shared Gateway plus a ReferenceGrant in the tenant
// namespace. Ported from the pre-OSS kubespaces-infra tenant chart, which did
// the same with per-tenant `helm install`s.
package controller

import (
	"context"
	"fmt"
	"os"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	kubespacesv1alpha1 "github.com/kubespaces-io/kubespaces/operator/api/v1alpha1"
)

// Environment variables configuring tenant API exposure. All of gateway
// namespace, gateway name and domain must be set for the feature to activate;
// otherwise the operator provisions tenants without public endpoints (demo
// tier, port-forward access).
const (
	// EnvGatewayNamespace is the namespace of the shared Gateway that carries
	// tenant API traffic (e.g. kubespaces-system).
	EnvGatewayNamespace = "KUBESPACES_GATEWAY_NAMESPACE"
	// EnvGatewayName is the name of that Gateway.
	EnvGatewayName = "KUBESPACES_GATEWAY_NAME"
	// EnvGatewaySection optionally pins routes to a listener section (e.g. a
	// TLS passthrough listener named "apipassthrough").
	EnvGatewaySection = "KUBESPACES_GATEWAY_SECTION"
	// EnvAPIDomain is the base domain: tenant API servers are exposed at
	// <tenant>.api.<domain> (one wildcard record *.api.<domain> covers all).
	EnvAPIDomain = "KUBESPACES_API_DOMAIN"
	// EnvExternalDNSTarget optionally overrides the external-dns target (e.g.
	// the public IP of an on-prem gateway appliance).
	EnvExternalDNSTarget = "KUBESPACES_EXTERNAL_DNS_TARGET"
)

const (
	// tlsRoutePrefix names the per-tenant TLSRoute in the gateway namespace.
	tlsRoutePrefix = "tenant-api-"
	// referenceGrantName is the ReferenceGrant in the tenant namespace that
	// lets the gateway-namespace TLSRoute target the vCluster Service.
	referenceGrantName = "kubespaces-tenant-api"
	// vclusterAPIPort is the port the vCluster proxy serves on.
	vclusterAPIPort = 443

	externalDNSHostnameAnnotation = "external-dns.alpha.kubernetes.io/hostname"
	externalDNSTargetAnnotation   = "external-dns.alpha.kubernetes.io/target"
)

// NetworkingConfig configures public exposure of tenant API servers.
type NetworkingConfig struct {
	GatewayNamespace  string
	GatewayName       string
	GatewaySection    string
	APIDomain         string
	ExternalDNSTarget string
}

// NetworkingConfigFromEnv reads the KUBESPACES_GATEWAY_* / KUBESPACES_API_*
// environment variables.
func NetworkingConfigFromEnv() NetworkingConfig {
	return NetworkingConfig{
		GatewayNamespace:  os.Getenv(EnvGatewayNamespace),
		GatewayName:       os.Getenv(EnvGatewayName),
		GatewaySection:    os.Getenv(EnvGatewaySection),
		APIDomain:         os.Getenv(EnvAPIDomain),
		ExternalDNSTarget: os.Getenv(EnvExternalDNSTarget),
	}
}

// Enabled reports whether tenant API exposure is configured.
func (n NetworkingConfig) Enabled() bool {
	return n.GatewayNamespace != "" && n.GatewayName != "" && n.APIDomain != ""
}

// HostFor returns the public API hostname for a tenant.
func (n NetworkingConfig) HostFor(tenant *kubespacesv1alpha1.Tenant) string {
	return fmt.Sprintf("%s.api.%s", tenant.Name, n.APIDomain)
}

// URLFor returns the public API server URL for a tenant.
func (n NetworkingConfig) URLFor(tenant *kubespacesv1alpha1.Tenant) string {
	return fmt.Sprintf("https://%s:%d", n.HostFor(tenant), vclusterAPIPort)
}

func tlsRouteName(tenant *kubespacesv1alpha1.Tenant) string {
	return tlsRoutePrefix + tenant.Name
}

// ensureTenantNetworking creates/updates the ReferenceGrant (tenant namespace)
// and TLSRoute (gateway namespace) exposing the tenant API server.
func (r *TenantReconciler) ensureTenantNetworking(ctx context.Context, tenant *kubespacesv1alpha1.Tenant, namespaceName string) error {
	if err := r.ensureReferenceGrant(ctx, tenant, namespaceName); err != nil {
		return fmt.Errorf("ensuring reference grant: %w", err)
	}
	if err := r.ensureTLSRoute(ctx, tenant, namespaceName); err != nil {
		return fmt.Errorf("ensuring tls route: %w", err)
	}
	return nil
}

// ensureReferenceGrant allows TLSRoutes in the gateway namespace to reference
// Services in the tenant namespace. The grant lives in the tenant namespace
// (Gateway API requires the target namespace to consent) and disappears with
// it on teardown.
func (r *TenantReconciler) ensureReferenceGrant(ctx context.Context, tenant *kubespacesv1alpha1.Tenant, namespaceName string) error {
	grant := &gatewayv1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{Name: referenceGrantName, Namespace: namespaceName},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, grant, func() error {
		if grant.Labels == nil {
			grant.Labels = map[string]string{}
		}
		grant.Labels[managedByLabel] = managedByValue
		grant.Labels[tenantLabel] = tenant.Name
		grant.Spec = gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{{
				Group:     gatewayv1.GroupName,
				Kind:      "TLSRoute",
				Namespace: gatewayv1.Namespace(r.Networking.GatewayNamespace),
			}},
			To: []gatewayv1beta1.ReferenceGrantTo{{
				Group: "",
				Kind:  "Service",
			}},
		}
		return nil
	})
	return err
}

// ensureTLSRoute creates/updates the SNI-passthrough route from the shared
// Gateway to the tenant's vCluster Service. The external-dns annotations make
// per-tenant DNS automatic where external-dns runs; a wildcard *.api.<domain>
// record covers all tenants otherwise.
func (r *TenantReconciler) ensureTLSRoute(ctx context.Context, tenant *kubespacesv1alpha1.Tenant, namespaceName string) error {
	host := r.Networking.HostFor(tenant)

	route := &gatewayv1alpha2.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{Name: tlsRouteName(tenant), Namespace: r.Networking.GatewayNamespace},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, route, func() error {
		if route.Labels == nil {
			route.Labels = map[string]string{}
		}
		route.Labels[managedByLabel] = managedByValue
		route.Labels[tenantLabel] = tenant.Name
		if route.Annotations == nil {
			route.Annotations = map[string]string{}
		}
		route.Annotations[externalDNSHostnameAnnotation] = host
		if r.Networking.ExternalDNSTarget != "" {
			route.Annotations[externalDNSTargetAnnotation] = r.Networking.ExternalDNSTarget
		} else {
			delete(route.Annotations, externalDNSTargetAnnotation)
		}

		parentRef := gatewayv1.ParentReference{
			Name:      gatewayv1.ObjectName(r.Networking.GatewayName),
			Namespace: ptrTo(gatewayv1.Namespace(r.Networking.GatewayNamespace)),
		}
		if r.Networking.GatewaySection != "" {
			parentRef.SectionName = ptrTo(gatewayv1.SectionName(r.Networking.GatewaySection))
		}

		route.Spec = gatewayv1alpha2.TLSRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{parentRef},
			},
			Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(host)},
			Rules: []gatewayv1alpha2.TLSRouteRule{{
				BackendRefs: []gatewayv1.BackendRef{{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						// vCluster's proxy Service is named after the release
						// (= tenant name) in the tenant namespace.
						Name:      gatewayv1.ObjectName(tenant.Name),
						Namespace: ptrTo(gatewayv1.Namespace(namespaceName)),
						Port:      ptrTo(gatewayv1.PortNumber(vclusterAPIPort)),
					},
				}},
			}},
		}
		return nil
	})
	return err
}

// deleteTenantNetworking removes the TLSRoute from the gateway namespace. It
// lives outside the tenant namespace, so namespace teardown cannot collect it
// (and cross-namespace owner references are not allowed).
func (r *TenantReconciler) deleteTenantNetworking(ctx context.Context, tenant *kubespacesv1alpha1.Tenant) error {
	route := &gatewayv1alpha2.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{Name: tlsRouteName(tenant), Namespace: r.Networking.GatewayNamespace},
	}
	if err := r.Delete(ctx, route, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting tls route %s/%s: %w", r.Networking.GatewayNamespace, tlsRouteName(tenant), err)
	}
	return nil
}

func ptrTo[T any](v T) *T { return &v }

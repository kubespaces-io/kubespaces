package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	kubespacesv1alpha1 "github.com/kubespaces-io/kubespaces/operator/api/v1alpha1"
)

func testApps() AppsConfig {
	return AppsConfig{
		GatewayNamespace: "kubespaces-system",
		GatewayName:      "kubespaces-apps",
		Domain:           "example.com",
		ClusterIssuer:    "test-issuer",
	}
}

func appsGateway() *gatewayv1.Gateway {
	return &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "kubespaces-apps", Namespace: "kubespaces-system"},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: "envoy",
			Listeners: []gatewayv1.Listener{{
				Name:     "http",
				Port:     80,
				Protocol: gatewayv1.HTTPProtocolType,
			}},
		},
	}
}

func getAppsGateway(t *testing.T, r *TenantReconciler) *gatewayv1.Gateway {
	t.Helper()
	gw := &gatewayv1.Gateway{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "kubespaces-apps", Namespace: "kubespaces-system"}, gw); err != nil {
		t.Fatalf("getting apps gateway: %v", err)
	}
	return gw
}

func TestReconcileAppsAddsListenerAndCertificate(t *testing.T) {
	// Arrange
	tenant, statefulSet, secret := readyTenantObjects("shop")
	prov := &fakeProvisioner{}
	r, c := newReconciler(t, prov, tenant, statefulSet, secret, appsGateway())
	r.Apps = testApps()

	// Act
	reconcile(t, r, tenant.Name, 1)

	// Assert: listener appended, base listener untouched.
	gw := getAppsGateway(t, r)
	if len(gw.Spec.Listeners) != 2 {
		t.Fatalf("listeners = %d, want 2", len(gw.Spec.Listeners))
	}
	l := gw.Spec.Listeners[1]
	if string(l.Name) != "t-shop" || l.Hostname == nil || string(*l.Hostname) != "*.shop.apps.example.com" {
		t.Errorf("listener = %+v, want t-shop / *.shop.apps.example.com", l)
	}
	if l.TLS == nil || len(l.TLS.CertificateRefs) != 1 || string(l.TLS.CertificateRefs[0].Name) != "shop-apps-tls" {
		t.Errorf("listener TLS = %+v, want certRef shop-apps-tls", l.TLS)
	}
	if l.AllowedRoutes == nil || l.AllowedRoutes.Namespaces == nil ||
		l.AllowedRoutes.Namespaces.Selector.MatchLabels[tenantLabel] != "shop" {
		t.Errorf("allowedRoutes = %+v, want selector %s=shop", l.AllowedRoutes, tenantLabel)
	}

	// Assert: Certificate created in the gateway namespace.
	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certificateGVK)
	if err := c.Get(context.Background(), types.NamespacedName{Name: "shop-apps-tls", Namespace: "kubespaces-system"}, cert); err != nil {
		t.Fatalf("getting certificate: %v", err)
	}
	dnsNames, _, _ := unstructured.NestedStringSlice(cert.Object, "spec", "dnsNames")
	if len(dnsNames) != 1 || dnsNames[0] != "*.shop.apps.example.com" {
		t.Errorf("dnsNames = %v", dnsNames)
	}
	issuer, _, _ := unstructured.NestedString(cert.Object, "spec", "issuerRef", "name")
	if issuer != "test-issuer" {
		t.Errorf("issuerRef.name = %q", issuer)
	}

	// Assert: status + provisioner sync flag.
	got := getTenant(t, c, tenant.Name)
	if got.Status.AppsDomain != "*.shop.apps.example.com" {
		t.Errorf("appsDomain = %q", got.Status.AppsDomain)
	}
	if len(prov.installs) == 0 || !prov.installs[len(prov.installs)-1].SyncGatewayAPI {
		t.Error("provision request should enable Gateway API sync")
	}

	// Act again: reconcile must be idempotent (no duplicate listener).
	reconcile(t, r, tenant.Name, 1)
	if gw := getAppsGateway(t, r); len(gw.Spec.Listeners) != 2 {
		t.Errorf("listeners after second reconcile = %d, want 2", len(gw.Spec.Listeners))
	}
}

func TestReconcileAppsDisabledTouchesNothing(t *testing.T) {
	// Arrange
	tenant, statefulSet, secret := readyTenantObjects("quiet")
	prov := &fakeProvisioner{}
	r, c := newReconciler(t, prov, tenant, statefulSet, secret, appsGateway())

	// Act
	reconcile(t, r, tenant.Name, 1)

	// Assert
	gw := getAppsGateway(t, r)
	if len(gw.Spec.Listeners) != 1 {
		t.Errorf("listeners = %d, want 1 (untouched)", len(gw.Spec.Listeners))
	}
	if got := getTenant(t, c, tenant.Name); got.Status.AppsDomain != "" {
		t.Errorf("appsDomain = %q, want empty", got.Status.AppsDomain)
	}
	if len(prov.installs) == 0 || prov.installs[len(prov.installs)-1].SyncGatewayAPI {
		t.Error("provision request should not enable Gateway API sync")
	}
}

func TestReconcileDeleteRemovesListenerAndCertificate(t *testing.T) {
	// Arrange: tenant under deletion, gateway with its listener, cert present.
	tenant := newTenant("bye", func(tn *kubespacesv1alpha1.Tenant) {
		tn.Finalizers = []string{kubespacesv1alpha1.TenantFinalizer}
		now := metav1.Now()
		tn.DeletionTimestamp = &now
	})
	gw := appsGateway()
	gw.Spec.Listeners = append(gw.Spec.Listeners, gatewayv1.Listener{
		Name:     "t-bye",
		Port:     443,
		Protocol: gatewayv1.HTTPSProtocolType,
	})
	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certificateGVK)
	cert.SetName("bye-apps-tls")
	cert.SetNamespace("kubespaces-system")
	prov := &fakeProvisioner{}
	r, c := newReconciler(t, prov, tenant, gw, cert)
	r.Apps = testApps()

	// Act
	reconcile(t, r, tenant.Name, 1)

	// Assert: listener pruned, other listeners intact, certificate gone.
	after := getAppsGateway(t, r)
	if len(after.Spec.Listeners) != 1 || after.Spec.Listeners[0].Name != "http" {
		t.Errorf("listeners after delete = %+v, want only http", after.Spec.Listeners)
	}
	err := c.Get(context.Background(), types.NamespacedName{Name: "bye-apps-tls", Namespace: "kubespaces-system"}, cert)
	if err == nil {
		t.Error("certificate should be deleted")
	}
}

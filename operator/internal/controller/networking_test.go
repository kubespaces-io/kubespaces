package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	kubespacesv1alpha1 "github.com/kubespaces-io/kubespaces/operator/api/v1alpha1"
)

func testNetworking() NetworkingConfig {
	return NetworkingConfig{
		GatewayNamespace: "kubespaces-system",
		GatewayName:      "kubespaces",
		GatewaySection:   "apipassthrough",
		APIDomain:        "example.com",
	}
}

// readyTenantObjects returns a tenant plus the StatefulSet/Secret that make
// vclusterReady pass.
func readyTenantObjects(name string) (*kubespacesv1alpha1.Tenant, *appsv1.StatefulSet, *corev1.Secret) {
	tenant := newTenant(name, func(tn *kubespacesv1alpha1.Tenant) {
		tn.Finalizers = []string{kubespacesv1alpha1.TenantFinalizer}
	})
	namespaceName := tenant.TargetNamespace()
	replicas := int32(1)
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: tenant.Name, Namespace: namespaceName},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 1},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "vc-" + tenant.Name, Namespace: namespaceName},
		Data:       map[string][]byte{"config": []byte("kubeconfig")},
	}
	return tenant, statefulSet, secret
}

func TestReconcileNetworkingCreatesRouteAndGrant(t *testing.T) {
	// Arrange
	tenant, statefulSet, secret := readyTenantObjects("net-tenant")
	prov := &fakeProvisioner{}
	r, c := newReconciler(t, prov, tenant, statefulSet, secret)
	r.Networking = testNetworking()

	// Act
	reconcile(t, r, tenant.Name, 1)

	// Assert: TLSRoute in the gateway namespace, wired to gateway and backend.
	route := &gatewayv1alpha2.TLSRoute{}
	routeKey := types.NamespacedName{Name: "tenant-api-net-tenant", Namespace: "kubespaces-system"}
	if err := c.Get(context.Background(), routeKey, route); err != nil {
		t.Fatalf("getting TLSRoute: %v", err)
	}
	if got, want := string(route.Spec.Hostnames[0]), "net-tenant.api.example.com"; got != want {
		t.Errorf("hostname = %q, want %q", got, want)
	}
	parent := route.Spec.ParentRefs[0]
	if string(parent.Name) != "kubespaces" || parent.SectionName == nil || string(*parent.SectionName) != "apipassthrough" {
		t.Errorf("parentRef = %+v, want gateway kubespaces section apipassthrough", parent)
	}
	backend := route.Spec.Rules[0].BackendRefs[0]
	if string(backend.Name) != tenant.Name || backend.Namespace == nil || string(*backend.Namespace) != tenant.TargetNamespace() {
		t.Errorf("backendRef = %+v, want service %s in %s", backend, tenant.Name, tenant.TargetNamespace())
	}
	if got := route.Annotations[externalDNSHostnameAnnotation]; got != "net-tenant.api.example.com" {
		t.Errorf("external-dns hostname annotation = %q", got)
	}

	// Assert: ReferenceGrant in the tenant namespace.
	grant := &gatewayv1beta1.ReferenceGrant{}
	grantKey := types.NamespacedName{Name: referenceGrantName, Namespace: tenant.TargetNamespace()}
	if err := c.Get(context.Background(), grantKey, grant); err != nil {
		t.Fatalf("getting ReferenceGrant: %v", err)
	}
	if string(grant.Spec.From[0].Namespace) != "kubespaces-system" || string(grant.Spec.From[0].Kind) != "TLSRoute" {
		t.Errorf("grant from = %+v, want TLSRoute in kubespaces-system", grant.Spec.From[0])
	}

	// Assert: status carries the public URL and the provisioner request the
	// SAN + kubeconfig server values.
	got := getTenant(t, c, tenant.Name)
	if got.Status.APIServerURL != "https://net-tenant.api.example.com:443" {
		t.Errorf("apiServerUrl = %q", got.Status.APIServerURL)
	}
	if len(prov.installs) == 0 {
		t.Fatal("provisioner not called")
	}
	lastInstall := prov.installs[len(prov.installs)-1]
	if lastInstall.PublicAPIHost != "net-tenant.api.example.com" ||
		lastInstall.PublicAPIURL != "https://net-tenant.api.example.com:443" {
		t.Errorf("provision request public endpoint = %q / %q", lastInstall.PublicAPIHost, lastInstall.PublicAPIURL)
	}
}

func TestReconcileNetworkingDisabledCreatesNothing(t *testing.T) {
	// Arrange: no networking config.
	tenant, statefulSet, secret := readyTenantObjects("plain-tenant")
	prov := &fakeProvisioner{}
	r, c := newReconciler(t, prov, tenant, statefulSet, secret)

	// Act
	reconcile(t, r, tenant.Name, 1)

	// Assert
	got := getTenant(t, c, tenant.Name)
	if got.Status.APIServerURL != "" {
		t.Errorf("apiServerUrl = %q, want empty", got.Status.APIServerURL)
	}
	grant := &gatewayv1beta1.ReferenceGrant{}
	err := c.Get(context.Background(), types.NamespacedName{Name: referenceGrantName, Namespace: tenant.TargetNamespace()}, grant)
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected no ReferenceGrant, got err=%v", err)
	}
	if len(prov.installs) == 0 || prov.installs[len(prov.installs)-1].PublicAPIHost != "" {
		t.Errorf("provision request should carry no public endpoint")
	}
}

func TestReconcileDeleteRemovesTLSRoute(t *testing.T) {
	// Arrange: tenant being deleted with an existing TLSRoute in the gateway
	// namespace (outside the tenant namespace, so it needs explicit cleanup).
	tenant := newTenant("gone-tenant", func(tn *kubespacesv1alpha1.Tenant) {
		tn.Finalizers = []string{kubespacesv1alpha1.TenantFinalizer}
		now := metav1.Now()
		tn.DeletionTimestamp = &now
	})
	route := &gatewayv1alpha2.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "tenant-api-gone-tenant", Namespace: "kubespaces-system"},
	}
	prov := &fakeProvisioner{}
	r, c := newReconciler(t, prov, tenant, route)
	r.Networking = testNetworking()

	// Act
	reconcile(t, r, tenant.Name, 1)

	// Assert
	err := c.Get(context.Background(), types.NamespacedName{Name: "tenant-api-gone-tenant", Namespace: "kubespaces-system"}, &gatewayv1alpha2.TLSRoute{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected TLSRoute deleted, got err=%v", err)
	}
}

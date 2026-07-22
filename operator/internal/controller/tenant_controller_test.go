package controller

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	kubespacesv1alpha1 "github.com/kubespaces-io/kubespaces/operator/api/v1alpha1"
	"github.com/kubespaces-io/kubespaces/operator/internal/provisioner"
)

// fakeProvisioner records calls instead of running Helm.
type fakeProvisioner struct {
	installs     []provisioner.Request
	uninstalls   []string
	installErr   error
	uninstallErr error
}

func (f *fakeProvisioner) Install(_ context.Context, req provisioner.Request) error {
	if f.installErr != nil {
		return f.installErr
	}
	f.installs = append(f.installs, req)
	return nil
}

func (f *fakeProvisioner) Uninstall(_ context.Context, releaseName, _ string) error {
	if f.uninstallErr != nil {
		return f.uninstallErr
	}
	f.uninstalls = append(f.uninstalls, releaseName)
	return nil
}

func (f *fakeProvisioner) Status(context.Context, string, string) (string, error) {
	return "deployed", nil
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("adding client-go scheme: %v", err)
	}
	if err := kubespacesv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("adding kubespaces scheme: %v", err)
	}
	if err := gatewayv1.Install(scheme); err != nil {
		t.Fatalf("adding gateway v1 scheme: %v", err)
	}
	if err := gatewayv1alpha2.Install(scheme); err != nil {
		t.Fatalf("adding gateway v1alpha2 scheme: %v", err)
	}
	if err := gatewayv1beta1.Install(scheme); err != nil {
		t.Fatalf("adding gateway v1beta1 scheme: %v", err)
	}
	return scheme
}

func newTenant(name string, mutate func(*kubespacesv1alpha1.Tenant)) *kubespacesv1alpha1.Tenant {
	tenant := &kubespacesv1alpha1.Tenant{
		ObjectMeta: metav1.ObjectMeta{Name: name, Generation: 1},
		Spec:       kubespacesv1alpha1.TenantSpec{Owner: "alice@example.com"},
	}
	if mutate != nil {
		mutate(tenant)
	}
	return tenant
}

func newReconciler(t *testing.T, prov provisioner.Provisioner, objects ...client.Object) (*TenantReconciler, client.Client) {
	t.Helper()
	scheme := newTestScheme(t)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&kubespacesv1alpha1.Tenant{}).
		Build()
	return &TenantReconciler{Client: fakeClient, Scheme: scheme, Provisioner: prov}, fakeClient
}

// reconcile runs Reconcile n times, failing the test on unexpected errors.
func reconcile(t *testing.T, r *TenantReconciler, name string, times int) ctrl.Result {
	t.Helper()
	var result ctrl.Result
	var err error
	for i := 0; i < times; i++ {
		result, err = r.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: types.NamespacedName{Name: name},
		})
		if err != nil {
			t.Fatalf("reconcile %d/%d: %v", i+1, times, err)
		}
	}
	return result
}

func getTenant(t *testing.T, c client.Client, name string) *kubespacesv1alpha1.Tenant {
	t.Helper()
	tenant := &kubespacesv1alpha1.Tenant{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: name}, tenant); err != nil {
		t.Fatalf("getting tenant %q: %v", name, err)
	}
	return tenant
}

func TestReconcileCreate(t *testing.T) {
	tests := []struct {
		name            string
		tenant          *kubespacesv1alpha1.Tenant
		wantNamespace   string
		wantQuota       bool
		wantQuotaCPU    string
		wantChartVer    string
		wantK8sVersion  string
		wantValuesKey   string
		wantValuesValue any
	}{
		{
			name:          "defaults: namespace derived from name, no quota",
			tenant:        newTenant("acme", nil),
			wantNamespace: "kubespaces-tenant-acme",
			wantQuota:     false,
		},
		{
			name: "explicit targetNamespace and resources create quota",
			tenant: newTenant("bravo", func(tn *kubespacesv1alpha1.Tenant) {
				tn.Spec.TargetNamespace = "custom-ns"
				tn.Spec.Resources = &kubespacesv1alpha1.TenantResources{
					CPU: "4", Memory: "8Gi", Storage: "20Gi",
				}
			}),
			wantNamespace: "custom-ns",
			wantQuota:     true,
			wantQuotaCPU:  "4",
		},
		{
			name: "vcluster options forwarded to provisioner",
			tenant: newTenant("charlie", func(tn *kubespacesv1alpha1.Tenant) {
				tn.Spec.VCluster = &kubespacesv1alpha1.TenantVCluster{
					Version:           "0.25.0",
					KubernetesVersion: "v1.32.1",
					ValuesOverrides:   jsonValue(t, map[string]any{"sync": map[string]any{"toHost": map[string]any{"ingresses": map[string]any{"enabled": true}}}}),
				}
			}),
			wantNamespace:   "kubespaces-tenant-charlie",
			wantQuota:       false,
			wantChartVer:    "0.25.0",
			wantK8sVersion:  "v1.32.1",
			wantValuesKey:   "sync",
			wantValuesValue: map[string]any{"toHost": map[string]any{"ingresses": map[string]any{"enabled": true}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			prov := &fakeProvisioner{}
			r, c := newReconciler(t, prov, tc.tenant)

			// Act: 1st pass adds finalizer, 2nd provisions.
			result := reconcile(t, r, tc.tenant.Name, 2)

			// Assert: finalizer present.
			got := getTenant(t, c, tc.tenant.Name)
			if !containsString(got.Finalizers, kubespacesv1alpha1.TenantFinalizer) {
				t.Errorf("finalizer %q not added, got %v", kubespacesv1alpha1.TenantFinalizer, got.Finalizers)
			}

			// Assert: namespace created with labels.
			namespace := &corev1.Namespace{}
			if err := c.Get(context.Background(), types.NamespacedName{Name: tc.wantNamespace}, namespace); err != nil {
				t.Fatalf("expected namespace %q: %v", tc.wantNamespace, err)
			}
			if namespace.Labels[tenantLabel] != tc.tenant.Name {
				t.Errorf("namespace label %s = %q, want %q", tenantLabel, namespace.Labels[tenantLabel], tc.tenant.Name)
			}

			// Assert: quota presence matches expectation.
			quota := &corev1.ResourceQuota{}
			quotaErr := c.Get(context.Background(), types.NamespacedName{
				Name: tc.tenant.Name + resourceQuotaSuffix, Namespace: tc.wantNamespace,
			}, quota)
			if tc.wantQuota {
				if quotaErr != nil {
					t.Fatalf("expected resource quota: %v", quotaErr)
				}
				if cpu := quota.Spec.Hard[corev1.ResourceRequestsCPU]; cpu.String() != tc.wantQuotaCPU {
					t.Errorf("quota requests.cpu = %s, want %s", cpu.String(), tc.wantQuotaCPU)
				}
			} else if !apierrors.IsNotFound(quotaErr) {
				t.Errorf("expected no resource quota, got err=%v", quotaErr)
			}

			// Assert: a LimitRange with container defaults accompanies any
			// quota that constrains limits (pods without limits must still
			// be admitted).
			limitRange := &corev1.LimitRange{}
			limitRangeErr := c.Get(context.Background(), types.NamespacedName{
				Name: tc.tenant.Name + limitRangeSuffix, Namespace: tc.wantNamespace,
			}, limitRange)
			if tc.wantQuota {
				if limitRangeErr != nil {
					t.Fatalf("expected limit range: %v", limitRangeErr)
				}
				if len(limitRange.Spec.Limits) == 0 || limitRange.Spec.Limits[0].Default.Cpu().IsZero() {
					t.Errorf("limit range has no container default limits: %+v", limitRange.Spec.Limits)
				}
			} else if !apierrors.IsNotFound(limitRangeErr) {
				t.Errorf("expected no limit range, got err=%v", limitRangeErr)
			}

			// Assert: provisioner called with the right request.
			if len(prov.installs) == 0 {
				t.Fatal("provisioner.Install not called")
			}
			installReq := prov.installs[len(prov.installs)-1]
			if installReq.ReleaseName != tc.tenant.Name || installReq.Namespace != tc.wantNamespace {
				t.Errorf("install request = %+v, want release %q in %q", installReq, tc.tenant.Name, tc.wantNamespace)
			}
			if installReq.ChartVersion != tc.wantChartVer {
				t.Errorf("chart version = %q, want %q", installReq.ChartVersion, tc.wantChartVer)
			}
			if installReq.KubernetesVersion != tc.wantK8sVersion {
				t.Errorf("kubernetes version = %q, want %q", installReq.KubernetesVersion, tc.wantK8sVersion)
			}
			if tc.wantValuesKey != "" {
				if diff := installReq.ValuesOverrides[tc.wantValuesKey]; diff == nil {
					t.Errorf("values overrides missing key %q: %+v", tc.wantValuesKey, installReq.ValuesOverrides)
				}
			}

			// Assert: not ready yet -> Provisioning with requeue.
			if got.Status.Phase != kubespacesv1alpha1.TenantPhaseProvisioning {
				t.Errorf("phase = %q, want %q", got.Status.Phase, kubespacesv1alpha1.TenantPhaseProvisioning)
			}
			if result.RequeueAfter != provisioningRequeueInterval {
				t.Errorf("requeue after = %s, want %s", result.RequeueAfter, provisioningRequeueInterval)
			}
			if got.Status.ObservedGeneration != tc.tenant.Generation {
				t.Errorf("observedGeneration = %d, want %d", got.Status.ObservedGeneration, tc.tenant.Generation)
			}
		})
	}
}

func TestReconcileReady(t *testing.T) {
	// Arrange: tenant with finalizer plus a ready StatefulSet and kubeconfig Secret.
	tenant := newTenant("ready-tenant", func(tn *kubespacesv1alpha1.Tenant) {
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
	prov := &fakeProvisioner{}
	r, c := newReconciler(t, prov, tenant, statefulSet, secret)

	// Act
	result := reconcile(t, r, tenant.Name, 1)

	// Assert
	got := getTenant(t, c, tenant.Name)
	if got.Status.Phase != kubespacesv1alpha1.TenantPhaseReady {
		t.Fatalf("phase = %q (%s), want Ready", got.Status.Phase, got.Status.Message)
	}
	if got.Status.KubeconfigSecretRef == nil ||
		got.Status.KubeconfigSecretRef.Name != "vc-ready-tenant" ||
		got.Status.KubeconfigSecretRef.Key != "config" {
		t.Errorf("kubeconfigSecretRef = %+v, want {vc-ready-tenant config}", got.Status.KubeconfigSecretRef)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("requeue after = %s, want 0", result.RequeueAfter)
	}
	readyCondition := findCondition(got.Status.Conditions, kubespacesv1alpha1.ConditionReady)
	if readyCondition == nil || readyCondition.Status != metav1.ConditionTrue {
		t.Errorf("Ready condition = %+v, want True", readyCondition)
	}
}

func TestReconcileNotReadyWithoutKubeconfigSecret(t *testing.T) {
	// Arrange: ready StatefulSet but no vc-<name> secret.
	tenant := newTenant("no-secret", func(tn *kubespacesv1alpha1.Tenant) {
		tn.Finalizers = []string{kubespacesv1alpha1.TenantFinalizer}
	})
	replicas := int32(1)
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: tenant.Name, Namespace: tenant.TargetNamespace()},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 1},
	}
	r, c := newReconciler(t, &fakeProvisioner{}, tenant, statefulSet)

	// Act
	result := reconcile(t, r, tenant.Name, 1)

	// Assert
	got := getTenant(t, c, tenant.Name)
	if got.Status.Phase != kubespacesv1alpha1.TenantPhaseProvisioning {
		t.Errorf("phase = %q, want Provisioning", got.Status.Phase)
	}
	if got.Status.KubeconfigSecretRef != nil {
		t.Errorf("kubeconfigSecretRef = %+v, want nil", got.Status.KubeconfigSecretRef)
	}
	if result.RequeueAfter != provisioningRequeueInterval {
		t.Errorf("requeue after = %s, want %s", result.RequeueAfter, provisioningRequeueInterval)
	}
}

func TestReconcileDelete(t *testing.T) {
	// Arrange: tenant with finalizer, its namespace already exists.
	tenant := newTenant("doomed", func(tn *kubespacesv1alpha1.Tenant) {
		tn.Finalizers = []string{kubespacesv1alpha1.TenantFinalizer}
	})
	namespaceName := tenant.TargetNamespace()
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	prov := &fakeProvisioner{}
	r, c := newReconciler(t, prov, tenant, namespace)
	ctx := context.Background()

	// Act: delete sets deletionTimestamp (finalizer blocks removal), then reconcile.
	if err := c.Delete(ctx, tenant); err != nil {
		t.Fatalf("deleting tenant: %v", err)
	}
	reconcile(t, r, tenant.Name, 1)

	// Assert: vCluster uninstalled.
	if len(prov.uninstalls) != 1 || prov.uninstalls[0] != "doomed" {
		t.Errorf("uninstalls = %v, want [doomed]", prov.uninstalls)
	}
	// Assert: namespace deleted.
	nsErr := c.Get(ctx, types.NamespacedName{Name: namespaceName}, &corev1.Namespace{})
	if !apierrors.IsNotFound(nsErr) {
		t.Errorf("namespace still present (err=%v), want NotFound", nsErr)
	}
	// Assert: finalizer removed -> CR gone.
	tenantErr := c.Get(ctx, types.NamespacedName{Name: tenant.Name}, &kubespacesv1alpha1.Tenant{})
	if !apierrors.IsNotFound(tenantErr) {
		t.Errorf("tenant still present (err=%v), want NotFound", tenantErr)
	}
}

func TestReconcileDeleteUninstallErrorKeepsFinalizer(t *testing.T) {
	// Arrange
	tenant := newTenant("stuck", func(tn *kubespacesv1alpha1.Tenant) {
		tn.Finalizers = []string{kubespacesv1alpha1.TenantFinalizer}
	})
	prov := &fakeProvisioner{uninstallErr: errors.New("helm exploded")}
	r, c := newReconciler(t, prov, tenant)
	ctx := context.Background()

	// Act
	if err := c.Delete(ctx, tenant); err != nil {
		t.Fatalf("deleting tenant: %v", err)
	}
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: tenant.Name}})

	// Assert: error surfaces and the CR is still there.
	if err == nil {
		t.Fatal("expected reconcile error, got nil")
	}
	got := getTenant(t, c, tenant.Name)
	if !containsString(got.Finalizers, kubespacesv1alpha1.TenantFinalizer) {
		t.Error("finalizer removed despite uninstall failure")
	}
	if got.Status.Phase != kubespacesv1alpha1.TenantPhaseDeleting {
		t.Errorf("phase = %q, want Deleting", got.Status.Phase)
	}
}

func TestReconcileInstallErrorSetsFailed(t *testing.T) {
	// Arrange
	tenant := newTenant("broken", func(tn *kubespacesv1alpha1.Tenant) {
		tn.Finalizers = []string{kubespacesv1alpha1.TenantFinalizer}
	})
	prov := &fakeProvisioner{installErr: errors.New("chart not found")}
	r, c := newReconciler(t, prov, tenant)

	// Act
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: tenant.Name},
	})

	// Assert
	if err == nil {
		t.Fatal("expected reconcile error, got nil")
	}
	got := getTenant(t, c, tenant.Name)
	if got.Status.Phase != kubespacesv1alpha1.TenantPhaseFailed {
		t.Errorf("phase = %q, want Failed", got.Status.Phase)
	}
	readyCondition := findCondition(got.Status.Conditions, kubespacesv1alpha1.ConditionReady)
	if readyCondition == nil || readyCondition.Reason != "ProvisioningFailed" {
		t.Errorf("Ready condition = %+v, want reason ProvisioningFailed", readyCondition)
	}
}

func TestReconcileInvalidQuotaSetsFailed(t *testing.T) {
	// Arrange
	tenant := newTenant("bad-quota", func(tn *kubespacesv1alpha1.Tenant) {
		tn.Finalizers = []string{kubespacesv1alpha1.TenantFinalizer}
		tn.Spec.Resources = &kubespacesv1alpha1.TenantResources{CPU: "not-a-quantity"}
	})
	r, c := newReconciler(t, &fakeProvisioner{}, tenant)

	// Act
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: tenant.Name},
	})

	// Assert
	if err == nil {
		t.Fatal("expected reconcile error, got nil")
	}
	got := getTenant(t, c, tenant.Name)
	if got.Status.Phase != kubespacesv1alpha1.TenantPhaseFailed {
		t.Errorf("phase = %q, want Failed", got.Status.Phase)
	}
}

func jsonValue(t *testing.T, value map[string]any) *apiextensionsv1.JSON {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshaling values overrides: %v", err)
	}
	return &apiextensionsv1.JSON{Raw: raw}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

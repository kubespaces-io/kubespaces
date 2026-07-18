// Package controller implements the Tenant reconciler: namespace, quota and
// vCluster provisioning for each cluster-scoped Tenant CR.
package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kubespacesv1alpha1 "github.com/kubespaces-io/kubespaces/operator/api/v1alpha1"
	"github.com/kubespaces-io/kubespaces/operator/internal/provisioner"
)

const (
	// provisioningRequeueInterval is how often we re-check vCluster readiness.
	provisioningRequeueInterval = 15 * time.Second

	managedByLabel      = "app.kubernetes.io/managed-by"
	managedByValue      = "kubespaces-operator"
	tenantLabel         = "kubespaces.io/tenant"
	resourceQuotaSuffix = "-quota"
	limitRangeSuffix    = "-limits"
)

// TenantReconciler reconciles a Tenant object.
type TenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// Provisioner performs the actual vCluster install/uninstall. It is an
	// interface so tests can substitute a fake.
	Provisioner provisioner.Provisioner
}

// RBAC for the Tenant CR itself.
// +kubebuilder:rbac:groups=kubespaces.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubespaces.io,resources=tenants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubespaces.io,resources=tenants/finalizers,verbs=update
//
// Tenant namespace + quota management.
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=resourcequotas;limitranges,verbs=get;list;watch;create;update;patch;delete
//
// Resources the Helm SDK creates/reads when installing the vCluster chart
// (workloads, config, networking, RBAC) plus the kubeconfig Secret.
// +kubebuilder:rbac:groups="",resources=secrets;configmaps;serviceaccounts;services;endpoints;persistentvolumeclaims;pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets;deployments;replicasets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete;bind;escalate
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

// Reconcile drives a Tenant to its desired state: namespace, optional
// ResourceQuota, vCluster release, then status.
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	tenant := &kubespacesv1alpha1.Tenant{}
	if err := r.Get(ctx, req.NamespacedName, tenant); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !tenant.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, tenant)
	}

	if controllerutil.AddFinalizer(tenant, kubespacesv1alpha1.TenantFinalizer) {
		if err := r.Update(ctx, tenant); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
		// The update event re-triggers reconciliation.
		return ctrl.Result{}, nil
	}

	if tenant.Status.Phase == "" {
		if err := r.markPhase(ctx, tenant, kubespacesv1alpha1.TenantPhasePending, "Tenant accepted"); err != nil {
			return ctrl.Result{}, err
		}
	}

	namespaceName := tenant.TargetNamespace()

	if err := r.ensureNamespace(ctx, tenant, namespaceName); err != nil {
		return r.fail(ctx, tenant, "NamespaceFailed", fmt.Errorf("ensuring namespace %q: %w", namespaceName, err))
	}
	if err := r.ensureResourceQuota(ctx, tenant, namespaceName); err != nil {
		return r.fail(ctx, tenant, "QuotaFailed", fmt.Errorf("ensuring resource quota: %w", err))
	}
	if err := r.ensureLimitRange(ctx, tenant, namespaceName); err != nil {
		return r.fail(ctx, tenant, "QuotaFailed", fmt.Errorf("ensuring limit range: %w", err))
	}

	provisionReq, err := buildProvisionRequest(tenant, namespaceName)
	if err != nil {
		return r.fail(ctx, tenant, "InvalidSpec", err)
	}
	if err := r.Provisioner.Install(ctx, provisionReq); err != nil {
		return r.fail(ctx, tenant, "ProvisioningFailed", fmt.Errorf("installing vCluster: %w", err))
	}

	ready, waitReason, err := r.vclusterReady(ctx, tenant, namespaceName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("checking vCluster readiness: %w", err)
	}
	if !ready {
		log.V(1).Info("vCluster not ready yet", "tenant", tenant.Name, "reason", waitReason)
		tenant.Status.Phase = kubespacesv1alpha1.TenantPhaseProvisioning
		tenant.Status.Message = waitReason
		tenant.Status.KubeconfigSecretRef = nil
		setReadyCondition(tenant, metav1.ConditionFalse, "Provisioning", waitReason)
		if err := r.updateStatus(ctx, tenant); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: provisioningRequeueInterval}, nil
	}

	tenant.Status.Phase = kubespacesv1alpha1.TenantPhaseReady
	tenant.Status.Message = "vCluster is ready"
	tenant.Status.KubeconfigSecretRef = &kubespacesv1alpha1.SecretKeyRef{
		Name: tenant.KubeconfigSecretName(),
		Key:  kubespacesv1alpha1.KubeconfigSecretKey,
	}
	setReadyCondition(tenant, metav1.ConditionTrue, "Provisioned", "vCluster is ready")
	if err := r.updateStatus(ctx, tenant); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// reconcileDelete tears down the vCluster and namespace, then removes the
// finalizer so the CR can disappear.
func (r *TenantReconciler) reconcileDelete(ctx context.Context, tenant *kubespacesv1alpha1.Tenant) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(tenant, kubespacesv1alpha1.TenantFinalizer) {
		return ctrl.Result{}, nil
	}

	if tenant.Status.Phase != kubespacesv1alpha1.TenantPhaseDeleting {
		if err := r.markPhase(ctx, tenant, kubespacesv1alpha1.TenantPhaseDeleting, "Tearing down vCluster and namespace"); err != nil {
			return ctrl.Result{}, err
		}
	}

	namespaceName := tenant.TargetNamespace()

	if err := r.Provisioner.Uninstall(ctx, tenant.Name, namespaceName); err != nil {
		return ctrl.Result{}, fmt.Errorf("uninstalling vCluster release %q: %w", tenant.Name, err)
	}

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	if err := r.Delete(ctx, namespace); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("deleting namespace %q: %w", namespaceName, err)
	}

	controllerutil.RemoveFinalizer(tenant, kubespacesv1alpha1.TenantFinalizer)
	if err := r.Update(ctx, tenant); err != nil {
		return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
	}
	log.Info("tenant deleted", "tenant", tenant.Name, "namespace", namespaceName)
	return ctrl.Result{}, nil
}

// ensureNamespace creates the tenant namespace if it does not exist.
func (r *TenantReconciler) ensureNamespace(ctx context.Context, tenant *kubespacesv1alpha1.Tenant, name string) error {
	namespace := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: name}, namespace)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				managedByLabel: managedByValue,
				tenantLabel:    tenant.Name,
			},
		},
	}
	if err := controllerutil.SetControllerReference(tenant, namespace, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}
	if err := r.Create(ctx, namespace); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// ensureResourceQuota creates/updates the quota built from spec.resources, or
// removes it when spec.resources is absent/empty.
func (r *TenantReconciler) ensureResourceQuota(ctx context.Context, tenant *kubespacesv1alpha1.Tenant, namespaceName string) error {
	quotaName := tenant.Name + resourceQuotaSuffix

	hard, err := quotaHardLimits(tenant.Spec.Resources)
	if err != nil {
		return err
	}

	if len(hard) == 0 {
		quota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespaceName},
		}
		if err := r.Delete(ctx, quota); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespaceName},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, quota, func() error {
		if quota.Labels == nil {
			quota.Labels = map[string]string{}
		}
		quota.Labels[managedByLabel] = managedByValue
		quota.Labels[tenantLabel] = tenant.Name
		quota.Spec.Hard = hard
		return nil
	})
	return err
}

// ensureLimitRange pairs the ResourceQuota with default requests/limits so
// pods that do not declare them (e.g. the vCluster syncer) are still admitted
// in a namespace whose quota constrains limits.cpu/limits.memory. Removed when
// the tenant has no quota.
func (r *TenantReconciler) ensureLimitRange(ctx context.Context, tenant *kubespacesv1alpha1.Tenant, namespaceName string) error {
	limitRangeName := tenant.Name + limitRangeSuffix

	hard, err := quotaHardLimits(tenant.Spec.Resources)
	if err != nil {
		return err
	}

	needsDefaults := false
	for _, name := range []corev1.ResourceName{corev1.ResourceLimitsCPU, corev1.ResourceLimitsMemory} {
		if _, ok := hard[name]; ok {
			needsDefaults = true
		}
	}

	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: limitRangeName, Namespace: namespaceName},
	}
	if !needsDefaults {
		if err := r.Delete(ctx, limitRange); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, limitRange, func() error {
		if limitRange.Labels == nil {
			limitRange.Labels = map[string]string{}
		}
		limitRange.Labels[managedByLabel] = managedByValue
		limitRange.Labels[tenantLabel] = tenant.Name
		limitRange.Spec.Limits = []corev1.LimitRangeItem{
			{
				Type: corev1.LimitTypeContainer,
				Default: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				DefaultRequest: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
		}
		return nil
	})
	return err
}

// quotaHardLimits translates spec.resources into ResourceQuota hard limits.
func quotaHardLimits(resources *kubespacesv1alpha1.TenantResources) (corev1.ResourceList, error) {
	if resources == nil {
		return nil, nil
	}

	hard := corev1.ResourceList{}
	parse := func(field, value string, names ...corev1.ResourceName) error {
		if value == "" {
			return nil
		}
		quantity, err := resource.ParseQuantity(value)
		if err != nil {
			return fmt.Errorf("invalid spec.resources.%s %q: %w", field, value, err)
		}
		for _, name := range names {
			hard[name] = quantity
		}
		return nil
	}

	if err := parse("cpu", resources.CPU, corev1.ResourceRequestsCPU, corev1.ResourceLimitsCPU); err != nil {
		return nil, err
	}
	if err := parse("memory", resources.Memory, corev1.ResourceRequestsMemory, corev1.ResourceLimitsMemory); err != nil {
		return nil, err
	}
	if err := parse("storage", resources.Storage, corev1.ResourceRequestsStorage); err != nil {
		return nil, err
	}
	return hard, nil
}

// buildProvisionRequest converts the Tenant spec into a provisioner request.
func buildProvisionRequest(tenant *kubespacesv1alpha1.Tenant, namespaceName string) (provisioner.Request, error) {
	req := provisioner.Request{
		ReleaseName: tenant.Name,
		Namespace:   namespaceName,
	}
	vcluster := tenant.Spec.VCluster
	if vcluster == nil {
		return req, nil
	}
	req.ChartVersion = vcluster.Version
	req.KubernetesVersion = vcluster.KubernetesVersion
	if vcluster.ValuesOverrides != nil && len(vcluster.ValuesOverrides.Raw) > 0 {
		overrides := map[string]any{}
		if err := json.Unmarshal(vcluster.ValuesOverrides.Raw, &overrides); err != nil {
			return req, fmt.Errorf("invalid spec.vcluster.valuesOverrides: %w", err)
		}
		req.ValuesOverrides = overrides
	}
	return req, nil
}

// vclusterReady reports whether the vCluster workload is ready and the
// kubeconfig Secret exists.
func (r *TenantReconciler) vclusterReady(ctx context.Context, tenant *kubespacesv1alpha1.Tenant, namespaceName string) (bool, string, error) {
	workloadReady, reason, err := r.workloadReady(ctx, tenant.Name, namespaceName)
	if err != nil {
		return false, "", err
	}
	if !workloadReady {
		return false, reason, nil
	}

	secret := &corev1.Secret{}
	secretName := tenant.KubeconfigSecretName()
	err = r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespaceName}, secret)
	if apierrors.IsNotFound(err) {
		return false, fmt.Sprintf("waiting for kubeconfig secret %s/%s", namespaceName, secretName), nil
	}
	if err != nil {
		return false, "", err
	}
	if _, ok := secret.Data[kubespacesv1alpha1.KubeconfigSecretKey]; !ok {
		return false, fmt.Sprintf("kubeconfig secret %s/%s missing key %q", namespaceName, secretName, kubespacesv1alpha1.KubeconfigSecretKey), nil
	}
	return true, "", nil
}

// workloadReady checks the vCluster StatefulSet (default) or Deployment.
func (r *TenantReconciler) workloadReady(ctx context.Context, name, namespaceName string) (bool, string, error) {
	key := types.NamespacedName{Name: name, Namespace: namespaceName}

	statefulSet := &appsv1.StatefulSet{}
	err := r.Get(ctx, key, statefulSet)
	if err == nil {
		desired := int32(1)
		if statefulSet.Spec.Replicas != nil {
			desired = *statefulSet.Spec.Replicas
		}
		if desired > 0 && statefulSet.Status.ReadyReplicas >= desired {
			return true, "", nil
		}
		return false, fmt.Sprintf("waiting for statefulset %s/%s: %d/%d replicas ready",
			namespaceName, name, statefulSet.Status.ReadyReplicas, desired), nil
	}
	if !apierrors.IsNotFound(err) {
		return false, "", err
	}

	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, key, deployment)
	if err == nil {
		desired := int32(1)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}
		if desired > 0 && deployment.Status.ReadyReplicas >= desired {
			return true, "", nil
		}
		return false, fmt.Sprintf("waiting for deployment %s/%s: %d/%d replicas ready",
			namespaceName, name, deployment.Status.ReadyReplicas, desired), nil
	}
	if !apierrors.IsNotFound(err) {
		return false, "", err
	}

	return false, fmt.Sprintf("waiting for vCluster workload %s/%s to appear", namespaceName, name), nil
}

// fail records a Failed phase + condition and returns the error so
// controller-runtime retries with backoff.
func (r *TenantReconciler) fail(ctx context.Context, tenant *kubespacesv1alpha1.Tenant, reason string, cause error) (ctrl.Result, error) {
	logf.FromContext(ctx).Error(cause, "reconciliation failed", "tenant", tenant.Name, "reason", reason)
	tenant.Status.Phase = kubespacesv1alpha1.TenantPhaseFailed
	tenant.Status.Message = cause.Error()
	setReadyCondition(tenant, metav1.ConditionFalse, reason, cause.Error())
	if statusErr := r.updateStatus(ctx, tenant); statusErr != nil {
		logf.FromContext(ctx).Error(statusErr, "updating status after failure")
	}
	return ctrl.Result{}, cause
}

// markPhase sets phase+message and persists status immediately.
func (r *TenantReconciler) markPhase(ctx context.Context, tenant *kubespacesv1alpha1.Tenant, phase kubespacesv1alpha1.TenantPhase, message string) error {
	tenant.Status.Phase = phase
	tenant.Status.Message = message
	return r.updateStatus(ctx, tenant)
}

// updateStatus stamps observedGeneration and writes the status subresource.
func (r *TenantReconciler) updateStatus(ctx context.Context, tenant *kubespacesv1alpha1.Tenant) error {
	tenant.Status.ObservedGeneration = tenant.Generation
	if err := r.Status().Update(ctx, tenant); err != nil {
		return fmt.Errorf("updating tenant status: %w", err)
	}
	return nil
}

func setReadyCondition(tenant *kubespacesv1alpha1.Tenant, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&tenant.Status.Conditions, metav1.Condition{
		Type:               kubespacesv1alpha1.ConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: tenant.Generation,
	})
}

// SetupWithManager wires the reconciler into the manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubespacesv1alpha1.Tenant{}).
		Owns(&corev1.Namespace{}).
		Named("tenant").
		Complete(r)
}

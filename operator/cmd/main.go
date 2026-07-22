// The kubespaces operator watches cluster-scoped Tenant CRs and reconciles
// each into a namespace, an optional ResourceQuota and a vCluster.
package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	kubespacesv1alpha1 "github.com/kubespaces-io/kubespaces/operator/api/v1alpha1"
	"github.com/kubespaces-io/kubespaces/operator/internal/controller"
	"github.com/kubespaces-io/kubespaces/operator/internal/provisioner"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubespacesv1alpha1.AddToScheme(scheme))
	// Gateway API types for tenant API exposure (TLSRoute, ReferenceGrant).
	utilruntime.Must(gatewayv1alpha2.Install(scheme))
	utilruntime.Must(gatewayv1beta1.Install(scheme))
}

func main() {
	var metricsAddr string
	var probeAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. Use :8443 for HTTPS or :8080 for HTTP, or 0 to disable.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	opts := zap.Options{Development: false}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "tenants.kubespaces.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	networking := controller.NetworkingConfigFromEnv()
	if networking.Enabled() {
		setupLog.Info("tenant API exposure enabled",
			"gateway", networking.GatewayNamespace+"/"+networking.GatewayName,
			"domain", networking.APIDomain)
	} else {
		setupLog.Info("tenant API exposure disabled (set KUBESPACES_GATEWAY_NAMESPACE, KUBESPACES_GATEWAY_NAME and KUBESPACES_API_DOMAIN to enable)")
	}

	if err := (&controller.TenantReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		Provisioner: provisioner.NewHelmProvisioner(mgr.GetConfig()),
		Networking:  networking,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Tenant")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

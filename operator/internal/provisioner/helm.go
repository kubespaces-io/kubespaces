package provisioner

import (
	"context"
	"errors"
	"fmt"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// DefaultChartRef is the KubeSpaces-owned OCI mirror of the vCluster chart
	// (D16): upstream repo changes can never break existing installs.
	DefaultChartRef = "oci://ghcr.io/kubespaces-io/charts/vcluster"
	// DefaultChartVersion pins the mirrored vCluster chart version. Bump
	// deliberately (mirror first via the mirror-vcluster-chart workflow).
	DefaultChartVersion = "0.35.2"
	// ChartName is the vCluster chart name (classic repo layout).
	ChartName = "vcluster"

	// EnvChartRef overrides the chart source: an oci:// reference or a classic
	// https:// repo URL.
	EnvChartRef = "VCLUSTER_CHART_REF"
	// EnvChartVersion overrides the default pinned chart version.
	EnvChartVersion = "VCLUSTER_CHART_VERSION"
)

// ErrReleaseNotFound is returned by Status when the release does not exist.
var ErrReleaseNotFound = errors.New("release not found")

// HelmProvisioner provisions vClusters with the Helm SDK using in-cluster
// (or manager) REST credentials.
type HelmProvisioner struct {
	restConfig   *rest.Config
	settings     *cli.EnvSettings
	chartRef     string
	chartVersion string
}

var _ Provisioner = (*HelmProvisioner)(nil)

// NewHelmProvisioner builds a HelmProvisioner from the manager's REST config.
// Chart source and version default to the pinned KubeSpaces OCI mirror and can
// be overridden with VCLUSTER_CHART_REF / VCLUSTER_CHART_VERSION.
func NewHelmProvisioner(cfg *rest.Config) *HelmProvisioner {
	chartRef := os.Getenv(EnvChartRef)
	if chartRef == "" {
		chartRef = DefaultChartRef
	}
	chartVersion := os.Getenv(EnvChartVersion)
	if chartVersion == "" {
		chartVersion = DefaultChartVersion
	}
	return &HelmProvisioner{
		restConfig:   cfg,
		settings:     cli.New(),
		chartRef:     chartRef,
		chartVersion: chartVersion,
	}
}

func (p *HelmProvisioner) actionConfig(ctx context.Context, namespace string) (*action.Configuration, error) {
	log := logf.FromContext(ctx).WithName("helm")
	cfg := new(action.Configuration)
	getter := &restClientGetter{restConfig: p.restConfig, namespace: namespace}
	if err := cfg.Init(getter, namespace, "secret", func(format string, v ...any) {
		log.V(1).Info(fmt.Sprintf(format, v...))
	}); err != nil {
		return nil, fmt.Errorf("initializing helm action configuration: %w", err)
	}
	registryClient, err := registry.NewClient()
	if err != nil {
		return nil, fmt.Errorf("creating helm registry client: %w", err)
	}
	cfg.RegistryClient = registryClient
	return cfg, nil
}

// Install installs or upgrades the vCluster chart for the request.
func (p *HelmProvisioner) Install(ctx context.Context, req Request) error {
	cfg, err := p.actionConfig(ctx, req.Namespace)
	if err != nil {
		return err
	}

	history := action.NewHistory(cfg)
	history.Max = 1
	_, histErr := history.Run(req.ReleaseName)
	releaseExists := histErr == nil
	if histErr != nil && !errors.Is(histErr, driver.ErrReleaseNotFound) {
		return fmt.Errorf("checking release history for %q: %w", req.ReleaseName, histErr)
	}

	values := buildValues(req)

	if !releaseExists {
		install := action.NewInstall(cfg)
		install.ReleaseName = req.ReleaseName
		install.Namespace = req.Namespace
		install.CreateNamespace = false
		install.Wait = false
		p.applyChartSource(&install.ChartPathOptions, req.ChartVersion)

		ch, err := p.locateChart(&install.ChartPathOptions)
		if err != nil {
			return err
		}
		if _, err := install.RunWithContext(ctx, ch, values); err != nil {
			return fmt.Errorf("installing vcluster release %q: %w", req.ReleaseName, err)
		}
		return nil
	}

	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = req.Namespace
	upgrade.Wait = false
	upgrade.MaxHistory = 5
	p.applyChartSource(&upgrade.ChartPathOptions, req.ChartVersion)

	ch, err := p.locateChart(&upgrade.ChartPathOptions)
	if err != nil {
		return err
	}
	if _, err := upgrade.RunWithContext(ctx, req.ReleaseName, ch, values); err != nil {
		return fmt.Errorf("upgrading vcluster release %q: %w", req.ReleaseName, err)
	}
	return nil
}

// Uninstall removes the vCluster release; a missing release is not an error.
func (p *HelmProvisioner) Uninstall(ctx context.Context, releaseName, namespace string) error {
	cfg, err := p.actionConfig(ctx, namespace)
	if err != nil {
		return err
	}
	uninstall := action.NewUninstall(cfg)
	uninstall.IgnoreNotFound = true
	uninstall.Wait = false
	if _, err := uninstall.Run(releaseName); err != nil {
		return fmt.Errorf("uninstalling vcluster release %q: %w", releaseName, err)
	}
	return nil
}

// Status returns the Helm release status string.
func (p *HelmProvisioner) Status(ctx context.Context, releaseName, namespace string) (string, error) {
	cfg, err := p.actionConfig(ctx, namespace)
	if err != nil {
		return "", err
	}
	status := action.NewStatus(cfg)
	rel, err := status.Run(releaseName)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return "", ErrReleaseNotFound
		}
		return "", fmt.Errorf("getting status of release %q: %w", releaseName, err)
	}
	return rel.Info.Status.String(), nil
}

// applyChartSource sets version (tenant override wins over the pinned default)
// and, for classic repos, the RepoURL. OCI refs carry the source in the chart
// reference itself.
func (p *HelmProvisioner) applyChartSource(opts *action.ChartPathOptions, requestedVersion string) {
	opts.Version = requestedVersion
	if opts.Version == "" {
		opts.Version = p.chartVersion
	}
	if !registry.IsOCI(p.chartRef) {
		opts.RepoURL = p.chartRef
	}
}

// chartLocation is what LocateChart resolves: the full ref for OCI, the chart
// name within RepoURL for classic repos.
func (p *HelmProvisioner) chartLocation() string {
	if registry.IsOCI(p.chartRef) {
		return p.chartRef
	}
	return ChartName
}

func (p *HelmProvisioner) locateChart(opts *action.ChartPathOptions) (*chart.Chart, error) {
	location := p.chartLocation()
	chartPath, err := opts.LocateChart(location, p.settings)
	if err != nil {
		return nil, fmt.Errorf("locating chart %q version %q: %w", location, opts.Version, err)
	}
	ch, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("loading chart from %q: %w", chartPath, err)
	}
	return ch, nil
}

// buildValues merges the generated base values (Kubernetes version, public
// endpoint) with the user-supplied overrides; overrides win.
func buildValues(req Request) map[string]any {
	base := map[string]any{}

	controlPlane := map[string]any{}
	if req.KubernetesVersion != "" {
		// vCluster >= 0.20 values schema: controlPlane.distro.k8s.image.tag.
		controlPlane["distro"] = map[string]any{
			"k8s": map[string]any{
				"image": map[string]any{
					"tag": req.KubernetesVersion,
				},
			},
		}
	}
	if req.PublicAPIHost != "" {
		// The proxy cert must cover the public hostname (SNI passthrough
		// keeps TLS end-to-end, so the vCluster's own cert is what clients
		// verify).
		controlPlane["proxy"] = map[string]any{
			"extraSANs": []any{req.PublicAPIHost},
		}
	}
	if len(controlPlane) > 0 {
		base["controlPlane"] = controlPlane
	}
	if req.PublicAPIURL != "" {
		base["exportKubeConfig"] = map[string]any{
			"server": req.PublicAPIURL,
		}
	}
	if req.SyncGatewayAPI {
		// Native Gateway API sync (vCluster >= 0.35): HTTPRoutes created in
		// the virtual cluster land in the tenant host namespace. TLSRoute
		// sync stays off deliberately — the shared API gateway must never
		// admit tenant-authored passthrough routes.
		base["sync"] = map[string]any{
			"toHost": map[string]any{
				"gatewayApi": map[string]any{
					"enabled": true,
					"httpRoutes": map[string]any{
						"enabled": true,
					},
				},
			},
		}
	}

	if len(req.ValuesOverrides) == 0 {
		return base
	}
	return chartutil.CoalesceTables(req.ValuesOverrides, base)
}

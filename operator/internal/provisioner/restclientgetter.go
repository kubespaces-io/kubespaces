package provisioner

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	memcached "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// restClientGetter adapts a rest.Config (in-cluster / manager credentials)
// plus a target namespace to Helm's RESTClientGetter interface.
type restClientGetter struct {
	restConfig *rest.Config
	namespace  string
}

var _ genericclioptions.RESTClientGetter = (*restClientGetter)(nil)

func (g *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	if g.restConfig == nil {
		return nil, fmt.Errorf("rest config is nil")
	}
	return rest.CopyConfig(g.restConfig), nil
}

func (g *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	cfg, err := g.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating discovery client: %w", err)
	}
	return memcached.NewMemCacheClient(dc), nil
}

func (g *restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	dc, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(dc), nil
}

func (g *restClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	// Helm only uses this to resolve the namespace; return a config whose
	// namespace override is the target namespace.
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{},
		&clientcmd.ConfigOverrides{Context: clientcmdapi.Context{Namespace: g.namespace}},
	)
}

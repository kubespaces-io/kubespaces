// Package kubeconfig merges tenant kubeconfigs into the user's kubeconfig
// file under a "kubespaces-<name>" context.
package kubeconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const filePerm = 0o600

// DestinationPath returns the kubeconfig file to merge into: the first path
// in $KUBECONFIG if set, otherwise ~/.kube/config.
func DestinationPath() (string, error) {
	if env := os.Getenv(clientcmd.RecommendedConfigPathEnvVar); env != "" {
		paths := filepath.SplitList(env)
		if len(paths) > 0 && paths[0] != "" {
			return paths[0], nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, ".kube", "config"), nil
}

// Merge writes the cluster/user/context of kubeconfigYAML into the file at
// destPath under the name contextName, replacing existing entries with that
// name. The destination file is created if missing. The destination's
// current-context is left untouched unless it was empty.
func Merge(kubeconfigYAML []byte, contextName, destPath string) error {
	src, err := clientcmd.Load(kubeconfigYAML)
	if err != nil {
		return fmt.Errorf("parsing tenant kubeconfig: %w", err)
	}
	cluster, authInfo, err := primaryEntries(src)
	if err != nil {
		return err
	}

	dest, err := loadOrEmpty(destPath)
	if err != nil {
		return err
	}
	dest.Clusters[contextName] = cluster
	dest.AuthInfos[contextName] = authInfo
	dest.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:  contextName,
		AuthInfo: contextName,
	}
	if dest.CurrentContext == "" {
		dest.CurrentContext = contextName
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creating kubeconfig directory: %w", err)
	}
	if err := clientcmd.WriteToFile(*dest, destPath); err != nil {
		return fmt.Errorf("writing %s: %w", destPath, err)
	}
	// clientcmd preserves existing file modes; tighten fresh files.
	if err := os.Chmod(destPath, filePerm); err != nil {
		return fmt.Errorf("setting kubeconfig permissions: %w", err)
	}
	return nil
}

// primaryEntries picks the cluster and user referenced by the source's
// current context, falling back to the sole entries when unset.
func primaryEntries(src *clientcmdapi.Config) (*clientcmdapi.Cluster, *clientcmdapi.AuthInfo, error) {
	if ctx, ok := src.Contexts[src.CurrentContext]; ok {
		cluster, cOK := src.Clusters[ctx.Cluster]
		authInfo, aOK := src.AuthInfos[ctx.AuthInfo]
		if cOK && aOK {
			return cluster, authInfo, nil
		}
	}
	if len(src.Clusters) != 1 || len(src.AuthInfos) != 1 {
		return nil, nil, fmt.Errorf("tenant kubeconfig has no usable current-context and is ambiguous")
	}
	var cluster *clientcmdapi.Cluster
	for _, c := range src.Clusters {
		cluster = c
	}
	var authInfo *clientcmdapi.AuthInfo
	for _, a := range src.AuthInfos {
		authInfo = a
	}
	return cluster, authInfo, nil
}

func loadOrEmpty(path string) (*clientcmdapi.Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return clientcmdapi.NewConfig(), nil
	}
	cfg, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", path, err)
	}
	return cfg, nil
}

package kubeconfig

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const tenantKubeconfig = `apiVersion: v1
kind: Config
current-context: my-vcluster
clusters:
- name: my-vcluster
  cluster:
    server: https://vcluster.example.com:443
    certificate-authority-data: Zm9v
contexts:
- name: my-vcluster
  context:
    cluster: my-vcluster
    user: my-vcluster
users:
- name: my-vcluster
  user:
    client-certificate-data: Zm9v
    client-key-data: YmFy
`

func TestMergeIntoNewFile(t *testing.T) {
	// Arrange
	dest := filepath.Join(t.TempDir(), ".kube", "config")

	// Act
	err := Merge([]byte(tenantKubeconfig), "kubespaces-alpha", dest)

	// Assert
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	got, err := clientcmd.LoadFromFile(dest)
	if err != nil {
		t.Fatalf("loading merged file: %v", err)
	}
	cluster, ok := got.Clusters["kubespaces-alpha"]
	if !ok {
		t.Fatal("merged file missing cluster kubespaces-alpha")
	}
	if cluster.Server != "https://vcluster.example.com:443" {
		t.Errorf("cluster server = %q", cluster.Server)
	}
	if _, ok := got.AuthInfos["kubespaces-alpha"]; !ok {
		t.Error("merged file missing user kubespaces-alpha")
	}
	ctx, ok := got.Contexts["kubespaces-alpha"]
	if !ok {
		t.Fatal("merged file missing context kubespaces-alpha")
	}
	if ctx.Cluster != "kubespaces-alpha" || ctx.AuthInfo != "kubespaces-alpha" {
		t.Errorf("context = %+v", ctx)
	}
	if got.CurrentContext != "kubespaces-alpha" {
		t.Errorf("current-context = %q, want kubespaces-alpha for a fresh file", got.CurrentContext)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("kubeconfig mode = %o, want 600", perm)
	}
}

func TestMergePreservesExistingEntries(t *testing.T) {
	// Arrange
	dest := filepath.Join(t.TempDir(), "config")
	existing := clientcmdapi.NewConfig()
	existing.Clusters["prod"] = &clientcmdapi.Cluster{Server: "https://prod.example.com"}
	existing.AuthInfos["prod"] = &clientcmdapi.AuthInfo{Token: "tok"}
	existing.Contexts["prod"] = &clientcmdapi.Context{Cluster: "prod", AuthInfo: "prod"}
	existing.CurrentContext = "prod"
	if err := clientcmd.WriteToFile(*existing, dest); err != nil {
		t.Fatalf("writing existing kubeconfig: %v", err)
	}

	// Act
	err := Merge([]byte(tenantKubeconfig), "kubespaces-alpha", dest)

	// Assert
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	got, err := clientcmd.LoadFromFile(dest)
	if err != nil {
		t.Fatalf("loading merged file: %v", err)
	}
	if _, ok := got.Contexts["prod"]; !ok {
		t.Error("existing context prod was dropped")
	}
	if _, ok := got.Contexts["kubespaces-alpha"]; !ok {
		t.Error("new context kubespaces-alpha missing")
	}
	if got.CurrentContext != "prod" {
		t.Errorf("current-context = %q, want prod (must not be hijacked)", got.CurrentContext)
	}
}

func TestMergeReplacesExistingSameNameContext(t *testing.T) {
	// Arrange
	dest := filepath.Join(t.TempDir(), "config")
	if err := Merge([]byte(tenantKubeconfig), "kubespaces-alpha", dest); err != nil {
		t.Fatalf("first Merge() error = %v", err)
	}
	updated := []byte(tenantKubeconfig)
	updated = append([]byte(nil), updated...)

	// Act: merging again must not error or duplicate entries.
	err := Merge(updated, "kubespaces-alpha", dest)

	// Assert
	if err != nil {
		t.Fatalf("second Merge() error = %v", err)
	}
	got, err := clientcmd.LoadFromFile(dest)
	if err != nil {
		t.Fatalf("loading merged file: %v", err)
	}
	if len(got.Contexts) != 1 {
		t.Errorf("contexts = %d, want 1", len(got.Contexts))
	}
}

func TestMergeRejectsGarbage(t *testing.T) {
	// Arrange
	dest := filepath.Join(t.TempDir(), "config")

	// Act
	err := Merge([]byte(":::not a kubeconfig"), "kubespaces-alpha", dest)

	// Assert
	if err == nil {
		t.Error("Merge() error = nil, want parse error")
	}
}

func TestDestinationPathKubeconfigEnv(t *testing.T) {
	// Arrange
	custom := filepath.Join(t.TempDir(), "custom-config")
	t.Setenv("KUBECONFIG", custom+string(os.PathListSeparator)+"/elsewhere/config")

	// Act
	path, err := DestinationPath()

	// Assert
	if err != nil {
		t.Fatalf("DestinationPath() error = %v", err)
	}
	if path != custom {
		t.Errorf("DestinationPath() = %q, want first KUBECONFIG entry %q", path, custom)
	}
}

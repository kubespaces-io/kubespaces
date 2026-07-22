// Package cli wires the kubespaces cobra commands.
package cli

import (
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/kubespaces-io/kubespaces/cli/internal/auth"
	"github.com/kubespaces-io/kubespaces/cli/internal/client"
	"github.com/kubespaces-io/kubespaces/cli/internal/config"
)

const httpTimeout = 30 * time.Second

// BuildInfo carries version metadata injected via -ldflags.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// NewRootCmd builds the kubespaces command tree.
func NewRootCmd(build BuildInfo) *cobra.Command {
	root := &cobra.Command{
		Use:           "kubespaces",
		Short:         "kubespaces controls KubeSpaces tenants",
		Long:          "kubespaces is the command-line client for the KubeSpaces API:\nlog in, manage tenants, and grab kubeconfigs for their virtual clusters.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String("server", "", "KubeSpaces server URL (overrides config and $"+config.EnvServer+")")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})

	root.AddCommand(
		newLoginCmd(),
		newLogoutCmd(),
		newWhoamiCmd(),
		newTenantCmd(),
		newVersionCmd(build),
	)
	return root
}

// newAPIClient builds an authenticated API client from flags + config.
func newAPIClient(cmd *cobra.Command) (*client.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	serverFlag, _ := cmd.Flags().GetString("server")
	server := cfg.ResolveServer(serverFlag)
	if server == "" {
		return nil, errNoServer
	}
	credsPath, err := auth.CredentialsPath()
	if err != nil {
		return nil, err
	}
	ts := &auth.TokenSource{
		CredentialsPath: credsPath,
		Issuer:          cfg.Issuer,
		ClientID:        cfg.ClientID,
		HTTP:            &http.Client{Timeout: httpTimeout},
	}
	return client.New(server, &http.Client{Timeout: httpTimeout}, ts.Token), nil
}

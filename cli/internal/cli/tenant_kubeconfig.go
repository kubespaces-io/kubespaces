package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubespaces-io/kubespaces/cli/internal/kubeconfig"
)

const contextPrefix = "kubespaces-"

func newTenantKubeconfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig <name>",
		Short: "Fetch the kubeconfig for a tenant's virtual cluster",
		Long:  "Fetch the kubeconfig for a tenant's virtual cluster.\nPrints to stdout by default; --merge writes it into your kubeconfig file\n(respects $KUBECONFIG, defaults to ~/.kube/config) as context \"kubespaces-<name>\".",
		Args:  exactArgs(1),
		RunE:  runTenantKubeconfig,
	}
	cmd.Flags().Bool("merge", false, "merge into your kubeconfig instead of printing")
	return cmd
}

func runTenantKubeconfig(cmd *cobra.Command, args []string) error {
	api, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	name := args[0]
	raw, err := api.Kubeconfig(cmd.Context(), name)
	if err != nil {
		return err
	}
	merge, _ := cmd.Flags().GetBool("merge")
	if !merge {
		_, err := cmd.OutOrStdout().Write(raw)
		return err
	}

	destPath, err := kubeconfig.DestinationPath()
	if err != nil {
		return err
	}
	contextName := contextPrefix + name
	if err := kubeconfig.Merge(raw, contextName, destPath); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Merged context %q into %s.\n", contextName, destPath)
	fmt.Fprintf(cmd.OutOrStdout(), "Switch to it with: kubectl config use-context %s\n", contextName)
	return nil
}

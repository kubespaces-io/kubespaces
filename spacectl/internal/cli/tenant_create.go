package cli

import (
	"github.com/spf13/cobra"

	"github.com/kubespaces-io/kubespaces/spacectl/internal/client"
)

func newTenantCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a tenant",
		Args:  exactArgs(1),
		RunE:  runTenantCreate,
	}
	cmd.Flags().String("display-name", "", "human-friendly display name")
	cmd.Flags().String("cpu", "", "CPU limit for the tenant (e.g. 4)")
	cmd.Flags().String("memory", "", "memory limit for the tenant (e.g. 8Gi)")
	cmd.Flags().String("storage", "", "storage limit for the tenant (e.g. 20Gi)")
	addOutputFlag(cmd)
	return cmd
}

func runTenantCreate(cmd *cobra.Command, args []string) error {
	format, err := outputFormat(cmd)
	if err != nil {
		return err
	}
	api, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	req := buildCreateRequest(cmd, args[0])
	tenant, err := api.CreateTenant(cmd.Context(), req)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if format != outputTable {
		return printEncoded(out, format, tenant)
	}
	return printTenantDetail(out, tenant)
}

func buildCreateRequest(cmd *cobra.Command, name string) *client.CreateTenantRequest {
	displayName, _ := cmd.Flags().GetString("display-name")
	cpu, _ := cmd.Flags().GetString("cpu")
	memory, _ := cmd.Flags().GetString("memory")
	storage, _ := cmd.Flags().GetString("storage")

	req := &client.CreateTenantRequest{Name: name, DisplayName: displayName}
	if cpu != "" || memory != "" || storage != "" {
		req.Resources = &client.Resources{CPU: cpu, Memory: memory, Storage: storage}
	}
	return req
}

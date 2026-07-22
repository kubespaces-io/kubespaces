package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTenantDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <name>",
		Short:   "Delete a tenant",
		Aliases: []string{"rm"},
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := newAPIClient(cmd)
			if err != nil {
				return err
			}
			if err := api.DeleteTenant(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Tenant %s deletion requested.\n", args[0])
			return nil
		},
	}
}

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newTenantListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List tenants",
		Aliases: []string{"ls"},
		Args:    noArgs,
		RunE:    runTenantList,
	}
	addOutputFlag(cmd)
	return cmd
}

func runTenantList(cmd *cobra.Command, _ []string) error {
	format, err := outputFormat(cmd)
	if err != nil {
		return err
	}
	api, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	tenants, err := api.ListTenants(cmd.Context())
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if format != outputTable {
		return printEncoded(out, format, tenants)
	}
	if len(tenants) == 0 {
		fmt.Fprintln(out, "No tenants found.")
		return nil
	}
	now := time.Now()
	w := newTable(out)
	fmt.Fprintln(w, "NAME\tDISPLAY NAME\tOWNER\tPHASE\tAGE")
	for _, t := range tenants {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			t.Name, dash(t.DisplayName), dash(t.Owner), dash(t.Phase), formatAge(t.CreatedAt, now))
	}
	return w.Flush()
}

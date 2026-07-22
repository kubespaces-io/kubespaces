package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/kubespaces-io/kubespaces/cli/internal/client"
)

func newTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tenant",
		Short:   "Manage KubeSpaces tenants",
		Aliases: []string{"tenants"},
	}
	cmd.AddCommand(
		newTenantListCmd(),
		newTenantCreateCmd(),
		newTenantGetCmd(),
		newTenantDeleteCmd(),
		newTenantKubeconfigCmd(),
	)
	return cmd
}

// printTenantDetail renders one tenant as a key/value table.
func printTenantDetail(out io.Writer, t *client.Tenant) error {
	w := newTable(out)
	fmt.Fprintf(w, "NAME\t%s\n", t.Name)
	fmt.Fprintf(w, "DISPLAY NAME\t%s\n", dash(t.DisplayName))
	fmt.Fprintf(w, "OWNER\t%s\n", dash(t.Owner))
	fmt.Fprintf(w, "PHASE\t%s\n", dash(t.Phase))
	fmt.Fprintf(w, "MESSAGE\t%s\n", dash(t.Message))
	fmt.Fprintf(w, "CPU\t%s\n", dash(t.Resources.CPU))
	fmt.Fprintf(w, "MEMORY\t%s\n", dash(t.Resources.Memory))
	fmt.Fprintf(w, "STORAGE\t%s\n", dash(t.Resources.Storage))
	fmt.Fprintf(w, "CREATED\t%s\n", formatCreatedAt(t.CreatedAt))
	return w.Flush()
}

func formatCreatedAt(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format(time.RFC3339)
}

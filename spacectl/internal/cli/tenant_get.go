package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/kubespaces-io/kubespaces/spacectl/internal/client"
)

const (
	waitPollInterval = 3 * time.Second
	phaseReady       = "Ready"
	phaseFailed      = "Failed"
)

func newTenantGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Show a tenant",
		Args:  exactArgs(1),
		RunE:  runTenantGet,
	}
	cmd.Flags().BoolP("wait", "w", false, "wait until the tenant reaches phase Ready or Failed")
	addOutputFlag(cmd)
	return cmd
}

func runTenantGet(cmd *cobra.Command, args []string) error {
	format, err := outputFormat(cmd)
	if err != nil {
		return err
	}
	api, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	name := args[0]
	tenant, err := api.GetTenant(cmd.Context(), name)
	if err != nil {
		return err
	}
	if wait, _ := cmd.Flags().GetBool("wait"); wait {
		tenant, err = waitForTenant(cmd.Context(), cmd.ErrOrStderr(), api, tenant, waitPollInterval)
		if err != nil {
			return err
		}
	}
	out := cmd.OutOrStdout()
	if format != outputTable {
		return printEncoded(out, format, tenant)
	}
	if err := printTenantDetail(out, tenant); err != nil {
		return err
	}
	if tenant.Phase == phaseFailed {
		return fmt.Errorf("tenant %s failed: %s", tenant.Name, dash(tenant.Message))
	}
	return nil
}

// waitForTenant polls until the tenant is Ready or Failed, printing phase
// transitions to progress as they happen.
func waitForTenant(ctx context.Context, progress io.Writer, api *client.Client, tenant *client.Tenant, interval time.Duration) (*client.Tenant, error) {
	phase := tenant.Phase
	fmt.Fprintf(progress, "Waiting for tenant %s (phase: %s)...\n", tenant.Name, dash(phase))
	for phase != phaseReady && phase != phaseFailed {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
		fresh, err := api.GetTenant(ctx, tenant.Name)
		if err != nil {
			return nil, err
		}
		if fresh.Phase != phase {
			fmt.Fprintf(progress, "  %s -> %s\n", dash(phase), dash(fresh.Phase))
			phase = fresh.Phase
		}
		tenant = fresh
	}
	return tenant, nil
}

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newWhoamiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show the identity of the logged-in user",
		Args:  noArgs,
		RunE:  runWhoami,
	}
	addOutputFlag(cmd)
	return cmd
}

func runWhoami(cmd *cobra.Command, _ []string) error {
	format, err := outputFormat(cmd)
	if err != nil {
		return err
	}
	api, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	me, err := api.Me(cmd.Context())
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if format != outputTable {
		return printEncoded(out, format, me)
	}
	w := newTable(out)
	fmt.Fprintf(w, "SUBJECT\t%s\n", dash(me.Subject))
	fmt.Fprintf(w, "EMAIL\t%s\n", dash(me.Email))
	fmt.Fprintf(w, "ROLES\t%s\n", dash(strings.Join(me.Roles, ", ")))
	return w.Flush()
}

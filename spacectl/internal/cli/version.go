package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd(build BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print spacectl build information",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "spacectl %s\n", build.Version)
			fmt.Fprintf(out, "  commit: %s\n", build.Commit)
			fmt.Fprintf(out, "  built:  %s\n", build.Date)
			fmt.Fprintf(out, "  go:     %s (%s/%s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}

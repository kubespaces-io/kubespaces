package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubespaces-io/kubespaces/spacectl/internal/auth"
)

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out by removing cached credentials",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := auth.CredentialsPath()
			if err != nil {
				return err
			}
			if err := auth.DeleteCredentials(path); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
			return nil
		},
	}
}

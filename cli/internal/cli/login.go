package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/kubespaces-io/kubespaces/cli/internal/auth"
	"github.com/kubespaces-io/kubespaces/cli/internal/config"
)

const defaultClientID = "kubespaces"

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to KubeSpaces via the OIDC device flow",
		Args:  noArgs,
		RunE:  runLogin,
	}
	cmd.Flags().String("issuer", "", "OIDC issuer URL (e.g. https://keycloak.example.com/realms/kubespaces)")
	cmd.Flags().String("client-id", "", "OIDC client id (default \""+defaultClientID+"\")")
	return cmd
}

func runLogin(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := applyLoginFlags(cmd, cfg); err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return err
	}

	ctx := cmd.Context()
	httpClient := &http.Client{Timeout: httpTimeout}
	eps, err := auth.Discover(ctx, httpClient, cfg.Issuer)
	if err != nil {
		return err
	}

	flow := &auth.Flow{HTTP: httpClient, ClientID: cfg.ClientID}
	da, err := flow.Start(ctx, eps.DeviceAuthorizationEndpoint)
	if err != nil {
		return err
	}

	promptDeviceLogin(cmd, da)

	tok, err := flow.Poll(ctx, eps.TokenEndpoint, da)
	if err != nil {
		return err
	}
	credsPath, err := auth.CredentialsPath()
	if err != nil {
		return err
	}
	if err := auth.SaveCredentials(credsPath, flow.Credentials(tok)); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Login successful.")
	return nil
}

// applyLoginFlags merges --server/--issuer/--client-id into cfg, keeping
// previously persisted values when flags are omitted.
func applyLoginFlags(cmd *cobra.Command, cfg *config.Config) error {
	if server, _ := cmd.Flags().GetString("server"); server != "" {
		cfg.Server = server
	} else if cfg.Server == "" {
		cfg.Server = cfg.ResolveServer("")
	}
	if issuer, _ := cmd.Flags().GetString("issuer"); issuer != "" {
		cfg.Issuer = issuer
	}
	if cfg.Issuer == "" {
		return &usageError{err: fmt.Errorf("no OIDC issuer known — pass --issuer on first login")}
	}
	if clientID, _ := cmd.Flags().GetString("client-id"); clientID != "" {
		cfg.ClientID = clientID
	}
	if cfg.ClientID == "" {
		cfg.ClientID = defaultClientID
	}
	return nil
}

func promptDeviceLogin(cmd *cobra.Command, da *auth.DeviceAuthorization) {
	out := cmd.OutOrStdout()
	verifyURL := da.VerificationURIComplete
	if verifyURL == "" {
		verifyURL = da.VerificationURI
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "To log in, open this URL in your browser:")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "    %s\n", verifyURL)
	fmt.Fprintln(out)
	if da.VerificationURIComplete == "" && da.UserCode != "" {
		fmt.Fprintf(out, "and enter the code: %s\n\n", da.UserCode)
	} else if da.UserCode != "" {
		fmt.Fprintf(out, "Confirm the code shown: %s\n\n", da.UserCode)
	}
	if err := openBrowser(verifyURL); err == nil {
		fmt.Fprintln(out, "(opened in your default browser)")
	}
	expires := time.Duration(da.ExpiresIn) * time.Second
	fmt.Fprintf(out, "Waiting for approval (expires in %s)...\n", expires)
}

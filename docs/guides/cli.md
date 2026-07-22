# Using the CLI

`kubespaces` is the command-line client: OIDC device-flow login, tenant
CRUD, kubeconfig retrieval. Same API and permissions as the portal.

## Install

=== "Homebrew (macOS/Linux)"

    ```bash
    brew install kubespaces-io/tap/kubespaces
    ```

=== "curl | sh (Linux/macOS)"

    ```bash
    curl -fsSL https://kubespaces.io/install.sh | sh
    ```

    The installer detects OS/arch, downloads the latest release, verifies
    the checksum, and installs to `/usr/local/bin` or `~/.local/bin`. Pin a
    version with `KUBESPACES_VERSION=v0.2.0`, change the destination with
    `KUBESPACES_INSTALL_DIR`.

=== "Manual / Windows"

    Grab a signed archive from the
    [releases page](https://github.com/kubespaces-io/kubespaces/releases) —
    linux/darwin/windows, amd64/arm64, with SBOMs and cosign-signed
    checksums.

!!! note "Formerly spacectl"
    The CLI was renamed from `spacectl` in v0.2.0 — the old name collided
    with Spacelift's CLI.

## Login

```bash
kubespaces login --server https://kubespaces.example.com
```

This runs the OIDC **device flow**: the CLI prints a URL and code (and opens
your browser), you authenticate against your identity provider, and the CLI
receives tokens — no passwords ever touch the terminal. Tokens are cached at
`~/.config/kubespaces/credentials.json` (mode 0600) and refreshed
automatically; the server default persists in
`~/.config/kubespaces/config.yaml` (override per-call with `--server` or
`$KUBESPACES_CLI_SERVER`).

```bash
kubespaces whoami
kubespaces logout
```

## Tenants

```bash
kubespaces tenant create team-atlas --cpu 8 --memory 16Gi --storage 50Gi
kubespaces tenant list
kubespaces tenant get team-atlas --wait     # block until Ready
kubespaces tenant delete team-atlas
```

Every command takes `-o json` or `-o yaml` for scripting; the default is a
human table.

## Kubeconfigs

```bash
# print to stdout
kubespaces tenant kubeconfig team-atlas

# merge into ~/.kube/config as context "team-atlas"
kubespaces tenant kubeconfig team-atlas --merge
kubectl config use-context team-atlas
kubectl get nodes
```

With tenant API exposure configured on the platform, the kubeconfig points
at `https://<tenant>.api.<domain>` and works from anywhere. Without it, see
the port-forward pattern in the [Quickstart](../getting-started/quickstart.md).

## Verifying releases

```bash
# checksums are cosign-signed (keyless, GitHub OIDC)
cosign verify-blob \
  --certificate checksums.txt.pem --signature checksums.txt.sig \
  --certificate-identity-regexp 'github.com/kubespaces-io/kubespaces' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

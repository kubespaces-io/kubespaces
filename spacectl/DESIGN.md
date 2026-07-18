# spacectl — rewrite design

Status: **planned**. Ground-up rewrite in this monorepo (the old
kubespaces-io/spacectl repo is private/archived). Nothing from the old CLI is
imported; the API contract (docs/contracts.md) is the only dependency.

## Shape

Go (same toolchain as api/ and operator/), cobra + a thin generated-free client
for the KubeSpaces HTTP API. Module: `github.com/kubespaces-io/kubespaces/spacectl`.

```
spacectl/
├── cmd/spacectl/main.go
├── internal/
│   ├── cli/           # cobra commands, one file per command
│   ├── client/        # HTTP client for /api/v1 (thin, hand-written)
│   ├── auth/          # OIDC device flow + token cache
│   └── config/        # ~/.config/spacectl/config.yaml
└── DESIGN.md
```

## Auth: OIDC device authorization grant

`spacectl login` runs the **device flow** against the same Keycloak/OIDC issuer
the portal uses (public client, no secret): print/open verification URL, poll,
cache tokens in `~/.config/spacectl/` (0600), refresh silently. This needs the
`kubespaces` realm client to enable the device grant — add to the chart's realm
bootstrap when implementing.

## v1 commands (map 1:1 to the API — no client-side cleverness)

| Command | API |
|---------|-----|
| `spacectl login` / `logout` / `whoami` | device flow / drop cache / GET /me |
| `spacectl tenant list` | GET /tenants |
| `spacectl tenant create <name> [--display-name] [--cpu] [--memory] [--storage]` | POST /tenants |
| `spacectl tenant get <name>` (incl. `-w` wait for Ready) | GET /tenants/{name} |
| `spacectl tenant delete <name>` | DELETE /tenants/{name} |
| `spacectl tenant kubeconfig <name> [--merge]` | GET /tenants/{name}/kubeconfig; `--merge` into ~/.kube/config with context `kubespaces-<name>` |
| `spacectl version` | build info |

Output: human tables by default, `-o json|yaml` everywhere. Errors surface the
API's `{"error": ...}` verbatim plus exit codes (0/1/2 usage).

## Config

`~/.config/spacectl/config.yaml`: `server` (portal URL, e.g.
https://kubespaces.example.com), token cache reference. `--server` flag and
`SPACECTL_SERVER` env override. Multiple contexts deferred until asked for.

## Distribution

- goreleaser: darwin/linux, amd64/arm64, plus `kubespaces-io/homebrew-tap`
  (`brew install kubespaces-io/tap/spacectl`).
- Release workflow trigger: `spacectl-v*` tags in this repo.
- CI job (build + test) joins .github/workflows/ci.yml when code lands.

## Non-goals for v1

Web-terminal-ish features, imperative resource management inside tenants
(that's kubectl's job — spacectl hands you the kubeconfig and gets out of the
way), plugin system, PAT auth (OIDC only, per D10 scope cut).

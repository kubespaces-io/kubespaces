# kubespaces — rewrite design

Status: **planned**. Ground-up rewrite in this monorepo (the old
kubespaces-io/kubespaces repo is private/archived). Nothing from the old CLI is
imported; the API contract (docs/contracts.md) is the only dependency.

## Shape

Go (same toolchain as api/ and operator/), cobra + a thin generated-free client
for the KubeSpaces HTTP API. Module: `github.com/kubespaces-io/kubespaces/cli`.

```
kubespaces/
├── cmd/kubespaces/main.go
├── internal/
│   ├── cli/           # cobra commands, one file per command
│   ├── client/        # HTTP client for /api/v1 (thin, hand-written)
│   ├── auth/          # OIDC device flow + token cache
│   └── config/        # ~/.config/kubespaces/config.yaml
└── DESIGN.md
```

## Auth: OIDC device authorization grant

`kubespaces login` runs the **device flow** against the same Keycloak/OIDC issuer
the portal uses (public client, no secret): print/open verification URL, poll,
cache tokens in `~/.config/kubespaces/` (0600), refresh silently. This needs the
`kubespaces` realm client to enable the device grant — add to the chart's realm
bootstrap when implementing.

## v1 commands (map 1:1 to the API — no client-side cleverness)

| Command | API |
|---------|-----|
| `kubespaces login` / `logout` / `whoami` | device flow / drop cache / GET /me |
| `kubespaces tenant list` | GET /tenants |
| `kubespaces tenant create <name> [--display-name] [--cpu] [--memory] [--storage]` | POST /tenants |
| `kubespaces tenant get <name>` (incl. `-w` wait for Ready) | GET /tenants/{name} |
| `kubespaces tenant delete <name>` | DELETE /tenants/{name} |
| `kubespaces tenant kubeconfig <name> [--merge]` | GET /tenants/{name}/kubeconfig; `--merge` into ~/.kube/config with context `kubespaces-<name>` |
| `kubespaces version` | build info |

Output: human tables by default, `-o json|yaml` everywhere. Errors surface the
API's `{"error": ...}` verbatim plus exit codes (0/1/2 usage).

## Config

`~/.config/kubespaces/config.yaml`: `server` (portal URL, e.g.
https://kubespaces.example.com), token cache reference. `--server` flag and
`KUBESPACES_CLI_SERVER` env override. Multiple contexts deferred until asked for.

## Distribution

- goreleaser: darwin/linux, amd64/arm64, plus `kubespaces-io/homebrew-tap`
  (`brew install kubespaces-io/tap/kubespaces`).
- Release workflow trigger: `kubespaces-v*` tags in this repo.
- CI job (build + test) joins .github/workflows/ci.yml when code lands.

## Non-goals for v1

Web-terminal-ish features, imperative resource management inside tenants
(that's kubectl's job — kubespaces hands you the kubeconfig and gets out of the
way), plugin system, PAT auth (OIDC only, per D10 scope cut).

# KubeSpaces

> The open control plane for virtual Kubernetes tenants.

**[Roadmap](ROADMAP.md)** · [Project board](https://github.com/orgs/kubespaces-io/projects/3) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md)

[![CI](https://github.com/kubespaces-io/kubespaces/actions/workflows/ci.yml/badge.svg)](https://github.com/kubespaces-io/kubespaces/actions/workflows/ci.yml) [![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

This is the KubeSpaces monorepo: backend API, Tenant operator, web frontend,
and the umbrella Helm chart that deploys a complete installation.
(Predecessor repos — kubespaces-io/login-api, ams0/frontend — are superseded by
this codebase; spacectl will move here too.)

| Component | Where | What |
|-----------|-------|------|
| Backend API | `api/` | Go · chi · pgx · OIDC. Auth, tenant CRUD → Tenant CRs, kubeconfig |
| Operator | `operator/` | Go · controller-runtime. Sole provisioner: Tenant CR → vCluster |
| Frontend | `frontend/` | Next.js 15 · Auth.js (Keycloak). Self-service portal |
| Chart | `charts/kubespaces/` | The product: one `helm install` for everything |

## Repository layout

```
.
├── api/                       # Backend API — Go (chi, pgx, go-oidc)
│   ├── cmd/api/               #   entrypoint
│   └── internal/              #   auth, server (handlers), store (Postgres + migrations), k8s (Tenant CRs)
├── operator/                  # Tenant operator — Go (controller-runtime, kubebuilder layout)
│   ├── api/v1alpha1/          #   Tenant types (source of truth for the CRD)
│   ├── config/                #   generated CRD + RBAC (make manifests)
│   └── internal/              #   controller (reconciler), provisioner (vCluster via Helm SDK)
├── frontend/                  # Self-service portal — Next.js 15 + Auth.js (Keycloak)
│   └── src/                   #   app router, components, hooks, lib
├── spacectl/                  # CLI — Go (cobra), OIDC device-flow login
│   ├── cmd/spacectl/          #   entrypoint
│   ├── internal/              #   cli, client, auth, config, kubeconfig
│   └── DESIGN.md              #   command set & design decisions
├── charts/
│   └── kubespaces/            # Umbrella Helm chart — THE product: one install for everything
├── examples/                  # values profiles: kind demo, GKE test, production
├── docs/
│   ├── contracts.md           # inter-component contract (read before touching any component)
│   └── prerequisites.md       # what you need: demo tier vs production tier
├── .github/workflows/         # ci (per-component), build (images → ghcr), release (spacectl), chart mirror
├── .goreleaser.yaml           # spacectl release: binaries, checksums, Homebrew tap
├── Makefile                   # helm lint / template / install, kind dev loop, CRD apply
├── LICENSE                    # Apache 2.0
└── NOTICE                     # vCluster credit
```

Versioning: one line for the whole repo — a `vX.Y.Z` tag releases the spacectl
binaries (goreleaser) and the `api`/`operator`/`frontend` images together.

## Quickstart

```bash
helm install kubespaces charts/kubespaces \
  --namespace kubespaces --create-namespace
```

The default install is fully self-contained: API + frontend + PostgreSQL
(official `postgres` image) + Keycloak (official image, dev mode, realm
auto-imported). For production, toggle in external Postgres, an external OIDC
provider, and real ingress — see `examples/values-production.yaml`.

**Acceptance test (gates every release):** fresh kind cluster →
`helm install kubespaces` → working tenant + kubeconfig in under 10 minutes.

## Dev loop

```bash
make lint        # helm lint
make template    # render manifests locally
make kind-up     # local kind cluster
make install     # install/upgrade the chart into the current context
```

## Design notes

- **No Bitnami subcharts.** Broadcom moved Bitnami images behind a subscription
  (2025); a new OSS project should not depend on them. Postgres and Keycloak are
  shipped as minimal built-in StatefulSets on official images for the
  all-in-one profile, and are expected to be replaced by external services in
  production (`postgresql.enabled=false`, `keycloak.enabled=false`).
- **Env-var contract with the API is provisional** until the backend refactor
  lands — names are centralized in `templates/api/deployment.yaml`.
- Operator architecture discussion: [operator/README.md](operator/README.md).
- Planning & decision log: the `plan` repo (sibling directory).

## License

Apache 2.0 — see [LICENSE](LICENSE). Built on [vCluster](https://github.com/loft-sh/vcluster) (Apache 2.0) — see [NOTICE](NOTICE).

# KubeSpaces

> The open control plane for virtual Kubernetes tenants.

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

## Layout

```
.
├── api/                  # Backend API (Go)
├── operator/             # Tenant CRD + controller (Go, kubebuilder layout)
├── frontend/             # Portal (Next.js)
├── charts/
│   └── kubespaces/       # Umbrella Helm chart — THE product
├── examples/             # values files: kind demo, GKE test, production
├── docs/                 # contracts.md — the inter-component contract
└── Makefile              # lint / template / kind dev loop
```

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

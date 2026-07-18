# KubeSpaces — Component Contracts (v0.1)

Single source of truth for how components talk to each other. Changes here
require touching every consumer — keep it small.

## Architecture (Pattern B, decided 2026-07-18)

```
UI (Next.js) / spacectl ──HTTP+JWT──> backend API ──┐
                                                    ├─ persists to Postgres (UI metadata, audit)
                                                    └─ creates/deletes Tenant CRs
GitOps / kubectl ────────────────────────────────────> Tenant CRs
                                       operator ──watches──> Tenant CRs ──provisions──> vCluster
```

- The **Tenant CR is the source of truth** for desired + observed tenant state.
- Postgres stores what Kubernetes shouldn't: user/session metadata, audit log,
  display metadata. The API reads tenant *status* from the CR, not the DB.
- Only the operator provisions. The API never touches vCluster.

## Versions

| Thing | Version |
|-------|---------|
| Postgres | 18 (official image) |
| Keycloak | 26.7 (official image) |
| Go | 1.26.x |
| Next.js | 15.x |
| CRD | kubespaces.io/v1alpha1, kind Tenant, cluster-scoped |

## Backend HTTP API

Base path `/api/v1`, JSON. Auth: `Authorization: Bearer <access token>` (JWT,
validated against OIDC issuer discovery; audience/azp = `kubespaces`).
Roles from `realm_access.roles`: `kubespaces-admin` (sees all tenants),
`kubespaces-member` (sees own only, matched on token `sub`/`email` vs owner).

| Method | Path | Body | Returns |
|--------|------|------|---------|
| GET | /healthz | – | 200 `{"status":"ok"}` (no auth) |
| GET | /api/v1/me | – | `{subject, email, roles}` |
| GET | /api/v1/tenants | – | `[Tenant]` |
| POST | /api/v1/tenants | `{name, displayName?, resources?, vcluster?}` | 201 `Tenant` |
| GET | /api/v1/tenants/{name} | – | `Tenant` |
| DELETE | /api/v1/tenants/{name} | – | 202 |
| GET | /api/v1/tenants/{name}/kubeconfig | – | kubeconfig YAML (`text/yaml`) — read from the Secret referenced by CR status |

`Tenant` JSON: `{name, displayName, owner, phase, message, resources{cpu,memory,storage}, createdAt}`
— `phase` mirrors CR status (`Pending|Provisioning|Ready|Deleting|Failed`);
API returns `Unknown` if the CR is missing.

Errors: `{"error": "<message>"}` with 400/401/403/404/409/500.

`name`: DNS-1123 label, max 40 chars (leaves room for `kubespaces-tenant-` prefix).

## Backend environment (set by the Helm chart)

| Var | Meaning |
|-----|---------|
| KUBESPACES_DB_HOST / PORT / NAME / USER / PASSWORD | Postgres connection |
| KUBESPACES_OIDC_ISSUER_URL | OIDC issuer (discovery) |
| KUBESPACES_OIDC_CLIENT_ID | expected audience/azp (`kubespaces`) |
| KUBESPACES_LISTEN_ADDR | default `:8080` |

In-cluster kubeconfig is implicit (ServiceAccount).

## Database schema (owned by backend, migrations embedded, run at startup)

```sql
CREATE TABLE tenants (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name         TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL DEFAULT '',
  owner        TEXT NOT NULL,             -- OIDC subject or email
  spec         JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at   TIMESTAMPTZ
);
CREATE TABLE audit_log (
  id     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  actor  TEXT NOT NULL,
  action TEXT NOT NULL,                   -- tenant.create | tenant.delete | ...
  detail JSONB NOT NULL DEFAULT '{}'::jsonb
);
```

## Tenant CR interaction

- API **creates** Tenant with `spec.owner` = token subject (email preferred),
  labels `app.kubernetes.io/managed-by: kubespaces-api`.
- API **deletes** the CR on DELETE; operator finalizer
  (`kubespaces.io/finalizer`) tears down and then the CR disappears. DB row is
  soft-deleted (`deleted_at`) immediately.
- Operator sets `status.phase`, `status.message`, `status.kubeconfigSecretRef`
  ({name, key}, Secret lives in `spec.targetNamespace`).
- vCluster's kubeconfig Secret is `vc-<vcluster-name>` with key `config`;
  operator copies/points status at it.
- Tenant namespace: `kubespaces-tenant-<name>`; vCluster release name = tenant
  name.

## Frontend

- Next.js 15 (app router) + Auth.js with Keycloak provider (PKCE public client
  `kubespaces`).
- Calls backend same-origin under `/api/v1/...` (ingress routes `/api` → API).
  Dev override: `KUBESPACES_API_URL`.
- Env: `AUTH_KEYCLOAK_ISSUER`, `AUTH_KEYCLOAK_ID` (=kubespaces),
  `AUTH_SECRET`, `AUTH_URL`.

## Container images

| Component | Image |
|-----------|-------|
| API | ghcr.io/kubespaces-io/api |
| Frontend | ghcr.io/kubespaces-io/frontend |
| Operator | ghcr.io/kubespaces-io/operator |

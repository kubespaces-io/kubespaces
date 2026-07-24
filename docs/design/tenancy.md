# Tenancy model: organizations, projects, memberships

Status: **in progress** (phased). This document tracks the port of the
organization/project/membership/role/invitation model from the pre-open-source
`login-api` into the open-source API, adapted for Keycloak.

## Hierarchy

```
Organization ──< Project ──< Tenant
     │              │
     └─ members     └─ members        (users + role)
     └─ invitations └─ invitations    (email, pending/accepted/declined/expired)
```

- **Organization** groups projects and members. Roles: `owner`, `admin`, `member`.
- **Project** belongs to an org and carries tenancy quotas (`maxTenants`,
  `maxCompute`, `maxMemoryGb`). Roles: `maintainer`, `member`.
- **Tenant** (existing) is linked to a project + org via nullable FKs; the
  operator/`Tenant` CR provisioning flow is unchanged.

## Adaptations from `login-api`

The old `login-api` **owned identity** (local passwords, GitHub OAuth, email
verification, approval, trials). The open-source stack delegates identity to
**Keycloak** (OIDC). So:

- **`users` is a thin projection** of Keycloak identities, JIT-provisioned from
  the OIDC token on first authenticated request (`UpsertUser` by `subject`).
  No password/verification/refresh columns — Keycloak owns those. `is_admin`
  tracks a Keycloak admin claim (and is togglable via the admin API).
- **Invitations** stay in the app DB (email-addressed, lifecycle preserved).
  Email delivery is best-effort/optional (logged when no SMTP is configured) —
  the invite is valid regardless.
- **HostCluster is deferred**: the open-source stack is single-host-cluster;
  multi-cluster placement is a separate roadmap item, so tenants don't carry a
  `host_cluster_id` yet.

## RBAC

Enforced in the API from the caller's membership role:

- Org: `owner` = full control incl. delete + member management; `admin` =
  manage projects + members; `member` = read + create projects.
- Project: `maintainer` = manage project, members, quotas, tenants; `member` =
  use tenants.
- Platform `is_admin` bypasses membership checks (admin surface).

## Phases

1. **Schema + store layer** (this change): migration `0002_tenancy.sql`;
   `store/tenancy.go` with users, orgs, projects, memberships.
2. **API + RBAC**: chi routes and handlers for orgs/projects/memberships, an
   RBAC middleware, JIT user provisioning in the auth middleware, tenant→project
   linkage on create.
3. **Invitations + admin**: org/project invitation endpoints (send/accept/
   decline), admin user management.
4. **UI**: portal screens for organizations, projects, members, and roles.

The endpoint contract mirrors `login-api`'s (see its `API_REFERENCE.md`),
minus the local-auth routes that Keycloak replaces.

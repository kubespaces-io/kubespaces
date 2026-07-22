# KubeSpaces Roadmap

Where KubeSpaces is going. This document states direction and intent — the
[GitHub Project](https://github.com/orgs/kubespaces-io/projects/3) and issues
are the operational source of truth; dates are deliberately absent. Items
move between milestones as reality intrudes. PRs and issue votes influence
ordering — tell us what you need.

**Current state (v0.1.x):** the full loop works — `helm install` → portal/CLI
→ `Tenant` CR → operator → vCluster → kubeconfig — with OIDC auth, quotas,
signed releases, and a pinned/mirrored vCluster supply chain.

## v0.2 — Reachable tenants (next up)

The theme: no more port-forwarding.

- [x] **Operator-managed tenant API exposure**: per-tenant `TLSRoute` (SNI
      passthrough) + `ReferenceGrant` on a shared Gateway;
      `status.apiServerUrl` on the Tenant; served kubeconfigs point at the
      public endpoint (#15)
- [x] **Tenant app exposure**: vCluster's native Gateway API sync with a
      per-tenant listener + certificate on the shared apps Gateway —
      structural hostname isolation (`*.{tenant}.apps.{domain}`) (#16)
- [ ] Chart `networking.*` values + documented Envoy Gateway / cert-manager /
      external-dns setup (see docs/prerequisites.md tiers)
- [ ] **E2E acceptance test in CI**: kind → install → tenant Ready →
      kubeconfig works, gating every release
- [x] Docs site at **docs.kubespaces.io** (MkDocs Material from `docs/`, #19)
- [x] CLI naming: resolved — the CLI is now `kubespaces` (was `spacectl`,
      which collided with Spacelift's CLI; see #20)
- [ ] Helm chart published as OCI artifact with provenance
      (`oci://ghcr.io/kubespaces-io/charts/kubespaces`)

## v0.3–v0.5 — Hardening & operability

The theme: safe to run for strangers. Tracks [docs/security.md](docs/security.md).

- [ ] Per-tenant **NetworkPolicy** (default-deny toward host services) and
      Pod Security Standards (restricted) on tenant namespaces
- [ ] `valuesOverrides` **policy guard** (admin-defined allowlist of vCluster
      options)
- [ ] Operator RBAC tightened to the generated role (drop cluster-admin)
- [ ] **Tenant lifecycle**: sleep/wake (vCluster pause), per-tenant vCluster
      version upgrades, tenant templates ("plans") for self-service with
      guardrails
- [ ] Usage visibility in the portal (quota consumption, tenant health) and
      audit log surfacing
- [ ] SLSA provenance attestations; GitHub Actions pinned by SHA
- [ ] Upgrade testing in CI (N-1 → N `helm upgrade` with live tenants)
- [ ] **k3k evaluated** as a second provisioner backend (competition keeps
      the vCluster dependency honest — see decision D16)

## v1.0 — Stability contract

The theme: things financial institutions ask about in the first meeting.

- [ ] `Tenant` CRD to **v1beta1 → v1** with conversion and a documented
      deprecation policy; strict semver everywhere from here on
- [ ] Written **threat model**; external penetration test (tenant-escape
      focused) with published summary
- [ ] HA guidance: control-plane replicas, external Postgres/IdP reference
      architectures, backup/restore documentation
- [ ] Production Keycloak guidance (bundled dev-mode instance stays
      evaluation-only)
- [ ] Conformance-style test matrix across managed clouds (GKE/EKS/AKS) and
      bare metal

## Exploring (no commitment)

- Multi-cluster: one control plane, many host clusters (agent model)
- Organizations/teams beyond the flat admin/member model (revisited when
  real installations ask — decision D11)
- GitOps tenant catalogs: Flux/Argo examples and reference pipelines
- **kubespaces.cloud** — hosted KubeSpaces ("KubeSpaces without running
  KubeSpaces"); waitlist first, operations honesty before promises

## Non-goals

- Replacing kubectl: the CLI hands you a kubeconfig and gets out of the way
- Web terminals in the portal (large attack surface, little value over a
  kubeconfig)
- Becoming a general PaaS — KubeSpaces provisions *clusters*, not apps

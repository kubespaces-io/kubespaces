# Changelog

All notable changes to KubeSpaces are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[semver](https://semver.org) (pre-1.0: minor bumps may break — see
[RELEASING.md](RELEASING.md)).

## [Unreleased]

### Added
- Windows release artifacts (zip) for spacectl
- SBOMs (syft) and cosign keyless signatures on release artifacts and images
- SECURITY.md, security architecture doc, CodeQL, Dependabot, secret-scanning
  push protection
- CONTRIBUTING.md (including the AI-assisted contribution policy), RELEASING.md

## [0.1.0] — 2026-07-18

First public release. 🎉

### Added
- **API** (Go): OIDC-authenticated tenant CRUD; persists metadata/audit to
  PostgreSQL and drives `Tenant` custom resources; kubeconfig retrieval
- **Operator** (Go, controller-runtime): sole provisioner — reconciles
  `Tenant` CRs (kubespaces.io/v1alpha1, cluster-scoped) into namespace +
  ResourceQuota + LimitRange + vCluster; finalizer teardown; status with
  phases and kubeconfig secret reference
- **Frontend** (Next.js): self-service portal with Keycloak (OIDC/PKCE)
  login, tenant list/create/delete, live provisioning status, kubeconfig
  download; tokens never reach the browser
- **spacectl** (Go): OIDC device-flow login, tenant management,
  `tenant kubeconfig --merge`; Homebrew tap + `curl | sh` installer
- **Umbrella Helm chart**: all-in-one install (API, portal, PostgreSQL 18,
  Keycloak 26.7) with production toggles (external DB, external OIDC,
  ingress or Gateway API HTTPRoute)
- **vCluster supply chain**: chart pinned (0.35.2) and installed from the
  KubeSpaces OCI mirror `ghcr.io/kubespaces-io/charts/vcluster`

[Unreleased]: https://github.com/kubespaces-io/kubespaces/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/kubespaces-io/kubespaces/releases/tag/v0.1.0

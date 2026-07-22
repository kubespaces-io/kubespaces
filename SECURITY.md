# Security Policy

KubeSpaces is a control plane for multi-tenant Kubernetes. We treat every
report that could affect tenant isolation, authentication, or the host cluster
as a priority. Thank you for helping keep the project and its users safe.

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Preferred channel — **GitHub private vulnerability reporting**:

1. Go to [Security → Report a vulnerability](https://github.com/kubespaces-io/kubespaces/security/advisories/new)
2. Describe the issue, affected component/version, and reproduction steps.

Alternative channel — email **security@kubespaces.io** with the same details.
If you need to send sensitive material encrypted, say so in a first plain
message and we will provide a key.

A good report includes: affected component (api / operator / frontend /
CLI / Helm chart), version or commit, environment (host Kubernetes
version, vCluster version), impact assessment, and a proof of concept or
reproduction steps. Crashes, PoCs and logs are welcome; live exploitation of
other people's deployments is not.

## What to expect

| Stage | Target |
|-------|--------|
| Acknowledgement | within **48 hours** |
| Triage & severity assessment (CVSS) | within **7 days** |
| Fix or mitigation for Critical/High | within **30 days** |
| Fix for Medium/Low | next scheduled release |
| Coordinated public disclosure | within **90 days** of report, or earlier once a fix ships |

We will keep you informed at every stage, credit you in the advisory and
release notes (unless you prefer anonymity), and publish a GitHub Security
Advisory with a CVE for confirmed vulnerabilities.

## Scope

**In scope**

- This repository: `api`, `operator`, `frontend`, `cli`, the umbrella
  Helm chart, CI/release workflows
- Container images published under `ghcr.io/kubespaces-io/*`
- The vCluster chart **mirror** at `ghcr.io/kubespaces-io/charts/vcluster`
  (integrity of the mirroring itself)
- Classes of issues we care deeply about: **tenant isolation escapes**
  (tenant → host or tenant → tenant), authentication/authorization bypass in
  the API or portal, privilege escalation via the operator's RBAC, secrets
  exposure (kubeconfigs, tokens), injection via Tenant CR fields
  (e.g. `valuesOverrides`)

**Out of scope**

- Vulnerabilities in vCluster itself → report to
  [Loft Labs](https://github.com/loft-sh/vcluster/security) (we still
  appreciate a heads-up so we can pin a fixed version)
- Vulnerabilities in Keycloak, PostgreSQL, or other upstream dependencies →
  report upstream; tell us if KubeSpaces's default configuration makes them
  exploitable
- Denial of service requiring unrealistic volumes, results from automated
  scanners without a demonstrated impact, social engineering, and issues in
  deployments configured against our documented hardening guidance

## Supported versions

Pre-1.0, only the **latest release** receives security fixes. From 1.0 on,
the latest minor release and the one before it will be supported.

| Version | Supported |
|---------|-----------|
| latest release (0.x) | ✅ |
| older releases | ❌ upgrade |

## Safe harbor

We will not pursue legal action against, or report to law enforcement,
researchers who: act in good faith, avoid privacy violations and data
destruction, do not exploit findings beyond what is needed to demonstrate
them, give us reasonable time to remediate before public disclosure, and
comply with applicable laws. Testing must be done against **your own**
KubeSpaces deployment, not someone else's.

## Current security posture & roadmap

See [docs/security.md](docs/security.md) for the project's security
architecture, current guarantees, and the hardening roadmap (image signing,
SBOMs, provenance, threat model).

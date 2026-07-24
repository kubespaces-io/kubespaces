# Versioning & Releases

## Semver policy

KubeSpaces follows [Semantic Versioning 2.0.0](https://semver.org) with one
version line for the entire monorepo: a single `vX.Y.Z` tag releases the
`api`/`operator`/`frontend` images, the `kubespaces` CLI binaries, and (soon) the
Helm chart at the same version.

- **Pre-1.0 (`0.y.z`)**: minor bumps (`0.y`) may contain breaking changes —
  always called out in the changelog under **Breaking**. Patch bumps
  (`0.y.z`) never break.
- **From 1.0**: strict semver. Breaking changes to the HTTP API, the
  `Tenant` CRD schema, chart values, or CLI flags require a major bump.
- **CRD versions are independent of release versions**: `kubespaces.io/v1alpha1`
  graduates through Kubernetes API conventions (v1alpha1 → v1beta1 → v1) with
  conversion support, regardless of the repo version.

## Changelog

[CHANGELOG.md](CHANGELOG.md) follows [Keep a Changelog](https://keepachangelog.com):
an `Unreleased` section accumulates rich entries per PR (the conventional-commit
type decides the section). At release time, `make changelog VERSION=X.Y.Z`
(via [`scripts/release-changelog.sh`](scripts/release-changelog.sh)) promotes
`Unreleased` into a dated `[X.Y.Z]` section and rebuilds the compare-link
footer — no hand-editing. If `Unreleased` happens to be empty, the script
synthesizes entries from conventional commits since the previous tag
(feat→Added, fix→Fixed, perf/refactor→Changed, `!`/BREAKING→Breaking;
docs/chore/ci/test/build excluded).

Separately, goreleaser generates the **GitHub Release notes** from conventional
commits, grouped Breaking / Features / Fixes / Other. The two are complementary:
CHANGELOG.md is the curated human history; the Release notes are the raw commit
digest for that tag.

## Cutting a release (maintainers)

1. Ensure `main` is green and, if the vCluster pin changed, the mirror ran
   first (`mirror-vcluster-chart` workflow, then bump `DefaultChartVersion`).
2. Prepare the release — promotes the changelog and bumps the chart in one
   step (or run `make changelog` / `make chart-version` individually):
   ```bash
   make release-prep VERSION=X.Y.Z
   git diff                       # review the generated changelog + chart bump
   git commit -am "chore: release vX.Y.Z"
   ```
3. Tag and push:
   ```bash
   git tag vX.Y.Z -m "KubeSpaces X.Y.Z"
   git push origin main vX.Y.Z
   ```
4. Automation takes over:
   - **release.yml** → goreleaser: darwin/linux/windows binaries, archives,
     `checksums.txt`, **SBOMs** (syft, SPDX), **cosign keyless signatures**
     on the checksums, GitHub release with grouped notes, Homebrew formula
     push to `kubespaces-io/homebrew-tap`.
   - **build.yml** → multi-arch images to `ghcr.io/kubespaces-io/{api,operator,frontend}`
     tagged `X.Y.Z`, `X.Y`, `sha-*`, signed with **cosign** (keyless).
5. Verify: `kubespaces version` from a fresh
   `curl -fsSL https://kubespaces.io/install.sh | sh`, and
   `cosign verify` per [docs/security.md](docs/security.md).

## Release integrity

- Releases are built **only by CI from tags** — never from laptops.
- Signatures are keyless (Sigstore): the signing identity is this repo's
  GitHub Actions workflow, verifiable by anyone without key management.
- `install.sh` verifies checksums; Homebrew verifies its own SHAs.

## Secrets required by the pipeline

| Secret | Used by | Purpose |
|--------|---------|---------|
| `TAP_GITHUB_TOKEN` | release.yml | push formula to homebrew-tap (fine-grained PAT, contents:write) |

Everything else runs on the default `GITHUB_TOKEN` (+ OIDC for cosign).

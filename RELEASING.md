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
an `Unreleased` section accumulates entries per PR (the conventional-commit
type decides the section), and release notes are generated from conventional
commits by goreleaser, grouped Breaking / Features / Fixes / Other.

## Cutting a release (maintainers)

1. Ensure `main` is green and, if the vCluster pin changed, the mirror ran
   first (`mirror-vcluster-chart` workflow, then bump `DefaultChartVersion`).
2. Move `Unreleased` entries in CHANGELOG.md under the new version + date;
   commit `chore: release vX.Y.Z`.
3. Tag and push:
   ```bash
   git tag vX.Y.Z -m "KubeSpaces X.Y.Z"
   git push origin vX.Y.Z
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

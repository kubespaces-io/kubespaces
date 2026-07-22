# Installation

One Helm chart installs the whole control plane. This page covers the knobs
that matter; the full value list is in [Chart values](../reference/chart-values.md).

## The chart

```bash
git clone https://github.com/kubespaces-io/kubespaces
cd kubespaces
helm install kubespaces charts/kubespaces \
  --namespace kubespaces --create-namespace \
  --set operator.enabled=true
```

!!! note "OCI chart publishing"
    Publishing the chart as a versioned OCI artifact
    (`oci://ghcr.io/kubespaces-io/charts/kubespaces`) is tracked in
    [#21](https://github.com/kubespaces-io/kubespaces/issues/21) — until
    then, install from a repo checkout.

## What the default install contains

| Component | Default | Production guidance |
|---|---|---|
| API | ✅ | — |
| Web portal | ✅ | — |
| Tenant operator | opt-in (`operator.enabled=true`) | enable it; it is the product |
| PostgreSQL | ✅ built-in StatefulSet, official `postgres` image | point at managed Postgres: `postgresql.enabled=false` + `externalDatabase.*` |
| Keycloak | ✅ built-in, **dev mode**, realm auto-imported | bring your own OIDC: `keycloak.enabled=false` + `oidc.*` — the bundled instance is for evaluation only |

There are **no Bitnami subcharts** — the built-ins are minimal StatefulSets
on official images, designed to be swapped out, not scaled up.

## Profiles

The `examples/` directory ships three values profiles:

- `values-kind-demo.yaml` — the [Quickstart](quickstart.md) tier
- `values-gke-test.yaml` — a real cluster with tenant API + app exposure
  enabled (the configuration the E2E suite runs against)
- `values-production.yaml` — external Postgres, external OIDC, real ingress

## Enabling tenant exposure

Public tenant endpoints require one-time host-cluster preparation (Gateway
API implementation, shared Gateways, DNS, cert-manager) — the full runbook is
[Host cluster preparation](../host-cluster.md). Once prepared:

```yaml
operator:
  enabled: true
  tenantApi:
    domain: example.com
    gateway: {name: kubespaces-api, namespace: kubespaces-system, sectionName: apipassthrough}
  tenantApps:
    domain: example.com
    gateway: {name: kubespaces-apps, namespace: kubespaces-system}
    clusterIssuer: letsencrypt-dns01
```

Leave these empty and KubeSpaces runs in evaluation mode (port-forward
access) — nothing else changes.

## Verifying what you run

Every image is built by GitHub Actions and signed with cosign (keyless,
GitHub OIDC identity). Verify before trusting:

```bash
cosign verify ghcr.io/kubespaces-io/operator:0.2.0 \
  --certificate-identity-regexp \
    '^https://github.com/kubespaces-io/kubespaces/.github/workflows/build.yml@refs/tags/v.*$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

SBOMs (syft) ship with every release artifact. The vCluster chart the
operator installs is **pinned and mirrored** at
`oci://ghcr.io/kubespaces-io/charts/vcluster` — upstream changes can never
break existing installs. Details: [Security](../security.md).

## Upgrading

One version line: a `vX.Y.Z` tag releases the images and the CLI together.
Watch the [changelog](https://github.com/kubespaces-io/kubespaces/blob/main/CHANGELOG.md);
pre-1.0, minor versions may break (see
[RELEASING.md](https://github.com/kubespaces-io/kubespaces/blob/main/RELEASING.md)).

```bash
git pull
kubectl apply -f operator/config/crd/kubespaces.io_tenants.yaml   # CRDs first
helm upgrade kubespaces charts/kubespaces -n kubespaces -f your-values.yaml
```

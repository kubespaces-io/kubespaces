# Chart values

The umbrella chart (`charts/kubespaces`) installs the whole control plane.
This page covers the values that change behavior;
[`values.yaml`](https://github.com/kubespaces-io/kubespaces/blob/main/charts/kubespaces/values.yaml)
is the exhaustive, commented source of truth, and `examples/` holds complete
profiles (kind demo, GKE test, production).

## Components

```yaml
api:
  image: {repository: ghcr.io/kubespaces-io/api, tag: "", pullPolicy: IfNotPresent}

frontend:
  image: {repository: ghcr.io/kubespaces-io/frontend, tag: ""}

operator:
  enabled: false          # flip on — it is the product
  installCRDs: true
  image: {repository: ghcr.io/kubespaces-io/operator, tag: ""}
```

Empty tags follow the chart's appVersion. For dev clusters use
`pullPolicy: Always` or pin digests — mutable tags and node caches disagree.

## Data & identity

```yaml
postgresql:
  enabled: true           # built-in StatefulSet, official postgres image
  persistence: {enabled: true, size: 8Gi, storageClass: ""}
# production: enabled: false + externalDatabase host/port/user/passwordSecret

keycloak:
  enabled: true           # built-in, DEV MODE, realm auto-imported
# production: enabled: false + oidc issuer/clientID of your IdP
```

The bundled Keycloak runs `start-dev` and exists for evaluation — bringing
your own OIDC issuer is a values change, not a migration: the API and
portal speak generic OIDC.

## Exposure of the control plane itself

```yaml
ingress:
  enabled: false          # classic Ingress for portal + API

gatewayApi:               # or: ride an existing Gateway as an HTTPRoute
  enabled: false
  host: kubespaces.example.com
  gateway: {name: kubespaces-apps, namespace: kubespaces-system, sectionName: ""}
```

## Tenant exposure (operator)

```yaml
operator:
  # vCluster chart source — defaults to the pinned KubeSpaces OCI mirror
  vclusterChartRef: ""      # e.g. oci://ghcr.io/kubespaces-io/charts/vcluster
  vclusterChartVersion: ""  # e.g. 0.35.2

  tenantApi:                # <tenant>.api.<domain> — TLS passthrough
    domain: ""
    gateway:
      name: ""              # e.g. kubespaces-api
      namespace: ""         # e.g. kubespaces-system
      sectionName: ""       # e.g. apipassthrough
    externalDnsTarget: ""   # optional external-dns override (on-prem IP)

  tenantApps:               # <app>.<tenant>.apps.<domain> — TLS terminate
    domain: ""
    gateway:
      name: ""              # e.g. kubespaces-apps
      namespace: ""
    clusterIssuer: ""       # cert-manager ClusterIssuer for per-tenant wildcards
```

Semantics:

- **Empty = off.** With no `tenantApi`/`tenantApps` config the operator
  provisions tenants without any public endpoints (the evaluation tier) —
  identical behavior to pre-exposure releases.
- `tenantApi` requires domain + gateway name + namespace; `tenantApps`
  additionally requires the ClusterIssuer.
- These values become `KUBESPACES_*` environment variables on the operator
  Deployment — useful to know when debugging (`kubectl describe deploy`).
- The Gateways themselves are **not** chart-managed — they are host
  infrastructure, one per cluster: [Host cluster preparation](../host-cluster.md).

## Verifying a render

```bash
helm template kubespaces charts/kubespaces -f your-values.yaml | less
helm lint charts/kubespaces
```

# The production gateway stack

This guide is the production-flavored, end-to-end walkthrough of the three
host-cluster components that make tenants reachable from the internet:

| Component | Role |
|---|---|
| **[Envoy Gateway](#1-envoy-gateway)** | Gateway API implementation — the two shared Gateways (API TLS passthrough, apps TLS terminate) |
| **[cert-manager](#2-cert-manager-dns-01-wildcard-issuer)** | Issues the per-tenant apps wildcard certificates via a **DNS-01** `ClusterIssuer` |
| **[external-dns](#3-external-dns)** | Publishes `*.api.<domain>` / `*.apps.<domain>` records automatically |

These are **host infrastructure**, installed once per cluster and **not**
bundled in the KubeSpaces umbrella chart. After they exist, the operator
automates everything per tenant (`TLSRoute`, `ReferenceGrant`, per-tenant
wildcard `Certificate`, per-tenant listener, DNS annotations, kubeconfig).

This is the **Tier 2** ("real deployment") stack. If you only want the
10-minute demo — port-forward access, no domain, no load balancer — you need
none of this; see [Prerequisites](../prerequisites.md) for the two-tier model,
and [Local reachable tenants (kind)](../getting-started/local-kind.md) for a
NodePort variant of the same two Gateways. For the concepts (why the API path
is passthrough, why hostname theft is impossible), see
[Networking](../concepts/networking.md).

!!! note "Nothing here is Envoy-specific"
    KubeSpaces depends only on a Gateway API implementation with **`TLSRoute`**
    support. Envoy Gateway is the reference; Contour and Istio are equally
    known-good (the pre-OSS KubeSpaces ran this exact Gateway shape on Istio
    for years). Swap the `GatewayClass` `controllerName` and the install
    command accordingly.

## Prerequisites

- A **domain you control**, e.g. `kubespaces.example.com`.
- A **DNS zone with API access** (Cloud DNS, Route 53, Azure DNS, Cloudflare,
  …). Required for cert-manager DNS-01 (wildcard certs cannot be issued over
  HTTP-01) and, optionally, external-dns.
- `LoadBalancer` Service support with a reachable IP (cloud: built-in;
  on-prem: MetalLB / kube-vip).

One IP can serve both Gateways; splitting API and apps onto two IPs is cleaner
but not required.

---

## 1. Envoy Gateway

Install the implementation. Envoy Gateway bundles the Gateway API CRDs it needs
(`TLSRoute` included), so this is a one-liner:

```bash
helm install eg oci://docker.io/envoyproxy/gateway-helm \
  -n envoy-gateway-system --create-namespace --wait
```

Confirm `TLSRoute` is present (the API gateway needs it):

```bash
kubectl get crd tlsroutes.gateway.networking.k8s.io
```

`TLSRoute` is GA (`v1`, Standard channel) since **Gateway API v1.5**
(February 2026). On an older Gateway API install it only exists in the
**experimental channel** as `v1alpha2`; if the CRD is missing, install the
experimental-channel CRDs from the
[Gateway API releases](https://github.com/kubernetes-sigs/gateway-api/releases).

### GatewayClass + the two Gateways

Create one namespace, one `GatewayClass`, and the two shared Gateways. This is
the production counterpart of [`examples/host/kind/host.yaml`](https://github.com/kubespaces-io/kubespaces/tree/main/examples/host/kind/host.yaml)
(which pins NodePorts for kind); here the Envoy Services are plain
`LoadBalancer`s.

```yaml
# gateway-stack.yaml — kubectl apply -f gateway-stack.yaml  (replace <domain>)
apiVersion: v1
kind: Namespace
metadata:
  name: kubespaces-system
---
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: envoy
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
---
# API Gateway — TLS PASSTHROUGH. Routes on SNI, never terminates TLS, so
# clients verify vCluster's own serving cert end-to-end and client-cert
# kubeconfigs keep working. No cert-manager on this path.
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: kubespaces-api
  namespace: kubespaces-system
spec:
  gatewayClassName: envoy
  listeners:
    - name: apipassthrough          # -> networking.apiGateway.sectionName
      hostname: "*.api.<domain>"
      port: 443
      protocol: TLS
      tls:
        mode: Passthrough
      allowedRoutes:
        namespaces:
          from: Same                # only the operator's namespace may attach
                                    # routes here — tenant namespaces never can
---
# Apps Gateway — TLS TERMINATE. One static base listener; the operator adds one
# HTTPS listener per tenant (t-<tenant>, *.<tenant>.apps.<domain>, allowedRoutes
# locked to that tenant's namespace). The http listener is the static base
# (Gateway API requires >=1 listener; useful for HTTP->HTTPS redirects).
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: kubespaces-apps
  namespace: kubespaces-system
spec:
  gatewayClassName: envoy
  listeners:
    - name: http
      port: 80
      protocol: HTTP
      allowedRoutes:
        namespaces:
          from: Same
```

Wait for each Gateway to program and pick up an address:

```bash
kubectl get gateway -n kubespaces-system
# PROGRAMMED=True and an ADDRESS on each
```

!!! warning "GitOps: ignore listener drift on the apps Gateway"
    The operator writes `spec.listeners` on `kubespaces-apps` (one listener per
    tenant). If you manage it with Flux/Argo CD, configure the tool to ignore
    drift on `spec.listeners` (Flux `spec.ignore`, Argo CD
    `ignoreDifferences`) or the reconciler will fight the operator. The API
    Gateway is fully static — no exception needed.

---

## 2. cert-manager (DNS-01 wildcard issuer)

The **apps** path terminates TLS, so it needs real certificates. The operator
issues a per-tenant **wildcard** `Certificate` (`*.<tenant>.apps.<domain>`),
and a wildcard certificate can **only** be issued via **DNS-01** — HTTP-01
cannot cover wildcards. (The API path is passthrough and needs no cert-manager
at all.)

Install cert-manager with its CRDs:

```bash
helm install cert-manager oci://quay.io/jetstack/charts/cert-manager \
  -n cert-manager --create-namespace --set crds.enabled=true --wait
```

### A DNS-01 `ClusterIssuer`

Give cert-manager API credentials for your DNS zone as a Secret, then create a
Let's Encrypt DNS-01 `ClusterIssuer`. Cloudflare shown; Route 53 / Cloud DNS /
Azure DNS follow the same shape (see the
[cert-manager DNS-01 docs](https://cert-manager.io/docs/configuration/acme/dns01/)).

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cloudflare-api-token
  namespace: cert-manager
stringData:
  token: <cloudflare-api-token>       # Zone.DNS edit permission
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-dns01
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: you@example.com
    privateKeySecretRef:
      name: letsencrypt-dns01-key
    solvers:
      - dns01:
          cloudflare:
            apiTokenSecretRef:
              name: cloudflare-api-token
              key: token
```

This `ClusterIssuer` name (`letsencrypt-dns01`) is what you hand KubeSpaces as
`networking.clusterIssuer` below. For a throwaway evaluation you can instead
use a `selfSigned` `ClusterIssuer` (browsers warn) — see
[`examples/host/gateway-apps.yaml`](https://github.com/kubespaces-io/kubespaces/tree/main/examples/host/gateway-apps.yaml).

---

## 3. external-dns

Two options for publishing DNS. **Static wildcards** are simplest; skip
external-dns entirely and create two records by hand:

| Record | Points at | Covers |
|---|---|---|
| `*.api.<domain>` | API Gateway address | `<tenant>.api.<domain>` |
| `*.apps.<domain>` | Apps Gateway address | `<app>.<tenant>.apps.<domain>` (DNS wildcards match multiple labels) |

**Or run external-dns** for per-tenant records. The operator already annotates
every `TLSRoute` it creates with
`external-dns.alpha.kubernetes.io/hostname` (the tenant's
`<tenant>.api.<domain>`), so external-dns watching `tlsroute` (and
`gateway-httproute`) sources publishes records automatically as tenants come
and go:

```bash
helm install external-dns oci://registry-1.docker.io/bitnamicharts/external-dns \
  -n external-dns --create-namespace \
  --set provider=cloudflare \
  --set 'sources={gateway-httproute,gateway-tlsroute,service}' \
  --set cloudflare.apiToken=<cloudflare-api-token>
```

On-prem, where the Gateway's LoadBalancer IP is not the address the world
should resolve to (e.g. a NAT / appliance in front), set
`networking.externalDnsTarget` to the public IP — the operator adds
`external-dns.alpha.kubernetes.io/target` to the routes so external-dns
publishes that address instead.

---

## 4. Wire it into KubeSpaces

Point the chart at the stack you just built. The top-level `networking.*`
values are the shared, documented home for the domain and the two Gateways;
they drive the operator for both the API and apps paths:

```yaml
# values.yaml
operator:
  enabled: true

networking:
  domain: kubespaces.example.com     # tenants get <tenant>.api.<domain>
                                     # and <app>.<tenant>.apps.<domain>
  apiGateway:
    name: kubespaces-api
    namespace: kubespaces-system
    sectionName: apipassthrough      # the TLS-passthrough listener name
  appsGateway:
    name: kubespaces-apps
    namespace: kubespaces-system
  clusterIssuer: letsencrypt-dns01   # the cert-manager DNS-01 ClusterIssuer
  # externalDnsTarget: 203.0.113.10  # on-prem only: public IP override
```

```bash
helm upgrade --install kubespaces charts/kubespaces -f values.yaml
```

These values become `KUBESPACES_*` environment variables on the operator
Deployment (`KUBESPACES_API_DOMAIN`, `KUBESPACES_GATEWAY_NAME/NAMESPACE/SECTION`,
`KUBESPACES_APPS_DOMAIN`, `KUBESPACES_APPS_GATEWAY_NAME/NAMESPACE`,
`KUBESPACES_APPS_CLUSTER_ISSUER`, `KUBESPACES_EXTERNAL_DNS_TARGET`) — handy when
debugging with `kubectl describe deploy`.

!!! note "Older `operator.tenantApi.*` / `operator.tenantApps.*` values"
    Those per-path keys still work and take precedence over `networking.*` when
    set — the two spellings are fully interchangeable and backward compatible.
    `networking.*` is the recommended, DRY entry point (set the domain once);
    the granular keys let you override a single field (e.g. a different domain
    for apps than for the API). See the
    [Chart values reference](../reference/chart-values.md).

### Enablement rules

- **Empty = off** (Tier 1 demo — tenants provisioned with no public endpoints).
- The **API path** activates when domain + API gateway name + namespace are set.
- The **apps path** additionally needs the apps gateway name + namespace and
  the `clusterIssuer`.

## 5. Verify

```bash
kubectl apply -f - <<'EOF'
apiVersion: kubespaces.io/v1alpha1
kind: Tenant
metadata: {name: smoke}
spec: {owner: you@example.com}
EOF
kubectl wait tenant smoke --for=jsonpath='{.status.phase}'=Ready --timeout=5m

# Public API endpoint (TLS passthrough — verifies vCluster's own cert)
kubectl get secret vc-smoke -n kubespaces-tenant-smoke \
  -o jsonpath='{.data.config}' | base64 -d > /tmp/smoke.kubeconfig
kubectl --kubeconfig /tmp/smoke.kubeconfig get nodes

# Per-tenant apps wildcard Certificate issued by cert-manager
kubectl get certificate -n kubespaces-tenant-smoke

kubectl delete tenant smoke
```

## Related

- [Host cluster preparation](../host-cluster.md) — the reference-manifest,
  step-by-step version of this stack (including the apps-path GitOps notes).
- [Prerequisites](../prerequisites.md) — the demo vs production tier model.
- [Networking](../concepts/networking.md) — why the API path is passthrough and
  why hostname theft is structurally impossible.
- [Exposing apps](expose-apps.md) — how tenant users publish an `HTTPRoute`.
- [Chart values](../reference/chart-values.md) — the full `networking.*` /
  `operator.tenant*` reference.

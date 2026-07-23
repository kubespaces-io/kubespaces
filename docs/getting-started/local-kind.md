# Local reachable tenants on kind

The [Prerequisites](../prerequisites.md) split KubeSpaces into two tiers: Tier 1
(kind, port-forward) and Tier 2 (a domain, DNS, a load balancer). This guide is
the missing middle — the **full Tier 2 networking path on a local kind
cluster**, with no domain, no cloud DNS, and no load balancer. You get real,
TLS-verified tenant API endpoints and working app URLs, entirely on your
laptop.

Two tricks make it work:

- **[nip.io](https://nip.io) for DNS.** Any hostname ending in
  `<anything>.127.0.0.1.nip.io` resolves to `127.0.0.1`. That matches the
  operator's hostname scheme (`<tenant>.api.<domain>`,
  `*.<tenant>.apps.<domain>`) with zero per-tenant DNS work — one reason to
  prefer it over `/etc/hosts`, which can't do the apps wildcard.
- **kind port mappings + pinned Envoy NodePorts** to expose the gateways on the
  host. The operator bakes the port into each served kubeconfig
  (`https://<tenant>.api.<domain>:443`), so the API gateway must answer on the
  host's real `:443`; the apps gateway rides `:8443`.

Everything below lives in
[`examples/host/kind/`](https://github.com/kubespaces-io/kubespaces/tree/main/examples/host/kind).

## Prerequisites

- Docker, [kind](https://kind.sigs.k8s.io/), `kubectl`, `helm`
- Ports `80`, `443` and `8443` free on `127.0.0.1`
- This repository checked out (you build the operator image locally, and apply
  the CRD and example manifests from it)

## 1. Create the cluster

The port mappings wire the host to the (pinned) Envoy NodePorts:

```bash
kind create cluster --config examples/host/kind/kind-cluster.yaml
```

## 2. Install the host prerequisites

Envoy Gateway (brings the Gateway API CRDs, including `TLSRoute`) and
cert-manager (for the per-tenant apps certificates):

```bash
helm install eg oci://docker.io/envoyproxy/gateway-helm \
  -n envoy-gateway-system --create-namespace --wait
helm install cert-manager oci://quay.io/jetstack/charts/cert-manager \
  -n cert-manager --create-namespace --set crds.enabled=true --wait
```

Then the two shared Gateways, the self-signed issuer, and the EnvoyProxy
objects that pin the NodePorts:

```bash
kubectl apply -f examples/host/kind/host.yaml
kubectl -n kubespaces-system wait --for=condition=Programmed \
  gateway/kubespaces-api gateway/kubespaces-apps --timeout=120s
```

## 3. Build and load the operator image

Released operator images (≤ 0.2.0) predate the tenant-networking feature, so
build from source and side-load it into the cluster:

```bash
docker build -t ghcr.io/kubespaces-io/operator:dev-kind operator
kind load docker-image ghcr.io/kubespaces-io/operator:dev-kind --name kubespaces
```

## 4. Install KubeSpaces

The chart does not yet ship the `Tenant` CRD, so apply it first, then install
with the kind values (operator networking pointed at nip.io):

```bash
kubectl apply -f operator/config/crd/kubespaces.io_tenants.yaml
helm install kubespaces charts/kubespaces -n kubespaces --create-namespace \
  -f examples/host/kind/values.yaml
kubectl -n kubespaces rollout status deploy/kubespaces-operator --timeout=180s
```

## 5. Create a tenant

```bash
kubectl apply -f - <<'EOF'
apiVersion: kubespaces.io/v1alpha1
kind: Tenant
metadata:
  name: demo
spec:
  owner: you@example.com
EOF
kubectl wait tenant/demo --for=jsonpath='{.status.phase}'=Ready --timeout=5m
```

When it is `Ready`, the operator has published the endpoints on the CR:

```console
$ kubectl get tenant demo -o jsonpath='{.status.apiServerUrl}{"\n"}{.status.appsDomain}{"\n"}'
https://demo.api.127.0.0.1.nip.io:443
*.demo.apps.127.0.0.1.nip.io
```

## 6. Reach the tenant API

The served kubeconfig already points at the public endpoint and carries the
vCluster's own CA — no `-k`, no port-forward:

```bash
kubectl -n kubespaces-tenant-demo get secret vc-demo \
  -o jsonpath='{.data.config}' | base64 -d > demo.kubeconfig

kubectl --kubeconfig demo.kubeconfig get nodes
# NAME                       STATUS   ROLES    AGE   VERSION
# kubespaces-control-plane   Ready    <none>   1m    v1.36.0
```

That request went out over `https://demo.api.127.0.0.1.nip.io:443`, through the
passthrough gateway by SNI, into the vCluster — fully TLS-verified.

## 7. Expose an app from inside the tenant

Deploy a workload and a plain `HTTPRoute` in the tenant's **virtual `default`
namespace**, pointing at the apps Gateway that vCluster projects there:

```bash
kubectl --kubeconfig demo.kubeconfig apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: default}
spec:
  replicas: 1
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec:
      containers:
        - name: web
          image: hashicorp/http-echo:1.0
          args: ["-text=hello from tenant demo", "-listen=:5678"]
---
apiVersion: v1
kind: Service
metadata: {name: web, namespace: default}
spec:
  selector: {app: web}
  ports: [{port: 80, targetPort: 5678}]
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata: {name: web, namespace: default}
spec:
  parentRefs: [{name: kubespaces-apps}]
  hostnames: ["web.demo.apps.127.0.0.1.nip.io"]
  rules: [{backendRefs: [{name: web, port: 80}]}]
EOF
```

vCluster syncs the route to the host and translates the parentRef back to the
real gateway. Curl it over the apps endpoint (self-signed cert, so `-k`):

```bash
curl -sk https://web.demo.apps.127.0.0.1.nip.io:8443/
# hello from tenant demo
```

## Isolation still holds

The isolation is structural, not policy-based — the same on kind as in
production. A tenant route that claims a hostname outside its own subdomain
matches no listener it is allowed to attach to:

```console
$ # an HTTPRoute in tenant demo claiming web.victim.apps.127.0.0.1.nip.io
$ kubectl -n kubespaces-tenant-demo get httproute -o \
    'jsonpath={range .items[*]}{.metadata.name}{"\t"}{.status.parents[0].conditions[?(@.type=="Accepted")].reason}{"\n"}{end}'
web-x-default-x-demo     Accepted
evil-x-default-x-demo    NoMatchingListenerHostname
```

## Tear down

```bash
kind delete cluster --name kubespaces
```

## How this differs from production

| | This guide (kind) | Production (Tier 2) |
|---|---|---|
| DNS | nip.io (`*.127.0.0.1.nip.io`) | your domain + external-dns |
| Apps certificate | self-signed `ClusterIssuer` (`-k`) | DNS-01 issuer, real wildcard cert |
| Gateway address | host port mappings + pinned NodePorts | `LoadBalancer` IP(s) |
| Apps port | `:8443` (host `:443` taken by the API gateway) | `:443` on a second IP |
| Operator image | built + side-loaded | released image |

The operator, the `Tenant` CR, the TLSRoute/listener wiring, and the isolation
model are **identical** — only the host plumbing around them changes.

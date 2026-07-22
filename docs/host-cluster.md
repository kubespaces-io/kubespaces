# Preparing the host cluster for tenant exposure

**The host cluster must be prepared before tenants can be reached from
outside.** A bare cluster + `helm install kubespaces` gives you the Tier 1
demo (port-forward access, see [prerequisites.md](prerequisites.md)); public
tenant endpoints additionally need the pieces on this page, installed once
per host cluster. Every step here is cluster-admin, day-0 platform work —
after it, tenant exposure is fully automated by the operator.

The reference implementation below uses **Envoy Gateway**, but nothing in
KubeSpaces depends on it: any Gateway API implementation that supports
**`TLSRoute`** works (the pre-OSS KubeSpaces production setup ran the exact
same Gateway shape on Istio for years). Contour, Istio, Envoy Gateway are all
known-good TLSRoute implementations; check your implementation's conformance
docs before picking something else.

## 1. Gateway API implementation (with TLSRoute)

`TLSRoute` is GA (`v1`, Standard channel) since **Gateway API v1.5**
(February 2026); on older Gateway API installs it only exists in the
**experimental channel** as `v1alpha2` — a pre-1.5 standard channel CRD
install does *not* include it. Envoy Gateway bundles the CRDs it needs
(TLSRoute included), which makes this a one-liner:

```bash
helm install eg oci://docker.io/envoyproxy/gateway-helm \
  -n envoy-gateway-system --create-namespace --wait
```

If you bring another implementation, make sure
`kubectl get crd tlsroutes.gateway.networking.k8s.io` succeeds afterwards; if
it does not, install the experimental channel CRDs from
[gateway-api releases](https://github.com/kubernetes-sigs/gateway-api/releases).

## 2. The tenant API Gateway (TLS passthrough)

One shared Gateway carries every tenant's API server traffic. The listener is
TLS **Passthrough**: the gateway routes on SNI and never terminates TLS, so
clients verify vCluster's own serving certificate end-to-end and client-cert
kubeconfigs keep working. No cert-manager involvement on this path.

```bash
kubectl apply -f examples/host/gateway-api.yaml
```

That manifest (adjust `<domain>`) is:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: envoy
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: kubespaces-api
  namespace: kubespaces-system
spec:
  gatewayClassName: envoy
  listeners:
    - name: apipassthrough          # referenced by operator.tenantApi.gateway.sectionName
      hostname: "*.api.<domain>"
      port: 443
      protocol: TLS
      tls:
        mode: Passthrough
      allowedRoutes:
        namespaces:
          from: All                 # operator-created TLSRoutes live in this namespace,
                                    # backends in tenant namespaces (via ReferenceGrant)
```

Wait for an address:

```bash
kubectl get gateway kubespaces-api -n kubespaces-system
# PROGRAMMED=True and an ADDRESS
```

## 3. DNS

Point tenant API hostnames at the Gateway's address. Two options:

- **Wildcard record** (simplest): `*.api.<domain>` → the Gateway's
  LoadBalancer IP. One record, covers every tenant forever.
- **external-dns**: the operator annotates every TLSRoute with
  `external-dns.alpha.kubernetes.io/hostname` (and honors an optional target
  override for on-prem appliances via `operator.tenantApi.externalDnsTarget`),
  so external-dns watching `tlsroute` sources creates per-tenant records
  automatically.

## 4. Tell KubeSpaces about it

```yaml
# values.yaml
operator:
  enabled: true
  tenantApi:
    domain: <domain>                 # tenants get <tenant>.api.<domain>
    gateway:
      name: kubespaces-api
      namespace: kubespaces-system
      sectionName: apipassthrough
```

All three of `domain`, `gateway.name` and `gateway.namespace` must be set;
leave them empty and the operator skips exposure entirely (Tier 1 behavior).

From here the operator automates everything per tenant: TLSRoute +
ReferenceGrant, vCluster cert SANs, kubeconfig server URL, and
`status.apiServerUrl` on the Tenant.

## 5. Verify

```bash
kubectl apply -f - <<'EOF'
apiVersion: kubespaces.io/v1alpha1
kind: Tenant
metadata: {name: smoke}
spec: {owner: you@example.com}
EOF
kubectl wait tenant smoke --for=jsonpath='{.status.phase}'=Ready --timeout=5m
kubectl get secret vc-smoke -n kubespaces-tenant-smoke \
  -o jsonpath='{.data.config}' | base64 -d > /tmp/smoke.kubeconfig
kubectl --kubeconfig /tmp/smoke.kubeconfig get nodes   # over the public endpoint
kubectl delete tenant smoke
```

## GitOps

All of the above is declarative and belongs in your platform GitOps
repository (Flux/Argo) rather than in shell history — the pre-OSS KubeSpaces
ran exactly this layout (gateway-api → implementation → gateways → DNS) as
Flux Kustomizations. A reference layout ships in
[`examples/host/`](../examples/host/); reconcile that directory and the host
is ready.

## Related

- The **apps** gateway (wildcard TLS termination + cert-manager, for tenant
  workloads and the portal) is the second half of the networking story —
  tracked in [#16](https://github.com/kubespaces-io/kubespaces/issues/16)
  and [#17](https://github.com/kubespaces-io/kubespaces/issues/17).
- [prerequisites.md](prerequisites.md) — the tier model and why the API path
  needs no certificates.

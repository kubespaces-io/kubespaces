# What you need to run KubeSpaces

Two tiers. Be explicit about this in every doc and the README — "works on kind
in 10 minutes" and "production needs a domain" are both true and must not be
conflated.

## Tier 1 — Evaluate / demo (kind, any throwaway cluster)

**No domain, no load balancer, no public IP.**

- `helm install kubespaces` → port-forward the portal.
- Tenant API access via `kubectl port-forward` to the vCluster service
  (kubeconfig from the Secret, server rewritten to localhost).
- TLS: vCluster's own self-signed certs. Keycloak in dev mode.

This tier must always keep working — it is the 10-minute demo that carries the
launch.

## Tier 2 — Real deployment (tenants reachable from outside)

| Requirement | Why | Notes |
|-------------|-----|-------|
| A domain you control | `{tenant}.api.{domain}`, `*.apps.{domain}` | Any registrar |
| DNS zone with **API access** (Cloud DNS, Route53, Azure DNS, Cloudflare…) | cert-manager **DNS-01** for the wildcard apps cert; external-dns for records | API creds are needed for the wildcard; HTTP-01 cannot issue wildcards |
| `LoadBalancer` Service support with a reachable IP | The Envoy Gateway listeners | Cloud: built-in. On-prem: MetalLB/kube-vip. "Public" strictly required only for public reach — a private LB + internal DNS works for internal platforms |
| Gateway API implementation (Envoy Gateway) + cert-manager (+ external-dns) | API gateway (TLS passthrough → per-tenant TLSRoute), apps gateway (wildcard TLS → HTTPRoutes) | Installed alongside KubeSpaces; not bundled in the umbrella chart (yet) |

Nuances worth knowing:

- **The tenant API endpoint needs no cert-manager at all** — the API gateway is
  TLS **passthrough**; clients terminate against vCluster's own serving certs.
  Only DNS is needed for `{tenant}.api.{domain}`. The wildcard cert story is
  exclusively about the **apps** gateway (and the portal riding on it).
- **One IP can serve both** gateways/listeners; two IPs (api/apps split, as in
  the original kubespaces-infra setup) is cleaner but not required.
- No domain but want external reach for a PoC? `sslip.io`/`nip.io` hostnames
  pointed at the LB IP work for routing (with self-signed/no TLS for apps);
  fine for demos, not for production.

## What the operator automates in Tier 2 (D17)

Per tenant: `TLSRoute` (SNI `{tenant}.api.{domain}`) + `ReferenceGrant` on the
API gateway, `status.apiServerUrl` published on the Tenant CR, and the served
kubeconfig pointing at the public URL. Nothing manual per tenant.

**The host cluster itself must be prepared once** (Gateway API implementation
with TLSRoute support, the shared passthrough Gateway, DNS) — step-by-step in
[host-cluster.md](host-cluster.md).

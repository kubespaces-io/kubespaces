# Exposing apps

You are a tenant user. You have a workload inside your tenant and you want a
public HTTPS URL. This takes one `HTTPRoute` — everything else already
happened when your tenant was created.

*(Platform not set up for app exposure yet? That's the admin-side
[Host cluster preparation](../host-cluster.md).)*

## Your namespace under the sun

Every tenant owns a wildcard: **`*.<tenant>.apps.<domain>`** — check yours
with:

```bash
kubectl get tenant <name> -o jsonpath='{.status.appsDomain}'   # host cluster
# or: kubespaces tenant get <name>
```

Any hostname under it is yours; hostnames outside it are structurally
unreachable from your tenant (see
[how isolation works](../concepts/networking.md)).

## Expose a deployment

Inside your tenant (your kubeconfig, your `default` namespace):

```bash
kubectl create deployment web --image=nginx:alpine --port 80
kubectl expose deployment web --port 80
```

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: web
spec:
  parentRefs:
    - name: kubespaces-apps        # the platform Gateway, projected into
                                   # your default namespace — no host
                                   # namespaces to know about
  hostnames:
    - web.team-atlas.apps.example.com
  rules:
    - backendRefs:
        - name: web
          port: 80
```

```bash
kubectl apply -f route.yaml
curl https://web.team-atlas.apps.example.com     # 🎉
```

TLS is already handled: the platform issued a certificate for your wildcard
when the tenant was created and terminates HTTPS at the shared gateway.

## Rules of the road

- **Routes go in your `default` namespace** — the gateway is projected
  there, and the route sync authorizes routes that sit next to a Gateway
  they can see.
- **Hostnames must be under your wildcard.** A route claiming anything else
  attaches to no listener (`NoMatchingListenerHostname` in the route
  status) — it does not error loudly, it simply never receives traffic.
- **Standard HTTPRoute features work**: path matching, header matching,
  weighted backends, multiple hostnames (all under your wildcard). The
  route is plain Gateway API — nothing KubeSpaces-specific to learn.

## Debugging

```bash
# route accepted? (inside your tenant)
kubectl get httproute web -o jsonpath='{.status.parents[0].conditions}'

# common reasons
# - RefNotPermitted: route is not in the default namespace
# - NoMatchingListenerHostname: hostname not under your *.{tenant}.apps wildcard
# - backend 503: service/port name mismatch, or pods not ready
```

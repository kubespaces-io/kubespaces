# Your first tenant

Three doors into the same room: the portal, the CLI and `kubectl` all create
the same `Tenant` custom resource, and the operator does the same work
regardless of which one you used.

=== "CLI"

    ```bash
    kubespaces login --server https://kubespaces.example.com
    # opens the OIDC device flow in your browser

    kubespaces tenant create team-atlas --cpu 8 --memory 16Gi
    kubespaces tenant get team-atlas --wait
    # NAME         PHASE   API
    # team-atlas   Ready   https://team-atlas.api.example.com:443

    kubespaces tenant kubeconfig team-atlas --merge
    kubectl config use-context team-atlas
    kubectl get nodes
    ```

=== "Portal"

    Log in, hit **New tenant**, pick a name and quota, watch the status go
    `Pending → Provisioning → Ready`, then download the kubeconfig from the
    tenant page.

=== "kubectl / GitOps"

    ```yaml
    apiVersion: kubespaces.io/v1alpha1
    kind: Tenant
    metadata:
      name: team-atlas
    spec:
      owner: alice@example.com
      resources:
        cpu: "8"
        memory: 16Gi
        storage: 50Gi
    ```

    Apply it, commit it to your Flux/Argo repository, template it per team —
    the CR is the source of truth, so GitOps is not a workaround, it is the
    design. Field reference: [Tenant CRD](../reference/tenant-crd.md).

## What just happened

For every Tenant the operator created, on the host cluster:

- a **namespace** (`kubespaces-tenant-<name>`) labeled with the tenant name
- a **ResourceQuota** from `spec.resources` and a **LimitRange** with sane
  container defaults (so pods without explicit limits still schedule)
- a **vCluster** — the tenant's own control plane — installed from the
  pinned KubeSpaces chart mirror
- with exposure configured: a **TLSRoute** for the API server, an **apps
  listener + certificate**, and public URLs in the Tenant status

Inside the tenant you are cluster-admin. On the host you own exactly one
namespace, capped by the quota — that asymmetry is the entire point.

## Day-2

```bash
kubespaces tenant list                 # everything you own
kubectl get tenant team-atlas -o yaml  # status: phase, endpoints, conditions
kubespaces tenant delete team-atlas    # full teardown via finalizer
```

Deletion removes the vCluster, routes, certificates and namespace — nothing
orphaned, verified by the E2E suite.

## Next

- [Exposing apps](../guides/expose-apps.md) — from `kubectl apply` inside
  your tenant to a public HTTPS URL
- [Tenant lifecycle](../concepts/tenants.md) — phases, conditions and what
  the operator reconciles

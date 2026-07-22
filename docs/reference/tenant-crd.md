# Tenant CRD

`tenants.kubespaces.io/v1alpha1` — cluster-scoped, short name `ten`.
Generated from
[`operator/api/v1alpha1/tenant_types.go`](https://github.com/kubespaces-io/kubespaces/blob/main/operator/api/v1alpha1/tenant_types.go),
which is the source of truth.

!!! note "API stability"
    `v1alpha1` may change between 0.x minors. Graduation to `v1beta1` → `v1`
    with conversion and a deprecation policy is a 1.0 gate — see the
    [roadmap](https://github.com/kubespaces-io/kubespaces/blob/main/ROADMAP.md).

## Example, fully loaded

```yaml
apiVersion: kubespaces.io/v1alpha1
kind: Tenant
metadata:
  name: team-atlas            # DNS-1123, max 40 chars — becomes part of
                              # namespace names and public hostnames
spec:
  displayName: "Team Atlas"
  owner: alice@example.com
  targetNamespace: ""         # default: kubespaces-tenant-team-atlas
  resources:
    cpu: "8"
    memory: 16Gi
    storage: 50Gi
  vcluster:
    version: ""               # vCluster chart version (default: pinned mirror version)
    kubernetesVersion: v1.32.1
    valuesOverrides:          # raw vCluster chart values — admin-grade escape hatch
      sync:
        toHost:
          ingresses:
            enabled: true
```

## Spec

| Field | Type | Required | Notes |
|---|---|---|---|
| `displayName` | string | no | Human-friendly name for the portal |
| `owner` | string | **yes** | OIDC subject or email. Authorization is enforced by the API; the operator treats it as metadata |
| `targetNamespace` | string | no | Host namespace for the vCluster; defaults to `kubespaces-tenant-<name>` |
| `resources.cpu` | quantity | no | Becomes `requests.cpu` **and** `limits.cpu` in the ResourceQuota |
| `resources.memory` | quantity | no | Same pattern for memory |
| `resources.storage` | quantity | no | Becomes `requests.storage` |
| `vcluster.version` | string | no | vCluster chart version; empty = the operator's pinned default |
| `vcluster.kubernetesVersion` | string | no | Kubernetes version inside the virtual cluster |
| `vcluster.valuesOverrides` | object | no | Raw vCluster values merged over operator defaults (overrides win) |

Setting any of `resources.*` also makes the operator pair the quota with a
LimitRange (container defaults 1 CPU / 1Gi limit, 100m / 128Mi request) so
limit-less pods still schedule.

## Status

| Field | Meaning |
|---|---|
| `phase` | `Pending` · `Provisioning` · `Ready` · `Deleting` · `Failed` |
| `message` | Human-readable explanation of the current phase |
| `conditions` | Standard conditions; type `Ready` with reasons like `Provisioned`, `QuotaFailed`, `NetworkingFailed` |
| `kubeconfigSecretRef` | `{name: vc-<tenant>, key: config}` in the target namespace |
| `apiServerUrl` | Public API endpoint (`https://<tenant>.api.<domain>:443`) when API exposure is configured |
| `appsDomain` | The tenant's app wildcard (`*.<tenant>.apps.<domain>`) when app exposure is configured |
| `observedGeneration` | Last spec generation the operator acted on |

## Printer columns

```console
$ kubectl get tenants
NAME         OWNER               PHASE   API                                          AGE
team-atlas   alice@example.com   Ready   https://team-atlas.api.example.com:443       5m
```

## Lifecycle contracts

- Finalizer `kubespaces.io/finalizer` guarantees teardown ordering — see
  [Tenant lifecycle](../concepts/tenants.md).
- The kubeconfig Secret (`vc-<name>`, key `config`) is written by vCluster;
  with API exposure configured, its server URL is already the public
  endpoint.

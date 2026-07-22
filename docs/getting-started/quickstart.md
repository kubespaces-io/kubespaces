# Quickstart

A complete KubeSpaces on a throwaway [kind](https://kind.sigs.k8s.io/)
cluster in about ten minutes. No domain, no load balancer, no cloud account —
this is the [Tier 1 evaluation path](../prerequisites.md); tenants are
reached by port-forward. For public endpoints, continue to
[Host cluster preparation](../host-cluster.md) afterwards.

## 1. A cluster

```bash
kind create cluster --name kubespaces
```

Any Kubernetes ≥ 1.31 works — kind, minikube, k3d, or a spare real cluster.

## 2. Install KubeSpaces

```bash
git clone https://github.com/kubespaces-io/kubespaces
cd kubespaces
helm install kubespaces charts/kubespaces \
  --namespace kubespaces --create-namespace \
  --set operator.enabled=true
```

The default install is fully self-contained: the API, the web portal,
PostgreSQL (official image), Keycloak (official image, dev mode, realm
auto-imported) and the Tenant operator. Wait for everything to come up:

```bash
kubectl get pods -n kubespaces --watch
```

## 3. Create a tenant

The declarative way (the portal and CLI produce exactly this object):

```bash
kubectl apply -f - <<'EOF'
apiVersion: kubespaces.io/v1alpha1
kind: Tenant
metadata:
  name: demo
spec:
  owner: you@example.com
  resources:
    cpu: "4"
    memory: 8Gi
EOF

kubectl get tenants --watch
# NAME   OWNER             PHASE
# demo   you@example.com   Pending → Provisioning → Ready
```

`Ready` means the virtual cluster is up and its kubeconfig exists.

## 4. Use it

```bash
kubectl get secret vc-demo -n kubespaces-tenant-demo \
  -o jsonpath='{.data.config}' | base64 -d > demo.kubeconfig

# Tier 1: reach the API server through a port-forward
kubectl port-forward -n kubespaces-tenant-demo svc/demo 8443:443 &
kubectl --kubeconfig demo.kubeconfig --server https://localhost:8443 \
  --insecure-skip-tls-verify get namespaces
```

You are now cluster-admin *inside* the tenant: create namespaces, install
CRDs, break things — the host cluster and other tenants never notice. The
quota you set caps the tenant's total footprint on the host.

## 5. The portal

```bash
kubectl port-forward -n kubespaces svc/kubespaces-frontend 3000:80
```

Open <http://localhost:3000> and log in with the demo Keycloak credentials
printed by `helm install`'s notes (`helm get notes kubespaces -n kubespaces`).

## 6. Clean up

```bash
kubectl delete tenant demo   # finalizer tears down vCluster + namespace
kind delete cluster --name kubespaces
```

## Where next

- Give tenants **public API endpoints and app URLs** —
  [Host cluster preparation](../host-cluster.md)
- Install the **CLI** with device-flow login — [Using the CLI](../guides/cli.md)
- Understand what just happened — [Architecture](../concepts/architecture.md)

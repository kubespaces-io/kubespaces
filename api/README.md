# KubeSpaces API

Backend HTTP API for KubeSpaces. Authenticates users via OIDC (Keycloak),
persists tenant metadata and an audit log in Postgres, and creates/deletes
`Tenant` custom resources (`kubespaces.io/v1alpha1`, cluster-scoped). It never
provisions vClusters itself — the operator does; tenant status is read live
from the CR.

See `docs/contracts.md` at the repo root for the full API contract.

## Run locally

Requires a kubeconfig pointing at a cluster with the Tenant CRD installed,
plus reachable Postgres and Keycloak (port-forward both from the cluster):

```sh
kubectl port-forward svc/postgres 5432:5432 &
kubectl port-forward svc/keycloak 8081:8080 &

export KUBESPACES_DB_HOST=localhost
export KUBESPACES_DB_PORT=5432
export KUBESPACES_DB_NAME=kubespaces
export KUBESPACES_DB_USER=kubespaces
export KUBESPACES_DB_PASSWORD=changeme
export KUBESPACES_OIDC_ISSUER_URL=http://localhost:8081/realms/kubespaces
export KUBESPACES_OIDC_CLIENT_ID=kubespaces

go run ./cmd/api
```

Migrations are embedded and run automatically at startup. Kubernetes access
uses in-cluster config when deployed, falling back to `~/.kube/config` locally.

## Environment variables

| Var | Required | Default | Meaning |
|-----|----------|---------|---------|
| KUBESPACES_DB_HOST | yes | – | Postgres host |
| KUBESPACES_DB_PORT | no | 5432 | Postgres port |
| KUBESPACES_DB_NAME | yes | – | Database name |
| KUBESPACES_DB_USER | yes | – | Database user |
| KUBESPACES_DB_PASSWORD | yes | – | Database password |
| KUBESPACES_OIDC_ISSUER_URL | yes | – | OIDC issuer URL (discovery) |
| KUBESPACES_OIDC_CLIENT_ID | yes | – | Expected audience/azp (`kubespaces`) |
| KUBESPACES_LISTEN_ADDR | no | :8080 | HTTP listen address |

## Test

```sh
go test ./...
```

The store integration test is skipped unless `KUBESPACES_TEST_DB_DSN` is set
(pgx keyword/value DSN pointing at a throwaway database).

## Build image

```sh
docker build -t ghcr.io/kubespaces-io/api .
```

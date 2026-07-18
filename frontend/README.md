# KubeSpaces Frontend

Self-service portal for [KubeSpaces](../README.md) — the open control plane
for virtual Kubernetes tenants. Sign in with Keycloak, create tenants, watch
them provision, download a kubeconfig when they turn Ready.

Stack: Next.js 15 (app router) · TypeScript · Tailwind CSS v4 ·
Auth.js (next-auth v5) with the Keycloak provider (public client, PKCE).

## Run locally

Prerequisites: Node 22, a reachable Keycloak realm and KubeSpaces API —
typically port-forwarded from a dev cluster:

```sh
# Keycloak on :8081
kubectl -n keycloak port-forward svc/keycloak 8081:8080
# KubeSpaces API on :8080
kubectl -n kubespaces port-forward svc/kubespaces-api 8080:8080
```

Then:

```sh
cp .env.example .env.local
# fill in AUTH_SECRET:  openssl rand -base64 32
npm install
npm run dev
```

Open <http://localhost:3000> and sign in.

### Keycloak client requirements

The `kubespaces` client in the realm must be:

- **public** (no client secret), with **PKCE (S256)** enabled
- redirect URI: `http://localhost:3000/api/auth/callback/keycloak`
  (or `${AUTH_URL}/api/auth/callback/keycloak` in other environments)
- web origin: `http://localhost:3000`

### Environment

| Var | Meaning |
|-----|---------|
| `AUTH_KEYCLOAK_ISSUER` | Realm issuer URL, e.g. `http://localhost:8081/realms/kubespaces` |
| `AUTH_KEYCLOAK_ID` | Client id, `kubespaces` |
| `AUTH_SECRET` | Cookie/JWT encryption secret (`openssl rand -base64 32`) |
| `AUTH_URL` | Canonical URL of this app, e.g. `http://localhost:3000` |
| `KUBESPACES_API_URL` | Backend base URL, default `http://localhost:8080` |

## How API access works

The browser never sees a Keycloak token. Client code calls same-origin
`/api/v1/...`; a route handler (`src/app/api/v1/[...path]/route.ts`) decodes
the encrypted Auth.js session cookie server-side, attaches the access token
as `Authorization: Bearer …`, refreshes it against Keycloak when expired,
and proxies to `KUBESPACES_API_URL`.

## Scripts

```sh
npm run dev     # dev server
npm run build   # production build (standalone output)
npm run start   # serve the production build
npm run lint    # eslint
```

## Docker

```sh
docker build -t ghcr.io/kubespaces-io/frontend:dev .
docker run --rm -p 3000:3000 \
  -e AUTH_KEYCLOAK_ISSUER=... -e AUTH_KEYCLOAK_ID=kubespaces \
  -e AUTH_SECRET=... -e AUTH_URL=http://localhost:3000 \
  -e KUBESPACES_API_URL=http://host.docker.internal:8080 \
  ghcr.io/kubespaces-io/frontend:dev
```

## Layout

```
src/
├── app/                  # routes: /, /tenants, /tenants/[name], API proxy
├── components/
│   ├── layout/           # SiteHeader
│   ├── tenants/          # TenantList, TenantDetail, dialogs, PhaseBadge
│   └── ui/               # Button, Badge, Dialog, Field
├── hooks/                # useTenants, useTenant (5s polling), useFocusTrap
├── lib/                  # api client, keycloak refresh, types, validation
├── auth.ts               # Auth.js config (Keycloak public client + PKCE)
└── middleware.ts         # protects /tenants*
```

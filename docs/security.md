# Security architecture & roadmap

How KubeSpaces approaches security today, and what's planned before 1.0.
For reporting vulnerabilities see [SECURITY.md](../SECURITY.md).

## Trust model

- **Tenants are untrusted.** A tenant gets a full virtual cluster; everything
  it can do against the host must flow through vCluster's syncer plus the
  namespace boundary (ResourceQuota, LimitRange — and NetworkPolicy/PSS on the
  roadmap). Tenant isolation escapes are our highest-severity bug class.
- **The API is untrusted until proven otherwise.** Every request is
  authenticated via OIDC (JWT verified against the issuer; audience/azp
  checked). Members only see their own tenants; name enumeration is prevented
  (404, not 403, for foreign tenants).
- **Components hold least privilege.** The API's RBAC is limited to Tenant
  CRs + reading kubeconfig Secrets. Only the operator holds provisioning
  rights. (Tightening the operator's broad role to the generated
  controller-gen role is on the roadmap.)

## Current guarantees

| Area | Status |
|------|--------|
| Images | Distroless/static (Go) and alpine-nonroot (frontend), non-root UIDs, CGO disabled |
| Auth | OIDC only; PKCE public client; no passwords or PATs in the platform; frontend keeps tokens server-side |
| Secrets | Generated at install (stable across upgrades) or bring-your-own via `existingSecret`; never in values files or git |
| Supply chain | vCluster chart pinned + mirrored to our registry (D16); deps resolved via go.sum / package-lock |
| Repo hygiene | gitleaks-clean history; secret scanning + push protection; Dependabot; CodeQL on api/operator/spacectl/frontend |
| Releases | Built only by CI from tags; SBOMs (syft) + keyless signatures (cosign) on artifacts and images; checksums on every release |
| Tenant limits | ResourceQuota + LimitRange stamped per tenant by the operator |

## Verifying releases

```bash
# spacectl checksums signature (keyless, GitHub OIDC identity)
cosign verify-blob \
  --certificate-identity-regexp 'github.com/kubespaces-io/kubespaces' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --signature checksums.txt.sig --certificate checksums.txt.pem checksums.txt

# container images
cosign verify ghcr.io/kubespaces-io/api:<tag> \
  --certificate-identity-regexp 'github.com/kubespaces-io/kubespaces' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Roadmap to 1.0 (holistic hardening)

- [ ] **Threat model** document (STRIDE over the Pattern-B architecture)
- [ ] Operator RBAC tightened from cluster-admin-equivalent to the generated role
- [ ] **NetworkPolicy** per tenant namespace (default-deny egress to host services)
- [ ] Pod Security Standards (restricted) on tenant namespaces
- [ ] `valuesOverrides` policy guard (deny privileged vCluster options unless allowed by admin)
- [ ] SLSA provenance attestations on images (`actions/attest-build-provenance`)
- [ ] Pin GitHub Actions by commit SHA
- [ ] Rate limiting on the API; audit log surfacing in the portal
- [ ] External penetration test before 1.0 (tenant-escape focused)
- [ ] Keycloak production-mode guidance (the bundled dev-mode instance is for evaluation only — bring your own IdP in production)

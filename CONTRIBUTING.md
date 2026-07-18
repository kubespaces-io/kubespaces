# Contributing to KubeSpaces

Thanks for considering a contribution — issues, docs, and code are all welcome.
This document is short on ceremony and explicit about expectations.

## Ground rules

- Be respectful. We follow the [Contributor Covenant](CODE_OF_CONDUCT.md).
- **Security issues never go in public issues** — see [SECURITY.md](SECURITY.md).
- Before large changes, open an issue first and agree on the direction. PRs
  that land unannounced architecture rewrites will be closed with a pointer
  to this line.

## Getting started

1. Read [README.md](README.md) for the repository layout and
   [docs/contracts.md](docs/contracts.md) for how components talk to each
   other. The contract doc is the source of truth — changes that touch it
   affect every component and need an issue first.
2. Each component builds independently:
   - `api/`, `operator/`, `spacectl/`: `go build ./... && go test ./...`
   - `frontend/`: `npm ci && npm run lint && npm run build`
   - chart: `make lint template`
3. A full local environment is one `make kind-up kind-install` away.

## Pull requests

- **Conventional commits** (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`,
  `test:`, `ci:`, `perf:`). The changelog and release notes are generated
  from them, so the type you pick is user-visible.
- Keep PRs focused: one logical change, tests included, CI green. The CI
  checks (build/vet/test per component, chart lint, generated-manifest drift
  for the operator) are the merge bar.
- Update docs in the same PR as behavior changes.
- New Go code follows the shape around it: small files, early returns,
  explicit error handling, no naked globals.

## AI-assisted contributions

This project was bootstrapped with heavy AI assistance, and we're not going
to pretend otherwise — AI tooling is a normal part of the workflow here.
That said, the following policy applies to every contribution, however it
was produced:

1. **You are the author.** Submitting a change means you understand every
   line of it, have run it, and can defend it in review. "The model wrote
   it" is never an answer to a review question.
2. **Verified, not just generated.** AI-generated code must be built,
   tested, and exercised against a real deployment before it lands in a PR —
   the same bar as hand-written code.
3. **Disclose substantial generation.** If a PR is largely AI-generated,
   note it in the description. It's not a mark against the PR; it tells
   reviewers where to look harder (subtle API misuse, invented functions,
   stale idioms).
4. **No slop.** Unreviewed AI dumps, bulk speculative refactors, low-effort
   AI-written issues, and AI-"discovered" vulnerabilities that haven't been
   reproduced by a human waste maintainer time and will be closed without
   detailed feedback. Repeated slop leads to a ban. AI-generated security
   reports **must** include a human-verified reproduction — hallucinated
   vulnerability reports are treated as bad-faith.
5. **Licensing.** You are responsible for ensuring anything you submit is
   yours to license under Apache 2.0 — this includes AI output trained
   reproductions of incompatible code.

## Issues

- Use the issue templates. For bugs: versions (KubeSpaces, host Kubernetes,
  vCluster), reproduction steps, and relevant operator/API logs.
- `good first issue` labels are curated and genuinely scoped for newcomers.

## Releases

Maintainers cut releases per [RELEASING.md](RELEASING.md). Contributors don't
need to touch versions, tags, or the changelog header — automation and
maintainers handle both.

## License

By contributing you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE).

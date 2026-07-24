# AI Policy

KubeSpaces is built with AI in the loop, and we don't hide it. This document
states two things plainly: **how the project uses AI**, and **how we accept
AI-assisted contributions**. It expands on the short
[AI-assisted contributions](CONTRIBUTING.md#ai-assisted-contributions) section
in `CONTRIBUTING.md` — that section is the summary, this is the full policy.

Our contribution rules are modeled on the
[Linux Foundation Generative AI Policy](https://www.linuxfoundation.org/legal/generative-ai)
and align with how CNCF projects handle the same question. Where the LF policy
assumes a DCO, we adapt it to KubeSpaces' licensing model: **inbound = outbound**
under the [Apache License 2.0](LICENSE) (by contributing, you license your
contribution under Apache 2.0 — see [CONTRIBUTING.md](CONTRIBUTING.md#license)).

---

## Part 1 — How we use AI

Transparency first: this codebase was bootstrapped with heavy AI assistance,
and AI tooling remains a normal part of day-to-day work here. That means:

- **AI is a tool, humans are accountable.** Every line that lands in `main` was
  reviewed, understood, and is owned by a human maintainer. "The model wrote it"
  is never a defense for a merged change, a released binary, or a design call.
- **AI assists authorship, review, and triage** — drafting code, tests, and
  docs; summarizing diffs; and helping label and route issues. It does **not**
  make merge decisions, cut releases, or approve its own output. Authoring and
  reviewing stay separate passes with a human on the reviewing side.
- **Generated code meets the same bar as hand-written code.** It is built,
  tested, and exercised against a real deployment before it ships. Our CI gates
  (per-component build/vet/test, chart lint, generated-manifest drift, and the
  end-to-end tenant-lifecycle acceptance run) apply identically regardless of
  how a change was produced.
- **We don't ship AI slop.** Speculative refactors, invented APIs, stale idioms,
  and confidently-wrong output are caught in review or they don't merge.

If you're evaluating KubeSpaces for production: the provenance of a change never
lowers its quality bar. AI-assisted or not, everything crosses the same gates.

---

## Part 2 — Accepting AI-assisted contributions

AI-assisted contributions are **welcome**. Using an AI tool to help write a
patch is not a strike against it. What matters is that the result is correct,
that you stand behind it, and that it's clean to license. Every contribution —
however it was produced — must satisfy the three conditions below.

### 1. You are the author, and you certify the contribution

By opening a pull request you certify, under KubeSpaces' inbound = outbound
model, that:

- you have the right to submit the contribution under the Apache License 2.0;
- you understand every line of it, have run it, and can defend it in review; and
- it is either your original work or is otherwise properly licensed and
  attributed (see condition 3).

This is the same certification any contributor makes — AI involvement does not
change it, and it cannot be delegated to a model. "The model wrote it" is never
an answer to a review question.

### 2. The AI tool's terms must be compatible with Apache 2.0

You are responsible for ensuring that the terms and conditions of any
generative AI tool you used **do not impose restrictions on the output** that
are inconsistent with the Apache License 2.0 or with this project's inbound =
outbound licensing. If a tool claims rights over its output, or restricts how
that output may be used, redistributed, or relicensed in a way that conflicts
with Apache 2.0, its output cannot be contributed here.

### 3. Pre-existing third-party material must be properly licensed, attributed, and disclosed

If an AI tool's output includes any pre-existing copyrighted material — for
example, verbatim or near-verbatim reproduction of existing open source code —
you must ensure you have the right to contribute it under Apache 2.0, comply
with the applicable license terms (including attribution and license
compatibility), and **disclose it** in the pull request. Apache 2.0 is
incompatible with several common licenses (e.g. GPL/AGPL); output that
reproduces such code cannot be accepted.

### Disclosure

If a pull request is **largely AI-generated**, note it in the description. This
is not held against the PR — it tells reviewers where to look harder (subtle API
misuse, invented functions, stale idioms, or reproduced third-party code). Small
AI-assisted edits do not need a disclosure; substantial generation does.

### Verification — no slop

AI-generated code must be **verified, not just generated**: built, tested, and
exercised against a real deployment before it lands, to the same standard as
hand-written code. The following waste maintainer time and are closed without
detailed feedback:

- unreviewed AI dumps and bulk speculative refactors;
- low-effort AI-written issues; and
- AI-"discovered" vulnerabilities that have not been reproduced by a human.

Repeated slop leads to a ban.

### Security reports

AI-generated security reports **must** include a human-verified reproduction.
Hallucinated or unreproduced vulnerability reports are treated as bad-faith.
Report security issues privately — never in public issues — per
[SECURITY.md](SECURITY.md).

---

## Summary

| Question | Answer |
|----------|--------|
| Can I use AI to write a contribution? | Yes. |
| Do I have to disclose it? | Disclose substantial generation; small edits, no. |
| Who is responsible for the code? | You, the contributor — fully. |
| Does AI-generated code get an easier review? | No. Same bar, same CI gates. |
| What licenses can the output carry? | Apache-2.0-compatible only; reproduced third-party code must be licensed, attributed, and disclosed. |
| Does the project itself use AI? | Yes — for authoring, review, and triage, always with a human accountable. |

Questions about this policy? Open a discussion or an issue. See
[CONTRIBUTING.md](CONTRIBUTING.md) for the full contribution workflow.

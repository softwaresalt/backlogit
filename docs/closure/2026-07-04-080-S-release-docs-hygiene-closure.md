---
chunk_strategy: h1-h2-h3
description: 'Pre-merge operational closure for shipment 080-S — release pipeline and documentation hygiene. Consolidates CI status (4/4 green at code-review HEAD afab513: test 1.23, test 1.24, CLI Reference Drift, Docline frontmatter gate), Copilot review readiness (§1.9 PASS: fresh review covers HEAD, 16/16 files reviewed with zero comments and zero unresolved threads), runtime verification (PASS WITH FOLLOW-UP — release.yml guard statically verified via actionlint + YAML + both-branch logic walkthrough since the tag-triggered workflow cannot be exercised in-tree; characterization test executed green locally and in CI), invariants to preserve (secret never logged, guard is non-suppressing, SHA pins intact, characterization isolation, docs honesty), and the rollback path (git revert of the additive guard/test/docs commits). Deployment path is merge-only (CI-workflow YAML + a test artifact + docs; no service/migration/binary command). Readiness READY WITH CONDITIONS — operator merge approval + admin bypass of the PR-Review ruleset (sole author cannot self-approve), merge-commit strategy only (P-009). Not merged this run (P-014 / Principle VII). Follow-up: observe the guard on the next real tagged release; external npm-org/NPM_TOKEN provisioning (stash 34F11E5A) and .tmpl edits (EED25928) remain out of scope (Principle IV).'
doc_type: closure
docline:
    ms.date: 2026-07-04T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-04T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-04-080-S-release-docs-hygiene-closure.md
title: 080-S release pipeline & docs hygiene — Pre-Merge Operational Closure
---

# Operational Closure — 080-S release pipeline & docs hygiene

- **Date**: 2026-07-04
- **Mode**: `pre-merge`
- **Shipment**: `080-S` · Feature `080-F` · Tasks `080.001-T`, `080.002-T`, `080.003-T`
- **PR**: #174 — https://github.com/softwaresalt/backlogit/pull/174
- **Branch**: `feat/080-release-docs-hygiene` · code-review HEAD `afab513` (closure commit is docs-only, re-reviewed)
- **Verification report**: `docs/closure/2026-07-04-080-S-release-docs-hygiene-runtime-verification.md` (verdict **PASS WITH FOLLOW-UP**)
- **Readiness**: **READY WITH CONDITIONS** (operator merge approval + admin bypass; merge-commit strategy only)

## Change summary

Three mutually-independent, low-risk (P3-only) hygiene units from plan
`docs/exec-plans/2026-07-04-release-docs-hygiene-plan.md`:

- **Unit A (`ci`)** — guard the `npm-publish` job in `.github/workflows/release.yml` on
  `NPM_TOKEN` presence via an env-indirection preflight step emitting a boolean `has_token`
  output; both publish steps gated `if: steps.preflight.outputs.has_token == 'true'`. No red X
  when the token is intentionally absent. SHA pins, `contents: read`, `persist-credentials: false`,
  and `continue-on-error: true` preserved.
- **Unit B (`test`)** — characterization test pinning `scripts/package-npm.sh` output (6 valid,
  version-stamped `package.json`; wrapper `optionalDependencies` synced) run against an isolated
  copy. Shell script + thin Go wrapper (2-file stop rule). `scripts/package-npm.sh` unchanged.
- **Unit C (`docs`)** — correct misleading `make docs-lint --path` wording in the two planned
  files, distinguishing the repo-wide no-arg `make docs-lint` from scoped
  `go run ./cmd/backlogit docs lint --path <file>`.

## CI status

| Check | Conclusion (code-review HEAD `afab513`) |
|---|---|
| `test (1.23)` | success |
| `test (1.24)` | success |
| `CLI Reference Drift` | success |
| `Docline frontmatter gate` | success |

Whole-suite local gates: `go test -run=^$ ./...` (compile) ✅ · `go vet ./...` ✅ ·
`go test ./...` ✅ · `golangci-lint run` ✅ (0 findings) · `gofmt -l` = CRLF false-positives
only (new `.go` file LF-clean; CI-LF authoritative) · `actionlint` ✅ · `backlogit docs lint` ✅ (0 violations).

## Review status (§1.9 pre-merge readiness gate)

Evaluated at code-review HEAD `afab513`:

- **Check 1 — completion**: no pending Copilot review request (`reviewRequests.nodes == []`). ✅
- **Check 2 — freshness**: latest Copilot review (`2026-07-04T08:56:15Z`) `commit.oid == afab513 == headRefOid`. ✅
- **Check 3 — threads**: zero unresolved Copilot threads (`reviewThreads.nodes == []`). Copilot
  reviewed 16/16 changed files and generated no comments. ✅
- **Gate: PASS.**
- `reviewDecision`: `REVIEW_REQUIRED` — reflects the branch-protection **PR-Review ruleset**
  requiring an approving review that the sole author-identity cannot self-supply. This is the
  expected operator admin-bypass situation (same as 078-S / 079-S), not a Copilot-gate failure.

The docs-only closure commit (this artifact + the runtime-verification report + finalized ship
memory) is pushed after code review; a fresh Copilot re-review + a re-run of §1.9 on the closure
HEAD confirm readiness before merge presentation.

## Invariants to preserve

- **Secret never logged**: the preflight step emits only a boolean `has_token`; the `NPM_TOKEN`
  value is never echoed or interpolated into a log line.
- **Guard is non-suppressing**: the `if:` gate only *skips* publish when the token is absent;
  when present, both publish steps run exactly as before. No legitimate failure is masked.
- **Supply-chain pinning intact**: every third-party `uses:` remains full-SHA pinned; no pin
  downgraded to a mutable tag.
- **Least privilege preserved**: top-level `permissions: contents: read` and every checkout's
  `persist-credentials: false` are unchanged.
- **Characterization isolation**: the shell test copies into a `mktemp -d` workspace and never
  mutates tracked `npm/**/package.json`; `scripts/package-npm.sh` itself is unmodified.
- **Docs honesty / docline**: repo-wide `make docs-lint` (no args) is clearly distinguished from
  scoped `docs lint --path <file>`; the Docline frontmatter gate stays green.

## Pre-deploy audits

- No database migration, no persistence-schema change, no new Go CLI command, no service config.
  Additive workflow guard + a new test artifact + docs prose.
- `.github/workflows/release.yml`: additive-only diff; `actionlint` + YAML parse clean.
- Docs closure artifacts carry docline frontmatter → `Docline frontmatter gate` stays green.

## Deployment / rollout path

- **Merge-only.** This ships CI-workflow YAML + a characterization test + docs. There is no
  deployed service, canary, or maintenance window, and no binary command surface changed. The
  release-workflow guard takes effect on the **next `v*.*.*` tag push**.
- **Merge strategy: merge commit only (P-009).** Squash/rebase are disabled for this repo
  (verified: `allow_merge_commit=true`, `allow_squash_merge=false`, `allow_rebase_merge=false`).

## Post-merge checks

1. Confirm the merge commit is in `origin/main` history (`git merge-base --is-ancestor`).
2. Confirm the `Docline frontmatter gate` and `test` matrix are green on `main` post-merge.
3. On the next real tagged release, observe the `npm-publish` preflight: `has_token=false` cleanly
   skips publish (no red X) when `NPM_TOKEN` is absent; publish runs when it is present.

## Healthy signals

- CI on `main` stays green post-merge (test matrix + docline gate + CLI drift).
- On a real release tag: the preflight step logs a clear present/absent decision and the publish
  steps skip-or-run accordingly with no spurious red X.

## Failure signals

- `Docline frontmatter gate` fails on `main` (closure-doc frontmatter regressed).
- On a release tag: publish steps run when `NPM_TOKEN` is absent (guard inverted), or the
  preflight leaks the token value into logs, or a genuine publish failure is silently swallowed
  in a way not attributable to the pre-existing `continue-on-error`.

## Risky action record

None. All changes are additive: a non-suppressing CI guard, a new isolated characterization
test, and docs prose. No destructive, migration, secret-provisioning, or rollout-sensitive
action was taken. External npm-org/`NPM_TOKEN` provisioning was deliberately kept out of scope.

## Monitoring plan

- **Owner**: Ship operator (repo maintainer).
- **Signals**: post-merge CI on `main` (durable). The `Docline frontmatter gate` guards the
  closure docs; the `test` matrix guards the characterization test. The release-workflow guard
  has no runtime dashboard — its validation window is the next tagged release.

## Rollback trigger

- Post-merge CI on `main` fails on the docline gate or the test matrix, OR the first tagged
  release after merge shows the guard inverting publish behavior or leaking the token.

## Rollback procedure

- `git revert` the additive commits on a fresh branch and open a revert PR. Because the change is
  purely additive (workflow guard + test + docs), revert is low-risk: reverting Unit A restores
  the prior always-attempt-publish behavior (which red-X's on an absent token), reverting Unit B
  removes the characterization test, and reverting Unit C restores the prior docs wording. No
  data migration to unwind.

## Validation window

- Through the next Ship session / next shipment intake for CI health, plus the next real tagged
  release for the workflow-guard end-to-end confirmation.

## Readiness recommendation

**READY WITH CONDITIONS.** All quality gates, runtime verification (PASS WITH FOLLOW-UP), and the
§1.9 Copilot readiness gate pass at code-review HEAD `afab513`. Merge is gated on:

1. **Explicit operator merge approval** (P-014 / Constitution Principle VII — not merged this run).
2. **Admin bypass of the PR-Review ruleset** — the sole author-identity cannot self-supply the
   required approving review (same as 078-S / 079-S).
3. **Merge-commit strategy** (P-009) — squash/rebase disabled.

## Follow-up

- **Observe the guard on the next real tagged release** (workflow end-to-end confirmation).
- External npm-org / `NPM_TOKEN` provisioning (stash `34F11E5A`) → human-only, out of scope (Principle IV).
- External `.tmpl` parity edits (stash `EED25928`) → out of scope (out-of-tree; Principle IV).
- Optional test hardening (P3 advisory, deferred): wrap the shell-test exec in
  `exec.CommandContext` with a timeout in `tests/integration/package_npm_characterization_test.go`.
- Post-merge closure (Step 6) — shipment archival (`shipment ship 080-S --sha <merge>`) +
  knowledge graduation — runs on a dedicated `post-merge/080-S` branch only AFTER operator-approved merge.

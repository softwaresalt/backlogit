---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 065-S — docline frontmatter standardization (PR #136 + #137)'
doc_type: closure
docline:
    ms.date: 2026-06-26T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-26T07:06:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-26-065-S-docline-frontmatter-closure.md
title: 065-S Docline Frontmatter — Post-Merge Closure
---

# Operational Closure — Shipment 065-S

**Shipment**: 065-S — Standardize documentation frontmatter on docline base schema
**Feature**: 065-F (11 tasks: 065.001-T … 065.011-T)
**Merge SHAs**: `2a5df85b` (PR #136, run 1 — tooling stack) · `23a8b045` (PR #137, run 2 — bulk migration + CI gate)
**Recorded shipment merge SHA**: `23a8b045`
**Closure branch**: post-merge/065-docline-frontmatter
**Mode**: post-merge
**Verification report**: [065-S runtime verification](2026-06-26-065-S-docline-frontmatter-runtime-verification.md)
**Readiness**: **READY** (merge already completed; this is post-merge closure)

---

## Change Summary

Standardized repository documentation frontmatter on the **docline base schema**
(`schemas/docline/base-frontmatter-v1.schema.json`). Delivered across two merged
runs:

* a **body-preserving frontmatter codec** (CRLF and body bytes never mutated);
* a policy-as-code `BaseFrontmatter` model + validator;
* a doc **classifier** + **idempotent normalizer** (legacy keys folded under the
  `docline` namespace; move, never duplicate);
* a docline **application service** (`lint` / `plan` / `apply`);
* the `backlogit docs` **CLI** + **MCP parity tools** (`docs_lint` / `_migrate` / `_scope`);
* an **authoring guide** (`docs/docline-frontmatter-authoring-guide.md`) + an
  `ARCHITECTURE.md` section;
* **~213 in-scope docs migrated** (0 body-byte changes; idempotent);
* a **CI "Docline frontmatter gate"** (`make docs-lint`);
* a **gen-docs** update so generated `docs/cli-reference/**` are born-compliant.

Operator policy sign-off (recorded in `065.002-T` + the taxonomy decision doc):
**Q1** `ingested_at` = seed-once at migration; **Q2** `source` = repo-relative
POSIX path (full URIs allowed where the source is a known online source).

## CI & Review Status (at merge)

* PR #136 (run 1) merged → `2a5df85b`; PR #137 (run 2) merged → `23a8b045`.
* Both merge commits confirmed ancestors of `origin/main`.
* `23a8b045` confirmed merged via `gh pr view 137` (`state: MERGED`,
  `mergedAt: 2026-06-26T06:50:56Z`).

## Invariants to Preserve

1. Docline lint passes with zero violations on all in-scope docs.
2. Migration is idempotent — re-running yields **zero body-byte changes**.
3. The frontmatter codec never mutates Markdown body bytes.
4. Generated `docs/cli-reference/**` are born docline-compliant via gen-docs.
5. `ingested_at` is seeded once (not re-stamped on subsequent migrations).

## Pre-Deploy Audits

Not applicable — no migrations, feature flags, config, or access changes.
This is a documentation-tooling change with no runtime data path.

## Deployment / Rollout Path

**Merge-only.** No service deploy, no canary, no data migration. The change is
absorbed the moment PRs #136/#137 land on `main`; the CI gate then enforces the
contract on every subsequent PR.

## Post-Deploy Checks (executed)

See the runtime verification report. All PASS:

* `backlogit docs lint` → 0 violations on current `main`.
* `backlogit docs migrate` dry-run → 213 entries, **0 body-byte changes**;
  single-file apply on a compliant doc is a byte-identical no-op (idempotent).
* `backlogit docs classify` → correct doc_type derivation.
* `go test -count=1 ./internal/docline/... ./cmd/gen-docs/...` → PASS.

## Healthy Signals

* "Docline frontmatter gate" CI check green on PRs.
* `make docs-lint` exits 0 locally and in CI.
* Migrations report 0 body-byte changes (idempotent, no content drift).
* New/generated docs are born-compliant (no manual frontmatter fix-ups needed).

## Failure Signals

* Lint violations on newly added or edited in-scope docs (the gate fails the PR).
* Body-byte drift reported by `docs migrate` (codec regression — would indicate
  the body-preservation invariant was broken).
* `ingested_at` re-stamping on already-migrated docs (seed-once regression).

## Monitoring Plan

No runtime telemetry required (docs-tooling change). Ongoing enforcement is the
**CI "Docline frontmatter gate"** (`make docs-lint` → `backlogit docs lint`)
plus the `cli-reference-drift.yml` workflow that keeps generated docs in sync.
Local guardrail: `backlogit docs lint` before committing doc changes.

## Rollback Trigger

A confirmed regression in the codec (body-byte drift) or a false-positive lint
gate that blocks legitimate docs and cannot be hot-fixed forward.

## Rollback Procedure

Revert the merge commits in reverse order: `git revert -m 1 23a8b045` then, if
necessary, `git revert -m 1 2a5df85b`. Because the migration is idempotent and
made zero body-byte changes, reverting restores prior frontmatter without body
loss. The CI gate disappears with the revert of `23a8b045`.

## Risky Action Record

None. No destructive or high-blast-radius actions were taken during closure.
The only state mutations were backlog archival (shipment ship) and additive
documentation commits, both reversible.

## Source Artifact Cleanup

Step 6.7 inspected the shipped scope for `custom_fields.source_stash_id` and
`custom_fields.source_deliberation_id` on `065-F` and all 11 tasks. **None are
present** — there are no source stash or deliberation pointers to retire. The
docline deliberation/decision/plan docs remain in `docs/decisions/` and
`docs/exec-plans/` as durable, in-scope (docline-compliant) artifacts and are
intentionally retained.

* Stash entries removed: 0 (no `source_stash_id` recorded)
* Deliberations archived: 0 (no `source_deliberation_id` recorded)

RUN-1 follow-up stashes (`0615F487`, `B349CBED`, `A2436E1E`, `AE53BC5C`,
`E4B7767C`, `98C4F063`) and pre-existing stashes are **deferred to Stage** for
triage, consistent with Ship's role boundary (stash operations are out of
scope for Ship). `E4B7767C` + `98C4F063` (gen-docs / cli-reference born-compliance)
were functionally resolved via Option A in RUN 2; Stage should confirm and
archive them.

## Validation Window & Owner

* **Owner**: softwaresalt (repository maintainer).
* **Window**: none required beyond the standard PR CI cycle. The gate is
  self-enforcing on every future PR; no observation window applies to a
  docs-tooling change.

## Readiness Status

**READY** — the shipment is merged and operationally absorbed. The docline
contract is enforced in CI, verification passed, and the rollback path is clean.

## Follow-Ups

No new operational follow-ups were generated by this closure (verification passed
clean). Pre-existing RUN-1 follow-up stashes are deferred to Stage triage (see
Source Artifact Cleanup).

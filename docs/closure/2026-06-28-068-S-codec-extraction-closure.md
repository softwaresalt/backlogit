---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 068-S — shared frontmatter codec extraction into the stdlib-only leaf packages internal/mdfront and internal/atomicfile, breaking the docline<->core duplication and import cycle (PR #148, merge 7450271a). Behavior-preserving refactor; monitoring = existing CI guardrails.'
doc_type: closure
docline:
    ms.date: 2026-06-28T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-28T19:52:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-28-068-S-codec-extraction-closure.md
title: 068-S Shared Frontmatter Codec Extraction — Post-Merge Operational Closure
---

# Operational Closure — Shipment 068-S (Shared Frontmatter Codec Extraction)

- **Shipment**: 068-S — Shared frontmatter codec extraction
- **Feature**: 068-F (4 tasks: 068.001-T … 068.004-T, all done/archived)
- **PR**: #148 — *Shared frontmatter codec extraction into leaf packages*
- **Merge commit**: `7450271a3334fc9c780ba757de6cf390a40edf3c` (merge commit on `main`, P-009 compliant; squash/rebase disabled repo-wide)
- **Closure branch**: `post-merge/068-codec-extraction`
- **Mode**: post-merge
- **Verification**: `docs/closure/2026-06-28-068-S-codec-extraction-runtime-verification.md` — **PASS**
- **Readiness**: **READY** (the change is already merged; this artifact records monitoring + rollback for the shipped scope)

## Summary of the change

A pure, behavior-preserving refactor (Option B) that removes the
`internal/docline <-> internal/core` codec + atomic-write duplication introduced
by the 062-F import-cycle workaround:

- **068.001-T** — new `internal/mdfront`: stdlib-only body-preserving frontmatter codec (`Markdown` type with `Decode`/`Encode`), behavior-identical to the former `internal/docline/codec.go`. Imports `bytes`, `fmt`, `gopkg.in/yaml.v3` only.
- **068.002-T** — new `internal/atomicfile`: hardened `WriteFileAtomic` (temp + rename, clamped mode). Pure stdlib leaf.
- **068.003-T** — `internal/docline` migrated onto the leaf packages via a **true type alias**: `type Markdown = mdfront.Markdown`; `(*Markdown).Encode()` is inherited (Go forbids re-declaring a method on an aliased type), and only `Decode` is re-exported as a forwarding function. `WriteFileAtomic` forwards to `atomicfile`.
- **068.004-T** — `internal/core/doctor.go` archived_from repair migrated onto the leaf packages. The `docline -> core` codec duplication is deleted and the import cycle stays broken because both new packages are leaves.
- **Behavior preserved byte-identically**: public `docline` API + `cmd/gen-docs` output unchanged (CLI Reference Drift green); docline migration still idempotent + body-preserving; doctor `--check-archived-from` unchanged. Golden differential byte-equality tests lock the codec + `rewriteArchivedFromField`.
- **Plan**: `docs/exec-plans/2026-06-27-shared-frontmatter-codec-extraction-plan.md`. **Deliberation**: `docs/decisions/2026-06-27-shared-frontmatter-codec-extraction-deliberation.md`.

## Invariants to preserve

1. `internal/docline`'s public API is unchanged: `docline.Markdown` ≡ `mdfront.Markdown` (type alias), `Encode` inherited, `Decode`/`WriteFileAtomic` forward to the leaf packages.
2. `cmd/gen-docs` emits byte-identical output (no CLI Reference Drift).
3. The docline frontmatter migration stays idempotent and **body-preserving** (0 body-byte changes on any re-apply).
4. `doctor --check-archived-from` audit + the archive frontmatter-stamping path behave exactly as before the extraction.
5. `internal/mdfront` and `internal/atomicfile` remain **stdlib-only leaf packages** (no internal imports), so the `docline <-> core` import cycle cannot reappear.

## Pre-deploy audits

Not applicable as a service deploy — this is a library + CLI refactor that ships with the merge. No migrations, flags, config, or data changes were introduced (no on-disk format change; codec output is byte-identical).

## Deployment / rollout path

**Merge-only.** No service deploy, canary, or feature flag. The refactor takes effect for any binary built from `main` at or after `7450271a`. The repo-root `backlogit.exe` is already rebuilt from this commit.

## Post-deploy checks (performed at closure)

- Targeted package suites (`mdfront`, `atomicfile`, `docline`, `core`, `core/templates`) green — **confirmed** (runtime-verification E1).
- `gen-docs` regeneration produced **0** CLI-reference drift — **confirmed** (E2).
- `docs migrate` dry-run: 233 frontmatter normalizations, **0 body-byte changes** — **confirmed** (E3).
- `docs lint` valid, 0 violations (CI Docline gate baseline) — **confirmed** (E4).
- First `shipment ship` since the extraction (068-S itself) re-stamped 6 records with canonical `archived_from`; body bytes preserved; `doctor --check-archived-from` = 0 self-referential — **confirmed** (E5).

## Healthy signals

- `gen-docs` output stays byte-identical (CLI Reference Drift workflow green).
- `docs migrate` dry-run reports `body_bytes_changed: false` for every entry; docline migration is a byte-stable no-op on already-compliant files.
- `docs lint` reports 0 violations on every PR.
- `doctor --check-archived-from` reports 0 `archived_from_self_ref`.
- `go build ./...` / `go vet ./...` show no import cycle.

## Failure signals

- Any CLI Reference Drift (a non-empty `git diff docs/cli-reference/` after `gen-docs`).
- Any `body_bytes_changed: true` in a `docs migrate` plan, or a non-idempotent migration diff.
- A re-introduced `internal/docline <-> internal/core` (or `mdfront`/`atomicfile` upward) import cycle — `go build`/`go vet` failure.
- A new `archived_from_self_ref` from `doctor --check-archived-from`, or a body-byte change in a stamped/repaired archive record.

## Monitoring plan

This is a refactor with no behavior change and no runtime service, so "monitoring" = the existing CI guardrails plus an on-demand audit:

- **CI guardrails** (every PR):
  - `test (1.24)` — runs the `mdfront`/`atomicfile`/`docline`/`core` suites incl. the golden byte-equality and migration-idempotency tests.
  - `Docline frontmatter gate` (`make docs-lint`) — codec body-preservation contract on docs.
  - `CLI Reference Drift Check` (`go run ./cmd/gen-docs docs/cli-reference` + `git diff --exit-code`) — gen-docs byte-identity.
- **On-demand audit**: `backlogit doctor --check-archived-from` at each future shipment closure (Ship dogfooding check). Expect 0 self-referential.

## Rollback trigger

- A CI guardrail above goes red on `main` for a reason traceable to the codec/atomic-write extraction (byte drift in gen-docs, a body-byte change in a migrate plan, or a re-introduced import cycle), **or** `doctor --check-archived-from` reports a self-referential record introduced after `7450271a`.

## Rollback procedure

- Revert the merge commit: `git revert -m 1 7450271a3334fc9c780ba757de6cf390a40edf3c` and rebuild. Because the change is a pure code-location refactor with byte-identical codec output and no on-disk format change, reverting is behavior-exact and carries no data-migration reversal.

## Validation window & owner

- **Window**: through the next 1–2 shipment closures (each re-runs the doctor dogfooding check and the full CI guardrail set). No active service to watch.
- **Owner**: maintainer (softwaresalt) — the CI guardrails + doctor audit are the standing checks.

## Source artifact cleanup

- `068-F` carries no `custom_fields.source_stash_id` and no `custom_fields.source_deliberation_id` (custom_fields = `{harness_status: pending}` only). Per the Ship Step 6 protocol, **no heuristic search** was performed and no source stash/deliberation artifact was removed or archived.
  - Archived source artifacts: **none**.
  - Skipped (no covering custom field): source_stash_id, source_deliberation_id.
- For traceability only (referenced in the feature body / `references`, not acted on): source stash `8863C6C8` (medium) and the deliberation `docs/decisions/2026-06-27-shared-frontmatter-codec-extraction-deliberation.md`. Verified at closure: stash `8863C6C8` is **no longer in the active stash list** (already consumed/removed when it was harvested into 068-F), so there is nothing to retire. The deliberation doc is retained for traceability.

## Follow-ups

- **No follow-up items required a stash.** Source-artifact cleanup found no covering
  custom field on 068-F and the referenced source stash `8863C6C8` is already consumed
  (absent from the active stash list), so nothing was queued for Stage from this closure.
- **Malformed `archived_from: done` records** (`038-DL`, `039-DL`): pre-existing, flag-only by deliberate operator decision; doctor surfaces them every run. Not introduced by 068-S; disposition remains deferred (already tracked from prior closures — no new stash created).

---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 070-S — internal robustness cluster (PR #154, merge b4c317e). Three narrow hardening tasks: batched canonical-uniqueness scan via a new CanonicalCache with a zero-value-cache seeding guard (070.001-T, O(N^2)->O(N) for bulk create), *slog.Logger dependency injection into internal/db Rehydrate without slog.SetDefault mutation (070.002-T), and explicit empty-string-vs-absent-key distinction in docline ValidateFields minLength (070.003-T). Internal CLI/library only; monitoring = CI test + docline gate; rollback = revert b4c317e.'
doc_type: closure
docline:
    ms.date: 2026-06-29T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-29T21:51:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-29-070-S-internal-robustness-cluster-closure.md
title: 070-S Internal Robustness Cluster — Post-Merge Operational Closure
---

# Operational Closure — Shipment 070-S (Internal Robustness Cluster)

- **Shipment**: 070-S — internal robustness cluster (066-S / PR#132 / 069-S follow-ups)
- **Feature**: 070-F (3 tasks: 070.001-T, 070.002-T, 070.003-T — all done/archived)
- **PR**: #154 — *070-S: internal robustness cluster (create-scan batching, db-log DI, ValidateFields empty-vs-absent)*
- **Merge commit**: `b4c317e0f4ae1920553857ee0aba9d2f4c855131` (merge commit on `main`, P-009 compliant; squash/rebase disabled repo-wide). Required-review branch protection was satisfied by operator-authorized `--admin` (Copilot comments but never approves); no CI check or unresolved Copilot thread was bypassed.
- **Closure branch**: `post-merge/070-internal-robustness-cluster`
- **Mode**: post-merge
- **Verification**: `docs/closure/2026-06-29-070-S-internal-robustness-cluster-runtime-verification.md` — **PASS**
- **Readiness**: **READY** (already merged; this artifact records monitoring + rollback for the shipped scope)

## Summary of the change

Three independent, narrow hardening tasks shipped under one feature:

- **070.001-T** — Batch the canonical-uniqueness scan for bulk `CreateArtifact` callers. `scanCanonicalArtifacts` walked + parsed every queue/archive `.md` on each create, making bulk callers (migrate external-import loop, stash priority harvest) O(files × creates) → O(N²) on a large backlog. New `CanonicalCache` (refs map) scans once per batch via `NewCanonicalCache` + `WithCanonicalCache`, and records each freshly minted ID so within-batch collisions are still caught without a re-scan. A `scanCanonicalArtifactsFn` package var is the test seam (counts total scans). **Zero-value-cache seeding guard**: because `CanonicalCache` is exported, a caller in another package could pass `&core.CanonicalCache{}` (refs == nil); `ensureSeeded` lazily runs the one-time scan on first use so an unseeded cache cannot silently bypass the uniqueness guard (the issue Copilot flagged, now fixed). Single interactive creates pass no cache and scan per call, exactly as before. (commit `e4fefd3`)
- **070.002-T** — Dependency-inject `*slog.Logger` into `internal/db` Rehydrate / `warnOnDuplicateSourceIDs`. Variadic `RehydrateOption` + `WithLogger` + `newRehydrateConfig` (defaults to `slog.Default()`), preserving all ~70 existing 3-arg callers unchanged. Tests capture log output via the injected logger only — no `slog.SetDefault` mutation. `applyLogLevel`'s global `slog.SetDefault` wiring (root.go, CLI log-level config) intentionally kept, out of scope. (commit `60753ae`)
- **070.003-T** — Explicit empty-string-vs-absent-key distinction in docline `ValidateFields` minLength parity. `BaseFrontmatter` gains an unexported `present map[string]bool` populated by `FromMap` from the *source* frontmatter keys (captured BEFORE defaults are applied, so a defaulted `chunk_strategy`/`schema_version` is not mistaken for present). minLength loop: present+blank → `min_length` violation; absent optional key → no violation; presence unknown (struct literal, not via `FromMap`) → preserves historical whitespace-only behavior so direct callers are unaffected. Closes the 069-S advisory follow-up. (commit `1ab8f0d`)
- **Deliberation**: 049-DL (archived with the shipment).

## Invariants to preserve

1. Bulk create batches scan the canonical artifact set exactly once; single interactive creates scan per call. Within-batch ID collisions are still detected.
2. A zero-value / externally constructed `CanonicalCache` is seeded on first use and never bypasses the uniqueness guard (no duplicate IDs across queue + archive).
3. `internal/db` Rehydrate logs through its injected logger; existing 3-arg callers compile and behave unchanged; no `slog.SetDefault` side effect from the library path.
4. docline `ValidateFields` reports a `min_length` violation for a present-but-empty key, no violation for an absent optional key, and preserves whitespace-only behavior for struct-literal (non-`FromMap`) callers.
5. No new module dependencies; `go vet` + `golangci-lint` stay clean.

## Pre-deploy audits

- Merge-only change; no migrations, flags, config, or access changes. None required.

## Deployment / rollout path

- Merge-only. Already merged to `main`; `backlogit` rebuilds from `main`. No deploy step, no service surface.

## Post-deploy checks

- `go test ./internal/core/... ./internal/db/... ./internal/docline/...` green. ✅
- Bulk `add` ×N in a scratch workspace → unique canonical IDs; `sync` clean. ✅
- `backlogit sync` rehydrate emits logger output (INFO). ✅
- `backlogit docs lint` flags present-but-empty required keys, passes absent-optional keys. ✅

## Healthy signals

- Bulk import / harvest stays O(N) on create; no duplicate-ID collisions across batches.
- Rehydrate log output appears via the injected logger; no global logger mutation from the library.
- Valid docs pass `docs lint`; empty-but-required frontmatter values are rejected.

## Failure signals

- Duplicate canonical IDs created in a bulk batch (cache seeding regressed) or a quadratic slowdown returns on large-backlog imports.
- Rehydrate goes silent or a library call mutates the global default logger.
- `docs lint` newly accepts an empty required value or newly rejects an absent optional key.

## Monitoring plan

- Internal CLI/library — no external monitoring surface. Coverage = the CI gates run on every PR: `test (1.23)` / `test (1.24)`, `CLI Reference Drift`, `Docline frontmatter gate`. The canonical-cache scan-count seam (`scanCanonicalArtifactsFn`) keeps the "scan once per batch" invariant test-asserted.

## Rollback trigger

- A canonical-uniqueness regression (duplicate IDs or batch bypass), a rehydrate logging regression, or a false-positive validation failure that blocks legitimate doc edits.

## Rollback procedure

- `git revert b4c317e0f4ae1920553857ee0aba9d2f4c855131` (revert the merge commit); rebuild the binary.

## Validation window

- One closure cycle. No deploy surface; low blast radius (internal robustness, additive options with preserved-caller defaults).

## Owner

- Ship pipeline operator (softwaresalt).

## Source artifact cleanup

- Source deliberation **049-DL** was archived to `.backlogit/archive/049-DL.md` by `shipment ship 070-S` (in the same archival commit `a1a3941`).
- Feature 070-F frontmatter carries no `source_stash_id` (no originating stash entry to remove) — none to clean up.
- Commit traceability: merge SHA `b4c317e` stamped as the single `commit` value on archived 070-S, 070-F, and all three tasks. The explicit `track_commit` step was satisfied by `shipment ship` stamping; a redundant `update --commit` was deliberately skipped to avoid re-appending a duplicate SHA into the `commit` frontmatter (the dual-SHA ambiguity Copilot flagged on 070.001-T and that was resolved in this PR).

## Follow-up

- None blocking. Advisory (documented, not stashed): `CanonicalCache` is explicitly scoped to one sequential batch and is NOT concurrency-safe; current bulk callers (migrate import loop is sequential; harvest holds the stash lock) honor that. If a future concurrent bulk-create path is introduced, it must build its own cache or add synchronization.

## Readiness

**READY** — change is merged; monitoring = CI test + docline gate; rollback = revert b4c317e.

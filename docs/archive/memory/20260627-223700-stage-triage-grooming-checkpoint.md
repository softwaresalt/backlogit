# Stage Checkpoint — Triage + Grooming (2026-06-27)

- **Date**: 2026-06-27T22:37 -0700
- **Phase**: triage + grouping complete; entering deliberation
- **Branch**: main @ fd7b68f0 (clean); staging artifacts only (no code branch)
- **Index**: 633 artifacts after archival

## Grooming — stash entries archived as consumed/discharged (5)

| Stash | Kind/Pri | Disposition | Verification evidence |
|---|---|---|---|
| 98C4F063 | deliberation/high | CONSUMED (resolved) | gen-docs emits docline frontmatter via shared codec (cmd/gen-docs/main.go normalizeFrontmatter); `backlogit docs lint` → valid:true, 0 violations; cli-reference files carry doc_type:reference + source. Option A (PR #137). |
| E4B7767C | task/high | CONSUMED (resolved) | Same Option A resolution as 98C4F063. gen-docs born-compliant; no drift. |
| 71A2CB10 | task/low | DISCHARGED | docs/memory/ = 18 files (< 40 threshold); 067-S closure compaction archived 150 stale checkpoints. |
| 0615F487 | task/low | CONSUMED (resolved) | L1 zero-write preflight. Resolved by commit a366bd3d ("preflight-validate apply plan"). service.go:162-192 validates all non-noop changes (SafeResolve+BodyBytesChanged) then writes; all-noop plan ⇒ empty writes ⇒ zero filesystem writes. Doc comment aligned. |
| A2436E1E | task/low | CONSUMED (resolved) | L3 decorative --dry-run. Resolved by commit 887522ad. Flag removed entirely from internal/cli/docs.go; dry-run-by-default model documented (absence of --apply = plan-only). |

NOTE: 0615F487 + A2436E1E were NOT in the operator's category-A list. They were
discovered stale during diligence on the docline cluster (C): main moved past the
2026-06-25 run-1 review (which DEFERRED L1-L4) via post-review commits a366bd3d/887522ad.

## Docline cluster (C) re-assessed against current main

| Stash | Finding | State |
|---|---|---|
| 0615F487 | L1 zero-write preflight | RESOLVED → archived |
| A2436E1E | L3 --dry-run decorative | RESOLVED → archived |
| AE53BC5C | L4 TOCTOU re-read at apply | VALID (ApplyMigration still writes plan-time After w/o re-read) — kept active |
| B349CBED | L2 full JSON-schema validation | VALID (ValidateFields still presence-only + vocab) — kept active |

The cluster shrank from 4 → 2. Remaining L2+L4 is a thin next-pass candidate; L2 carries
design nuance (which profile gets full-schema; no JSON-schema lib in go.mod → hand-roll vs add dep).

## Classification of remaining 9 active entries

- Feature-shaped: 21E17BFC (low, large/contingency), D070FD3C (low, forward-UX)
- Task-shaped: C55C5158 (med, design-gated counter), D6B44FF6 (low, perf), 2797E9F8 (low, test ergo),
  B349CBED (low, L2), AE53BC5C (low, L4), 8863C6C8 (med, codec extraction), 9685B1AA (low, archived_from disposition)

## Grouping decision — SELECTED: 8863C6C8 (solo, synthesized covering feature)

Selected unit: **Shared frontmatter codec extraction** (stash 8863C6C8, medium).
Rationale: highest-priority no-design-gate well-bounded unit. Removes active divergence risk
introduced in 067-S (internal/core/doctor.go inlined rewriteArchivedFromField/frontmatterOpenLen/
splitAtFrontmatterFence/atomicWriteArchiveFile mirroring internal/docline codec.go +
service.go atomicWrite). Import cycle: docline→core, so a leaf package neither imports breaks it.
Decomposes cleanly into ≤2h TDD/characterization-first tasks; strong learnings grounding (high conf).

Deferred this pass: docline L2+L4 (thin, L2 design-nuanced), C55C5158 (design gate open),
D6B44FF6/2797E9F8/D070FD3C/9685B1AA (low), 21E17BFC (large, own deliberation).

## Learnings (Step 1.8) — confidence: HIGH

Source: docs/compound/2026-06-26-docline-frontmatter-contract.md + best-practices (atomic
tmp+rename Windows GOOS gate, short-write guard, crash-safe rollback) + the 065-S review/closure.
Invariants to preserve in the extraction: body-byte stability (body_bytes_changed:false), codec
slices body verbatim, fence = first line exactly "---", atomicWrite tmp+Stat-mode+short-write-guard+
Sync+Windows-only-pre-remove+rename, mode preservation (C1). Reuse verification recipe:
`docs lint --format json` (0 violations), single-file --apply idempotency proof (git diff no hunks),
`go test -count=1 ./internal/docline/... ./internal/core/... ./cmd/gen-docs/...`.

## Next steps
1. Deliberate on 8863C6C8 covering-feature scope (leaf package name/boundary; which helpers move).
2. impl-plan → plan-harden (if blast radius warrants; touches internal/core production path) → plan-review.
3. Harvest feature + TDD tasks (dep-wired) → assemble queued shipment.
4. Archive 8863C6C8 as consumed. Stage branch + PR + CI + readiness gate → HALT for operator merge.

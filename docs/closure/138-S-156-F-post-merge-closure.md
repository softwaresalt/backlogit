---
schema_version: "1.0"
chunk_strategy: h1-h2-h3
source: docs/closure/138-S-156-F-post-merge-closure.md
doc_type: closure
title: "138-S Post-merge Closure — S4 Cross-surface golden parity harness"
shipment_id: "138-S"
feature_id: "156-F"
merge_sha: "9145869ee84ce8e7242fceadf7fce9ea2105de6b"
pr_number: 431
merge_date: "2026-09-08T22:25:10Z"
closure_date: "2026-09-08"
releasability: "READY"
compaction_status: "done"
closure_status: READY
---

# 138-S Post-merge Closure — S4 Cross-surface golden parity harness

## Items Completed

| Task | Title | Commit |
|---|---|---|
| 156.001-T (U1) | Parallel-safe three-surface scenario driver | 966dc1a0 |
| 156.004-T (U4a-decl) | Fault-line evidence contract declarations + shape harness (red-deliverable) | 966dc1a0 |
| 156.006-T (U4a-behavior) | Fault-line evidence contract behavior — canonical/decode/validate + golden | c5ba5260 |
| 156.002-T (U2) | Cross-surface comparator and divergence report | 19c924bb |
| 156.005-T (U4b) | Bind S5-S8/S9 producer + S10/S11 consumer obligations | 19c924bb |
| 156.003-T (U3) | Seed recurring-failure corpus | 4c0a9b55 |

## PR #431 Review History

- Local adversarial review: READY_WITH_FOLLOWUPS (0 P0/P1, F-C1+F-C2 P2 — remediated in ab4776f2)
- Copilot review round 1 (HEAD e8ba9b66): 10 threads — 9 fixed in c27b5e6f, 1 deferred (792B0C29)
- Copilot review round 2 (HEAD c27b5e6f): 3 threads — 3 fixed in 788059be
- Copilot review round 3 (HEAD 788059be): SATISFIED — 0 new threads
- Final local re-review (HEAD 788059be): READY_WITH_FOLLOWUPS (0 P0/P1, 1 P2 out-of-scope, deferred BAC83DC4)

## Deferred Follow-ups

| Stash ID | Summary |
|---|---|
| 4DB1DFF1 | Pre-existing golangci-lint/gofmt debt in non-faultline files |
| C549C922 | Comparator hardenings U3-U6 (parseBody direction, early-exit, PostStatePath) |
| FE4FA974 | F-M1 (SanitizeDiagnostic U+FFFD), F-P1 (buildEvidence last-write-wins) |
| 792B0C29 | Real PostStatePath durable-state comparison (architecture, deliberation needed) |
| BAC83DC4 | gateevidence misclassification ErrProofInvalid vs ErrProofUnverifiable (other-event path) |

## Source Artifact Cleanup

- Source stash 5A4DBE3C: not found (already archived by prior Stage session) — skipped

## Documentation Updates

- `docs/ARCHITECTURE.md`: added `internal/faultline` and `internal/faultline/parity` packages to Domain Map and Dependency Direction
- `docs/compound/2026-09-08-parity-harness-design-patterns.md`: new compound learning document (8 patterns)

## Runtime Verification

Not applicable — harness-only changes, no production runtime surfaces.

## Releasability

**READY**: test infrastructure only; no production code changes; all CI green.

## Compaction Status

**done** — P-020 compact-context invoked at post-merge closure. Below threshold (18 files, 39.4 KB); no forced consolidation required. Compact-context report: `docs/memory/2026-09-08-ship-138s-compact-context.md`.


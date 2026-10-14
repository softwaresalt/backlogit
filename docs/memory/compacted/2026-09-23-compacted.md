---
title: "Compacted memory: 2026-09-23 155-S review circuit breaker and Windows test-tooling breakers"
date_range: "2026-09-23 to 2026-09-23"
doc_type: compacted-memory
source_count: 3
---

# Compacted memory: 2026-09-23

Compacts 3 source files. The originals were moved to docs/archive/memory/ (not deleted).

## Decisions made

- 155-S halted at the mandatory local review gate after the three-cycle review-fix limit was exceeded. All 25 task members were done at that point. Shipment 154-S stayed queued and untouched.
- The operator authorized one final bounded remediation cycle from commit fc659e3f. Four deferred scope expansions were captured under P-021 C2 before their findings were closed: 52D18E44 (R6), 5247D4BC (R7), 497D20E3 (R9), 75E02C17 (R11).
- R10 was rejected and not captured. The Orchestrator is routing-only; its continuity carve-out covers only .backlogit/stash.jsonl, docs/memory, start.ps1, and .gitignore.
- The canonical byte-stability circuit breaker stopped further in-session execution of that test. Suggested resolution: a fresh governed session, or running the suite in CI or on Linux where checkout line endings match the committed blobs.
- The Windows handle-relative rename circuit breaker (task 174.068-T) stopped further local tests. Suggested next step: read-only diagnosis of the FILE_RENAME_INFO contract, then a new governed authorization.

## Files modified or created

- internal/core/shipment_ops_windows.go and internal/core/shipment_ops_security_windows_test.go (rename-breaker files)
- internal/faultline/testdata/parity_v1.golden.json and internal/faultline/evidence_conformance_test.go (canonical byte breaker files; the fixture was not modified)
- Remediation commit 74ea112a (three P-021 C1 findings); an uncommitted nine-file final remediation delta was preserved for operator review.
- Tasks completed in continuation: 174.048-T, 174.049-T, 174.050-T, 174.057-T, 174.058-T, 174.062-T.

## Key learnings

- The native Windows full suite keeps a CRLF-only mismatch in TestU4aBehaviorCanonicalByteStable. Blob evidence showed crlf=0 and lf=1 for both HEAD and origin/main.
- Final standard review of the current HEAD returned BLOCKED with 11 P1 findings: pending-intent barriers, legacy claim recovery accepting non-flat artifacts, incomplete journal validation, a read-only Doctor deleting temporary journal evidence, journal directory identity across Windows path operations, Ship bypassing shipment-reconcile safe-close, a reconciliation lock domain that did not fence membership writers, returned_ids null conflicting with the safe-close envelope, a malformed Ship closure predicate, overbroad Orchestrator authority, and closure staging of unrelated .backlogit state.
- The new R1 lock ordering (shipment-lifecycle-global acquired before artifact locks) created an artifact-versus-global lock inversion. The full core test run timed out after ten minutes. Releasing the global lock early broke a claim-serialization contract test.

## Failed or rejected approaches

- Canonical byte test under Windows CRLF checkout: four attempts (checkout-index, git archive materialization, and two variants) all failed with the same LF-versus-CRLF mismatch.
- FILE_RENAME_INFO fixes for the Windows rename test: correcting the allocation size and adding the traverse right did not change the parameter error.
- Releasing the shipment lifecycle global lock after recovery (broke the contract test).
- A fourth autonomous repair after the review breaker tripped.

## Outcomes and open items

- Operator review is required before any further review-fix cycle.
- 155-S was not shipped, no PR was created, and no remediation commit or push occurred. The active checkpoint remained unresolved.
- Shipment 154-S and PR 449 remained untouched.
- Operator-owned follow-up: correcting every inverse lock caller requires production changes outside the authorized surface.

## Sources

- docs/memory/2026-09-23-ship-155s-review-circuit-breaker.md
- docs/memory/2026-09-23/circuit-break-canonical-byte-stability-test.md
- docs/memory/2026-09-23/circuit-break-windows-handle-relative-rename.md

---
chunk_strategy: h1-h2-h3
description: 'Runtime verification for shipment 084-S (ancestor-aware shipment-gate member-evidence staleness) feature PR #182. Exercises the REAL ship-gate entry point gateShipmentCompletion (the function shipment ship invokes) against a REAL git repository with a genuine git merge-base --is-ancestor subprocess. Demonstrates all five behaviors end to end: (1) the fix — a member whose recorded head is an ANCESTOR of the shipment head (post-merge) now PASSES the ship gate where strict equality would reject; (2) a genuinely divergent (non-ancestor) head is still REFUSED; (3) exact equality still PASSES (fast-path); (4) an absent/unverifiable object FAILS CLOSED (git exit 128); (5) a cancelled context FAILS CLOSED (no silent skip). All scenarios ran green. The scratch driver was run against the built code and then removed; permanent regression coverage lives in the committed unit tests.'
doc_type: closure
docline:
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-06-084-S-feature-pr-runtime-verification.md
title: 084-S Ancestor-Aware Shipment-Gate Staleness — Feature PR Runtime Verification
---

# 084-S Feature PR — Runtime Verification

**Shipment:** 084-S — Ancestor-aware shipment-gate member-evidence staleness
**Branch:** `feat/084-ancestor-aware-staleness`
**PR:** #182
**Date:** 2026-07-06
**Verified commit:** `c29b189` (post-Copilot-remediation)

## Method

A scratch runtime-verification harness drove the **real** ship-gate entry point
`gateShipmentCompletion` — the function `shipment ship` invokes via
`internal/core/shipment_lifecycle.go:171` — against a **real** git repository, so the
actual `git merge-base --is-ancestor` subprocess executes (no mocking of the lineage
check). The broker's aggregate diff check (#2) was set to proceed so the member-evidence
ancestor check (#1) and the head-drift bracket (#3) are isolated and observable.

Git fixture (real commits): base **A** → head **B** on `main` (B is a descendant of A —
the post-merge state); divergent **D** is a real sibling of A that is not reachable from B.
The workspace HEAD (= the shipment head) is **B**.

The harness also exercised the new `boundedHelperTimeout` hard cap (broker
`TimeoutSeconds=30` → `min(5s, 30s) = 5s`), confirming the Copilot-remediated cap is on the
live path.

The scratch driver was run against the built code and then removed (it was a throwaway
"scratch scenario," per the runtime-verification step). Permanent regression coverage lives
in the committed unit tests (`TestValidateMemberGateEvidence_StaleRefused`, `TestIsAncestor`,
`TestHeadSHABounded`, `TestHeadDriftError`, `TestHeadResolveError`).

## Results — all five scenarios green

Fixture: `base(A)=89161cb873e2  head(B)=a95ed6a47ba8  divergent(D)=f6894dc0840e`; HEAD = B.

| # | Scenario | Member head | Expected | Observed |
|---|---|---|---|---|
| 1 | **The fix** — ancestor member head (post-merge) | A (ancestor of B) | **PASS** (strict equality A≠B would reject) | `PASS: ship gate PROCEEDS` ✅ |
| 2 | Divergent (non-ancestor) head | D (sibling of A) | **REFUSE** | `BLOCKED: gate evidence is stale (recorded at a divergent head)` ✅ |
| 3 | Exact equality (control) | B (== HEAD) | **PASS** (fast-path) | `PASS: ship gate PROCEEDS` ✅ |
| 4 | Absent / unverifiable object | `deadbeef…deadbeef` (valid shape, no object) | **FAIL CLOSED** | `REFUSED: git merge-base --is-ancestor exit 128 … cannot verify gate evidence lineage` ✅ |
| 5 | Cancelled context | A (ancestor) + cancelled ctx | **FAIL CLOSED** (no silent skip) | `REFUSED: context canceled` ✅ |

### Verbatim transcript (key lines)

```
RUNTIME-VERIFY git fixture: base(A)=89161cb873e2 head(B)=a95ed6a47ba8 divergent(D)=f6894dc0840e
RUNTIME-VERIFY current HEAD (shipment head) = B = a95ed6a47ba8

--- ancestor_member_head_passes(the_fix)
PASS: member head A=89161cb873e2 is an ancestor of shipment head B=a95ed6a47ba8
      -> ship gate PROCEEDS (strict equality would have rejected)

--- divergent_member_head_refused
REFUSE: divergent member head D=f6894dc0840e is NOT an ancestor of B=a95ed6a47ba8
      -> ship gate BLOCKED: shipment refused: member 002.001-T gate evidence is stale
         (recorded at a divergent head): gate blocked: 002.001-T remains active

--- equal_member_head_passes(control)
PASS: member head == shipment head (B=a95ed6a47ba8)
      -> ship gate PROCEEDS (equality fast-path preserved)

--- absent_object_fails_closed
WARN member evidence lineage check failed member=004.001-T
     member_head=deadbeef… shipment_head=a95ed6a47ba8…
     error="git merge-base --is-ancestor exit 128: fatal: Not a valid commit name deadbeef…"
FAIL-CLOSED: absent object -> git exit 128 -> ship gate REFUSED (not a silent pass):
     shipment refused: member 004.001-T cannot verify gate evidence lineage: … exit status 128

--- cancelled_context_fails_closed
FAIL-CLOSED: cancelled context on an ancestor member (A=89161cb873e2)
     -> REFUSED (no silent skip): … context canceled

--- PASS: TestRuntimeVerification_ShipmentGateAncestorAware (2.50s)
    --- PASS: …/ancestor_member_head_passes(the_fix) (0.29s)
    --- PASS: …/divergent_member_head_refused (0.24s)
    --- PASS: …/equal_member_head_passes(control) (0.25s)
    --- PASS: …/absent_object_fails_closed (0.24s)
    --- PASS: …/cancelled_context_fails_closed (0.09s)
PASS
```

## Interpretation

- **The fix is demonstrated on the real path:** scenario 1 proves that a member whose recorded
  gate-evidence head is an ancestor of the current shipment head — precisely the post-merge
  case where a feature-branch build commit sits below the merge commit — now clears the ship
  gate. Under the pre-fix strict-equality rule (`A != B`), this would have been rejected as
  stale, blocking closure. This is the exact regression that blocked 083-S.
- **Non-weakening is demonstrated:** scenario 2 confirms a genuinely divergent (non-ancestor)
  head is still refused, so ancestor-awareness does not admit unrelated work.
- **Fail-closed is demonstrated on the real path:** scenarios 4 and 5 confirm that an
  unverifiable object (git error) and a cancelled context both **refuse** rather than silently
  skip the guard. (In scenario 5 the cancellation was caught at the evidence-load step, which
  is upstream of `isAncestor`; the `isAncestor`/`headSHABounded` cancel/timeout fail-closed
  paths themselves are covered by the dedicated unit tests `TestIsAncestor` and
  `TestHeadSHABounded`.)

## Verdict

**PASS.** The shipped ancestor-aware ship gate behaves exactly as designed on the real
`gateShipmentCompletion` + real-git path: ancestor member heads pass (the fix), divergent heads
are refused (non-weakening), equality is preserved, and unverifiable/cancelled paths fail closed.

---
chunk_strategy: h1-h2-h3
description: "P-002 TDD red-phase gap incident for 148.002-T / U3 in shipment 131-S"
doc_type: incident
schema_version: "1.0"
source: ship-audit-p002
title: "P-002 Incident — 131-S / 148-F / U3 red-phase gap"
---

# P-002 Incident Record — 131-S / 148-F / U3

**Incident type**: P-002 TDD red-phase gap  
**Shipment**: 131-S (Checkpoint create/write path security hardening)  
**Feature**: 148-F  
**Task in question**: 148.002-T (U3: Classify syncWriteFileAtomic outcomes on checkpoint create)  
**Harness file in question**: `internal/events/checkpoint_u3_harness_test.go`  
**Detected by**: Orchestrator post-completion audit  
**Recorded by**: Ship agent (read-only audit session)  
**Date**: 2026-08-28  

## Factual Evidence

### Git History — Single-Commit Introduction

`git log --all --oneline -- internal/events/checkpoint_u3_harness_test.go` returns:

```
0b97483d feat(core): harden checkpoint create/write paths (148-F)
1f470fe6 fix(core): address 148-F adversarial review P0/P1 findings
```

The harness test file was FIRST introduced in `0b97483d` — the same commit that
introduced ALL production code for 148-F (U1 through U5).

### Parent Commit State

At parent commit `323c5247` (the adversarial review gate evidence commit, immediately
before the implementation began), `git ls-tree 323c5247 internal/events/` shows:

- NO `checkpoint_u3_harness_test.go`
- NO `checkpoint_u1_harness_test.go`
- NO `checkpoint_u2_harness_test.go`
- NO `checkpoint_u5_harness_test.go`

All four harness test files for 148-F are ABSENT at the pre-implementation commit.

### Seam Co-Creation

`git diff 323c5247 0b97483d -- internal/events/fsutil.go` shows the
`syncWriteFileAtomicHook` variable was introduced in `0b97483d`:

```go
+var syncWriteFileAtomicHook = syncWriteFileAtomic
```

The seam that the U3 harness test requires to compile was introduced in the SAME
commit as both the test and the production code that uses it.

### Red Phase Analysis

For a valid P-002 red phase for U3, the following intermediate state must be
observable (i.e., compiled AND failed):

1. Seam variable exists: `var syncWriteFileAtomicHook = syncWriteFileAtomic`
2. Test exists and overrides the seam
3. `CreateCheckpoint` still calls `syncWriteFileAtomic` DIRECTLY (seam not yet wired)
4. Test FAILS: `require.Error(t, err)` fails because CreateCheckpoint succeeds

This intermediate state was NEVER committed. There is no commit between
`323c5247` and `0b97483d` showing this state.

### Prior Completion Report Language

The prior Ship agent completion report described U3 red evidence as:
> `PASS immediately via seam`

A test that PASSES immediately was never in a red state. Per P-002:
"write test, confirm it fails (red), implement, confirm it passes (green)."
A passing test cannot be red evidence.

### Comparison to P-002-Compliant Pattern

Compare to 147-F shipment (130-S), which contains SEPARATE harness scaffold commits:
- `9d6cc834` — `test(harness): scaffold RED regressions for parse-error wrap gaps`
- `78f98e70` — `test(harness): scaffold RED regression for Unicode fold duplicate bypass`

These commits contain ONLY test scaffolding (no production code), enabling
verification that the tests fail before implementation. No such commits exist for 148-F.

## P-002 Breach Determination

**DETERMINATION: GENUINE P-002 BREACH**

The "PASS immediately via seam" language in the prior completion report is NOT
an erroneous description of a genuine red phase — it is an accurate description
of a missing red phase. The test was written simultaneously with its production
code. The seam required for the test to compile was also created simultaneously.
The test passed on first run, was never observed to fail.

This is a procedural P-002 violation. The implementation correctness is not in
question (tests pass, Copilot review clean, adversarial review clean), but the
TDD discipline was not followed for U3.

**Note on other units (U1, U2, U5)**: The same single-commit pattern applies
to all 148-F harness tests. The orchestrator specifically called out U3/148.002-T,
which has the clearest evidence of "PASS immediately" due to the seam pattern.
The scope of this incident is U3/148.002-T as the cited violation.

## Scope of Impact

- **Code correctness**: NOT affected. The tests are functionally correct and
  DO test the intended behavior. They will catch future regressions.
- **P-002 procedural compliance**: VIOLATED for U3/148.002-T.
- **131-S shipped status**: 131-S is merged at origin/main (merge commit
  `084c10d3ba43814970f501f318d954901fda99f9`). The breach is procedural;
  the code is behaviorally verified.
- **Subsequent groups**: BLOCKED until orchestrator decides corrective process.

## Recovery Requirements (for Orchestrator)

Per P-002 violation action rules:
1. No retroactive substitute harness may be created to paper over the gap
2. No concealment of the gap
3. Required: Stage plan amendment documenting the P-002 gap and its
   disposition before any subsequent group (Groups 2/3/4) is shipped
4. Required: Decision on whether 131-S needs a formal return-from-shipped
   review (recommended: NO, because code is behaviorally correct and
   merging to main is irreversible without a revert)
5. Required: Future shipments in the dark factory scope must use separate
   harness scaffold commits (pattern: separate `test(harness): scaffold RED`)

## Return-From-Shipped Recommendation

The orchestrator asks whether 131-S needs a "formal return-from-shipped decision."

**Recommendation: No formal revert required**, for the following reasons:
- The merge is irreversible without a `git revert 084c10d3` (P-IX compliant
  history preservation required)
- The P-002 breach is procedural: the implementation IS correct and the tests
  ARE passing and functional
- Three adversarial review personas and three Copilot review rounds found no
  functional bugs in the tests or the implementation
- A revert would reintroduce the security gaps that 131-S fixed (malformed JSON
  checkpoint write, duplicate context key injection, write outcome classification,
  filesystem containment, Extra validation)

The appropriate action is: formally document the P-002 gap (this incident),
require Stage to amend the plan/process for subsequent groups, and prevent
Groups 2/3/4 from shipping until this process gap is acknowledged and addressed.


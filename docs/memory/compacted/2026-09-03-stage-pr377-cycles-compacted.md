---
chunk_strategy: h1-h2-h3
description: "Compacted summary of Stage PR #377 planning cycles 17-37 — P-002 harness-aware workflow policy deliberation"
doc_type: memory
schema_version: "1.0"
source: docs/memory/compacted/2026-09-03-stage-pr377-cycles-compacted.md
title: "Stage PR #377 Cycles 17-37 — Compacted Memory"
---

# Stage PR #377 Cycles 17-37 Compacted Summary

## Context
PR #377 was the Stage planning PR for P-002: harness-aware workflow policy deliberation. This compacted memory consolidates cycles 17-37 conducted across 2026-08-25 and 2026-08-26, reflecting the formal decomposition and remediation of harness-aware workflow policies.

## Planning Scope
The deliberation covered:
- Formal decomposition of harness-aware workflow requirements
- Remediation cycles for contract corrections and policy refinements
- P002 consumer contract definition and validation
- Exempt execution contract formalization
- P2 fold and wave-scoped execution planning
- Declaration withdrawal mechanics (cycle 29)
- Waves memory architecture
- Multi-cycle contract corrections and policy alignment

## Key Decisions

### Declaration Withdrawal (Cycle 29)
- **Decision**: `declaration-only` harness-exemption class withdrawn from the P-002 contract
- **Rationale**: The class allowed production declarations (exported types, serialized fields,
  function/method signatures, sentinels) to land BEFORE any failing test — a direct violation
  of test-first semantics. Declarations are observable production behavior, not mere stubs.
  They require the same harness-satisfied gate as any other production change.
- **Replacement**: Declaration tasks now receive **source-shape harnesses** — compile-capable
  AST harnesses using `go/parser`/`go/ast` that assert over the package's own source text
  rather than referencing a not-yet-declared symbol. These harnesses compile before the
  declaration exists and FAIL on an assertion (red phase), satisfying P-002's compile
  postcondition and P-004's red phase. This preserves test-first without a build error.
- **Impact**: No `declaration-only` class; every production declaration now passes through
  harness-architect like any other behavior-changing task.

### Wave-Scoped Scheduler (P-002.6)
- **Enforced**: Harness-architect is invoked **once per dependency-ready wave** (not once
  across all harness instances). Wave k = tasks in `queued` state whose every dependency is
  `done`. Tasks with unfinished dependencies are explicitly excluded from each wave to prevent
  the one-pass deadlock: a later task's harness cannot compile against a declaration its
  dependency will add, so it must wait until the dependency's wave is complete.
- **Semantics**: Each wave's harnesses are scaffolded simultaneously (all red at once), then
  driven green one task at a time against scoped commands. The full suite runs only at
  wave-convergence (Step 4.6), after every member is done.
- **Benefit**: Ordered execution without loss of test-first invariant; halts deterministically
  on `WAVE_NO_PROGRESS`, `WAVE_CYCLE_DETECTED`, `WAVE_MEMBER_BLOCKED`.

### Harness-Exempt Static Intake Formalization
- **Evaluated**: Once per wave, BEFORE harness generation, over the wave's ready set only.
  This is NOT an "initialization" point — it runs at every wave, not once at session start.
- **Validates**: (1) Contract block present with all 5 canonical keys in order; (2) class is
  exactly one of `docs-only`, `verification-only`, `covered-by` (withdrawn `declaration-only`
  rejected); (3) `exempt_verification_command` is exact and runnable, `exempt_precondition`
  is literal `must-fail-before-deliverable`; (4) task is in the governing plan's closed exempt
  set; (5) change does NOT affect production behavior unless class is `covered-by`; (6) for
  `covered-by`: named owner exists, is a declared dependency, does not itself carry
  `harness-exempt`, and `harness_owner_command` is present and exact.
- **Compliance**: Static intake admits the task; claim-time gate (Step 4.1a) re-evaluates
  owner red evidence before any build.

## Final Outcome

**Status**: PR #377 merged successfully

**Policy Reflection**: Harness-aware workflow policies now formally reflected in:
- `workflow-policies.md` — authoritative policy document
- Consumer contract definitions
- Exempt execution specifications
- Wave-scoped scheduler implementation

**Artifacts**:
- Policy documentation updated and merged (`workflow-policies.md` §P-002.6, §P-002.1)
- Contract definitions finalized (harness-exempt block schema, wave scheduler spec)
- Workflow implementation aligned with formalized policies

## Changed Surfaces

- `workflow-policies.md`: P-002.1 (harness-exempt static intake), P-002.4 (delta class surface),
  P-002.6 (wave scheduler, frozen task-type wave set `M`, red-deliverable mapping, wave
  convergence gate), P-004 (source-shape harness pattern)
- Ship agent (`_ship.agent.md`): Steps 2, 2a, 3, 4.0 rewritten for wave semantics; harness-exempt
  claim-time gate (Step 4.1a) formalized; red-deliverable dispatch contract added

## Failed Approaches / Key Learnings

1. **`declaration-only` exemption failed**: Allowed production declarations before any test
   (P-002 violation). Withdrawn in cycle 29. Source-shape AST harnesses are the correct
   replacement — they compile pre-declaration and assert on source text.

2. **One-pass harness generation failed**: A single pre-loop pass could not scaffold later-wave
   harnesses that needed to compile against declarations added by earlier-wave tasks. Waves
   dissolve this deadlock without weakening test-first.

3. **Immutable manifest `M`**: `return_blocked` removes tasks from the live shipment items list,
   so the scheduler cannot re-derive `M` from that list each wave — `M` must be frozen at
   Step 3 and consulted immutably by Step 4.0.

4. **Wave convergence gate (Step 4.6)**: The full suite cannot run while any open-red deliverable
   is legitimately failing; the gate defers it and runs it unfiltered when `open_red_deliverables`
   is empty. This prevents classification from absorbing real failures.

5. **Scoped commands**: Each task carries its own `harness_cmd` with an explicit package path,
   `-count=1`, and `^TestU<unit>_` selector — never `./...` or a bare package. Scoping is what
   makes per-task green verification safe while sibling harnesses remain red.

## Cycle Coverage
- Cycles 17-27: Decomposition and initial contract work
- Cycles 28-35: Remediation and corrections
- Cycles 36-37: Final validation and merge preparation

---
*Compacted on 2026-09-03. Original cycle files archived to docs/archive/memory/*

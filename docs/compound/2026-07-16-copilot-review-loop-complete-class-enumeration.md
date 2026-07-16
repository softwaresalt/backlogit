---
chunk_strategy: h1-h2-h3
description: How to terminate a recurring Copilot review whack-a-mole loop on prose-consistency defects by enumerating the entire defect class in one adversarial pass instead of patching flagged lines individually
doc_type: learning
docline:
  date: 2026-07-16T00:00:00Z
  severity: medium
  tags:
    - review
    - copilot
    - process
    - adversarial-review
    - circuit-breaker
schema_version: "1.0"
source: docs/compound/2026-07-16-copilot-review-loop-complete-class-enumeration.md
title: Terminate Copilot review whack-a-mole with complete-class enumeration
---

## Problem

An automated reviewer (Copilot) kept raising new findings each cycle on the same
underlying defect class — a technical claim restated inconsistently across many
prose passages in planning/spec docs. Patching only the specific lines the
reviewer flagged each round produced a whack-a-mole loop: fix the 3 flagged
spots, re-request review, get 5 new spots of the identical class elsewhere in the
same files. Each cycle costs a full review-latency round-trip (~4.5 min) plus
reply/resolve/PR-body churn. This is the classic symptom the operator flagged:
"repeated rounds of copilot reviews is inefficient."

## Root cause

When a defect is a *consistency* defect (the same assertion appears in an
inventory entry, a synthesis section, a changelog, a summary, and two sibling
backlog items), the reviewer samples a few instances per pass. Fixing only the
sampled instances leaves the rest to be flagged next round. The loop length is
proportional to how many un-swept instances remain, not to the difficulty of the
fix.

## Solution

Break the loop by treating the finding as a *class*, not a line:

1. **Establish code-verified ground truth once.** Read the actual implementation
   (here: `models.ArtifactFromFrontmatter` vs `core.SetArtifactSize`) and pin the
   exact, correct statement of the claim before editing any prose. Do not
   paraphrase from memory across cycles — that reintroduces drift.
2. **Exhaustively grep every restatement** of the claim across all affected
   files — including passages the reviewer has NOT flagged yet. Search for the
   defect's linguistic signature, not just the flagged line text.
3. **Patch the entire class in one commit**, byte-exact (a Python anchor-span
   script avoids PowerShell quoting hell and preserves CRLF via
   `newline=""`).
4. **Adversarial re-grep** for residual instances (expect 0) AND for
   correct-boundary presence (expect >0) before committing.
5. **Run the readiness gate.** If the next fresh review returns zero new
   findings, the class is closed. If it returns another finding of the SAME
   class, the sweep was incomplete — finish it (that same-class recurrence, not
   a new topic, is what this technique eliminates). Genuinely NEW, different
   findings are normal review iteration, NOT a circuit-breaker trip: per
   `.github/instructions/circuit-breaker.instructions.md` the universal breaker
   trips only on the SAME error 3 times. Handle new findings within the
   review-fix cycle limit (3 cycles). Reaching that cap stops additional
   automated FIXING but does NOT clear the merge gate: per the GitHub PR
   automation policy (§1.8), unresolved valid Copilot findings stay
   merge-blocking until each thread is resolved — fixed, or declined with a
   rationale reply and filed as accurate backlog — or explicitly overridden by
   the operator. Never present residual UNRESOLVED threads as mergeable; resolve
   or decline each, then let the operator make the merge decision.

## Evidence

PR #242 review cycles 9-12. Cycles 9-11 patched flagged lines and each spawned a
fresh batch (the "except X" enumeration-gap loop). Cycle 12 did the exhaustive
complete-class sweep of the entire size-plan/109 body-preservation model in one
pass; the subsequent fresh Copilot review returned ZERO new findings and the
§1.9 gate passed clean. No cycle 13 was needed.

## Applicability

Use this whenever a reviewer repeatedly flags the same *kind* of issue in
different locations — inconsistent terminology, a claim that must be qualified
everywhere, a safety caveat repeated across sections. It does NOT apply to
independent substantive defects (those are genuinely separate work). The tell is:
the new findings are the same sentence template with different anchors.

## Guardrail

Complete-class enumeration is bounded work; open-ended architecture questions are
not. Keep the two separate: sweep the prose-consistency class to closure, but do
NOT let the reviewer pull genuinely unbounded architecture items (evidence-trust
models, replay/binding, exact-byte implementation, rollback guarantees) into the
patch loop. Defer those explicitly to backlog.

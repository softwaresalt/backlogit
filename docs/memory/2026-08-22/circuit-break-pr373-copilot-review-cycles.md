---
type: circuit-breaker
timestamp: 2026-08-22T21:07:00-07:00
agent: Ship
skill: review (pr-lifecycle Copilot review loop)
breaker_type: skill-managed
operation: Copilot review-fix cycle on PR #373 (shipment 129-S / feature 146-F)
attempts: 5
---

# Circuit Breaker: PR #373 Copilot Review-Fix Cycles (129-S / 146-F)

## Failure Chain

Not failures in the sense of broken commands — each cycle was a genuine, distinct, valid
finding from GitHub Copilot's automated PR review, fixed in its own commit. The circuit breaker
triggered on cycle count, not on repeated identical errors.

### Cycle 1 (commit `ec3fe430`)
Three findings on the initial diff: (1) `docline-frontmatter-authoring-guide.md` conflated two
distinct MCP error mappings for docs-lint failures; (2) `TestDocsLintTool_DegradedCorpus_...`
claimed CLI/MCP parity but only exercised MCP; (3) the ship-129-s memory file's historical
red-test count (23) didn't match its own itemized breakdown (25).

### Cycle 2 (commit `3bf7e207`)
Two findings on `internal/events/checkpoint_strict.go`: (1) the reflection-derived create
allowlist admitted the four administrative `disposition*` fields, letting a caller forge
`disposition:"abandoned"` and defeat `AbandonCheckpoint`'s audit trail via its
idempotent-already-abandoned short-circuit; (2) the top-level decode used
`map[string]json.RawMessage`, which silently collapses an exact-duplicate key (not just a
mixed-case alias), letting an unknown key hidden in a shadowed `"progress"` object escape
detection.

### Cycle 3 (commit `f9a41311`)
Two findings, both consequences of cycle 2's own fix: (1) `CheckpointV1.DispositionAt` being a
value-typed `time.Time` defeated its own `,omitempty` tag (encoding/json never omits a struct
zero value), so an ORDINARY create persisted the literal zero-time string, which the cycle-2 fix
then rejected on resubmission — breaking round-trip create; (2) the already-committed
`checkpoint-20260822-064434.json` carried the same spurious `disposition_at` and an obsolete
`status: "active"` approval-gate-halt state.

### Cycle 4 (commit `36b38011`)
One finding: excluding the disposition* KEYS alone was insufficient — `status` itself remained a
legal top-level key, so `status:"abandoned"` with zero disposition fields still passed both the
closed-namespace check and `ValidateCheckpoint`'s oneof, persisting a checkpoint that looked
abandoned but was never audited, and could never be repaired afterward (`AbandonCheckpoint`
refuses any non-"active" status before reaching its own idempotent check).

### Cycle 5 (deferred, NOT fixed)
One new active finding plus two suppressed ("previously missed") findings, all the same class:
`isModeledContextKey` (checkpoint_schema.go) and the closed-namespace key-membership checks
(checkpoint_strict.go) compare keys via `strings.ToLower`, which is NOT equivalent to the Unicode
simple case-folding (`strings.EqualFold`) that `encoding/json` actually uses for field matching.
A crafted key using U+017F (LATIN SMALL LETTER LONG S) can fold-match a modeled field under
Pass 1 (`ParseCheckpoint`) while disagreeing under Pass 2's ToLower-based check, in principle
allowing a checkpoint's `ShipmentID` (or other modeled fields) to flip on reparse. A related
suppressed finding: `CheckpointContext.emit()` appends `Extra` values without validating they are
well-formed JSON, so a directly-constructed (not decoded) `CheckpointContext` with a malformed
`Extra` entry can make `MarshalJSON()`/`Keys()` return invalid JSON with a nil error.

## Context

- Files involved: `internal/events/checkpoint_schema.go`, `internal/events/checkpoint_strict.go`
- Repository policy: `.github/instructions/circuit-breaker.instructions.md` — "Review-fix cycles
  per task | 3 | Accept remaining as backlog, commit, move on"
- Resolution: cycles 1–4 were fixed and their review threads resolved. Cycle 5's finding is real
  and NOT a false positive, but the review-fix loop has now run 5 cycles, exceeding the 3-cycle
  limit. Per policy, further automated fixing on this specific PR is halted; the finding is
  recorded as backlog follow-ups (stash `6D03554D` — Unicode fold-matching fix; stash `F89CADB7`
  — Extra JSON-validation fix) with the reviewer's exact repro and suggested fix preserved. The
  corresponding PR review thread is left **unresolved** (not force-resolved) so the operator can
  see it is a real, deferred, not-yet-fixed finding.
- The PA-8/PA-3 operator authorization that permitted this implementation work does not cover
  merge approval; this deferral does not change that — the PR remains at the merge-approval gate,
  now with one open (low-severity, low-probability) residual-risk thread the operator should
  review before deciding to merge, fix, or explicitly accept the risk.

## Addendum: Operator-Authorized Exceptional Cycle 6 (2026-08-22T14:53:06-07:00)

The operator issued an explicit, narrowly-scoped authorization for exactly one additional
review-fix cycle beyond the 3-cycle policy limit recorded above, covering the two specific
unresolved Copilot threads then present at reviewed HEAD `894daab4e7f6dcd0b610889c2d51da29c7a77135`:

1. Thread `PRRT_kwDORzozKM6bbTUC` — the Unicode simple-case-folding gap this record's Cycle 5
   entry already identified (stash `6D03554D`).
2. Thread `PRRT_kwDORzozKM6bbbVm` — the committed checkpoint
   `.backlogit/checkpoints/checkpoint-20260822-212617.json` carrying the reserved zero-value
   `disposition_at` field that the Cycle 4 create-boundary fix now rejects at create.

This authorization explicitly does NOT cover merge approval, PA-5, any out-of-workspace pinned
binary operation, or any further review-fix cycle beyond this one. Ship executed this cycle as a
bounded remediation:

- Finding 1 fixed by replacing the `strings.ToLower`-based modeled-key membership checks in
  `internal/events/checkpoint_schema.go` (`isModeledContextKey`) and
  `internal/events/checkpoint_strict.go` (closed top-level and nested-progress namespace checks)
  with a single shared `isFoldKeyIn` matcher built on `strings.EqualFold`, which mirrors
  `encoding/json`'s own Unicode simple-case-folding field-matching semantics. Ordering-sensitive
  regression coverage was added in `internal/events/checkpoint_unicode_fold_test.go`, proven to
  fail at pre-fix HEAD and pass after the fix, for both key orderings of the `ſhipment_id` /
  `shipment_id` collision plus a closed-namespace top-level fold-variant acceptance case. Stash
  `6D03554D` and `F89CADB7` are preserved: `F89CADB7` (the `Extra` well-formed-JSON validation gap)
  remains a distinct, unauthorized-for-this-cycle finding and stays open.
- Finding 2 fixed by parsing the committed checkpoint with the current schema, clearing the
  reserved `DispositionAt` pointer (never persisted by an ordinary create per
  `TestCreateCheckpoint_NormalCreateOmitsDispositionAt`), and re-marshalling through
  `jsonutil.MarshalReadable` — the same function `CreateCheckpoint` itself uses — so the repaired
  bytes are exactly what current code would emit. Every other field (timestamps, context,
  progress, decisions, resume hint) was verified byte-identical before and after repair; the
  repaired bytes were independently proven to satisfy the create boundary by round-tripping them
  through `events.CreateCheckpoint` into a scratch directory.

Per this cycle's operator authorization, Ship stops at the merge-approval gate after this fix is
verified clean (fresh Copilot review, CI, and the full defense-in-depth readiness gate). Merge
itself remains outside this authorization and requires separate explicit operator approval.

Generated by autoharness | circuit-breaker.instructions.md log format

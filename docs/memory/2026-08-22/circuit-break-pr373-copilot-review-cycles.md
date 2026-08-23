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

### Cycle 6 outcome: BLOCKED — two new findings outside this cycle's authorization

Both authorized findings were fixed, pushed at commit `fa466658` (branch
`feat/146-f-success-shaped-evidence-loss`), verified with targeted and full-suite `go test ./...`,
`go vet ./...`, `golangci-lint run`, and `gofmt` (clean once the Windows CRLF checkout artifact —
present repo-wide, unrelated to this change — is normalized to LF, matching the LF content
`git` actually stores and CI actually lints). All required CI checks are green at `fa466658`. A
fresh Copilot review was requested and completed at `fa466658` (review id `5001174961`,
`submittedAt` 2026-08-22T22:22:53Z), matching `headRefOid` exactly. Both originally-authorized
threads (`PRRT_kwDORzozKM6bbTUC`, `PRRT_kwDORzozKM6bbbVm`) were replied to with the fix commit SHA
and rationale, then resolved after the fix was confirmed.

That same fresh review produced **two new findings**, neither of which is one of the two findings
this cycle's operator authorization covers:

1. Thread `PRRT_kwDORzozKM6bbr58` (`.backlogit/stash.jsonl`): the active stash entry `6D03554D`
   (the Unicode fold-matching follow-up) is now stale/obsolete at `fa466658` — the fix that entry
   describes has landed and is covered by `checkpoint_unicode_fold_test.go` — and Copilot flags
   that leaving it "active" risks presenting already-completed work as an unresolved bug.
2. Thread `PRRT_kwDORzozKM6bbr6C` (`.backlogit/checkpoints/checkpoint-20260822-212617.json`): the
   checkpoint's own `decisions`/`resume_hint` narrative still says the Unicode fold defect is
   unresolved and directs a future session to run another fix cycle, which is now stale given the
   `fa466658` fix.

Per the operator's explicit instruction for this cycle ("If the fresh review produces ANY new
unresolved finding, do not fix it automatically; persist a blocker and return to the operator"),
Ship did **not** fix either of these two new findings. Both threads are left **unresolved** on the
PR. `mergeStateStatus` is `BLOCKED` on these two unresolved threads (all other gate conditions —
checks, review freshness, mergeable state otherwise — are satisfied). This is a genuine stop
condition, not a policy violation: this cycle's one-time authorization covered exactly two named
findings and did not extend to any further finding, including ones the authorized fix itself
surfaced as a side effect (stale stash/checkpoint narrative). PR #373 remains at the
merge-approval gate; explicit merge approval cannot yet be requested because two Copilot threads
are unresolved. The operator must decide: authorize a further bounded cycle to update the stash
entry and checkpoint narrative (non-code, low-risk housekeeping), accept the two findings as
residual/backlog and instruct Ship to force past them, or provide other direction.

### Cycle 6 addendum: a third new finding after the docs-only blocker commit

Ship recorded the above blocker in this file and pushed it as a docs-only commit, `8009efa7`
(no code changes). GitHub automatically re-ran Copilot review against that new HEAD (review id
`5001186088`, submitted 2026-08-22T22:30:07Z) without Ship requesting it. That review surfaced a
**third new finding**, thread `PRRT_kwDORzozKM6bbuAs`, on the PR **description** text (not a
repository file): the PR's "Post-Review Update" section still describes the Unicode-fold defect
and the checkpoint artifact as unresolved at HEAD `894daab4`, which is now stale given the
`fa466658` fix and its fix-reply/resolve pair. Per the same operator instruction, this finding was
also **not** auto-fixed — editing the PR description is a further action beyond this cycle's
two-finding authorization. All three new findings (`PRRT_kwDORzozKM6bbr58`,
`PRRT_kwDORzozKM6bbr6C`, `PRRT_kwDORzozKM6bbuAs`) remain unresolved pending operator direction.
`headRefOid` is now `8009efa7985c7020f566dc94299a796ad0d05c91`; all required CI checks are green
at that HEAD; `mergeStateStatus` remains `BLOCKED` on the three unresolved threads.

Generated by autoharness | circuit-breaker.instructions.md log format

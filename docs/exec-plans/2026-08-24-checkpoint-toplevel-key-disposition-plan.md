---
chunk_strategy: h1-h2-h3
description: 'Refuse to rewrite a non-conforming checkpoint on the abandon and resolve paths, close the ResolveCheckpoint validity gap, and widen quarantine classification so every checkpoint stays dispositionable by exactly one verb.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md
title: 'Checkpoint disposition rewrites: refuse on unmodeled top-level keys'
---

# Checkpoint disposition rewrites: refuse on unmodeled top-level keys

**Source document**: `docs/decisions/2026-08-24-checkpoint-toplevel-key-disposition-deliberation.md`

**Stash provenance**: `D3CE9E81` (task, high) — scope-boundary follow-up 1 of 8 recorded in
`docs/exec-plans/2026-08-21-success-shaped-evidence-loss-plan.md`; parent work feature `146-F`,
shipment `129-S`; external source `3C7AAC71`. Not release-blocking for `129-S`.

## Problem Frame

Two governed write paths perform an identical `parse -> mutate -> re-marshal` round-trip over a
pre-existing checkpoint file:

* `core.AbandonCheckpoint` (`internal/core/checkpoint_disposition.go:44-124`)
* `events.ResolveCheckpoint` (`internal/events/checkpoint_lifecycle.go:139-180`)

`events.ParseCheckpoint` (`internal/events/checkpoint_schema.go:400-408`) is a plain
`json.Unmarshal` into `CheckpointV1` with no unknown-field handling, and
`jsonutil.MarshalReadable(cp)` re-emits only the modeled fields. Every unmodeled **top-level** key
present in the file on disk is therefore discarded on write. `146-F` closed the nested `context`
case with an `Extra` carrier; the top level was explicitly deferred to this work.

Two facts sharpen the technical frame beyond the stash text:

1. **The two paths are not symmetric.** `AbandonCheckpoint` gates on `ParseCheckpoint` **and**
   `ValidateCheckpoint` and refuses failures with `ErrCheckpointUseQuarantine`. All nine live
   legacy files under `.backlogit/checkpoints/` fail `ValidateCheckpoint` (no `schema_version`, no
   `agent`, no `session_id`, no `created_at`, no `updated_at`), so abandon already refuses them.
   Abandon's residual loss surface is only a parseable, schema-valid document that *also* carries
   extra top-level keys.
2. **`ResolveCheckpoint` has no validity gate at all**, and its failure mode is larger than key
   loss. Run against a legacy file it replaces the whole document with a fabricated skeleton —
   `"schema_version": 0`, `"agent": ""`, `"session_id": ""`, `"created_at": "0001-01-01T00:00:00Z"`,
   `"context": {}` — plus `"status": "resolved"` and a fresh `updated_at`. Every decision, PR
   number, merge SHA, and next-step record in the file is destroyed, and the replacement is itself
   schema-invalid. Its reachable callers are `internal/cli/checkpoint.go:228` and
   `internal/mcp/tools.go:1223`, and both the Stage and Ship session-start recovery protocols
   instruct agents to call resolve on leftover checkpoints.

The decided behaviour (Option B in the source document) is: **refuse to rewrite a non-conforming
checkpoint, route it to the verbatim-move verb, and close the resolve validity gap.** A third
change is mandatory rather than optional: `QuarantineCheckpoint` today refuses any target that
parses and validates (`ErrCheckpointUseAbandon`), so a bare refusal on abandon and resolve would
strand a valid-but-non-conforming file with no disposition verb at all.

## Requirements Trace

| # | Requirement | Provenance | Implementation units |
|---|---|---|---|
| R1 | Shared read-boundary conformance helper, reflection-derived, disposition keys and `status:"abandoned"` legal | source doc, Decided behaviour §1 | U2, U2b, U2c, U2d, U2e, U2g |
| R2 | `AbandonCheckpoint` refuses non-conforming targets before the audit append | source doc, Decided behaviour §2 | U4 |
| R3 | `ResolveCheckpoint` gains a `ValidateCheckpoint` gate and the same conformance check | source doc, Decided behaviour §3 | U3 |
| R4 | `QuarantineCheckpoint` widens malformed classification so the verb pair stays total over its scoped population | source doc, Decided behaviour §4 | U5 |
| R5 | No preservation carrier is added to `CheckpointV1` **for this scope** (decision-anchored, not a permanent ban) | source doc, Decided behaviour §5 | negative requirement — guarded in U2 |
| R6 | The nine live legacy files are left untouched by this work | source doc, Decided behaviour §6 | negative requirement — asserted in U10 (live-corpus hash guard) and U10b (mirror, not live, for the sweep) |
| R7 | Typed, machine-readable refusal naming the offending keys, with one canonical "quarantine is the remedy" predicate, one bounded **raw** machine projection, and one separate human-facing rendering | plan-originated (source doc Unresolved Q1); bounding added in cycle 15, isolated in cycle 16, split machine/human in cycle 17 | U1, U1b, U1c, U7, U7d, U7e, U8 |
| R8 | Every checkpoint **read** surface agrees with the mutation verbs about which files are rewrite-safe, and exposes the offending members atomically per file | plan-originated (source doc Unresolved Q3, widened by plan review; offender projection added in cycle 16) | U6, U6b, U6c, U6d, U8c, U15 |
| R9 | Human-facing design doc restates the verb pair as total over its scoped population | plan-originated (source doc Option B cons) | U9 |
| R10 | Agent-facing instruction surfaces teach the new `resolve` failure mode and the quarantine remedy, without publishing an unsafe repair runbook and without publishing an unbound executable command | plan-originated (plan review; narrowed in cycle 16, executable text removed in cycle 17) | U9b |
| R11 | Every in-place checkpoint rewrite routes through **one guarded seam** that requires parse, validate, and conformance to succeed before any marshal or atomic replace | cycle-16 gate finding H8 | U11, U12, U13, U14, U2f |
| R12 | Remediation is published as **structured, non-executable intent** by `core`/`events`; only the CLI boundary renders an operator command, bound to the canonical workspace and to the A4c approval / preimage / no-clobber contract | cycle-16 gate finding H1 | U1d, U6, U6b, U6c, U16, U9b |
| R13 | The `AbandonCheckpoint` validation wrap preserves both `ErrCheckpointUseQuarantine` and `ErrCheckpointInvalid` through multi-`%w` | cycle-16 gate finding H7 (Principle I is NON-waivable on a touched path) | U17 |

## Resolved Design Questions

The source document deferred three questions to planning. All three are resolved here and each
resolution is pinned by a named test.

**Q1 — Reuse `ErrCheckpointUseQuarantine`/`ErrCheckpointUnknownField`, or introduce a new
sentinel?** → **New sentinel `ErrCheckpointNonConforming` with a typed
`CheckpointNonConformingError{Fields []string}`.**
`ErrCheckpointUnknownField`'s doc comment is scoped to the *create* boundary, whose legal key set
deliberately differs from the read boundary (create rejects the four `disposition*` keys and the
reserved `status: "abandoned"` literal; the read boundary must accept both, or an
already-abandoned checkpoint becomes unreadable). Overloading one sentinel across two different
key sets would make `errors.Is` ambiguous about which contract was violated. The rejected
alternative — returning the existing typed error and wrapping it at the call site with
`fmt.Errorf("%w: %w", ErrCheckpointUseQuarantine, ufErr)` — was set aside because it produces two
competing MCP error codes for one failure.

The cost of a second sentinel is that a client must now match **two** sentinels to answer one
question. U1 therefore also pins the canonical predicate as an exported, tested helper:

```go
// QuarantineIsRemedy reports whether err means "this checkpoint cannot be rewritten;
// route it to QuarantineCheckpoint".
func QuarantineIsRemedy(err error) bool {
    return errors.Is(err, ErrCheckpointUseQuarantine) || errors.Is(err, ErrCheckpointNonConforming)
}
```

**Q2 — Which error for `ResolveCheckpoint`'s new invalid-document refusal?** → **Wrap
`ErrCheckpointUseQuarantine` and the validation error together with multi-`%w`:**
`fmt.Errorf("%w: %w", backlogiterrors.ErrCheckpointUseQuarantine, valErr)`.

The shipped `AbandonCheckpoint` idiom is `fmt.Errorf("%w: %v", ErrCheckpointUseQuarantine, valErr)`
(`internal/core/checkpoint_disposition.go:70-73`). Copying it verbatim would be a defect: `%v`
drops the `ErrCheckpointInvalid` sentinel that `ValidateCheckpoint` returns
(`internal/events/checkpoint_schema.go`), so `errors.Is(err, ErrCheckpointInvalid)` becomes false
and the one mapping that would classify the refusal as `validation_failed` is lost. Go 1.20+
supports multiple `%w` verbs and the module is Go 1.24, so both sentinels stay traversable. U3
asserts **both** `errors.Is` checks hold. The pre-existing `%v` defect in `AbandonCheckpoint` is
recorded as a named follow-up rather than propagated (see Follow-ups).

**Q3 — Should `ListCheckpoints` flag valid-but-non-conforming files?** → **Yes, and widened by
plan review to cover `GetCheckpoint` as well (U6).**

The source document's Scope boundary listed `ListCheckpoints` as out of scope. **This plan
explicitly overrides that exclusion**, for a reason the source document did not have: once the
mutation verbs refuse a class of file, every *read* surface that still reports that file as
healthy is actively misleading. `GetCheckpoint` hardcodes `valid: true` for any document that
parses and validates (`internal/events/checkpoint_lifecycle.go:105-137`,
`internal/mcp/tools.go:1197-1210`, `internal/cli/checkpoint.go:193-200`), so without U6 an agent
running the canonical `list` → `get` → choose-verb sequence would read `needs_quarantine: true`
from one surface and `valid: true` from the other, then pick the verb the plan just closed. Read
surfaces must agree with write surfaces about what is rewrite-safe. That is R8, and it is
plan-originated rather than inherited — the Requirements Trace labels it as such.

## Implementation Units

**Cycle-17 formal decomposition (normative).** The cycle-16 gate returned `decision: FAIL` with
`restage_recommendation: formal-decomposition`. The remaining work is organised into five DAG
partitions. Partition order is a hard execution order: no unit in partition *n* may begin before
every unit in partitions `1..n-1` that it depends on has landed. Each partition is independently
reviewable, and the partition a unit belongs to is stated in that unit's section.

| # | Partition | Units | Owns |
|---|---|---|---|
| 1 | Foundation diagnostics and conformance | U1, U1b, U1c, U1d, U2, U2b, U2c, U2d, U2e, U2g, U2h | The typed refusal carrier, the bounded machine projection, the human rendering, the structured remediation intent, and the read-boundary conformance predicate |
| 2 | Guarded rewrite seam | U11, U12, U13, U14, U2f | One centralized, precondition-guarded rewrite seam for every in-place checkpoint rewrite, plus a supplemental caller-set regression guard |
| 3 | Declarations and genuine RED harness order | U15, U8b | The read-result carrier declaration and the cross-surface parity harness, both landing **before** any behavioural implementation in partition 4 |
| 4 | Implementation plus MCP/CLI/instruction contracts | U3, U3b, U4, U17, U5, U6, U6d, U6b, U6c, U7, U7d, U7e, U7b, U7c, U8, U16, U8c, U9, U9b | The per-verb gates and sentinel contracts, the read-surface projections, the MCP and CLI surfaces, and the agent-facing instruction delta |
| 5 | Runtime verification and closure | U10, U10b, U10c | Runtime proof of refusal, acceptance, evidence integrity, cross-surface context-duplicate behaviour, and the abandoned-resolve handler mapping |

**Test-first posture (two-step red, mandatory).** A test file that references an undeclared
symbol does not compile, and a build error is **not** a red assertion. Development Workflow #1
requires a *compiling but failing* harness. Every code unit therefore runs in two steps:

1. **Declaration step** — land the minimum compilable stub so the package builds: the sentinel
   `var`, the type with `Error()` / `Unwrap()`, or the function with a `return nil` body. No
   behaviour.
2. **Harness step** — land the tests. They must now **compile and fail on assertions**. The
   expected red is recorded per unit below. Only then implement.

A unit is not red until its **exact** red-verification command prints assertion failures rather
than a build error. The placeholder `go test ./<pkg>` phrasing used through cycle 15 was not
executable — it named no package, no test selector, and no cache-defeating flag, so an implementer
could report "red" from a stale cached result or from an unrelated failing test in the same
package. Cycle 16 replaces it with the per-unit table below.

**Harness naming contract (mandatory).** Every harness test function a unit lands MUST be named
`TestU<unit>_<Descriptor>` — `TestU2g_ExactDuplicateContextMember`, `TestU7e_AbandonedResolveMaps`,
and so on. The unit token is the plan's unit label verbatim, lower-case suffix included. This is
what makes each `-run` regex below exact and drift-proof: `^TestU2_` cannot match `TestU2b_`,
`TestU2c_`, or `TestU2g_`, so a unit's red gate can never be satisfied by a sibling unit's
failures. `-count=1` is mandatory on every invocation: without it a cached `ok` line from a prior
run is indistinguishable from a genuine pass.

| Unit | Task | Red-verification command |
|---|---|---|
| U1 | `147.001-T` | `go test -count=1 -run '^TestU1_' ./internal/errors` |
| U1b | `147.030-T` | `go test -count=1 -run '^TestU1b_' ./internal/errors` |
| U1c | `147.031-T` | `go test -count=1 -run '^TestU1c_' ./internal/errors` |
| U1d | `147.032-T` | `go test -count=1 -run '^TestU1d_' ./internal/events` |
| U2 | `147.002-T` | `go test -count=1 -run '^TestU2_' ./internal/events` |
| U2b | `147.003-T` | `go test -count=1 -run '^TestU2b_' ./internal/events` |
| U2c | `147.004-T` | `go test -count=1 -run '^TestU2c_' ./internal/events` |
| U2d | `147.005-T` | `go test -count=1 -run '^TestU2d_' ./internal/events` |
| U2e | `147.020-T` | `go test -count=1 -run '^TestU2e_' ./internal/events` |
| U2f | `147.021-T` | `go test -count=1 -run '^TestU2f_' ./internal/events` |
| U2g | `147.028-T` | `go test -count=1 -run '^TestU2g_' ./internal/events` |
| U2h | `147.033-T` | `go test -count=1 -run '^TestU2h_' ./internal/events` |
| U11 | `147.034-T` | `go test -count=1 -run '^TestU11_' ./internal/events` |
| U12 | `147.035-T` | `go test -count=1 -run '^TestU12_' ./internal/events` |
| U13 | `147.036-T` | `go test -count=1 -run '^TestU12_' ./internal/events` (U12 owns the seam harness; U13 turns it green) |
| U14 | `147.037-T` | `go test -count=1 -run '^TestU14_' ./internal/events ./internal/core` |
| U15 | `147.038-T` | `go test -count=1 -run '^TestU15_' ./internal/events` |
| U3 | `147.006-T` | `go test -count=1 -run '^TestU3_' ./internal/events` |
| U3b | `147.007-T` | `go test -count=1 -run '^TestU3b_' ./internal/events` |
| U4 | `147.008-T` | `go test -count=1 -run '^TestU4_' ./internal/core` |
| U17 | `147.040-T` | `go test -count=1 -run '^TestU17_' ./internal/core` |
| U5 | `147.009-T` | `go test -count=1 -run '^TestU5_' ./internal/core` |
| U6 | `147.011-T` | `go test -count=1 -run '^TestU6_' ./internal/events` |
| U6b | `147.012-T` | `go test -count=1 -run '^TestU6b_' ./internal/events` |
| U6c | `147.022-T` | `go test -count=1 -run '^TestU6c_' ./internal/mcp` |
| U6d | `147.023-T` | `go test -count=1 -run '^TestU6d_' ./internal/events` |
| U7 | `147.013-T` | `go test -count=1 -run '^TestU7_' ./internal/mcp` |
| U7b | `147.014-T` | `go test -count=1 -run '^TestU7b_' ./internal/mcp` |
| U7c | `147.024-T` | `go test -count=1 -run '^TestU7c_' ./internal/mcp` |
| U7d | `147.025-T` | `go test -count=1 -run '^TestU7d_' ./internal/mcp` |
| U7e | `147.029-T` | `go test -count=1 -run '^TestU7e_' ./internal/mcp` |
| U8 | `147.015-T` | `go test -count=1 -run '^TestU8_' ./internal/cli` |
| U8b | `147.016-T` | `go test -count=1 -run '^TestU8b_' ./internal/cli` |
| U8c | `147.027-T` | `go test -count=1 -run '^TestU8c_' ./internal/cli` |
| U16 | `147.039-T` | `go test -count=1 -run '^TestU16_' ./internal/cli` |
| U9, U9b | `147.017-T`, `147.018-T` | `go run ./cmd/backlogit --cwd . docs lint` — docs units, no Go harness |
| U10, U10b, U10c | `147.019-T`, `147.026-T`, `147.041-T` | verification units — see Deepened runtime verification |

**Selector disjointness holds for every label above.** The mandatory `_` separator in
`TestU<unit>_<Descriptor>` is what makes the regexes exact: `^TestU1_` cannot match `TestU1b_`,
`TestU1c_`, `TestU1d_`, `TestU11_`, `TestU14_`, `TestU15_`, `TestU16_`, or `TestU17_`; `^TestU2_`
cannot match `TestU2f_`, `TestU2g_`, or `TestU2h_`; `^TestU10_` cannot match `TestU10b_` or
`TestU10c_`. Every new cycle-17 label was chosen against this rule.

This table is the single source of truth for red verification. Each unit's **Expected red** line
names *which* cases fail; this table names *how* the implementer observes them. Tasks reference the
table by unit rather than restating the command, so the two cannot drift.

**Declared regression guards.** Not every scenario listed under a unit is a red assertion. A
scenario that asserts already-shipped behaviour, or that expects `nil` from a `return nil`
declaration stub, passes from the moment it lands. Those cases are **declared regression guards**:
each unit's **Expected red** line names which of its cases fail and which are guards. A guard is
not a test-first violation — silently counting one as red is, because it hides the fact that no
assertion ever failed. **P-004 has no "all-guards" exemption**: every unit still needs at least
one assertion that fails against a compilable pre-implementation state, and that red assertion
must be recorded per unit. A unit's role as a parity or invariant guard is compatible with a red
assertion — it does not replace one (cycle-8 correction).

### U1 — Non-conforming sentinel, typed error, and the canonical remedy predicate

* **Partition**: 1 (foundation diagnostics and conformance)
* **Domain**: code (errors)
* **Files**: `internal/errors/checkpoint_errors.go`, `internal/errors/checkpoint_errors_test.go`
* **Change**: add `ErrCheckpointNonConforming`, `CheckpointNonConformingError{Fields []string}`
  with `Error()` and `Unwrap() error` returning the sentinel (mirroring
  `CheckpointUnknownFieldError`, `internal/errors/checkpoint_errors.go:81-108`), and the exported
  `QuarantineIsRemedy(err error) bool` predicate from Q1. `Fields` is sorted, de-duplicated key
  paths only — **never** key values. **Neither bounding nor quoting is in this unit**: the bounded
  **raw** machine projection is owned by **U1b** (`147.030-T`) and the **human-facing quoted**
  rendering is owned by **U1c** (`147.031-T`). U1 declares the carrier; U1b bounds it for machines;
  U1c renders it for humans. Splitting machine from human is the cycle-17 resolution of gate
  finding H4: a machine array must carry raw paths, and escaping belongs only where a human reads
  the text.
* **Tests** (3): `errors.Is` matches the sentinel through the typed error and `errors.As` recovers
  `Fields` from a wrapped error; `Error()` renders a non-empty message naming the field count;
  `QuarantineIsRemedy` is true for both `ErrCheckpointUseQuarantine` and `ErrCheckpointNonConforming`
  and false for `ErrCheckpointNotActive` and `nil`.
* **Expected red**: all three fail on assertions after the declaration step (typed error returns
  zero-value `Fields`, `Error()` returns the bare sentinel text, `QuarantineIsRemedy` returns
  `false`).

### U1b — Bounded raw offender path projection with truncation metadata

* **Partition**: 1 (foundation diagnostics and conformance)
* **Domain**: code (errors)
* **Files**: `internal/errors/checkpoint_errors.go`, `internal/errors/checkpoint_errors_test.go`
* **Change (cycle-17 rewrite — machine form is raw)**: add the exported method

  ```go
  // BoundedFieldPaths returns the sorted, de-duplicated offender paths in RAW form,
  // bounded for machine consumption. Truncation is reported structurally, never as
  // a synthetic path element.
  func (e *CheckpointNonConformingError) BoundedFieldPaths() BoundedFieldPathSet
  ```

  where `BoundedFieldPathSet` is an exported struct with
  `Paths []string \`json:"paths"\``, `Truncated bool \`json:"truncated"\``,
  `OmittedPaths int \`json:"omitted_paths"\``, and
  `TruncatedPaths int \`json:"truncated_paths"\`` — none with `omitempty`, per
  `docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md`.
* **Raw-path rule (this is the point of the cycle-17 rewrite)**: `Paths` carries the offender key
  paths **verbatim**, exactly as decoded from the document. It MUST NOT contain `strconv.Quote`
  output, and it MUST NOT contain a synthetic `"+N more"` pseudo-element. Cycle 15 and cycle 16
  put both into the machine array; the cycle-16 gate ruled that blocking (H4). A consumer that
  receives a quoted path cannot compare it against the document's own key without re-parsing a Go
  string literal, and a `+N more` element is indistinguishable from a real offender named
  `+N more`. Truncation is therefore reported in `Truncated`, `OmittedPaths`, and `TruncatedPaths`,
  which are unambiguous and machine-checkable.
* **Cap semantics (UTF-8 safe, decided here)**:
  * at most **16** paths are returned; when more offenders exist, `Truncated` is `true` and
    `OmittedPaths` carries the exact count of paths not returned;
  * each returned path is capped at **128 bytes**; a path longer than the cap is cut at the last
    rune boundary at or before byte 128 (`utf8.DecodeLastRuneInString` / `utf8.RuneStart`), so a
    returned path is **always** valid UTF-8 and never ends mid-rune;
  * when at least one path was byte-capped, `Truncated` is `true` and `TruncatedPaths` carries how
    many paths were shortened;
  * a path whose **first** rune already exceeds the byte cap is returned as the empty string and
    counted in `TruncatedPaths`, so the caller still learns an offender existed without receiving
    invalid bytes;
  * selection is deterministic: sort, de-duplicate, then take the first 16.
* **Single-projection rule**: `BoundedFieldPaths()` is the **only** sanctioned source of machine
  offender data. U6b's `NonConformingFields`, U7's `unknown_fields`, and U6c's
  `non_conforming_fields` all read it; none re-derives a list from the raw `Fields` slice, and none
  applies its own cap. Copying the raw slice into the MCP payload would leave one surface bounded
  and another unbounded — the exact cross-surface drift R8 exists to prevent.
* **Tests** (3): an under-cap field list round-trips verbatim and unquoted with `Truncated: false`,
  `OmittedPaths: 0`, `TruncatedPaths: 0`; a 21-path list yields exactly 16 raw entries with
  `Truncated: true`, `OmittedPaths: 5`, and **no** synthetic marker element; a path built from
  multi-byte runes that crosses the 128-byte cap is returned cut on a rune boundary, is valid UTF-8
  (`utf8.ValidString`), and is counted in `TruncatedPaths`.
* **Expected red**: all three fail (`BoundedFieldPaths` returns the zero `BoundedFieldPathSet`
  after the declaration step, so the raw round-trip, the truncation-metadata, and the rune-boundary
  assertions all fail on assertions rather than on a build error).
* **Consumed by**: U6b (`NonConformingFields`), U6c (`non_conforming_fields`), U7 (`unknown_fields`),
  U1c (human rendering).
* **Depends on**: U1.

### U1c — Human-facing quoted rendering of offender paths

* **Partition**: 1 (foundation diagnostics and conformance)
* **Domain**: code (errors)
* **Files**: `internal/errors/checkpoint_errors.go`, `internal/errors/checkpoint_errors_test.go`
* **Change**: add the exported method
  `func (e *CheckpointNonConformingError) FieldPathsForDisplay() string`, and re-point `Error()`
  onto it. `FieldPathsForDisplay` reads `BoundedFieldPaths()` and renders each path through
  `strconv.Quote`, joined with `", "`, followed by an explicit human clause when the set is
  truncated — for example `… (5 more omitted, 1 shortened)`. This is the **only** place quoting or
  escaping happens.
* **Why this is a separate unit from U1b**: quoting is a presentation concern with a different
  correctness criterion (no raw control bytes reach an operator terminal or a log line) than
  bounding (no unbounded interpolation reaches any consumer). Folding them together is what
  produced the cycle-16 H4 finding: one method served a machine array and a human string, so the
  machine array inherited the human escaping. Keeping them apart makes the invariant checkable —
  no machine surface may call `FieldPathsForDisplay`, and no human surface may print `Paths`
  directly.
* **Tests** (3): `Error()` contains every offender path in quoted form and contains no raw
  unescaped control byte; a truncated set renders the omitted and shortened counts in the human
  clause; a path containing a double quote and a newline is escaped rather than emitted verbatim.
* **Expected red**: all three fail (`FieldPathsForDisplay` returns `""` after the declaration step
  and `Error()` still renders the bare sentinel text from U1).
* **Consumed by**: U8 (CLI refusal text), U16 (CLI remediation block).
* **Depends on**: U1b.

### U1d — Structured, non-executable remediation intent

* **Partition**: 1 (foundation diagnostics and conformance)
* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_schema.go`,
  `internal/events/checkpoint_remediation_test.go` (new)
* **Change (cycle-17 — closes gate finding H1)**: declare an exported, **non-executable** carrier

  ```go
  // RemediationIntent describes what an operator must do to dispose of a checkpoint
  // that cannot be safely rewritten. It carries no shell text and is not runnable.
  type RemediationIntent struct {
      Verb             string `json:"verb"`               // "quarantine"
      TargetFilename   string `json:"target_filename"`    // bare filename, never a path
      RequiresApproval bool   `json:"requires_approval"`  // always true
      ApprovalClass    string `json:"approval_class"`     // "A4c"
      Reason           string `json:"reason"`             // "schema_invalid" | "non_conforming" | "unparseable"
  }
  ```

  and add `RemediationIntent *RemediationIntent \`json:"remediation_intent"\`` to
  `CheckpointSummary` (`internal/events/checkpoint_schema.go:371`). `TargetFilename` is the bare,
  already-validated filename; it is **never** a path, never shell-quoted, and never concatenated
  with a directory.
* **Why a struct and not a string**: the shipped `CheckpointSummary.RemediationCommand`
  (`internal/events/checkpoint_schema.go:396-398`) is "the exact CLI invocation an operator or
  agent can run". Emitting that from `internal/events` means a library with no knowledge of the
  caller's working directory publishes a command whose meaning depends on an ambient cwd, and every
  consumer — MCP payload, agent instruction file, plan text — inherits a paste-runnable string that
  is not bound to the workspace it was computed for and carries no approval, preimage, or
  no-clobber obligation. That is the cycle-16 P0. A struct cannot be pasted into a shell.
* **Shipped-field disposition (bounded, no silent contract change)**: `RemediationCommand` keeps
  its shipped meaning and its shipped population on the **parse-failure** branch exactly as it is
  today. It is marked `// Deprecated: use RemediationIntent; RemediationCommand is an unbound
  command string and will be removed.` This unit adds **no** new `RemediationCommand` population —
  in particular U6's new conformance branch populates `RemediationIntent` only. Removing the
  deprecated field is a separate contract change recorded as follow-up stash **`F350503F`**.
* **Tests** (3): a `RemediationIntent` marshals with all five keys present and no `omitempty`
  elision; `TargetFilename` round-trips a bare filename unchanged and the struct contains no field
  holding shell text; a `CheckpointSummary` with a nil intent marshals `remediation_intent: null`
  rather than omitting the key.
* **Expected red**: cases 1 and 2 fail (the type does not exist until the declaration step, then
  returns zero values). Case 3 is a declared regression guard for the always-present-key contract.
* **Consumed by**: U6, U6b, U6c, U16, U9b.
* **Depends on**: none inside this plan (it is a leaf declaration in partition 1).

### U2 — Conformance helper: unknown top-level key rule

* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_conformance.go` (new), `internal/events/checkpoint_conformance_test.go` (new)
* **Change**: add exported `CheckConformingTopLevelNamespace(data []byte) error`. It reuses
  `decodeTopLevelEntries` and `isFoldKeyIn` unchanged and returns
  `*backlogiterrors.CheckpointNonConformingError` directly. The read-boundary legal-key set is
  `checkpointV1TopLevelKeys ∪ checkpointV1ReservedKeys` — U2 wires this as an **inline
  two-set check** (`isFoldKeyIn(k, checkpointV1TopLevelKeys) || isFoldKeyIn(k, checkpointV1ReservedKeys)`)
  against the two already-existing sets. The named package-level derived set
  `checkpointV1AllTopLevelKeys` is deliberately **not** introduced by this unit — its introduction
  and the read-boundary refactor onto it belong to **U2d** as that unit's real production delta.
  U2 performs **no** reserved-status-value check: `status: "abandoned"` is a legal read-boundary
  value because the reserved keys are admitted. A new file (not `checkpoint_strict.go`) because
  the read boundary and the create boundary have deliberately different legal key sets and must
  not read as one mechanism.
* **Tests** (3): a conforming V1 document returns nil; a document with two unknown top-level keys
  returns the typed error with both keys sorted; a document containing all four `disposition*`
  reserved keys with `status: "abandoned"` returns nil (proving reserved-key admission and the
  deliberate absence of a reserved-status-value check at the read boundary).
* **Expected red**: case 2 fails against the `return nil` stub. Case 1 (conforming V1 → nil) is a
  declared regression guard for the trivial-conforming boundary condition; case 3 (reserved-key
  admission) is a declared regression guard for the read-boundary admission of the disposition*
  namespace, which is a shipped behaviour of `checkpointV1ReservedKeys` and not introduced here.
  **P-004 is satisfied** by case 2 as the single red assertion.
* **Depends on**: U1.

### U2b — Conformance helper: nested `progress` rule and open `context` namespace

* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_conformance.go`, `internal/events/checkpoint_conformance_test.go`
* **Change**: recurse into every case-insensitive `progress` match using `unknownNestedProgressKeys`,
  reporting offending paths in the existing `progress.<key>` form. A `progress` value that is not a
  JSON object (null, scalar, array) is not a conformance failure — `ParseCheckpoint` already governs
  that — and must not panic.
* **Tests** (3): an unknown nested key returns `progress.<key>`; **unmodeled `context` keys return
  nil** (the open namespace shipped in 146-F must not be swept into refusal — this is the
  highest-risk regression in the plan); a non-object `progress` returns nil without panicking.
* **Expected red**: case 1 fails. Case 2 (unmodeled `context` keys return `nil`) is the permanent
  146-F regression guard, and case 3 (non-object `progress` returns `nil`) also passes before the
  recursion lands, because the U2 top-level check never descends into `progress` at all — both are
  declared regression guards, not red assertions.
* **Scope note**: `unknownNestedProgressKeys` decodes `progress` into a `map[string]json.RawMessage`,
  so duplicate and fold-variant nested keys collapse before the unknown-key diff and are invisible
  to this unit. That gap is closed by **U2e** and must not be folded into this unit's three
  scenarios.
* **Depends on**: U2.

### U2c — Conformance helper: duplicate and fold-variant top-level keys

* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_conformance.go`, `internal/events/checkpoint_conformance_test.go`
* **Change**: the predicate is **round-trip safety**, not merely "no unknown keys". Two top-level
  entries whose keys are `strings.EqualFold`-equal — including exact duplicates — make the document
  non-conforming, reported as `duplicate:<key>`. Without this rule
  `{"status":"active", ..., "Status":"active"}` yields zero unknown keys, is judged conforming, and
  is rewritten with one member's bytes destroyed — the identical loss class this plan exists to
  close. `decodeTopLevelEntries` already preserves exact-duplicate members precisely so this is
  detectable (`internal/events/checkpoint_strict.go:75-90`).
* **Tests** (3): an exact duplicate top-level key is non-conforming and reported as `duplicate:<key>`;
  a case-variant duplicate (`status` + `Status`) is non-conforming; a document with one occurrence of
  every modeled key remains conforming.
* **Expected red**: cases 1 and 2 fail.
* **Depends on**: U2.

### U2e — Conformance helper: duplicate and fold-variant nested `progress` keys

* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_conformance.go`, `internal/events/checkpoint_conformance_test.go`
* **Change**: U2c closes the duplicate and fold-variant loss class at the top level; the identical
  class is still open one level down. `unknownNestedProgressKeys`
  (`internal/events/checkpoint_strict.go:210-232`) unmarshals `progress` into a
  `map[string]json.RawMessage`, so `{"progress":{"step":1,"Step":2}}` collapses to a single member
  before the unknown-key diff runs, is judged conforming, and is then rewritten with one member's
  bytes destroyed — the same round-trip loss this plan exists to close. Walk the `progress` object
  as an ordered token stream (the `decodeTopLevelEntries` technique) inside the **read-boundary**
  helper and report offenders as `duplicate:progress.<key>`, matching U2c's reporting form one
  level down.
* **Shared-function constraint**: `unknownNestedProgressKeys` is also called by
  `checkClosedSchemaNamespace`, the shipped 146-F **create** boundary. This unit must **not** alter
  that function or the create boundary's behaviour; the ordered walk lands as a new
  read-boundary-only helper.
* **Tests** (3): an exact duplicate nested `progress` key is non-conforming and reported as
  `duplicate:progress.<key>`; a case-variant nested duplicate (`step` + `Step`) is non-conforming;
  a `progress` object with one occurrence of each key stays conforming **and** the create boundary's
  verdict on the same bytes is unchanged.
* **Expected red**: cases 1 and 2 fail.
* **Depends on**: U2b (nested recursion), U2c (`duplicate:` reporting form).

### U2d — Read-boundary derived key set and decision-anchored carrier guard

* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_conformance.go`, `internal/events/checkpoint_conformance_test.go`
* **Change**: introduce `checkpointV1AllTopLevelKeys` as a **package-level derived set** —
  `modeledJSONTagKeys(reflect.TypeOf(CheckpointV1{}))` with **no** reserved-key subtraction — and
  **refactor `CheckConformingTopLevelNamespace` off U2's inline two-set check onto the single
  derived set**. This is U2d's real production delta: the read boundary stops carrying the
  reserved-key admission as two runtime lookups against
  `checkpointV1TopLevelKeys ∪ checkpointV1ReservedKeys` and instead consults one authoritative
  derivation of every modeled top-level key, so the hand-written `checkpointV1ReservedKeys`
  literal cannot drift out of the modeled field set without a failing test. Second, add a
  **decision-anchored** guard: `CheckpointV1` declares no `json:"-"` map carrier, with a comment
  naming `docs/decisions/2026-08-24-checkpoint-toplevel-key-disposition-deliberation.md`. This is
  a "revisit the decision before changing this" marker, **not** a permanent ban on top-level
  preservation (see Decisions and Rationale).
* **Two-step red posture**: (a) **Declaration step** lands
  `var checkpointV1AllTopLevelKeys = map[string]struct{}{}` — a compilable empty stub with the
  right identity but empty content — leaving `CheckConformingTopLevelNamespace` on U2's inline
  two-set check unchanged so U2's own tests stay green. (b) **Harness step** lands the tests;
  case 1 fails on assertions (empty ≠ union). (c) **Implementation step** fills in the derivation
  and refactors `CheckConformingTopLevelNamespace` to consult `checkpointV1AllTopLevelKeys`.
* **Tests** (3): `checkpointV1AllTopLevelKeys` equals `checkpointV1TopLevelKeys ∪
  checkpointV1ReservedKeys` — set equality against the hand-written literal, guarding drift in the
  reserved-key set rather than the reflected field set (the narrower claim is the accurate one);
  absence of a `json:"-"` map carrier on `CheckpointV1`; **every exported field of `CheckpointV1`
  carries a non-empty `json:"..."` tag**. The third closes a latent escape hatch: `modeledJSONTagKeys`
  skips untagged exported fields, so a future field added without a tag would appear in the
  derived set only when the escape hatch is closed.
* **Expected red**: case 1 fails against the declaration-step empty stub — set equality does not
  hold when the derived set is empty. Cases 2 and 3 are declared regression guards: `CheckpointV1`
  already declares no `json:"-"` carrier and every exported field is already tagged, so both
  assertions expect a state that already holds. **P-004 is satisfied** by case 1 as the single
  red assertion; cases 2 and 3 are guards, and the unit is no longer claiming an all-guards
  exemption.
* **Depends on**: U2.

### U11 — Guarded rewrite seam: declaration

* **Partition**: 2 (guarded rewrite seam)
* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_rewrite.go` (new),
  `internal/events/checkpoint_rewrite_test.go` (new)
* **Change (cycle-17 — closes gate finding H8)**: declare the single seam through which **every**
  in-place checkpoint rewrite must pass:

  ```go
  // RewriteCheckpointFile is the only sanctioned in-place rewrite path for a stored
  // checkpoint. It reads the file, requires ParseCheckpoint, ValidateCheckpoint, and
  // CheckConformingTopLevelNamespace to all succeed, applies mutate to the parsed
  // document, and only then marshals and atomically replaces the file.
  func RewriteCheckpointFile(
      ctx context.Context,
      checkpointDir, filename string,
      mutate func(*CheckpointV1) error,
  ) error
  ```

  The declaration step lands a compilable stub whose body performs the **currently shipped**
  behaviour of the callers it will absorb — read, parse, mutate, marshal, atomic replace — with no
  validity and no conformance precondition. That is deliberate: the stub must compile and must not
  yet exhibit the guard, so U12's harness fails on assertions rather than on a build error.
* **Scope boundary — quarantine is not in the seam**: `QuarantineCheckpoint`'s `moveNoReplace`
  path never parses and never re-marshals; it moves bytes verbatim. It is **correct by
  construction** and MUST NOT be routed through this seam. Doing so would introduce a parse
  precondition on the one verb whose entire purpose is to dispose of documents that cannot be
  parsed. `CleanupCheckpoints`' `os.Rename` is excluded for the same reason, and
  `CreateCheckpoint` is excluded because it creates a new file rather than rewriting an existing
  one.
* **Tests** (2): the seam is declared with the stated signature and is reachable from
  `internal/core` (an exported-surface compile assertion); calling it with a `mutate` that returns
  an error propagates that error and leaves the file byte-unchanged.
* **Expected red**: case 2 fails against the declaration stub, which writes before checking the
  mutate result. Case 1 is a declared regression guard for the exported surface.
* **Depends on**: U2 (the conformance predicate the seam will call).

### U12 — Guarded rewrite seam: contract harness

* **Partition**: 2 (guarded rewrite seam)
* **Domain**: tests
* **Files**: `internal/events/checkpoint_rewrite_contract_test.go` (new). **No production change.**
* **Change**: land the executable contract for the seam. This is a harness-only unit: it compiles
  against U11's declaration stub and fails on assertions because the stub has no preconditions.
* **Tests** (3): an unparseable document is refused with `ErrCheckpointCorrupt` and the file bytes
  are SHA-identical afterwards; a parseable but schema-invalid document is refused with
  `ErrCheckpointInvalid` and the bytes are SHA-identical; a valid-but-non-conforming document is
  refused with `*CheckpointNonConformingError` naming the offender paths and the bytes are
  SHA-identical.
* **Verdict contract (decided here)**: the seam returns the **raw verdict errors** —
  `ErrCheckpointCorrupt`, `ErrCheckpointInvalid`, and `*CheckpointNonConformingError`. It does
  **not** choose a verb-facing sentinel. Wrapping the verdict into `ErrCheckpointUseQuarantine` or
  `ErrCheckpointNonConforming` is the caller's job, because the wrap differs per verb and per
  gate-ordering rule (I2). This keeps the seam a mechanism and leaves the contract with the verb.
* **Expected red**: all three fail — the U11 stub performs no validity and no conformance check, so
  every case currently rewrites the file and returns `nil`.
* **Depends on**: U11, U2c (the completed top-level conformance predicate the assertions expect).

### U13 — Guarded rewrite seam: implementation

* **Partition**: 2 (guarded rewrite seam)
* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_rewrite.go`
* **Change**: implement the preconditions U12 asserts, in this exact order: `ParseCheckpoint` →
  `ValidateCheckpoint` → `CheckConformingTopLevelNamespace` → `mutate` → `jsonutil.MarshalReadable`
  → `syncWriteFileAtomic`. Any precondition failure returns the raw verdict error **before** any
  marshal or write. Filename validation and path containment
  (`validateCheckpointFilename`, `ensurePathContained`) run first and are unchanged.
* **Tests**: none new — U12 owns the contract. This unit turns U12's three assertions green.
* **Expected red**: not applicable; U13 is the implementation half of U12's red gate. Its
  verification command is U12's selector (`go test -count=1 -run '^TestU12_' ./internal/events`),
  which must go from three failures to zero.
* **Depends on**: U12.

### U14 — Caller migration onto the guarded seam

* **Partition**: 2 (guarded rewrite seam)
* **Domain**: code (events, core)
* **Files**: `internal/events/checkpoint_lifecycle.go`, `internal/core/checkpoint_disposition.go`,
  `internal/events/checkpoint_migration_test.go` (new)
* **Change**: replace the two in-place rewrite sites with seam calls.
  `ResolveCheckpoint` (`internal/events/checkpoint_lifecycle.go:178`) and `AbandonCheckpoint`
  (`internal/core/checkpoint_disposition.go:105-118`) stop calling `syncWriteFileAtomic` /
  `atomicfile.WriteFileAtomic` directly and call `RewriteCheckpointFile` with their existing
  mutation closure. **This unit introduces no new verb-facing sentinel and changes no ordering.**
  Its only claim is that the write mechanism moved.
* **Why the migration is its own unit**: the seam and the per-verb contracts are different skill
  problems. Folding them together would mean one task changed a write mechanism in two packages
  **and** three error contracts **and** two gate orderings — far past the 2-Hour Rule and past
  width isolation. Keeping the migration bare also gives U3, U3b, and U4 an honest red: after U14
  the two verbs refuse untrustworthy documents with the seam's **raw** verdict errors, so each
  verb's own `errors.Is` sentinel assertion still fails until that verb's unit lands.
* **Tests** (3): `ResolveCheckpoint` on a conforming active document still resolves and the
  resulting bytes are unchanged from the pre-migration expectation (shipped-accept-path guard);
  `AbandonCheckpoint` on a conforming active document still abandons and still appends exactly one
  audit event (shipped-accept-path guard); neither `internal/events/checkpoint_lifecycle.go` nor
  `internal/core/checkpoint_disposition.go` retains a direct atomic-write call whose target
  resolves under the checkpoint directory — asserted structurally against the two files.
* **Expected red**: case 3 fails before the migration. Cases 1 and 2 are declared regression
  guards pinning the shipped accept paths, which the migration must not disturb. **P-004 is
  satisfied** by case 3 as the single red assertion.
* **Depends on**: U13.

### U2f — Supplemental caller-set regression guard for the guarded seam

* **Partition**: 2 (guarded rewrite seam)
* **Domain**: tests
* **Files**: `internal/events/checkpoint_writesite_test.go` (new). No production change.
* **Change (cycle-17 rewrite — no longer the authoritative I1 mechanism)**: the cycle-16 gate ruled
  (H8) that an AST enumeration of write calls cannot fully enforce I1: it must resolve call
  targets, alias-imported helpers, indirect writers, and any future helper that wraps an atomic
  write, and any gap in that resolution is a silent hole in the invariant. **The authoritative I1
  enforcement is now architectural** — U11/U12/U13/U14 make one guarded seam the only in-place
  rewrite path, so an ungated rewrite cannot exist without adding a new direct write site.
  This unit keeps the enumeration as a **supplemental caller-set regression guard**: it walks
  `internal/events` and `internal/core` for calls to `syncWriteFileAtomic` /
  `atomicfile.WriteFileAtomic` / `os.WriteFile` whose target resolves under the checkpoint
  directory and asserts the resulting call-site set equals the post-migration allow-list — which
  after U14 contains the seam's own write and the excluded verbatim-move / create sites, and
  nothing else.
* **Honest bound on what this test proves**: it proves that no **direct, statically resolvable**
  atomic-write call to the checkpoint directory was added outside the seam. It does **not** prove
  the absence of an indirect or dynamically dispatched writer, and it is not relied on for that.
  The seam is what makes the invariant hold; this test makes a common regression loud.
* **Halt condition**: if the enumeration cannot be implemented reliably, mark the unit `blocked`
  and record it as a follow-up. Because I1 no longer depends on it, a blocked U2f does **not**
  block the release unit — this is the concrete benefit of moving enforcement into the seam.
* **Tests** (2): the enumerated call-site set equals the post-migration allow-list across both
  packages; a synthetic ungated rewrite site added to the fixture corpus fails the assertion.
* **Expected red**: case 2 fails until the enumeration exists.
* **Depends on**: U14.

### U2g — Context-member duplicate detection read boundary

* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_conformance.go`,
  `internal/events/checkpoint_conformance_test.go`.
* **Change**: extend the read-boundary conformance helper to walk the **original raw bytes** of the
  top-level `context` member as an ordered token stream (the `decodeTopLevelEntries` technique,
  `internal/events/checkpoint_strict.go:75-90`) and report context-member collisions that the
  shipped decode cannot round-trip. `ParseCheckpoint` and `CheckpointContext.UnmarshalJSON` stay
  **lenient and unchanged** — the helper reads the same bytes the caller already holds, before any
  rewrite, so no parse-path behaviour moves.
* **Contract (cycle-15 resolution — one contract, not two)**: offenders are returned inside the
  existing `*backlogiterrors.CheckpointNonConformingError` / `ErrCheckpointNonConforming` pair from
  U1, reported as `duplicate:context.<key>`, matching U2c's `duplicate:<key>` and U2e's
  `duplicate:progress.<key>` one level down. **No new sentinel is introduced.** The rejected
  alternative — detecting duplicates inside `CheckpointContext.UnmarshalJSON` behind a new
  `ErrCheckpointContextDuplicateKey` — was set aside in cycle 15 because it splits one question
  ("is this document rewrite-safe?") across two contracts: a parse-time sentinel fires on every
  read including the non-mutating ones, `ParseCheckpoint`'s shipped
  `fmt.Errorf("%w: %v", ErrCheckpointCorrupt, err)` wrap drops it with `%v` so `errors.Is` cannot
  recover it, and `ListCheckpoints` / `GetCheckpoint` / `resolve` / `abandon` / `quarantine` would
  disagree about the same file. Keeping the verdict in the caller-invoked helper means every gate
  unit (U3b, U4, U5) and every read surface (U6, U6b) inherits it for free through the single
  predicate they already call.
* **Refusal rule (decided, not left to the implementer)**:
  1. **Exact duplicate decoded member names are refused universally.** Comparison is on the
     *decoded* JSON member name, so `{"foo":1,"\u0066oo":2}` is an exact duplicate and is caught;
     escaped-equivalent spellings cannot slip past a raw-byte comparison. This is the loss class:
     `map[string]json.RawMessage` collapses them last-wins before any caller code runs.
  2. **Fold variants are refused only when they alias a modeled context field.** A pair whose
     members are `strings.EqualFold`-equal but not byte-equal is refused **iff** either member
     matches one of `shipment_id`, `feature_id`, `task_ids`, `branch` under `isModeledContextKey`.
     That is the real loss path: `UnmarshalJSON`'s shadow decode picks one winner for the modeled
     field and `isModeledContextKey` filters **both** members out of `Extra`
     (`internal/events/checkpoint_schema.go:196-223`), so the loser's bytes are destroyed.
  3. **Distinct unmodeled names stay conforming.** `foo` and `Foo`, and NFC/NFD-distinct
     extensions, occupy distinct `Extra` map keys and round-trip losslessly. They MUST NOT be
     refused. Broad `EqualFold`-everything or Unicode-normalizing comparison is **forbidden** here
     because it would narrow the open `context` namespace U2b exists to protect — the highest-risk
     regression in this plan.
  4. **Unicode semantics are Go `strings.EqualFold`, no normalization.** This is the same matcher
     `isFoldKeyIn` already uses and the same equivalence relation `encoding/json` itself applies to
     field matching. No `strings.ToLower`, no NFC/NFD folding.
  5. **Scope of the scan is the canonical `context` spelling only.** This unit walks the top-level
     member whose key is literally `context`. The wider question — that `encoding/json` routes
     **every** top-level key satisfying `strings.EqualFold(name, "context")` to the modeled
     `Context` field, so a document carrying a lone `Context`, `CONTEXT`, or `conTexT` and no
     literal `context` still has its members decoded into `CheckpointContext` — is owned by
     **U2h** (`147.033-T`). Splitting it out keeps this unit at three scenarios; folding it in
     would make four.
* **Modeled-collision disposition**: a refused fold-collision against a modeled context field is
  **not** auto-repairable — `legacy_top_level` cannot relocate a pair that both members of collapse
  into the same modeled slot. It carries the operator-choice-or-quarantine semantics U9b already
  states for modeled duplicates, extended to the `context` object.
* **Tests** (3 — two red, one combined green guard; cycle-16 recomposition): **(1, RED)** an
  exact-duplicate `context` member, including an escape-equivalent spelling
  (`{"foo":1,"\u0066oo":2}`), is non-conforming and reported as `duplicate:context.foo`;
  **(2, RED)** a fold variant aliasing a modeled context field (`shipment_id` + `Shipment_Id`) is
  non-conforming and reported as `duplicate:context.shipment_id`; **(3, GREEN)** the
  **open-namespace preservation guard** — one table carrying distinct unmodeled fold variants
  (`foo` + `Foo`), NFC/NFD-distinct extension keys, and a `context` object of unique extension keys
  — asserts every row stays conforming and every key survives the `Extra` round-trip.
  Cycle 15 split these preservation assertions across the tail of case 2 and the whole of case 3,
  which read as four effective flows; they are one predicate ("the open namespace is not narrowed")
  and belong in one already-green regression scenario.
* **Expected red, with green guards separated from the red gate (cycle-17)**: the red gate for this
  unit is **only** `go test -count=1 -run '^TestU2g_Duplicate' ./internal/events`, which selects
  exactly `TestU2g_DuplicateExactContextMember` and
  `TestU2g_DuplicateFoldVariantAliasingModeledField`. Both fail before implementation because no
  context-member walk exists. The open-namespace preservation guard is
  `TestU2g_OpenNamespacePreserved`; it is green from the moment it lands, is deliberately outside
  the red selector, and MUST NOT be counted toward the red gate. The full-unit selector
  `go test -count=1 -run '^TestU2g_' ./internal/events` is what the implementer runs after the
  implementation lands, when all three must pass. Separating the two selectors is the cycle-17
  resolution of the timing ambiguity: a guard that is green on landing can never be evidence that
  the harness was red.
* **Depends on**: U2b (the helper's `context` handling and the open-namespace guard it must not
  narrow), U2c (the `duplicate:` reporting form this unit reuses one level down). The backlog also
  carries a direct `147.028-T -> 147.001-T` (U1) edge from cycle 14; it is redundant with the
  transitive path through U2 but is retained so the typed error this unit returns is explicitly
  pinned as a prerequisite.

### U2h — Context routing under fold-variant `context` spellings

* **Partition**: 1 (foundation diagnostics and conformance)
* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_conformance.go`,
  `internal/events/checkpoint_conformance_test.go`
* **Change (cycle-17 — closes gate finding H, false-negative in the U2g scan)**: `encoding/json`
  matches struct fields case-insensitively, so **every** top-level member whose key satisfies
  `strings.EqualFold(name, "context")` is decoded into `CheckpointV1.Context`. A document carrying
  a lone `Context` — one occurrence, no literal `context` sibling — therefore routes its members
  through `CheckpointContext.UnmarshalJSON` and is subject to the identical collapse loss U2g
  refuses, yet a scan keyed on the literal spelling `context` never inspects it. Extend the
  read-boundary helper so the context-member walk runs against **whichever** top-level member the
  decoder would route to the modeled field, selecting it with `isFoldKeyIn` against the single-key
  set `{"context"}` rather than a literal string comparison. Offenders keep U2g's reporting form,
  `duplicate:context.<key>`, so the reported path names the **modeled** member regardless of the
  spelling found on disk.
* **Interaction with U2c (no double refusal, no gap)**: when a document carries **two or more**
  fold-equal `context` spellings, U2c already refuses it at the top level as `duplicate:context`
  and this unit's walk is not reached. This unit closes only the **lone non-canonical spelling**
  case, which U2c cannot see because there is no duplicate at the top level.
* **Explicitly not claimed**: this unit makes no claim about the **create** boundary. Create-side
  duplicate handling remains deferred under stash `E429A031`, and no universal cross-boundary
  guarantee is stated here or anywhere else in this plan.
* **Tests** (2): a document whose only context-routing member is spelled `Context` and whose inner
  members carry an exact duplicate is non-conforming and reports `duplicate:context.<key>`; a
  document whose only context-routing member is spelled `CONTEXT` and whose inner members are all
  distinct unmodeled extensions stays conforming and every key survives the `Extra` round-trip.
* **Expected red**: case 1 fails — the U2g walk keys on the literal spelling and never inspects a
  `Context` member. Case 2 is a declared regression guard for the open namespace under a
  non-canonical spelling; it is green on landing and is outside the red selector
  (`go test -count=1 -run '^TestU2h_Duplicate' ./internal/events`).
* **Depends on**: U2g.

### U3 — `ResolveCheckpoint` validity gate

* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_lifecycle.go`, `internal/events/checkpoint_lifecycle_test.go`
* **Change**: after `ParseCheckpoint`, the already-resolved short-circuit, and the
  `ErrCheckpointCannotResolveAbandoned` guard (all three keep their current positions and
  semantics), add `ValidateCheckpoint(cp)` refusing with
  `fmt.Errorf("%w: %w", backlogiterrors.ErrCheckpointUseQuarantine, valErr)`. **Multi-`%w`, not
  `%v`** — see Q2. The gate does not write; the file is left byte-identical on refusal.
* **Tests** (3): a legacy nine-file-shaped document is refused and **both**
  `errors.Is(err, ErrCheckpointUseQuarantine)` and `errors.Is(err, ErrCheckpointInvalid)` hold, with
  the file SHA compared before and after; a conforming active document still resolves; an
  already-resolved conforming document is still an idempotent no-op.
* **Expected red**: case 1 fails (resolve currently succeeds and rewrites); cases 2 and 3 pass and
  are regression guards. Verified with `go test -count=1 -run '^TestU3_' ./internal/events`.
* **Depends on**: U2c (the completed predicate, including the duplicate rule — this is the declared
  backlog edge `147.006-T -> 147.004-T`; cycle-15 prose said "U2", which named a transitive
  ancestor rather than the real prerequisite), U14 (the guarded seam and caller migration), and
  U8b (the parity harness that must be red first).

### U3b — `ResolveCheckpoint` conformance gate and the named already-resolved exception

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_lifecycle.go`, `internal/events/checkpoint_lifecycle_test.go`
* **Change**: after the U3 validity gate, add `CheckConformingTopLevelNamespace(data)` returning its
  typed error unchanged, then mutate and write. **Named, accepted residual**: a document whose
  `status` is already `"resolved"` returns `nil` at the step-5 short-circuit and never reaches
  either new gate. This is deliberate (a non-writing terminal answer; moving the gates ahead of it
  would turn a shipped idempotent no-op into a new error) but it means the exact fabricated
  skeletons produced by the *pre-fix* `ResolveCheckpoint` — which carry `status: "resolved"` and are
  schema-invalid — bypass both gates. Their discovery path is U6, not resolve.
* **Tests** (3): a valid-but-non-conforming document is refused with `ErrCheckpointNonConforming`
  naming the keys, bytes unchanged; an **invalid, already-resolved** document returns `nil` with
  bytes unchanged **and** U6 flags the same file `NeedsQuarantine: true` (pins the residual and its
  discovery path together, so neither half can be deleted alone); a document with
  `disposition: "abandoned"` still returns `ErrCheckpointCannotResolveAbandoned`.
* **Expected red**: case 1 fails (resolve currently succeeds and rewrites a non-conforming
  document). Case 2 passes once this unit's `U6` dependency has landed — the already-resolved
  short-circuit is shipped behaviour and `U6` already flags the file — and case 3 asserts the
  shipped `ErrCheckpointCannotResolveAbandoned` guard. Both are declared regression guards, not
  red assertions.
* **Granularity check before starting**: case 2 asserts across the U3b gate *and* the U6 discovery
  path. Before beginning, count the functions modified across `checkpoint_lifecycle.go` and
  `checkpoint_lifecycle_test.go` for U3b combined with any still-open U6 work; if the total reaches
  5, split the cross-unit assertion into a separate `U3c` regression unit rather than breaching the
  2-Hour Rule.
* **Depends on**: U3, U6, U2g, U14, U8b — the declared backlog edges (`147.007-T` ->
  `147.006-T`, `147.011-T`, `147.028-T`, `147.037-T`, `147.016-T`). The cycle-14 `147.007-T ->
  147.021-T` (U2f) edge is replaced by `147.007-T -> 147.037-T` (U14): U2f is no longer the I1
  mechanism, so the guarded seam — not the enumeration test — is what must land before this gate
  touches its call site.

### U4 — `AbandonCheckpoint` conformance gate

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (core)
* **Files**: `internal/core/checkpoint_disposition.go`, `internal/core/checkpoint_disposition_test.go`
* **Change**: call `events.CheckConformingTopLevelNamespace(data)` **immediately after
  `ValidateCheckpoint` and BEFORE the already-abandoned short-circuit**, returning its typed error
  unchanged. Placing it before the short-circuit matters: a file carrying
  `disposition: "abandoned"` *plus* an unmodeled key would otherwise return `nil` (reported
  success, no write) from abandon while U5's widened quarantine accepts it and U6 reports
  `NeedsQuarantine: true` — three surfaces disagreeing about one file. It is a non-writing refusal,
  so nothing is lost by refusing earlier. It remains strictly before
  `appendCheckpointDispositionAudit`, preserving the shipped audit-then-mutate ordering. **The seam
  does not satisfy this unit.** After U14 the guarded seam refuses the same document, but it does
  so at the *write* step — which in `AbandonCheckpoint` is **after** the audit append and after the
  already-abandoned short-circuit. The ordering this unit installs is what makes the refusal
  audit-free and short-circuit-proof, and it is what U4's assertions pin.
* **Tests** (3): a valid-but-non-conforming active document is refused with
  `ErrCheckpointNonConforming` naming the keys; the disposition audit JSONL is byte-unchanged after
  that refusal; a non-conforming **already-abandoned** document returns `ErrCheckpointNonConforming`
  rather than `nil`. The existing "conforming active document abandons successfully" test must stay
  green as a regression guard.
* **Expected red**: all three fail. Verified with `go test -count=1 -run '^TestU4_' ./internal/core`.
* **Depends on**: U2c, U2g, U14 (the guarded seam and caller migration), U8b (the parity harness).
  The cycle-16 `147.008-T -> 147.021-T` (U2f) edge is replaced by `147.008-T -> 147.037-T` (U14):
  U2f is no longer the I1 mechanism, so it is no longer a prerequisite for touching this call site.

### U17 — `AbandonCheckpoint` validation wrap preserves both sentinels

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (core)
* **Files**: `internal/core/checkpoint_disposition.go`, `internal/core/checkpoint_disposition_test.go`
* **Change (cycle-17 — closes gate finding H7)**: `AbandonCheckpoint` wraps its validation failure
  as `fmt.Errorf("%w: %v", ErrCheckpointUseQuarantine, valErr)`
  (`internal/core/checkpoint_disposition.go:~70-73`). The `%v` verb drops the
  `ErrCheckpointInvalid` sentinel that `ValidateCheckpoint` returns, so
  `errors.Is(err, ErrCheckpointInvalid)` is false on a path this plan **already touches** in U4 and
  U14. Change the verb to multi-`%w`:
  `fmt.Errorf("%w: %w", backlogiterrors.ErrCheckpointUseQuarantine, valErr)`. Go 1.20+ supports
  multiple `%w` verbs and the module is Go 1.24, so both sentinels stay traversable — exactly the
  idiom Q2 already requires of `ResolveCheckpoint` (U3).
* **Why the deviation cannot stand (Principle I is not waivable here)**: cycles 1–16 recorded this
  as a "documented deviation" from Constitution Principle I on the grounds that the wrap is
  pre-existing. The cycle-16 gate rejected that: Principle I says *all* errors must be wrapped with
  context and sentinels must remain traversable, and the deviation table's own escape hatch —
  "fixing it changes an unrelated shipped error contract" — does not apply once U4 and U14 modify
  the very function that contains it. A one-verb change in a function this plan already edits is
  not an unrelated contract change; leaving it is a knowingly-shipped Principle I violation on a
  touched path.
* **Bounded blast radius**: the change is strictly **additive** for consumers. Every existing
  `errors.Is(err, ErrCheckpointUseQuarantine)` match still holds because the first `%w` is
  unchanged, the rendered message text is identical (`%w` and `%v` format an `error` the same
  way), and only the previously-lost `errors.Is(err, ErrCheckpointInvalid)` match becomes true.
  `QuarantineIsRemedy` (U1) is unaffected.
* **Tests** (2): abandoning a schema-invalid document yields an error for which **both**
  `errors.Is(err, ErrCheckpointUseQuarantine)` and `errors.Is(err, ErrCheckpointInvalid)` hold; the
  rendered `Error()` string is unchanged from the shipped form, asserted against the same fixture,
  so no consumer parsing the message regresses.
* **Expected red**: case 1 fails — `errors.Is(err, ErrCheckpointInvalid)` is false against the
  shipped `%v` wrap. Case 2 is a declared regression guard for the message text.
* **Width**: 2 files, 2 scenarios, one function, one verb.
* **Depends on**: U4 (same function, same file — U4 lands the gate, U17 corrects the wrap beside
  it), U8b.

### U5 — `QuarantineCheckpoint` widened classification (deadlock closure)

* **Domain**: code (core)
* **Files**: `internal/core/checkpoint_disposition.go`, `internal/core/checkpoint_disposition_test.go`
* **Change**: extend the in-memory classification so `validTarget` is
  `parse OK && validate OK && conformance OK`. Only a target satisfying all three is refused with
  `ErrCheckpointUseAbandon`. The verbatim `moveNoReplace` path, the audit-before-move ordering, the
  sidecar upsert, and the `MutationEnvelope` compensation are unchanged.
* **Tests** (2 scenarios, expressed as one table so the paired assertions cannot be removed
  independently — cycle-16 width normalization):
  1. a valid-but-non-conforming **active** document is **accepted** by quarantine and **refused**
     by abandon in the same table row, and the **archived bytes are byte-identical to the
     pre-quarantine original** as a postcondition of that same row. Cycle 15 counted byte-identity
     as a separate scenario; it is not a separate flow — it is the accept-half's postcondition and
     is strictly stronger when asserted on the row that produced the archive;
  2. a **conforming active** document is refused by quarantine with `ErrCheckpointUseAbandon`.
* **Withdrawn (cycle 16): the `status: "resolved"` state-conflict rows.** Cycle 10 absorbed three
  regression rows from retired U5b into this unit, and cycle 15 re-presented them as extra rows of
  scenario 2. They still assert the **out-of-scope state-conflict class** the origin decision
  excluded, on a second document state this unit's production delta does not touch, and they took
  the unit to four effective flows in every cycle since. They are removed here and recorded as
  follow-up stash `6FA45E69` so the exclusion stays visible without being smuggled into a unit that
  does not own it. Nothing this plan changes depends on them: I3 row 3 remains a documented
  pre-existing condition, and U9's design-doc rewrite still states the `active`-only scope
  qualifier.
* **Expected red**: scenario 1's accept-half fails (quarantine currently refuses it); its
  byte-identity postcondition is green on landing. Scenario 2 is a declared regression guard.
  **P-004 is satisfied** by scenario 1. Verified with
  `go test -count=1 -run '^TestU5_' ./internal/core`.
* **Depends on**: U4 — the single declared backlog edge (`147.009-T` -> `147.008-T`) — and U8b. The
  conformance predicate this unit calls arrives transitively through U4 -> U2c; cycle-15 prose
  listed U2c as a direct prerequisite, which the graph does not carry and which does not need to be
  added because the transitive path already orders it.

### U5b — RETIRED (cycle 10)

Unit U5b is retired. Its production delta — changing the `QuarantineCheckpoint` refusal sentinel
for non-active conforming targets from `ErrCheckpointUseAbandon` to `ErrCheckpointNotActive` —
contradicted the origin decision's explicit scope exclusion of the state-conflict class. The
decision document states the double-refusal row is "pre-existing behaviour introduced by neither
this work nor 146-F" and the plan's Decisions and Rationale names it as "explicitly out of scope:
widening quarantine to accept it would change what 'quarantine' means." Changing the sentinel
returned for that row widens/reopens the decision's scope boundary, and inventing the delta solely
to satisfy P-004's red-gate requirement is not a valid justification.

**Disposition of U5b's assertions (revised in cycle 16):** cycle 10 absorbed U5b's three
state-conflict regression rows into U5 (`147.009-T`) as already-green pinned guards. Cycle 16
**withdraws** them. They assert the out-of-scope state-conflict class on a document state U5's
production delta never touches, and they were the reason U5 read as four effective flows in every
review cycle from 10 through 15. They are recorded as follow-up stash `6FA45E69` ("pin the
conforming + `resolved` double-refusal state-conflict class as a tested invariant") so the class
stays visible and can be picked up by a unit that actually owns it. U5 keeps its genuine red gate
(scenario 1's accept-half) and its byte-identity postcondition; P-004 is unaffected. I3 table row 3
remains documented as pre-existing behaviour, and U9's design-doc rewrite still carries the
`active`-only scope qualifier, so no claim in the plan now rests on an assertion no unit makes.

**Task disposition:** `147.010-T` is archived (history preserved in `.backlogit/archive/`),
removed from shipment `130-S`, and removed from the dependency graph. No downstream tasks depended
on `147.010-T`.

### U6 — `ListCheckpoints` surfaces non-conforming files

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_lifecycle.go`, `internal/events/checkpoint_lifecycle_test.go`
* **Change**: after the existing `ValidateCheckpoint` branch, run
  `CheckConformingTopLevelNamespace(data)`; on failure set `NeedsQuarantine = true` and
  `RemediationIntent`, and **append** to `ValidationErr` rather than overwriting it — a file can
  fail both validation and conformance and the operator needs both reasons.
  **The conformance branch publishes structured intent, never a command string (cycle-17, closes
  gate finding H1).** It sets `RemediationIntent = &RemediationIntent{Verb: "quarantine",
  TargetFilename: <bare filename>, RequiresApproval: true, ApprovalClass: "A4c", Reason:
  "non_conforming"}` — the carrier U1d declares — and leaves `RemediationCommand` untouched.
  `internal/events` has no knowledge of the caller's working directory, so any command string it
  emits is bound to an ambient cwd, carries no approval, preimage, or no-clobber obligation, and is
  paste-runnable by whoever reads it. Rendering an operator command is the **CLI boundary's** job
  and is owned by **U16** (`147.039-T`), which binds it to the resolved storage root and prints the
  A4c preamble. The MCP surface publishes the structured intent as-is (U6c) and never a shell
  string. The shipped `RemediationCommand` population on the pre-existing **parse-failure** branch
  is left exactly as it is — this unit neither extends nor removes it; removal is follow-up stash
  `F350503F`.
  `ListCheckpoints` stays strictly read-only — no move, no rewrite, no error propagation — and the
  conformance branch must run **before** the filter block, so the verdict is computed for every
  parsed document rather than only for documents the caller's filter happens to select.
  **Ordering is not exemption**: only the `ParseCheckpoint` failure path is filter-exempt today
  (`internal/events/checkpoint_lifecycle.go:~46-57` appends and `continue`s before the filter
  block), while the `valErr` branch falls through into the `Agent` / `Status` / `ShipmentID` /
  `FeatureID` / `MaxAge` checks like any other summary. Whether a quarantine candidate also becomes
  filter-exempt is a separate behavioural change owned by **U6d**; this unit must not claim a
  drop-through guarantee it does not implement.
* **Tests** (3): a valid-but-non-conforming file lists with `NeedsQuarantine: true` and a
  `RemediationIntent` naming verb `quarantine`, the bare target filename, `requires_approval: true`,
  and approval class `A4c` — and the summary carries **no** shell text; a file failing **both**
  validation and conformance reports both reasons in `ValidationErr`; the verdict is computed before
  the filter block, asserted by a case whose filter matches the document. **Byte-identity is a
  postcondition of all three, not a fourth scenario** (cycle-15 width normalization): each case
  records the corpus SHA before and after its own `ListCheckpoints` call and asserts equality, which
  is a stronger guarantee than the separate untouched-corpus case it replaces because it binds
  read-only-ness to the three code paths this unit actually changes.
* **Expected red**: cases 1 and 2 fail. Case 3 and the byte-identity postconditions are declared
  regression guards — `ListCheckpoints` is already read-only, so the postcondition holds before
  implementation. **P-004 is satisfied** by cases 1 and 2.
* **Depends on**: U2c, U1d, U8b.

### U6d — quarantine candidates survive the `ListCheckpoints` filter block

* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_lifecycle.go`, `internal/events/checkpoint_lifecycle_test.go`
* **Change**: U6 computes the conformance verdict before the filter block, but the summary then
  falls through into the filter checks, so a non-conforming document is still dropped by a caller
  filtering on `status: "active"` — and its own `status` field is exactly the untrusted data this
  feature refuses to rely on. The remediation surface would then be invisible to precisely the
  query an agent runs at session start. Extend the shipped parse-failure exemption
  (`internal/events/checkpoint_lifecycle.go:~46-57`) to conformance: **any summary with
  `NeedsQuarantine: true` bypasses the whole filter block**, exactly as a parse failure does today.
  This is deliberately a blanket exemption rather than a per-field one — splitting filters into
  "lifecycle" and "identity" tiers would require trusting `Agent`, `ShipmentID`, and `FeatureID`
  read out of a document already judged unsafe to round-trip, and would leave a schema-invalid file
  with an empty `agent` silently dropped by an agent filter. `ListCheckpoints` stays read-only.
  **The exemption is a published contract, not silent behaviour** (PR #377 review cycle 4): the
  exported `ListCheckpoints` doc comment is updated in the same commit to state that a
  `NeedsQuarantine: true` summary bypasses every optional filter, so a caller filtering on status,
  agent, shipment, feature, or age is told to expect rows that do not match its filter. The
  agent-facing `list_checkpoints` description carries the same statement and is owned by U7b,
  which therefore depends on this unit.
* **Tests** (3): a valid-but-non-conforming `status: "resolved"` file is still returned when
  `filter.Status == "active"`, carrying `NeedsQuarantine: true`; a **conforming**
  `status: "resolved"` file is still dropped by that same filter, proving the exemption is scoped to
  quarantine candidates and is not a blanket filter bypass; a non-conforming file is still returned
  when `filter.Agent` names a different agent, matching the parse-failure precedent.
* **Expected red**: cases 1 and 3 fail; case 2 is a declared regression guard. The doc-comment
  update is a contract obligation carried by this unit, not a fourth scenario.
* **Depends on**: U6, U8b.

### U15 — `CheckpointReadResult` carrier declaration

* **Partition**: 3 (declarations and genuine RED harness order)
* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_lifecycle.go`,
  `internal/events/checkpoint_readresult_test.go` (new)
* **Change (cycle-17 — closes gate finding H2's compile premise)**: `GetCheckpoint`
  (`internal/events/checkpoint_lifecycle.go:108`) returns `(*CheckpointV1, error)` and there is no
  get-result type at all, so the cross-surface parity harness (U8b) cannot even compile until a
  carrier exists. Declare it here, ahead of every behavioural unit, so partition 3's harness lands
  before partition 4's implementations:

  ```go
  type CheckpointReadResult struct {
      Checkpoint          *CheckpointV1
      Valid               bool
      Conforming          bool
      NeedsQuarantine     bool
      RemediationIntent   *RemediationIntent
      NonConformingFields backlogiterrors.BoundedFieldPathSet
  }

  func GetCheckpointResult(ctx context.Context, checkpointDir, filename string) (*CheckpointReadResult, error)
  ```

  The declaration step lands `GetCheckpointResult` as a thin wrapper that calls the **existing**
  `GetCheckpoint` and returns `&CheckpointReadResult{Checkpoint: cp, Valid: err == nil}` with every
  other field at its zero value. `GetCheckpoint` is retained unchanged as a wrapper returning
  `res.Checkpoint`, so every existing caller compiles untouched.
* **Declaration-only boundary**: this unit adds **no** conformance evaluation, **no** intent
  population, and **no** offender projection. Those are U6b's production delta. Keeping the
  declaration separate is what makes U8b's red honest — the harness compiles against a real type
  and fails because the fields are unpopulated, not because a symbol is missing.
* **Tests** (2): `GetCheckpointResult` on a conforming active document returns a non-nil result
  whose `Checkpoint` matches `GetCheckpoint`'s return and whose `Valid` is true; on a schema-invalid
  document it returns the pre-existing `ErrCheckpointInvalid` **unwrapped**, so
  `errors.Is(err, ErrCheckpointInvalid)` holds and `QuarantineIsRemedy(err)` is false — a read is
  not a rewrite and there is nothing to refuse.
* **Expected red**: case 1 fails against the pre-declaration state (the symbol does not exist until
  the declaration step, then the wrapper must be wired to return the parsed document rather than
  nil). Case 2 is a declared regression guard pinning the shipped read contract that U6b, U6c, U8c,
  and U8b all depend on **not** changing.
* **Depends on**: U1b (the `BoundedFieldPathSet` field type), U1d (the `RemediationIntent` field
  type).

### U6b — `GetCheckpoint` agrees with `ListCheckpoints`

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_lifecycle.go`, `internal/events/checkpoint_lifecycle_test.go`
* **Change**: `GetCheckpoint` currently reports `valid: true` for any document that parses and
  validates (`internal/events/checkpoint_lifecycle.go:105-137`). After U4/U5 that is actively
  misleading: an agent running the canonical `list` → `get` → choose-verb sequence would read
  `needs_quarantine: true` from list and `valid: true` from get, then pick the verb the plan just
  closed. **The carrier is declared by U15** (`147.038-T`); this unit is the **production delta
  only** — it populates `Conforming`, `NeedsQuarantine`, `RemediationIntent`, and
  `NonConformingFields` on the result U15 declared. `valid` retains its existing meaning
  (schema-valid) and is **not** repurposed; conformance is reported as a distinct field so no
  existing consumer's contract silently changes. Schema-invalid documents keep returning
  `ErrCheckpointInvalid` — this unit adds conformance reporting for **valid-but-non-conforming**
  documents only. That sentinel is returned **unwrapped**: `GetCheckpointResult` does not wrap it
  in `ErrCheckpointUseQuarantine`, because a read is not a rewrite and there is nothing to refuse.
  Downstream surfaces must therefore expect the pre-existing validation-class refusal on `get`, not
  a disposition code.
* **Offending-field projection (cycle-16 addition, cycle-17 corrected to raw)**:
  `NonConformingFields` is the `BoundedFieldPathSet` recovered with `errors.As` from the conformance
  verdict and produced by **U1b's `BoundedFieldPaths()`** — never from the raw `Fields` slice and
  never re-capped here. Its `Paths` carry **raw** offender paths; quoting belongs only to U1c's
  human rendering. This is what makes `checkpoint get` an **atomic, bounded, per-file** offender
  source with machine-checkable truncation metadata.
* **Remediation is structured**: the result carries `RemediationIntent` (U1d), not a command
  string. Nothing in `internal/events` renders a shell command.
* **Tests** (3): a valid-but-non-conforming file returns `valid: true, conforming: false,
  needs_quarantine: true` with a `RemediationIntent` naming verb `quarantine` and approval class
  `A4c` **and** a `NonConformingFields.Paths` slice naming the offenders in raw, unquoted form; a
  conforming file returns `conforming: true`, a nil `RemediationIntent`, and an empty
  `NonConformingFields.Paths` with `Truncated: false`; the file is byte-unchanged after get.
* **Expected red**: cases 1 and 2 fail — U15's declaration wrapper leaves every projected field at
  its zero value, so `Conforming` is false for a conforming file and `NeedsQuarantine` is false for
  a non-conforming one. Case 3 is a declared regression guard (`GetCheckpoint` is already
  read-only). Verified with `go test -count=1 -run '^TestU6b_' ./internal/events`.
* **Consumed by**: U6c projects this result onto the MCP `backlogit_get_checkpoint` response; U8c
  (`147.027-T`) projects it onto `backlogit checkpoint get`.
* **Depends on**: U6 — the declared backlog edge (`147.012-T` -> `147.011-T`) — plus U15 (the
  carrier), U1b (the bounded raw projection), U1d (the intent), and U8b (the parity harness that
  must be red before this implementation lands).

### U6c — MCP `backlogit_get_checkpoint` projects the conformance verdict

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (mcp)
* **Files**: `internal/mcp/tools.go`, `internal/mcp/checkpoint_disposition_test.go`
* **Change**: `handleGetCheckpoint` (`internal/mcp/tools.go:1194-1212`) returns a **literal**
  `"valid": true` and carries no conformance field, so the MCP read surface would keep answering the
  superseded question after U6b lands. Call `events.GetCheckpointResult` and project `valid`,
  `conforming`, `needs_quarantine`, `remediation_intent`, and `non_conforming_fields` from the
  returned result; the hardcoded `"valid": true` is removed, not shadowed.
  `non_conforming_fields` is the **object** `{"paths": [...], "truncated": bool,
  "omitted_paths": int, "truncated_paths": int}` copied straight from
  `CheckpointReadResult.NonConformingFields`, which U6b already produced through U1b's bounded raw
  projection — this handler must not re-derive it from a typed error, must not re-cap it, and must
  not quote its entries, or the MCP surface would drift from the CLI for the same stored bytes.
  `remediation_intent` is the **structured** `RemediationIntent` object (U1d): the MCP surface
  publishes no shell string, so an agent cannot paste an unbound command out of a tool response
  (cycle-17, gate finding H1).
  Without this unit U7b's `backlogit_get_checkpoint`
  description and U8b's MCP `get` parity rows would describe behaviour no unit implements.
  **Schema-invalid documents keep their existing refusal.** `handleGetCheckpoint` routes errors
  through `domainError` (`internal/mcp/errors.go:148`), which takes no filename and already maps
  `ErrCheckpointInvalid` to `code: validation_failed`. U6b returns that sentinel unwrapped, so
  `get` on a legacy file surfaces the pre-existing validation-class refusal — **not**
  `checkpoint_use_quarantine`, which U7 only ever emits from `checkpointDispositionError` on the
  *mutation* handlers. Demanding a disposition code here would require re-routing the read handler
  through a mutation-shaped error path, widening this unit into U7's file set and changing a
  shipped read contract for no safety gain: the quarantine remedy is already discoverable from
  `backlogit_list_checkpoints`, which reports `needs_quarantine: true` for the same file (U6).
* **Tests** (3): a valid-but-non-conforming file returns `valid: true, conforming: false,
  needs_quarantine: true`, a `remediation_intent` object whose `verb` is `quarantine` and whose
  `approval_class` is `A4c`, and a `non_conforming_fields.paths` array matching the `events` result
  byte-for-byte with no added quoting; a conforming file returns `conforming: true`,
  `remediation_intent: null`, and `non_conforming_fields.paths: []` read through a `.([]any)` type
  assertion so an absent or `null` key fails
  (`docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md`); a schema-invalid
  file returns the pre-existing `validation_failed` refusal
  produced by `domainError` from an unwrapped `ErrCheckpointInvalid`, rather than a success payload
  asserting validity, asserted against the handler's actual payload.
* **Expected red**: cases 1 and 2 fail; case 3 is a declared regression guard pinning the shipped
  read contract. Verified with `go test -count=1 -run '^TestU6c_' ./internal/mcp`.
* **Depends on**: U6b (result type and projection), U8b (the parity harness that must be red
  first). The former dependency on U7 is removed — this unit pins the existing error mapping rather
  than consuming the new one.

### U7 — MCP error mapping and response shape

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (mcp)
* **Files**: `internal/mcp/errors.go`, `internal/mcp/checkpoint_disposition_test.go`
* **Change**: three owned defects in the mapping layer, plus one correction of record. The
  handler-routing half of the original U7 is now **U7d**, because it touches `internal/mcp/tools.go`
  and would have taken this unit to three files across two behavioural surfaces; the `domainError`
  rows are **U7e**.
  1. `checkpointDispositionError` (`internal/mcp/errors.go:309`) currently emits
     `checkpoint_use_quarantine` for the abandon/quarantine handlers only; add
     `ErrCheckpointNonConforming` with `Code: "checkpoint_non_conforming"`. Its existing
     `ErrCheckpointNotFound` → `NotFound` case and its `default: InternalError` tail both stay.
  2. The two existing response shapes are incompatible: `checkpointUnknownFieldsResponse`
     (`internal/mcp/errors.go:29-39`) carries `error`/`message`/`unknown_fields` while
     `checkpointDispositionErrorResponse` (`:291-299`) carries
     `error`/`message`/`code`/`filename`/`retryable`/`outcome`/`remediation`. Extend
     `checkpointDispositionErrorResponse` with
     `UnknownFields []string \`json:"unknown_fields"\``,
     `UnknownFieldsTruncated bool \`json:"unknown_fields_truncated"\``,
     `UnknownFieldsOmitted int \`json:"unknown_fields_omitted"\``, and
     `UnknownFieldsShortened int \`json:"unknown_fields_shortened"\`` — **none with `omitempty`** —
     populated via `errors.As` from **U1b's `BoundedFieldPaths()`**, so one refusal shape answers
     both "what went wrong" and "which keys". **`unknown_fields` carries RAW paths (cycle-17, gate
     finding H4)**: no `strconv.Quote` output and no synthetic `"+N more"` element ever appears in
     the array. Truncation is reported by the three sibling scalars, which are unambiguous and
     machine-checkable, and quoting is confined to U1c's human rendering. Copying the raw `Fields`
     slice here is still **forbidden** — that would leave the MCP payload unbounded while the CLI
     is bounded, which is the cross-surface drift R8 exists to prevent. Re-capping here is equally
     forbidden: the bound is a property of the error, not of a renderer.
  3. `domainError` (`internal/mcp/errors.go:148`) is the fallback surface for handlers that carry no
     filename. **This unit owns no `domainError` row and no mapping-table doc-comment change.**
     Cycle 16 audited the three rows cycle 15 had planned: `ErrCheckpointUseQuarantine` and
     `ErrCheckpointNonConforming` were **removed as unreachable** — after U7d, resolve reroutes
     every `QuarantineIsRemedy` match to `checkpointDispositionError`, and `get` never produces
     either sentinel — and only `ErrCheckpointCannotResolveAbandoned` remains, owned by **U7e**
     (`147.029-T`). Cycle-17 correction: earlier text in this unit described those two rows as
     "moved to U7e", which is stale — they were **deleted**, not relocated, and U7e never carried
     them. `ErrCheckpointInvalid` already maps to `validation_failed`
     (`internal/mcp/errors.go:~188-193`), grouped with `ErrValidation` and `ErrCheckpointCorrupt`,
     and a dedicated `ErrCheckpointUnknownField` case already precedes it; neither is touched here.
  4. The three disposition-class remediation strings — `checkpoint_use_quarantine`,
     `checkpoint_use_abandon`, and the new `checkpoint_non_conforming` — currently name a hardcoded
     originating verb (the shipped `checkpoint_use_quarantine` string is `"this target is
     malformed; call backlogit_quarantine_checkpoint instead of backlogit_abandon_checkpoint"`).
     After U7d routes `handleResolveCheckpoint` through the same formatter, that hardcode would
     make a resolve refusal advertise `backlogit_abandon_checkpoint` as the operator's original
     verb — a lie that would send the operator to the wrong entry point. The formatter already
     receives `op` (`"abandon checkpoint"`, `"quarantine checkpoint"`, and after U7d
     `"resolve checkpoint"`); derive the operator-facing verb from the first word of `op` and
     interpolate `backlogit_<verb>_checkpoint` into the "instead of" clause of each of the three
     disposition-class remediations. This keeps the formatter operation-aware without adding a new
     parameter, honours the width-isolation split (formatter ownership stays with U7 /
     `147.013-T`; handler-side assertions stay with U7d / `147.025-T`), and lets U7d assert on
     the `remediation` field without changing the shape or ownership boundary. The remedy verb
     itself (`quarantine` for the two "target is malformed / non-conforming" classes, `abandon`
     for the "target is valid" class) is unchanged; only the wronged verb becomes op-derived.
* **Tests** (3): `checkpointDispositionError` returns `checkpoint_non_conforming` for
  `ErrCheckpointNonConforming` and its `ErrCheckpointNotFound` → `NotFound` case still fires;
  invoking `handleAbandonCheckpoint` on a non-conforming target returns
  `checkpoint_non_conforming` with a populated `unknown_fields` read through a `.([]any)` type
  assertion so an absent or `null` key fails
  (`docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md`), whose entries are
  **raw and unquoted**, alongside `unknown_fields_truncated: false`, `unknown_fields_omitted: 0`,
  and `unknown_fields_shortened: 0`, and a
  `remediation` string naming `backlogit_abandon_checkpoint` as the originating verb (proving the
  op-derived interpolation is in place — the assertion holds because abandon is the caller here,
  but the formatter is not hardcoded to it, which U7d's resolve-side assertions confirm from the
  other direction); a conforming refusal returns `unknown_fields: []` with all three truncation
  scalars present rather than omitting any of the four keys.
* **Expected red**: all three fail. Verified with `go test -count=1 -run '^TestU7_' ./internal/mcp`.
* **Depends on**: U1, U1b (the bounded raw projection `unknown_fields` renders through), U3b, U4,
  U5, U8b (the parity harness that must be red first).

### U7d — `handleResolveCheckpoint` routes disposition refusals through the disposition shape

* **Domain**: code (mcp)
* **Files**: `internal/mcp/tools.go`, `internal/mcp/checkpoint_disposition_test.go` (2 files —
  `internal/mcp/errors.go` moved to U7e in cycle 14)
* **Change**: U7 extends `checkpointDispositionErrorResponse` with `code`, `filename`, and
  `unknown_fields`, but `handleResolveCheckpoint` (`internal/mcp/tools.go:1214-1232`) calls
  `domainError("resolve checkpoint", err)`. `domainError` takes **no filename argument**, so it can
  never populate `filename`, and it does not build the disposition response at all — no routing
  change means no `code`, no `filename`, and no `unknown_fields` on a resolve refusal, however
  complete the mapping table is. Route **by class**, using the U1 predicate — imported through the
  existing `backlogiterrors` alias `internal/mcp/tools.go` already carries for
  `github.com/softwaresalt/backlogit/internal/errors`, since U1 declares the predicate in that
  package (`internal/errors/checkpoint_errors.go`), not in `internal/events`:
  `if backlogiterrors.QuarantineIsRemedy(err) { return checkpointDispositionError("resolve checkpoint", filename, err) }`
  falling through to `domainError` otherwise. Passing the `op` string `"resolve checkpoint"` here
  is what the U7 op-derived remediation reads to render `backlogit_resolve_checkpoint` as the
  wronged verb rather than the hardcoded `backlogit_abandon_checkpoint` a shipped resolve refusal
  would otherwise advertise. A wholesale swap would regress the errors
  `checkpointDispositionError` does not name — `ErrCheckpointCannotResolveAbandoned` has no
  explicit case in `domainError` and currently falls to `default: InternalError`; its explicit
  `validation_failed` mapping is owned by **U7e**, not by this unit. `ErrCheckpointCorrupt` already
  maps
  to `validation_failed` through `domainError`. `ErrCheckpointNotFound` is safe either way: it is
  handled explicitly at `internal/mcp/errors.go:~358` before the default. The predicate matches both
  new refusals: U3 wraps the validity gate as
  `fmt.Errorf("%w: %w", ErrCheckpointUseQuarantine, valErr)` and U3b returns
  `ErrCheckpointNonConforming`, so `errors.Is` matches through the wrap.
* **Tests** (3): invoking the **`handleResolveCheckpoint` handler** (not the events function) on a
  schema-invalid legacy document returns `code: checkpoint_use_quarantine` with a populated
  `filename`, a `remediation` string naming `backlogit_resolve_checkpoint` (not
  `backlogit_abandon_checkpoint`) as the originating verb — pinning U7's op-derived interpolation
  from the resolve side, so the formatter cannot silently regress to a hardcoded wronged verb —
  and explicitly asserts the payload is **not** `"error":"internal"`; invoking it on a
  valid-but-non-conforming document returns `code: checkpoint_non_conforming` with `unknown_fields`
  non-empty and a `remediation` string naming `backlogit_resolve_checkpoint` as the originating
  verb; a missing file still returns the pre-existing not-found refusal.
* **Expected red**: cases 1 and 2 fail (routing and both remediation-verb assertions); case 3 is a
  declared regression guard.
* **Depends on**: U1 (the predicate and its host package), U7 (the response shape, the code, and
  the op-derived remediation the resolve-side assertions read), U8b (the parity harness).
* **Split note (cycle 14, corrected cycle 17)**: The
  `ErrCheckpointCannotResolveAbandoned` → `validation_failed`
  mapping and its handler-level test (original case 4) are owned by **U7e** (147.029-T) because
  they are a `domainError` mapping in `internal/mcp/errors.go` and keeping them here pushed this
  task to 3 files and 4 scenarios. The original plan text incorrectly called `validation_failed`
  "pre-existing" — `domainError` has no explicit case for `ErrCheckpointCannotResolveAbandoned`
  and falls to `default: InternalError`; the `validation_failed` mapping is a **genuine red delta**
  created by U7e. This task's file scope is now `internal/mcp/tools.go` +
  `internal/mcp/checkpoint_disposition_test.go` (2 files, 3 scenarios).

### U7e — `domainError` maps `ErrCheckpointCannotResolveAbandoned` to `validation_failed`

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (mcp)
* **Files**: `internal/mcp/errors.go`, `internal/mcp/checkpoint_disposition_test.go`.
* **Sole ownership (cycle-17 restatement)**: this unit owns **one** `domainError` row and nothing
  else. It does **not** own, retain, or reinstate any `ErrCheckpointUseQuarantine` or
  `ErrCheckpointNonConforming` mapping — cycle 16 **deleted** both as unreachable, and cycle 17
  removes every remaining sentence in this plan that described them as "moved to U7e" or as a
  "safety net". No unit in this plan may add either row. The reachability audit that produced the
  deletion is preserved below as the record of why.

  | Row | Sentinel | Reachable at `domainError`? | Disposition |
  |---|---|---|---|
  | 1 | `ErrCheckpointUseQuarantine` | **No** | **Deleted (cycle 16).** The only checkpoint handlers routing through `domainError` are `handleGetCheckpoint` (`internal/mcp/tools.go:1205`) and `handleResolveCheckpoint` (`:1224`). Get never produces this sentinel — U6b returns `ErrCheckpointInvalid` unwrapped — and after U7d resolve reroutes every `QuarantineIsRemedy` match to `checkpointDispositionError`. No live filename-less path reaches it. |
  | 2 | `ErrCheckpointNonConforming` | **No** | **Deleted (cycle 16).** Same two handlers. Get returns a *result* carrying `conforming: false` rather than this error; resolve is rerouted by U7d. |
  | 3 | `ErrCheckpointCannotResolveAbandoned` | **Yes** | **Owned by this unit.** `QuarantineIsRemedy` does not match it, so after U7d it still falls through to `domainError` at `internal/mcp/tools.go:1224` and still surfaces as `default: InternalError` — a 500 on a shipped agent-facing path. |

  Rows 1 and 2 were written as a "safety net for any handler that carries no filename". No such
  handler exists, and a mapping no code path can reach is a green-looking table entry that proves
  nothing. Adding cases for hypothetical future handlers is exactly the speculative construction
  the scope boundary forbids; if such a handler is ever added, its own unit adds its own mapping.
* **Change**: add one explicit `domainError` case —
  `ErrCheckpointCannotResolveAbandoned` → `validation_failed` — and add the corresponding row to
  the mapping-table doc comment (`internal/mcp/errors.go:127-144`). The classification is
  consistent with the neighbouring `ErrCheckpointInvalid` and `ErrCheckpointCorrupt` rows: the
  document exists and parses, and resolve is refused because of its administrative state.
* **Ordering constraint withdrawn with rows 1 and 2 (cycle 16)**: cycle 15 made "insert ahead of
  the combined `ErrValidation` / `ErrCheckpointInvalid` / `ErrCheckpointCorrupt` case" a mandatory
  correctness rule, because U3's multi-`%w` refusal satisfies both matchers and an appended
  `ErrCheckpointUseQuarantine` row would be permanently shadowed. That hazard belonged to row 1,
  which no longer exists. `ErrCheckpointCannotResolveAbandoned` is a bare `errors.New` value
  (`internal/errors/checkpoint_errors.go:59-64`) that wraps nothing, so it can collide with no
  other case and its placement is unconstrained. The Architecture P1 asking for a named ordering
  invariant is therefore **closed by removal**, not by adding an invariant that guards a row the
  plan no longer contains. Group the new case with the other checkpoint rows for readability.
* **Tests** (1, against a **realistically wrapped** error, never a bare sentinel): `domainError`
  maps `fmt.Errorf("resolve %s: %w", filename, ErrCheckpointCannotResolveAbandoned)` to
  `validation_failed` rather than `internal`.
* **Expected red**: the single case fails — `domainError` currently falls to
  `default: InternalError` for this sentinel. Verified with
  `go test -count=1 -run '^TestU7e_' ./internal/mcp`. Width: 2 files, 1 scenario. A unit below the
  four-scenario ceiling is compliant; the ceiling is a maximum, not a target.
* **Handler and CLI coverage**: the MCP handler-level assertion for abandoned-resolve lives in
  **U10c** (`147.041-T`), the owned bounded runtime unit added in cycle 17 — `backlogit_resolve_checkpoint`
  on an already-abandoned scratch fixture returns `validation_failed`, not `"error":"internal"`.
  The **CLI** twin — `backlogit checkpoint resolve`
  on an already-abandoned document — is deliberately **not** added to U8: U8 already carries three
  scenarios and a fourth would breach the granularity limit, and the CLI does not consume
  `domainError` at all, so no CLI behaviour changes here. It is recorded as follow-up stash
  `DBBA62AA` rather than left as an unstated gap.
* **Depends on**: U1, U8b.

### U7b — MCP read-surface tool descriptions (exact replacement strings)

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: docs (agent-facing tool contract)
* **Files**: `internal/mcp/tools.go` (descriptions at `:178` and `:189`, one line after each
  `mcplib.NewTool` name literal at `:177` and `:188`), `internal/mcp/tools_test.go`;
  `.autoharness/backlog-registry.yaml` and an `internal/cli/registry_parity_test.go` re-run only if
  the registry carries description text for these two tools
* **Change**: the **two read-surface** descriptions. The three mutation-surface descriptions moved
  to **U7c**: five rows in one unit exceeded the four-scenario limit and mixed a read contract with
  a refusal contract. This table is the single source of truth and is reproduced verbatim in
  `147.014-T`; the two must not drift. **Tool names are the registered `backlogit_*` identifiers
  (cycle-17, gate finding H6)** — verified against the `mcplib.NewTool` literals in
  `internal/mcp/tools.go` at `:177`, `:188`, `:195`, `:209`, and `:218`. A description that names
  a bare `quarantine_checkpoint` tells an agent to call a tool that is not registered.

  | Line | Registered tool | Delta |
  |---|---|---|
  | `:178` | `backlogit_list_checkpoints` | append: ` A summary with needs_quarantine true is not safely rewritable; use backlogit_quarantine_checkpoint, not backlogit_resolve_checkpoint or backlogit_abandon_checkpoint. Such a summary is returned regardless of the status, agent, shipment_id, feature_id, and max_age filters, so a filtered result can contain rows that do not match the filter. The accompanying remediation_intent is a structured record of the required disposition, not a runnable command.` |
  | `:189` | `backlogit_get_checkpoint` | append: ` For a schema-valid document, returns conforming false when it carries unmodeled top-level keys; such a document cannot be resolved or abandoned. non_conforming_fields carries raw offender paths with explicit truncation counts. A schema-invalid document is refused before any conformance verdict is produced.` |

  The `backlogit_get_checkpoint` qualifier is load-bearing: `GetCheckpoint` runs `ValidateCheckpoint`
  and returns `ErrCheckpointInvalid` before any conformance result exists
  (`internal/events/checkpoint_lifecycle.go:~105-137`), so an unqualified "returns conforming
  false for a document with unmodeled top-level keys" would promise a verdict the read path
  cannot produce for the nine legacy files. The `backlogit_list_checkpoints` filter sentence was
  added in PR #377 review cycle 4 and is why this unit depends on **U6d**: a published tool
  description must not promise an exemption no shipped code performs. The `remediation_intent`
  sentence was added in cycle 17 so the description does not imply a runnable string.
* **Tests** (2): a table-driven assertion over the two registered read descriptions, read from the
  **built tool set** rather than a duplicated literal, each row keyed by the registered
  `backlogit_*` name; and the existing registry-parity /
  fallback-map drift test
  (`docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`, Rule 1)
  re-run and staying green, with `.autoharness/backlog-registry.yaml` updated in the same commit
  if it carries description text for these tools.
* **Expected red**: both rows of case 1 fail. Case 2 is a declared regression guard.
* **Depends on**: U6b, U6c, U6d, U8b.

### U7c — MCP mutation-surface tool descriptions (exact replacement strings)

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: docs (agent-facing tool contract)
* **Files**: `internal/mcp/tools.go` (descriptions at `:196`, `:211`, and `:220`, one line after
  each `mcplib.NewTool` name literal at `:195`, `:209`, and `:218`), `internal/mcp/tools_test.go`;
  `.autoharness/backlog-registry.yaml` and an `internal/cli/registry_parity_test.go` re-run only if
  the registry carries description text for these three tools
* **Change**: the **three mutation-surface** descriptions. Split out of U7b. This table is the
  single source of truth and is reproduced verbatim in `147.024-T`; the two must not drift. **Tool
  names are the registered `backlogit_*` identifiers (cycle-17, gate finding H6)**, verified
  against the `mcplib.NewTool` literals cited above.

  | Line | Registered tool | Delta |
  |---|---|---|
  | `:196` | `backlogit_resolve_checkpoint` | append: ` Refuses a stored document it cannot safely rewrite rather than replacing it: checkpoint_use_quarantine when the document is schema-invalid, checkpoint_non_conforming when it carries unmodeled top-level keys. Use backlogit_quarantine_checkpoint instead.` |
  | `:211` | `backlogit_abandon_checkpoint` | append: ` Also refuses when the document carries unmodeled top-level keys.` |
  | `:220` | `backlogit_quarantine_checkpoint` | replace `malformed checkpoint file` → `checkpoint file that cannot be safely rewritten (malformed, schema-invalid, or carrying unmodeled top-level keys)` |

  The `backlogit_resolve_checkpoint` row promises two **codes**, which only reach that surface once
  U7d routes the handler through `checkpointDispositionError` — hence the dependency on U7d as well
  as on the U7 mapping.
* **Tests** (2): a table-driven assertion over the three registered mutation descriptions, read
  from the **built tool set**, each row keyed by the registered `backlogit_*` name, asserting its
  required substring and that
  `backlogit_resolve_checkpoint` distinguishes `checkpoint_use_quarantine` from
  `checkpoint_non_conforming`;
  and the registry-parity / fallback-map drift test re-run and staying green.
* **Expected red**: all three rows of case 1 fail. Case 2 is a declared regression guard.
* **Depends on**: U7, U7d, U8b.

### U8 — CLI refusal surfacing

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (cli)
* **Files**: `internal/cli/checkpoint.go`, `internal/cli/checkpoint_test.go`
* **Change (cycle-17 — no command string in this unit)**: surface the new refusals on
  `backlogit checkpoint resolve` and
  `backlogit checkpoint abandon` as actionable operator messages naming the offending keys through
  **U1c's `FieldPathsForDisplay()`** and stating the required disposition verb from the structured
  `RemediationIntent`. **Rendering an executable remediation command is not in this unit** — it is
  owned by **U16** (`147.039-T`), which binds the command to the resolved storage root and prints
  the A4c approval, preimage, and no-clobber preamble. This unit prints the refusal and the
  offender list; U16 prints the bound command block underneath it. Splitting them is the cycle-17
  resolution of gate finding H1: a refusal message that carries a paste-runnable command with no
  workspace binding and no approval obligation is the P0 the gate rejected.
  **The
  offending-key list is only available for a valid-but-non-conforming target.** A schema-invalid
  legacy document is refused by the U3 validity gate before conformance runs, so no key list
  exists for it; that refusal prints the validation reason and the required verb instead.
  **The `checkpoint get` conformance projection is not in this unit**: it moved to **U8c** in
  PR #377 review cycle 4, because a read projection is a different contract from a refusal
  rendering and folding it in made a fourth scenario. This unit must not touch
  `newCheckpointGetCmd`. **No JSON error envelope is added** — that is stash `63E810D9` and stays
  out of scope; the CLI/MCP shape asymmetry it describes is a documented, pre-existing condition
  recorded in **Follow-ups recorded (not in scope)** under Runtime Verification and Closure.
  *(Cycle-15 correction: this sentence previously claimed the asymmetry was "restated in U9b".
  U9b's section and `147.018-T` contain no such restatement, so the cross-reference pointed at
  nothing. Repaired by naming the record that actually exists rather than by growing U9b, which is
  already the widest docs unit in the plan and carries the hard merge gate.)*
* **Tests** (3): `checkpoint resolve` on a **schema-invalid legacy** document exits non-zero,
  reports the `checkpoint_use_quarantine` class with the validation reason and names `quarantine`
  as the required verb, and prints **no** key list; `checkpoint resolve` on a
  **valid-but-non-conforming**
  document exits non-zero and names the offending top-level keys in quoted, bounded form; `checkpoint
  abandon` on that same valid-but-non-conforming document does likewise. No case asserts a
  paste-runnable command — that assertion belongs to U16. The named key list in cases 2 and 3 is
  rendered from **U1c's `FieldPathsForDisplay()`**, whose bound comes from U1b, so the CLI text and
  the MCP `unknown_fields` array describe the same offender set for the same stored bytes while
  differing only in escaping.
* **Expected red**: all three fail. Verified with
  `go test -count=1 -run '^TestU8_' ./internal/cli`.
* **Not in scope (cycle 16)**: a CLI assertion for `checkpoint resolve` on an **already-abandoned**
  document. That path does not consume `domainError`, so U7e changes nothing about it, and a fourth
  scenario would breach the granularity limit. Recorded as follow-up stash `DBBA62AA`.
* **Depends on**: U7, U1c (the human rendering the key list uses), U8b.

### U16 — CLI-boundary remediation command rendering

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (cli)
* **Files**: `internal/cli/checkpoint.go`, `internal/cli/checkpoint_remediation_test.go` (new)
* **Change (cycle-17 — the safe half of gate finding H1)**: the CLI is the **only** surface allowed
  to render an operator-runnable command, because it is the only layer that knows the resolved
  workspace. Add a single renderer that consumes `events.RemediationIntent` (U1d) and emits a
  bound, approval-gated block wherever a refusal or a `needs_quarantine` summary is printed:

  ```text
  Disposition required: quarantine
    Workspace : <resolved storage root, absolute>
    Target    : <bare filename>
    Approval  : A4c — operator approval is REQUIRED immediately before execution
    Preimage  : take a byte copy of the target before running the command
    Destination: archive/checkpoints/<bare filename> must be ABSENT (no-clobber)
  Command (run only after approval):
    backlogit --cwd <resolved storage root parent> checkpoint quarantine <bare filename> --operator <you> --reason "<why>"
  ```

* **Binding rules (all mandatory)**:
  1. the rendered command **always** carries an explicit `--cwd` bound to the workspace the intent
     was computed for; a command without `--cwd` is forbidden, because an ambient cwd is exactly
     what made the cycle-16 form unsafe;
  2. the argument is the **bare filename** from `RemediationIntent.TargetFilename`, never a path
     and never a concatenation;
  3. the A4c approval line, the preimage line, and the no-clobber destination line are part of the
     block and cannot be suppressed by a flag;
  4. when the resolved workspace path or the filename contains a character the target shell would
     treat specially, the renderer emits the block **without** the command line and prints
     `command not rendered: workspace or filename requires manual quoting` instead. Refusing to
     render is the safe failure mode; emitting a half-quoted command is not. The natural corpus
     never hits this branch — `CreateCheckpoint` (`internal/events/memory.go:59`) only ever writes
     `checkpoint-YYYYMMDD-HHMMSS.json` — so the branch exists for out-of-band files only.
* **Explicitly not claimed**: no cross-shell paste-safety claim is made. The block is rendered for
  the operator to read and adapt; the approval step is a human step by construction, so a command
  that needs hand-adaptation on one shell costs nothing this plan needs.
* **Tests** (3): a rendered block for a valid-but-non-conforming target contains an absolute
  `--cwd`, the bare filename, and all three of the approval, preimage, and no-clobber lines; a
  target whose filename contains a shell metacharacter renders the block with the
  `command not rendered` line and **no** command; the renderer emits nothing at all when
  `RemediationIntent` is nil, so a conforming file's output is unchanged.
* **Expected red**: cases 1 and 2 fail (no renderer exists). Case 3 is a declared regression guard
  for the conforming path.
* **Depends on**: U1d, U8, U8b.

### U8c — CLI `checkpoint get` projects the conformance verdict

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (cli)
* **Files**: `internal/cli/checkpoint.go`, `internal/cli/checkpoint_test.go`
* **Change**: `newCheckpointGetCmd` (`internal/cli/checkpoint.go:180-210`) calls
  `events.GetCheckpoint` and prints a **literal** `"valid": true` with no conformance field, so the
  CLI read surface would keep answering the superseded question after U6b lands. Call
  `events.GetCheckpointResult` and project `valid`, `conforming`, `needs_quarantine`,
  `remediation_intent`, and `non_conforming_fields`; the hardcoded literal is removed, not
  shadowed. `non_conforming_fields` is copied from `CheckpointReadResult.NonConformingFields`
  unchanged — already bounded by U1b through U6b — so CLI and MCP cannot render different offender
  lists for the same file. This is the CLI twin of
  U6c and takes the same shape. It is a **separate unit from U8** — added in PR #377 review cycle
  4 — because U8 owns refusal rendering on `resolve` / `abandon`, already carries three scenarios,
  and a fourth would breach the granularity limit; the two units share
  `internal/cli/checkpoint.go` but not a function, exactly as U6c and U7c share
  `internal/mcp/tools.go`. Without this unit, U8b's `valid-but-non-conforming` parity row — "CLI
  `checkpoint get` → the same" — asserts behaviour no unit implements.
  **Schema-invalid documents keep their existing refusal**: U6b returns `ErrCheckpointInvalid`
  unwrapped from `GetCheckpointResult` and the CLI already exits non-zero on it, so `get` on a
  legacy file surfaces the pre-existing validation-class refusal rather than a disposition code.
  This unit **pins** that behaviour rather than changing it.
* **Tests** (3): a valid-but-non-conforming file reports `valid: true`, `conforming: false`,
  `needs_quarantine: true`, a `remediation_intent` naming verb `quarantine` and approval class
  `A4c`, and a `non_conforming_fields.paths` list
  identical to the MCP projection for the same stored bytes; a conforming file reports
  `conforming: true`; a schema-invalid file exits non-zero with the pre-existing validation-class
  refusal rather than a success payload asserting validity, asserted against the command's actual
  output rather than a literal. The "literal `"valid": true` is gone" assertion rides on cases 1
  and 2 rather than being a fourth scenario.
* **Expected red**: cases 1 and 2 fail; case 3 is a declared regression guard pinning the shipped
  read contract. Verified with `go test -count=1 -run '^TestU8c_' ./internal/cli`.
* **Depends on**: U6b, U8b.

### U8b — Cross-surface parity harness (RED) from one stored state

* **Partition**: 3 (declarations and genuine RED harness order)
* **Domain**: tests
* **Files**: `internal/cli/checkpoint_parity_test.go` (new). **No production change.**
* **Ordering contract (cycle-17 — closes gate finding H2)**: this harness lands **after** the
  partition-1 and partition-2 declarations and **before** every partition-4 implementation. Its
  dependencies are declarations only — U1, U1b, U1d, U2, U15 — and the eleven behavioural units it
  pins (U3, U3b, U4, U5, U6, U6b, U6c, U7, U7d, U8, U8c) each depend on **it**, so the harness is
  observably red before any of them lands. Cycle 15 and cycle 16 wired the reverse direction, which
  is why the gate ruled the RED premise unsatisfiable: a harness that depends on the
  implementations it pins can never be red against a pre-implementation state.
* **Change**: `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`
  Rule 3 requires that a guard spanning CLI and MCP exercise **both surfaces from the same stored
  state**. U6/U6b/U6c/U7/U7b/U8/U8c each build their own fixtures, so they could drift apart and
  still all pass. Add one shared fixture table — `legacy-shaped` (schema-invalid **and**
  non-conforming), `valid-but-non-conforming`, `conforming-active` — written once per case, then
  driven through the CLI command handler **and** the MCP handler **and** the `events` read layer
  against that same file, asserting the refusal classification agrees. Shape differences (exit code
  + text vs JSON payload) are expected; the **classification** must not differ. A schema-invalid
  document never reaches a conformance verdict, so the `conforming: false` assertions belong to the
  `valid-but-non-conforming` row **only**; the `legacy-shaped` row asserts the **validation-class**
  refusal on `get` — `events.ErrCheckpointInvalid`, MCP `code: validation_failed` (U6c), CLI
  non-zero exit — instead of a success payload, while `checkpoint_use_quarantine` belongs to the
  `resolve` column, where U7d routes it.
* **Tests** (3): one row per fixture shape, each asserting CLI, MCP, and `events` reach the same
  accept/refuse verdict and the same remedy verb. Byte-identity postcondition applies only to
  refused mutation paths; accepted mutations (conforming-active row's `abandon`) verify the
  intended rewrite/archive outcome using a fresh fixture copy.
* **Harness posture (cycle-17 replacement for the batch-harness model)**: U8b compiles against the
  partition-3 declaration `events.GetCheckpointResult` / `CheckpointReadResult` (U15), the
  partition-1 carriers `BoundedFieldPathSet` (U1b) and `RemediationIntent` (U1d), and the
  already-shipped CLI and MCP handlers. Every symbol it references exists at the moment it lands,
  so it **compiles**; every projection and refusal it asserts is unimplemented, so it **fails on
  assertions**. That is the two-step red posture applied across surfaces instead of inside one
  package. The cycle-15/16 "batch harness generation phase" framing is withdrawn: it made the red
  gate depend on when an implementer chose to generate harnesses rather than on the dependency
  graph.
* **Already-green assertions removed (cycle-17, gate finding H2)**: cycle 16 claimed the
  `legacy-shaped` row's `get` assertions were red. They were not. `GetCheckpoint` already runs
  `ValidateCheckpoint` and already returns `ErrCheckpointInvalid` for a schema-invalid document
  (`internal/events/checkpoint_lifecycle.go:~105-137`), `domainError` already maps that sentinel to
  `validation_failed` (`internal/mcp/errors.go:~188-193`), and the CLI already exits non-zero on
  it. Those three assertions are **declared regression guards**, green from the moment they land,
  and they are removed from this unit's red gate rather than being counted as failures that never
  occur.
* **Expected red** (against the partition-3 declaration state, per row):

  * **`legacy-shaped` row**: the three `get` assertions —
    `errors.Is(events.GetCheckpointResult(...), ErrCheckpointInvalid)`, MCP
    `backlogit_get_checkpoint` → `code: validation_failed`, and CLI `checkpoint get` → non-zero
    exit — are **declared regression guards**, green on landing, and are excluded from the red
    gate. The row's single **RED** assertion is `resolve`: it currently succeeds and rewrites the
    file with a fabricated skeleton (F2 in memory) before U3's validity gate and U7d's routing
    land, so the assertion that resolve is refused with `checkpoint_use_quarantine` **fails** on
    all three surfaces.
  * **`valid-but-non-conforming` row**: `events.GetCheckpointResult` — the U15
    declaration returns a result whose projected fields are all zero-valued; assertions
    `result.NeedsQuarantine == true`, `result.RemediationIntent != nil`,
    and `result.Conforming == false` **fail** on the first two, because the declaration yields
    `NeedsQuarantine: false` and `RemediationIntent: nil` — where `Conforming == false` alone is
    ambiguous (it matches the zero value), the positive-projection assertions
    (`NeedsQuarantine == true`, `RemediationIntent != nil`, `NonConformingFields.Paths` non-empty)
    prove the declaration is unpopulated. MCP `backlogit_get_checkpoint` — before U6c's
    projection lands, the response has no `conforming` field; assertion that
    `conforming: false` appears in the payload **fails**. CLI `checkpoint get` — before
    U8c/147.027-T reprojects `newCheckpointGetCmd` from `events.GetCheckpointResult`, the CLI
    prints hardcoded `"valid": true` with no conformance field; assertion that `conforming:
    false` appears in stdout **fails**. `resolve` and `abandon` — before U3b's conformance gate
    and U4's conformance gate land, both mutations succeed on a valid-but-non-conforming
    document; assertions of refusal with `checkpoint_non_conforming` **fail** on both surfaces.
  * **`conforming-active` row**: every surface accepts `abandon` — this is pre-existing shipped
    behaviour and passes from the moment the assertion lands. The three `get` assertions
    (`events.GetCheckpointResult` → `conforming: true`, MCP `backlogit_get_checkpoint` →
    `conforming: true`, CLI `checkpoint get` → `conforming: true`) are **RED** until U6b/U6c/U8c
    land: the U15 declaration yields `Conforming: false`, the
    MCP payload lacks a `conforming` field before U6c, and the CLI prints hardcoded
    `"valid": true` with no conformance field before U8c. Only accepted `abandon` is an
    already-green regression guard.
  * **Byte-identity postcondition**: applies **only** to refused-mutation paths (rows 1 and 2's
    non-mutation surfaces exercised against the canonical fixture directly). The
    `conforming-active` row's accepted `abandon` necessarily mutates its fresh fixture and
    asserts the intended rewrite/archive outcome, not byte identity. Against the current
    `ResolveCheckpoint` — which rewrites on the `valid-but-non-conforming` row — the assertion
    **fails** for row 2's fixture.

  Rows 1 and 2 carry this unit's red gate; row 3's three `get` assertions add to it, and row 3's
  accepted `abandon` plus row 1's three `get` assertions are the parity/regression guards that pin
  already-shipped behaviour. This unit claims no all-guards exemption — rows 1, 2, and 3 each
  produce concrete assertion failures against the partition-3 declaration state, and P-004 is
  satisfied by their aggregate red load.

* **Parity/regression-guard role after impls land**: once U3, U3b, U4, U5, U6, U6b, U6c, U7,
  U7b, U7c, U7d, U8, U8c have landed in dep order, the assertions above turn green: every
  surface projects the same classification for the same stored bytes. U8b's role at that point
  is exactly the parity contract it has always been — pin the agreement so a future regression
  in any one surface (a stale MCP projection, a CLI hardcode, an events-layer default) surfaces
  as a failing test instead of a silent drift.

* **Depends on**: U1, U1b, U1d, U2, U15 — declarations only. **Depended on by**: U3, U3b, U4, U5,
  U6, U6b, U6c, U7, U7d, U8, U8c.

### U9 — Design doc: total classification

* **Domain**: docs
* **Files**: `docs/design-docs/checkpoint-administrative-disposition.md`, regenerated
  `docs/cli-reference/backlogit_checkpoint_*.md`
* **Change**: restate the "Malformed-Only vs Valid-Only Split Rationale" section as a
  **state-scoped four-class** classification. For a `status: "active"` target with no administrative disposition:

  | Class | `abandon` | `resolve` | `quarantine` |
  |---|---|---|---|
  | valid + conforming | accept | accept | refuse (`ErrCheckpointUseAbandon`) |
  | valid but non-conforming | refuse (`ErrCheckpointNonConforming`) | refuse (`ErrCheckpointNonConforming`) | **accept** |
  | parses but schema-invalid | refuse (`ErrCheckpointUseQuarantine`) | refuse (`ErrCheckpointUseQuarantine`) | accept |
  | does not parse | refuse (`ErrCheckpointUseQuarantine`) | refuse (`ErrCheckpointUseQuarantine`) | accept |

  The third row must be written out separately rather than folded into "malformed": it is the
  exact shape of the nine live legacy files — they parse cleanly and fail `ValidateCheckpoint` —
  and a three-class table that only names parse failure is **not total** over active checkpoints.
  State explicitly that non-`active` states (`ErrCheckpointNotActive`) are a separate, pre-existing
  class **not** addressed here, so the totality claim is not overstated. Document the
  `ResolveCheckpoint` validity and conformance gates as a behaviour change, note the named
  already-resolved residual from U3b, and record that the top-level namespace is closed in both
  directions with `context` remaining the sole open one. Regenerate CLI reference docs via
  `gen-docs` if any command help text changed.
* **Tests**: `backlogit docs lint` reports 0 violations; the CLI Reference Drift check is clean.
* **Expected red**: n/a (docs-only; runs after behaviour is final and claims no RED behaviour).
* **Depends on**: U2e, U6b, U8b, U16, U17.

### U9b — Agent instruction file and quarantine-first disposition guidance

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: docs (agent-facing)
* **Files**: `.github/instructions/backlogit.instructions.md`
* **Change (cycle-16 rewrite — quarantine-only)**: the Lifecycle Hygiene Protocol currently teaches
  every agent in this workspace (`applyTo: "**"`) that abandon and quarantine are disjoint by
  validity alone and treats `resolve` as infallible. After U3/U3b/U4/U5 that guidance is wrong at
  exactly the moment an agent needs it — session-start recovery. Update it to state that `resolve`
  may now return `checkpoint_use_quarantine` or `checkpoint_non_conforming`, that the remedy is
  **quarantine** rather than retry, and that the disjointness discriminator is validity **and**
  top-level conformance.

  **The paste-runnable hand-repair and restore procedure published through cycle 15 is withdrawn.**
  Cycles 10-15 grew a two-entry-point runbook telling an operator to rename archived evidence
  aside, copy preserved bytes back into the active directory, hand-edit JSON members in place, and
  conditionally delete the restored copy on abort. The cycle-15 security review rejected publishing
  it, and the rejection holds: that procedure performs rename, copy, in-place rewrite, and
  conditional delete against a directory this codebase does not open with real-root / no-follow
  semantics (stash `35A27CD0`), with no handle-or-content compare-and-swap between the read that
  classified the file and the write that repairs it, and no adversarial coverage for a symlinked or
  concurrently-replaced path. Publishing it as copy-and-run instructions in a file that applies to
  **every** agent in the workspace claims a safety property this release cannot guarantee.
  Withholding it costs nothing this plan needs: refusal plus verbatim-move is already a complete
  and reversible disposition. The runbook is deferred to `35A27CD0` rather than weakened, and any
  future version must carry real-root / no-follow open semantics, handle-or-content CAS,
  no-clobber destinations, and adversarial tests.

  **What this unit publishes instead.**

  1. **The refusal contract.** `resolve` and `abandon` refuse a stored document that is
     schema-invalid or non-conforming; `quarantine` accepts it and moves the bytes verbatim into
     `archive/checkpoints/` with a `<filename>.disposition.json` sidecar. Within `status: "active"`
     exactly one verb accepts any given file.
  2. **The offender-diagnosis step, with its exact source.** Offending members are read from
     `checkpoint get` — the `non_conforming_fields` projection declared by U15, populated by U6b,
     and surfaced by U6c (MCP) and U8c (CLI). It is atomic, per-file, and bounded, its `paths`
     carry **raw** offender key paths, and its `truncated` / `omitted_paths` / `truncated_paths`
     scalars say explicitly when the list is incomplete. Explicitly forbid the two wrong
     sources: the decoded `checkpoint` object in the same payload, because the collapsing decode has
     already destroyed the duplicate members and they are not in it; and a whole-directory
     `checkpoint list` scan, which is neither atomic nor per-file.
  3. **A diagnostic classification table.** It records what each offender code means and whether the
     class is even theoretically relocatable. It is diagnostic, not a runbook — every row's
     disposition in this release is quarantine.

     | Reported offender | Meaning | Disposition in this release |
     |---|---|---|
     | `<key>` | a plain unmodeled top-level key | Quarantine. The class is theoretically relocatable, but automated relocation ships with `35A27CD0`. |
     | `duplicate:<key>` | two top-level members whose keys are `strings.EqualFold`-equal, at least one of which is modeled or collapses into a modeled slot | Quarantine. Any repair requires an explicit raw-token-aware operator selection of the surviving member, made outside agent automation. |
     | `duplicate:context.<key>` | an exact-duplicate `context` member, or a fold variant aliasing `shipment_id`, `feature_id`, `task_ids`, or `branch` (U2g) | Quarantine. Same operator-selection requirement. |
     | `duplicate:progress.<key>` | a duplicate or fold-variant member inside the modeled `progress` object (U2e) | Quarantine. No top-level container can relocate it. |

  4. **The survivor-selection rule, with no spelling-based exception.** Every duplicate equivalence
     class that touches a modeled field requires an explicit, recorded, raw-token-aware operator
     selection of the surviving member, or quarantine. There is **no** automatic survivor and **no**
     "move the unmodeled variant and leave the modeled one in place" shortcut. Cycle 15 published
     exactly that shortcut for `duplicate:<key>` with one modeled member; cycle 16 removes it. A
     spelling-based rule silently decides which of two on-disk values is authoritative, which is the
     information-destroying choice this feature exists to refuse.
  5. **The scoped no-implicit-survivor invariant.** State it as: *within the stored-checkpoint
     administrative-disposition read and rewrite surfaces this work governs — `resolve`, `abandon`,
     `quarantine`, `list`, and `get` — no duplicate member is ever silently collapsed; a document
     containing one is refused or quarantined.* Cycle 15 asserted the rule "universally", at every
     level, "whether at the top level, inside `context`, inside `progress`, or inside
     `legacy_top_level` itself". That claim is **false at the create boundary**, which still parses
     caller-supplied bytes and re-marshals them and can still collapse duplicate `context` members
     before anything reaches disk. Create-boundary hardening is deliberately out of scope — it is
     146-F's shipped surface and no pre-existing on-disk evidence is at risk there — and is deferred
     under stash `E429A031`. The instruction file must say so rather than publish a universal
     guarantee the code does not provide.
  6. **The operator-decision framing for quarantine itself.** Quarantining a checkpoint whose
     `status` is `active` moves live recovery state into the archive. It is a deliberate operator
     decision, not an automatic agent reflex, and **no automatic session-start repair, restore, or
     quarantine sweep of live checkpoints may be prescribed**. Session-start recovery reads
     `checkpoint list` and `checkpoint get`; any mutating disposition against
     `.backlogit/checkpoints/` is action A5 and requires its own explicit operator approval.
  7. **The recovery direction, stated without executable steps.** Quarantine is reversible in
     principle: the archived bytes and their sidecar are the verbatim pre-quarantine record, and an
     operator may restore them by hand. This release does **not** publish, automate, or verify that
     restore. Name `35A27CD0` as its home and state the two properties any future restore must
     carry — no-clobber on every destination, and evidence never left live under the same name in
     both `.backlogit/checkpoints/` and `archive/checkpoints/`, because `CleanupCheckpoints` calls
     `os.Remove(dst)` before its rename on Windows
     (`internal/events/checkpoint_lifecycle.go:238-242`) and would destroy the quarantined copy.
  8. **The create-time scope repair.** Continuity Protocol §6 currently tells agents that a rejected
     create "means retry with the offending keys nested under context… there is nothing to recover".
     That is correct for **create** and wrong for a **stored legacy** document, whose keys are
     already on disk. Scope §6 explicitly to create.
  9. **No executable remediation text (cycle-17, closes gate finding H1).** This file MUST NOT
     contain a paste-runnable disposition command, a repair procedure, a restore procedure, or a
     cleanup sweep. It states the required **verb** and points at the structured
     `remediation_intent` record (`verb`, `target_filename`, `requires_approval`, `approval_class`,
     `reason`) that `checkpoint list` and `checkpoint get` return. The only surface that renders a
     runnable command is the CLI (U16), which binds it to the resolved workspace with an explicit
     `--cwd` and prints the A4c approval, preimage, and no-clobber preamble with it. An instruction
     file that applies to **every** agent in this workspace (`applyTo: "**"`) cannot publish a
     command whose meaning depends on whatever directory the reader happens to be in, and cannot
     publish one without the approval obligation attached. Any acceptance check for this unit
     greps the changed file for `backlogit checkpoint quarantine` and fails if a bare invocation
     is present.
* **Tests**: `backlogit docs lint` reports 0 violations on the changed file.
* **Acceptance / merge gate**: this unit and U9 must land in the **same merge commit** as U3b/U4/U5.
  A pull request that includes any of U3b, U4, or U5 **MUST NOT be merged** unless the
  `.github/instructions/backlogit.instructions.md` delta from this unit is present in that same
  merge commit. This is a merge-checklist item, not a recommendation: shipping the behaviour change
  without the instruction update would leave every agent in the workspace following superseded
  session-start recovery guidance at exactly the moment the guidance becomes wrong.
  **Scope clarification (cycle 15)**: this constrains the implementation units **to each other
  inside one pull request**, not the backlog tasks to separate merges. Ship builds one release-unit
  branch and one PR for shipment `130-S`, so the natural outcome is a single merge commit carrying
  all of them, and the gate simply forbids deferring the U9b delta to a later PR. It does **not**
  require, imply, or permit a merge per backlog task.
* **Depends on**: U9.

### U10 — Runtime verification of the refusal path

* **Partition**: 5 (runtime verification and closure)
* **Domain**: verification
* **Files**: `.gitignore` (scratch-directory ignore rule); otherwise none (produces `docs/closure/`
  evidence)
* **Change**: none to product code. Build the binary **from the branch under test** (not the pinned
  repo-root `backlogit.exe`, which predates the change) and exercise the **refusal** path against a
  **scratch** workspace seeded with copies of the legacy document shapes. The acceptance and
  evidence-integrity rows moved to **U10b**: five rows exceeded the four-scenario limit. The nine
  live files under `.backlogit/checkpoints/` are read for shape reference only and are never
  mutation targets (R6); the check is a programmatic before/after SHA-256 comparison of **every**
  file under `.backlogit/checkpoints/`, not a visual one, and not a count-pinned subset — twelve
  files are present on this branch now that the staging checkpoint has landed, and that number
  drifts as sessions add checkpoints, so the guard enumerates the directory rather than a literal.
* **Binary provenance (cycle-16 correction)**: build with the repository's own version ldflags —
  the `LDFLAGS` variable in `Makefile:5-8`, whose shape the release workflow reproduces at
  `.github/workflows/release.yml:99-107`:

  ```text
  go build -ldflags "-X github.com/softwaresalt/backlogit/internal/version.Version=verify-<short-sha> -X github.com/softwaresalt/backlogit/internal/version.Commit=<short-sha> -X github.com/softwaresalt/backlogit/internal/version.BuildDate=<rfc3339>" -o docs/scratch/checkpoint-verification/backlogit-verify.exe ./cmd/backlogit
  ```

  where `<short-sha>` is `git rev-parse --short HEAD`, matching the Makefile's `COMMIT` derivation.
  Then assert that
  `docs/scratch/checkpoint-verification/backlogit-verify.exe version --format json --no-update-check`
  reports a `commit` field **equal to that `<short-sha>`**. The flags are exact:
  `--format json` is the JSON selector (`internal/cli/version_cmd.go`) — there is no `--json` flag —
  and `--no-update-check` suppresses the remote release lookup so the assertion is hermetic. A plain
  `go build ./cmd/backlogit` leaves `internal/version.Version` at its `DevVersion` default with an
  **empty** commit and proves nothing about provenance.
* **Execution contract (cycle 16, corrected cycle 17 — mandatory, fail-closed)**: every command in
  this unit MUST
  1. **run under an explicit A4c approval batch** obtained immediately before execution, naming the
     files and the directory it will touch;
  2. **bind to the canonical workspace** with `--cwd docs/scratch/checkpoint-verification` and pass
     **bare filenames**, never paths, and log the resolved storage root before the first mutating
     verb;
  3. **display, per file, before and after each step**: the bare `filename`, its SHA-256 `hash`,
     its classified `state` (`valid` / `conforming` / `needs_quarantine`), and the `destination`
     path the step will write or move to;
  4. **take a pre-run byte copy (preimage)** of every fixture a mutating verb will touch, as a
     precondition of the approval request;
  5. **apply the precondition that matches the operation class** — the two classes are different
     and cycle 16 conflated them:
     * **in-place rewrite** (`resolve`, `abandon`): the destination **is** the target file and is
       necessarily present. Absent-destination and no-clobber are **not** applicable and must not
       be asserted. The required preconditions are the preimage byte copy from step 4, the
       guarded-seam verdict (parse, validate, conformance), and a post-step SHA comparison that is
       *equality* on a refusal and *inequality plus a re-parse that validates* on an acceptance.
     * **archive move** (`quarantine`): the destination is a new path under `archive/checkpoints/`.
       The destination MUST be asserted absent before the move and the move MUST refuse to clobber
       an occupied destination, matching the shipped `moveNoReplace` semantics.
     * **intentional-collision tests** are the deliberate exception: a row that exists to prove
       no-clobber *must* create the collision first. Such a row declares itself as an intentional
       collision, and its expected outcome is a **refusal**, not a successful move.
  6. **fail closed**: a destination outside the bound root, a missing preimage, an occupied
     destination on a move that is not an intentional-collision row, or an unclassifiable state is
     a **halt**, not a warning, and not a retry.
* **Rows** (3): **read verdict** — `list` reports `needs_quarantine: true` with a structured
  `remediation_intent` naming verb `quarantine` and approval class `A4c` (and **no** shell string),
  and `get` reports `conforming: false` **and a `non_conforming_fields.paths` list naming the
  offenders in raw form** for the same fixture;
  **refusal** — legacy-shaped resolve refused and valid-but-non-conforming abandon refused naming
  keys, with the fixture bytes unchanged after both (in-place class: SHA equality); **quarantine
  accept** — quarantine accepted into an asserted-absent destination and the archived bytes
  byte-identical to the pre-quarantine original (archive-move class).
* **Evidence persistence (cycle-17)**: each row writes a deterministic, human-readable record to
  `docs/closure/2026-08-checkpoint-disposition-runtime-verification.md` — a tracked file — carrying
  per-file `filename`, `sha256`, `state`, `destination`, and outcome. The scratch directory stays
  git-ignored and is **not** the evidence of record: an ignored, machine-local artifact cannot be
  reviewed and does not survive teardown. Cycle-17 makes the closure file the artifact and the
  scratch directory a working area.
* **Scratch containment**: the scratch workspace is created **inside the repository working tree**
  at `docs/scratch/checkpoint-verification/` (never `%TEMP%`, never outside the cwd — Constitution
  IV), the resolved path is asserted to be repo-root-relative **before the first write**, it is
  added to the freeze-scope declaration, and — because `.gitignore` carries no `docs/scratch/` rule
  today (`*.exe` already covers the built binary, the copied fixtures are not covered) — adding that
  ignore rule is owned by this unit. U10b and U10c inherit all of it. **Teardown does not run
  here**: U10b and U10c consume this unit's quarantine archive, fixtures, and branch-built binary,
  so the workspace is handed over intact and teardown ownership moves to **U10c** (cycle-17; it was
  U10b through cycle 16). It stays classified `ActionRisk: destructive` (A4b) requiring
  operator approval (Constitution VII) at the point U10c performs it. If approval is not granted
  the directory is left in place and recorded as a cleanup follow-up.
* **Depends on**: U9b.

### U10b — Runtime verification of acceptance, evidence integrity, and the recovery sweep

* **Partition**: 5 (runtime verification and closure)
* **Domain**: verification
* **Files**: none. Runs entirely inside the scratch workspace U10 created; no repository file
  changes.
* **Change**: none to product code. **The recovery sweep runs against a scratch mirror, never the
  live directory.** U10 requires every file under `.backlogit/checkpoints/` to be byte-unchanged,
  while the nine-file acknowledgement requires a sweep that "succeeds on every other file in that
  directory" — a real `resolve` against the conforming active checkpoints there. The two cannot
  both hold against the live corpus, so the sweep operates on a **copied mirror** inside the
  scratch workspace (workspace root: `docs/scratch/checkpoint-verification/mirror/`; checkpoints go
  into `docs/scratch/checkpoint-verification/mirror/.backlogit/checkpoints/`). All sweep CLI
  invocations use `--cwd docs/scratch/checkpoint-verification/mirror` with bare filename
  arguments. The nine enumerated legacy
  filenames keep their names in the mirror, so the discrimination assertion is unchanged while the
  live bytes stay read-only.
* **Execution contract**: U10's six-point contract (A4c approval batch, `--cwd` binding with bare
  filenames, per-file filename/hash/state/destination display, preimage byte copy, absent-destination
  and no-clobber assertion, fail-closed halts) applies unchanged to every row here, against both the
  scratch root and the mirror root.
* **Rows** (3): **acceptance is not over-refused** — a conforming active fixture is accepted by
  abandon and a second conforming active fixture is accepted by resolve; **quarantine evidence
  integrity** — the archive U10 row 3 produced is verified as a verbatim record whose **payload**
  move is no-clobber: the archived bytes are SHA-identical to the preimage, the
  `<filename>.disposition.json` sidecar names the original filename, and a **second** quarantine of
  a fixture with the same filename is refused rather than overwriting the existing evidence pair
  — this is an **intentional-collision row** under U10's step-5 classification, and its expected
  outcome is the refusal; **recovery sweep
  discrimination** — against the mirror, a session-start recovery sweep refuses **exactly** the nine
  enumerated legacy filenames and succeeds on every other mirrored file.
* **Narrowed evidence-pair claim (cycle-17)**: the payload move uses `moveNoReplace`
  (`internal/core/checkpoint_disposition.go:~259-282`) and is genuinely no-clobber; the **sidecar**
  is written with `atomicfile.WriteFileAtomic` (`:229-231`), which is documented as an *idempotent
  upsert* and **replaces** an existing destination. The pair is therefore protected in aggregate
  only because the payload move runs first and refuses the collision before the sidecar step is
  reached. This unit asserts exactly that — payload no-clobber, and the pair intact after a refused
  second quarantine — and makes **no** claim that the sidecar write is itself no-replace. Cycles
  16 and earlier stated the stronger claim; it is false against the shipped code. Giving the
  sidecar its own `O_EXCL` create is a production behaviour change outside this plan's gate
  findings and is recorded as follow-up stash **`A12BBAFA`**.
* **Restore row withdrawn (cycle 16)**: cycles 4-15 carried a mandatory "restore path" row that
  executed U9b's rename-aside, copy-back, and hand-repair sequence and called it proof that
  quarantine is recoverable. U9b's executable restore procedure is withdrawn in cycle 16 on security
  grounds (no real-root / no-follow open, no read-to-write CAS, no adversarial coverage — stash
  `35A27CD0`), so a verification row that executes it would be verifying text this plan no longer
  publishes. The row is replaced by the evidence-integrity row above, which proves the property the
  restore row actually needed: the verbatim pre-quarantine bytes and their sidecar survive intact
  and cannot be clobbered. Demonstrating an automated round trip moves to `35A27CD0` with the
  procedure it verifies.
* **Nine-file acknowledgement**: satisfied by row 3 against the mirror. U10's live-corpus SHA-256
  comparison must still pass after this unit runs.
* **Inherited inputs and teardown**: U10 hands this unit a **live** workspace — the branch-built
  binary, the copied fixtures, the mirror source, and the quarantine archive row 2 inspects.
  Confirm those inputs are present before row 1 rather than rebuilding them; a missing workspace
  blocks this unit on re-running U10, it does not license a hand-rebuild. **Teardown is deferred to
  U10c (cycle-17)**: U10c consumes the same scratch workspace, so tearing it down here would
  destroy U10c's inputs. Teardown ownership moves to U10c, still `ActionRisk: destructive` (A4b),
  still requiring explicit operator approval immediately before execution, and still skipped and
  recorded as a cleanup follow-up when approval is withheld.
* **Evidence persistence**: rows append to the same tracked closure file U10 writes,
  `docs/closure/2026-08-checkpoint-disposition-runtime-verification.md`.
* **Depends on**: U10 (scratch workspace, ignore rule, branch-built binary, and the quarantine
  archive row 2 inspects).

### U10c — Runtime verification of context-duplicate parity and the abandoned-resolve handler

* **Partition**: 5 (runtime verification and closure)
* **Domain**: verification
* **Files**: none beyond appending to
  `docs/closure/2026-08-checkpoint-disposition-runtime-verification.md`. Runs inside the scratch
  workspace U10 created.
* **Change (cycle-17 — closes gate finding H3)**: cycles 14–16 parked two behaviours in a
  "Runtime Verification and Closure" table row that no unit owned, so no task carried them, no
  dependency scheduled them, and nothing forced them to run. This unit owns them.
* **Execution contract**: U10's six-point contract applies unchanged, including the cycle-17
  step-5 operation-class split (in-place rewrite versus archive move versus intentional collision).
* **Rows** (3):
  1. **Exact-duplicate `context` member, cross-surface** — a fixture whose `context` carries an
     exact-duplicate member (including an escape-equivalent spelling) is refused by `resolve`
     **and** by `abandon` with `checkpoint_non_conforming` naming `duplicate:context.<key>` in a
     raw, bounded `unknown_fields` array; is reported `needs_quarantine: true` by `list` **and**
     `get` with a matching `non_conforming_fields.paths` list; is **accepted** by `quarantine`; and
     its bytes are SHA-identical across both refusals.
  2. **Fold variant aliasing a modeled field, plus the open-namespace guard** — a fixture whose
     `context` carries `shipment_id` + `Shipment_Id` is refused the same way, while a sibling
     fixture carrying distinct unmodeled `foo` + `Foo` **resolves successfully** and both keys
     survive the rewrite. A third fixture whose only context-routing member is spelled `Context`
     with an inner exact duplicate is refused, pinning U2h at runtime.
  3. **Abandoned-resolve handler mapping** — invoking `backlogit_resolve_checkpoint` on an
     already-abandoned document returns `validation_failed`, **not** `"error":"internal"`, proving
     U7e's retained `domainError` row reaches the handler.
* **Why this is its own unit and not more rows on U10/U10b**: U10 and U10b each already carry
  three rows, which is the granularity ceiling. Adding these would put either at five or six.
  Splitting also gives the context-duplicate behaviour (U2g, U2h) and the abandoned-resolve
  mapping (U7e) a named owner, so a reviewer can see which unit fails if either regresses.
* **Teardown**: `docs/scratch/checkpoint-verification/` teardown is owned by this unit and runs
  only after all three rows pass — `ActionRisk: destructive` (A4b), explicit operator approval
  immediately before execution, skipped and recorded as a cleanup follow-up when approval is
  withheld. The tracked closure evidence file survives teardown by construction.
* **Depends on**: U10b (the workspace and its inherited inputs), U2h, U6c, U7d, U7e, U8c.

## Dependency Graph

Canonical as of **cycle 17**. Every active unit and every executable edge appears below; U5b is
retired and appears only in the historical count. This section, the edge table, the requirements
trace, the execution order, and the counts are all regenerated from one authoritative source — the
`item_deps` rows for queued tasks under `147-F`, read back with `backlogit --cwd . query` after
`backlogit --cwd . sync`. No edge appears here that is not in the backlog, and no backlog edge is
missing here.

The graph is layered by the five cycle-17 partitions. Partition order is the hard execution order:
every edge crosses a partition boundary forward or stays inside one partition, and no edge points
backward from a later partition to an earlier one.

```text
PARTITION 1 — foundation diagnostics and conformance
  U1 ──┬──▶ U1b ──▶ U1c
       ├──▶ U2 ──┬──▶ U2b ──┬──▶ U2e
       │         │          └──▶ U2g ──▶ U2h
       │         ├──▶ U2c ──┬──▶ U2e
       │         │          ├──▶ U2g
       │         │          └──▶ U2d
       │         └──▶ U2d
       └──▶ U2g
  U1d                                    (independent leaf declaration — second ready root)

PARTITION 2 — guarded rewrite seam
  U2 ──▶ U11 ──▶ U12 ──▶ U13 ──▶ U14 ──▶ U2f
  U2c ─▶ U12          U2d ─▶ U11

PARTITION 3 — declarations and genuine RED harness order
  U1b ─┐
  U1d ─┴──▶ U15 ─┐
  U1 ────────────┤
  U1b ───────────┼──▶ U8b        (U8b compiles against declarations, fails on assertions)
  U1d ───────────┤
  U2 ────────────┘

PARTITION 4 — implementation plus MCP/CLI/instruction contracts
  U8b ──┬──▶ U3 ──▶ U3b ─┐
        ├──▶ U4 ──┬──▶ U17          U14 ──┬──▶ U3
        │         └──▶ U5            │    ├──▶ U3b
        ├──▶ U6 ──┬──▶ U6d           │    └──▶ U4
        │         └──▶ U6b ──┬──▶ U6c
        └──▶ U7e             └──▶ U8c
  U3b ─┐
  U4 ──┼──▶ U7 ──┬──▶ U7d ──▶ U7c
  U5 ──┘         └──▶ U8 ──▶ U16
  U6b ──▶ U7b        U6c ──▶ U7b        U6d ──▶ U7b
  U1c ──▶ U8
  U2e ─┐
  U6b ─┤
  U8b ─┼──▶ U9 ──▶ U9b
  U16 ─┤
  U17 ─┤
  U7b ─┤
  U7c ─┘

PARTITION 5 — runtime verification and closure
  U9b ──▶ U10 ──▶ U10b ──▶ U10c
  U2h ─┐
  U6c ─┼──▶ U10c
  U7d ─┤
  U7e ─┤
  U8c ─┘
```

Edges declared, no cycles. The table below carries all **98** executable edges, grouped by source
task. Each row lists one dependent task and every prerequisite it declares, so the row count (38)
is the number of tasks that declare at least one dependency, not the edge count.

| Dependent | Unit | Prerequisites (`item_deps` rows) | Reason |
|---|---|---|---|
| `147.002-T` | U2 | `147.001-T` | The helper returns the typed error U1 declares. |
| `147.003-T` | U2b | `147.002-T` | Extends the same helper. |
| `147.004-T` | U2c | `147.002-T` | Extends the same helper. |
| `147.005-T` | U2d | `147.002-T`, `147.004-T` | Extends the same helper, and the derived-set refactor lands only after the duplicate rule makes the predicate feature-complete. |
| `147.006-T` | U3 | `147.004-T`, `147.037-T`, `147.016-T` | Calls the completed predicate; the guarded seam and its caller migration land first; the parity harness must be red before this gate. |
| `147.007-T` | U3b | `147.006-T`, `147.011-T`, `147.028-T`, `147.037-T`, `147.016-T` | Conformance gate sits after the validity gate; its residual test asserts U6 flags the same file; inherits the context-member verdict; seam first; harness first. |
| `147.008-T` | U4 | `147.004-T`, `147.028-T`, `147.037-T`, `147.016-T` | Calls the completed predicate; inherits the context-member verdict; seam first; harness first. |
| `147.009-T` | U5 | `147.008-T`, `147.016-T` | U5's paired row asserts the refusal U4 introduces; harness first. |
| `147.011-T` | U6 | `147.004-T`, `147.032-T`, `147.016-T` | The list verdict calls the completed predicate and publishes the structured remediation intent; harness first. |
| `147.012-T` | U6b | `147.011-T`, `147.030-T`, `147.038-T`, `147.032-T`, `147.016-T` | Both read surfaces must report the same field set; renders through the bounded raw projection; populates the carrier U15 declares; carries the intent; harness first. |
| `147.013-T` | U7 | `147.001-T`, `147.030-T`, `147.007-T`, `147.008-T`, `147.009-T`, `147.016-T` | The mapping layer matches U1's sentinel; `unknown_fields` renders through the bounded raw projection; MCP maps the sentinels U3b, U4, and U5 emit; harness first. |
| `147.014-T` | U7b | `147.012-T`, `147.022-T`, `147.023-T`, `147.016-T` | The read descriptions promise U6b's field as U6c projects it and must not promise U6d's unshipped filter exemption; harness first. |
| `147.015-T` | U8 | `147.013-T`, `147.030-T`, `147.031-T`, `147.016-T` | The CLI consumes the mapping layer and renders the key list through the human projection, whose bound comes from U1b; harness first. |
| `147.016-T` | U8b | `147.001-T`, `147.030-T`, `147.032-T`, `147.002-T`, `147.038-T` | **Declarations only.** The harness compiles against the typed error, the bounded projection, the remediation intent, the conformance helper, and the read-result carrier — and fails on assertions because none of the behaviour exists. |
| `147.017-T` | U9 | `147.020-T`, `147.012-T`, `147.016-T`, `147.039-T`, `147.040-T`, `147.014-T`, `147.024-T` | The design doc restates the completed predicate, the final read contract, cross-surface parity, the CLI remediation block, the corrected wrap, and the published tool descriptions. |
| `147.018-T` | U9b | `147.017-T` | Agent guidance follows the design doc. |
| `147.019-T` | U10 | `147.018-T` | Refusal verification follows the published guidance. |
| `147.020-T` | U2e | `147.003-T`, `147.004-T` | The nested duplicate rule extends U2b's recursion and reuses U2c's `duplicate:` reporting form. |
| `147.021-T` | U2f | `147.037-T` | The supplemental caller-set guard enumerates the post-migration allow-list, so the migration lands first. |
| `147.022-T` | U6c | `147.012-T`, `147.016-T` | The MCP handler projects U6b's result type; harness first. |
| `147.023-T` | U6d | `147.011-T`, `147.016-T` | The filter exemption extends U6's conformance branch; harness first. |
| `147.024-T` | U7c | `147.013-T`, `147.025-T`, `147.016-T` | The mutation descriptions promise U7's codes, which only reach `resolve` once U7d routes them; harness first. |
| `147.025-T` | U7d | `147.001-T`, `147.013-T`, `147.016-T` | The handler routes on U1's predicate into U7's response shape; harness first. |
| `147.026-T` | U10b | `147.019-T` | Acceptance and evidence verification consume U10's workspace. |
| `147.027-T` | U8c | `147.012-T`, `147.016-T` | The CLI handler projects U6b's result type; harness first. |
| `147.028-T` | U2g | `147.001-T`, `147.003-T`, `147.004-T` | Returns U1's typed error; must not narrow the open `context` namespace U2b protects; reuses U2c's reporting form one level down. |
| `147.029-T` | U7e | `147.001-T`, `147.016-T` | The `domainError` row matches a sentinel in U1's package; harness first. |
| `147.030-T` | U1b | `147.001-T` | The bounded projection is a method on the typed error U1 declares. |
| `147.031-T` | U1c | `147.030-T` | The human rendering reads the bounded raw projection. |
| `147.033-T` | U2h | `147.028-T` | Extends U2g's context-member walk to every routing spelling. |
| `147.034-T` | U11 | `147.002-T`, `147.005-T` | The seam calls the conformance predicate, and takes that dependency only after the predicate's key source is pinned. |
| `147.035-T` | U12 | `147.034-T`, `147.004-T` | The contract harness compiles against the seam declaration and asserts the completed top-level predicate. |
| `147.036-T` | U13 | `147.035-T` | The implementation turns U12's contract green. |
| `147.037-T` | U14 | `147.036-T` | Callers migrate onto a seam that already enforces its preconditions. |
| `147.038-T` | U15 | `147.030-T`, `147.032-T` | The carrier's fields are the bounded projection set and the remediation intent. |
| `147.039-T` | U16 | `147.032-T`, `147.015-T`, `147.016-T` | The renderer consumes the structured intent and attaches to U8's refusal output; harness first. |
| `147.040-T` | U17 | `147.008-T`, `147.016-T` | The wrap correction lands beside the gate U4 installs in the same function; harness first. |
| `147.041-T` | U10c | `147.026-T`, `147.033-T`, `147.022-T`, `147.025-T`, `147.029-T`, `147.027-T` | Consumes U10b's workspace and verifies the routing rule, both read projections, the resolve routing, and the abandoned-resolve mapping at runtime. |

`147.032-T` (U1d) declares no prerequisites and is the second ready root.

**Suggested execution order**: U1, U1d, U1b, U1c, U2, U2b, U2c, U2d, U2e, U2g, U2h, U11, U12, U13,
U14, U2f, U15, U8b, U3, U6, U6d, U6b, U6c, U8c, U7b, U3b, U4, U17, U5, U7, U7e, U7d, U7c, U8, U16,
U9, U9b, U10, U10b, U10c.

Partition boundaries are the load-bearing part of that order: everything through U2h is partition 1;
U11 through U2f is partition 2; U15 and U8b are partition 3; U3 through U9b is partition 4; U10
through U10c is partition 5. Inside a partition, units that share no edge may be taken in any order.
U1d is independent of everything in partition 1 and only has to land before U15, U8b, U6, U6b, and
U16. U2f is terminal by design — nothing depends on it, and a blocked U2f does not block the release
unit, because the guarded seam rather than the enumeration is what enforces I1.

**Measured topology (cycle 17, `backlogit --cwd . query` after `backlogit --cwd . sync`)**: 40
queued tasks, **98** queued-to-queued executable edges, 41 shipment members in `130-S`, ready set
exactly `{147.001-T, 147.032-T}` (two roots — `U1`, the typed-error declaration, and `U1d`, the
remediation-intent declaration). **Historical total edges: 99** — the 98 executable edges plus the
one archived edge `147.010-T -> 147.009-T` retained in the archived U5b artifact. The historical
count is deliberately kept distinguishable from the executable topology: only the 98
queued-to-queued edges govern execution order, and an agent must never schedule against the
historical figure. The graph is verified acyclic by Kahn topological sort — all 40 nodes ordered
from the two roots.

## Decisions and Rationale

| Decision | Rationale |
|---|---|
| Refuse rather than preserve | The repository already fixed the general form twice: the top level is a CLOSED namespace at create (`checkpoint_strict.go`, 146.011-T), and a document that cannot be trusted to round-trip must be moved verbatim rather than rewritten (`checkpoint-administrative-disposition.md`). A preservation carrier would make one namespace closed in one direction and open in the other. |
| **Distinguishing `2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`** | That learning is this repository's standing resolution for a re-persist seam that drops data, and its precedents (`MoveInQueue`, `serializer_provenance_hardening`, `attachCommitToItems`) all resolved **preserve**, not refuse. It is deliberately **not** followed here, for three reasons that do not hold in those cases. (1) *Ownership*: frontmatter provenance fields are **modeled, owned** fields the serializer forgot to reload — the fix restores a known value. Checkpoint top-level extras are **arbitrary and unowned**: there is no schema that says what they mean, so "reload and re-emit" preserves bytes without preserving meaning. (2) *Namespace direction*: the checkpoint top level is **already closed at create** (146.011-T). Preserving on rewrite would make the same namespace closed inbound and open outbound — an asymmetry no other backlogit surface has. (3) *An open counterpart already exists*: `CheckpointContext.Extra` is the sanctioned carrier shipped in 146-F, so refusing at the top level does not deny users a preservation mechanism; it directs them to the one that already exists. Where the learning **is** followed: the mutation seam is treated as the enforcement point (U3/U4), not the create boundary alone. |
| Add the `ResolveCheckpoint` validity gate | It is the larger, live, agent-reachable half of the defect. Without it the refusal only narrows the loss instead of removing it. |
| Widen quarantine classification | Mandatory, not optional. Refusing on both mutation verbs without widening quarantine strands valid-but-non-conforming files with no disposition path. |
| New `ErrCheckpointNonConforming` sentinel | The create and read boundaries have deliberately different legal key sets; one sentinel across both would make `errors.Is` ambiguous. |
| Reuse `ErrCheckpointUseQuarantine` for resolve's invalid-document refusal | Exact symmetry with `AbandonCheckpoint`'s existing idiom and the already-correct MCP remediation code. |
| Do NOT make `ParseCheckpoint` strict | Already rejected in `docs/exec-plans/2026-08-21-success-shaped-evidence-loss-plan.md`: it would sweep the on-disk corpus into quarantine candidacy. The conformance check is caller-invoked at two write boundaries and two read boundaries, never inside `ParseCheckpoint`. |
| Reflection-derived key set, never a literal | Per `docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md`: keep the derivation pinned so the create and read boundaries cannot drift apart as `CheckpointV1` gains fields. |
| Enforce in the mutation seam, not only at create | Same learning: "the setter protects the file (source of truth)". The create boundary cannot protect a file written by an older binary or edited by hand — which is precisely how all nine live legacy files came to exist. This is the one point where `2026-07-28-attach-commit-repersist-must-reload-from-markdown.md` **is** honoured. |
| Round-trip safety, not "no unknown keys", is the predicate | A document with `"status"` and `"Status"` has zero *unknown* keys yet still loses a member on rewrite. Naming the predicate correctly is what makes U2c non-optional. |
| Conformance is a **new** reported field, not a redefinition of `valid` | `GetCheckpoint`'s `valid` already has consumers. U6b adds `conforming` alongside it so no shipped contract changes meaning underneath a caller. |
| **U2f's gated-seam fallback reinstated as the primary mechanism** (cycle 17, gate finding H8) | Cycle 3 withdrew the "enumeration test **or** exported `events.RewriteCheckpointFile` seam" fallback and made the enumeration the only mechanism, on width grounds. The cycle-16 gate ruled that an AST enumeration cannot fully enforce I1: it must resolve call targets, aliased imports, indirect writers, and any future helper wrapping an atomic write, and each unresolved case is a silent hole. Cycle 17 therefore builds the seam — but as **four bounded units** (U11 declaration, U12 contract harness, U13 implementation, U14 caller migration) rather than one five-file unit, which is what made the fallback unshippable in cycle 3. The enumeration survives as U2f, a supplemental caller-set regression guard that is explicitly not load-bearing. |
| **Quarantine's verbatim move stays outside the seam** (cycle 17) | `moveNoReplace` never parses and never re-marshals, so it cannot drop keys. Routing it through a parse-and-validate seam would impose a parse precondition on the one verb whose purpose is disposing of documents that cannot be parsed. `CleanupCheckpoints`' `os.Rename` and `CreateCheckpoint`'s new-file writes are excluded for the same structural reason. |
| **Machine offender arrays carry raw paths; only human text is quoted** (cycle 17, gate finding H4) | A quoted path in a machine array forces the consumer to parse a Go string literal before it can compare the value against the document's own key, and a synthetic `"+N more"` element is indistinguishable from a real offender of that name. Truncation is therefore structural — `truncated`, `omitted_paths`, `truncated_paths` — and escaping moves to U1c, which owns the only human rendering. |
| **Remediation is published as structured intent, not a command string** (cycle 17, gate finding H1) | `internal/events` does not know the caller's working directory, so any command string it emits is bound to an ambient cwd and carries no approval, preimage, or no-clobber obligation. The CLI is the only layer that knows the resolved workspace, so it is the only layer allowed to render a command — with an explicit `--cwd`, a bare filename, and the A4c preamble attached (U16). The MCP surface and the agent instruction file publish the structured record only. |
| **The `AbandonCheckpoint` `%v` wrap is fixed, not deviated from** (cycle 17, gate finding H7) | Cycles 1-16 recorded it as a documented Principle I deviation on "pre-existing" grounds. That escape hatch closes the moment U4 and U14 edit the same function: a one-verb change in a function this plan already touches is not an unrelated contract change. U17 makes it multi-`%w`, and the Constitution Check row moves from deviation to pass. |
| **MCP `get` keeps its validation-class refusal for schema-invalid documents** (PR #377 review cycle 3) | `handleGetCheckpoint` routes through `domainError`, which takes no filename and already maps `ErrCheckpointInvalid` to `validation_failed`. Making `get` emit `checkpoint_use_quarantine` would mean routing a **read** through a mutation-shaped error path, widening U6c into U7's file set and changing a shipped contract. The quarantine remedy is already discoverable from `backlogit_list_checkpoints` (`needs_quarantine: true`), so the safety value is nil and the churn is real. Disposition codes stay on the mutation verbs. |

## Risks and Caveats

| Risk | Severity | Mitigation |
|---|---|---|
| `ResolveCheckpoint` becomes stricter and breaks an existing caller | Medium | Existing fixtures are built from a `CheckpointV1` struct (`internal/events/checkpoint_lifecycle_test.go:19-25`) and conform by construction. U3 adds explicit red-phase coverage for both refusals and re-asserts the conforming and idempotent paths. |
| **Agent session-start recovery hits nine refusals on day one** | **High** | This is *correct* behaviour, not a regression, and must not be read as one. The nine filenames are enumerated in Runtime Verification and Closure; the rollback trigger fires only on a refusal **outside** that set. U9b updates the agent-facing instruction file in the same PR so recovery guidance matches. |
| The quarantine widening is dropped and the deadlock ships | **High** | U5 is a first-class unit whose primary test asserts accept-by-quarantine and refuse-by-abandon **in the same table row**, so neither assertion can be deleted alone. |
| **`ResolveCheckpoint` refusal surfaces to agents as a 500** | **High** | `handleResolveCheckpoint` routes through `domainError`, which has no case for the two new sentinels and would fall to `InternalError`. **U7d** is the operative mitigation: it reroutes every `QuarantineIsRemedy` match to `checkpointDispositionError`, so both new refusals carry `code` and `filename` and never reach the `internal` default. U7d asserts the payload is not `"error":"internal"`. **U7e** covers the one sentinel U7d's predicate does not match — `ErrCheckpointCannotResolveAbandoned` — which is the only remaining live path to a 500 on this handler. Cycle 15's "U7 adds both cases as a safety net" was superseded in cycle 16: the two `domainError` rows it referred to were unreachable and were removed. |
| **Nested `context` keys swept into refusal, regressing 146-F** | **High** | U2b's second scenario is a permanent regression guard asserting unmodeled `context` keys return nil. |
| Duplicate / fold-variant top-level keys pass conformance and are then collapsed | Medium | U2c makes `strings.EqualFold`-equal top-level keys non-conforming, reported as `duplicate:<key>`. |
| Conformance key set drifts from `CheckpointV1` | Medium | U2d asserts set equality against the create-boundary set plus the reserved keys, guarding the hand-written reserved literal. |
| A future change reintroduces a top-level preservation carrier | Low | U2d asserts `CheckpointV1` declares no `json:"-"` map carrier, anchored to the deliberation so the guard reads as "revisit the decision", not "never". |
| Widened quarantine increases traffic into `archive/checkpoints/`, where `CleanupCheckpoints` `os.Remove`s a colliding destination | Medium | `moveNoReplace` already refuses to overwrite on the quarantine path. The reverse direction — copying archived bytes back under the original name — is the real hazard, and cycle 16 removes it from this release entirely: U9b no longer publishes an executable restore, and U10b no longer executes one. U10b's evidence-integrity row instead asserts that a second quarantine of the same filename is refused rather than clobbering the existing pair. Restoring archived evidence safely (real-root / no-follow open, handle-or-content CAS, no-clobber destinations, adversarial tests) is deferred to stash `35A27CD0`. |
| The offender list is unbounded or drifts between CLI and MCP | Medium | U1b owns the single bounded **raw** projection `BoundedFieldPaths()` (16 paths, 128 bytes per path cut on a rune boundary, structural truncation counts). U6b, U6c, and U7 all render through it and are forbidden from re-deriving or re-capping, so neither surface can emit an unbounded or differing set. U1c owns the only quoted rendering, so escaping cannot leak into a machine array. |
| **An ungated in-place rewrite path is added later** | **High** | The guarded seam (U11/U12/U13/U14) is the only in-place rewrite path after cycle 17, so a new ungated rewrite requires adding a new direct atomic-write call. U2f's supplemental enumeration makes the common form of that regression loud, but the invariant does not depend on it — an honest bound recorded in U2f itself. |
| **An agent or operator pastes an unbound remediation command** | **High** | No library surface emits one. `internal/events` publishes `RemediationIntent`; the MCP surface publishes the same structure; the agent instruction file states the verb and points at the record. The CLI renderer (U16) is the only command surface and always emits an explicit `--cwd`, a bare filename, and the A4c approval / preimage / no-clobber preamble, and refuses to render at all when quoting would be required. |
| The nine live legacy files are mutated during verification | Medium | U10 and U10b run against an in-tree scratch workspace only — U10b's recovery sweep uses a **copied mirror**, never the live directory. Live files are read for shape reference and never used as mutation targets. Every file under `.backlogit/checkpoints/` is hash-compared programmatically before and after. |
| Windows atomic-write regression | Low | No change to `atomicfile.WriteFileAtomic` or `syncWriteFileAtomic`; only additional pre-write gates. |
| CLI reference drift blocks the PR | Low | U9 regenerates `gen-docs` output and runs `backlogit docs lint` before handoff. |
| `CreateCheckpoint` same-second filename collision silently overwrites (adjacent, **out of scope**) | Medium | Surfaced during the entry-point audit (I1). Not fixed here and not stashed, to hold the bounded scope. Recorded in Plan Hardening as a named follow-up. |

## Constitution Check

| Principle | Verdict | Notes |
|---|---|---|
| I. Safety-First Go | **pass** | All production changes are Go; no `unsafe`. New wraps use multi-`%w` so both sentinels resolve. **Cycle-17 change**: the pre-existing `%v` validation wrap in `AbandonCheckpoint` (`internal/core/checkpoint_disposition.go:~70-73`) is **fixed** by U17 rather than recorded as a deviation. The cycle-16 gate ruled the deviation unavailable: Principle I is not satisfiable by documenting a departure from it, and the "unrelated shipped contract" justification lapses once U4 and U14 edit that same function. |
| II. Test-First Development (NON-NEGOTIABLE) | **pass** | Every code unit uses the two-step red posture declared at the head of Implementation Units: a declaration stub so the package **compiles**, then a harness that **fails on assertions**. Expected red is stated per unit, and cycle 16 pins the exact per-unit `-run` selector and `-count=1` invocation that observes it, so "red" is verifiable rather than asserted. U2d owns a real production delta with a compiling-but-failing harness case that fails against the pre-delta state. U8b lands in partition 3 against the U15/U1b/U1d/U2 declarations and fails on assertion behaviour before any partition-4 implementation lands (see U8b's Expected red enumeration); the cycle-15/16 batch-harness-generation framing is withdrawn because it made the red gate depend on implementer sequencing rather than the dependency graph. U5's withdrawn state-conflict rows never contributed to its red gate; U5 retains its genuine red assertion (scenario 1's accept-half). P-004 does not permit an all-guards exemption, so cycle-8 replaced the earlier exemption claims with concrete red-load statements, cycle-10 retired U5b whose production delta contradicted the decision's scope boundary, and cycle-16 corrected U7e's expected-red statement, which had claimed `default: InternalError` for a row that in fact shadows through the generic `validation_failed` case. |
| III. Workspace Isolation and Security Boundaries | **pass** | No path handling changes. `ResolveDispositionTarget`, `ensurePathContained`, and `validateCheckpointFilename` are untouched. The new gates operate on already-read bytes. `Fields` carries key **paths** only, never values, so a refusal cannot leak checkpoint content. No secrets introduced. |
| IV. CLI Workspace Containment (NON-NEGOTIABLE) | **pass** | All edits are inside the repository tree. U10's scratch workspace is pinned to `docs/scratch/checkpoint-verification/` **inside** the working tree — never `%TEMP%`, never a sibling or parent — and the path is asserted to be repo-root-relative before any write. |
| V. Structured Observability | **deviation (documented)** | Refusals are typed and machine-readable: `unknown_fields` (raw paths plus structural truncation scalars) on MCP, named keys on CLI, `NeedsQuarantine` + a structured `RemediationIntent` on list and get. The audit-before-mutation ordering is **preserved** (not strengthened — the ordering already existed; U4 only moves the new gate to sit ahead of it). **Deviation**: no new counter, log line, or telemetry event is emitted when a refusal occurs, so a spike in refusals is observable only through agent-visible errors. Accepted for this scope; recorded as a follow-up. |
| VI. Single Responsibility | **pass** | No new dependencies. The helper reuses `decodeTopLevelEntries`, `isFoldKeyIn`, `modeledJSONTagKeys`, and `unknownNestedProgressKeys` already present in `internal/events`. |
| VII. Destructive Command Approval (NON-NEGOTIABLE) | **pass (conditional — see the approval condition below)** | Cycle 16 withdraws the two cycle-15 "documented deviations" against this principle. A NON-NEGOTIABLE principle cannot be satisfied by documenting a departure from it; either every destructive action is approval-gated or the plan fails the check. Both are now gated. **A4c is the single operative contract for every checkpoint-file-moving or checkpoint-file-overwriting command** — `quarantine`, `abandon`, `resolve`, and any operator-performed copy-back — and requires explicit operator approval **immediately before each execution batch**, never once at plan time. **A4b** (scratch teardown) requires its own approval immediately before execution. **A4d** confines live post-merge observation to the read verbs or a byte-copy mirror. **A5** (mutating the live corpus) stays forbidden without separate authorization. The change itself remains net **anti**-destructive: it removes a silent data-destruction path. **Condition**: this verdict is `pass` only while every live quarantine, resolve, abandon, or operator repair batch carries a recorded approval taken immediately before execution. An unapproved batch is a P-005 violation and a halt, not a documented exception. |
| VIII. Explicit Safety Modes | **pass** | Work executes under **freeze-scope**. Declared boundary: `internal/errors/`, `internal/events/`, `internal/core/`, `internal/mcp/`, `internal/cli/`, `docs/design-docs/checkpoint-administrative-disposition.md`, `docs/cli-reference/backlogit_checkpoint_*.md`, `.github/instructions/backlogit.instructions.md`, `.autoharness/backlog-registry.yaml`, `docs/closure/`, and `docs/scratch/checkpoint-verification/`. The nine live checkpoint files are explicitly **outside** the mutation boundary. |
| IX. Git-Friendly Persistence | **pass** | Checkpoint JSON stays human-readable; `jsonutil.MarshalReadable` and the atomic-write helpers are unchanged. |
| X. Agent Context Efficiency | **pass** | Refusals carry structured field lists so an agent does not parse message text to learn which keys were rejected. U6b closes the `list` / `get` disagreement that would otherwise cost an agent an extra round trip and a wrong verb. |
| XI. Merge Commit History Preservation (NON-NEGOTIABLE) | **pass** | Ships through a merge commit. Squash and rebase merge are forbidden and must be verified before merge. |

### Documented deviations

| Principle | Deviation | Justification | Simpler alternative rejected |
|---|---|---|---|
| V. Structured Observability | No refusal counter, log, or telemetry event. | The refusal is already agent-visible and typed; adding a telemetry surface pulls `internal/telemetry` into a freeze-scoped change and widens the blast radius past the defect. | "Emit a telemetry event per refusal" — rejected: nine known refusals on day one would immediately produce noise with no consumer defined. |

**Principle I carries no deviation (cycle 17).** The row recorded here through cycle 16 —
`AbandonCheckpoint`'s `%v` validation wrap left in place — is withdrawn. The cycle-16 gate found
(H7) that the justification depended on the wrap living in code this plan does not touch, which
stopped being true when U4 placed a gate in that function and U14 moved its write onto the seam.
**U17** corrects the verb to multi-`%w`, asserts both `errors.Is` matches, and pins the rendered
message text so no consumer regresses. The alternative "leave it and record a follow-up" is
rejected: it is a knowingly-shipped Principle I violation on a touched path.

**Principle VII carries no deviation (cycle 16).** The two rows recorded here through cycle 15 were
withdrawn. A NON-NEGOTIABLE principle is not satisfiable by documenting a departure from it, so
each was converted into an approval obligation instead:

* *Quarantine moves a live `active` file.* It is now covered by **A4c** — explicit operator
  approval immediately before each execution batch, with a preimage byte copy as a precondition of
  the request. U9b states quarantine is a deliberate operator decision and forbids prescribing an
  automatic session-start sweep. The alternative "refuse on all three verbs" is still rejected: it
  strands the file with no disposition path, which is strictly worse than an approved,
  evidence-preserving move.
* *U10b's scratch teardown is a directory deletion.* It is covered by **A4b** — explicit approval
  immediately before execution, skipped and recorded as a cleanup follow-up if withheld. The
  alternative "use `%TEMP%`" is still rejected outright by Constitution IV.

Cycle 15 also justified the first row with "reversible by copy-back, which U10b proves". That
justification no longer holds and is not reused: U10b's restore row is withdrawn (see U10b), so the
plan claims only that the archived bytes and their sidecar are a preserved, no-clobber, verbatim
record — which U10b's evidence-integrity row does prove — and defers a verified automated round
trip to stash `35A27CD0`.

Constitution Check: documented-deviations

## Plan Hardening Signals

| Signal | Present | Justification |
|---|---|---|
| Public API, schema, or contract change | **yes** | `ResolveCheckpoint` changes from lenient to gated — previously-succeeding calls now return an error. A new exported function, a new exported error sentinel, and a new exported predicate are added; `GetCheckpoint` gains a reported field. Five MCP tool descriptions change, which is an agent-facing contract surface. |
| Security, auth, permission, or compliance-sensitive behaviour | **yes** | The paths carry the checkpoint disposition audit trail. Gate placement relative to `appendCheckpointDispositionAudit` is load-bearing: getting it wrong would append an audit event for a disposition that never happened. |
| Migration, backfill, destructive data/config action, or irreversible step | **yes** | No data migration, but the change is a behavioural break on a governed write path, and the work is motivated by an existing irreversible data-destruction bug. Nine live files sit in the affected class. |
| External integration, operator checkpoint, or external dependency | **yes** | Both the CLI and the MCP tool surface change, and the Stage and Ship session-start recovery protocols call `resolve` directly. |
| High runtime, rollout, or rollback risk | **partial** | Rollback is a clean revert (no data migration), but a mis-scoped gate could refuse legitimate conforming checkpoints and block agent session recovery. |

Requires plan hardening: yes

## Runtime Verification and Closure

| Unit | Runtime surface changed | What runtime verification must prove |
|---|---|---|
| U3, U3b | CLI `backlogit checkpoint resolve`, MCP `backlogit_resolve_checkpoint` | A legacy-shaped document in the scratch workspace is refused and its bytes are byte-identical (SHA before/after). A valid-but-non-conforming document is refused with `checkpoint_non_conforming`. A conforming active checkpoint still resolves. The MCP payload is **not** `"error":"internal"`. |
| U4 | CLI `backlogit checkpoint abandon`, MCP `backlogit_abandon_checkpoint` | A valid-but-non-conforming document is refused, the offending keys are named, and the disposition audit JSONL is unchanged. |
| U5 | CLI `backlogit checkpoint quarantine`, MCP `backlogit_quarantine_checkpoint` | The same document is accepted, moved byte-identically into the archive, and given a disposition sidecar. A conforming `status:"active"` file is refused by quarantine with `ErrCheckpointUseAbandon` (U5's scenario-2 hard gate). The conforming `status:"resolved"` double-refusal row is **not** verified here — cycle 16 withdrew U5's absorbed state-conflict guards as out-of-scope (follow-up stash `6FA45E69`), so no runtime row asserts an assertion the plan no longer owns. |
| U6, U6b | CLI `backlogit checkpoint list` / `get`, MCP `backlogit_list_checkpoints` / `backlogit_get_checkpoint` | A non-conforming file reports `needs_quarantine: true` with a **structured** `remediation_intent` on **both** read surfaces — verb, bare target filename, `requires_approval: true`, approval class `A4c`, and no shell text — and the file is unchanged after reading. |
| U8, U8b, U16 | CLI error output, cross-surface parity, CLI remediation block | Both refusals exit non-zero with actionable text naming the offenders in quoted form; CLI, MCP, and the `events` read layer reach the same classification from the same stored file; and the CLI remediation block carries an explicit `--cwd`, a bare filename, and the approval / preimage / no-clobber lines. |
| U10 | Live workspace, read-only | Every live SHA-256 hash under `.backlogit/checkpoints/` is unchanged across the whole verification run. |
| U10b | Scratch mirror of `.backlogit/checkpoints/` | A session-start recovery sweep against the mirror refuses **exactly** the nine enumerated legacy filenames and succeeds on every other mirrored file. The quarantine **payload** archive is a verbatim, no-clobber record, and a second quarantine of the same filename is refused rather than overwriting it. The sidecar's replace semantics are not claimed as no-clobber (see U10b). |
| U10c | CLI `checkpoint resolve` / `abandon` / `quarantine` / `list` / `get`, MCP `backlogit_resolve_checkpoint` and `backlogit_abandon_checkpoint` | The three rows U10c owns, run against scratch fixtures under an A4c approval batch and U10's six-point execution contract: (1) an exact-duplicate `context` member is refused by `resolve` **and** `abandon` with `checkpoint_non_conforming` naming `duplicate:context.<key>` in a raw bounded `unknown_fields` array, is reported `needs_quarantine: true` by `list` **and** `get` with a matching `non_conforming_fields.paths` list, is **accepted** by `quarantine`, and its bytes are SHA-identical across both refusals; (2) a fold variant aliasing a modeled field (`shipment_id` + `Shipment_Id`) is refused the same way, a sibling carrying distinct unmodeled `foo` + `Foo` **resolves successfully** with both keys surviving, and a fixture whose only routing member is spelled `Context` with an inner exact duplicate is refused (U2h); (3) invoking `backlogit_resolve_checkpoint` on an already-abandoned document returns `validation_failed`, **not** `"error":"internal"`, proving U7e's row reaches the handler. |

**The nine expected-refusal filenames** (enumerated so a correct refusal is never misread as a
regression — recorded from the live corpus inspection performed during deliberation; the exact list
is re-captured and pinned in the closure artifact before merge):

> All nine currently-schema-invalid files under `.backlogit/checkpoints/`. They are the complete set
> of live checkpoints that fail `ValidateCheckpoint` today, and they already fail `abandon` today
> (F1). The two conforming files are `checkpoint-20260822-064434.json` (resolved) and
> `checkpoint-20260822-212617.json` (active, stale `129-S`).

**Operational closure artifacts required before the work is absorbed**:

* **Healthy signal** — `checkpoint resolve` / `abandon` succeed on checkpoints that are
  **schema-valid, top-level-conforming, `status: "active"`, and carry no administrative
  disposition**, and refusals occur **only** on filenames in the enumerated nine-file set. That
  scoping is load-bearing (cycle-17): a `status: "resolved"` conforming file is an idempotent no-op
  on resolve and is refused by abandon with `ErrCheckpointNotActive` (I3 row 3, pre-existing and
  out of scope), and a document carrying `disposition: "abandoned"` is refused by resolve with
  `ErrCheckpointCannotResolveAbandoned`. Neither is a failure of this work, and an unscoped
  "conforming checkpoints resolve and abandon" signal would misclassify both as incidents.
* **Failure signal** — any refusal of a **conforming, active, undisposed** checkpoint; any refusal
  on a filename **outside** the enumerated set; any audit event appended for a refused disposition;
  any MCP refusal that surfaces as `"error":"internal"`.
* **Rollback trigger** — a refusal on any checkpoint filename outside the enumerated nine, **or**
  any **conforming, active, undisposed** checkpoint failing to resolve or abandon. Revert the merge
  commit; there is no data migration to unwind. A refusal *within* the nine does not trigger
  rollback at any frequency, and neither does a state-scoped refusal on a non-active or
  administratively disposed document.
* **Pre-merge acknowledgement** — the merging operator must confirm in the PR that day-one refusals
  on the nine legacy files are expected behaviour, so the post-merge observation window is not
  interpreted as an incident.
* **Ownership and validation window** — one Ship session post-merge, verified by running Stage and
  Ship session-start recovery against the live workspace **using the read verbs only** (A4d).
* **Blocked-path handling (cycle-17 correction)** — if agent recovery is blocked by a refusal on
  one of the nine, the remedy is **not** a revert and **not** an in-place hand repair. Disposing of
  any of the nine live legacy files is **A5**, which this plan classifies `abandoned` and forbids:
  it requires its own separately approved unit of work. Until that unit exists, the operator's
  options inside this scope are (a) leave the file in place — recovery reads `list` and `get`,
  which report it as a quarantine candidate and do not block, or (b) authorize a new unit of work
  for live disposition. Cycles 15 and 16 said "the remedy is `checkpoint quarantine` under an A4c
  approval", which contradicts A5 for exactly the nine files the sentence is about; the
  contradiction is removed rather than reconciled, because A5 is the more conservative rule.
* **Follow-ups recorded (not in scope)**: dispose of the nine live legacy checkpoint files and the
  stale active `129-S` checkpoint `checkpoint-20260822-212617.json` as workspace hygiene (this is
  the **A5** unit of work referenced by Blocked-path handling above);
  refusal observability; the `CreateCheckpoint`
  same-second filename collision; the CLI/MCP error-shape asymmetry (stash `63E810D9`); the
  cycle-15 security-lens items adjudicated out of this bounded scope — create-boundary `context`
  duplicate handling (**stash `E429A031`**), and symlink / no-follow containment together with the
  read-to-write CAS race on rewrite (**stash `35A27CD0`**); and the three cycle-16 items —
  the withdrawn conforming + `resolved` state-conflict guards (**stash `6FA45E69`**), CLI coverage
  for `checkpoint resolve` on an already-abandoned document (**stash `DBBA62AA`**), and the
  body-preserving repair and post-quarantine restore runbook, which is folded into **`35A27CD0`**
  because it cannot be published safely until that stash's containment work lands. **Cycle-17
  additions**: removing the deprecated `CheckpointSummary.RemediationCommand` string field once
  every consumer reads `RemediationIntent` (**stash `F350503F`**), and giving the quarantine
  disposition sidecar its own no-replace create so the evidence pair is protected in both halves
    rather than only through the payload move's `moveNoReplace` (**stash `A12BBAFA`**). Rollback
  safety
  beyond revert-the-merge is not recorded as a follow-up: with no data migration, no schema change,
  and no on-disk format change, revert is already complete for this scope.

**P-016 is a Ship execution precondition, not a plan defect (cycle-17).** At the time this
decomposition was written the repository carried a second linked worktree
(`.copilot/session-state/ecebe820-.../files/dark-factory-worktree`, branch `chore/121-s-closure`)
unrelated to this shipment. It does not invalidate this plan and it is not a containment violation
— the cycle-14, cycle-15, and cycle-16 gates each rejected that claim, and the linked worktree
remains the valid mechanism for the single dedicated implementation branch `chore/stage-130-s`.
It is recorded here as an explicit **precondition Ship must verify before claiming `130-S`**:
exactly one active implementation worktree may exist for this release unit, so the unrelated
worktree must be finished or removed by its own owner first. Stage does not touch it — removing
another agent's worktree is outside Stage's Role Boundary and would be a destructive action
requiring its own approval.

### Final mandatory gate sequence

Run in constitutional order before the work is handed to review. Do not skip or reorder a gate;
a later gate's output is not evidence for an earlier one.

**These gates are mandatory for the release unit as a whole, not per unit.** Shipment `130-S` ships
one branch and one PR, and that branch contains Go source changes, so **every** gate below runs
against the final branch state before handoff — including when the last task merged into the branch
happens to be docs-only. A docs-only unit does not exempt the branch from the Go gates; it only
means that unit's own commit introduced no Go delta.

**Every gate below is written to be executable in Windows PowerShell against the whole branch
(cycle-17).** The repository's `Makefile` uses GNU-make shell syntax (`$$(...)`) and `bash`, so
`make` targets are not portable to the implementing session's shell. Each gate therefore states the
PowerShell form and, where one exists, the equivalent Makefile target it reproduces. Gates that
report by writing to stdout rather than by exit code assert **empty output** explicitly.

```powershell
# Gate 1 — Tests (Makefile: test)
go test -race ./...
if ($LASTEXITCODE -ne 0) { throw 'Gate 1 FAILED: go test' }

# Gate 2 — Vet (Makefile: vet)
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Gate 2 FAILED: go vet' }

# Gate 3 — Lint (Makefile: lint)
golangci-lint run
if ($LASTEXITCODE -ne 0) { throw 'Gate 3 FAILED: golangci-lint' }

# Gate 4 — Format (Makefile: fmt). gofmt -l reports by STDOUT, not exit code.
$bad = gofmt -l .
if ($bad) { $bad; throw 'Gate 4 FAILED: unformatted files' }

# Gate 4b — Import ordering (go.instructions.md Commands; same stdout-not-exit-code trap)
$badimp = go run golang.org/x/tools/cmd/goimports@v0.39.0 -l .
if ($LASTEXITCODE -ne 0) { throw 'Gate 4b HALTED: goimports unavailable (cold module cache / offline)' }
if ($badimp) { $badimp; throw 'Gate 4b FAILED: import ordering' }

# Gate 5 — Markdown (P-008: MD001 / MD025 / MD041, repo-wide; Makefile: md-lint)
bash scripts/md-lint.sh
if ($LASTEXITCODE -ne 0) { throw 'Gate 5 FAILED: markdownlint' }

# Gate 6 — Docline frontmatter compliance on authored docs (Makefile: docs-lint)
go run ./cmd/backlogit docs lint
if ($LASTEXITCODE -ne 0) { throw 'Gate 6 FAILED: docs lint' }

# Gate 7 — Docline compliance on the changed backlog and plan artifacts
go run ./cmd/backlogit --cwd . docs lint
if ($LASTEXITCODE -ne 0) { throw 'Gate 7 FAILED: backlog docs lint' }

# Gate 8 — CLI Reference Drift (no-diff check; U9 owns regeneration if Cobra metadata changed)
go run ./cmd/gen-docs docs/cli-reference
git diff --exit-code -- docs/cli-reference
if ($LASTEXITCODE -ne 0) { throw 'Gate 8 FAILED: CLI reference drift' }

# Gate 9 — Build and provenance (reproduces Makefile: build, LDFLAGS at Makefile:6-8)
$sha  = (git rev-parse --short HEAD).Trim()
$ver  = (git describe --tags --always --dirty).Trim()
$date = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
$ld = "-X github.com/softwaresalt/backlogit/internal/version.Version=$ver " +
      "-X github.com/softwaresalt/backlogit/internal/version.Commit=$sha "  +
      "-X github.com/softwaresalt/backlogit/internal/version.BuildDate=$date"
go build -ldflags $ld -o bin/backlogit.exe ./cmd/backlogit
if ($LASTEXITCODE -ne 0) { throw 'Gate 9 FAILED: build' }
$reported = (& ./bin/backlogit.exe version --format json --no-update-check | ConvertFrom-Json).commit
if ($reported -ne $sha) { throw "Gate 9 FAILED: provenance mismatch - binary reports '$reported', HEAD is '$sha'" }
```

**Gate 4 is not a bare `gofmt -l .` (cycle-16 correction, retained).** `gofmt -l .` exits **0**
even when it lists unformatted files — it reports by writing filenames to stdout, not by status
code — so a CI step or a checklist item that only inspects the exit code passes on a
formatting-dirty tree. `Makefile:24-25` implements the correct gate in `bash`
(`bad=$(gofmt -l .); test -z "$bad" || { printf '%s\n' "$bad"; exit 1; }`); the PowerShell form
above is its exact equivalent. Gate 4b has the identical trap and is written the same way.

**Gate 4b is version-pinned and network-free by construction (cycle-17).** `golang.org/x/tools`
is pinned at `v0.39.0` in `go.sum`, and `go run <pkg>@<version>` executes in module-independent
mode, so the invocation neither adds a module dependency (Principle VI) nor floats its version. If
the module cache is cold and the environment is offline, the gate **halts** — it is never skipped,
and the implementing session asks the operator to warm the cache. `go.instructions.md` states the
three-group import ordering rule normatively and lists `goimports -l .` as the check for it; the
default `golangci-lint` linter set does not include `goimports` and this repository has no
`.golangci.yml`, so nothing else enforces the rule.

**Gate 9 asserts provenance programmatically (cycle-17).** Cycles 1-16 wrote the SHA comparison as
a trailing comment (`# commit must equal git rev-parse --short HEAD`), which no runner evaluates.
The form above captures `git rev-parse --short HEAD` into `$sha`, reproduces the Makefile's own
`LDFLAGS` shape (`Makefile:6-8`), and **throws** on inequality. `--format json` is the JSON
selector (`internal/cli/version_cmd.go:98`) — there is no `--json` flag — and `--no-update-check`
(`:99`) suppresses the remote release lookup so the assertion is hermetic. A plain
`go build ./cmd/backlogit` leaves `internal/version.Version` at its `DevVersion` default with an
**empty** commit and proves nothing about provenance.

Gates 1-4b are the constitutional Go gates plus the import-ordering check. Gates 5-8 are the
repository's already-required documentation gates, restated here in one place so the implementing
session does not have to reassemble them from U9, U9b, and the Risks table. Gate 9 pins the
branch-built binary's provenance and is the same mechanism U10's environment precheck uses.

## Plan Hardening

**Hardening required: yes.** Four of five signals in the Plan Hardening Signals table are present
and one is partial. The decisive one is that this plan adds a refusal gate to a **governed,
audited, agent-reachable state transition** — exactly the shape that produced the most severe
finding of the 117-S review cycle.

### Learnings and instruction files consulted

| Source | What it changed in this plan |
|---|---|
| `docs/compound/security-issues/2026-08-09-audit-all-entry-points-sharing-guarded-state-transition.md` (severity: critical) | Forced the entry-point completeness audit below. A guard on `AbandonCheckpoint` and `ResolveCheckpoint` alone is only as strong as the weakest *other* path that can rewrite a checkpoint file in place. |
| `docs/compound/security-issues/2026-08-09-authenticate-before-filter-security-check-ordering.md` | Forced the explicit gate-ordering invariants below. A "does this even apply" filter placed ahead of the trustworthiness verification creates an invisible bypass — and drove U4's gate **ahead** of the already-abandoned short-circuit and U6's conformance branch **ahead** of the list filter block. |
| `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md` | **Deliberately distinguished, not followed.** This is the repository's standing "preserve at a re-persist seam" resolution and its precedents all chose preserve. The Decisions and Rationale table records the three grounds on which checkpoint top-level extras differ (unowned/arbitrary data, a namespace already closed at create, and an existing sanctioned open carrier in `context`). Its seam-enforcement half **is** followed: U3/U4 gate the mutation seam, not just create. |
| `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md` | The direct precedent naming this exact preserve-vs-refuse fork. Its Guidance section drove the body-preserving hand-repair procedure added to U9b in cycle 10. Cycle 16 **withdrew** the executable form of that procedure on security grounds and deferred it to stash `35A27CD0`; the learning's substance is honoured by preserving the pre-quarantine bytes and their sidecar verbatim and by refusing every implicit-survivor shortcut, rather than by publishing an unsafe runbook. |
| `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` | Rule 3 forced U8b: a cross-surface guard must exercise CLI **and** MCP **from the same stored state**, not from independent fixtures. Rule 1 forced the registry-drift re-run, split across U7b (two read-surface descriptions) and U7c (three mutation-surface descriptions). |
| `docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md` | Forced the `unknown_fields` array contract in U7: no `omitempty`, and the empty case asserted with a `.([]any)` type assertion. |
| `docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md` and `docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md` | Forced the fresh-binary requirement in U10: the pinned repo-root `backlogit.exe` predates this work and must not be used to verify it. |
| `docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md` | Already reflected: enforce in the mutation seam, reflection-derive the key set. |
| `.github/instructions/strict-safety.instructions.md` | Risky actions expressed as `ProposedAction` / `ActionRisk` / `ActionResult` below. |
| `.github/instructions/constitution.instructions.md`, `.github/instructions/circuit-breaker.instructions.md` | Constitution Check above; stop conditions below. |
| `.github/instructions/backlogit.instructions.md` | Identified as an **artifact to update** (R10, U9b), not merely consulted — its Lifecycle Hygiene Protocol currently teaches every agent that `resolve` is infallible and that the abandon/quarantine split is validity-only. |

### Entry-point completeness audit (protected invariant I1)

**I1 — Every code path that rewrites an existing checkpoint file in place must be gated, not just
the two named in the stash text.** Audit of every checkpoint write site in `internal/`, with the
cycle-17 post-migration verdict:

| Write site | Kind | Verdict |
|---|---|---|
| `internal/events/checkpoint_rewrite.go` (`RewriteCheckpointFile`, **new** — U11/U13) | in-place rewrite of an existing file | **The seam.** The only sanctioned in-place rewrite path. Requires parse, validate, and conformance before any marshal or atomic replace. |
| `internal/events/checkpoint_lifecycle.go:178` (`ResolveCheckpoint`) | in-place rewrite of an existing file | **Migrated onto the seam — U14.** Its verb-specific sentinel mapping and gate ordering are U3 and U3b. |
| `internal/core/checkpoint_disposition.go:105` (`AbandonCheckpoint`) | in-place rewrite of an existing file | **Migrated onto the seam — U14.** Its conformance-gate ordering ahead of the already-abandoned short-circuit and the audit append is U4; its multi-`%w` wrap is U17. |
| `internal/events/memory.go:106` (`CreateCheckpoint`, V1 branch) | new file | **Already gated** by `checkClosedSchemaNamespace` (146.011-T/U4). Not a rewrite; outside the seam. No change. |
| `internal/events/memory.go:112` (`CreateCheckpoint`, legacy branch) | new file, verbatim bytes | **Deliberately unchanged.** This is the legitimate origin of arbitrary top-level keys on disk; making it strict was rejected in the source document. Not a rewrite; outside the seam. |
| `internal/events/checkpoint_lifecycle.go:242` (`CleanupCheckpoints`) | `os.Rename` — verbatim move | **Correct by construction.** Never parses or re-marshals, so it cannot drop keys. Explicitly outside the seam. No change. |
| `internal/core/checkpoint_disposition.go` `moveNoReplace` (`QuarantineCheckpoint`) | verbatim move | **Correct by construction and deliberately outside the seam.** U5 widens *which* files reach it, not how it writes. Routing it through a parse-and-validate seam would impose a parse precondition on the one verb that exists to dispose of unparseable documents. |
| `internal/core/checkpoint_disposition.go:~231` (quarantine disposition sidecar) | new sidecar file, replace semantics | **Out of the seam and out of this scope.** It writes a generated record, not the checkpoint document, so it cannot drop checkpoint keys. Its replace-on-collision behaviour is recorded honestly in U10b and deferred as stash `A12BBAFA`. |
| `internal/telemetry/checkpoint.go:94` | different artifact (telemetry harvest cursor), different schema, not under `.backlogit/checkpoints/` | **Out of scope.** Named here so the audit is total and a reviewer does not read its absence as an omission. |

**Making I1 enforceable (cycle-17 rewrite).** A table in a plan document decays, and cycle 16 ruled
that an AST write-call enumeration cannot carry the invariant on its own: it must resolve call
targets, aliased imports, indirect writers, and any future helper that wraps an atomic write, and
each unresolved case is a silent hole in the guarantee. **The enforcement is therefore
architectural.** U11/U12/U13/U14 make `RewriteCheckpointFile` the single in-place rewrite path and
migrate both existing callers onto it, so an ungated rewrite cannot exist without someone adding a
new direct atomic-write call to the checkpoint directory. **U2f** keeps the enumeration as a
*supplemental* caller-set regression guard over `internal/events` and `internal/core`, with an
explicitly stated bound: it proves no new **direct, statically resolvable** write was added outside
the seam, and it makes that common regression loud. It does not carry I1 by itself, nothing depends
on it, and a blocked U2f does not block the release unit. A comment does not satisfy I1; neither
does an enumeration alone.

**Hardening-surfaced adjacent defect, explicitly NOT fixed here.** `CreateCheckpoint`
(`internal/events/memory.go:58`) derives its filename at **one-second** granularity
(`checkpoint-20060102-150405.json`) and writes with `syncWriteFileAtomic`, which overwrites
unconditionally. Two checkpoint creates inside the same UTC second silently destroy the first
file. This is a distinct pre-existing defect (create-collision overwrite), not an instance of the
`parse -> mutate -> re-marshal` key-loss class this plan closes, and fixing it here would breach
the bounded scope of `D3CE9E81`. It is recorded as a named follow-up in the Ship handoff and must
not be silently absorbed into any unit of this plan.

### Gate-ordering invariants (protected invariant I2)

**I2 — In every gated function the ordering below is load-bearing and must be pinned by test, not
by comment.** Reordering any of these produces either a false refusal or a silent bypass.

`ResolveCheckpoint` — exact required order (existing steps 1–6 keep their current positions):

1. `validateCheckpointFilename`
2. `ensurePathContained`
3. `os.ReadFile` (not-found → `ErrCheckpointNotFound`)
4. `ParseCheckpoint`
5. already-resolved idempotent short-circuit (`cp.Status == "resolved"` → `nil`, no write)
6. `cp.Disposition == DispositionAbandoned` → `ErrCheckpointCannotResolveAbandoned`
7. **NEW** `ValidateCheckpoint` → `ErrCheckpointUseQuarantine`
8. **NEW** `CheckConformingTopLevelNamespace` → `ErrCheckpointNonConforming`
9. mutate + write **through `RewriteCheckpointFile`** (the guarded seam)

**Cycle-17 note on steps 7-9.** After U14 the seam re-checks validity and conformance immediately
before it marshals, so steps 7 and 8 are not the *only* thing standing between an untrustworthy
document and a rewrite — they are what makes the refusal carry the **correct verb-facing sentinel**
and what places it ahead of the mutation closure. The seam is defence in depth; steps 7 and 8 are
the contract. Removing either half is a regression: without the seam an ungated caller could be
added later, and without steps 7 and 8 the refusal would surface as a raw verdict error that no
MCP mapping row matches.

Steps 5 and 6 stay ahead of 7 and 8 deliberately: both are non-writing terminal answers, and
moving the validity gate ahead of them would convert a shipped idempotent no-op into a new
error for an invalid-but-already-resolved document.

**Named, tested residual for step 5.** That choice has a cost: an invalid or non-conforming
document whose `status` is already `"resolved"` never reaches gates 7 or 8, so `resolve` returns
`nil` on it forever. This includes the exact fabricated skeletons the *pre-fix* `ResolveCheckpoint`
produced. The residual is accepted because resolve does not write in that branch — nothing further
is lost — and its **discovery path is U6/U6b**, which flag the file `needs_quarantine: true`. U3b
pins both halves in a single test so neither can be removed alone.

`AbandonCheckpoint` — the conformance gate goes **immediately after `ValidateCheckpoint` and
strictly before the already-abandoned short-circuit**, which is itself strictly before
`appendCheckpointDispositionAudit`. Two orderings are load-bearing here:

* *Before the already-abandoned short-circuit*: a non-conforming file carrying
  `disposition: "abandoned"` would otherwise return `nil` (reported success, no write) from
  `abandon` while `quarantine` accepts it and `list`/`get` report `needs_quarantine: true` — three
  surfaces disagreeing about one file. This is the "authenticate before filter" learning applied
  literally: the trustworthiness check must precede the applicability filter.
* *Before the audit append*: the shipped ordering guarantee is audit-then-mutate, so a refusal must
  land before the append and leave **no** audit event and **no** byte change. U4 asserts the audit
  JSONL is unchanged after refusal — this is the assertion that would catch a future reordering.

`QuarantineCheckpoint` — classification is a pure in-memory decision (`parse`, `validate`,
**new** `conformance`) that must complete **before** any audit append or `moveNoReplace`. The
widened predicate is `valid && conforming` → refuse with `ErrCheckpointUseAbandon`; anything else
is quarantinable.

### Scoped totality invariant (protected invariant I3)

**I3 — For an `active` checkpoint, the two disposition verbs must remain both DISJOINT and TOTAL.**
The scope qualifier is essential: totality is claimed **only** over the `status: "active"` state,
because a separate, pre-existing state-conflict class already exists and is deliberately not
addressed here.

| `status` | parses + validates + conforms | `abandon` | `quarantine` |
|---|---|---|---|
| `active` | yes | **accepts** | refuses `ErrCheckpointUseAbandon` |
| `active` | no (any of the three fails) | refuses `ErrCheckpointUseQuarantine` / `ErrCheckpointNonConforming` | **accepts** |
| `resolved` | yes | refuses `ErrCheckpointNotActive` | refuses `ErrCheckpointUseAbandon` |
| `resolved` | no | refuses `ErrCheckpointUseQuarantine` / `ErrCheckpointNonConforming`¹ | **accepts** |

¹ The validity and conformance gates run **ahead of** the `ErrCheckpointNotActive` check at
`internal/core/checkpoint_disposition.go:~87-89`, so a resolved document that is malformed,
schema-invalid, or non-conforming is refused by abandon on **trustworthiness** grounds — not on
state — and reaches quarantine. `ErrCheckpointNotActive` is therefore reached only by a resolved
document that parses, validates, and conforms, which is row 3.

**Row 3 is a real double-refusal**, and it is **pre-existing behaviour introduced by neither this
plan nor 146-F**. A conforming, valid, non-active checkpoint has no disposition verb. That is the
named **state-conflict class**, explicitly out of scope: widening quarantine to accept it would
change what "quarantine" means (from "these bytes are untrustworthy" to "I want this file gone"),
which is a separate decision requiring its own deliberation. Cycle 16 **withdrew** U5's absorbed
state-conflict guards (follow-up stash `6FA45E69`), so the exclusion is documented here and in U9's
design-doc scope qualifier rather than pinned by a test inside a unit that does not own the class.
No claim elsewhere in the plan now depends on an assertion no unit makes.

Within the `active` scope there is no third class and no file both verbs refuse. U5's primary test
asserts the accept-by-quarantine and refuse-by-abandon halves **in the same table row** precisely
so a future edit cannot delete one half and leave a deadlock behind. This is the single
highest-value assertion in the plan; if review trims anything, it must not be this.

### Risky actions

| # | ProposedAction | Targets | Change kind | ActionRisk | Approval | Rollback | ActionResult |
|---|---|---|---|---|---|---|---|
| A1 | Add a refusal gate to `ResolveCheckpoint`, a path both Stage and Ship session-start recovery call | `internal/events/checkpoint_lifecycle.go` | behaviour change on a governed write path | **high** | not required (net anti-destructive), but must be called out at PR review | revert the merge commit; no data migration | planned |
| A2 | Widen `QuarantineCheckpoint` classification so more files become movable to the archive | `internal/core/checkpoint_disposition.go` | contract change on a file-moving verb | **high** | not required; move is verbatim and audited, and `moveNoReplace` never overwrites | revert; quarantined files are recoverable from the archive dir with their sidecar | planned |
| A3 | Change five MCP tool descriptions (`list`, `get`, `resolve`, `abandon`, `quarantine`) and update `.github/instructions/backlogit.instructions.md` | `internal/mcp/tools.go`, `.github/instructions/backlogit.instructions.md`, `.autoharness/backlog-registry.yaml` | agent-facing contract change | moderate | not required | revert | planned |
| A4 | Create and seed the scratch verification workspace — `mkdir`, byte-copy fixtures in, build the verify binary, and run the **read** verbs (`list`, `get`, `version`) against it | `docs/scratch/checkpoint-verification/` only | local file creation, no checkpoint mutation | moderate | not required — creation and reads only; **this row grants no authority to run a disposition verb**. Cycle 15's wording ("run disposition verbs … not required") contradicted A4c and is removed: any command that moves or overwrites a checkpoint file, in scratch or anywhere else, is A4c | discard the scratch contents | planned |
| A4c | Execute a **disposition command that overwrites or moves a checkpoint file** — `checkpoint quarantine` (moves the file into `archive/checkpoints/` and writes a sidecar), `checkpoint abandon` (rewrites the document in place), `checkpoint resolve` (rewrites the document in place), and any operator-performed archive copy-back | scratch fixtures, scratch mirror, live corpus, and any archive/sidecar they create | file move / in-place overwrite | **destructive** | **required, and A4c is the sole operative contract for this class** — obtain explicit operator approval **immediately before each execution batch**, not once at plan time (Constitution VII). Approval covers the named files in the named directory only; a new directory or a new filename set needs a fresh approval. The batch request MUST carry the U10 execution contract: `--cwd` binding with bare filenames, per-file filename/hash/state/destination display, a pre-run preimage byte copy, the **operation-class-appropriate** destination precondition (cycle-17: absent-destination and no-clobber for an **archive move**; preimage plus a post-step SHA comparison for an **in-place rewrite**, where the destination is necessarily present and absent-destination must not be asserted; a declared intentional-collision row expects a refusal), and fail-closed halting | quarantine is reversible in principle by operator copy-back from `archive/checkpoints/`; an in-place rewrite is reversible only from the pre-run byte copy, so a pre-run byte copy of every mutating fixture is a **precondition** of the approval request | planned |
| A4b | Tear down the scratch verification workspace after closure (**owned by U10c** since cycle 17, only after its three rows pass — U10 and U10b hand the workspace over intact) | `docs/scratch/checkpoint-verification/` | directory deletion | **destructive** | **required** (Constitution VII) — obtained immediately before execution | none needed — contents are reproducible from the plan, and the durable evidence lives in the tracked closure artifact rather than in the scratch directory | planned |
| A4d | Post-merge observation of the live workspace | `.backlogit/checkpoints/` | **read-only** — `checkpoint list` and `checkpoint get` only | low | not required **while read-only**; any mutating verb against the live corpus leaves this row and becomes A4c or A5 | n/a | planned |
| A5 | Mutate the nine live legacy checkpoints or the stale `129-S` checkpoint | `.backlogit/checkpoints/` | destructive, irreversible | **destructive** | **FORBIDDEN in this work.** Out of scope; requires explicit operator approval in a separate unit of work. | n/a | **abandoned** |

**Live post-merge observation is read-only or mirror-based (cycle 15, reinforced cycle 16).** The
closure step "run one Stage and one Ship session-start recovery against the live workspace" MUST
NOT invoke `resolve`, `abandon`, or `quarantine` against `.backlogit/checkpoints/`. Session-start
recovery protocols do call `resolve` on leftover checkpoints, so the observation is performed either
(a) with the read verbs only, reading `needs_quarantine`, `conforming`, and `non_conforming_fields`
from `checkpoint list` / `checkpoint get`, or (b) against a byte-copy mirror under `docs/scratch/`,
exactly as U10b's sweep already does. Any live mutating disposition is A4c at minimum and A5 if it
targets one of the nine legacy files or the stale `129-S` checkpoint; neither is authorized by this
plan. **No automatic session-start repair, restore, or quarantine sweep of live checkpoints is
prescribed anywhere in this plan**, and U9b is explicitly forbidden from publishing one.

### Deepened runtime verification (U10, U10b, U10c)

* **Environment precheck** — build a fresh binary from the branch HEAD **with the repository's own
  version ldflags**, because a plain `go build ./cmd/backlogit` leaves
  `internal/version.Version` at its `DevVersion` default and `Resolve()`'s `debug.ReadBuildInfo`
  fallback records no module version for a locally built main package, so `backlogit version` would
  report `dev` with an **empty** commit and prove nothing about provenance
  (`internal/version/version.go`). Use the repository's own `LDFLAGS` variable (`Makefile:5-8`),
  whose shape the release workflow reproduces (`.github/workflows/release.yml:99-107`):

  ```text
  go build -ldflags "-X github.com/softwaresalt/backlogit/internal/version.Version=verify-<short-sha> -X github.com/softwaresalt/backlogit/internal/version.Commit=<short-sha> -X github.com/softwaresalt/backlogit/internal/version.BuildDate=<rfc3339>" -o <scratch>/backlogit-verify.exe ./cmd/backlogit
  ```

  where `<short-sha>` is `git rev-parse --short HEAD`, matching the Makefile's `COMMIT` derivation.
  Then assert that
  `<scratch>/backlogit-verify.exe version --format json --no-update-check` reports a `commit` field
  **equal to that `<short-sha>`**. **Cycle-16 correction**: cycle 15 wrote `version --json`, which
  is not a flag this CLI has — the JSON selector is `--format json`
  (`internal/cli/version_cmd.go`) — and omitted `--no-update-check`, which makes the assertion
  depend on a remote release lookup. Both are corrected here. **Do not** use the repo-root pinned
  `backlogit.exe`: it predates this work, so a green run against it would prove nothing
  (self-hosted version-skew learning).
* **Scratch workspace** — create the verification workspace *inside* the repository working tree at
  `docs/scratch/checkpoint-verification/` (Constitution IV; never `%TEMP%`), assert the resolved
  path is repo-root-relative before the first write, add the directory to the freeze-scope
  declaration and to `.gitignore` if it is not already covered, seed it with byte-copies of the
  legacy document shapes, and confirm `.backlogit/checkpoints/` is not the target directory before
  running any mutating verb.
* **Live-corpus guard** — record `Get-FileHash` for all files under `.backlogit/checkpoints/`
  **before** and **after** the verification run and assert programmatically that every hash is
  identical (R6, A5). A visual comparison is not sufficient.
* **Target scenarios** — the three rows of U10 (read verdict, refusal, quarantine accept), the
  three rows of U10b (acceptance, quarantine evidence integrity, recovery-sweep discrimination),
  and the three rows of U10c (exact-duplicate `context` member refused cross-surface, fold-variant
  aliasing with the open-namespace guard, and the abandoned-resolve handler mapping to
  `validation_failed`), each asserting the fixture's SHA before and after for refusal cases. **The
  restore-path row is withdrawn (cycle 16)**: U9b no longer publishes an executable restore
  procedure, so a row that executes one would verify text the plan does not contain. What that row
  actually needed to prove —
  that the verbatim pre-quarantine bytes and their sidecar survive intact and cannot be clobbered —
  is proved by U10b's evidence-integrity row. A verified automated round trip moves to stash
  `35A27CD0` together with the containment work that makes it safe.
* **Mirror, not live corpus** — U10b's recovery-sweep row runs against
  `docs/scratch/checkpoint-verification/mirror/.backlogit/checkpoints/` (workspace root:
  `docs/scratch/checkpoint-verification/mirror/`), a byte-copy of `.backlogit/checkpoints/`; all
  sweep CLI invocations use `--cwd docs/scratch/checkpoint-verification/mirror` with bare filename
  arguments. The
  sweep needs successful `resolve` calls on the conforming files to prove discrimination, and those
  succeed by rewriting; running it against the live directory would contradict the live-corpus
  guard above. Mirrored filenames are preserved so the nine-name assertion still means what it
  says.
* **Recovery-procedure contract (cycle 15, extended cycle 16)** — every command in the recovery
  sweep and in **every A4c batch** MUST:
  1. **Bind to the canonical workspace explicitly.** Pass `--cwd <mirror-or-scratch-root>` on every
     invocation and pass **bare filenames**, never paths. An unbound invocation resolves against the
     ambient working directory and can reach the live corpus, which is exactly the A5 boundary.
     Assert the resolved storage root in the run log before the first mutating verb.
  2. **Display, per file, before and after each step**: the bare `filename`, its SHA-256 `hash`,
     its classified `state` (`valid`/`conforming`/`needs_quarantine`), and the `destination` path
     the step will write or move to. A step whose destination is not under the bound root is a halt,
     not a warning.
  3. **Handle evidence no-clobber for the archive payload move.** The archive write (quarantine's
     payload move) MUST refuse to overwrite an existing destination, matching the shipped
     `moveNoReplace` semantics: if a destination is occupied, stop and report the collision rather
     than overwriting quarantined evidence, which is the only verbatim record of the pre-quarantine
     bytes. **The `.disposition.json` sidecar write is not yet no-replace**: it is written with
     `atomicfile.WriteFileAtomic`, a documented idempotent upsert that replaces an existing
     destination, so the pair is protected only in aggregate because the payload move refuses the
     collision first. Hardening the sidecar to its own no-replace create is tracked separately as
     follow-up stash `A12BBAFA` and is not asserted by this verification pass.
  4. **Take a pre-run byte copy (preimage)** of every fixture a mutating verb will touch
     (precondition of the A4c approval request).
  5. **Apply the precondition that matches the operation class.** An **in-place rewrite**
     (`resolve`, `abandon`) targets a destination that **is** the existing file and is necessarily
     present: absent-destination and no-clobber are **not** applicable and must not be asserted;
     the required preconditions are the preimage byte copy from step 4 and a post-step SHA
     comparison — equality on a refusal, inequality plus a re-parse that validates on an
     acceptance. A **normal archive move** (`quarantine`) requires the destination under
     `archive/checkpoints/` to be asserted **absent** before the move, and the move MUST refuse to
     clobber an occupied destination. A declared **intentional-collision test** is the deliberate
     exception: it requires an **occupied** destination — the row creates the collision itself —
     and its expected outcome is a refusal with the pre-existing evidence left unchanged, never a
     successful move.
* **Blocked-path handling** — if any refusal case instead succeeds and rewrites the file, halt
  immediately, do not proceed to closure, and treat it as a red-phase failure of the owning unit.
  Conversely, a refusal on one of the nine enumerated legacy filenames is **expected** and must not
  be treated as a blocked path.
* **Teardown** — owned by **U10c** and performed only after its three rows pass; U10 and U10b
  leave the workspace standing because U10c consumes its archive, fixtures, mirror, and binary.
  Classified `ActionRisk: destructive` (A4b) and executed only with explicit operator approval
  taken immediately before execution. If approval is withheld, leave the directory in place and
  record it as a cleanup follow-up rather than deleting it unilaterally.

### Deepened operational closure

* **Monitoring signal (no metrics backend — manual observation)** — after merge, run one Stage and
  one Ship session-start recovery against the live workspace **using the read verbs only**
  (`checkpoint list`, `checkpoint get`) or against a byte-copy mirror under `docs/scratch/`, per
  A4d, and confirm neither is blocked by a **false** refusal. Refusals on the nine enumerated
  legacy filenames are expected and healthy. A mutating disposition against
  `.backlogit/checkpoints/` is **not** part of this observation. Record the outcome in the closure
  artifact.
* **Rollback trigger** — a refusal on any checkpoint filename **outside** the enumerated nine, or
  any conforming checkpoint refused by `resolve`/`abandon`. Threshold: **one** occurrence. A
  refusal *within* the nine does not trigger rollback at any frequency; treating it as a trigger
  would revert correct behaviour on day one.
* **Rollback procedure** — revert the merge commit. No data migration, no schema change, no
  on-disk format change, so revert is complete and sufficient. Files quarantined between merge and
  revert stay in the archive directory with their sidecars; recovering them is an operator action
  under A4c, and this release publishes no automated restore (stash `35A27CD0`).
* **Owner and validation window** — the Ship session that merges the work; one session
  post-merge.
* **Human checkpoint** — the P-014 merge approval is the only operator checkpoint, and the merging
  operator must acknowledge the nine expected day-one refusals in the PR. There is no partial
  rollout, feature flag, or external dependency. Scratch teardown (A4b) is a second, separate
  approval, and every A4c batch is its own approval.

### Stop conditions for the implementing session

Per `.github/instructions/circuit-breaker.instructions.md`: 5 build/test fix attempts per unit,
3 on the same recurring error, 3 review-fix cycles, 5 fix-CI cycles. If U5's **scoped** totality
assertion — disjoint and total over `status: "active"` — cannot be made to pass, **halt** rather
than weakening it: a green suite with an `active`-state deadlock shipped is worse than a blocked
task. Row 3 of the I3 table (conforming + `resolved` → refused by both) is **not** covered by that
stop condition; it is pre-existing, out of scope, and after cycle 16 it is documented rather than
asserted (follow-up stash `6FA45E69`).

### Unresolved operator decisions

None block execution. Two items are carried forward as declared out-of-scope follow-ups requiring
their own authorization: disposing the nine live legacy checkpoints plus the stale `129-S`
checkpoint (A5), and the `CreateCheckpoint` same-second filename-collision overwrite surfaced by
the entry-point audit.

## Plan Review

> [!IMPORTANT]
> **Historical record — cycles 1 through 13 only.** This gate result does **not** cover cycle 14,
> cycle 15, or cycle 16. The current gate state is the **fourth** `## Plan Review` record at the
> end of this document (`cycle: 16`, `decision: FAIL`).

dispatch_mode: multi-agent-dispatch

decision: PASS (superseded — historical, scoped to cycles 1-13)

### Dispatch record

**Attempt 1** — all seven review personas dispatched as independent sub-agents against the
post-hardening plan: Architecture Strategist, Scope Boundary Auditor, Go Reviewer, Constitution
Reviewer, Security Lens Reviewer, Agent-Native Parity Reviewer, Learnings Researcher. The
Agent-Native Parity Reviewer returned no response on its first dispatch (model
`gemini-3.1-pro-preview`); it was retried once with `claude-sonnet-4.6` and returned successfully,
so coverage is complete with no persona skipped. Result: **FAIL** — 1 P0, 16 P1 (pre-dedup), plus
P2/P3 findings.

**Attempt 2** — after full in-plan remediation, three verification personas were re-dispatched
against the remediated plan, selected as the sources of every P0/P1: Learnings Researcher (owner of
the P0), Go Reviewer (owner of the technical P1s), Constitution Reviewer (owner of the compliance
P1s). Verdicts: **Learnings Researcher PASS**, **Go Reviewer ADVISORY** (no remaining P0/P1),
**Constitution Reviewer ADVISORY** (one new P1, remediated below). The Architecture Strategist,
Scope Boundary Auditor, Security Lens Reviewer, and Agent-Native Parity Reviewer were not
re-dispatched: every finding they raised was P1-or-lower and each was remediated in-plan and
independently re-verified by one of the three attempt-2 personas covering the same surface.

### Gate rationale

The gate opens because no P0 or P1 finding remains open. The single P0 — an undisclosed
contradiction with this repository's standing "preserve at a re-persist seam" resolution — is
resolved by citation, explicit distinction on three named grounds, and rescoping the carrier
assertion from a permanent ban to a decision-anchored revisit marker. All sixteen attempt-1 P1
findings were remediated inside the plan rather than deferred. The one new P1 raised in attempt 2
(U9b's ordering expressed as prose caution rather than a hard merge gate) was remediated
immediately and is recorded below.

### Hardening satisfaction

The `plan-harden` output is intact and was itself a review target. Attempt 1 falsified invariant I3
as originally written; it is now a state-scoped table with a named, tested out-of-scope class.
Invariant I1 was upgraded from a prose table to an executable requirement (a write-site enumeration
test covering `internal/events` and `internal/core`; the alternative gated-seam mechanism was
withdrawn in PR #377 review cycle 3 rather than left as an open branch). Invariant I2 gained the corrected
`AbandonCheckpoint` gate placement and a named, tested residual for the already-resolved
short-circuit. The consulted-learnings table gained three learnings the reviewers proved were
directly applicable and missing.

### Findings by severity

**P0 — 1, remediated**

| ID | Source | Finding | Remediation |
|---|---|---|---|
| P0-1 | Learnings Researcher | Plan chose refuse while contradicting `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`, the repo's standing preserve-at-seam resolution, without citing or distinguishing it; U2b read as a permanent ban on any carrier field. | Added to the consulted-learnings table; added a Decisions and Rationale row distinguishing it on ownership, namespace direction, and the existing open `context` carrier, while explicitly honouring its seam-enforcement half; R5/U2d recast as decision-anchored. |

**P1 — 17 raised (16 in attempt 1, 1 in attempt 2), all remediated**

| ID | Source | Finding | Remediation |
|---|---|---|---|
| P1-A | Go, Security, Parity | `handleResolveCheckpoint` routes through `domainError`, which has no case for the new sentinels, so every new refusal would surface as `InternalError` (500). Response shapes `checkpointUnknownFieldsResponse` and `checkpointDispositionErrorResponse` are incompatible. | U7 rewritten: add all three sentinels to `domainError`, add `ErrCheckpointNonConforming` to `checkpointDispositionError`, extend `checkpointDispositionErrorResponse` with non-`omitempty` `UnknownFields`, and assert the payload is not `"error":"internal"` by invoking the handler. |
| P1-B | Go, Scope, Architecture | Invariant I3's totality claim was provably false: a valid, conforming `status:"resolved"` file is refused by abandon (`ErrCheckpointNotActive`) and by quarantine (`ErrCheckpointUseAbandon`). | I3 rewritten as a state-scoped table; the double-refusal row named as a pre-existing out-of-scope state-conflict class; U5b added to pin it; stop condition narrowed to the `active` scope. |
| P1-C | Go | U3's `fmt.Errorf("%w: %v", ...)` drops `ErrCheckpointInvalid`. | Q2 and U3 now mandate multi-`%w`; U3's test asserts both `errors.Is` checks. The pre-existing `%v` in `AbandonCheckpoint` is recorded as a documented deviation and follow-up rather than silently claimed compliant. |
| P1-D | Constitution, Scope, Architecture | Closure signals collided with reality: after U3, every post-merge session start hits nine refusals, tripping the one-occurrence rollback trigger on correct behaviour. | Healthy signal, failure signal, and rollback trigger all rewritten to discriminate the enumerated nine legacy filenames; pre-merge operator acknowledgement added; blocked-path handling added. |
| P1-E | Constitution | `Constitution Check: pass` was unearned (overclaims on I, V, VII, VIII). | Verdict changed to `documented-deviations`; a Documented Deviations subsection added with principle, justification, and rejected simpler alternative; rows I, V, VII, VIII corrected. |
| P1-F | Constitution | No *compiling* red phase — every harness referenced undeclared symbols, producing build errors rather than red assertions. | A mandatory two-step red posture (declaration stub → harness) added at the head of Implementation Units, with `Expected red` stated per unit; the two pure regression guards declare their exemption explicitly. |
| P1-G | Parity, Architecture, Learnings | `.github/instructions/backlogit.instructions.md` (`applyTo: '**'`) teaches every agent that `resolve` is infallible and that the abandon/quarantine split is validity-only; only the design doc was being updated. | U9b added: instruction-file update, the new refusal codes, and a documented body-preserving hand-repair procedure so quarantine is not the only escape. |
| P1-H | Parity | U7 did not specify exact replacement tool-description strings. | U7b added with verbatim current → replacement strings for all five affected tools. |
| P1-I | Parity | `GetCheckpoint` hardcodes `valid: true`, contradicting `ListCheckpoints`' `needs_quarantine: true` and steering agents to the wrong verb. | Q3 rewritten to override the source document's `ListCheckpoints`-only scope; U6b added covering `GetCheckpoint` on both surfaces with `conforming` as a new field rather than a redefinition of `valid`. |
| P1-J | Learnings | `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` Rule 3 requires a cross-surface guard to exercise CLI and MCP from the same stored state; U7/U8 used independent fixtures. Rule 1 requires registry honesty for changed descriptions. | U8b added with one shared fixture table driven through both surfaces; U7b adds the registry-drift re-run and `.autoharness/backlog-registry.yaml` update in the same commit. |
| P1-K | Go | Gate ordering: U4's conformance gate sat after the already-abandoned short-circuit, letting a non-conforming already-abandoned file return `nil` from abandon while quarantine accepted it. | U4 gate moved to immediately after `ValidateCheckpoint` and strictly before the short-circuit; I2 records both load-bearing orderings; a test asserts the non-`nil` result. |
| P1-L | Go | The conformance predicate was "no unknown keys", not round-trip safety, so `"status"` + `"Status"` passed and was then collapsed. | U2c added: `strings.EqualFold`-equal top-level keys are non-conforming, reported as `duplicate:<key>`; Decisions row records the corrected predicate name. |
| P1-M | Architecture, Learnings | `2026-07-17-backlogit-update-drops-archive-provenance.md` and `2026-07-28-attach-commit-repersist...` and `2026-07-03-cli-mcp-honest-fallback-map...` were absent from the consulted-learnings table. | All three added with the concrete plan change each forced. |
| P1-N | Scope, Architecture | Dependency graph was wrong: missing `U4 → U5`; `U6 → U7` drawn but undeclared; prose claimed U3/U4/U5/U6 were mutually independent, contradicting U5's own declared dependencies. | Graph redrawn, every edge given a declared reason in a table, and the independence prose corrected. |
| P1-O | Constitution, Scope | U10's scratch workspace lacked root-pinning, an ignore rule, and a containment assertion; its teardown was an unclassified deletion. | U10 pins the path to `docs/scratch/checkpoint-verification/`, asserts repo-root containment before first write, adds the ignore rule, and classifies teardown as `ActionRisk: destructive` (A4b) requiring approval. |
| P1-P | Security, Parity | Verification relied on the pinned repo-root `backlogit.exe`, which predates the change, and on visual hash comparison of the live corpus. | U10 requires a fresh binary built from the branch HEAD and a programmatic before/after SHA-256 comparison of all eleven live files. |
| P1-Q | Constitution (attempt 2) | U9b's ordering requirement was prose caution ("or immediately after within the same PR"), leaving a window in which the behaviour change merges without the agent-instruction update. | Rewritten as a hard merge gate: a PR containing U3b, U4, or U5 MUST NOT be merged unless the `.github/instructions/backlogit.instructions.md` delta is in the same merge commit. |

**P2 / P3 — acknowledged**

Remediated in-plan: the `AbandonCheckpoint` gate placement, the duplicate/fold-variant rule, the
`ValidationErr` append rather than overwrite, POSIX-safe `RemediationCommand` (relabelled from
"PowerShell-safe" in PR #377 review cycle 7 with a Windows-first-workspace rationale — see U6),
the untagged-exported-field escape hatch in `modeledJSONTagKeys` (U2d test 3), the
unquarantine/restore
runtime row, the `U2b` merge into a coherent U2 family, the conformance branch ordering ahead of the
list filter block, and the archive-collision risk row.

Accepted without change: the narrowed `CleanupCheckpoints` Windows `os.Remove` collision (attempt 2
established that quarantine and cleanup operate on different directories, so the plan does not
materially worsen it — carried as a Ship-handoff follow-up); the U3b/U6 cross-unit granularity
advisory (converted into an explicit pre-start count check in U3b rather than a plan rewrite).

### Runtime and closure gaps

None open. Runtime verification names the surface, the fixture class, and the byte-level assertion
per unit; closure declares discriminating healthy/failure signals, a rollback trigger that cannot
fire on correct day-one behaviour, an owner, a validation window, a pre-merge acknowledgement, and
a blocked-path procedure. Four out-of-scope follow-ups are named rather than absorbed: the nine
live legacy checkpoints plus the stale `129-S` checkpoint, `AbandonCheckpoint`'s `%v` wrap, refusal
observability, and the `CreateCheckpoint` same-second filename collision.

<!-- plan-review-attempt: 1 -->
<!-- plan-review-attempt: 2 -->

### PR #377 Copilot review remediation (post-harvest)

Eight Copilot review threads on PR #377 were reconciled against this plan and the harvested tasks.
All eight were confirmed valid against the current source; one was partially valid on its premise
and valid on its remedy. The plan deltas are recorded here so the reviewed artefact and the backlog
stay in step.

| Thread | Finding | Plan delta |
|---|---|---|
| `147.003-T` nested progress | `unknownNestedProgressKeys` map-decodes `progress`, so nested duplicate and fold-variant keys collapse before the diff — the same loss class U2c closes at the top level. | **U2e added** (read-boundary ordered walk, `duplicate:progress.<key>`); U2b gains a scope note; edges `U2b, U2c → U2e → U9`. |
| `147.012-T` get result shape | U6b had no implementable carrier: `GetCheckpoint` returns `*CheckpointV1`, `NeedsQuarantine` / `RemediationCommand` live only on `CheckpointSummary`, and the MCP handler hardcodes `"valid": true` with no unit owning the projection. | U6b now declares `CheckpointReadResult` + `GetCheckpointResult` with `GetCheckpoint` retained as a wrapper; **U6c added** for the MCP projection; R8 retraced to U6, U6b, U6c. |
| `147.014-T` resolve error code | The harvested description row named `checkpoint_use_quarantine` for unmodeled top-level keys, contradicting U7's `ErrCheckpointNonConforming` → `checkpoint_non_conforming` mapping. | Task-only fix; the plan's verbatim strings were already correct. |
| `147.016-T` parity fixtures | The harvest collapsed the three-row fixture matrix to one legacy file and asserted `conforming: false` on a document `GetCheckpoint` rejects with `ErrCheckpointInvalid`. | Task-only fix; the plan's three-row table was already correct. |
| `147.018-T` repair path | Three different recovery procedures were in circulation (U9b in-place repair, U10 restore-then-repair, harvested "create a fresh checkpoint"). | U9b now states **one** procedure with two entry points — direct repair and post-quarantine restore — and explicitly rejects creating a replacement checkpoint. |
| `147.019-T` verification hardening | The harvest dropped scratch-path pinning, the containment assertion, ignore-rule ownership, the branch-built binary, and full-corpus hashing. | U10 now owns the `.gitignore` rule, asserts repo-root containment before first write, and hash-compares **every** file under `.backlogit/checkpoints/` rather than a count-pinned subset. |
| `memories.json` | The new continuity key carried an empty value. | No plan delta; the memory record was populated. |
| `147.005-T` I1 bundling | U2d bundled the I1 write-site enumeration — a possible production seam — into a declared green-on-landing regression-guard unit, breaching width isolation. | **U2f added** for I1 with edges `U2d → U2f → U3b, U4`; the "Making I1 executable" paragraph reassigned from U2d. |

Net effect: 19 implementation units become 22 (U2e, U2f, U6c added); the dependency graph gains
nine edges. No unit was removed, no task ID was renumbered, and the reviewed decision, scope, and
test-first ordering are unchanged.

<!-- copilot-review-remediation: pr-377 -->

### PR #377 Copilot review remediation, cycle 3

A third Copilot review (`PRR_kwDORzozKM8AAAABKsDFyQ`) against head
`d6c11c5ef55d2a053cf1c05f488feb70743a4359` raised eight threads under the summary "several
implementation contracts are inconsistent, and multiple tasks violate mandatory granularity
limits". All eight were confirmed valid against `internal/mcp/errors.go`, `internal/mcp/tools.go`,
and `internal/events/checkpoint_lifecycle.go`; two were valid on the remedy while carrying a
partly incorrect premise, and the correction is recorded in the owning task rather than silently
adopted.

| Thread | Finding | Plan delta |
|---|---|---|
| `PRRT_kwDORzozKM6b18Ht` (`147.011-T`) | U6's acceptance criteria claimed schema-invalid documents are "already surfaced" by `list_checkpoints`. They are not: the `valErr` branch of `ListCheckpoints` falls through into the Agent / Status / ShipmentID / FeatureID / MaxAge filter block, so a quarantine candidate can be filtered out of the very listing that advertises its remedy. | **U6d added** — quarantine candidates bypass the filter block, matching the shipped parse-failure precedent. U6's false guarantee removed and replaced with an "ordering is not exemption" note. Edge `U6 → U6d`. |
| `PRRT_kwDORzozKM6b18I5` (`147.021-T`) | U2f offered two mechanisms — an enumeration test **or** an exported `events.RewriteCheckpointFile` seam — so its true size was one new test file or five files across two packages and two skill domains. Unknowable scope, and the larger branch breaches both the three-file heuristic and width isolation. | U2f **commits to enumeration only**; the seam is withdrawn. A halt condition replaces it: if the enumeration proves unimplementable, U2f goes `blocked` and the seam is re-planned under a new ID. Recorded in Decisions and Rationale. |
| `PRRT_kwDORzozKM6b18GZ` (`147.022-T`) | U6c's third acceptance criterion asserted MCP `get_checkpoint` returns `code: checkpoint_use_quarantine` for a schema-invalid document. `handleGetCheckpoint` routes through `domainError`, which takes no filename and cannot emit disposition codes; the criterion was unreachable. | U6c's criterion retargeted to the **reachable, pre-existing** `validation_failed` refusal. Dependency on U7 dropped (U6c now needs only U6b). Edge `U7 → U6c` removed. Decision recorded: disposition codes stay on the mutation verbs. |
| `PRRT_kwDORzozKM6b18HQ` (`147.013-T`) | U7 mixed `internal/mcp/errors.go` sentinel mapping with a `tools.go` handler-routing change, and its criterion claimed `domainError` has "no case for `ErrCheckpointInvalid`". It has one — grouped with `ErrValidation` and `ErrCheckpointCorrupt` — so only two sentinels are genuinely absent. | U7 trimmed to `errors.go` only (two new cases). **U7d added** for the `handleResolveCheckpoint` routing change. The false "no case" claim corrected. Edges `U1, U7 → U7d`. |
| `PRRT_kwDORzozKM6b18Js` (`147.014-T`) | U7b covered five tool descriptions across two skill domains — read surfaces and mutation surfaces — in one unit, breaching width isolation. | U7b reduced to the **two read-surface** rows; **U7c added** for the **three mutation-surface** rows. U7b's dependency retargeted from U7 to U6c. Edges `U7, U7d → U7c → U8b`. |
| `PRRT_kwDORzozKM6b18IM` (`147.018-T`) | U9b's repair entry point (a) told operators to "move unmodeled keys into `context`" without classifying which offenders that actually works for. `duplicate:progress.<key>` cannot be repaired that way at all, and a blind move can produce a new duplicate. | Entry point (a) gains a five-row **offender classification table** (`<key>`; `duplicate:<key>` where one side is modeled; `duplicate:<key>` where both unmodeled with distinct spellings; `duplicate:<key>` where both unmodeled with identical spelling — exact duplicates, not auto-repairable; `duplicate:progress.<key>`) plus a **termination rule** — re-run the conformance check after each repair, and stop after one round-trip. A generalized **exact-duplicate safety invariant** states the no-implicit-survivor rule applies regardless of modeled/unmodeled status. |
| `PRRT_kwDORzozKM6b18Ip` (`147.019-T`) | U10 required every file under `.backlogit/checkpoints/` to be byte-unchanged **and** a recovery sweep that "succeeds on every other file" in that same directory. Success means a rewrite; the two requirements were mutually exclusive. | The recovery sweep moves to a **copied mirror** at workspace root `docs/scratch/checkpoint-verification/mirror/` (checkpoints in `.backlogit/checkpoints/` inside it); all sweep CLI invocations use `--cwd docs/scratch/checkpoint-verification/mirror` with bare filename arguments. The live directory stays hash-guarded and read-only. Mirrored filenames are preserved so the nine-name assertion is unchanged. |
| `PRRT_kwDORzozKM6b18JP` (`147.019-T`) | U10 carried five verification rows spanning refusal, acceptance, restore, and a recovery sweep — over the four-scenario limit and over the two-hour rule. | U10 keeps **three refusal-path rows**; **U10b added** with the three acceptance / restore / sweep rows. Edge `U10 → U10b`. |

Additional drift found while reconciling and fixed in the same pass, not raised by the review:
`147.016-T`'s parity table named `checkpoint_use_quarantine` for the MCP `get` legacy-shaped row —
the same impossible code the `147.022-T` thread flagged one surface over. Corrected to
`validation_failed` for consistency with U6c.

Net effect: 22 implementation units become 26 (U6d, U7c, U7d, U10b added); one edge removed
(`U7 → U6c`), seven added (`U6 → U6d`, `U1 → U7d`, `U7 → U7d`, `U7 → U7c`, `U7d → U7c`,
`U7c → U8b`, `U10 → U10b`), and two retargeted (`U7b` from U7 to U6c, `U6c` off U7) — 34 `blocks`
edges in the harvested task graph become 40. No unit was removed, no
task ID was renumbered, and the reviewed decision, scope, data-loss safety posture, and fail-closed
refusal remain unchanged.

<!-- copilot-review-remediation: pr-377-cycle-3 -->

### PR #377 Copilot review remediation, cycle 4

A fourth Copilot review (`PRR_kwDORzozKM8AAAABKsXFuA`) against head
`bb4879237ca2ada40cf3416530563acbbecd6ac9` raised five threads under the summary "the plan omits
the required CLI get projection unit, relies on disposable cross-task state, and contains
inconsistent lifecycle metadata". This cycle is an **operator-authorized extension** past the
three-cycle limit, recorded as such rather than treated as a silent counter reset. All five were
confirmed valid against `internal/cli/checkpoint.go`, `internal/events/checkpoint_lifecycle.go`,
and the staged artifacts; one is valid on the remedy while carrying a partly stale premise, and one
is only partially remediable without violating the tool-managed data-ownership rule.

| Thread | Finding | Plan delta |
|---|---|---|
| `PRRT_kwDORzozKM6b2qq7` (`147.015-T`) | U8 dropped the CLI `checkpoint get` projection that U6b produces and U8b's parity table consumes. `newCheckpointGetCmd` (`internal/cli/checkpoint.go:180-210`) still calls `events.GetCheckpoint` and prints a hardcoded `valid: true`, while U8 specifies only the resolve/abandon rows and carries no dependency on U6b. Folding it into U8 would add a fourth scenario and breach the task limit. | **U8c added** (`147.027-T`) — a three-scenario, single-domain unit that reprojects `newCheckpointGetCmd` from `events.GetCheckpointResult`. U8's change note now states the projection moved out and that U8 must not touch `newCheckpointGetCmd`; U6c gained the reciprocal note that it must not touch the CLI. Edges `U6b → U8c → U8b`. Added to the `130-S` manifest and to U8b's dependency list. |
| `PRRT_kwDORzozKM6b2qrn` (`147.026-T`) | U10b consumes U10's scratch workspace — the branch-built binary, the copied fixtures, and the quarantine archive its second row restores from — but U10 owned an approved teardown of that directory at its own completion. Ordering was left to chance, and the losing order silently destroys U10b's inputs. | **Teardown ownership moved to U10b**, after all three of its rows pass. U10 now hands the workspace over intact; U10b gained an explicit *Inherited inputs and teardown* bullet making the live workspace a precondition and a missing workspace a blocker on re-running U10 rather than a licence to hand-rebuild. The Constitution VII deviation, the conflict-resolution row, risky action **A4b**, and the Teardown bullet all re-attribute to U10b; the `ActionRisk: destructive` classification and approval requirement are unchanged. |
| `PRRT_kwDORzozKM6b2qsD` (`147.023-T`) | U6d makes `ListCheckpoints` stop applying its documented Agent / Status / ShipmentID / FeatureID / MaxAge filters to quarantine candidates, but changed no caller-facing contract. An exported function silently ignoring its own documented options is a trap for every caller that is not this feature. | U6d now **publishes** the exemption: the exported `ListCheckpoints` doc comment states that quarantine candidates bypass the filter block and that filtered results may therefore include entries mismatching status, agent, feature, shipment, or age. A sixth acceptance criterion covers the doc comment, and its expected-red note records that it is a contract obligation rather than a fourth test scenario — the three-scenario budget is untouched. U7b's agent-facing `list_checkpoints` description carries the same sentence and gains a `U6d → U7b` edge so a published description can never promise unshipped behaviour. |
| `PRRT_kwDORzozKM6b2qsr` (`.backlogit/hooks_queue.jsonl`) | The committed event stream stops at the original 19-task shipment creation, so consumers polling this durable queue never receive creation signals for `147.020-T` through `147.026-T`. | Partially remediable. Every cycle-4 mutation was executed through the **supported backlogit CLI surface** against this worktree, so nine genuine events (seq 2305-2313) landed: one `create_artifact` for `147.027-T` and eight `update_artifact` events covering `147-F`, `147.014-T`, `147.015-T`, `147.016-T`, `147.019-T`, `147.022-T`, `147.023-T`, and `147.026-T`. Retroactive `create_artifact` events for artifacts that already exist cannot be truthfully emitted, and hand-appending them is forbidden by the tool-managed data-ownership rule. Reverting the file was rejected: the existing rows are genuine tool-emitted history, and reverting while the worktree runtime keeps appending forks the sequence space. **Residual**: `147.020-T`, `147.021-T`, `147.024-T`, and `147.025-T` still have no lifecycle event; each emits a genuine `update_artifact` the moment Ship claims it, and all four remain discoverable through the index and the queue view. |
| `PRRT_kwDORzozKM6b2qtQ` (`checkpoint-20260824-191617.json`) | The handoff labelled PR #377 "CI green" while `push_state` said the cycle-3 remediation was local-only, so Ship could resume treating unvalidated code as validated. | Premise partly stale — the branch **is** pushed and CI **is** green on `bb487923`. The artifact was still internally inconsistent and goes stale the instant cycle 4 commits. Fixed in the `context` namespace **only**, because the top-level namespace is closed and this very feature refuses unmodeled top-level keys: `ci_state` now records the last verified green SHA and the six named checks, `push_state` and `pr` carry the local-only cycle-4 tip, and `resume_hint` requires push plus fresh CI before merge is even considered. `task_ids` gained `147.027-T`; `review_remediation` gained the cycle-4 entry. |

Net effect: 26 implementation units become 27 (U8c added); three edges added (`U6b → U8c`,
`U8c → U8b`, `U6d → U7b`) — 40 `blocks` edges in the harvested task graph become 43 — and the
`130-S` manifest grows from 27 to 28 members. Teardown ownership moved from U10 to U10b without
changing its risk class. No unit was removed, no task ID was renumbered, and the reviewed decision,
scope, data-loss safety posture, and fail-closed refusal remain unchanged.

<!-- copilot-review-remediation: pr-377-cycle-4 -->

### PR #377 Copilot review remediation, cycle 7

A seventh Copilot review against head `122cdf30723196f8ebdeb9e1ce9ae2a04e5bdf69` (post-cycle-6)
raised six comments, grouped by four root causes: canonical checkpoint stale at cycle 4; canonical
memory stale at cycle 3; wrong get-result producer/projection task references in `147.018-T`; a
cross-platform shell contract conflict between the plan's/tasks' "PowerShell-safe" claim and the
shipped POSIX single-quote-escape idiom in `remediationQuarantineCommand`
(`internal/events/checkpoint_lifecycle.go:274-290`); the same conflict mirrored in plan units
U6/U8/U10 and their runtime verification row; and U9 generated-doc-ownership drift in `147.017-T`.
Cycles 5, 6, and 7 are **operator-authorized extensions** past the original three-cycle limit,
recorded as such rather than treated as silent counter resets.

**Design decision — cross-platform shell contract.** `RemediationCommand` is defined as
**POSIX-safe**, matching the shipped implementation and the shipped
`TestListCheckpoints_RemediationCommandIsShellSafe` assertion. It is **not** claimed
PowerShell-safe for arbitrary accepted filenames: the POSIX `'\''` close-escape-reopen idiom that
`shellQuoteSingle` emits is not a valid PowerShell literal (PowerShell escapes a single quote
inside a single-quoted string as doubled `''`), and no single command string can be paste-runnable
in both shells for a filename containing a single quote. Two alternatives were considered and
rejected:

* **Separate shell-specific renderings** (add `remediation_command_posix` and
  `remediation_command_powershell`) — doubles the read-surface API, requires new MCP/CLI
  projections on U6b/U6c/U8c/U8b, needs new test scenarios, and grows the width of at least four
  tasks. Justified only if PowerShell-native support is a hard requirement, which the Windows-first
  workspace note does not actually establish: the generator produces safe filenames that are
  literally paste-safe in `pwsh` without any escape.
* **Filename grammar restriction** to `[A-Za-z0-9._-]+` — requires product-code change
  (`validateCheckpointFilename`), a new refusal path that has no existing regression to guard
  against (no legacy filename on disk contains a shell metacharacter), and a new width-isolated
  task at minimum. Justified only if the "PowerShell-safe" claim cannot be honestly reframed,
  which it can.

**Why POSIX-safe is clearly safer than the current "PowerShell-safe" claim.** The current claim is
empirically false: pasting `'\''` into `pwsh` parses as three string tokens, not as an escaped
single quote. Correcting the claim to POSIX-safe matches the shipped code and shipped test,
removes a false invariant that agents and reviewers would otherwise try to enforce, and preserves
the practical Windows recovery path via the generator-shape note: `CreateCheckpoint`
(`internal/events/memory.go:59`) only writes `checkpoint-YYYYMMDD-HHMMSS.json`, which contains no
shell-special characters and is therefore literally paste-safe in `pwsh` for every
backlogit-generated file. The POSIX-safe form only differs from a hypothetical PowerShell-safe
form for filenames containing single quotes, spaces, or other metacharacters, which the generator
does not produce; a Windows operator who encounters such an out-of-band file recovers via Git Bash
/ WSL / `bash -c`, hand-adapts the `'\''` escape to `''`, or renames the file to the generator's
shape.

**Grounds** (grounded in existing code, validation rules, call sites, CLI/MCP projections, and
tests, not by assertion):

* Shipped `remediationQuarantineCommand` and `shellQuoteSingle`
  (`internal/events/checkpoint_lifecycle.go:274-290`) emit the POSIX `'\''` escape idiom.
* Shipped `TestListCheckpoints_RemediationCommandIsShellSafe`
  (`internal/events/checkpoint_lifecycle_test.go:430-455`) uses filename
  `checkpoint-weird 'name' & rm -rf.json`, comments "safe to run verbatim in a POSIX shell", and
  asserts the POSIX-safe escape output — this is the test's intent by construction.
* `validateCheckpointFilename` (`internal/events/checkpoint_lifecycle.go:254-273`) rejects only
  empty, path-separator-bearing, or non-`checkpoint-*.json` names; it does not restrict interior
  characters. So the accepted filename grammar today already includes shell-special characters,
  which the POSIX escape correctly handles and any PowerShell claim cannot.
* CLI/MCP projections in U6b/U6c/U8c reproject the shipped `RemediationCommand` string verbatim;
  no projection layer re-quotes or re-renders it. The read-surface contract is therefore whatever
  the events layer emits, and the events layer emits POSIX-safe.
* No shipped call site claims PowerShell-native invocation of the returned command; the plan/task
  text was the only source of the "PowerShell-safe" invariant.

**Backlog shape effect: unchanged.** This is a documentation correction — no product-code change,
no new exported API, no new width-isolated task. The 27-task / 43-edge / 28-shipment-member shape
is preserved. The alternative (adding a U6e width-isolated task) was considered and rejected: it
would ship additional runtime surface and additional test scenarios to enforce a contract the
shipped code and shipped test already meet honestly under the POSIX-safe framing.

| Thread | Finding | Plan / backlog delta |
|---|---|---|
| checkpoint handoff stale | `.backlogit/checkpoints/checkpoint-20260824-191617.json` recorded cycle-4 state and obsolete push/review flags; it did not name the reviewed HEAD versus the local unreviewed commits generated in cycles 5, 6, and 7. | Regenerated with cycle-7 `context` metadata: `reviewed_head` records the last cycle-6 head Copilot reviewed against, `local_unreviewed_state` records the cycle-7 tip that Ship must push and re-review, `review_remediation` gained cycle-5, cycle-6, and cycle-7 entries. Schema-valid: only `context` keys are added; the top-level namespace stays closed, matching the very invariant this feature exists to enforce. |
| canonical memory stale | `.backlogit/memories.json` key `stage-d3ce9e81-checkpoint-toplevel-keys` stopped at cycle 3 (26 tasks / 40 edges) and did not describe cycles 4-7. | Regenerated through cycle-7 final state: 27 tasks / 43 edges / 28 shipment members (unchanged shape from cycle 4), cycle-4 U8c addition documented, cycles 5-6 recorded (backlog-only remediation, no shape change), cycle-7 shell-contract decision recorded with grounds and rejected alternatives, reviewed-head vs local-unreviewed handoff pinned. |
| repair task IDs wrong | `147.018-T` cited `(147.010-T / U6b, 147.011-T / U6c, 147.023-T / U8c)` for the get-result producer and its projections. Actual mapping: 147.010-T is U5b (I3 state classification), 147.011-T is U6 (list surface), 147.023-T is U6d (filter exemption). None produces the get-result. | Corrected to `(147.012-T / U6b, 147.022-T / U6c, 147.027-T / U8c)`. 147.012-T declares `GetCheckpointResult`, 147.022-T projects it onto MCP `get_checkpoint`, and 147.027-T projects it onto CLI `checkpoint get` — the actual get-result producer and its two projections. |
| shell contract conflict on 147.011-T | 147.011-T body and acceptance claimed `RemediationCommand` was PowerShell-safe. The shipped implementation is POSIX-safe. Neither is paste-runnable in the other shell for a filename containing a single quote. | 147.011-T body and acceptance now claim POSIX-safe with a Windows-first-workspace note explaining why the natural filename generator produces shapes literally paste-safe in `pwsh` for every backlogit-generated file. 147.015-T (U8) and 147.019-T (U10) carry the same correction. No new dependency, no new task, no code or exported API change. |
| shell contract conflict in plan | Plan units U6, U8, and U10, and the Runtime Verification row for `U6, U6b`, mirrored the "PowerShell-safe" claim. | Plan U6, U8, and U10 relabelled to POSIX-safe with the Windows-first-workspace generator-shape rationale reproduced once in U6 and cross-referenced from U8/U10. Runtime Verification row for `U6, U6b` says "POSIX-runnable" and references the same rationale. Plan-hardening remediation register updated to record the relabelling and cross-reference the decision. |
| U9 generated-doc ownership drift | 147.017-T listed only `docs/design-docs/checkpoint-administrative-disposition.md` in Files, and had no CLI Reference Drift acceptance criterion. Plan U9 assigns regenerated `docs/cli-reference/backlogit_checkpoint_*.md` **and** the CLI Reference Drift check to U9. | 147.017-T Files list now includes `docs/cli-reference/backlogit_checkpoint_*.md`, the body records the regeneration obligation and its trigger (any U6/U6b/U8/U8c help-text or output-projection change), a "CLI Reference Drift check clean" acceptance criterion is added, and an explicit acceptance criterion states `.github/instructions/backlogit.instructions.md` is not touched by this unit (owned by 147.018-T / U9b). Ownership of U9b's agent-instruction file is preserved. |

Net effect: no unit added, no edge added, no shipment member added, no task ID renumbered. Backlog
shape stays at **27 tasks / 43 edges / 28 shipment members**. The reviewed decision, scope,
data-loss safety posture, fail-closed refusal, 147.018-T same-merge requirement, and the
147.009-T paired-assertion halt condition are all unchanged. The shell-contract decision preserves
shipped behaviour (POSIX-safe, matching the shipped test) rather than replacing it.

<!-- copilot-review-remediation: pr-377-cycle-7 -->

### PR #377 Copilot review remediation, cycle 8

An eighth Copilot review against the current PR head flagged three tasks whose bodies claimed a
"regression guard — green on landing, no red phase" exemption from P-004: 147.005-T (U2d),
147.010-T (U5b), and 147.016-T (U8b). The reviewer is correct — the harness workflow-policy
excerpt in `.github/policies/workflow-policies.md` P-004 defines the harness-ready label as
"harness compiles and fails RED" and does not carve out a regression-guard exemption. Prior
cycles allowed those tasks to be declared exempt on the grounds that they pin pre-existing
shipped behaviour; that framing is retired here, and each of the three units is rebalanced to
own a real red assertion that fails against the pre-implementation state, without inventing a
speculative seam or fabricating production behaviour solely to make a test go red.

| Thread | Issue | Cycle-8 correction |
|---|---|---|
| P-004 all-guards exemption text | The Test-First posture section and Constitution Check II declared U2d, U5b, and U8b exempt from the two-step red rule on the grounds that they are pure regression guards. P-004 does not permit this: the harness-ready label requires the harness to compile and fail on assertions before implementation. | The "all-guards exemption" language is removed from the Test-First posture section (a unit may still contain guards, but at least one case must fail against the pre-delta state) and from Constitution Check II. Each of U2d, U5b, and U8b is rebalanced below to state its concrete red load. |
| 147.005-T / U2d — regression-guard exemption on a code unit | U2d claimed an exemption while U2 owned the introduction of the `checkpointV1AllTopLevelKeys` derived set and the refactor of `CheckConformingTopLevelNamespace` to consult it. That left U2d with only invariant assertions and no red load. | The derived-set introduction and the conformance-check refactor **move from U2 to U2d**. U2 keeps its inline two-set check against `checkpointV1TopLevelKeys` and `checkpointV1ReservedKeys`. U2d's declaration step lands `var checkpointV1AllTopLevelKeys = map[string]struct{}{}` empty; U2d's harness step lands three cases including a set-equality assertion that **fails RED** against that empty stub; U2d's implementation step fills the derivation and refactors the conformance check to consult the single set. The task and file lists are updated accordingly. No task added, no dep edge changed. |
| 147.010-T / U5b — "no production change" test-only task | U5b claimed an exemption and declared "no production change" while pinning the state-scoping of invariant I3. That left U5b without a red load. | U5b **gains a real state-scoped classification delta**: extend `QuarantineCheckpoint`'s in-memory classification so a valid+conforming target whose `Status != "active"` is refused with `ErrCheckpointNotActive` wrapped as `%w: status=%s` — not the bare `ErrCheckpointUseAbandon`. This closes a pre-existing misleading redirect (a `status:"resolved"` conforming target is currently pointed at abandon by quarantine, and abandon then refuses on `ErrCheckpointNotActive` — the operator is bounced between verbs with neither surface naming the actual state-class problem); after U5b, both mutation verbs converge on the same truthful state-class sentinel for the non-active conforming case, so the pre-existing double-refusal state-conflict class named in the Decisions and Rationale is disclosed at first contact. `errors.Is(err, ErrCheckpointUseAbandon)` continues to hold for the sole active-scope classification remainder (U5's row-2 hard gate), and `errors.Is(err, ErrCheckpointNotActive)` now holds on the quarantine surface for the non-active conforming case, matching abandon's existing shape. The MCP mapping layer (`internal/mcp/errors.go:329`) already routes `ErrCheckpointNotActive` to `checkpoint_not_active`, so this change adds no new MCP code and no U7/U7d follow-up. U5b's row-1 case asserts `errors.Is(err, ErrCheckpointNotActive)` on the quarantine surface for a `status:"resolved"` conforming target and **fails RED** against U5's landing state where `QuarantineCheckpoint` returns bare `ErrCheckpointUseAbandon`. Rows 2 and 3 remain declared regression guards (abandon's status guard on the same document; U5's row-2 refusal on an active conforming document). Files list gains `internal/core/checkpoint_disposition.go`. Quarantine is not widened to accept the state-conflict class; U5's active-scope hard gate is preserved; no new sentinel; no error-message enrichment as end-goal — the observability wrap proposed in the prior cycle-8 iteration is retracted as out-of-scope refusal-observability, and this classification-scoped delta replaces it. No task added, no dep edge changed. |
| 147.016-T / U8b — "Expected red: none" against batch harness contract | U8b claimed an exemption on the grounds that it lands after every dep's implementation. That framing was incompatible with the batch harness contract: harness generation lands before impls, so U8b's assertions run against the deps' declaration stubs and current handlers. | The Expected Red section is restructured to enumerate specific failing assertions per fixture row against the pre-implementation state: legacy-shaped row — `events.ErrCheckpointInvalid` refusal on `get` fails against pre-U6b/pre-U6c/pre-U8c handlers, `resolve` refusal with `checkpoint_use_quarantine` fails against pre-U3/pre-U7d; valid-but-non-conforming row — `conforming: false` projection assertions fail against the U6b declaration stub's zero-value result, U6c's unfilled projection, and U8c's hardcoded `"valid": true`; refusal-on-mutation assertions fail against pre-U3b/pre-U4 handlers; byte-identity postcondition fails against the current `ResolveCheckpoint` rewrite; conforming-active row stays as a declared regression guard. U8b remains test-only with parity-contract role after impls land; its cycle-8 red load is honest against the batch-harness moment. No task added, no dep edge changed. |

Net effect: no unit added, no edge added, no shipment member added, no task ID renumbered. Backlog
shape was **27 tasks / 43 edges / 28 shipment members** at cycle-8 close (subsequently reduced
to **26 tasks / 42 edges / 27 shipment members** by cycle-10's U5b retirement). Prior-cycle decisions
(checkpoint-safety design, shell contract, repair mapping, hard merge gate, 147.009-T
paired-assertion halt condition, ownership splits) are unchanged. The three affected tasks now
carry P-004-compliant harnesses with concrete pre-implementation-state red assertions.

<!-- copilot-review-remediation: pr-377-cycle-8 -->

### PR #377 Copilot review remediation, cycle 10

A tenth Copilot review (operator-authorized extension) flagged three findings:

| Thread | Issue | Cycle-10 correction |
|---|---|---|
| 147.016-T / U8b — `Conforming == false` zero-value ambiguity | The `valid-but-non-conforming` row's `result.Conforming == false` assertion is a bool zero value and does not prove `ListCheckpoints` populated the projection; an unset declaration stub also yields `false`. | U8b's harness now asserts **at least one required positive projected field alongside `Conforming == false`**: `Valid == true`, `NeedsQuarantine == true`, and `RemediationCommand != ""`. Against the declaration stub's zero-value result, all three fail — proving the projection is unpopulated rather than relying on the ambiguous zero-value alone. Plan U8b description and 147.016-T acceptance criteria updated to match. |
| 147.018-T / U9b — post-quarantine restore abort cleanup lacks P-VII approval gate | Entry point (b)'s abort step can delete an invalid restored active copy, but Constitution Principle VII requires explicit approval before every destructive file action. The approval-gate was missing. | U9b's entry point (b) now requires explicit operator approval for the **potential** abort cleanup **before** the restore begins (step 2). If approval is withheld, restoration must not start. The implementation task must carry this rule into generated instructions. Plan U9b description and 147.018-T acceptance criteria updated. |
| 147.010-T / U5b — production delta contradicts origin decision scope exclusion | U5b's cycle-8 production delta (changing the `QuarantineCheckpoint` refusal sentinel for non-active conforming targets from `ErrCheckpointUseAbandon` to `ErrCheckpointNotActive`) widens the decision's explicitly out-of-scope state-conflict class. Inventing a behaviour change solely to force a RED assertion does not justify reopening a scoped-out decision. | U5b is **retired**. Its state-conflict regression rows are absorbed into U5 (147.009-T) as already-green pinned guards — they test the I3 row-3 invariant without requiring any production change. U5 retains its genuine red gate (row 1). 147.010-T is archived (history preserved), removed from shipment 130-S, and the U5 → U5b dependency edge is removed. Plan U5b section replaced with a retirement notice; dep graph, edge table, execution order, Constitution Check II, I3 discussion, runtime verification table, and stop conditions updated. |

Net effect: **1 task retired** (147.010-T / U5b archived), **1 edge removed** (U5 → U5b), **1 shipment member removed** (147.010-T from 130-S). Backlog shape changes from **27 tasks / 43 edges / 28 shipment members** to **26 tasks / 42 edges / 27 shipment members**. No new task, no new edge, no new shipment member. Prior-cycle decisions (checkpoint-safety design, shell contract, POSIX-safe RemediationCommand, repair mapping, hard merge gate, 147.009-T paired-assertion halt condition, ownership splits) are unchanged. The duplicate modeled-key rule, schema-invalid quarantine rule, valid-nonconforming-only repair rule, operation-aware remediation formatting, and body-preserving repair hard same-merge gate are all preserved.

<!-- copilot-review-remediation: pr-377-cycle-10 -->

### PR #377 Copilot review remediation, cycle 11

An eleventh Copilot review (operator-authorized extension) flagged nine root causes across tasks, plan, memory, and checkpoint artifacts. All nine are resolved in this cycle without adding tasks, edges, or shipment members. Backlog shape remains **26 tasks / 42 edges / 27 shipment members**.

| Thread | Issue | Cycle-11 correction |
|---|---|---|
| U10b mirror workspace (147.026-T) | The recovery sweep described its mirror at `docs/scratch/checkpoint-verification/mirror/` but CLI operations (`checkpoint resolve`, `checkpoint abandon`, `checkpoint list`) use `--cwd` to locate `.backlogit/checkpoints/` — the mirror must be a proper workspace with a `.backlogit/checkpoints/` subdirectory. | Mirror copy target changed to `docs/scratch/checkpoint-verification/mirror/.backlogit/checkpoints/`; all sweep CLI invocations updated to use `--cwd docs/scratch/checkpoint-verification/mirror` with bare filename arguments. 147.026-T and plan U10b updated. |
| 147.010-T canonical archive lifecycle | The archive file was placed manually into `.backlogit/archive/` without going through `archive_item`, so it lacked `archived_status`, `archived_from`, and the canonical lifecycle metadata (JSONL log events, hook queue events). | 147.010-T restored to queue temporarily; archive copy removed; canonical archive file created with `archived_from`, `archived_status: done`, `status: archived`, and retirement notice; three JSONL log events and hook queue events emitted. Lifecycle metadata now matches the `archive_item` format. |
| U8b fixture isolation (147.016-T) | The conforming-active row accepts `abandon`, which mutates the checkpoint. The task stated every fixture must be byte-identical after all surfaces are exercised — which cannot hold when an accepted mutation rewrites the file. | Task body clarified: one canonical fixture per state; each mutating test case gets a fresh copy; byte-identity postcondition applies only to refused mutation paths; the conforming-active row's `abandon` acceptance asserts the intended rewrite/archive outcome, not byte identity. |
| ErrCheckpointCannotResolveAbandoned mapping (147.025-T) | The task body incorrectly stated that `ErrCheckpointCannotResolveAbandoned` already mapped to `validation_failed` through `domainError`. It has no explicit case and falls to `default: InternalError`. Case 4 therefore asserted a pre-existing behavior that did not exist. | Task body corrected: `ErrCheckpointCannotResolveAbandoned` falls to `default: InternalError` before U7d's explicit mapping. Case 4 is a genuine red delta: pre-impl `InternalError`, post-impl `validation_failed` with exact message `"resolve checkpoint: backlogit: checkpoint has been administratively abandoned; resolve is refused"`. Expected red updated. |
| gen-docs / output projection mismatch (147.017-T) | `gen-docs` renders Cobra help metadata, not runtime JSON projection. U8/U8c are output-only changes, not Cobra help-text changes, so `gen-docs` produces no diff for them. The acceptance criterion wrongly said the drift check would be "clean" only when the files match a fresh run — it needed to state the expected outcome is NO diff for output-only changes. | Task body and acceptance criterion updated: CLI Reference Drift check verifies no-diff when only output projection changed; committed files are updated only when actual Cobra metadata changes cause a diff. |
| Reversed U8b unit IDs (147.016-T) | The red-state explanation named `147.008-T / U3` for the validity gate (should be `147.006-T / U3`) and `147.008-T / U3b` / `147.007-T / U4` for conformance gates (should be `147.007-T / U3b` / `147.008-T / U4`). | All three reversed references corrected in the expected red section of 147.016-T. |
| I3 owner points to retired U5b (147.017-T) | The I3 totality scoping in 147.017-T was attributed to `U5b` which is retired. | `I3 scoping, pinned by U5b` changed to `I3 scoping, pinned by U5 / 147.009-T`. Four-class table header explicitly scoped to "with no administrative disposition". |
| Dangling cycle-10 memory reference | The checkpoint `memory_path` referenced `stage-pr377-remediation-cycle-10-memory.md` which did not exist. | Created `docs/memory/2026-08-24/stage-pr377-remediation-cycle-10-memory.md` and `stage-pr377-remediation-cycle-11-memory.md`. Checkpoint and memories.json updated. Compaction threshold: 11 files / ~313KB — below both triggers. |
| U9 matrix not total over administrative disposition (147.017-T) | `ResolveCheckpoint` checks disposition BEFORE the four-class classification. A valid conforming active document with `disposition: abandoned` returns `ErrCheckpointCannotResolveAbandoned` before any gate. The four-class table did not distinguish "no administrative disposition" from the general active population. | Four-class table header explicitly scoped to "active documents with no administrative disposition". Added a separate precedence section: `disposition: abandoned` → `ErrCheckpointCannotResolveAbandoned` fires before the classification table (regression guard, not a new feature). Acceptance criteria updated. |

Net effect: no unit added, no edge added, no shipment member added, no task ID renumbered. Backlog shape remains **26 tasks / 42 edges / 27 shipment members**. Prior-cycle decisions (checkpoint-safety design, shell contract, repair mapping, hard merge gate, 147.009-T paired-assertion halt condition, ownership splits, U5b retirement) are unchanged.

<!-- copilot-review-remediation: pr-377-cycle-11 -->

### PR #377 Copilot review remediation, cycle 12

A twelfth Copilot review (operator-authorized extension) flagged two findings on the same PR head (ee13e2e9):

| Thread | Issue | Cycle-12 correction |
|---|---|---|
| All-unmodeled duplicate repair row safety | The exact-duplicate raw key names were treated as safely round-trippable via `legacy_top_level`. `CheckpointContext.UnmarshalJSON` decodes context into `map[string]json.RawMessage`, so exact-duplicate keys collapse via Go last-wins map insertion. | Split the row into distinct-spelling (safe to move) and exact-same-spelling (not auto-repairable, requires operator choice with duplicate-preserving method or quarantine). Generalized exact-duplicate safety invariant: no-implicit-survivor rule applies regardless of modeled/unmodeled key status. Plan cycle-1 remediation row updated from four-row to five-row. Task acceptance criteria updated. |

Net effect: no task added, no edge added, no shipment member added. Backlog shape remains **26 tasks / 42 edges / 27 shipment members**. Prior-cycle decisions unchanged.

<!-- copilot-review-remediation: pr-377-cycle-12 -->

### PR #377 Copilot review remediation, cycle 13

A thirteenth Copilot review (operator-authorized extension) flagged two current-head synchronization findings:

| Thread | Issue | Cycle-13 correction |
|---|---|---|
| U8b byte-identity postcondition universality (plan ~line 830) | A stale paragraph still required every fixture file to remain byte-identical on disk, contradicting the corrected U8b contract. Accepted `abandon` necessarily mutates its fresh fixture. | Byte-identity postcondition paragraph rewritten: applies only to refused-mutation paths (rows 1 and 2); the `conforming-active` row's accepted `abandon` asserts the intended rewrite/archive outcome, not byte identity. |
| U8b `conforming-active` row expected-red classification (147.016-T) | The expected-red section claimed the `conforming-active` row was a pure regression guard with all assertions already green. `GetCheckpointResult` begins as a zero-value declaration stub, and current MCP/CLI `get` payloads lack `conforming`. The three `conforming: true` get assertions are RED until U6b/U6c/U8c land. Only accepted `abandon` is already-green. | Plan and 147.016-T corrected: `conforming-active` row's three `get` assertions (`conforming: true` on core/MCP/CLI) reclassified as RED against declaration stubs; only `abandon` acceptance remains as an already-green regression guard. |

Net effect: no task added, no edge added, no shipment member added. Backlog shape remains **26 tasks / 42 edges / 27 shipment members**. Prior-cycle decisions unchanged.

<!-- copilot-review-remediation: pr-377-cycle-13 -->

### PR #377 Copilot review remediation, cycle 14

A fourteenth Copilot review (operator-authorized extension) flagged four structural findings:

| Thread | Finding | Cycle-14 correction |
|---|---|---|
| PRRT_kwDORzozKM6b7YzW / 3849365620 on 147.018-T | Context-member duplicate safety: `CheckpointContext.UnmarshalJSON` decodes into `map[string]json.RawMessage` where exact or fold-equivalent duplicate context names silently lose data. The cycle-12 universal no-implicit-survivor invariant is not code-enforced for `context` members. | **U2g added** (147.028-T): context-member duplicate detection read boundary. Performs ordered duplicate detection for immediate `context` members BEFORE map collapse. Refuses exact-duplicate and case-fold-equivalent member names while preserving all unique extension keys. 3 scenarios, 2 files. Depends on U1 + U2b; blocks U3b + U4. |
| PRRT_kwDORzozKM6b7Yzk / 3849365643 on 147.013-T | U7 scenario width: 147.013-T defines four independent scenarios, violating `<4 scenarios`. | domainError scope (scenario 2: two missing-sentinel safety-net mappings) moved from U7 to **U7e** (147.029-T). U7 reduced to 3 scenarios, 2 files. |
| PRRT_kwDORzozKM6b7Yzv / 3849365657 on 147.025-T | U7d file/scenario width: 147.025-T owns 3 files and 4 scenarios, violating both `<3 files` and `<4 scenarios`. | `ErrCheckpointCannotResolveAbandoned` → `validation_failed` mapping (scenario 4, `errors.go` file) moved from U7d to **U7e** (147.029-T). U7d reduced to 3 scenarios, 2 files. |
| PRRT_kwDORzozKM6b7Yz9 / 3849365682 on plan | U7d wording: plan called `validation_failed` "pre-existing" for the abandoned-resolve case, but current behavior is `InternalError`; the mapping is a genuine red delta. | Plan U7d section corrected: wording now states the `ErrCheckpointCannotResolveAbandoned` → `validation_failed` mapping is a genuine red delta owned by U7e, not a pre-existing code path. |

**New tasks**:

| Task | Unit | Scenarios | Files | Dependencies | Shipment |
|---|---|---|---|---|---|
| 147.028-T | U2g | 3 | 2 (`checkpoint_schema.go`, `checkpoint_schema_test.go`) | U1, U2b (declared); blocks U3b, U4 (added to their deps) | 130-S |
| 147.029-T | U7e | 3 | 2 (`errors.go`, `checkpoint_disposition_test.go`) | U1 (declared) | 130-S |

**Modified tasks**:

| Task | Unit | Change | New scenarios | New files |
|---|---|---|---|---|
| 147.013-T | U7 | Removed domainError scope (item 3) and scenario 2 → U7e | 3 | 2 |
| 147.025-T | U7d | Removed scenario 4 (abandoned→validation_failed) and `errors.go` → U7e; corrected wording | 3 | 2 |
| 147.007-T | U3b | Added dependency on 147.028-T (U2g) | unchanged | unchanged |
| 147.008-T | U4 | Added dependency on 147.028-T (U2g) | unchanged | unchanged |

**New dependency edges** (5 queued-to-queued):

1. 147.028-T → 147.001-T (U2g depends on U1)
2. 147.028-T → 147.003-T (U2g depends on U2b)
3. 147.029-T → 147.001-T (U7e depends on U1)
4. 147.007-T → 147.028-T (U3b depends on U2g)
5. 147.008-T → 147.028-T (U4 depends on U2g)

Net effect: **2 tasks added, 5 edges added, 2 shipment members added**. Backlog shape: **28 queued tasks / 47 queued-to-queued edges / 29 shipment members**. Historical total edges: 48 (47 queued-to-queued + 1 archived 147.010-T → 147.009-T). Ready set: {147.001-T} (sole root, unchanged). U5b remains archived.

<!-- copilot-review-remediation: pr-377-cycle-14 -->

## Plan Review

cycle: 14

dispatch_mode: multi-agent-dispatch

decision: FAIL

**Superseded by the cycle-15 gate record immediately below, which is itself superseded by the
cycle-16 gate record at the end of this document.** This record remains the authoritative history
of the cycle-14 dispatch and its findings; the current gate state is `cycle: 16`, `decision: FAIL`.
It supersedes the earlier `## Plan Review` record in this document, which is scoped to cycles 1-13
and does **not** cover the cycle-14 dispatch. The prior PASS is retained as history and must not be
read as clearance for the plan in its cycle-14 state.

### Dispatch record

All seven selected personas were dispatched as independent sub-agents against the post-cycle-14
plan and all seven returned. Selection covers the plan's risk surface: a governed write path, a
Go error contract, agent-facing tool descriptions, backlog decomposition width, and workspace
security boundaries.

| Persona | Returned | Coverage assignment |
|---|---|---|
| Constitution Reviewer | yes | P-002 / P-004 test-first posture, task granularity, destructive-approval and worktree policy |
| Go Reviewer | yes | sentinel and typed-error contracts, `errors.Is` traversal, wrap verbs, `encoding/json` semantics |
| Scope Boundary Auditor | yes | YAGNI, unit reachability, scope creep against the origin decision |
| Learnings Researcher | yes | `docs/compound/` precedent for round-trip loss, `omitempty` array contracts, version skew |
| Architecture Strategist | yes | unit cohesion, read-boundary versus create-boundary separation, dependency direction |
| Agent-Native Parity Reviewer | yes | cross-surface agreement between CLI, MCP, and the `events` read layer |
| Security Lens Reviewer | yes | data-loss paths, diagnostic disclosure, filesystem containment, restore collisions |

### Gate rationale

The gate is **FAIL** because the merged finding set contains P0 and P1 entries. The decisive P0 is
a contract contradiction at the centre of the cycle-14 addition: the plan and its harvested task
described two mutually exclusive contracts for the same rule. A plan that hands an implementer two
incompatible contracts for one behaviour cannot be cleared, regardless of how many other units are
sound.

### Findings by severity

| # | Severity | Finding | Personas | Disposition |
|---|---|---|---|---|
| F1 | **P0** | U2g contract contradiction: the plan placed context-duplicate detection in `CheckpointContext.UnmarshalJSON` behind a new `ErrCheckpointContextDuplicateKey`, while `147.028-T` required raw ordered detection through the existing `ErrCheckpointNonConforming` contract. Parse-time placement creates cross-verb inconsistency, and `ParseCheckpoint`'s `%v` wrap drops the sentinel from `errors.Is`. | Constitution, Go, Scope, Agent-Parity, Security | **Fixed** in cycle 15 — one contract chosen: lenient parse, verdict in the caller-invoked conformance helper, existing typed error. |
| F2 | **P0** | Duplicate semantics permitted implicit last-wins and, as written in the task, would have refused distinct unmodeled names such as `foo` / `Foo`, narrowing the open `context` namespace U2b exists to protect. | Go, Security, Agent-Parity | **Fixed** — decided rule recorded: exact decoded duplicates refused universally; fold variants refused only when they alias a modeled context field; distinct unmodeled names stay conforming; `strings.EqualFold`, no normalization. |
| F3 | **P1** | Red/green honesty: `147.028-T` declared its unique-extension round-trip case as a RED assertion although it is already-shipped 146-F behaviour. | Constitution, Go | **Fixed** — reclassified as a declared regression guard; genuine red is now cases 1 and 2 only. |
| F4 | **P1** | Width and ownership drift: the plan's U7d section still carried 3 files and `Tests (4)` and still claimed the abandoned-resolve mapping; U6 carried 4 scenarios; U5 carried 5 after the cycle-10 absorption; U7 still said "four coupled defects". | Scope, Constitution, Architecture | **Fixed** — every active unit normalized to `<3 files` and `<4 independent scenarios`; see the cycle-15 appendix table. |
| F5 | **P1** | U7e reachability was asserted, not demonstrated; Scope called the unit YAGNI. | Scope, Go, Constitution | **Adjudicated and retained** — source inspection shows `domainError` has no case for any of the three sentinels and `handleResolveCheckpoint` still reaches it for `ErrCheckpointCannotResolveAbandoned`. Unit rewritten with a mandatory ordering constraint and wrapped-error tests. |
| F6 | **P1** | Normative plan staleness: the canonical dependency graph, edge table, requirements trace, and execution order omitted U2g and U7e entirely; corrections lived only in cycle appendices. | Architecture, Scope, Agent-Parity | **Fixed** — all four regenerated to include every active unit and edge. |
| F7 | **P1** | U9b and handoff inconsistency: U9b described the U2g rule with the withdrawn parse-time placement; archived `147.010-T` read as an actionable implementation instruction with no retirement notice; the U8 to U9b cross-reference pointed at text U9b does not contain. | Agent-Parity, Constitution | **Fixed** — U9b wording synchronized, retirement notice added and acceptance criteria voided, cross-reference repaired. |
| F8 | **P1** | Runtime and quality closure: no verification covered the new context-duplicate refusal or the retained U7e mapping; there was no single mandatory gate sequence; the binary-provenance precheck assumed a plain `go build` reports HEAD. | Constitution, Go, Learnings | **Fixed** — cycle-15 verification row added, eight-gate sequence added in constitutional order, ldflags provenance corrected against `.github/workflows/release.yml`. |
| F9 | **P1** | Strict safety: file-overwriting and file-moving disposition commands were not classified destructive; live post-merge observation implied mutating session-start recovery against the live corpus; the recovery procedure did not bind commands to a canonical workspace root. | Security, Constitution | **Fixed** — A4c and A4d added, live observation constrained to read-only or mirror, recovery-procedure contract added (`--cwd` binding, filename/hash/state/destination display, no-clobber evidence handling). |
| F10 | **P2** | Additional security-lens items raised beyond the bounded scope: create-boundary duplicate handling, symlink / no-follow containment, read-to-write CAS races, rollback safety, diagnostic key disclosure, archive restore collisions. | Security | **Split.** Diagnostic key disclosure and archive-restore no-clobber are absorbed (they are directly required for safe disposition). The remaining four are recorded as named follow-ups with rationale; see the cycle-15 appendix. |

### Rejected findings

| Claim | Persona | Why rejected |
|---|---|---|
| U2g and U7e are absent from the plan | Architecture Strategist | Stale. Both sections exist and are titled `### U2g — Context-member duplicate detection read boundary` and `### U7e — domainError safety-net mappings`. Removing them on this basis would have deleted the cycle-14 remediation itself. No regression applied. |
| A normal linked git worktree is a P-016 or containment violation | Constitution Reviewer | Not valid. P-016 requires **one** dedicated implementation branch in **one** active worktree; it does not forbid a linked worktree as the mechanism for that single branch. This work runs entirely inside one worktree on the single branch `chore/stage-130-s`, all writes resolve under that worktree root, and no second implementation branch exists. The topology is preserved deliberately. |
| The U9b same-merge constraint requires a separate merge per backlog task | Constitution Reviewer | Misreading. Ship builds one release-unit branch and one PR for the shipment, so "same merge commit" binds the **implementation units to each other inside that single PR** — U9b's instruction-file delta must not ship in a later PR than the gate units whose failure mode it documents. It is not a per-task merge requirement, and it does not imply multiple merges. Wording clarified rather than changed in substance. |

### Remediation queue

All P0 and P1 entries above are closed in cycle 15 within this bounded scope. No P0 or P1 finding
remains open. The P2 split items are recorded as follow-ups and are explicitly **not** blocking:
each is a pre-existing condition of the checkpoint subsystem rather than a defect this plan
introduces, and absorbing them would convert a bounded disposition change into an unbounded
filesystem-hardening project.

<!-- copilot-review-remediation: pr-377-cycle-14-gate -->

### PR #377 plan remediation, cycle 15

Cycle 15 is the remediation pass for the cycle-14 `FAIL` gate above. It is planning-only: no Go
source, test, or configuration file was modified.

**Central adjudication — one contract for U2g.** `CheckpointContext.UnmarshalJSON` and
`ParseCheckpoint` stay lenient. The context-member duplicate verdict moves into the caller-invoked
read-boundary conformance helper, which already receives the original bytes before any rewrite and
already returns `*CheckpointNonConformingError`. `ErrCheckpointContextDuplicateKey` is withdrawn.
Offenders report as `duplicate:context.<key>`, matching `duplicate:<key>` (U2c) and
`duplicate:progress.<key>` (U2e). Every gate unit and read surface inherits the verdict from the
predicate it already calls, so `resolve`, `abandon`, `quarantine`, `list`, and `get` cannot
disagree about one file.

**Decided duplicate semantics.**

| Case | Verdict | Why |
|---|---|---|
| Exact duplicate decoded member names, including escape-equivalent spellings | **Refuse**, universally | `map[string]json.RawMessage` collapses them last-wins before any caller code runs |
| Fold variant that aliases a modeled context field (`shipment_id`, `feature_id`, `task_ids`, `branch`) | **Refuse** | the shadow decode picks one winner and `isModeledContextKey` filters **both** out of `Extra`, destroying the loser |
| Fold-distinct unmodeled names (`foo` / `Foo`), NFC/NFD-distinct extensions | **Conforming** | distinct `Extra` map keys, lossless round-trip; refusing them would narrow the open namespace U2b protects |
| Comparison algorithm | Go `strings.EqualFold`, no normalization | the same relation `encoding/json` uses for field matching and `isFoldKeyIn` already implements |

**Width normalization — every active unit at `<3 files` and `<4 independent scenarios`.**

| Unit | Task | Before | After | Method |
|---|---|---|---|---|
| U2g | 147.028-T | 2 files, 3 scenarios, wrong contract | 2 files, 3 scenarios, correct contract | file scope repointed to the conformance helper |
| U7e | 147.029-T | 2 files, 3 scenarios, reachability unproven | 2 files, 3 scenarios, reachability proven | retained; ordering constraint and wrapped-error tests added |
| U7 | 147.013-T | "four coupled defects" narrative | three owned deltas plus one correction of record | narrative corrected; scenarios already 3 |
| U7d | 147.025-T | plan said 3 files, `Tests (4)` | 2 files, 3 scenarios | plan section synchronized with the task, which was already correct |
| U6 | 147.011-T | 4 scenarios | 3 scenarios | byte-identity recomposed as a postcondition of scenarios 1-3 |
| U5 | 147.009-T | 5 effective scenarios | 3 scenarios | the two absorbed state-conflict guards became additional rows of scenario 2 |

**U7e retain/archive decision: retained.** `internal/mcp/errors.go:148-200` carries no case for
`ErrCheckpointUseQuarantine`, `ErrCheckpointNonConforming`, or
`ErrCheckpointCannotResolveAbandoned`; all reach `default: InternalError`. After U7d,
`handleResolveCheckpoint` reroutes only `QuarantineIsRemedy` matches, which the abandoned-resolve
sentinel does not satisfy, so `internal/mcp/tools.go:1224` still surfaces it as a 500. The unit
gains a mandatory placement rule — the three rows must precede the combined
`ErrValidation` / `ErrCheckpointInvalid` / `ErrCheckpointCorrupt` case, because U3's multi-`%w`
refusal satisfies both matchers and an appended row would be permanently shadowed — and its tests
now assert against realistically wrapped errors rather than bare sentinels.

**Security-lens scope split.**

| Item | Disposition | Rationale |
|---|---|---|
| Diagnostic key disclosure | **Absorbed** into U1 | `Fields` is untrusted text from an already-malformed file rendered into an operator message, an MCP payload, and a CLI line; quoting and bounding are required for safe refusal |
| Archive restore collisions | **Absorbed** into the recovery-procedure contract | no-clobber on copy-back, archive writes, and sidecar writes protects the only verbatim record of pre-quarantine bytes |
| Create-boundary `context` duplicate handling | **Follow-up — stash `E429A031`** | the create boundary is 146-F's shipped scope and writes a new file from caller-supplied bytes; no pre-existing on-disk evidence is at risk, so it is not required for safe disposition |
| Symlink / no-follow containment on the checkpoint directory | **Follow-up — stash `35A27CD0`** | a pre-existing property of every checkpoint path, unchanged by this work; closing it is a filesystem-hardening unit of its own |
| Read-to-write CAS races between the conformance read and the rewrite | **Follow-up — stash `35A27CD0`** | a pre-existing TOCTOU window in the shipped `parse -> mutate -> re-marshal` shape; this plan narrows what may be written but does not introduce the window |
| Rollback safety beyond revert-the-merge | **Follow-up** | no data migration, no schema change, and no on-disk format change, so revert remains complete for this scope |

None of these is dismissed. Each is recorded here and in **Follow-ups recorded (not in scope)** so
a future reviewer can see the path was identified and deliberately deferred rather than missed.

**Topology after cycle 15.** One executable edge added: `147.028-T -> 147.004-T` (U2g reuses U2c's
`duplicate:` reporting form, mirroring the existing U2c -> U2e edge). No task added, none archived,
no shipment membership change.

| Measure | Cycle 14 | Cycle 15 |
|---|---|---|
| Queued tasks under `147-F` | 28 | 28 |
| Queued-to-queued executable edges | 47 | **48** |
| Shipment `130-S` members | 29 | 29 |
| Ready set | `{147.001-T}` | `{147.001-T}` |
| Historical total edges | 48 | **49** (48 executable + 1 archived `147.010-T -> 147.009-T`) |

<!-- copilot-review-remediation: pr-377-cycle-15 -->

## Plan Review

cycle: 15

dispatch_mode: multi-agent-dispatch

decision: FAIL

**This record is superseded by the cycle-16 gate record at the end of this document.** It
supersedes both earlier `## Plan Review` records: the cycles 1-13 `PASS` and the `cycle: 14`
`FAIL`. Neither may be read as clearance for the plan in its cycle-15 state. The cycle-16
remediation appendix beneath this record documents what changed in response; it did **not** clear
the gate. The required fresh, independent cycle-16 review is recorded at the end of this document
and also returned `FAIL`.

### Dispatch record

All seven selected personas were dispatched as independent sub-agents against the post-cycle-15
plan and all seven returned. Selection is unchanged from cycle 14 and covers the plan's risk
surface: a governed write path, a Go error contract, agent-facing tool descriptions, backlog
decomposition width, operator-facing runbook safety, and workspace security boundaries.

| Persona | Returned | Coverage assignment |
|---|---|---|
| Constitution Reviewer | yes | P-002 / P-004 test-first posture and red honesty, task granularity, destructive-approval and worktree policy |
| Go Reviewer | yes | sentinel and typed-error contracts, `errors.Is` traversal and case ordering, wrap verbs, `encoding/json` semantics |
| Scope Boundary Auditor | yes | YAGNI, unit reachability, effective scenario width, scope creep against the origin decision |
| Learnings Researcher | yes | `docs/compound/` precedent for round-trip loss, `omitempty` array contracts, version skew and fresh-binary provenance |
| Architecture Strategist | yes | unit cohesion, read-boundary versus create-boundary separation, dependency direction, named invariants |
| Agent-Native Parity Reviewer | yes | cross-surface agreement between CLI, MCP, and the `events` read layer; offender-source reachability |
| Security Lens Reviewer | yes | data-loss paths, diagnostic disclosure and bounding, filesystem containment, restore collisions, runbook safety |

### Cycle-14 P0 closure

**The cycle-14 P0 is CLOSED and is not re-raised.** U2g now carries **one** contract: `ParseCheckpoint`
and `CheckpointContext.UnmarshalJSON` stay lenient, the context-member duplicate verdict lives in the
caller-invoked read-boundary conformance helper, and offenders report through the existing
`ErrCheckpointNonConforming` / `*CheckpointNonConformingError` pair as `duplicate:context.<key>`.
`ErrCheckpointContextDuplicateKey` is withdrawn. All five personas that raised the cycle-14 F1
contract contradiction confirmed it resolved, and the plan and `147.028-T` no longer disagree. The
open `context` namespace is preserved: distinct unmodeled fold variants and NFC/NFD-distinct
extension keys stay conforming, which was the cycle-14 F2 half of the same P0.

### Gate rationale

The gate is **FAIL** because the merged cycle-15 finding set still contains P0 and P1 entries, all
of them **new or residual** rather than re-raised cycle-14 items. The decisive P0 is that the plan
publishes, as workspace-wide agent instructions, a paste-runnable filesystem repair and restore
runbook whose safety it cannot guarantee — no real-root / no-follow open, no read-to-write
compare-and-swap, no adversarial coverage — while simultaneously asserting a universal
no-implicit-survivor invariant that the create boundary does not honour. A plan that ships operator
instructions it cannot make safe, and a guarantee the code does not provide, cannot be cleared
regardless of how sound the refusal gates are.

### Findings by severity

| # | Severity | Finding | Personas | Disposition in cycle 16 |
|---|---|---|---|---|
| G1 | **P0** | Unsafe published runbook: U9b and `147.018-T` publish a two-entry-point hand-repair and post-quarantine restore procedure — rename archived evidence aside, copy bytes back, hand-edit members in place, conditionally delete on abort — in a file with `applyTo: '**'`. The sequence runs against a directory opened without real-root / no-follow semantics, with no handle-or-content CAS between the classifying read and the repairing write, and with no adversarial tests for a symlinked or concurrently-replaced path. | Security, Constitution, Architecture | **Fixed by withdrawal.** U9b is rewritten quarantine-only; the executable runbook is deferred to stash `35A27CD0` with its required properties named. U10b's restore row is replaced by a quarantine evidence-integrity row. |
| G2 | **P0** | False universal invariant: U9b asserts the no-implicit-survivor rule "universally … whether at the top level, inside `context`, inside `progress`, or inside `legacy_top_level` itself", but `CreateCheckpoint` still parses and re-marshals caller bytes and can collapse duplicate `context` members before disk. The claim is not true of the create boundary. | Security, Go, Agent-Parity | **Fixed by narrowing.** The invariant is scoped to the stored-checkpoint administrative-disposition read and rewrite surfaces (`resolve`, `abandon`, `quarantine`, `list`, `get`), and create-boundary hardening is stated as deferred under stash `E429A031`. A bounded create-boundary unit was considered and rejected as scope creep against the origin decision. |
| G3 | **P1** | Mixed modeled/unmodeled auto-survivor rule: U9b's repair table told the operator to move the unmodeled variant and leave the modeled member in place for `duplicate:<key>` with one modeled member — a spelling-based implicit survivor selection on a modeled field. | Security, Go, Constitution | **Fixed.** Removed. Every duplicate equivalence class touching a modeled field now requires an explicit, recorded, raw-token-aware operator selection or quarantine. |
| G4 | **P1** | Effective scenario width: U5 still had four effective flows (non-conforming active, conforming active, conforming resolved, byte-identity) and U2g still read as four (two refusals plus two separate preservation assertions), despite both being presented as three. | Scope, Constitution, Architecture | **Fixed.** U5 → 2 scenarios (byte-identity recomposed as a postcondition of the paired row; the `resolved` state-conflict guards withdrawn to stash `6FA45E69`). U2g → 2 red plus 1 combined green preservation guard. |
| G5 | **P1** | Red-honesty and unverifiable red: the plan's red posture said only `go test ./<pkg>` — no package, no selector, no `-count=1` — and U7e's expected red claimed `default: InternalError` for its multi-`%w` quarantine case, which in fact shadows through the generic `validation_failed` case at `internal/mcp/errors.go:188-193`. | Constitution, Go, Learnings | **Fixed.** A per-unit red-verification table with exact `-run` regexes and `-count=1` was added, plus a mandatory `TestU<unit>_` harness naming contract. U7e's expected red is corrected; the shadowing case is removed with its row. |
| G6 | **P1** | U7e reachability overstated: rows 1 and 2 (`ErrCheckpointUseQuarantine`, `ErrCheckpointNonConforming` → `domainError`) have no live filename-less path. The only checkpoint handlers reaching `domainError` are `handleGetCheckpoint` (`tools.go:1205`) and `handleResolveCheckpoint` (`:1224`); get never emits either sentinel and U7d reroutes every `QuarantineIsRemedy` match. | Scope, Go, Agent-Parity | **Fixed.** Rows 1 and 2 removed; only the reachable `ErrCheckpointCannotResolveAbandoned` → `validation_failed` row is retained, with a source-cited reachability table. |
| G7 | **P1** | Two destructive-approval contracts: A4 said runtime disposition mutation needs no approval while A4c said the same commands are destructive and require approval immediately before each batch. Principle VII was additionally recorded as a "documented deviation", which a NON-NEGOTIABLE principle cannot be. | Security, Constitution | **Fixed.** A4 narrowed to workspace creation and read verbs only; A4c named the sole operative contract for any checkpoint-file-moving or -overwriting command; Principle VII moved to conditional **pass** with the approval condition stated, and both VII deviation rows withdrawn. |
| G8 | **P1** | Unexecutable offender source: U9b instructs the operator to read offenders from `checkpoint get`, but `CheckpointReadResult` and both `get` projections carry no offender field. The instruction pointed at data no unit produced. | Agent-Parity, Architecture | **Fixed.** `NonConformingFields` added to `CheckpointReadResult` (U6b) and projected as `non_conforming_fields` by U6c and U8c — a width-neutral addition to carriers those units already declare. |
| G9 | **P1** | Unbounded diagnostic drift: cycle 15 bounded `Error()` only. U7's `unknown_fields` populates via `errors.As` from the raw `Fields` slice, so the MCP payload could be unbounded while the CLI was bounded. | Security, Agent-Parity | **Fixed.** New unit **U1b** (`147.030-T`) owns the single bounded projection `BoundedFieldPaths()`; U6b, U7, and U8 all render through it and are forbidden from re-deriving a list. |
| G10 | **P1** | Provenance and gate defects: the U10 precheck asserted `version --json`, which is not a flag this CLI has (`--format json` is), omitted `--no-update-check`, and cited the release workflow rather than the repository's own `LDFLAGS`. The final gate sequence used a bare `gofmt -l .`, which exits 0 while listing unformatted files, and implied docs-only units may skip the Go gates. | Learnings, Constitution, Go | **Fixed.** Provenance corrected against `Makefile:5-8` and `internal/cli/version_cmd.go`; Gate 4 is now `make fmt`; a build-and-provenance Gate 9 added; gates declared mandatory for the release-unit branch regardless of any individual unit being docs-only. |
| G11 | **P1** | Normative desynchronization: the dependency graph, edge table, execution order, and several `Depends on` lines disagreed with the backlog. U3 said "U2" where the edge is U2c; U6b said "U2c" where the edge is U6; U3b and U4 omitted the cycle-14 U2f and U2g prerequisites; U5 claimed a direct U2c edge the graph does not carry. | Architecture, Scope, Agent-Parity | **Fixed.** All normative sections regenerated from the `item_deps` rows; prose corrected only where the edge is real. No redundant edge was added merely to match prose. |
| G12 | **P2** | Architecture advisory: named-invariant coverage for U7e's ordering constraint; a note on dual `GetCheckpoint` / `GetCheckpointResult` API surface; a suggestion to add a CI gate for the conformance predicate. | Architecture | **Ordering invariant closed by removal** (the multi-`%w` row it guarded is gone). The dual-API and CI-gate items are advisory and deliberately not expanded this cycle. |
| G13 | **P3** | Presentational: cycle appendices have accumulated to the point where a reader must scan several of them to reconstruct current state. | Scope, Learnings | **Acknowledged, not fixed.** Consolidation is a compaction concern; the normative sections are now authoritative and each appendix is explicitly historical. |

### Rejected findings

| Claim | Persona | Why rejected |
|---|---|---|
| Add a bounded create-boundary duplicate-detection unit to this shipment | Security Lens Reviewer | Rejected as scope creep. The create boundary is 146-F's shipped surface, it writes a new file from caller-supplied bytes, and no pre-existing on-disk evidence is at risk there — so it is not required for safe disposition, which is this plan's origin requirement. The alternative — narrowing every invariant and claim to the stored-checkpoint disposition surfaces and naming the deferral — is smaller, honest, and was applied instead. |
| Keep the U9b restore runbook but add real-root / no-follow, CAS, and adversarial tests to this shipment | Security Lens Reviewer | Rejected as scope expansion. That is a filesystem-hardening project (stash `35A27CD0`), not a disposition change. Withholding the runbook costs nothing this plan needs: refusal plus verbatim-move is already a complete, reversible disposition. |
| The dedicated linked worktree is a P-016 or containment violation | Constitution Reviewer (re-raised from cycle 14) | Rejected again, unchanged. P-016 requires **one** dedicated implementation branch in **one** active worktree; a linked worktree is the mechanism for that, not a second branch. All work runs inside one worktree on `chore/stage-130-s`, every write resolves under that worktree root, and the root checkout is untouched. |

### Remediation queue

Every P0 and P1 entry above is addressed in the cycle-16 remediation appendix below, within the
bounded planning scope. G12 and G13 are advisory and are deliberately not expanded. The gate stays
**FAIL** until an independent cycle-16 review dispatches against the remediated plan and returns.

<!-- copilot-review-remediation: pr-377-cycle-15-gate -->

### PR #377 plan remediation, cycle 16

Cycle 16 is the remediation pass for the cycle-15 `FAIL` gate above. It is planning-only: no Go
source, test, or configuration file was modified.

**Central adjudication — this shipment is quarantine-only for malformed and non-conforming live
evidence.** `resolve` and `abandon` refuse; `quarantine` accepts and preserves the bytes verbatim
with a sidecar. The plan no longer publishes, automates, or verifies an in-place repair or a
post-quarantine restore. An operator may still make an explicit raw-token-aware survivor decision
by hand, outside agent automation; the plan simply stops claiming an automated path it cannot make
safe. The withdrawn runbook and its required safety properties are recorded under stash
`35A27CD0`.

**Width and red-honesty normalization.**

| Unit | Task | Before (cycle 15) | After (cycle 16) |
|---|---|---|---|
| U1 | `147.001-T` | 3 scenarios plus an in-line bounding contract | 3 scenarios; bounding split out to U1b |
| U1b | `147.030-T` | — | **new**: 2 files, 3 scenarios, single bounded projection |
| U2g | `147.028-T` | 3 declared / 4 effective | 2 red plus 1 combined green preservation guard |
| U5 | `147.009-T` | 3 declared / 4 effective | 2 scenarios; byte-identity is a postcondition; `resolved` guards withdrawn |
| U7e | `147.029-T` | 3 scenarios, 2 unreachable | 1 scenario, reachability source-cited |
| U6b | `147.012-T` | 3 scenarios | 3 scenarios plus `NonConformingFields` on the carrier it already declares |
| U6c, U8c | `147.022-T`, `147.027-T` | 3 scenarios | 3 scenarios plus `non_conforming_fields` on the projection they already implement |
| U9b | `147.018-T` | repair-and-restore runbook | quarantine-first guidance plus a diagnostic classification table |
| U10b | `147.026-T` | acceptance, restore, sweep | acceptance, quarantine evidence integrity, sweep |

**New follow-up stash entries.**

| Stash | Item |
|---|---|
| `6FA45E69` | Pin the conforming + `resolved` double-refusal state-conflict class as a tested invariant in a unit that owns it |
| `DBBA62AA` | CLI coverage for `backlogit checkpoint resolve` on an already-abandoned document |
| `35A27CD0` (extended) | Absorbs the withdrawn body-preserving repair and post-quarantine restore runbook alongside the existing symlink / no-follow and CAS work |

**Topology after cycle 16.** One task added (`147.030-T`, U1b) and four executable edges added:
`147.030-T -> 147.001-T`, `147.012-T -> 147.030-T`, `147.013-T -> 147.030-T`, and
`147.015-T -> 147.030-T`. No task archived.

| Measure | Cycle 15 | Cycle 16 |
|---|---|---|
| Queued tasks under `147-F` | 28 | **29** |
| Queued-to-queued executable edges | 48 | **52** |
| Shipment `130-S` members | 29 | **30** |
| Ready set | `{147.001-T}` | `{147.001-T}` |
| Historical total edges | 49 | **53** (52 executable + 1 archived `147.010-T -> 147.009-T`) |

<!-- copilot-review-remediation: pr-377-cycle-16 -->

## Plan Review

cycle: 16

dispatch_mode: single-agent-declared-degradation

TOOL_DEGRADED: reviewer-subagent-dispatch

decision: FAIL

severity counts: P0=1, P1=7, P2=3, P3=2

**This record is the current gate state.** It supersedes the cycles 1-13 `PASS`, the `cycle: 14`
`FAIL`, and the `cycle: 15` `FAIL`. None of the three earlier records may be read as clearance for
the plan in its cycle-16 state.

### Dispatch record — degraded to a single-agent sequential pass

The cycle-16 review is the fresh, independent dispatch the cycle-15 gate required, run against the
plan as it stood after the "PR #377 plan remediation, cycle 16" appendix above. The first attempt
used `multi-agent-dispatch`, matching cycles 14 and 15. That attempt is **invalid**: the Learnings
Researcher sub-agent returned findings without inspecting the full plan text, so its coverage
cannot be trusted. Per the plan-review skill's terminal-states rule, a dispatch that fails
mid-gate for any selected persona cannot be partially merged into a full-fidelity decision. The
multi-agent attempt is discarded in its entirety rather than salvaged, and the gate re-ran as a
complete sequential, single-agent pass applying every persona's adapter (identity file plus
plan-focused Focus criteria) over the full plan text, one lens at a time.
`TOOL_DEGRADED: reviewer-subagent-dispatch` records the degradation. All seven selected personas
completed under the fallback; coverage is complete, with the Learnings Researcher's pass re-run in
full rather than reused from the invalid attempt.

| Persona | Coverage mode | Coverage assignment |
|---|---|---|
| Constitution | sequential (single-agent) | P-002 / P-004 test-first posture and red honesty, task granularity, destructive-approval and worktree policy |
| Go | sequential (single-agent) | sentinel and typed-error contracts, `errors.Is` traversal and case ordering, wrap verbs, `encoding/json` semantics |
| Scope | sequential (single-agent) | YAGNI, unit reachability, effective scenario width, scope creep against the origin decision |
| Learnings | sequential (single-agent), re-run after the invalid attempt | `docs/compound/` precedent for round-trip loss, `omitempty` array contracts, version skew and fresh-binary provenance |
| Architecture | sequential (single-agent) | unit cohesion, read-boundary versus create-boundary separation, dependency direction, named invariants |
| Agent-Native Parity | sequential (single-agent) | cross-surface agreement between CLI, MCP, and the `events` read layer; offender-source reachability |
| Security | sequential (single-agent) | data-loss paths, diagnostic disclosure and bounding, filesystem containment, restore collisions, runbook safety |

### Gate rationale

The gate is **FAIL**. The merged finding set carries 1 P0 and 7 P1 blocking entries (severity
counts: P0=1, P1=7, P2=3, P3=2). The decisive P0 is that the remediated plan still describes, in U6
and U9b, an ambient-cwd runnable command that is not bound to the A4c cwd / approval / preimage /
no-clobber contract — the same class of risk the cycle-15 gate closed by withdrawing the repair and
restore runbook, recurring in different unit content. A plan that reintroduces an unsafe executable
path cannot be cleared, regardless of how many other findings are resolved.

### Findings by severity

**P0 — 1**

| ID | Finding | Disposition |
|---|---|---|
| H1 | Unsafe executable remediation: U6 and U9b advertised an ambient-cwd runnable command without binding it to the A4c cwd / approval / preimage / no-clobber contract. | Blocking. Requires a non-executable remediation intent, or rendering safely bound at the CLI/MCP boundaries. |

**P1 — 7**

| ID | Finding | Disposition |
|---|---|---|
| H2 | U8b cannot satisfy the RED ordering / current-source premise as written. | Blocking. Restage the unit: declarations before harness before implementation. |
| H3 | Runtime coverage is orphaned: the context-duplicate and abandoned-resolve behaviors have no owned, bounded runtime verification unit. | Blocking. Add an owned bounded runtime unit (recommended `U10c`). |
| H4 | Machine-readable arrays must carry bounded raw field paths with structured truncation metadata; quoting belongs only in the human-facing presentation. | Blocking. Separate the machine and human representations at the boundary that currently conflates them. |
| H5 | U7 and U7e carry stale normative ownership claims. | Blocking. Only the abandoned-resolve mapping is retained; the remaining stale ownership must be corrected. |
| H6 | U7b and U7c exact descriptions do not use the registered `backlogit_*` tool names. | Blocking. Correct the descriptions to the registered tool names. |
| H7 | Principle I `%v` wrap in the touched `AbandonCheckpoint` path cannot be waived. | Blocking. Add a focused multi-`%w` harness and implementation unit. |
| H8 | U2f AST sink enumeration cannot fully enforce I1 as structured. | Blocking. Formal decomposition should centralize the rewrites behind a guarded seam. |

**P2 — 3 (advisory, not itemized in this persistence record)**

**P3 — 2 (advisory, not itemized in this persistence record)**

The P2 and P3 findings are non-blocking and are counted in the severity summary above only. This
entry persists the gate verdict the sequential-fallback dispatch returned; it does not reproduce
every advisory observation, none of which changes the FAIL outcome.

### Rejected / stale findings

| Claim | Disposition |
|---|---|
| A repair/restore runbook is still live in the plan | Rejected — already withdrawn in the cycle-16 remediation appendix above (U9b rewritten quarantine-only). |
| A universal create-boundary claim still stands | Rejected — already narrowed in the cycle-16 remediation appendix (create-boundary hardening deferred to stash `E429A031`). |
| The linked worktree is a P-016 or containment violation | Rejected, unchanged from cycles 14 and 15 — the linked worktree remains a valid mechanism for the single dedicated implementation branch `chore/stage-130-s`. |
| A bounded create-boundary duplicate-detection unit belongs in this shipment | Rejected, unchanged from cycle 15 — the create boundary remains tracked as follow-up stash `E429A031`. |
| U7e mappings for the two unreachable sentinel rows should be re-added | Rejected — cycle 15 already removed the unreachable `ErrCheckpointUseQuarantine` / `ErrCheckpointNonConforming` rows from `domainError`; do not reintroduce them. |

### Remediation queue and restage recommendation

restage_recommendation: formal-decomposition

The gate stays **FAIL**. Given the P0 and the breadth of the P1 set, unit-by-unit patching is
rejected in favor of a formal decomposition of the remaining work into five DAG partitions, to be
planned and re-gated before any implementation begins:

1. Foundation diagnostics and conformance
2. Guarded rewrite seam
3. Declarations and genuine RED harness order
4. Implementation plus MCP/CLI/instruction contracts
5. Runtime `U10` / `U10b` / `U10c` and closure

No blocker fix and no restage decomposition is attempted in this persistence pass. The plan,
backlog, checkpoint, and memory are updated only to record this gate outcome. Do not push this
branch and do not hand this shipment to Ship until the formal decomposition is planned and passes
its own plan-review gate.

**Cycle-17 status of this record.** The formal decomposition this gate required was executed in
cycle 17 and is documented in "PR #377 plan remediation, cycle 17 — formal decomposition" below.
That appendix is remediation evidence, **not** a gate outcome: this `cycle: 16` record remains the
current gate state at `decision: FAIL` until a fresh, independent plan review is dispatched against
the decomposed plan. All eight blockers (1 P0, 7 P1) are dispositioned as closed in the appendix,
and the decomposition is ready for that review.

### PR #377 plan remediation, cycle 17 — formal decomposition

This appendix is **historical evidence of the cycle-17 remediation pass**. It does not override the
normative sections above; where this appendix and a normative section disagree, the normative
section governs. The `cycle: 16` `## Plan Review` record remains the **current gate state**: it is
`decision: FAIL`, and nothing in this appendix clears it. Cycle 17 executed the
`restage_recommendation: formal-decomposition` that gate required; the result must now pass a fresh,
independent plan-review gate before any implementation begins.

**Method.** Unit-by-unit patching was rejected by the cycle-16 gate. The remaining work was
re-partitioned into the five DAG partitions the gate named, each independently reviewable, and the
plan's normative sections — Requirements Trace, Implementation Units, Dependency Graph, Decisions,
Risks, Constitution Check, Runtime Verification and Closure, gate sequence, and the I1/I2 invariants
— were rewritten against that structure rather than annotated.

#### Blocker disposition

| Gate ID | Severity | Cycle-17 disposition |
|---|---|---|
| H1 | P0 | **Closed.** `internal/events` no longer emits any command string for the new conformance branch. **U1d** declares a structured, non-executable `RemediationIntent` (`verb`, `target_filename`, `requires_approval`, `approval_class`, `reason`); U6, U6b, and U6c publish it; **U16** is the sole surface that renders an operator command, always with an explicit `--cwd` bound to the resolved storage root, a bare filename, and the A4c approval / preimage / no-clobber preamble, and refuses to render at all when quoting would be needed. U8 no longer asserts a runnable command; U9b gains a normative rule (item 9) forbidding executable remediation, repair, restore, or sweep text and an acceptance grep. The shipped `RemediationCommand` field is deprecated in place, not silently redefined; removal is stash `F350503F`. |
| H2 | P1 | **Closed.** Partition 3 exists for this: **U15** declares `CheckpointReadResult` / `GetCheckpointResult` ahead of every behavioural unit, and **U8b** is rewritten as a partition-3 harness whose prerequisites are declarations only (U1, U1b, U1d, U2, U15). Seventeen partition-4 units now depend on U8b instead of the reverse. The already-green schema-invalid `get` assertions were reclassified as declared regression guards and removed from U8b's red gate; the "batch harness generation phase" framing is withdrawn. |
| H3 | P1 | **Closed.** **U10c** (`147.041-T`) owns the context-duplicate cross-surface runtime verification and the abandoned-resolve MCP handler verification, wired after U10b, U2h, U6c, U7d, U7e, and U8c and before closure. U10 and U10b stay at three rows each. All three runtime units now persist deterministic, human-readable evidence to the tracked `docs/closure/2026-08-checkpoint-disposition-runtime-verification.md`; the git-ignored scratch directory is a working area, not the artifact of record. Teardown moved from U10b to U10c so it cannot destroy U10c's inputs. |
| H4 | P1 | **Closed.** **U1b** returns `BoundedFieldPathSet{Paths, Truncated, OmittedPaths, TruncatedPaths}` carrying **raw** paths — no `strconv.Quote`, no `"+N more"` pseudo-element — with UTF-8-safe cap semantics (16 paths, 128 bytes per path cut on a rune boundary, empty string plus a count when the first rune exceeds the cap). **U1c** owns the only quoted rendering and backs `Error()` and the CLI text. U7's `unknown_fields` gains three sibling truncation scalars, none with `omitempty`. U6b, U6c, U8, and U8c were synchronized to the split. |
| H5 | P1 | **Closed.** U7 item 3 no longer says the two unreachable `domainError` rows were "moved to U7e" — they were **deleted**, and the text says so. U7e is retitled to name the one row it owns and carries an explicit sole-ownership clause forbidding reintroduction. U7d's split note was corrected the same way. |
| H6 | P1 | **Closed.** U7b and U7c tables now key on the registered identifiers `backlogit_list_checkpoints`, `backlogit_get_checkpoint`, `backlogit_resolve_checkpoint`, `backlogit_abandon_checkpoint`, and `backlogit_quarantine_checkpoint`, verified against the `mcplib.NewTool` literals at `internal/mcp/tools.go:177`, `:188`, `:195`, `:209`, and `:218`. The description bodies were corrected too — a description that told an agent to call a bare `quarantine_checkpoint` named a tool that is not registered. Both units' tests are keyed by registered name and read from the built tool set. |
| H7 | P1 | **Closed.** **U17** (`147.040-T`) changes `AbandonCheckpoint`'s validation wrap from `%v` to multi-`%w`, asserts both `errors.Is(err, ErrCheckpointUseQuarantine)` and `errors.Is(err, ErrCheckpointInvalid)`, and pins the rendered message text. Width: 2 files, 2 scenarios. The Constitution Check row for Principle I moves from `deviation (documented)` to `pass`, and the deviation row is withdrawn from the deviations table. |
| H8 | P1 | **Closed.** The AST enumeration is no longer the authoritative I1 mechanism. **U11** declares the guarded seam `events.RewriteCheckpointFile`, **U12** lands its contract harness, **U13** implements parse → validate → conformance → mutate → marshal → atomic replace, and **U14** migrates `ResolveCheckpoint` and `AbandonCheckpoint` onto it. Quarantine's verbatim `moveNoReplace`, `CleanupCheckpoints`' rename, and `CreateCheckpoint`'s new-file writes stay explicitly outside the seam. **U2f** is retitled and rewritten as a supplemental caller-set regression guard with an honest bound; nothing depends on it, and a blocked U2f no longer blocks the release unit. |

#### Additional corrections made in this pass

| Area | Correction |
|---|---|
| U2g context scan | The refusal rule gains item 5 scoping the scan to the canonical `context` spelling, and **U2h** (`147.033-T`) closes the false negative: `encoding/json` routes every top-level key satisfying `strings.EqualFold(name, "context")` to the modeled field, so a lone `Context` / `CONTEXT` / `conTexT` member is subject to the same collapse loss. No universal create-boundary claim is made — the create boundary stays deferred under stash `E429A031`. |
| U2g red timing | Green guards are separated from the red gate by selector: the red gate is `-run '^TestU2g_Duplicate'` (two failing cases); the open-namespace preservation guard runs under the full `-run '^TestU2g_'` selector and is explicitly excluded from the red count. U2h uses the same split. |
| Safety-contract classes | U10's execution contract step 5 is split into three operation classes — in-place rewrite (preimage plus SHA comparison; absent-destination must **not** be asserted), archive move (absent destination, no-clobber), and declared intentional-collision rows (expected outcome is a refusal). Cycle 16 imposed absent-destination on all three, which is wrong for `resolve` and `abandon`. |
| Evidence-pair claim | U10b's no-clobber claim is narrowed to the **payload** move (`moveNoReplace`). The sidecar is written with `atomicfile.WriteFileAtomic` and is documented as an idempotent upsert, so it has replace semantics; the pair survives a second quarantine only because the payload move refuses first. Giving the sidecar its own no-replace create is stash `A12BBAFA`. |
| Live-corpus contradiction | Blocked-path handling no longer tells an operator to quarantine one of the nine live legacy files under A4c. That contradicted **A5**, which classifies live-corpus mutation `abandoned` and forbids it in this work. The remedy is now either "leave it — the read verbs report it and recovery is not blocked" or "authorize a separate unit of work". A5 remains read-only-forbidden. |
| Closure signals | Healthy, failure, and rollback signals are scoped to **conforming, active, undisposed** checkpoints. An unscoped signal would misclassify the pre-existing `ErrCheckpointNotActive` and `ErrCheckpointCannotResolveAbandoned` refusals as incidents. |
| Gate sequence | Rewritten as executable Windows PowerShell, branch-wide, with explicit empty-output assertions for the stdout-reporting checks. **Gate 4b** adds the import-ordering check `go run golang.org/x/tools/cmd/goimports@v0.39.0 -l .` — version-pinned from `go.sum`, module-independent so it adds no dependency, halting rather than skipping on a cold offline cache. **Gate 9** now captures `git rev-parse --short HEAD`, reproduces the Makefile `LDFLAGS` shape from `Makefile:6-8`, and **throws** on SHA inequality instead of leaving the assertion in a trailing comment. |
| P-016 | Recorded as a **Ship execution precondition**, not a plan defect: an unrelated linked worktree existed at decomposition time and must be finished or removed by its owner before Ship claims `130-S`. It is not a containment violation, and Stage does not touch it. |
| Adjacent intake | Stash `3A33E404` (malformed / truncated V1 JSON treated as legacy and written) and stash `E429A031` (create-boundary duplicate handling) are linked as related context only. Neither is absorbed: both concern the **create** boundary, this plan governs the **stored-document read and rewrite** boundaries, and absorbing either would breach the bounded scope of `D3CE9E81`. |

#### Topology delta

| Measure | Cycle 16 | Cycle 17 |
|---|---|---|
| Queued tasks under `147-F` | 29 | 40 |
| Queued-to-queued executable edges | 52 | 98 |
| Shipment `130-S` members | 30 | 41 |
| Ready roots | `{147.001-T}` | `{147.001-T, 147.032-T}` |
| Historical total edges | 53 | 99 |

Eleven tasks were created: `147.031-T` (U1c), `147.032-T` (U1d), `147.033-T` (U2h), `147.034-T`
(U11), `147.035-T` (U12), `147.036-T` (U13), `147.037-T` (U14), `147.038-T` (U15), `147.039-T`
(U16), `147.040-T` (U17), `147.041-T` (U10c). Five were retitled: `147.021-T` (U2f), `147.030-T`
(U1b), `147.016-T` (U8b), `147.029-T` (U7e), `147.015-T` (U8). No task was archived and no task ID
was renumbered. The second ready root is `147.032-T` (U1d), an independent leaf declaration; cycle
16 had a single root.

Shipment `130-S` remains one release unit and one future Ship PR. The five partitions are execution
phases inside that shipment, not separate release units: they share one branch, one merge commit,
and the U9/U9b hard merge gate, and splitting them would violate that gate by deferring the
instruction delta to a later PR.

## Plan Review

cycle: 17

dispatch_mode: single-agent-declared-degradation

TOOL_DEGRADED: reviewer-subagent-dispatch

decision: FAIL

severity counts: P0=0, P1=3, P2=2, P3=4

**This record is the current gate state.** It supersedes the `cycle: 16` `FAIL` record for
gate-decision purposes; cycle 16 and every earlier record remain the historical trace of how the
plan reached its present shape. The "PR #377 plan remediation, cycle 17 — formal decomposition"
appendix above is remediation evidence, not a gate outcome, and this record supersedes it as
*evidence of closure* wherever a finding below identifies a gap between that appendix's disposition
claims and the plan's actual normative text or the live backlog state.

### Dispatch record — degraded to a single-agent sequential pass

This is the fresh, independent dispatch the cycle-16 gate required against the decomposed plan.
Reviewer sub-agent dispatch was unavailable for this pass; `TOOL_DEGRADED:
reviewer-subagent-dispatch` records the degradation, matching the cycle-16 precedent. The gate ran
as a complete sequential, single-agent pass applying all seven personas' adapters over the full
plan text, the referenced backlog artifacts (`147-F` and its forty queued tasks), and
`.backlogit/stash.jsonl`, rather than the plan text in isolation. Coverage is complete: all seven
selected personas completed.

| Persona | Coverage mode | Coverage assignment |
|---|---|---|
| Constitution | sequential (single-agent) | Principle II red-posture narrative against the partition-3/partition-4 ordering the units themselves declare; Principle VII approval-gating language |
| Go | sequential (single-agent) | sentinel wrap verbs, `errors.Is` traversal, source-line citations against `internal/core` and `internal/events` at current HEAD |
| Scope | sequential (single-agent) | topology and task-count claims against the live `147-F` queue; unit reachability |
| Learnings | sequential (single-agent) | prior-cycle precedent for citation and cross-reference drift; self-hosted version-skew guidance reuse in Gate 7 |
| Architecture | sequential (single-agent) | U10/U10b/U10c ownership and sequencing narrative against the units' own declared contracts |
| Agent-Native Parity | sequential (single-agent) | cross-reference agreement between the plan, the `147.0xx-T` task bodies, and `.backlogit/stash.jsonl` |
| Security | sequential (single-agent) | evidence no-clobber claims for the archive payload move versus the disposition sidecar |

### Gate rationale

The gate is **FAIL**. The five DAG partitions, the dependency graph, the 40-task /
98-executable-edge / 41-member topology, and the two-root ready set are architecturally sound and
are **not** findings — this pass confirms the cycle-17 formal decomposition holds together
structurally. The FAIL is driven entirely by three P1 **synchronization** defects: normative plan
text, a Constitution Check narrative, and cross-referenced follow-up IDs that the cycle-17
remediation appendix (above) claims are closed or recorded, but that the plan's own body text and
the live backlog did not yet reflect. A gate cannot certify a remediation pass whose evidence trail
contradicts its own normative sections.

### Findings by severity

**P0 — 0**

None.

**P1 — 3**

| ID | Finding | Disposition |
|---|---|---|
| S1 | Plan Hardening's "Deepened runtime verification (U10, U10b)" recap block was not synchronized with the cycle-17 remediation: its title omitted U10c, its Teardown bullet still attributed teardown to U10b, its Recovery-procedure-contract step 5 still stated a universal absent-destination requirement instead of the three operation classes the cycle-17 appendix records as fixed ("Safety-contract classes"), its evidence no-clobber bullet still named "Copy-back" among the writes required to be no-clobber and overstated the sidecar as matching `moveNoReplace` semantics, and its Target-scenarios bullet enumerated only U10 and U10b's six rows, omitting U10c's three rows entirely. | Blocking. Fixed in this pass: block retitled to `(U10, U10b, U10c)`; Teardown reassigned to U10c; step 5 replaced with the three operation classes (in-place rewrite requires the existing target plus preimage, a normal archive move requires an absent destination, an intentional-collision test requires an occupied destination and unchanged evidence); the no-clobber bullet narrowed to the payload move, with the sidecar upsert named as not-yet-no-replace and tracked under stash `A12BBAFA`; Target scenarios extended with U10c's three rows. |
| S2 | Constitution Check Principle II's narrative sentence on U8b still read "U8b's parity harness lands during batch harness generation and fails against the declaration stubs of U6b/U6c/U7b/U7c/U8/U8c" — the withdrawn cycle-15/16 framing that U8b's own unit section explicitly disclaims. U8b is a partition-3 harness depending only on the partition-1/3 declarations U1, U1b, U1d, U2, U15, and it fails on assertion behaviour before any partition-4 implementation lands. | Blocking. Fixed in this pass: the sentence now states U8b lands in partition 3 against the U15/U1b/U1d/U2 declarations and fails on assertion behaviour before partition-4 implementation; the batch-harness-generation language is removed. |
| S3 | Follow-up stash IDs `F1A47C02` and `9C4B10D7` were cited as already-recorded across the plan (U6, U10b, Constitution Check, the cycle-17 appendix), `147.011-T`, and `147.032-T` / `147.026-T`, but neither existed in `.backlogit/stash.jsonl`. The cycle-17 appendix's "Adjacent intake" row also cited stash `B2657A3E` for the malformed/truncated `CreateCheckpoint` bug; that ID does not exist, and the bug it describes is already tracked under the existing entry `3A33E404`. | Blocking. Fixed in this pass: `F1A47C02` materialized as `F350503F` (remove `CheckpointSummary.RemediationCommand` after migration to `RemediationIntent`, with compatibility and removal tests); `9C4B10D7` materialized as `A12BBAFA` (harden the disposition sidecar write to no-replace semantics, with sidecar-only and concurrent-collision tests); every plan/task reference updated to the real IDs; every `B2657A3E` reference replaced with `3A33E404`. |

**P2 — 2**

| ID | Finding | Disposition |
|---|---|---|
| S4 | Gate 7 in the Final mandatory gate sequence (`backlogit --cwd . docs lint`) and the U9/U9b row in the red-verification command table invoked the ambient/installed `backlogit` binary rather than the current-source `go run ./cmd/backlogit --cwd . docs lint` that Gate 6 immediately above it, and the Dependency Graph's own regeneration note, both correctly use. An ambient binary predates the branch under test — the same self-hosted version-skew risk U10's binary-provenance discussion warns against for the verification binary. | Non-blocking; fixed in this pass. Both occurrences replaced with the `go run ./cmd/backlogit` form. |
| S5 | Session memory `docs/memory/2026-08-25/stage-pr377-cycle-17-formal-decomposition-memory.md` repeats the same unmaterialized `F1A47C02` / `9C4B10D7` placeholders and the incorrect `B2657A3E` reference. | Non-blocking; not corrected in this pass. Memory artifacts are treated as immutable historical record rather than live plan/task content; a future memory entry should note the corrected IDs rather than rewriting history. |

**P3 — 4**

| ID | Finding | Disposition |
|---|---|---|
| S6 | The Dependency Graph's prose stated the per-task prerequisite table's row count as "(39)"; the table itself carries 38 rows — one per task that declares at least one dependency, i.e. 40 tasks minus the two dependency-free roots `147.001-T` and `147.032-T`. | Non-blocking; fixed in this pass. |
| S7 | The `AbandonCheckpoint` validation-wrap citation `internal/core/checkpoint_disposition.go:~76-81` (repeated at three locations) pointed at the idempotent already-abandoned check; the actual `%v` wraps this unit (U17) rewrites are at lines 70 and 73. | Non-blocking; fixed in this pass. |
| S8 | The same stale placeholder IDs (`F1A47C02`, `9C4B10D7`, `B2657A3E`) also appear inside historical `.backlogit/checkpoints/*.json` and `.backlogit/memories.json` tool-managed artifacts. | Non-blocking; not corrected in this pass. These are point-in-time historical state snapshots, not live plan/task content; rewriting them would edit history rather than current state. |
| S9 | The U14 migration table cites `internal/core/checkpoint_disposition.go:105` for `AbandonCheckpoint`'s in-place rewrite; current source places the pre-write marshal call at line 105 and the atomic-write call at line 118 — adjacent lines inside the same rewrite sequence. | Non-blocking; not corrected in this pass. Low materiality; flagged for a future citation pass. |

### Rejected / stale findings

| Claim | Disposition |
|---|---|
| The five-partition decomposition or the dependency graph needs restructuring | Rejected — this pass confirms the cycle-17 partitions, edges, and the two-root ready set are architecturally sound. No topology change is warranted or made. |
| The measured topology (40 tasks / 98 executable edges / 41 members) is inaccurate | Rejected — verified against the live `147-F` queue; unchanged by this gate. |

### Remediation queue and restage recommendation

restage_recommendation: targeted-synchronization-fix

The gate is **FAIL** on three P1 synchronization defects, not on architecture or topology. Given the
narrow, text-and-reference nature of the gaps, unit-by-unit or partition-level restaging is not
warranted. The remediation queue actioned in this cycle: (1) retitle and correct the Deepened
runtime verification recap block; (2) correct the Constitution Check Principle II narrative; (3)
materialize the two missing follow-up stashes and correct the `B2657A3E` reference; plus the
non-blocking Gate 7 correction (S4) and two of the four non-blocking citation/count corrections
(S6, S7), bundled into the same pass because they touch the same files. S5, S8, and S9 remain open,
non-blocking follow-ups. A fresh, independent plan review must still be dispatched against the
corrected plan before implementation begins. Do not push this branch and do not hand this shipment
to Ship until that review passes.


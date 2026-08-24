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
| R1 | Shared read-boundary conformance helper, reflection-derived, disposition keys and `status:"abandoned"` legal | source doc, Decided behaviour §1 | U2 |
| R2 | `AbandonCheckpoint` refuses non-conforming targets before the audit append | source doc, Decided behaviour §2 | U4 |
| R3 | `ResolveCheckpoint` gains a `ValidateCheckpoint` gate and the same conformance check | source doc, Decided behaviour §3 | U3 |
| R4 | `QuarantineCheckpoint` widens malformed classification so the verb pair stays total over its scoped population | source doc, Decided behaviour §4 | U5 |
| R5 | No preservation carrier is added to `CheckpointV1` **for this scope** (decision-anchored, not a permanent ban) | source doc, Decided behaviour §5 | negative requirement — guarded in U2 |
| R6 | The nine live legacy files are left untouched by this work | source doc, Decided behaviour §6 | negative requirement — asserted in U10 (live-corpus hash guard) and U10b (mirror, not live, for the sweep) |
| R7 | Typed, machine-readable refusal naming the offending keys, with one canonical "quarantine is the remedy" predicate | plan-originated (source doc Unresolved Q1) | U1, U7, U7d, U8 |
| R8 | Every checkpoint **read** surface agrees with the mutation verbs about which files are rewrite-safe | plan-originated (source doc Unresolved Q3, widened by plan review) | U6, U6b, U6c, U6d, U8c |
| R9 | Human-facing design doc restates the verb pair as total over its scoped population | plan-originated (source doc Option B cons) | U9 |
| R10 | Agent-facing instruction surfaces teach the new `resolve` failure mode and the repair-or-quarantine remedy | plan-originated (plan review) | U9b |

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
(`internal/core/checkpoint_disposition.go:76-81`). Copying it verbatim would be a defect: `%v`
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

**Test-first posture (two-step red, mandatory).** A test file that references an undeclared
symbol does not compile, and a build error is **not** a red assertion. Development Workflow #1
requires a *compiling but failing* harness. Every code unit therefore runs in two steps:

1. **Declaration step** — land the minimum compilable stub so the package builds: the sentinel
   `var`, the type with `Error()` / `Unwrap()`, or the function with a `return nil` body. No
   behaviour.
2. **Harness step** — land the tests. They must now **compile and fail on assertions**. The
   expected red is recorded per unit below. Only then implement.

A unit is not red until `go test ./<pkg>` prints assertion failures rather than a build error.

**Declared regression guards.** Not every scenario listed under a unit is a red assertion. A
scenario that asserts already-shipped behaviour, or that expects `nil` from a `return nil`
declaration stub, passes from the moment it lands. Those cases are **declared regression guards**:
each unit's **Expected red** line names which of its cases fail and which are guards, and a unit
whose cases are *all* guards declares that explicitly (U2d, U8b). A guard is not a test-first
violation — silently counting one as red is, because it hides the fact that no assertion ever
failed.

### U1 — Non-conforming sentinel, typed error, and the canonical remedy predicate

* **Domain**: code (errors)
* **Files**: `internal/errors/checkpoint_errors.go`, `internal/errors/checkpoint_errors_test.go`
* **Change**: add `ErrCheckpointNonConforming`, `CheckpointNonConformingError{Fields []string}`
  with `Error()` and `Unwrap() error` returning the sentinel (mirroring
  `CheckpointUnknownFieldError`, `internal/errors/checkpoint_errors.go:81-108`), and the exported
  `QuarantineIsRemedy(err error) bool` predicate from Q1. `Fields` is sorted, de-duplicated key
  paths only — **never** key values.
* **Tests** (3): `errors.Is` matches the sentinel through the typed error and `errors.As` recovers
  `Fields` from a wrapped error; `Error()` renders the sorted joined field list; `QuarantineIsRemedy`
  is true for both `ErrCheckpointUseQuarantine` and `ErrCheckpointNonConforming` and false for
  `ErrCheckpointNotActive` and `nil`.
* **Expected red**: all three fail on assertions after the declaration step (typed error returns
  zero-value `Fields`, `QuarantineIsRemedy` returns `false`).

### U2 — Conformance helper: unknown top-level key rule

* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_conformance.go` (new), `internal/events/checkpoint_conformance_test.go` (new)
* **Change**: add `checkpointV1AllTopLevelKeys` — `modeledJSONTagKeys(reflect.TypeOf(CheckpointV1{}))`
  with **no** reserved-key subtraction — and exported `CheckConformingTopLevelNamespace(data []byte) error`.
  It reuses `decodeTopLevelEntries` and `isFoldKeyIn` unchanged and returns
  `*backlogiterrors.CheckpointNonConformingError` directly. It performs **no** reserved-status-value
  check: `status: "abandoned"` is a legal read-boundary value. A new file (not
  `checkpoint_strict.go`) because the read boundary and the create boundary have deliberately
  different legal key sets and must not read as one mechanism.
* **Tests** (3): a conforming V1 document returns nil; a document with two unknown top-level keys
  returns the typed error with both keys sorted; a document carrying all four `disposition*` fields
  **and** `status:"abandoned"` returns nil.
* **Expected red**: case 2 fails against the `return nil` stub. Cases 1 and 3 both expect `nil`, so
  they pass from the moment they land — declared regression guards, not red assertions.
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

### U2d — Key-set derivation parity and decision-anchored carrier guard

* **Domain**: tests
* **Files**: `internal/events/checkpoint_conformance_test.go`
* **Change**: assert `checkpointV1AllTopLevelKeys` equals `checkpointV1TopLevelKeys` unioned with
  `checkpointV1ReservedKeys`. Both derive from the same `modeledJSONTagKeys` call, so this guards
  drift in the **hand-written `checkpointV1ReservedKeys` literal**, not in the reflected field set —
  the narrower claim is the accurate one. Second, add a **decision-anchored** guard: `CheckpointV1`
  declares no `json:"-"` map carrier, with a comment naming
  `docs/decisions/2026-08-24-checkpoint-toplevel-key-disposition-deliberation.md`. This is a
  "revisit the decision before changing this" marker, **not** a permanent ban on top-level
  preservation (see Decisions and Rationale).
* **Tests** (3): set equality; absence of a `json:"-"` map carrier on `CheckpointV1`; **every
  exported field of `CheckpointV1` carries a non-empty `json:"..."` tag**. The third closes a
  latent escape hatch: `modeledJSONTagKeys` skips untagged exported fields, so a future field added
  without a tag would appear in *neither* derived set, leaving set equality silently satisfied while
  the field is invisible to both boundaries. Every field is tagged today, so this test is green on
  landing and guards drift.
* **Expected red**: none. **Posture: regression guard — green on landing, no red phase expected.**
  This is the one unit exempt from the two-step red rule, and the exemption is declared here rather
  than claimed as test-first.
* **Depends on**: U2.

### U2f — Protected invariant I1: checkpoint rewrite write-site enumeration

* **Domain**: tests
* **Files**: `internal/events/checkpoint_writesite_test.go` (new). No production change.
* **Change**: land the executable form of **I1** described in "Entry-point completeness audit"
  below, as a write-site enumeration test. The test walks the sources of **both** `internal/events`
  and `internal/core` for calls to `syncWriteFileAtomic` / `atomicfile.WriteFileAtomic` /
  `os.WriteFile` whose target resolves under the checkpoint directory and asserts the resulting
  call-site set equals the audited allow-list. `internal/core` is in scope because
  `AbandonCheckpoint` (`internal/core/checkpoint_disposition.go:~110-125`) calls
  `atomicfile.WriteFileAtomic` directly inside its `MutationEnvelope`; an enumeration limited to
  `internal/events` would leave that rewrite invisible — the exact hole I1 exists to close.
* **Mechanism is decided here, not deferred**: the previously offered "or a single exported
  `events.RewriteCheckpointFile` seam" fallback is **withdrawn**. Carrying both mechanisms made the
  unit's file set either one new test file or five files across two packages and two skill domains,
  so its size could not be known before work started and the larger branch breached both the
  three-file heuristic and width isolation. The withdrawal is recorded in Decisions and Rationale.
* **Halt condition**: if the enumeration cannot be implemented reliably — for example if call-target
  resolution proves ambiguous — mark the unit `blocked` and re-plan the gated-seam alternative as
  its own units under a new task ID. Do **not** grow this unit into the seam.
* **Why this is a separate unit from U2d**: I1's executable form is a distinct deliverable that U3b
  and U4 must satisfy, so the allow-list must be pinned before those units touch their call sites.
  Bundling it into U2d would take that unit to four scenarios across two skill domains — breaching
  both the width-isolation rule and the 2-hour granularity rule.
* **Tests** (2): the enumerated call-site set equals the allow-list across both packages; a
  synthetic ungated rewrite site added to the fixture corpus fails the assertion.
* **Expected red**: case 2 fails until the enumeration exists.
* **Depends on**: U2d.

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
  are regression guards.
* **Depends on**: U2.

### U3b — `ResolveCheckpoint` conformance gate and the named already-resolved exception

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
* **Depends on**: U3, U6.

### U4 — `AbandonCheckpoint` conformance gate

* **Domain**: code (core)
* **Files**: `internal/core/checkpoint_disposition.go`, `internal/core/checkpoint_disposition_test.go`
* **Change**: call `events.CheckConformingTopLevelNamespace(data)` **immediately after
  `ValidateCheckpoint` and BEFORE the already-abandoned short-circuit**, returning its typed error
  unchanged. Placing it before the short-circuit matters: a file carrying
  `disposition: "abandoned"` *plus* an unmodeled key would otherwise return `nil` (reported
  success, no write) from abandon while U5's widened quarantine accepts it and U6 reports
  `NeedsQuarantine: true` — three surfaces disagreeing about one file. It is a non-writing refusal,
  so nothing is lost by refusing earlier. It remains strictly before
  `appendCheckpointDispositionAudit`, preserving the shipped audit-then-mutate ordering.
* **Tests** (3): a valid-but-non-conforming active document is refused with
  `ErrCheckpointNonConforming` naming the keys; the disposition audit JSONL is byte-unchanged after
  that refusal; a non-conforming **already-abandoned** document returns `ErrCheckpointNonConforming`
  rather than `nil`. The existing "conforming active document abandons successfully" test must stay
  green as a regression guard.
* **Expected red**: all three fail.
* **Depends on**: U2c.

### U5 — `QuarantineCheckpoint` widened classification (deadlock closure)

* **Domain**: code (core)
* **Files**: `internal/core/checkpoint_disposition.go`, `internal/core/checkpoint_disposition_test.go`
* **Change**: extend the in-memory classification so `validTarget` is
  `parse OK && validate OK && conformance OK`. Only a target satisfying all three is refused with
  `ErrCheckpointUseAbandon`. The verbatim `moveNoReplace` path, the audit-before-move ordering, the
  sidecar upsert, and the `MutationEnvelope` compensation are unchanged.
* **Tests** (3, expressed as one table so the paired assertions cannot be removed independently):
  a valid-but-non-conforming **active** document is **accepted** by quarantine and **refused** by
  abandon in the same table row; a fully conforming active document is refused by quarantine with
  `ErrCheckpointUseAbandon`; a quarantined non-conforming document's archived bytes are
  byte-identical to the original.
* **Expected red**: row 1's accept-half fails (quarantine currently refuses it).
* **Depends on**: U2c, U4.

### U5b — State-dimension classification row (I3 scoping)

* **Domain**: tests
* **Files**: `internal/core/checkpoint_disposition_test.go`
* **Change**: no production change. Pin the **pre-existing** state-conflict class that invariant I3
  is explicitly scoped to exclude: a conforming, valid, `status: "resolved"` checkpoint is refused
  by `abandon` with `ErrCheckpointNotActive` **and** by `quarantine` with `ErrCheckpointUseAbandon`.
  This is shipped behaviour, not introduced here, and widening quarantine to accept it is out of
  scope — but leaving it untested would let a future reader believe I3's totality claim covers it.
* **Tests** (2): the `status:"resolved"` conforming row asserts both refusals with their exact
  sentinels; a `status:"active"` conforming row asserts abandon accepts, proving the discriminator
  is `status` and not conformance.
* **Expected red**: none — regression guard, declared exempt from the two-step red rule.
* **Depends on**: U5.

### U6 — `ListCheckpoints` surfaces non-conforming files

* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_lifecycle.go`, `internal/events/checkpoint_lifecycle_test.go`
* **Change**: after the existing `ValidateCheckpoint` branch, run
  `CheckConformingTopLevelNamespace(data)`; on failure set `NeedsQuarantine = true` and
  `RemediationCommand`, and **append** to `ValidationErr` rather than overwriting it — a file can
  fail both validation and conformance and the operator needs both reasons.
  `RemediationCommand` must be **PowerShell-safe** (this is a Windows-first workspace: no
  unescaped double quotes, no backticks, runnable as-is when pasted into `pwsh`).
  `ListCheckpoints` stays strictly read-only — no move, no rewrite, no error propagation — and the
  conformance branch must run **before** the filter block, so the verdict is computed for every
  parsed document rather than only for documents the caller's filter happens to select.
  **Ordering is not exemption**: only the `ParseCheckpoint` failure path is filter-exempt today
  (`internal/events/checkpoint_lifecycle.go:~46-57` appends and `continue`s before the filter
  block), while the `valErr` branch falls through into the `Agent` / `Status` / `ShipmentID` /
  `FeatureID` / `MaxAge` checks like any other summary. Whether a quarantine candidate also becomes
  filter-exempt is a separate behavioural change owned by **U6d**; this unit must not claim a
  drop-through guarantee it does not implement.
* **Tests** (4): a valid-but-non-conforming file lists with `NeedsQuarantine: true` and a
  PowerShell-safe remediation command; a file failing **both** validation and conformance reports
  both reasons in `ValidationErr`; the verdict is computed before the filter block, asserted by a
  case whose filter matches the document; the files on disk are byte-unchanged after listing.
* **Expected red**: cases 1 and 2 fail.
* **Depends on**: U2c.

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
* **Depends on**: U6.

### U6b — `GetCheckpoint` agrees with `ListCheckpoints`

* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_lifecycle.go`, `internal/events/checkpoint_lifecycle_test.go`
* **Change**: `GetCheckpoint` currently reports `valid: true` for any document that parses and
  validates (`internal/events/checkpoint_lifecycle.go:105-137`). After U4/U5 that is actively
  misleading: an agent running the canonical `list` → `get` → choose-verb sequence would read
  `needs_quarantine: true` from list and `valid: true` from get, then pick the verb the plan just
  closed. Add a `Conforming bool` field (and reuse `NeedsQuarantine` / `RemediationCommand`) to the
  get result so both read surfaces answer the same question. **There is no get-result type today**:
  `GetCheckpoint` (`internal/events/checkpoint_lifecycle.go:108`) returns `(*CheckpointV1, error)`,
  and `NeedsQuarantine` / `RemediationCommand` exist only on `CheckpointSummary`
  (`internal/events/checkpoint_schema.go:371`), so this unit must declare the carrier before it can
  populate it. Declare an exported
  `CheckpointReadResult{Checkpoint *CheckpointV1; Valid, Conforming, NeedsQuarantine bool; RemediationCommand string}`
  returned by a new `GetCheckpointResult(ctx, checkpointDir, filename) (*CheckpointReadResult, error)`,
  and retain `GetCheckpoint` as a thin wrapper returning `res.Checkpoint` so every existing caller
  compiles unchanged. `valid` retains its existing meaning (schema-valid) and is **not** repurposed;
  conformance is reported as a distinct field so no existing consumer's contract silently changes.
  Schema-invalid documents keep returning `ErrCheckpointInvalid` — this unit adds conformance
  reporting for **valid-but-non-conforming** documents only. That sentinel is returned
  **unwrapped**: `GetCheckpointResult` does not wrap it in `ErrCheckpointUseQuarantine`, because a
  read is not a rewrite and there is nothing to refuse. Downstream surfaces must therefore expect
  the pre-existing validation-class refusal on `get`, not a disposition code.
* **Tests** (3): a valid-but-non-conforming file returns `valid: true, conforming: false,
  needs_quarantine: true` with a non-empty `RemediationCommand`; a conforming file returns
  `conforming: true`; the file is byte-unchanged after get.
* **Expected red**: cases 1 and 2 fail (the result type does not exist until the declaration step,
  then returns zero values).
* **Consumed by**: U6c projects this result onto the MCP `get_checkpoint` response; U8c
  (`147.027-T`) projects it onto `backlogit checkpoint get`.
* **Depends on**: U2c.

### U6c — MCP `get_checkpoint` projects the conformance verdict

* **Domain**: code (mcp)
* **Files**: `internal/mcp/tools.go`, `internal/mcp/checkpoint_disposition_test.go`
* **Change**: `handleGetCheckpoint` (`internal/mcp/tools.go:1194-1212`) returns a **literal**
  `"valid": true` and carries no conformance field, so the MCP read surface would keep answering the
  superseded question after U6b lands. Call `events.GetCheckpointResult` and project `valid`,
  `conforming`, `needs_quarantine`, and `remediation_command` from the returned result; the
  hardcoded `"valid": true` is removed, not shadowed. Without this unit U7b's `get_checkpoint`
  description and U8b's MCP `get` parity rows would describe behaviour no unit implements.
  **Schema-invalid documents keep their existing refusal.** `handleGetCheckpoint` routes errors
  through `domainError` (`internal/mcp/errors.go:148`), which takes no filename and already maps
  `ErrCheckpointInvalid` to `code: validation_failed`. U6b returns that sentinel unwrapped, so
  `get` on a legacy file surfaces the pre-existing validation-class refusal — **not**
  `checkpoint_use_quarantine`, which U7 only ever emits from `checkpointDispositionError` on the
  *mutation* handlers. Demanding a disposition code here would require re-routing the read handler
  through a mutation-shaped error path, widening this unit into U7's file set and changing a
  shipped read contract for no safety gain: the quarantine remedy is already discoverable from
  `list_checkpoints`, which reports `needs_quarantine: true` for the same file (U6).
* **Tests** (3): a valid-but-non-conforming file returns `valid: true, conforming: false,
  needs_quarantine: true` and a non-empty `remediation_command`; a conforming file returns
  `conforming: true`; a schema-invalid file returns the pre-existing `validation_failed` refusal
  produced by `domainError` from an unwrapped `ErrCheckpointInvalid`, rather than a success payload
  asserting validity, asserted against the handler's actual payload.
* **Expected red**: cases 1 and 2 fail; case 3 is a declared regression guard pinning the shipped
  read contract.
* **Depends on**: U6b (result type). The former dependency on U7 is removed — this unit pins the
  existing error mapping rather than consuming the new one.

### U7 — MCP error mapping and response shape

* **Domain**: code (mcp)
* **Files**: `internal/mcp/errors.go`, `internal/mcp/checkpoint_disposition_test.go`
* **Change**: four coupled defects, all in the mapping layer. The handler-routing half of the
  original U7 is now **U7d**, because it touches `internal/mcp/tools.go` and would have taken this
  unit to three files across two behavioural surfaces.
  1. `checkpointDispositionError` (`internal/mcp/errors.go:309`) currently emits
     `checkpoint_use_quarantine` for the abandon/quarantine handlers only; add
     `ErrCheckpointNonConforming` with `Code: "checkpoint_non_conforming"`. Its existing
     `ErrCheckpointNotFound` → `NotFound` case and its `default: InternalError` tail both stay.
  2. The two existing response shapes are incompatible: `checkpointUnknownFieldsResponse`
     (`internal/mcp/errors.go:29-39`) carries `error`/`message`/`unknown_fields` while
     `checkpointDispositionErrorResponse` (`:291-299`) carries
     `error`/`message`/`code`/`filename`/`retryable`/`outcome`/`remediation`. Extend
     `checkpointDispositionErrorResponse` with `UnknownFields []string \`json:"unknown_fields"\`` —
     **no `omitempty`** — populated via `errors.As`, so one refusal shape answers both "what went
     wrong" and "which keys".
  3. `domainError` (`internal/mcp/errors.go:148`) is the fallback surface for handlers that carry no
     filename. **Correcting a stale claim in this plan**: earlier text asserted it had "no case for
     `ErrCheckpointUseQuarantine`, `ErrCheckpointInvalid`, or `ErrCheckpointNonConforming`". The
     `ErrCheckpointInvalid` third of that is false — `internal/mcp/errors.go:~188-193` already maps
     it to `validation_failed`, grouped with `ErrValidation` and `ErrCheckpointCorrupt`, and a
     dedicated `ErrCheckpointUnknownField` case already precedes it. Only
     `ErrCheckpointUseQuarantine` and `ErrCheckpointNonConforming` are genuinely absent; add those
     two as a **safety net** so a refusal reaching a handler that has not been re-routed can never
     surface as a 500. Add the two rows to the mapping-table doc comment
     (`internal/mcp/errors.go:127-144`).
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
* **Tests** (4): `checkpointDispositionError` returns `checkpoint_non_conforming` for
  `ErrCheckpointNonConforming` and its `ErrCheckpointNotFound` → `NotFound` case still fires;
  `domainError` maps the two missing sentinels to their named codes instead of falling to
  `default: InternalError`; invoking `handleAbandonCheckpoint` on a non-conforming target returns
  `checkpoint_non_conforming` with a populated `unknown_fields` read through a `.([]any)` type
  assertion so an absent or `null` key fails
  (`docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md`) and a
  `remediation` string naming `backlogit_abandon_checkpoint` as the originating verb (proving the
  op-derived interpolation is in place — the assertion holds because abandon is the caller here,
  but the formatter is not hardcoded to it, which U7d's resolve-side assertions confirm from the
  other direction); a conforming refusal returns `unknown_fields: []` rather than omitting the
  key.
* **Expected red**: all four fail.
* **Depends on**: U1, U3b, U4, U5.

### U7d — `handleResolveCheckpoint` routes disposition refusals through the disposition shape

* **Domain**: code (mcp)
* **Files**: `internal/mcp/tools.go`, `internal/mcp/checkpoint_disposition_test.go`
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
  `checkpointDispositionError` does not name — `ErrCheckpointCannotResolveAbandoned` and
  `ErrCheckpointCorrupt` map to `validation_failed` through `domainError` today and would fall to
  that function's `default: InternalError` tail. `ErrCheckpointNotFound` is safe either way: it is
  handled explicitly at `internal/mcp/errors.go:~358` before the default. The predicate matches both
  new refusals: U3 wraps the validity gate as
  `fmt.Errorf("%w: %w", ErrCheckpointUseQuarantine, valErr)` and U3b returns
  `ErrCheckpointNonConforming`, so `errors.Is` matches through the wrap.
* **Tests** (4): invoking the **`handleResolveCheckpoint` handler** (not the events function) on a
  schema-invalid legacy document returns `code: checkpoint_use_quarantine` with a populated
  `filename`, a `remediation` string naming `backlogit_resolve_checkpoint` (not
  `backlogit_abandon_checkpoint`) as the originating verb — pinning U7's op-derived interpolation
  from the resolve side, so the formatter cannot silently regress to a hardcoded wronged verb —
  and explicitly asserts the payload is **not** `"error":"internal"`; invoking it on a
  valid-but-non-conforming document returns `code: checkpoint_non_conforming` with `unknown_fields`
  non-empty and a `remediation` string naming `backlogit_resolve_checkpoint` as the originating
  verb; a missing file still returns the pre-existing not-found refusal; an already-abandoned
  target still returns its pre-existing `validation_failed` refusal, proving the non-disposition
  path still reaches `domainError`.
* **Expected red**: cases 1 and 2 fail (routing and both remediation-verb assertions); cases 3 and
  4 are declared regression guards.
* **Depends on**: U1 (the predicate and its host package), U7 (the response shape, the code, and
  the op-derived remediation the resolve-side assertions read).

### U7b — MCP read-surface tool descriptions (exact replacement strings)

* **Domain**: docs (agent-facing tool contract)
* **Files**: `internal/mcp/tools.go` (descriptions at `:176-192`), `internal/mcp/tools_test.go`;
  `.autoharness/backlog-registry.yaml` and an `internal/cli/registry_parity_test.go` re-run only if
  the registry carries description text for these two tools
* **Change**: the **two read-surface** descriptions. The three mutation-surface descriptions moved
  to **U7c**: five rows in one unit exceeded the four-scenario limit and mixed a read contract with
  a refusal contract. This table is the single source of truth and is reproduced verbatim in
  `147.014-T`; the two must not drift.

  | Line | Tool | Delta |
  |---|---|---|
  | `:178` | `list_checkpoints` | append: ` A summary with needs_quarantine true is not safely rewritable; use quarantine_checkpoint, not resolve_checkpoint or abandon_checkpoint. Such a summary is returned regardless of the status, agent, shipment_id, feature_id, and max_age filters, so a filtered result can contain rows that do not match the filter.` |
  | `:189` | `get_checkpoint` | append: ` For a schema-valid document, returns conforming false when it carries unmodeled top-level keys; such a document cannot be resolved or abandoned. A schema-invalid document is refused before any conformance verdict is produced.` |

  The `get_checkpoint` qualifier is load-bearing: `GetCheckpoint` runs `ValidateCheckpoint` and
  returns `ErrCheckpointInvalid` before any conformance result exists
  (`internal/events/checkpoint_lifecycle.go:~105-137`), so an unqualified "returns conforming
  false for a document with unmodeled top-level keys" would promise a verdict the read path
  cannot produce for the nine legacy files. The `list_checkpoints` filter sentence was added in
  PR #377 review cycle 4 and is why this unit now depends on **U6d**: a published tool description
  must not promise an exemption no shipped code performs.
* **Tests** (2): a table-driven assertion over the two registered read descriptions, read from the
  **built tool set** rather than a duplicated literal; and the existing registry-parity /
  fallback-map drift test
  (`docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`, Rule 1)
  re-run and staying green, with `.autoharness/backlog-registry.yaml` updated in the same commit
  if it carries description text for these tools.
* **Expected red**: both rows of case 1 fail. Case 2 is a declared regression guard.
* **Depends on**: U6b, U6c, U6d.

### U7c — MCP mutation-surface tool descriptions (exact replacement strings)

* **Domain**: docs (agent-facing tool contract)
* **Files**: `internal/mcp/tools.go` (descriptions at `:193-224`), `internal/mcp/tools_test.go`;
  `.autoharness/backlog-registry.yaml` and an `internal/cli/registry_parity_test.go` re-run only if
  the registry carries description text for these three tools
* **Change**: the **three mutation-surface** descriptions. Split out of U7b. This table is the
  single source of truth and is reproduced verbatim in `147.024-T`; the two must not drift.

  | Line | Tool | Delta |
  |---|---|---|
  | `:196` | `resolve_checkpoint` | append: ` Refuses a stored document it cannot safely rewrite rather than replacing it: checkpoint_use_quarantine when the document is schema-invalid, checkpoint_non_conforming when it carries unmodeled top-level keys. Use quarantine_checkpoint instead.` |
  | `:211` | `abandon_checkpoint` | append: ` Also refuses when the document carries unmodeled top-level keys.` |
  | `:220` | `quarantine_checkpoint` | replace `malformed checkpoint file` → `checkpoint file that cannot be safely rewritten (malformed, schema-invalid, or carrying unmodeled top-level keys)` |

  The `resolve_checkpoint` row promises two **codes**, which only reach that surface once U7d
  routes the handler through `checkpointDispositionError` — hence the dependency on U7d as well as
  on the U7 mapping.
* **Tests** (2): a table-driven assertion over the three registered mutation descriptions, read
  from the **built tool set**, each row asserting its required substring and that
  `resolve_checkpoint` distinguishes `checkpoint_use_quarantine` from `checkpoint_non_conforming`;
  and the registry-parity / fallback-map drift test re-run and staying green.
* **Expected red**: all three rows of case 1 fail. Case 2 is a declared regression guard.
* **Depends on**: U7, U7d.

### U8 — CLI refusal surfacing

* **Domain**: code (cli)
* **Files**: `internal/cli/checkpoint.go`, `internal/cli/checkpoint_test.go`
* **Change**: surface the new refusals on `backlogit checkpoint resolve` and
  `backlogit checkpoint abandon` as actionable operator messages carrying the PowerShell-safe
  quarantine remediation command, matching the existing CLI error idiom. **The offending-key list
  is only available for a valid-but-non-conforming target.** A schema-invalid legacy document is
  refused by the U3 validity gate before conformance runs, so no key list exists for it; that
  refusal prints the validation reason and the remediation command instead. **The `checkpoint get`
  conformance projection is not in this unit**: it moved to **U8c** in PR #377 review cycle 4,
  because a read projection is a different contract from a refusal rendering and folding it in
  made a fourth scenario. This unit must not touch `newCheckpointGetCmd`. **No JSON error envelope is added**
  — that is stash `63E810D9` and stays out of scope; the CLI/MCP shape asymmetry it describes is a
  documented, pre-existing condition, restated in U9b rather than fixed here.
* **Tests** (3): `checkpoint resolve` on a **schema-invalid legacy** document exits non-zero,
  reports the `checkpoint_use_quarantine` class with the validation reason and the remediation
  command, and prints **no** key list; `checkpoint resolve` on a **valid-but-non-conforming**
  document exits non-zero and names the offending top-level keys alongside the same command;
  `checkpoint abandon` on that same valid-but-non-conforming document does likewise. The
  PowerShell-safety assertion rides on the rendered command in cases 2 and 3 rather than being a
  fourth scenario, keeping this unit inside the 2-Hour Rule.
* **Expected red**: all three fail.
* **Depends on**: U7.

### U8c — CLI `checkpoint get` projects the conformance verdict

* **Domain**: code (cli)
* **Files**: `internal/cli/checkpoint.go`, `internal/cli/checkpoint_test.go`
* **Change**: `newCheckpointGetCmd` (`internal/cli/checkpoint.go:180-210`) calls
  `events.GetCheckpoint` and prints a **literal** `"valid": true` with no conformance field, so the
  CLI read surface would keep answering the superseded question after U6b lands. Call
  `events.GetCheckpointResult` and project `valid`, `conforming`, `needs_quarantine`, and
  `remediation_command`; the hardcoded literal is removed, not shadowed. This is the CLI twin of
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
  `needs_quarantine: true`, and a non-empty `remediation_command`; a conforming file reports
  `conforming: true`; a schema-invalid file exits non-zero with the pre-existing validation-class
  refusal rather than a success payload asserting validity, asserted against the command's actual
  output rather than a literal. The "literal `"valid": true` is gone" assertion rides on cases 1
  and 2 rather than being a fourth scenario.
* **Expected red**: cases 1 and 2 fail; case 3 is a declared regression guard pinning the shipped
  read contract.
* **Depends on**: U6b.

### U8b — Cross-surface parity from one stored state

* **Domain**: tests
* **Files**: `internal/cli/checkpoint_parity_test.go` (new). **No production change.**
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
  accept/refuse verdict and the same remedy verb, and that every fixture file is byte-identical
  after all three surfaces have been exercised.
* **Expected red**: none. **Posture: regression guard — this unit is the parity contract itself.**
  It lands after U7b, U7c, U8, U8c, and U6c, so all three surfaces already carry the behaviour; the
  unit exists to pin their agreement, and the exemption is declared here rather than claimed as
  test-first (same precedent as U2d).
* **Depends on**: U6c, U7b, U7c, U8, U8c.

### U9 — Design doc: total classification

* **Domain**: docs
* **Files**: `docs/design-docs/checkpoint-administrative-disposition.md`, regenerated
  `docs/cli-reference/backlogit_checkpoint_*.md`
* **Change**: restate the "Malformed-Only vs Valid-Only Split Rationale" section as a
  **state-scoped four-class** classification. For a `status: "active"` target:

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
* **Expected red**: n/a (docs-only; runs after behaviour is final).
* **Depends on**: U6b, U8b.

### U9b — Agent instruction file and the body-preserving repair path

* **Domain**: docs (agent-facing)
* **Files**: `.github/instructions/backlogit.instructions.md`
* **Change**: the Lifecycle Hygiene Protocol currently teaches every agent in this workspace
  (`applyTo: '**'`) that abandon and quarantine are disjoint by validity alone and treats `resolve`
  as infallible. After U3/U3b/U4/U5 that guidance is wrong at exactly the moment an agent needs it —
  session-start recovery. Update it to state: `resolve` may now return `checkpoint_use_quarantine`
  or `checkpoint_non_conforming` and the remedy is quarantine, not retry; the disjointness
  discriminator is validity **and** top-level conformance; and — following the Guidance section of
  `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md` — a **body-preserving
  hand-repair** procedure so quarantine is not the only escape from a checkpoint an operator still
  wants. This is **one** procedure with two entry points, and both must be written out verbatim so
  this file, task `147.018-T`, and U10b's restore row cannot drift apart:

  **Validity is a precondition, not part of the repair.** `checkpoint get` reports validity and
  conformance as **separate** outcomes (U6b, U6c, U8c): a schema-invalid stored document is refused
  with `ErrCheckpointInvalid` **before** any conformance verdict is produced, so the classified
  offender list entry point (a) reads does not exist for that document. Entry point (a) is
  therefore the repair path for a stored document `checkpoint get` reports as
  `valid: true, conforming: false` **only**. A schema-invalid stored document is routed to
  `quarantine_checkpoint` directly — moving unmodeled keys cannot fix a validation defect
  (`legacy_top_level` relocates keys, not fixes shapes), and this instruction file promises **no**
  in-place repair for arbitrary validation defects. That is the same disposition U3's validity gate
  already forces on `resolve` (via the `ErrCheckpointUseQuarantine` wrap) and U4/U5 already forces
  on `abandon`; the instruction file's job is to teach the operator that the two dispositions
  agree, not to invent a hidden third repair for validation defects. The two disjoint entry
  conditions are therefore:

  * **(a) direct repair** — active file, `checkpoint get` reports `valid: true, conforming: false`.
  * **(b) post-quarantine restore** — file already quarantined; the operator wants the evidence
    back under active-checkpoint semantics.

  **(a) Direct repair** — the document is still under `.backlogit/checkpoints/` and
  `checkpoint get` has already reported `valid: true, conforming: false` on it. First **classify
  every offender** the conformance verdict reported, because they are not all repairable by moving
  keys:

  | Reported offender | Repair |
  |---|---|
  | `<key>` — a plain unmodeled top-level key | Move it under `context.legacy_top_level`, preserving name and value byte-for-byte. |
  | `duplicate:<key>` where one member is modeled | Move the **unmodeled** variant under `context.legacy_top_level`; the modeled member stays where it is. |
  | `duplicate:<key>` where **both** members are unmodeled | Move **both**, keeping their original names as distinct members of `context.legacy_top_level`. The container is unmodeled, so the pair round-trips verbatim and neither value is chosen over the other. |
  | `duplicate:<key>` where **both** members are modeled (for example two identical `status` members) | **Not repairable by moving keys.** Both members occupy the same modeled top-level slot and the collapsing decode silently picks one on re-marshal, so `legacy_top_level` cannot relocate the pair out of the loss path. Repair is allowed **only** with an explicit **operator choice** naming which member survives, recorded in the task log — an information-destroying choice that only a human may make. Without that recorded choice, **quarantine the document** and keep the verbatim bytes. **Never silently select a value.** |
  | `duplicate:progress.<key>` — a nested fold-variant pair inside `progress` (U2e) | **Not repairable by moving keys.** `progress` is a modeled field and `legacy_top_level` is a top-level container, so no move relocates the pair out of the collapsing decode. Either the operator decides which member survives — an information-destroying choice that only a human may make, recorded in the task log — or, if neither member may be dropped, **quarantine the document** and keep the verbatim bytes. |

  Then move every key the table assigns to the container object `context.legacy_top_level`,
  preserving each original key name and value byte-for-byte. **Do not flatten the keys directly
  into `context`.** `CheckpointContext.UnmarshalJSON`
  (`internal/events/checkpoint_schema.go:196-220`) skips any `context` member whose name is
  `strings.EqualFold`-equal to a modeled context field (`shipment_id`, `feature_id`, `task_ids`,
  `branch`) when populating `Extra`, so a flattened top-level `shipment_id` landing beside an
  existing `context.shipment_id` silently destroys one of the two values on the next re-marshal —
  the same loss class this feature exists to close. `legacy_top_level` is unmodeled, so it
  round-trips through `Extra` intact, and nesting **every** moved key under it makes the rule
  uniform rather than requiring per-key fold comparisons. If `context.legacy_top_level` already
  exists, merge into it and **refuse to overwrite an existing member** — stop and reconcile that
  conflict by hand under the same rule, never silently dropping one of two values. Then re-run
  `checkpoint get` to confirm `conforming: true` and use the normal verb (`resolve` or `abandon`).
  **Termination rule**: if `checkpoint get` still reports `conforming: false` after the classified
  moves are applied, the document holds an offender the move cannot relocate — a nested `progress`
  duplicate is the known case. **Stop and quarantine; do not iterate the repair**, because
  repeating a move-only repair against a nested offender cannot converge.

  **(b) Post-quarantine restore** — the document was already quarantined, so its bytes and its
  `<filename>.disposition.json` sidecar both live under `archive/checkpoints/`. **Never copy the
  archived file back under its original name while the archive copy is still there.** That leaves
  the same filename in `.backlogit/checkpoints/` *and* `archive/checkpoints/`, and the next
  `CleanupCheckpoints` sweep archives the active copy over the quarantined one — on Windows it
  calls `os.Remove(dst)` before the rename (`internal/events/checkpoint_lifecycle.go:238-242`),
  destroying the quarantined evidence outright. Reconcile the archive **first**, in this order:

  1. Rename the archived pair out of the cleanup-destination namespace, keeping the two files
     together: `archive/checkpoints/<filename>` →
     `archive/checkpoints/<filename>.quarantined-<disposition_at>` and
     `archive/checkpoints/<filename>.disposition.json` →
     `archive/checkpoints/<filename>.quarantined-<disposition_at>.disposition.json`, where
     `<disposition_at>` is the sidecar's timestamp rendered in compact UTC basic format
     (`20060102T150405Z`) so the name is legal on Windows. The evidence name no longer ends in
     `.json`, so it can never be a `CleanupCheckpoints` destination and can never be matched by the
     `checkpoint-*.json` glob. The sidecar still records the original base name in its `filename`
     field, so provenance survives the rename.
  2. **Copy** — never move — the preserved bytes to `.backlogit/checkpoints/<filename>`. The
     sidecar stays in the archive; it describes the quarantine event, not the restored working
     copy, and must not be carried into the active directory (a `checkpoint-*.json.disposition.json`
     file there would be swept up by the `checkpoint-*.json` glob). Stop if the active path already
     exists: never overwrite a live checkpoint.
  3. Apply entry point (a) unchanged to the restored file — including its schema-valid precondition.
     The precondition **is not a formality here**: quarantines routed by U3 (validity gate on
     resolve) or U4/U5 (validity gate on abandon) preserve schema-invalid bytes, so the archived
     evidence may fail validation. If `checkpoint get` refuses the restored bytes with
     `ErrCheckpointInvalid`, entry point (a) is inapplicable — the classified offender list does
     not exist for that document, and moving unmodeled keys cannot fix a validation defect. If
     entry point (a) is inapplicable, or if its **termination rule** fires (`conforming: false`
     persists after the classified moves because the offender is a nested `progress` duplicate or
     other move-untouchable class), **abort the restore**: remove `.backlogit/checkpoints/<filename>`
     and leave the file quarantined. The renamed archive evidence at
     `archive/checkpoints/<filename>.quarantined-<disposition_at>` and its
     `.disposition.json` sibling are **untouched** by the abort — they are the only verbatim
     record of the pre-quarantine bytes, and the append-only disposition audit log still names the
     quarantine event. The restore has succeeded only when `checkpoint get` reports the active copy
     as **both** `valid: true` **and** `conforming: true`; otherwise preserved evidence remains
     quarantined, exactly as if step 2 had never run.

  The renamed evidence pair is **retained, not deleted**: it is the only verbatim record of the
  pre-repair bytes, and the quarantine event itself remains in the append-only disposition audit
  log. Both entry points preserve the original document body. Neither creates a **replacement**
  checkpoint: a fresh file would abandon the original filename, `created_at`, and session identity,
  which is a different operation and must not be described as the repair path. State that quarantining a checkpoint whose
  `status` is `active` moves live recovery state to the archive and should be a deliberate operator
  decision, not an automatic agent reflex.
* **Tests**: `backlogit docs lint` reports 0 violations on the changed file.
* **Acceptance / merge gate**: this unit and U9 must land in the **same merge commit** as U3b/U4/U5.
  A pull request that includes any of U3b, U4, or U5 **MUST NOT be merged** unless the
  `.github/instructions/backlogit.instructions.md` delta from this unit is present in that same
  merge commit. This is a merge-checklist item, not a recommendation: shipping the behaviour change
  without the instruction update would leave every agent in the workspace following superseded
  session-start recovery guidance at exactly the moment the guidance becomes wrong.
* **Depends on**: U9.

### U10 — Runtime verification of the refusal path

* **Domain**: verification
* **Files**: `.gitignore` (scratch-directory ignore rule); otherwise none (produces `docs/closure/`
  evidence)
* **Change**: none to product code. Build the binary **from the branch under test** (not the pinned
  repo-root `backlogit.exe`, which predates the change) and exercise the **refusal** path against a
  **scratch** workspace seeded with copies of the legacy document shapes. The acceptance and
  restore rows moved to **U10b**: five rows exceeded the four-scenario limit, and the restore row
  also carried a contradiction with the live-corpus guard (see U10b). The nine
  live files under `.backlogit/checkpoints/` are read for shape reference only and are never
  mutation targets (R6); the check is a programmatic before/after SHA-256 comparison of **every**
  file under `.backlogit/checkpoints/`, not a visual one, and not a count-pinned subset — twelve
  files are present on this branch now that the staging checkpoint has landed, and that number
  drifts as sessions add checkpoints, so the guard enumerates the directory rather than a literal.
* **Rows** (3): **read verdict** — `list` reports `needs_quarantine: true` with a paste-runnable
  PowerShell-safe remediation command and `get` reports `conforming: false` for the same fixture;
  **refusal** — legacy-shaped resolve refused and valid-but-non-conforming abandon refused naming
  keys, with the fixture bytes unchanged after both; **quarantine accept** — quarantine accepted
  and the archived bytes byte-identical to the pre-quarantine original.
* **Scratch containment**: the scratch workspace is created **inside the repository working tree**
  at `docs/scratch/checkpoint-verification/` (never `%TEMP%`, never outside the cwd — Constitution
  IV), the resolved path is asserted to be repo-root-relative **before the first write**, it is
  added to the freeze-scope declaration, and — because `.gitignore` carries no `docs/scratch/` rule
  today (`*.exe` already covers the built binary, the copied fixtures are not covered) — adding that
  ignore rule is owned by this unit. U10b inherits all of it. **Teardown does not run here** (PR
  #377 review cycle 4): U10b consumes this unit's quarantine archive, fixtures, and branch-built
  binary, so the workspace is handed over intact and teardown ownership moves to U10b. It stays
  classified `ActionRisk: destructive` (A4b) requiring
  operator approval (Constitution VII) at the point U10b performs it. If approval is not granted
  the directory is left in place and recorded as a cleanup follow-up.
* **Depends on**: U9b.

### U10b — Runtime verification of acceptance, restore, and the recovery sweep

* **Domain**: verification
* **Files**: none. Runs entirely inside the scratch workspace U10 created; no repository file
  changes.
* **Change**: none to product code. **The recovery sweep runs against a scratch mirror, never the
  live directory.** U10 requires every file under `.backlogit/checkpoints/` to be byte-unchanged,
  while the nine-file acknowledgement requires a sweep that "succeeds on every other file in that
  directory" — a real `resolve` against the conforming active checkpoints there. The two cannot
  both hold against the live corpus, so the sweep operates on a **copied mirror** inside the
  scratch workspace (`docs/scratch/checkpoint-verification/mirror/`). The nine enumerated legacy
  filenames keep their names in the mirror, so the discrimination assertion is unchanged while the
  live bytes stay read-only.
* **Rows** (3): **acceptance is not over-refused** — a conforming active fixture is accepted by
  abandon and a second conforming active fixture is accepted by resolve; **restore path** — the
  quarantine archive U10 row 3 produced, which is **valid-but-non-conforming** (row 1's `get`
  already reported `conforming: false`, which under U6b/U6c is only possible on a schema-valid
  document — a schema-invalid document is refused with `ErrCheckpointInvalid` before any
  conformance verdict), is recovered per entry point (b) of the single U9b procedure (rename the
  archived bytes *and* their `.disposition.json` sidecar aside to
  `<filename>.quarantined-<disposition_at>` first, then copy the preserved bytes back into the
  now-free active filename), hand-repaired per the classified entry point (a), and — with
  `checkpoint get` then reporting the active copy as both `valid: true` **and** `conforming: true`
  — resolves normally — proving quarantine is recoverable rather than terminal **without** ever
  leaving one filename present in both the active fixture directory and the archive directory,
  and with the renamed evidence pair still byte-identical afterwards. The valid-but-non-conforming
  fixture class is deliberate: entry point (a)'s schema-valid precondition holds so its classified
  moves converge; a schema-invalid archive would land on U9b's restore-abort rule and this row
  would fail; **recovery sweep discrimination** — against the mirror, a session-start recovery
  sweep refuses **exactly** the nine enumerated legacy filenames and succeeds on every other
  mirrored file.
* **Nine-file acknowledgement**: satisfied by row 3 against the mirror. U10's live-corpus SHA-256
  comparison must still pass after this unit runs.
* **Inherited inputs and teardown**: U10 hands this unit a **live** workspace — the branch-built
  binary, the copied fixtures, the mirror source, and the quarantine archive row 2 consumes.
  Confirm those inputs are present before row 1 rather than rebuilding them; a missing workspace
  blocks this unit on re-running U10, it does not license a hand-rebuild. **Teardown of
  `docs/scratch/checkpoint-verification/` is owned by this unit** and runs only after all three
  rows pass, still `ActionRisk: destructive` (A4b) requiring operator approval, and still skipped
  and recorded as a cleanup follow-up when approval is withheld.
* **Depends on**: U10 (scratch workspace, ignore rule, branch-built binary, and the quarantine
  archive row 2 consumes).

## Dependency Graph

```text
U1 ──▶ U2 ──┬──▶ U2b ──┬──▶ U2e ─────────────────────────▶ U9
            ├──▶ U2c ──┤
            │          ├──▶ U3 ──▶ U3b
            │          ├──▶ U4 ──▶ U5 ──▶ U5b
            │          └──▶ U6 ──┬──▶ U6b ──┬──▶ U6c ──┬──▶ U7b
            │                    │          └──▶ U8c   │
            │                    └──▶ U6d ─────────────┘
            └──▶ U2d ──▶ U2f ──┬──▶ U3b
                               └──▶ U4

U1 ──┬────────────────────▶ U7d
     │
U3b ─┤
U4 ──┼──▶ U7 ──┬──▶ U7d ──▶ U7c ──┐
U5 ──┘         └──▶ U8 ───────────┤
                                  ├──▶ U8b ──▶ U9 ──▶ U9b ──▶ U10 ──▶ U10b
U6c ──────────────────────────────┤
U7b ──────────────────────────────┤
U8c ──────────────────────────────┘
```

Edges declared, no cycles:

| Edge | Reason |
|---|---|
| U1 → U2 | Helper returns the typed error declared in U1. |
| U2 → U2b, U2c, U2d | All extend the same helper. |
| U2b, U2c → U2e | The nested duplicate rule extends U2b's recursion and reuses U2c's `duplicate:` reporting form. |
| U2e → U9 | The design doc's totality claim covers the completed predicate, nested duplicates included. |
| U2d → U2f | I1's executable form builds on U2d's reflection guards. |
| U2f → U3b, U4 | The audited rewrite allow-list must be pinned **before** either gate unit touches its call site, or the enumeration is written against a moving target. |
| U2c → U3, U4, U6 | Every gate calls the completed predicate, including the duplicate rule. |
| U3 → U3b | Conformance gate sits after the validity gate. |
| U6 → U3b | U3b's residual test asserts U6 flags the same file, so the discovery path must exist. |
| U4 → U5 | U5's paired table asserts abandon **refuses** the row quarantine accepts; that refusal is U4's. |
| U5 → U5b | State-dimension rows extend U5's table. |
| U6 → U6b | Both read surfaces must report the same field set. |
| U6 → U6d | The filter exemption extends the conformance branch U6 introduces. |
| U6b → U6c | The MCP handler projects U6b's result type; without it `valid: true` stays hardcoded. |
| U6b → U8c | The CLI handler projects the same result type; without it `newCheckpointGetCmd` keeps its hardcoded `valid: true`. |
| U6b, U6c → U7b | The read-surface descriptions promise U6b's new field as projected by U6c. |
| U6d → U7b | The `list_checkpoints` description states the filter exemption U6d implements; a published description must not promise unshipped behaviour. |
| U1, U3b, U4, U5 → U7 | MCP maps every sentinel those units emit. |
| U1, U7 → U7d | The handler routes on U1's `QuarantineIsRemedy` predicate into U7's response shape. |
| U7, U7d → U7c | The mutation-surface descriptions promise codes that only reach `resolve` once U7d routes them. |
| U7 → U8 | The CLI consumes the mapping layer. |
| U7b, U7c, U8, U6c, U8c → U8b | Parity table drives every completed surface, including both the MCP and the CLI `get` projections. |
| U6b, U8b → U9 | Design doc restates final behaviour. |
| U9 → U9b → U10 → U10b | Docs, then refusal verification, then acceptance and restore verification against the scratch workspace U10 creates. |

Suggested execution order: U1, U2, U2b, U2c, U2d, U2e, U2f, U3, U6, U6b, U6c, U6d, U7b, U3b, U4,
U5, U5b, U7, U7d, U7c, U8, U8c, U8b, U9, U9b, U10, U10b. U2b, U2c, and U2d are mutually independent
once U2 lands; U6 and U4 are mutually independent once U2c lands; U6d is independent of everything
after U6 but must land before U7b; U8c is independent of everything after U6b and only has to land
before U8b; U2e is independent of the gate units and only has to land before U9. **U3 and U5 are
not independent of U4/U6** — see the edge table.

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
| **U2f's gated-seam fallback withdrawn** (PR #377 review cycle 3) | Offering "enumeration test **or** exported `events.RewriteCheckpointFile` seam" inside one unit meant its real file set was either one new test file or five files spanning `internal/events`, `internal/core`, and two skill domains — so the unit's size could not be known before work started, and the larger branch breached both the three-file heuristic and width isolation. The enumeration is now the only mechanism; if it proves unimplementable, U2f halts as `blocked` and the seam is re-planned as its own units under a new ID rather than absorbed. |
| **MCP `get` keeps its validation-class refusal for schema-invalid documents** (PR #377 review cycle 3) | `handleGetCheckpoint` routes through `domainError`, which takes no filename and already maps `ErrCheckpointInvalid` to `validation_failed`. Making `get` emit `checkpoint_use_quarantine` would mean routing a **read** through a mutation-shaped error path, widening U6c into U7's file set and changing a shipped contract. The quarantine remedy is already discoverable from `list_checkpoints` (`needs_quarantine: true`), so the safety value is nil and the churn is real. Disposition codes stay on the mutation verbs. |

## Risks and Caveats

| Risk | Severity | Mitigation |
|---|---|---|
| `ResolveCheckpoint` becomes stricter and breaks an existing caller | Medium | Existing fixtures are built from a `CheckpointV1` struct (`internal/events/checkpoint_lifecycle_test.go:19-25`) and conform by construction. U3 adds explicit red-phase coverage for both refusals and re-asserts the conforming and idempotent paths. |
| **Agent session-start recovery hits nine refusals on day one** | **High** | This is *correct* behaviour, not a regression, and must not be read as one. The nine filenames are enumerated in Runtime Verification and Closure; the rollback trigger fires only on a refusal **outside** that set. U9b updates the agent-facing instruction file in the same PR so recovery guidance matches. |
| The quarantine widening is dropped and the deadlock ships | **High** | U5 is a first-class unit whose primary test asserts accept-by-quarantine and refuse-by-abandon **in the same table row**, so neither assertion can be deleted alone. |
| **`ResolveCheckpoint` refusal surfaces to agents as a 500** | **High** | `handleResolveCheckpoint` routes through `domainError`, which has no case for the two new sentinels and would fall to `InternalError`. U7 adds both cases as a safety net, U7d routes disposition-class refusals through `checkpointDispositionError` so they also carry `code` and `filename`, and both units assert the payload is not `"error":"internal"`. |
| **Nested `context` keys swept into refusal, regressing 146-F** | **High** | U2b's second scenario is a permanent regression guard asserting unmodeled `context` keys return nil. |
| Duplicate / fold-variant top-level keys pass conformance and are then collapsed | Medium | U2c makes `strings.EqualFold`-equal top-level keys non-conforming, reported as `duplicate:<key>`. |
| Conformance key set drifts from `CheckpointV1` | Medium | U2d asserts set equality against the create-boundary set plus the reserved keys, guarding the hand-written reserved literal. |
| A future change reintroduces a top-level preservation carrier | Low | U2d asserts `CheckpointV1` declares no `json:"-"` map carrier, anchored to the deliberation so the guard reads as "revisit the decision", not "never". |
| Widened quarantine increases traffic into `archive/checkpoints/`, where `CleanupCheckpoints` `os.Remove`s a colliding destination | Medium | `moveNoReplace` already refuses to overwrite on the quarantine path. The reverse direction is the real hazard: U9b's restore entry point must rename the archived bytes **and** their `.disposition.json` sidecar aside before copying anything back, so no filename is ever live in both directories and no quarantined evidence can be removed by a later sweep. U10b's restore row asserts that property. |
| The nine live legacy files are mutated during verification | Medium | U10 and U10b run against an in-tree scratch workspace only — U10b's recovery sweep uses a **copied mirror**, never the live directory. Live files are read for shape reference and never used as mutation targets. Every file under `.backlogit/checkpoints/` is hash-compared programmatically before and after. |
| Windows atomic-write regression | Low | No change to `atomicfile.WriteFileAtomic` or `syncWriteFileAtomic`; only additional pre-write gates. |
| CLI reference drift blocks the PR | Low | U9 regenerates `gen-docs` output and runs `backlogit docs lint` before handoff. |
| `CreateCheckpoint` same-second filename collision silently overwrites (adjacent, **out of scope**) | Medium | Surfaced during the entry-point audit (I1). Not fixed here and not stashed, to hold the bounded scope. Recorded in Plan Hardening as a named follow-up. |

## Constitution Check

| Principle | Verdict | Notes |
|---|---|---|
| I. Safety-First Go | **deviation (documented)** | All production changes are Go; no `unsafe`. New wraps use multi-`%w` so both sentinels resolve. **Deviation**: `AbandonCheckpoint` already wraps its validation failure with `%v` (`internal/core/checkpoint_disposition.go:~76-81`), losing `ErrCheckpointInvalid`. This plan does not fix that pre-existing wrap — it is recorded as a named follow-up rather than silently claimed as compliant. |
| II. Test-First Development (NON-NEGOTIABLE) | **pass** | Every code unit uses the two-step red posture declared at the head of Implementation Units: a declaration stub so the package **compiles**, then a harness that **fails on assertions**. Expected red is stated per unit. U2d and U5b are pure regression guards and declare their exemption explicitly rather than claiming a red phase they do not have. |
| III. Workspace Isolation and Security Boundaries | **pass** | No path handling changes. `ResolveDispositionTarget`, `ensurePathContained`, and `validateCheckpointFilename` are untouched. The new gates operate on already-read bytes. `Fields` carries key **paths** only, never values, so a refusal cannot leak checkpoint content. No secrets introduced. |
| IV. CLI Workspace Containment (NON-NEGOTIABLE) | **pass** | All edits are inside the repository tree. U10's scratch workspace is pinned to `docs/scratch/checkpoint-verification/` **inside** the working tree — never `%TEMP%`, never a sibling or parent — and the path is asserted to be repo-root-relative before any write. |
| V. Structured Observability | **deviation (documented)** | Refusals are typed and machine-readable: `unknown_fields` on MCP, named keys on CLI, `NeedsQuarantine` + `RemediationCommand` on list and get. The audit-before-mutation ordering is **preserved** (not strengthened — the ordering already existed; U4 only moves the new gate to sit ahead of it). **Deviation**: no new counter, log line, or telemetry event is emitted when a refusal occurs, so a spike in refusals is observable only through agent-visible errors. Accepted for this scope; recorded as a follow-up. |
| VI. Single Responsibility | **pass** | No new dependencies. The helper reuses `decodeTopLevelEntries`, `isFoldKeyIn`, `modeledJSONTagKeys`, and `unknownNestedProgressKeys` already present in `internal/events`. |
| VII. Destructive Command Approval (NON-NEGOTIABLE) | **deviation (documented)** | The change is net **anti**-destructive: it removes a silent data-destruction path. **Two deviations**: (a) the remedy this plan directs operators toward — quarantine — *moves the source file out of the live directory*, which is destructive from the perspective of a live `active` checkpoint; U9b therefore states it must be a deliberate operator decision. (b) the scratch teardown — owned by **U10b**, after U10 hands the workspace over intact — is a directory deletion; it is classified `ActionRisk: destructive`, requires operator approval, and is skipped (leaving the directory as a cleanup follow-up) if approval is withheld. |
| VIII. Explicit Safety Modes | **pass** | Work executes under **freeze-scope**. Declared boundary: `internal/errors/`, `internal/events/`, `internal/core/`, `internal/mcp/`, `internal/cli/`, `docs/design-docs/checkpoint-administrative-disposition.md`, `docs/cli-reference/backlogit_checkpoint_*.md`, `.github/instructions/backlogit.instructions.md`, `.autoharness/backlog-registry.yaml`, `docs/closure/`, and `docs/scratch/checkpoint-verification/`. The nine live checkpoint files are explicitly **outside** the mutation boundary. |
| IX. Git-Friendly Persistence | **pass** | Checkpoint JSON stays human-readable; `jsonutil.MarshalReadable` and the atomic-write helpers are unchanged. |
| X. Agent Context Efficiency | **pass** | Refusals carry structured field lists so an agent does not parse message text to learn which keys were rejected. U6b closes the `list` / `get` disagreement that would otherwise cost an agent an extra round trip and a wrong verb. |
| XI. Merge Commit History Preservation (NON-NEGOTIABLE) | **pass** | Ships through a merge commit. Squash and rebase merge are forbidden and must be verified before merge. |

### Documented deviations

| Principle | Deviation | Justification | Simpler alternative rejected |
|---|---|---|---|
| I. Safety-First Go | `AbandonCheckpoint`'s pre-existing `%v` validation wrap is left in place. | Fixing it changes an unrelated shipped error contract inside a bounded-scope plan and would need its own red phase and cross-surface assertions. | "Fix it while we're in the file" — rejected: it is a silent contract change to a governed path, invisible in this plan's tests, and belongs in its own unit. |
| V. Structured Observability | No refusal counter, log, or telemetry event. | The refusal is already agent-visible and typed; adding a telemetry surface pulls `internal/telemetry` into a freeze-scoped change and widens the blast radius past the defect. | "Emit a telemetry event per refusal" — rejected: nine known refusals on day one would immediately produce noise with no consumer defined. |
| VII. Destructive Approval | Quarantine — a source-file-moving operation — is the directed remedy. | Without widening quarantine the plan creates an unremediable deadlock (F3); a move to `archive/checkpoints/` preserves the bytes verbatim and is reversible by copy-back, which U10b proves. | "Refuse on all three verbs" — rejected: strands the file with no disposition path at all, which is strictly worse than a reversible move. |
| VII. Destructive Approval | U10b's scratch directory teardown is a deletion (moved from U10 in PR #377 review cycle 4 so U10b's inherited inputs survive). | Verification needs a disposable workspace; leaving it permanently pollutes the tree. | "Use `%TEMP%`" — rejected outright by Constitution IV (containment) and by the workspace's no-temp-directory rule. |

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
| U5, U5b | CLI `backlogit checkpoint quarantine`, MCP `backlogit_quarantine_checkpoint` | The same document is accepted, moved byte-identically into the archive, and given a disposition sidecar. A conforming `status:"resolved"` file is refused by both verbs with its documented pre-existing sentinels. |
| U6, U6b | CLI `backlogit checkpoint list` / `get`, MCP `backlogit_list_checkpoints` / `backlogit_get_checkpoint` | A non-conforming file reports `needs_quarantine: true` with a **PowerShell-runnable** remediation command on **both** read surfaces, and the file is unchanged after reading. |
| U8, U8b | CLI error output, cross-surface parity | Both refusals exit non-zero with actionable text, and CLI, MCP, and the `events` read layer reach the same classification from the same stored file. |
| U10 | Live workspace, read-only | Every live SHA-256 hash under `.backlogit/checkpoints/` is unchanged across the whole verification run. |
| U10b | Scratch mirror of `.backlogit/checkpoints/` | A session-start recovery sweep against the mirror refuses **exactly** the nine enumerated legacy filenames and succeeds on every other mirrored file. A quarantined file restores from the archive and then resolves. |

**The nine expected-refusal filenames** (enumerated so a correct refusal is never misread as a
regression — recorded from the live corpus inspection performed during deliberation; the exact list
is re-captured and pinned in the closure artifact before merge):

> All nine currently-schema-invalid files under `.backlogit/checkpoints/`. They are the complete set
> of live checkpoints that fail `ValidateCheckpoint` today, and they already fail `abandon` today
> (F1). The two conforming files are `checkpoint-20260822-064434.json` (resolved) and
> `checkpoint-20260822-212617.json` (active, stale `129-S`).

**Operational closure artifacts required before the work is absorbed**:

* **Healthy signal** — `checkpoint resolve` / `abandon` succeed on **conforming** checkpoints, and
  refusals occur **only** on filenames in the enumerated nine-file set. Refusals on that set are
  expected, correct, and are **not** a failure signal.
* **Failure signal** — any refusal of a conforming checkpoint; any refusal on a filename **outside**
  the enumerated set; any audit event appended for a refused disposition; any MCP refusal that
  surfaces as `"error":"internal"`.
* **Rollback trigger** — a refusal on any checkpoint filename outside the enumerated nine, **or**
  any conforming checkpoint failing to resolve. Revert the merge commit; there is no data migration
  to unwind. A refusal *within* the nine does not trigger rollback at any frequency.
* **Pre-merge acknowledgement** — the merging operator must confirm in the PR that day-one refusals
  on the nine legacy files are expected behaviour, so the post-merge observation window is not
  interpreted as an incident.
* **Ownership and validation window** — one Ship session post-merge, verified by running Stage and
  Ship session-start recovery against the live workspace.
* **Blocked-path handling** — if agent recovery is blocked by a refusal on one of the nine, the
  remedy is the U9b hand-repair procedure or `checkpoint quarantine`, **not** a revert.
* **Follow-ups recorded (not in scope)**: dispose of the nine live legacy checkpoint files and the
  stale active `129-S` checkpoint `checkpoint-20260822-212617.json` as workspace hygiene;
  `AbandonCheckpoint`'s `%v` validation wrap; refusal observability; the `CreateCheckpoint`
  same-second filename collision.

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
| `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md` | The direct precedent naming this exact preserve-vs-refuse fork. Its Guidance section drove the **body-preserving hand-repair** procedure added to U9b, so quarantine is not the only escape from a non-conforming checkpoint an operator still wants. |
| `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` | Rule 3 forced U8b: a cross-surface guard must exercise CLI **and** MCP **from the same stored state**, not from independent fixtures. Rule 1 forced the registry-drift re-run, split across U7b (two read-surface descriptions) and U7c (three mutation-surface descriptions). |
| `docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md` | Forced the `unknown_fields` array contract in U7: no `omitempty`, and the empty case asserted with a `.([]any)` type assertion. |
| `docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md` and `docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md` | Forced the fresh-binary requirement in U10: the pinned repo-root `backlogit.exe` predates this work and must not be used to verify it. |
| `docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md` | Already reflected: enforce in the mutation seam, reflection-derive the key set. |
| `.github/instructions/strict-safety.instructions.md` | Risky actions expressed as `ProposedAction` / `ActionRisk` / `ActionResult` below. |
| `.github/instructions/constitution.instructions.md`, `.github/instructions/circuit-breaker.instructions.md` | Constitution Check above; stop conditions below. |
| `.github/instructions/backlogit.instructions.md` | Identified as an **artifact to update** (R10, U9b), not merely consulted — its Lifecycle Hygiene Protocol currently teaches every agent that `resolve` is infallible and that the abandon/quarantine split is validity-only. |

### Entry-point completeness audit (protected invariant I1)

**I1 — Every code path that rewrites an existing checkpoint file in place must be gated, not just
the two named in the stash text.** Audit of every checkpoint write site in `internal/`:

| Write site | Kind | Verdict |
|---|---|---|
| `internal/events/checkpoint_lifecycle.go:178` (`ResolveCheckpoint`) | in-place rewrite of an existing file | **In scope — U3.** Currently ungated. |
| `internal/core/checkpoint_disposition.go:105` (`AbandonCheckpoint`) | in-place rewrite of an existing file | **In scope — U4.** Parse + validate gated; conformance gate added. |
| `internal/events/memory.go:106` (`CreateCheckpoint`, V1 branch) | new file | **Already gated** by `checkClosedSchemaNamespace` (146.011-T/U4). No change. |
| `internal/events/memory.go:112` (`CreateCheckpoint`, legacy branch) | new file, verbatim bytes | **Deliberately unchanged.** This is the legitimate origin of arbitrary top-level keys on disk; making it strict was rejected in the source document. |
| `internal/events/checkpoint_lifecycle.go:242` (`CleanupCheckpoints`) | `os.Rename` — verbatim move | **Correct by construction.** Never parses or re-marshals, so it cannot drop keys. No change. |
| `internal/core/checkpoint_disposition.go` `moveNoReplace` (`QuarantineCheckpoint`) | verbatim move | **Correct by construction.** U5 widens *which* files reach it, not how it writes. |
| `internal/telemetry/checkpoint.go:94` | different artifact (telemetry harvest cursor), different schema, not under `.backlogit/checkpoints/` | **Out of scope.** Named here so the audit is total and a reviewer does not read its absence as an omission. |

**Making I1 executable.** A table in a plan document decays. **U2f** lands a
**write-site enumeration test**: it walks the sources of **both** `internal/events` and
`internal/core` for calls to
`syncWriteFileAtomic` / `atomicfile.WriteFileAtomic` / `os.WriteFile` whose target resolves under
the checkpoint directory and asserts the resulting call-site set equals the enumerated allow-list
above. A new in-place rewrite path added later fails that test rather than silently joining the
ungated set. `internal/core` is in scope because U4's write site lives there. The previously
offered "or a single exported `events.RewriteCheckpointFile` seam" fallback is **withdrawn**: see
Decisions and Rationale. If the enumeration cannot be implemented reliably, U2f halts as `blocked`
and the seam is re-planned as its own units rather than absorbed. A comment does not satisfy I1.
The mechanism decision is taken in U2f, which is why U2f precedes U3b and U4 in the dependency
graph and is not folded into U2d's schema-reflection guards.

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
9. mutate + `syncWriteFileAtomic`

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
which is a separate decision requiring its own deliberation. U5b pins the row so the exclusion is
tested rather than assumed, and U9's design-doc rewrite states the scope qualifier so a future
reader is not told the classification is total when it is total only over `active`.

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
| A4 | Run disposition verbs during runtime verification | `docs/scratch/checkpoint-verification/` only | local file mutation | moderate | not required — in-tree scratch only, live corpus hash-guarded | discard the scratch contents | planned |
| A4b | Tear down the scratch verification workspace after closure (**owned by U10b**, only after its three rows pass — U10 hands the workspace over intact) | `docs/scratch/checkpoint-verification/` | directory deletion | **destructive** | **required** (Constitution VII) | none needed — contents are reproducible from the plan | planned |
| A5 | Mutate the nine live legacy checkpoints or the stale `129-S` checkpoint | `.backlogit/checkpoints/` | destructive, irreversible | **destructive** | **FORBIDDEN in this work.** Out of scope; requires explicit operator approval in a separate unit of work. | n/a | **abandoned** |

### Deepened runtime verification (U10, U10b)

* **Environment precheck** — build a fresh binary from the branch HEAD
  (`go build -o <scratch>/backlogit-verify.exe ./cmd/backlogit`) and confirm `version` reports the
  branch commit. **Do not** use the repo-root pinned `backlogit.exe`: it predates this work, so a
  green run against it would prove nothing (self-hosted version-skew learning).
* **Scratch workspace** — create the verification workspace *inside* the repository working tree at
  `docs/scratch/checkpoint-verification/` (Constitution IV; never `%TEMP%`), assert the resolved
  path is repo-root-relative before the first write, add the directory to the freeze-scope
  declaration and to `.gitignore` if it is not already covered, seed it with byte-copies of the
  legacy document shapes, and confirm `.backlogit/checkpoints/` is not the target directory before
  running any mutating verb.
* **Live-corpus guard** — record `Get-FileHash` for all files under `.backlogit/checkpoints/`
  **before** and **after** the verification run and assert programmatically that every hash is
  identical (R6, A5). A visual comparison is not sufficient.
* **Target scenarios** — the three rows of U10 (read verdict, refusal, quarantine accept) plus the
  three rows of U10b (acceptance, restore, recovery-sweep discrimination), each asserting the
  fixture's SHA before and after for refusal cases. The **restore path** row in U10b is mandatory:
  copy a quarantined file back from the archive per the U9b entry point (b) rename-aside rule,
  hand-repair it per the classified U9b entry point (a), and confirm it then
  resolves. Without this row the plan asserts quarantine is a remedy without ever demonstrating
  it is recoverable.
* **Mirror, not live corpus** — U10b's recovery-sweep row runs against
  `docs/scratch/checkpoint-verification/mirror/`, a byte-copy of `.backlogit/checkpoints/`. The
  sweep needs successful `resolve` calls on the conforming files to prove discrimination, and those
  succeed by rewriting; running it against the live directory would contradict the live-corpus
  guard above. Mirrored filenames are preserved so the nine-name assertion still means what it
  says.
* **Blocked-path handling** — if any refusal case instead succeeds and rewrites the file, halt
  immediately, do not proceed to closure, and treat it as a red-phase failure of the owning unit.
  Conversely, a refusal on one of the nine enumerated legacy filenames is **expected** and must not
  be treated as a blocked path.
* **Teardown** — owned by **U10b** and performed only after its three rows pass; U10 leaves the
  workspace standing because U10b consumes its archive, fixtures, and binary. Classified
  `ActionRisk: destructive` (A4b) and executed only with operator
  approval. If approval is withheld, leave the directory in place and record it as a cleanup
  follow-up rather than deleting it unilaterally.

### Deepened operational closure

* **Monitoring signal (no metrics backend — manual observation)** — after merge, run one Stage and
  one Ship session-start recovery against the live workspace and confirm neither is blocked by a
  **false** refusal. Refusals on the nine enumerated legacy filenames are expected and healthy.
  Record the outcome in the closure artifact.
* **Rollback trigger** — a refusal on any checkpoint filename **outside** the enumerated nine, or
  any conforming checkpoint refused by `resolve`/`abandon`. Threshold: **one** occurrence. A
  refusal *within* the nine does not trigger rollback at any frequency; treating it as a trigger
  would revert correct behaviour on day one.
* **Rollback procedure** — revert the merge commit. No data migration, no schema change, no
  on-disk format change, so revert is complete and sufficient. Files quarantined between merge and
  revert stay in the archive directory with their sidecars and remain recoverable by copy-back.
* **Owner and validation window** — the Ship session that merges the work; one session
  post-merge.
* **Human checkpoint** — the P-014 merge approval is the only operator checkpoint, and the merging
  operator must acknowledge the nine expected day-one refusals in the PR. There is no partial
  rollout, feature flag, or external dependency. Scratch teardown (A4b) is a second, separate
  approval.

### Stop conditions for the implementing session

Per `.github/instructions/circuit-breaker.instructions.md`: 5 build/test fix attempts per unit,
3 on the same recurring error, 3 review-fix cycles, 5 fix-CI cycles. If U5's **scoped** totality
assertion — disjoint and total over `status: "active"` — cannot be made to pass, **halt** rather
than weakening it: a green suite with an `active`-state deadlock shipped is worse than a blocked
task. Row 3 of the I3 table (conforming + `resolved` → refused by both) is **not** covered by that
stop condition; it is pre-existing, out of scope, and U5b asserts it as-is.

### Unresolved operator decisions

None block execution. Two items are carried forward as declared out-of-scope follow-ups requiring
their own authorization: disposing the nine live legacy checkpoints plus the stale `129-S`
checkpoint (A5), and the `CreateCheckpoint` same-second filename-collision overwrite surfaced by
the entry-point audit.

## Plan Review

dispatch_mode: multi-agent-dispatch

decision: PASS

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
`ValidationErr` append rather than overwrite, PowerShell-safe `RemediationCommand`, the
untagged-exported-field escape hatch in `modeledJSONTagKeys` (U2d test 3), the unquarantine/restore
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
| `PRRT_kwDORzozKM6b18IM` (`147.018-T`) | U9b's repair entry point (a) told operators to "move unmodeled keys into `context`" without classifying which offenders that actually works for. `duplicate:progress.<key>` cannot be repaired that way at all, and a blind move can produce a new duplicate. | Entry point (a) gains a four-row **offender classification table** (`<key>`; `duplicate:<key>` where one side is modeled; `duplicate:<key>` where neither is; `duplicate:progress.<key>`) plus a **termination rule** — re-run the conformance check after each repair, and stop after one round-trip. |
| `PRRT_kwDORzozKM6b18Ip` (`147.019-T`) | U10 required every file under `.backlogit/checkpoints/` to be byte-unchanged **and** a recovery sweep that "succeeds on every other file" in that same directory. Success means a rewrite; the two requirements were mutually exclusive. | The recovery sweep moves to a **copied mirror** at `docs/scratch/checkpoint-verification/mirror/`. The live directory stays hash-guarded and read-only. Mirrored filenames are preserved so the nine-name assertion is unchanged. |
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

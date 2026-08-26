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
| R3 | `ResolveCheckpoint` gains a `ValidateCheckpoint` gate and the same conformance check | source doc, Decided behaviour §3 | U3 (validity gate and its verb-facing wrap); U14 delivers the conformance refusal through the guarded seam, with U3c as its failing harness and U3b as the verb-level contract pin |
| R4 | `QuarantineCheckpoint` widens malformed classification so the verb pair stays total over its scoped population | source doc, Decided behaviour §4 | U5 |
| R5 | No preservation carrier is added to `CheckpointV1` **for this scope** (decision-anchored, not a permanent ban) | source doc, Decided behaviour §5 | negative requirement — guarded in U2 |
| R6 | The nine live legacy files are left untouched by this work | source doc, Decided behaviour §6 | negative requirement — asserted in U10 (live-corpus hash guard) and U10b (mirror, not live, for the sweep) |
| R7 | Typed, machine-readable refusal naming the offending keys, with one canonical "quarantine is the remedy" predicate, one bounded **raw** machine projection, and one separate human-facing rendering | plan-originated (source doc Unresolved Q1); bounding added in cycle 15, isolated in cycle 16, split machine/human in cycle 17 | U1, U1b, U1c, U7, U7d, U7e, U8 |
| R8 | Every checkpoint **read** surface agrees with the mutation verbs about which files are rewrite-safe, and exposes the offending members atomically per file | plan-originated (source doc Unresolved Q3, widened by plan review; offender projection added in cycle 16) | U6, U6b, U6c, U6d, U8c, U15 |
| R9 | Human-facing design doc restates the verb pair as total over its scoped population | plan-originated (source doc Option B cons) | U9 |
| R10 | Agent-facing instruction surfaces teach the new `resolve` failure mode and the quarantine remedy, without publishing an unsafe repair runbook and without publishing an unbound executable command | plan-originated (plan review; narrowed in cycle 16, executable text removed in cycle 17) | U9b |
| R11 | Every in-place checkpoint rewrite routes through **one guarded seam** that requires parse, validate, and conformance to succeed before any marshal or atomic replace | cycle-16 gate finding H8 | U11, U12, U13, U14, U14b, U3c, U2f |
| R12 | Remediation is published as **structured, non-executable intent** by `core`/`events` on **every** quarantine-candidate branch; only the CLI boundary renders an operator command, bound to the canonical workspace and to the A4c approval / preimage / no-clobber contract | cycle-16 gate finding H1; totality over all three branches added in cycle 20 | U1d, U6, U6e, U6b, U6c, U16, U9b |
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
partitions, numbered to reflect the dependency structure rather than to impose an ordering rule of
their own: the binding constraint on any unit is its own declared dependencies (the Dependency
Graph and the per-task prerequisite table), and the partition number is a descriptive label that
follows from those edges, not an independent execution gate. In practice this means no unit in
partition *n* begins before every unit in partitions `1..n-1` that it actually depends on has
landed; it does **not** require every other unit in `1..n-1` to have landed first. Each partition is
independently reviewable, and the partition a unit belongs to is stated in that unit's section.

| # | Partition | Units | Owns |
|---|---|---|---|
| 1 | Foundation diagnostics and conformance | U1, U1b, U1c, U1d, U2, U2b, U2c, U2d, U2e, U2g, U2h | The typed refusal carrier, the bounded machine projection, the human rendering, the structured remediation intent, and the read-boundary conformance predicate |
| 2 | Guarded rewrite seam | U11, U12, U13, U14, U14b, U2f, U3c | One centralized, precondition-guarded rewrite seam for every in-place checkpoint rewrite, one caller migration per verb, the resolve-verb conformance harness the resolve migration turns green, plus a supplemental caller-set regression guard |
| 3 | Declarations and genuine RED harness order | U15, U8b | The read-result carrier declaration and the cross-surface parity harness, both landing **before** any behavioural implementation in partition 4 |
| 4 | Implementation plus MCP/CLI/instruction contracts | U3, U3b, U4, U17, U5, U6, U6d, U6e, U6b, U6c, U7, U7d, U7e, U7b, U7c, U8, U16, U8c, U9, U9b | The per-verb gates and sentinel contracts, the read-surface projections and their total remediation-intent population, the MCP and CLI surfaces, and the agent-facing instruction delta |
| 5 | Runtime verification and closure | U10, U10b, U10c | Runtime proof of refusal, acceptance, evidence integrity, cross-surface context-duplicate behaviour, and the abandoned-resolve handler mapping |

**Test lifecycle (two steps, normative — cycle-31 rewrite).** A test file that references an
undeclared symbol does not compile, and a build error is **not** a red assertion. Development
Workflow #1 requires a *compiling but failing* harness, and P-004's precondition is expected
failure markers **for every test function** the harness scaffolds. This plan carries exactly one
test lifecycle, and every unit runs it:

1. **Red harness step (always first)** — land *only* the test functions that fail. Every function
   named `TestU<unit>_<Descriptor>` must **compile against the pre-delta tree** and **fail on an
   assertion**. Where the unit's delta introduces a symbol the harness would otherwise have to
   name, the harness takes the **source-shape** form: it parses the named production file with
   `go/parser` and asserts the declared shape through `go/ast`, so it names no undeclared
   identifier. `harness-ready` is applied only when the unit's red-verification command reports a
   failure for **every** function it selects. A test that passes MUST NOT be committed in this
   step.
2. **Green step** — land the production delta (the declaration **and** its behaviour together),
   turn every red function green, and — in the *same* commit as the implementation — add the
   already-green regression guards as `TestU<unit>Guard_<Descriptor>`. Guards are green validation
   of the implementation, never harness scaffolding.

**The cycle-20 "declaration step" is withdrawn (cycle-31).** No production stub — not a sentinel
`var`, not a `return nil` body, not an "empty but correctly identified" map, and above all not a
body carrying currently-shipped read/mutate/write behaviour — may be committed **ahead of** the
unit's own harness. PR #377 review held that such a stub is production surface landing with no
observed failing test, which is a carve-out from NON-NEGOTIABLE Constitution Principle II. The
source-shape harness makes the stub unnecessary: it compiles before the declaration exists and
fails on an assertion, so the correct order is always **harness first, declaration second**.

**Behaviour beyond the declared shape needs its own prior red.** A unit may land a declaration and
its behaviour in one green step only when the behaviour is either (i) turned green by that same
commit and retained as a `TestU<unit>Guard_` regression guard, or (ii) owned by a named later unit
whose behaviour harness lands in a strictly later wave against the landed declaration. A **seam or
declaration whose body would absorb real behaviour** MUST be split into declaration → behaviour
harness → implementation, each gated by a harness that precedes it — the **U11 → U12 → U13** shape.
There is no stub loophole in either direction.

A unit is not red until its **exact** red-verification command prints assertion failures rather
than a build error. The placeholder `go test ./<pkg>` phrasing used through cycle 15 was not
executable — it named no package, no test selector, and no cache-defeating flag, so an implementer
could report "red" from a stale cached result or from an unrelated failing test in the same
package. Cycle 16 replaced it with the per-unit table below.

**Units that scaffold no harness (cycle-20, narrowed cycle-29).** A unit whose entire delta is a
document or a runtime-verification run has no missing behaviour for a Go test to fail against.
Such a unit scaffolds **zero** harness test functions, so P-002/P-004's universal quantifier over
scaffolded harness functions holds vacuously. It is recorded as `harness-exempt` with its class —
`docs-only`, `verification-only`, or `covered-by:<unit>` when a prior unit
already owns the failing harness that this unit turns green — and it MUST name the downstream or
owning unit whose red harness fails for the behaviour involved. Fabricating a failing assertion for
such a unit is forbidden: a test that can only fail because a symbol does not yet exist is a build
error, and a test that passes the moment the declared shape lands was never red.

**Declarations are NOT in that set (cycle-29).** Cycle 20 extended the exemption to a
`declaration-only` class covering U1d and U15. PR #377 review (`workflow-policies.md:75`) found
that this let observable production surface — a serialized `remediation_intent` field on
`CheckpointSummary`, an exported `GetCheckpointResult` wrapper — land with no observed failing
test, which is a carve-out from NON-NEGOTIABLE Constitution Principle II. The class is **withdrawn**
from the global policy, and U1d and U15 are **normal harness-required units**.

The dilemma cycle 20 was resolving is genuine but has a third answer: a **source-shape harness**.
It parses the named production file with `go/parser` and asserts the declared shape through
`go/ast`, so it references no undeclared identifier. It therefore **compiles** against the
pre-declaration tree (satisfying P-002's harness-compiles postcondition) and **fails on an
assertion** rather than on a build error (satisfying P-004's red phase), and it turns green only
when the declaration lands. It is neither the build error P-004 rejects nor the tautology cycle 20
rejected. Both U1d's `TestU1d_*` and U15's `TestU15_*` harnesses were executed against worktree
HEAD `6a822ceb` and observed compiling-and-red. The cycle-8 rule "every unit still needs at least
one assertion that fails" is therefore **reinstated for declaration units** in this corrected form;
cycle 20's withdrawal of it stands only for docs and verification units.

Test-first is preserved exactly where it is
load-bearing: **every behaviour-changing implementation unit still has a failing harness that
lands before it**, and the harness-exempt classes are enumerated in the Documented deviations
section rather than left to implementer judgement.

**Harness naming contract (mandatory).** Every harness test function a unit lands MUST be named
`TestU<unit>_<Descriptor>` — `TestU2g_ExactDuplicateContextMember`, `TestU7e_AbandonedResolveMaps`,
and so on. The unit token is the plan's unit label verbatim, lower-case suffix included. This is
what makes each `-run` regex below exact and drift-proof: `^TestU2_` cannot match `TestU2b_`,
`TestU2c_`, or `TestU2g_`, so a unit's red gate can never be satisfied by a sibling unit's
failures. `-count=1` is mandatory on every invocation: without it a cached `ok` line from a prior
run is indistinguishable from a genuine pass.

Green-step guards use the parallel token `TestU<unit>Guard_<Descriptor>` —
`TestU2Guard_ConformingDocumentAccepted`, `TestU14Guard_ResolveAcceptPathUnchanged`. The mandatory
`_` after `Guard` keeps every guard selector exact, and no unit label ends in `Guard`, so
`^TestU2Guard_` cannot match `TestU2bGuard_` or `TestU2gGuard_`. A red selector `^TestU<unit>_`
can never match a guard, because the character following the unit token in a guard name is `G`,
not `_`. That disjointness is what makes the P-004 gate mechanically checkable: everything the red
selector matches must fail, and nothing the guard selector matches may exist in the harness commit.

| Unit | Task | Red-verification command |
|---|---|---|
| U1 | `147.001-T` | `go test -count=1 -run '^TestU1_' ./internal/errors` |
| U1b | `147.030-T` | `go test -count=1 -run '^TestU1b_' ./internal/errors` |
| U1c | `147.031-T` | `go test -count=1 -run '^TestU1c_' ./internal/errors` |
| U1d | `147.032-T` | `go test -count=1 -run '^TestU1d_' ./internal/events` (source-shape harness; cycle-29 — no longer exempt) |
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
| U14 | `147.037-T` | `go test -count=1 -run '^TestU14_' ./internal/events` |
| U14b | `147.044-T` | `go test -count=1 -run '^TestU14b_' ./internal/core` |
| U15 | `147.038-T` | `go test -count=1 -run '^TestU15_' ./internal/events` (source-shape harness; cycle-29 — no longer exempt) |
| U3 | `147.006-T` | `go test -count=1 -run '^TestU3_' ./internal/events` |
| U3c | `147.042-T` | `go test -count=1 -run '^TestU3c_' ./internal/events` |
| U3b | `147.007-T` | harness-exempt (verification-only); green-step guards `go test -count=1 -run '^TestU3bGuard_' ./internal/events` |
| U4 | `147.008-T` | `go test -count=1 -run '^TestU4_' ./internal/core` |
| U17 | `147.040-T` | `go test -count=1 -run '^TestU17_' ./internal/core` |
| U5 | `147.009-T` | `go test -count=1 -run '^TestU5_' ./internal/core` |
| U6 | `147.011-T` | `go test -count=1 -run '^TestU6_' ./internal/events` |
| U6b | `147.012-T` | `go test -count=1 -run '^TestU6b_' ./internal/events` |
| U6c | `147.022-T` | `go test -count=1 -run '^TestU6c_' ./internal/mcp` |
| U6d | `147.023-T` | `go test -count=1 -run '^TestU6d_' ./internal/events` |
| U6e | `147.043-T` | `go test -count=1 -run '^TestU6e_' ./internal/events` |
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
`TestU1c_`, `TestU1d_`, `TestU11_`, `TestU14_`, `TestU15_`, `TestU16_`, or `TestU17_`; `^TestU14_`
cannot match `TestU14b_`; `^TestU2_`
cannot match `TestU2f_`, `TestU2g_`, or `TestU2h_`; `^TestU10_` cannot match `TestU10b_` or
`TestU10c_`. Every new cycle-17 label was chosen against this rule.

This table is the single source of truth for red verification. Each unit's **Expected red** line
names *which* functions the harness step scaffolds; this table names *how* the implementer observes
them. Tasks reference the table by unit rather than restating the command, so the two cannot drift.
Any unit's guard selector is the same command with `^TestU<unit>Guard_` substituted for
`^TestU<unit>_`; it is run in the green step and never in the harness step.

**Green-step regression guards (cycle-20 relocation, cycle-31 restatement).** A scenario that
asserts already-shipped behaviour, or that a unit's own green step turns true the moment the delta
lands, passes from the moment it lands. Through cycle 19 those scenarios were committed **inside**
the harness step and labelled "declared regression guards", with each unit's **Expected red** line
naming which of its cases failed and which did not. That arrangement cannot satisfy P-004, whose
precondition is expected failure markers *for every test function*, and PR #377's cycle-20 Copilot
review raised it as a P1 on fourteen tasks. It is withdrawn. Guards are **not part of the red
harness**. Each unit's **Expected red** line now names only the functions the harness step
scaffolds — every one of which fails — and a separate **Green-step guards** line names the
functions that land with the implementation. A guard remains a first-class obligation: omitting one
is a coverage regression. It is simply committed where it is honest, which is the green step and
never the harness commit. Splitting a unit's red selector to exclude an already-green function (the
cycle-17 device used on U2g and U2h) is also withdrawn: a guard that is excluded from the red
selector is still committed in the harness commit, and P-002/P-004 gate the harness commit, not the
selector.

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
* **Expected red**: **3 source-shape harness functions (cycle-31).** The harness lands **first** and
  parses `internal/errors/checkpoint_errors.go` with `go/parser`, asserting through `go/ast` that
  `ErrCheckpointNonConforming`, `CheckpointNonConformingError{Fields []string}` with
  `Error()`/`Unwrap()`, and `QuarantineIsRemedy` are declared. All three compile against the
  pre-declaration tree and fail on assertions naming the absent shape. No declaration stub is
  committed ahead of them.
* **Green-step guards** (3, `TestU1Guard_<Descriptor>`): the three behavioural scenarios above —
  `errors.Is`/`errors.As` recovery through the wrap, the sorted joined `Error()` rendering, and the
  `QuarantineIsRemedy` truth table. They land with the implementation.

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
* **Expected red**: **3 source-shape harness functions (cycle-31).** The harness lands **first** and
  parses `internal/errors/checkpoint_errors.go` with `go/parser`, asserting through `go/ast` that
  `BoundedFieldPathSet` is declared with `Paths`, `Truncated`, `OmittedPaths`, and `TruncatedPaths`
  carrying the exact JSON tags with no `omitempty`, and that `BoundedFieldPaths` is declared on
  `*CheckpointNonConformingError` with the stated signature. All three compile against the
  pre-declaration tree and fail on assertions naming the absent shape. No declaration stub is
  committed ahead of them.
* **Green-step guards** (3, `TestU1bGuard_<Descriptor>`): the three cap scenarios above — the
  under-cap verbatim round-trip, the 21-path truncation-metadata case, and the multi-byte
  rune-boundary case. They land with the implementation.
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
* **Expected red**: **3 source-shape harness functions (cycle-31).** The harness lands **first** and
  parses `internal/errors/checkpoint_errors.go` with `go/parser`, asserting through `go/ast` that
  `FieldPathsForDisplay` is declared on `*CheckpointNonConformingError` with the stated signature
  and that `Error()` delegates to it. All three compile against the pre-declaration tree and fail
  on assertions naming the absent shape. No `return ""` stub is committed ahead of them.
* **Green-step guards** (3, `TestU1cGuard_<Descriptor>`): the three rendering scenarios above — the
  quoted-path join, the truncation clause, and the control-byte escaping. They land with the
  implementation.
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
* **Expected red**: **3 source-shape harness functions (cycle-29).** This unit is **not**
  `harness-exempt`; the `declaration-only` class it carried from cycle 20 was withdrawn after PR
  #377 review found it admitted observable production surface — a serialized `remediation_intent`
  field — with no observed failing test, a carve-out from NON-NEGOTIABLE Principle II. Cycle 19's
  "cases 1 and 2 fail (the type does not exist until the declaration step)" did describe a **build
  error**, which P-004 rejects, and the value-marshalling assertions would indeed pass the instant
  the shape landed. The resolution is neither of those: a **source-shape** harness in
  `internal/events/checkpoint_remediation_test.go` parses `checkpoint_schema.go` with `go/parser`
  and asserts the declared shape via `go/ast`, referencing no undeclared identifier. It compiles
  against the pre-declaration tree and fails on assertions —
  `TestU1d_RemediationIntentCarrierDeclared` ("RemediationIntent struct is not declared in
  checkpoint_schema.go"), `TestU1d_CheckpointSummaryCarriesIntentField` ("CheckpointSummary has no
  field tagged json:\"remediation_intent\""), and   `TestU1d_RemediationIntentHoldsNoShellText`. **Verified at worktree HEAD `6a822ceb`**:
  `go vet ./internal/events` exits 0 and
  `go test -run='^$' -count=1 ./internal/events` reports `[no tests to run]` (compiles), while
  `go test -count=1 -v -run '^TestU1d_' ./internal/events` exits 1 with assertion failures, not
  build errors.
* **Red-verification command**: `go test -count=1 -run '^TestU1d_' ./internal/events`.
* **Green-step guards**: none separate. The three harness functions above are the guards — they go
  red first and green when the declaration lands, so the value-marshalling behaviour they express
  is still asserted, now with a genuine red phase in front of it.
* **Where the RED for this carrier lives**: the behaviour that populates the carrier is owned by
  **U6** (`147.011-T`, the non-conforming branch) and **U6e** (`147.043-T`, the parse-failure and
  schema-invalid branches); its projection is owned by **U6c** and its rendering by **U16**. Each
  of those units carries a failing harness that lands before its implementation.
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
* **Expected red** (1 harness function): `TestU2_UnknownTopLevelKeysRefused` — a source-shape
  function that lands **first** and fails against the pre-delta tree, where
  `internal/events/checkpoint_conformance.go` does not yet declare
  `CheckConformingTopLevelNamespace`. No `return nil` stub is committed ahead of it (cycle-31).
* **Green-step guards** (3, landing with the implementation as `TestU2Guard_<Descriptor>`): case 1
  (conforming V1 → nil) guards the trivial-conforming boundary condition; case 2 (two unknown
  top-level keys → typed error with both keys sorted) is the unit's behavioural obligation, turned
  green by this same commit; case 3 (reserved-key admission) guards the read-boundary admission of
  the `disposition*` namespace, which is a shipped behaviour of `checkpointV1ReservedKeys` and not
  introduced here. None is committed in the harness step (cycle-20; cycle-19 committed them inside
  it and the Copilot review raised the resulting green-in-harness state as a P1).
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
* **Expected red** (1 harness function): `TestU2b_UnknownNestedProgressKeyRefused` — case 1 fails.
* **Green-step guards** (2, `TestU2bGuard_<Descriptor>`): case 2 (unmodeled `context` keys return
  `nil`) is the permanent 146-F open-namespace guard, and case 3 (non-object `progress` returns
  `nil`) passes before the recursion lands because the U2 top-level check never descends into
  `progress` at all. Both land with the implementation, not in the harness commit.
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
* **Expected red** (2 harness functions): cases 1 and 2 fail.
* **Green-step guards** (1, `TestU2cGuard_<Descriptor>`): case 3 — a document with one occurrence
  of every modeled key remains conforming — passes against U2's landed predicate and lands with the
  implementation.
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
* **Expected red** (2 harness functions): cases 1 and 2 fail.
* **Green-step guards** (1, `TestU2eGuard_<Descriptor>`): case 3 — a `progress` object with one
  occurrence of each key stays conforming **and** the create boundary's verdict on the same bytes
  is unchanged — lands with the implementation.
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
* **Two-step lifecycle (cycle-31)**: (a) **Red harness step lands first** — a source-shape function
  parses `internal/events/checkpoint_conformance.go` with `go/parser` and asserts through `go/ast`
  that `checkpointV1AllTopLevelKeys` is declared from
  `modeledJSONTagKeys(reflect.TypeOf(CheckpointV1{}))` and that `CheckConformingTopLevelNamespace`
  consults it rather than U2's inline two-set check. It compiles against the pre-delta tree and
  fails on those assertions. (b) **Green step** lands the derivation, refactors
  `CheckConformingTopLevelNamespace` onto the single derived set, and lands the set-equality case
  and cases 2 and 3 as `TestU2dGuard_` functions. The cycle-8 "declaration step lands an empty
  `map[string]struct{}{}` stub" posture is **withdrawn**: no stub precedes the harness.
* **Tests** (3): `checkpointV1AllTopLevelKeys` equals `checkpointV1TopLevelKeys ∪
  checkpointV1ReservedKeys` — set equality against the hand-written literal, guarding drift in the
  reserved-key set rather than the reflected field set (the narrower claim is the accurate one);
  absence of a `json:"-"` map carrier on `CheckpointV1`; **every exported field of `CheckpointV1`
  carries a non-empty `json:"..."` tag**. The third closes a latent escape hatch: `modeledJSONTagKeys`
  skips untagged exported fields, so a future field added without a tag would appear in the
  derived set only when the escape hatch is closed.
* **Expected red** (1 harness function): the source-shape function fails against the pre-delta tree
  — `checkpointV1AllTopLevelKeys` is not declared and `CheckConformingTopLevelNamespace` still
  carries the inline two-set check.
* **Green-step guards** (3, `TestU2dGuard_<Descriptor>`): the set-equality assertion against the
  hand-written literal, plus cases 2 and 3 — `CheckpointV1` already declares no `json:"-"` carrier
  and every exported field is already tagged, so both assertions expect a state that already holds.
  They land with the implementation. Cycle 19's "P-004 is satisfied by case 1 as the single red
  assertion" framing is withdrawn: P-004 quantifies over every scaffolded harness function, not
  over the unit.
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

  **Harness-first, declaration/behaviour split (cycle-31).** The **source-shape** harness lands
  first: `TestU11_RewriteSeamDeclared` parses `internal/events/checkpoint_rewrite.go` with
  `go/parser` and asserts through `go/ast` that `RewriteCheckpointFile` is declared with the exact
  signature above. It names no undeclared identifier, so it compiles against the pre-declaration
  tree and fails on that assertion. Only then does the seam land. The cycle-17 instruction to land
  "a compilable stub whose body performs the currently shipped behaviour of the callers it will
  absorb" is **withdrawn**: that stub carried real read/parse/mutate/marshal/atomic-replace
  behaviour with no observed failing test, which is the Principle II carve-out the
  `declaration-only` withdrawal was meant to eliminate. The seam's **behaviour** red is owned by
  **U12**, whose contract harness lands in a strictly later wave against this landed declaration
  and is turned green by **U13** — declaration → behaviour harness → implementation, each gated by
  a harness that precedes it. The seam has **no caller** until U14 migrates them, so nothing
  observable changes on a live path in this unit.
* **Scope boundary — quarantine is not in the seam**: `QuarantineCheckpoint`'s `moveNoReplace`
  path never parses and never re-marshals; it moves bytes verbatim. It is **correct by
  construction** and MUST NOT be routed through this seam. Doing so would introduce a parse
  precondition on the one verb whose entire purpose is to dispose of documents that cannot be
  parsed. `CleanupCheckpoints`' `os.Rename` is excluded for the same reason, and
  `CreateCheckpoint` is excluded because it creates a new file rather than rewriting an existing
  one.
* **Tests** (2): the seam is declared with the stated signature (asserted source-shape, red before
  the declaration lands) and is reachable from `internal/core` (an exported-surface compile
  assertion, green on landing). The `mutate`-error propagation and byte-identity obligation is a
  **behaviour** assertion and belongs to U12's contract harness, which runs against this landed
  declaration in a later wave.
* **Expected red** (1 harness function): `TestU11_RewriteSeamDeclared`, the source-shape function,
  fails against the pre-declaration tree — `RewriteCheckpointFile` is not declared. It is an
  assertion failure, never a build error, and never a failure against a behaviour-carrying stub.
* **Green-step guards** (1, `TestU11Guard_<Descriptor>`): case 1, the exported-surface reachability
  assertion, lands with the implementation.
* **Behaviour red owner**: U12 (`147.035-T`). Case 2 — a `mutate` that returns an error propagates
  it and leaves the file byte-unchanged — is a behaviour obligation of the seam and is asserted in
  U12's contract harness against the landed declaration, not here.
* **Depends on**: U2 (the conformance predicate the seam will call).

### U12 — Guarded rewrite seam: contract harness

* **Partition**: 2 (guarded rewrite seam)
* **Domain**: tests
* **Files**: `internal/events/checkpoint_rewrite_contract_test.go` (new). **No production change.**
* **Change**: land the executable contract for the seam. This is a harness-only unit: it compiles
  against the declaration U11 landed in an earlier wave and fails on assertions because the seam
  carries no validity and no conformance precondition yet. This is the **behaviour** red for the
  seam — U11's own harness is source-shape and gates only the declared signature, so every
  behavioural obligation of the seam is observed failing here, before U13 implements it.
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
* **Expected red**: all three fail — the landed U11 seam performs no validity and no conformance
  check, so every case currently rewrites the file and returns `nil`. Case 2's byte-identity and
  `mutate`-error propagation obligation, formerly stated on U11, is asserted here.
* **Green-step guards**: none. Every scenario is a red harness function; U13 turns all three green.
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
* **Expected red**: **none — `harness-exempt: covered-by` with `harness_owner: 147.035-T` (U12).** U13 is the implementation half of
  U12's red gate and scaffolds no test function of its own. Its verification command is U12's
  selector (`go test -count=1 -run '^TestU12_' ./internal/events`), which must go from three
  failures to zero. This is the plan's single behaviour-changing exemption and the explicitly
  allowed `covered-by` carve-out under P-002.1: U12 is a declared dependency, must carry
  `harness-ready` at claim time — eligible only after harness generation runs — rather than an
  exemption, and its red evidence must be confirmed before U13 builds. No other unit may claim this
  shape.
* **Green-step guards**: none.
* **Depends on**: U12.

### U14 — Resolve-verb caller migration onto the guarded seam

* **Partition**: 2 (guarded rewrite seam)
* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_lifecycle.go`,
  `internal/events/checkpoint_migration_test.go` (new)
* **Change**: replace the resolve-verb in-place rewrite site with a seam call.
  `ResolveCheckpoint` (`internal/events/checkpoint_lifecycle.go:178`) stops calling
  `syncWriteFileAtomic` directly and calls `RewriteCheckpointFile` with its existing
  mutation closure. **This unit introduces no new verb-facing sentinel and changes no ordering.**
  Its only claim is that the write mechanism moved.
* **Why the migration is its own unit**: the seam and the per-verb contracts are different skill
  problems. Folding them together would mean one task changed a write mechanism in two packages
  **and** three error contracts **and** two gate orderings — far past the 2-Hour Rule and past
  width isolation. Keeping the migration bare also gives U3 an honest red: after U14
  the verb refuses untrustworthy documents with the seam's **raw** verdict errors, so its
  own `errors.Is` sentinel assertion still fails until that verb's unit lands. **U3b is the
  exception the cycle-20 review found** (`147.007-T`): the resolve verb's *conformance* refusal
  contract asks only for `ErrCheckpointNonConforming` with the offender keys and unchanged bytes,
  and the seam's raw `*CheckpointNonConformingError` already unwraps to exactly that sentinel. That
  contract is therefore **delivered by this unit**, not by U3b, and its failing harness is **U3c**
  (`147.042-T`), which lands before this migration and goes green when it completes.
* **Why the abandon verb is no longer here (cycle 37)**: through cycle 36 this unit also migrated
  `AbandonCheckpoint` in `internal/core`, which made it two production packages plus a new test
  file — three files, which is not *fewer than three* and therefore outside the 2-Hour Rule
  heuristic this plan has already enforced against `147.021-T` and `147.014-T`. The abandon-site
  migration is now **U14b** (`147.044-T`). The two migrations touch disjoint packages and share no
  code, so each is independently verifiable at two files, and the split needs no new mechanism: it
  is the same bounding move cycle 17 made when it broke the original five-file seam unit into
  U11/U12/U13/U14 and cycle 20 made when it split U7b into U7b and U7c.
* **Tests** (2): `ResolveCheckpoint` on a conforming active document still resolves and the
  resulting bytes are unchanged from the pre-migration expectation (shipped-accept-path guard);
  `internal/events/checkpoint_lifecycle.go` contains no direct atomic-write call whose target
  resolves under the checkpoint directory **and** contains a `RewriteCheckpointFile` call inside
  `ResolveCheckpoint` — asserted structurally against that file. Both halves are load-bearing:
  asserting only the removal would be satisfied by routing the write through any other helper,
  which is not this unit's contract.
* **Expected red** (1 harness function): the structural case fails before the migration. This unit
  additionally turns **U3c**'s (`147.042-T`) already-red `TestU3c_` function green — U3c is the
  verb-level conformance harness that must be red before this migration lands.
* **Green-step guards** (1, `TestU14Guard_ResolveAcceptPathUnchanged`): the shipped accept path,
  which the migration must not disturb; it lands with the migration commit.
* **Depends on**: U13, U3c (the resolve-verb conformance harness this unit turns green — the
  backlog edge `147.037-T -> 147.042-T`).

### U14b — Abandon-verb caller migration onto the guarded seam

* **Partition**: 2 (guarded rewrite seam)
* **Domain**: code (core)
* **Files**: `internal/core/checkpoint_disposition.go`,
  `internal/core/checkpoint_migration_test.go` (new)
* **Change**: replace the abandon-verb in-place rewrite site with a seam call.
  `AbandonCheckpoint` (`internal/core/checkpoint_disposition.go:105-118`) stops calling
  `atomicfile.WriteFileAtomic` directly and calls `events.RewriteCheckpointFile` with its existing
  mutation closure. **This unit introduces no new verb-facing sentinel and changes no ordering.**
  Its only claim is that the write mechanism moved.
* **Why it is a separate unit (cycle 37)**: see U14 above. Splitting the two verb migrations is
  what brings both inside the three-file heuristic, and it costs nothing in coupling — the two
  sites live in different packages, use different write helpers, and have no shared symbol.
* **Why it does not satisfy U4**: after this unit the seam refuses an untrustworthy document at the
  *write* step, which in `AbandonCheckpoint` is **after** the audit append and after the
  already-abandoned short-circuit. The audit-free, short-circuit-proof ordering is U4's delta and
  U4's assertions pin it, so U4 stays honestly red until it lands.
* **Tests** (2): `AbandonCheckpoint` on a conforming active document still abandons and still
  appends exactly one audit event (shipped-accept-path guard);
  `internal/core/checkpoint_disposition.go` contains no direct atomic-write call whose target
  resolves under the checkpoint directory **and** contains an `events.RewriteCheckpointFile` call
  inside `AbandonCheckpoint` — asserted structurally against that file. Both halves are
  load-bearing: asserting only the removal would be satisfied by routing the write through any
  other helper, which is not this unit's contract.
* **Expected red** (1 harness function): the structural case fails before the migration.
* **Green-step guards** (1, `TestU14bGuard_AbandonAcceptPathUnchanged`): the shipped accept path,
  which the migration must not disturb; it lands with the migration commit.
* **Depends on**: U13 (the implemented seam). It does **not** depend on U3c, which is a
  resolve-verb harness.

### U2f — Supplemental caller-set regression guard for the guarded seam

* **Partition**: 2 (guarded rewrite seam)
* **Domain**: tests
* **Files**: `internal/events/checkpoint_writesite_test.go` (new). No production change.
* **Change (cycle-17 rewrite — no longer the authoritative I1 mechanism)**: the cycle-16 gate ruled
  (H8) that an AST enumeration of write calls cannot fully enforce I1: it must resolve call
  targets, alias-imported helpers, indirect writers, and any future helper that wraps an atomic
  write, and any gap in that resolution is a silent hole in the invariant. **The authoritative I1
  enforcement is now architectural** — U11/U12/U13/U14/U14b make one guarded seam the only in-place
  rewrite path, so an ungated rewrite cannot exist without adding a new direct write site.
  This unit keeps the enumeration as a **supplemental caller-set regression guard**: it walks
  `internal/events` and `internal/core` for calls to `syncWriteFileAtomic` /
  `atomicfile.WriteFileAtomic` / `os.WriteFile` whose target resolves under the checkpoint
  directory and asserts the resulting call-site set equals the post-migration allow-list — which
  after U14 and U14b contains the seam's own write and the excluded verbatim-move / create sites,
  and nothing else.
* **Honest bound on what this test proves**: it proves that no **direct, statically resolvable**
  atomic-write call to the checkpoint directory was added outside the seam. It does **not** prove
  the absence of an indirect or dynamically dispatched writer, and it is not relied on for that.
  The seam is what makes the invariant hold; this test makes a common regression loud.
* **Halt condition**: if the enumeration cannot be implemented reliably, mark the unit `blocked`
  **and remove it from shipment `130-S` with the `return_blocked` operation**
  (`backlogit_return_blocked`, CLI `backlogit shipment return-blocked --shipment 130-S --item
  147.021-T --reason <reason>`), then record it as a follow-up. Marking the task `blocked` alone
  does **not** unblock the release: `147.021-T` stays in the `130-S` manifest and the shipment
  cannot ship while a manifest member is not `done`. `ReturnBlockedItem` is the operation that
  atomically removes a blocked item from a shipment manifest, and it is the halt path this unit
  requires (cycle-20 review, P2). Because I1 no longer depends on this unit, a **returned** U2f
  does not block the release unit — this is the concrete benefit of moving enforcement into the
  seam.
* **Tests** (2): the enumerated call-site set equals the post-migration allow-list across both
  packages; a synthetic ungated rewrite site added to the fixture corpus fails the assertion.
* **Expected red**: **none — `harness-exempt: verification-only`.** This unit has **no production
  change**: the enumeration it asserts lives inside the test file itself, so there is no compilable
  pre-implementation state in which either assertion fails on behaviour. Cycle 19's "case 2 fails
  until the enumeration exists" described the absence of the test's own helper, which is not a red
  phase. Both functions are green-step guards.
* **Green-step guards** (2, `TestU2fGuard_<Descriptor>`): the enumerated call-site set equals the
  post-migration allow-list across both packages; a synthetic ungated rewrite site added to the
  fixture corpus fails the assertion.
* **Where the RED for I1 lives**: architecturally, in U12 (`147.035-T`), whose three seam-contract
  functions are red before U13 implements the preconditions. This unit is supplemental and is not
  load-bearing for the invariant.
* **Depends on**: U14, U14b. Both caller migrations must land before the enumerated call-site set
  can equal the post-migration allow-list.

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
* **Expected red** (2 harness functions): `go test -count=1 -run '^TestU2g_' ./internal/events`
  selects exactly `TestU2g_DuplicateExactContextMember` and
  `TestU2g_DuplicateFoldVariantAliasingModeledField`, and **both fail** before implementation
  because no context-member walk exists. The harness commit contains no other function under this
  unit's prefix.
* **Green-step guards** (1, `TestU2gGuard_OpenNamespacePreserved`): the open-namespace preservation
  guard — one table carrying distinct unmodeled fold variants, NFC/NFD-distinct extension keys, and
  a `context` object of unique extension keys — lands **with the implementation**, never in the
  harness commit.
* **Cycle-20 correction**: cycle 17 kept this guard inside the harness commit and excluded it from
  the red gate with the narrower selector `^TestU2g_Duplicate`. The cycle-20 Copilot review raised
  that as a P1: P-002/P-004 gate the harness commit, not the selector, so a green function
  committed in the harness step defeats the gate no matter which regex the implementer runs. The
  narrowed selector is withdrawn; `^TestU2g_` is now exact because the guard is not there yet.
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
* **Expected red** (1 harness function): case 1 fails — the U2g walk keys on the literal spelling
  and never inspects a `Context` member. `go test -count=1 -run '^TestU2h_' ./internal/events`
  selects only this function in the harness commit.
* **Green-step guards** (1, `TestU2hGuard_<Descriptor>`): case 2 guards the open namespace under a
  non-canonical spelling and lands with the implementation. Cycle 17's narrowed red selector
  `^TestU2h_Duplicate` is withdrawn for the same reason as U2g's (cycle-20 review, P1).
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
* **Expected red** (1 harness function): case 1 fails (resolve currently succeeds and rewrites).
  Verified with `go test -count=1 -run '^TestU3_' ./internal/events`.
* **Green-step guards** (2, `TestU3Guard_<Descriptor>`): cases 2 and 3 pin the shipped accept and
  idempotent-no-op paths and land with the implementation.
* **Depends on**: U2c (the completed predicate, including the duplicate rule — this is the declared
  backlog edge `147.006-T -> 147.004-T`; cycle-15 prose said "U2", which named a transitive
  ancestor rather than the real prerequisite), U14 (the resolve-verb caller migration onto the
  guarded seam), and
  U8b (the parity harness that must be red first).

### U3c — Resolve-verb conformance refusal harness (RED, lands before the migration)

* **Partition**: 2 (guarded rewrite seam)
* **Domain**: tests
* **Files**: `internal/events/checkpoint_lifecycle_conformance_test.go` (new). **No production
  change.**
* **Why this unit exists (cycle-20, closes the Group-B P1 on `147.007-T`)**: the resolve verb's
  conformance refusal contract — *a valid-but-non-conforming document is refused with
  `ErrCheckpointNonConforming` naming the offender keys, and the file bytes are unchanged* — is
  **delivered by U14**, not by U3b. After the migration `ResolveCheckpoint` calls
  `RewriteCheckpointFile`, whose U13 implementation returns the raw `*CheckpointNonConformingError`
  before any marshal or write; that error unwraps to `ErrCheckpointNonConforming` (U1), so the
  contract already holds. Cycle 19 attached this contract to U3b's own implementation task and
  called it red, which is the pretence the cycle-20 review rejected. This unit is the harness the
  contract actually needs: it lands **before** U14 and fails against the pre-migration verb.
* **Tests** (1): `ResolveCheckpoint` on a valid-but-non-conforming document (schema-valid,
  `status: "active"`, carrying two unmodeled top-level keys) is refused with an error satisfying
  `errors.Is(err, ErrCheckpointNonConforming)`, `errors.As` recovers both offender keys sorted, and
  the file SHA is identical before and after the call.
* **Expected red** (1 harness function): `TestU3c_ResolveRefusesValidNonConformingDocument` fails
  against the pre-migration `ResolveCheckpoint`, which mutates and rewrites the document and
  returns `nil` — so the refusal assertion, the offender-key assertion, and the byte-identity
  assertion all fail on assertions rather than on a build error. Verified with
  `go test -count=1 -run '^TestU3c_' ./internal/events`.
* **Green-step guards**: none. U14 (`147.037-T`) turns this function green; no guard is added by
  this unit.
* **Ordering obligation**: this unit MUST be `harness-ready` and red before `147.037-T` is claimed.
  The edge `147.037-T -> 147.042-T` enforces it.
* **Depends on**: U1 (the sentinel and typed error the assertion matches), U2c (the completed
  conformance predicate the seam consults) — the backlog edges `147.042-T -> 147.001-T` and
  `147.042-T -> 147.004-T`.

### U3b — Resolve-verb conformance contract and the named already-resolved residual

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: tests
* **Files**: `internal/events/checkpoint_lifecycle_conformance_guard_test.go` (new). **No production
  change (cycle-20 rewrite; guard file renamed in cycle 26).** The guards deliberately live in
  their own file rather than in U3c's `checkpoint_lifecycle_conformance_test.go`, which is created
  earlier in the execution order — `build-feature` Step 5's `verification-only` exception admits
  only *new* files a task names, so appending to U3c's file would exceed the class.
* **Change (cycle-20 — closes the Group-B P1 raised on `147.007-T`)**: cycle 17 gave this unit a
  production delta — "after the U3 validity gate, add `CheckConformingTopLevelNamespace(data)`" —
  that U14 had already made redundant. Once `ResolveCheckpoint` routes through
  `RewriteCheckpointFile`, adding a second conformance call in the verb changes no observable
  behaviour and produces the same error the seam already returns. The delta is **withdrawn**. What
  survives is the verb-level contract pin: this unit records, as green validation after U14 and U6
  have landed, that the resolve verb refuses the non-conforming class through the seam and that the
  named residual behaves as documented.
* **Named, accepted residual (unchanged)**: a document whose `status` is already `"resolved"`
  returns `nil` at the step-5 short-circuit and never reaches the seam. This is deliberate — a
  non-writing terminal answer, and moving the gate ahead of it would turn a shipped idempotent
  no-op into a new error — but it means the exact fabricated skeletons produced by the *pre-fix*
  `ResolveCheckpoint`, which carry `status: "resolved"` and are schema-invalid, bypass the gate.
  Their discovery path is U6, not resolve.
* **Tests** (2): an **invalid, already-resolved** document returns `nil` with bytes unchanged
  **and** U6 flags the same file `NeedsQuarantine: true` (pins the residual and its discovery path
  together, so neither half can be deleted alone); a document with `disposition: "abandoned"` still
  returns `ErrCheckpointCannotResolveAbandoned`.
* **Expected red**: **none — `harness-exempt: verification-only`.** Both scenarios assert shipped
  behaviour once this unit's prerequisites have landed, and the conformance-refusal scenario that
  cycle 19 counted as red now lives in **U3c** (`147.042-T`), which is red before U14. No harness
  test function is scaffolded here.
* **Green-step guards** (2, `TestU3bGuard_<Descriptor>`): the two scenarios above, landing in this
  unit's own commit after U14 and U6.
* **Width**: 1 file, 2 scenarios, no production change — inside the 2-Hour Rule with margin.
* **Depends on**: U3, U6, U2g, U14, U8b — the declared backlog edges (`147.007-T` ->
  `147.006-T`, `147.011-T`, `147.028-T`, `147.037-T`, `147.016-T`). The U14 edge now carries a
  second obligation: it is what makes this unit's conformance claim true, so it must land first.
  The U8b edge is retained as a harness-order pin even though this unit no longer implements
  anything.


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
  does not satisfy this unit.** After U14b the guarded seam refuses the same document, but it does
  so at the *write* step — which in `AbandonCheckpoint` is **after** the audit append and after the
  already-abandoned short-circuit. The ordering this unit installs is what makes the refusal
  audit-free and short-circuit-proof, and it is what U4's assertions pin.
* **Tests** (3): a valid-but-non-conforming active document is refused with
  `ErrCheckpointNonConforming` naming the keys; the disposition audit JSONL is byte-unchanged after
  that refusal; a non-conforming **already-abandoned** document returns `ErrCheckpointNonConforming`
  rather than `nil`. The existing "conforming active document abandons successfully" test must stay
  green as a regression guard.
* **Expected red**: all three fail. Verified with `go test -count=1 -run '^TestU4_' ./internal/core`.
* **Green-step guards**: none. Every scenario is a red harness function.
* **Depends on**: U2c, U2g, U14b (the abandon-verb caller migration onto the guarded seam), U8b
  (the parity harness). The cycle-16 `147.008-T -> 147.021-T` (U2f) edge is replaced by
  `147.008-T -> 147.044-T` (U14b): U2f is no longer the I1 mechanism, so it is no longer a
  prerequisite for touching this call site. Cycle 37 retargeted this edge from `147.037-T` to
  `147.044-T` when the caller migration split by verb — this unit edits
  `internal/core/checkpoint_disposition.go`, which is U14b's file, not U14's.

### U17 — `AbandonCheckpoint` validation wrap preserves both sentinels

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (core)
* **Files**: `internal/core/checkpoint_disposition.go`, `internal/core/checkpoint_disposition_test.go`
* **Change (cycle-17 — closes gate finding H7)**: `AbandonCheckpoint` wraps its validation failure
  as `fmt.Errorf("%w: %v", ErrCheckpointUseQuarantine, valErr)`
  (`internal/core/checkpoint_disposition.go:~70-73`). The `%v` verb drops the
  `ErrCheckpointInvalid` sentinel that `ValidateCheckpoint` returns, so
  `errors.Is(err, ErrCheckpointInvalid)` is false on a path this plan **already touches** in U4 and
  U14b. Change the verb to multi-`%w`:
  `fmt.Errorf("%w: %w", backlogiterrors.ErrCheckpointUseQuarantine, valErr)`. Go 1.20+ supports
  multiple `%w` verbs and the module is Go 1.24, so both sentinels stay traversable — exactly the
  idiom Q2 already requires of `ResolveCheckpoint` (U3).
* **Why the deviation cannot stand (Principle I is not waivable here)**: cycles 1–16 recorded this
  as a "documented deviation" from Constitution Principle I on the grounds that the wrap is
  pre-existing. The cycle-16 gate rejected that: Principle I says *all* errors must be wrapped with
  context and sentinels must remain traversable, and the deviation table's own escape hatch —
  "fixing it changes an unrelated shipped error contract" — does not apply once U4 and U14b modify
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
* **Expected red** (1 harness function): case 1 fails — `errors.Is(err, ErrCheckpointInvalid)` is
  false against the shipped `%v` wrap.
* **Green-step guards** (1, `TestU17Guard_<Descriptor>`): case 2 pins the message text and lands
  with the implementation.
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
* **Expected red** (1 harness function): scenario 1's accept-half fails (quarantine currently
  refuses it). Verified with `go test -count=1 -run '^TestU5_' ./internal/core`.
* **Green-step guards** (1 function plus 1 postcondition, `TestU5Guard_<Descriptor>`): scenario 2
  pins the active-scope hard gate, and scenario 1's byte-identity postcondition — green on landing
  — moves into the green step with it. Cycle 19 committed the byte-identity assertion inside the
  harness step alongside the failing accept-half; cycle 20 separates them so the harness commit
  contains only failing functions.
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
* **Expected red** (2 harness functions): cases 1 and 2 fail.
* **Green-step guards** (1 function plus the byte-identity postconditions,
  `TestU6Guard_<Descriptor>`): case 3 — the verdict is computed before the filter block — and the
  per-case SHA-before/after postconditions are green on landing, because `ListCheckpoints` is
  already read-only. All of them land with the implementation.
* **Contract scope (cycle-20, P2 on `147.014-T`)**: this unit populates `RemediationIntent` on the
  **conformance** branch only. The shipped parse-failure branch
  (`internal/events/checkpoint_lifecycle.go:52-54`) and the schema-invalid branch (`:72-74`) both
  set `NeedsQuarantine: true` with `RemediationCommand` alone, so after this unit a
  `needs_quarantine` summary does **not** universally carry a structured intent. **U6e**
  (`147.043-T`) closes that gap; U7b's published description depends on U6e for its universal
  claim.
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
* **Expected red** (2 harness functions): cases 1 and 3 fail.
* **Green-step guards** (1, `TestU6dGuard_<Descriptor>`): case 2 — a **conforming**
  `status: "resolved"` file is still dropped by the same filter — lands with the implementation.
  The doc-comment update is a contract obligation carried by this unit, not a fourth scenario.
* **Depends on**: U6, U8b.

### U6e — `RemediationIntent` on the parse-failure and schema-invalid quarantine branches

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: code (events)
* **Files**: `internal/events/checkpoint_lifecycle.go`, `internal/events/checkpoint_lifecycle_test.go`
* **Why this unit exists (cycle-20, closes the P2 raised on `147.014-T`)**: U7b publishes the
  agent-facing sentence *"The accompanying remediation_intent is a structured record of the
  required disposition, not a runnable command"* on `backlogit_list_checkpoints`. After U6 alone
  that sentence is false: `ListCheckpoints` sets `NeedsQuarantine: true` on three branches, and U6
  populates `RemediationIntent` on only one of them. The shipped parse-failure branch
  (`internal/events/checkpoint_lifecycle.go:52-54`) and the schema-invalid branch (`:72-74`) still
  carry `RemediationCommand` alone. Publishing a universal contract the read path does not satisfy
  is exactly the class of defect this plan exists to close, so the contract is made true rather
  than narrowed.
* **Change**: populate `RemediationIntent` on the two remaining quarantine-candidate branches using
  the carrier U1d declares — `Verb: "quarantine"`, `TargetFilename: <bare filename>`,
  `RequiresApproval: true`, `ApprovalClass: "A4c"` — with `Reason: "unparseable"` on the
  parse-failure branch and `Reason: "schema_invalid"` on the schema-invalid branch. These are
  exactly the two remaining values U1d's `Reason` field already enumerates.
  **Precedence, decided here**: `unparseable` is terminal (the branch appends and `continue`s, so
  no later branch runs); when a document is both schema-invalid and non-conforming, U6's
  conformance branch runs after the validity branch and overwrites `Reason` with `non_conforming`,
  matching the `ValidationErr` append order that already reports both reasons.
* **Shipped-field disposition**: `RemediationCommand` keeps its shipped population on both
  branches, unchanged and still `// Deprecated:`. This unit neither extends nor removes it;
  removal remains follow-up stash `F350503F`.
* **Tests** (2): an unparseable file lists with `NeedsQuarantine: true` and a `RemediationIntent`
  whose `Reason` is `unparseable`, naming the bare filename with `requires_approval: true` and
  class `A4c`; a schema-invalid but **conforming** file lists with a `RemediationIntent` whose
  `Reason` is `schema_invalid`.
* **Expected red** (2 harness functions): both fail against the post-U6 state, where
  `RemediationIntent` is `nil` on both branches. Verified with
  `go test -count=1 -run '^TestU6e_' ./internal/events`.
* **Green-step guards**: none. Every scenario is a red harness function.
* **Width**: 2 files, 2 scenarios.
* **Depends on**: U6 (`147.011-T`, the branch structure and the conformance-branch precedent) and
  U1d (`147.032-T`, the carrier) — the backlog edges `147.043-T -> 147.011-T` and
  `147.043-T -> 147.032-T`.

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

  `GetCheckpointResult` lands as a thin wrapper that calls the **existing** `GetCheckpoint` and
  returns `&CheckpointReadResult{Checkpoint: cp, Valid: err == nil}` with every other field at its
  zero value. `GetCheckpoint` is retained unchanged as a wrapper returning `res.Checkpoint`, so
  every existing caller compiles untouched. Per the cycle-31 lifecycle this lands in the **green**
  step, after the `TestU15_*` source-shape harness below has been observed red — no stub precedes
  the harness.
* **Declaration boundary**: this unit adds **no** conformance evaluation, **no** intent
  population, and **no** offender projection. Those are U6b's production delta. Keeping the
  declaration separate is what makes U8b's red honest — the harness compiles against a real type
  and fails because the fields are unpopulated, not because a symbol is missing.
* **Tests** (2): `GetCheckpointResult` on a conforming active document returns a non-nil result
  whose `Checkpoint` matches `GetCheckpoint`'s return and whose `Valid` is true; on a schema-invalid
  document it returns the pre-existing `ErrCheckpointInvalid` **unwrapped**, so
  `errors.Is(err, ErrCheckpointInvalid)` holds and `QuarantineIsRemedy(err)` is false — a read is
  not a rewrite and there is nothing to refuse.
* **Expected red**: **3 source-shape harness functions (cycle-29).** This unit is **not**
  `harness-exempt`; the `declaration-only` class it carried from cycle 20 was withdrawn after PR
  #377 review found it admitted an exported executable wrapper (`GetCheckpointResult`) with no
  observed failing test. Cycle 19's "case 1 fails against the pre-declaration state (the symbol
  does not exist)" did describe a **build error**, and the `Valid: err == nil` assertion would pass
  the instant the symbol landed. A **source-shape** harness in
  `internal/events/checkpoint_readresult_test.go` is neither: it parses
  `checkpoint_lifecycle.go` with `go/parser` and asserts the declared shape via `go/ast`, so it
  compiles against the pre-declaration tree and fails on assertions —
  `TestU15_CheckpointReadResultDeclared`, `TestU15_GetCheckpointResultDeclared`, and
  `TestU15_GetCheckpointRetainedAsWrapper` (which carries the behaviour-preserving re-expression
  pin the withdrawn class used to require of the exempt gate). **Verified at worktree HEAD
  `6a822ceb`**: the package compiles with the harness present (`go vet` exit 0;
  `go test -run='^$' -count=1` reports `[no tests to run]`), and
  `go test -count=1 -v -run '^TestU15_' ./internal/events` exits 1 with assertion failures, not
  build errors.
* **Red-verification command**: `go test -count=1 -run '^TestU15_' ./internal/events`.
* **Green-step guards**: none separate. The three harness functions above are the guards.
* **Wave placement (P-002.6)**: wave 3 — after U1b (wave 2) and U1d (wave 1), before U8b (wave 4).
  This ordering is what removes the deadlock PR #377 review identified: U8b's harness needs U15
  landed, U15 needs non-exempt U1b landed, and one up-front harness pass cannot satisfy both.
* **Where the RED for this carrier lives**: **U6b** (`147.012-T`) owns the failing harness for the
  projected fields — its cases 1 and 2 fail precisely *because* this declaration leaves
  `Conforming`, `NeedsQuarantine`, `RemediationIntent`, and `NonConformingFields` at their zero
  values. U8b (`147.016-T`) adds the cross-surface red on the same declaration state.
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
* **Expected red** (2 harness functions): cases 1 and 2 fail — U15's declaration wrapper leaves
  every projected field at its zero value, so `Conforming` is false for a conforming file and
  `NeedsQuarantine` is false for a non-conforming one. Verified with
  `go test -count=1 -run '^TestU6b_' ./internal/events`.
* **Green-step guards** (1, `TestU6bGuard_<Descriptor>`): case 3 pins `GetCheckpoint`'s read-only
  contract and lands with the implementation.
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
* **Expected red** (2 harness functions): cases 1 and 2 fail. Verified with
  `go test -count=1 -run '^TestU6c_' ./internal/mcp`.
* **Green-step guards** (1, `TestU6cGuard_<Descriptor>`): case 3 pins the shipped read contract and
  lands with the implementation.
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
* **Green-step guards**: none. Every scenario is a red harness function.
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
* **Expected red** (2 harness functions): cases 1 and 2 fail (routing and both remediation-verb
  assertions).
* **Green-step guards** (1, `TestU7dGuard_<Descriptor>`): case 3 lands with the implementation.
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
* **Green-step guards**: none. The single scenario is a red harness function.
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
  sentence was added in cycle 17 so the description does not imply a runnable string, and cycle 20
  makes it **true for every `needs_quarantine` summary** by adding **U6e** (`147.043-T`) as a
  prerequisite. Before U6e only the conformance branch populated the structured intent, while the
  shipped parse-failure and schema-invalid branches carried `RemediationCommand` alone — so the
  sentence as published described a contract the read path did not satisfy for two of its three
  quarantine-candidate branches. The cycle-20 Copilot review raised this as a P2 and it is closed
  by making the code total rather than by weakening the description.
* **Tests** (2): a table-driven assertion over the two registered read descriptions, read from the
  **built tool set** rather than a duplicated literal, each row keyed by the registered
  `backlogit_*` name; and the existing registry-parity /
  fallback-map drift test
  (`docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`, Rule 1)
  re-run and staying green, with `.autoharness/backlog-registry.yaml` updated in the same commit
  if it carries description text for these tools.
* **Expected red** (1 harness function): both rows of case 1 fail.
* **Green-step guards** (1, `TestU7bGuard_<Descriptor>`): case 2, the registry-parity /
  fallback-map drift test, lands with the implementation.
* **Depends on**: U6b, U6c, U6d, U6e, U8b. The U6e edge (`147.014-T -> 147.043-T`) is what makes
  the `remediation_intent` sentence true for every `needs_quarantine` summary.

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
* **Expected red** (1 harness function): all three rows of case 1 fail.
* **Green-step guards** (1, `TestU7cGuard_<Descriptor>`): case 2, the registry-parity drift test,
  lands with the implementation.
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
* **Green-step guards**: none. Every scenario is a red harness function.
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
* **Expected red** (2 harness functions): cases 1 and 2 fail (no renderer exists).
* **Green-step guards** (1, `TestU16Guard_<Descriptor>`): case 3 pins the conforming path and lands
  with the implementation.
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
* **Expected red** (2 harness functions): cases 1 and 2 fail. Verified with
  `go test -count=1 -run '^TestU8c_' ./internal/cli`.
* **Green-step guards** (1, `TestU8cGuard_<Descriptor>`): case 3 pins the shipped read contract and
  lands with the implementation.
* **Depends on**: U6b, U8b.

### U8b — Cross-surface parity harness (RED) from one stored state

* **Partition**: 3 (declarations and genuine RED harness order)
* **Domain**: tests
* **Files**: `internal/cli/checkpoint_parity_test.go` (new). **No production change.**
* **Ordering contract (cycle-17 — closes gate finding H2)**: this harness lands **after** the
  partition-1 and partition-2 declarations and **before** every partition-4 implementation. Its
  dependencies are declarations only — U1, U1b, U1d, U2, U15 — and the eighteen behavioural units it
  pins (U3, U3b, U4, U5, U6, U6b, U6c, U6d, U7, U7b, U7c, U7d, U7e, U8, U8c, U9, U16, U17) each
  depend on **it**, so the harness is observably red before any of them lands. Cycle 15 and cycle 16
  wired the reverse direction, which
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
  assertions**. That is the two-step lifecycle applied across surfaces instead of inside one
  package. The cycle-15/16 "batch harness generation phase" framing is withdrawn: it made the red
  gate depend on when an implementer chose to generate harnesses rather than on the dependency
  graph.
* **Already-green assertions relocated (cycle-20; cycle-17 gate finding H2)**: cycle 16 claimed the
  `legacy-shaped` row's `get` assertions were red. They were not. `GetCheckpoint` already runs
  `ValidateCheckpoint` and already returns `ErrCheckpointInvalid` for a schema-invalid document
  (`internal/events/checkpoint_lifecycle.go:~105-137`), `domainError` already maps that sentinel to
  `validation_failed` (`internal/mcp/errors.go:~188-193`), and the CLI already exits non-zero on
  it. Cycle 17 kept those three assertions in the harness and excluded them from the red gate by
  description; cycle 20 relocates them to the green step as `TestU8bGuard_` functions, because
  P-002/P-004 gate the harness commit rather than the prose that describes it.
* **Expected red** (against the partition-3 declaration state, per row):

  * **`legacy-shaped` row**: the three `get` assertions —
    `errors.Is(events.GetCheckpointResult(...), ErrCheckpointInvalid)`, MCP
    `backlogit_get_checkpoint` → `code: validation_failed`, and CLI `checkpoint get` → non-zero
    exit — are green on landing and therefore land in the **green step** as
    `TestU8bGuard_<Descriptor>` functions, not in the harness commit. The row's single **RED**
    assertion is `resolve`: it currently succeeds and rewrites the
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
    `"valid": true` with no conformance field before U8c. Accepted `abandon` is green on landing
    and moves to the green step as a `TestU8bGuard_` function.
  * **Byte-identity postcondition**: applies **only** to refused-mutation paths (rows 1 and 2's
    non-mutation surfaces exercised against the canonical fixture directly). The
    `conforming-active` row's accepted `abandon` necessarily mutates its fresh fixture and
    asserts the intended rewrite/archive outcome, not byte identity. Against the current
    `ResolveCheckpoint` — which rewrites on the `valid-but-non-conforming` row — the assertion
    **fails** for row 2's fixture.

  Rows 1, 2, and 3 carry this unit's red gate. The already-green assertions — row 1's three `get`
  assertions and row 3's accepted `abandon` — are **not** committed in the harness step. They land
  with the parity implementation as `TestU8bGuard_<Descriptor>` functions, so the harness commit
  contains only functions that fail against the partition-3 declaration state. Cycle 19 committed
  them inside the harness and excluded them from the red gate by description; the cycle-20 Copilot
  review raised that as a P1, because P-002/P-004 gate the harness commit rather than the prose.
* **Green-step guards** (2, `TestU8bGuard_<Descriptor>`): the `legacy-shaped` row's three `get`
  refusal assertions, and the `conforming-active` row's accepted `abandon`.

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
* **Tests**: `go run ./cmd/backlogit --cwd . docs lint` reports 0 violations; the CLI Reference
  Drift check is clean.
* **Expected red**: n/a — `harness-exempt: docs-only`. This unit scaffolds no Go test function; it
  runs after behaviour is final and claims no RED behaviour. Its gate is
  `go run ./cmd/backlogit --cwd . docs lint`.
* **Green-step guards**: none.
* **Depends on**: U2e, U6b, U8b, U16, U17.

### U9b — Agent instruction file and quarantine-first disposition guidance

* **Partition**: 4 (implementation plus MCP/CLI/instruction contracts)
* **Domain**: docs (agent-facing)
* **Harness**: `harness-exempt: docs-only` — no Go test function is scaffolded; the gate is
  `go run ./cmd/backlogit --cwd . docs lint` plus the merge-gate check in `147.018-T`.
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
* **Tests**: `go run ./cmd/backlogit --cwd . docs lint` reports 0 violations on the changed file.
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
  **Discoverability (cycle 22)**: `147.007-T`, `147.008-T`, and `147.009-T` each carry a
  `merge-gate-dependent` label and a backreference paragraph naming this gate, and `147-F.md` and
  `130-S.md` state it at feature and shipment level. Those are labels and prose only — no
  dependency edge is added and executable ordering is unchanged.
* **Depends on**: U9.

### U10 — Runtime verification of the refusal path

* **Partition**: 5 (runtime verification and closure)
* **Domain**: verification
* **Harness**: `harness-exempt: verification-only` — no Go test function is scaffolded; the gate is
  the recorded runtime evidence in `docs/closure/`.
* **Files**: none (produces `docs/closure/` evidence only). **Cycle-29**: the `.gitignore`
  scratch-directory rule is removed from this unit's delta — the workspace moved to an
  already-ignored path, so no ignore rule is committed.
* **Change**: none to product code. Build the binary **from the branch under test** (not the pinned
  repo-root `backlogit.exe`, which predates the change) and exercise the **refusal** path against a
  **scratch** workspace seeded with copies of the legacy document shapes. The acceptance and
  evidence-integrity rows moved to **U10b**: five rows exceeded the four-scenario limit. The nine
  live files under `.backlogit/checkpoints/` are read for shape reference only and are never
  mutation targets (R6); the check is a programmatic before/after SHA-256 comparison of **every**
  file under `.backlogit/checkpoints/`, not a visual one, and not a count-pinned subset — the count
  has already moved from twelve (when the staging checkpoint first landed) to twenty as of the
  cycle-24 confirmatory review, purely from ordinary session checkpoints accumulating alongside the
  same nine schema-invalid legacy files, and it will keep drifting upward every session that adds
  one, so the guard enumerates the directory rather than a literal.
* **Binary provenance (cycle-16 correction)**: build with the repository's own version ldflags —
  the `LDFLAGS` variable in `Makefile:5-8`, whose shape the release workflow reproduces at
  `.github/workflows/release.yml:99-107`:

  ```text
  go build -ldflags "-X github.com/softwaresalt/backlogit/internal/version.Version=verify-<short-sha> -X github.com/softwaresalt/backlogit/internal/version.Commit=<short-sha> -X github.com/softwaresalt/backlogit/internal/version.BuildDate=<rfc3339>" -o .copilot/scratch/checkpoint-verification/backlogit-verify.exe ./cmd/backlogit
  ```

  where `<short-sha>` is `git rev-parse --short HEAD`, matching the Makefile's `COMMIT` derivation.
  Then assert that
  `.copilot/scratch/checkpoint-verification/backlogit-verify.exe version --format json --no-update-check`
  reports a `commit` field **equal to that `<short-sha>`**. The flags are exact:
  `--format json` is the JSON selector (`internal/cli/version_cmd.go`) — there is no `--json` flag —
  and `--no-update-check` suppresses the remote release lookup so the assertion is hermetic. A plain
  `go build ./cmd/backlogit` leaves `internal/version.Version` at its `DevVersion` default with an
  **empty** commit and proves nothing about provenance.
* **Execution contract (cycle 16, corrected cycle 17 — mandatory, fail-closed)**: every command in
  this unit MUST
  1. **run under an explicit A4c approval batch** obtained immediately before execution, naming the
     files and the directory it will touch;
  2. **bind to the canonical workspace** with `--cwd .copilot/scratch/checkpoint-verification` and pass
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
* **Evidence persistence (cycle-17, filename dated cycle-24)**: each row writes a deterministic,
  human-readable record to
  `docs/closure/2026-08-24-checkpoint-disposition-runtime-verification.md` — a tracked file — carrying
  per-file `filename`, `sha256`, `state`, `destination`, and outcome. The scratch directory stays
  git-ignored and is **not** the evidence of record: an ignored, machine-local artifact cannot be
  reviewed and does not survive teardown. Cycle-17 makes the closure file the artifact and the
  scratch directory a working area.
* **Scratch containment (cycle-29 relocation)**: the scratch workspace is created **inside the
  repository working tree** at `.copilot/scratch/checkpoint-verification/` (never `%TEMP%`, never
  outside the cwd — Constitution IV), and the resolved path is asserted to be repo-root-relative
  **before the first write**. It is added to the freeze-scope declaration. `.copilot/` is
  **already** ignored by the committed `.gitignore` (`git check-ignore -v` resolves to
  `.gitignore:5:.copilot/`; `git ls-files .copilot` returns 0 tracked entries), and `*.exe`
  (`.gitignore:25`) additionally covers the built binary — so **this unit commits no ignore rule
  at all**. Cycles 17-28 placed the workspace at `.copilot/scratch/checkpoint-verification/`, which
  `.gitignore` does **not** cover, which forced U10 to commit a `.gitignore` rule that its own
  `verification-only` class surface rejects (`EXEMPT_DELTA_EXCEEDS_CLASS`) while omitting the rule
  left the tree dirty and halted U10b's claim-time baseline. PR #377 review raised the
  unsatisfiable constraint on `147.019-T.md:83`; relocating to an existing ignored path removes the
  rule from the delta instead of widening the class contract, and adds no task and no edge. U10b
  and U10c inherit all of it. **Teardown does not run here**: U10b and U10c consume this unit's
  quarantine archive, fixtures, and branch-built binary,
  so the workspace is handed over intact and teardown ownership moves to **U10c** (cycle-17; it was
  U10b through cycle 16). It stays classified `ActionRisk: destructive` (A4b) requiring
  operator approval (Constitution VII) at the point U10c performs it. If approval is not granted
  the directory is left in place and recorded as a cleanup follow-up.
* **Depends on**: U9b.

### U10b — Runtime verification of acceptance, evidence integrity, and the recovery sweep

* **Partition**: 5 (runtime verification and closure)
* **Domain**: verification
* **Harness**: `harness-exempt: verification-only` — no Go test function is scaffolded; the gate is
  the recorded runtime evidence in `docs/closure/`.
* **Files**: none. Runs entirely inside the scratch workspace U10 created; no repository file
  changes.
* **Change**: none to product code. **The recovery sweep runs against a scratch mirror, never the
  live directory.** U10 requires every file under `.backlogit/checkpoints/` to be byte-unchanged,
  while the nine-file acknowledgement requires a sweep that "succeeds on every other file in that
  directory" — a real `resolve` against the conforming active checkpoints there. The two cannot
  both hold against the live corpus, so the sweep operates on a **copied mirror** inside the
  scratch workspace (workspace root: `.copilot/scratch/checkpoint-verification/mirror/`; checkpoints go
  into `.copilot/scratch/checkpoint-verification/mirror/.backlogit/checkpoints/`). All sweep CLI
  invocations use `--cwd .copilot/scratch/checkpoint-verification/mirror` with bare filename
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
  `docs/closure/2026-08-24-checkpoint-disposition-runtime-verification.md`.
* **Depends on**: U10 (scratch workspace, ignore rule, branch-built binary, and the quarantine
  archive row 2 inspects).

### U10c — Runtime verification of context-duplicate parity and the abandoned-resolve handler

* **Partition**: 5 (runtime verification and closure)
* **Domain**: verification
* **Harness**: `harness-exempt: verification-only` — no Go test function is scaffolded; the gate is
  the recorded runtime evidence in `docs/closure/`.
* **Files**: none beyond appending to
  `docs/closure/2026-08-24-checkpoint-disposition-runtime-verification.md`. Runs inside the scratch
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
* **Teardown**: `.copilot/scratch/checkpoint-verification/` teardown is owned by this unit and runs
  only after all three rows pass — `ActionRisk: destructive` (A4b), explicit operator approval
  immediately before execution, skipped and recorded as a cleanup follow-up when approval is
  withheld. The tracked closure evidence file survives teardown by construction.
* **Depends on**: U10b (the workspace and its inherited inputs), U2h, U6c, U7d, U7e, U8c.

## Dependency Graph

Canonical as of **cycle 20**. Every active unit and every executable edge appears below; U5b is
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
  U2 ──▶ U11 ──▶ U12 ──▶ U13 ──┬──▶ U14  ─┬──▶ U2f
                               └──▶ U14b ─┘
  U2c ─▶ U12          U2d ─▶ U11
  U1 ──┐
  U2c ─┴──▶ U3c ──▶ U14   (verb-level conformance harness: red before the resolve migration turns it green)

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
        │         └──▶ U5            │    └──▶ U3b
        ├──▶ U6 ──┬──▶ U6d           │
        │         ├──▶ U6e          U14b ──▶ U4
        │         └──▶ U6b ──┬──▶ U6c
        └──▶ U7e             └──▶ U8c
  U1d ──▶ U6e
  U3b ─┐
  U4 ──┼──▶ U7 ──┬──▶ U7d ──▶ U7c
  U5 ──┘         └──▶ U8 ──▶ U16
  U6b ──▶ U7b        U6c ──▶ U7b        U6d ──▶ U7b        U6e ──▶ U7b
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

Edges declared, no cycles. The table below carries all **106** executable edges, grouped by source
task. Each row lists one dependent task and every prerequisite it declares, so the row count (41)
is the number of tasks that declare at least one dependency, not the edge count.

| Dependent | Unit | Prerequisites (`item_deps` rows) | Reason |
|---|---|---|---|
| `147.002-T` | U2 | `147.001-T` | The helper returns the typed error U1 declares. |
| `147.003-T` | U2b | `147.002-T` | Extends the same helper. |
| `147.004-T` | U2c | `147.002-T` | Extends the same helper. |
| `147.005-T` | U2d | `147.002-T`, `147.004-T` | Extends the same helper, and the derived-set refactor lands only after the duplicate rule makes the predicate feature-complete. |
| `147.006-T` | U3 | `147.004-T`, `147.037-T`, `147.016-T` | Calls the completed predicate; the guarded seam and its caller migration land first; the parity harness must be red before this gate. |
| `147.007-T` | U3b | `147.006-T`, `147.011-T`, `147.028-T`, `147.037-T`, `147.016-T` | Verification-only contract pin: the residual test asserts U6 flags the same file; inherits the context-member verdict; the seam migration is what makes the conformance claim true; harness order pin. |
| `147.008-T` | U4 | `147.004-T`, `147.028-T`, `147.037-T`, `147.016-T` | Calls the completed predicate; inherits the context-member verdict; seam first; harness first. |
| `147.009-T` | U5 | `147.008-T`, `147.016-T` | U5's paired row asserts the refusal U4 introduces; harness first. |
| `147.011-T` | U6 | `147.004-T`, `147.032-T`, `147.016-T` | The list verdict calls the completed predicate and publishes the structured remediation intent; harness first. |
| `147.012-T` | U6b | `147.011-T`, `147.030-T`, `147.038-T`, `147.032-T`, `147.016-T` | Both read surfaces must report the same field set; renders through the bounded raw projection; populates the carrier U15 declares; carries the intent; harness first. |
| `147.013-T` | U7 | `147.001-T`, `147.030-T`, `147.007-T`, `147.008-T`, `147.009-T`, `147.016-T` | The mapping layer matches U1's sentinel; `unknown_fields` renders through the bounded raw projection; MCP maps the sentinels U3b, U4, and U5 emit; harness first. |
| `147.014-T` | U7b | `147.012-T`, `147.022-T`, `147.023-T`, `147.043-T`, `147.016-T` | The read descriptions promise U6b's field as U6c projects it, must not promise U6d's unshipped filter exemption, and their universal `remediation_intent` sentence is only true after U6e populates the intent on every quarantine-candidate branch; harness first. |
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
| `147.037-T` | U14 | `147.036-T`, `147.042-T` | The resolve-verb caller migrates onto a seam that already enforces its preconditions, and the verb-level conformance harness U3c owns must be red before the migration turns it green. |
| `147.038-T` | U15 | `147.030-T`, `147.032-T` | The carrier's fields are the bounded projection set and the remediation intent. |
| `147.039-T` | U16 | `147.032-T`, `147.015-T`, `147.016-T` | The renderer consumes the structured intent and attaches to U8's refusal output; harness first. |
| `147.040-T` | U17 | `147.008-T`, `147.016-T` | The wrap correction lands beside the gate U4 installs in the same function; harness first. |
| `147.041-T` | U10c | `147.026-T`, `147.033-T`, `147.022-T`, `147.025-T`, `147.029-T`, `147.027-T` | Consumes U10b's workspace and verifies the routing rule, both read projections, the resolve routing, and the abandoned-resolve mapping at runtime. |
| `147.042-T` | U3c | `147.001-T`, `147.004-T` | The verb-level conformance harness matches U1's sentinel and typed error and asserts the completed top-level predicate; it must be red before U14 migrates the call site. |
| `147.043-T` | U6e | `147.011-T`, `147.032-T` | The remaining quarantine-candidate branches extend U6's branch structure and populate the carrier U1d declares. |
| `147.044-T` | U14b | `147.036-T` | The abandon-verb caller migrates onto a seam that already enforces its preconditions; it needs no resolve-verb harness. |

`147.032-T` (U1d) declares no prerequisites and is the second ready root. The cycle-20 additions do
not change the root set: `147.042-T` (U3c) depends on `147.001-T` and `147.004-T`, and `147.043-T`
(U6e) depends on `147.011-T` and `147.032-T`, so both are interior nodes.

**Suggested execution order**: U1, U1d, U1b, U1c, U2, U2b, U2c, U2d, U2e, U2g, U2h, U11, U12, U3c,
U13, U14, U14b, U2f, U15, U8b, U3, U6, U6e, U6d, U6b, U6c, U8c, U7b, U3b, U4, U17, U5, U7, U7e,
U7d, U7c, U8, U16, U9, U9b, U10, U10b, U10c.

Partition boundaries are the load-bearing part of that order: everything through U2h is partition 1;
U11 through U2f — including U3c — is partition 2; U15 and U8b are partition 3; U3 through U9b is
partition 4; U10 through U10c is partition 5. Inside a partition, units that share no edge may be
taken in any order. U1d is independent of everything in partition 1 and only has to land before
U15, U8b, U6, U6e, U6b, and U16. U3c must land before U14, which is what turns it green. U2f is
terminal by design — nothing depends on it, and it now waits on both caller migrations, U14 and
U14b; a blocked U2f does not block the release unit,
because the guarded seam rather than the enumeration is what enforces I1; when it is blocked it
must also be removed from `130-S` with `return_blocked`.

**Harness → declaration → behaviour monotonicity (cycle-20 invariant I4, restated cycle-31)**:
every unit whose delta changes behaviour has a failing harness that lands **before its own
production delta** and no earlier than the declarations that harness compiles against. Concretely:
units with their own red harness satisfy this internally (red harness step → green step, in that
order inside the task, with the harness taking the source-shape form whenever the delta introduces
the symbol it would otherwise name); `U13` is covered by `U12`; `U14`'s verb-level conformance
contract is covered by `U3c`; `U11`'s **behaviour** is covered by `U12` while its **declaration**
is gated by its own `TestU11_` source-shape harness; and `U1d` and `U15` — formerly the two
`declaration-only` units — carry their **own** failing source-shape harnesses (`TestU1d_*`,
`TestU15_*`) in addition to the downstream behaviour REDs in `U6`/`U6e`/`U6c`/`U16` and
`U6b`/`U8b`. No behaviour-changing unit in this plan lacks a transitive harness prerequisite, no
harness-exempt unit implements behaviour, and **no unit lands a production stub ahead of its own
harness**.

**Wave schedule (cycle-29, corrected cycles 33-34 and 37, P-002.6)**: shipment `130-S` has **44
explicit manifest members** in set `S`. The scheduler freezes
`M = { id in S : artifact_type(id) = task }`, so `count(M) = 43`; the one excluded non-task ID is
the covering `147-F` (`feature`), which release closure handles and no wave schedules. The
43-task / 106-edge DAG partitions into **18** dependency waves that schedule all 43 members of `M`
with **zero** stalls and **zero** compile-order violations (every dependency lands in a strictly
earlier wave than its dependent). Retired archived sibling `147.010-T` remains under parent
`147-F` but is absent from `S`; parentage is not membership, so neither it nor covering feature
`147-F` enters `M` or a snapshot. Every SQL snapshot filters on `artifact_type = 'task'` and an
exact safely bound/quoted frozen-`M` ID list, never `parent_id`, and must return exactly 43 distinct
IDs. Without SQL, Ship gets each of those 43 IDs directly and exactly once at every status;
`list --type task` is forbidden. Non-shipment mode requires the same closed explicit
`frozen_task_ids` set or halts `WAVE_MANIFEST_UNAVAILABLE`; it never enumerates all feature
children. Ship interleaves harness generation and implementation per wave rather than scaffolding
the shipment up front.

| Wave | Tasks | Wave | Tasks |
|---|---|---|---|
| 1 | `147.001-T` (U1), `147.032-T` (U1d) | 10 | `147.007-T` (U3b, exempt), `147.009-T` (U5), `147.040-T` (U17) |
| 2 | `147.002-T` (U2), `147.030-T` (U1b) | 11 | `147.013-T` (U7) |
| 3 | `147.003-T` (U2b), `147.004-T` (U2c), `147.031-T` (U1c), `147.038-T` (U15) | 12 | `147.015-T` (U8), `147.025-T` (U7d) |
| 4 | `147.005-T` (U2d), `147.016-T` (U8b), `147.020-T` (U2e), `147.028-T` (U2g), `147.042-T` (U3c) | 13 | `147.024-T` (U7c), `147.039-T` (U16) |
| 5 | `147.011-T` (U6), `147.029-T` (U7e), `147.033-T` (U2h), `147.034-T` (U11) | 14 | `147.017-T` (U9, exempt) |
| 6 | `147.012-T` (U6b), `147.023-T` (U6d), `147.035-T` (U12), `147.043-T` (U6e) | 15 | `147.018-T` (U9b, exempt) |
| 7 | `147.022-T` (U6c), `147.027-T` (U8c), `147.036-T` (U13, exempt) | 16 | `147.019-T` (U10, exempt) |
| 8 | `147.014-T` (U7b), `147.037-T` (U14), `147.044-T` (U14b) | 17 | `147.026-T` (U10b, exempt) |
| 9 | `147.006-T` (U3), `147.008-T` (U4), `147.021-T` (U2f, exempt) | 18 | `147.041-T` (U10c, exempt) |

**Persistent red deliverables and convergence scope (cycle-32, P-002.6).** Three members of this
schedule are tasks whose *declared deliverable is a red harness*: they complete red and a later
task turns them green. `147.042-T` / U3c and `147.016-T` / U8b both complete in **wave 4**, and
`147.035-T` / U12 completes in **wave 6**. Each carries a machine-readable
`red-deliverable-contract` block naming its `red_selector_command`, its `green_maker_tasks`, and
the wave at which the last of them lands:

| Red deliverable | Wave | Declared green-maker(s) | Closes at wave |
|---|---|---|---|
| `147.035-T` (U12) | 6 | `147.036-T` (U13) | 7 |
| `147.042-T` (U3c) | 4 | `147.037-T` (U14) | 8 |
| `147.016-T` (U8b) | 4 | `147.006-T`, `147.007-T`, `147.008-T`, `147.009-T`, `147.011-T`, `147.012-T`, `147.013-T`, `147.014-T`, `147.015-T`, `147.022-T`, `147.024-T`, `147.025-T`, `147.027-T` | 13 |

`open_red_deliverables_k` is therefore non-empty from the wave-4 convergence gate through the
wave-12 one. Wave 4 **advances** with U8b and U3c legitimately red — the convergence gate runs the
repo-wide compile check, `go vet`, lint, format, every member's declared scoped command, and the
`red_selector_command` of every entry still open in the recomputed set (each re-confirmed RED,
including entries carried in from earlier waves), and **defers** the unfiltered `go test ./...`
with an explicit `FULL_SUITE_DEFERRED` record. The
unfiltered suite runs at the convergence gate of waves 1–3 (before any red deliverable lands) and
again from **wave 13**, the wave at which U8b's last green-maker (`147.024-T` / U7c) completes and
empties the set, through wave 18 and final closure. A green-maker `archived` rather than `done`
does not close its entry; an entry still open past its declared closing wave halts with
`WAVE_OPEN_RED_UNCLOSED`. The 1.19.0 claim that the tolerated-red set is empty at every convergence
gate was false from wave 4 onward and is corrected.

**Optional green-regression commands (cycle-33).** The only declaration form is the canonical
`green-regression-contract` JSON body block in P-002.6. An absent block means exactly
`green_regression_cmds: []`; it does not authorize Ship or `build-feature` to choose a package or
infer a command from prose. No task in `M` currently needs the block, so all 42 arrays are empty.

**The wave schedule is what dissolves the U15/U8b deadlock** PR #377 review raised on
`.ship.agent.md:254`. U8b (`147.016-T`) cannot compile until U15 (`147.038-T`) lands, but U15
depends on U1b (`147.030-T`), which is harness-required and so cannot be implemented before its own
harness exists. One up-front pass has no valid order. Under waves the chain is U1d (wave 1) → U1b
(wave 2) → U15 (wave 3) → U8b (wave 4): each harness is scaffolded only after every declaration it
compiles against is `done`. No early-execution pass, no exemption, and no waiver is required.
Every `covered-by` owner likewise precedes its dependent — U12 lands in wave 6 and U13 in wave 7 —
so the claim-time `EXEMPT_OWNER_NOT_RED` condition is satisfiable by construction. A cycle injected
into the graph as a negative control halts the scheduler at `WAVE_NO_PROGRESS` after 9 waves rather
than looping.

**Measured topology (cycle 20, count semantics corrected cycle 33, recounted cycle 37,
`backlogit --cwd . query` after `backlogit --cwd . sync`)**: shipment `130-S` has 44 explicit
members; task-type wave set `M` has 43 queued tasks; excluded non-task members are exactly
`147-F` (`feature`). `M` has **106** queued-to-queued executable edges and ready set exactly
`{147.001-T, 147.032-T}` (two roots — `U1`, the typed-error declaration, and `U1d`, the
remediation-intent declaration). **Historical total edges: 107** — the 106 executable edges plus
the one archived edge `147.010-T -> 147.009-T` retained in the archived U5b artifact. The
historical count is deliberately kept distinguishable from the executable topology: only the 106
queued-to-queued edges govern execution order, and an agent must never schedule against the
historical figure. The graph is verified acyclic by Kahn topological sort — all 43 nodes ordered
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
| CLI reference drift blocks the PR | Low | U9 regenerates `gen-docs` output and runs `go run ./cmd/backlogit --cwd . docs lint` before handoff. |
| `CreateCheckpoint` same-second filename collision silently overwrites (adjacent, **out of scope**) | Medium | Surfaced during the entry-point audit (I1). Not fixed here and not stashed, to hold the bounded scope. Recorded in Plan Hardening as a named follow-up. |

## Constitution Check

| Principle | Verdict | Notes |
|---|---|---|
| I. Safety-First Go | **pass** | All production changes are Go; no `unsafe`. New wraps use multi-`%w` so both sentinels resolve. **Cycle-17 change**: the pre-existing `%v` validation wrap in `AbandonCheckpoint` (`internal/core/checkpoint_disposition.go:~70-73`) is **fixed** by U17 rather than recorded as a deviation. The cycle-16 gate ruled the deviation unavailable: Principle I is not satisfiable by documenting a departure from it, and the "unrelated shipped contract" justification lapses once U4 and U14 edit that same function. |
| II. Test-First Development (NON-NEGOTIABLE) | **pass** | Every unit runs the single **two-step** lifecycle declared at the head of Implementation Units (cycle-31): a **red harness step that lands first** — only functions that **fail on assertions**, compiling against the pre-delta tree, taking the **source-shape** (`go/parser` + `go/ast`) form whenever the unit's delta introduces the symbol the harness would otherwise name — then a green step that lands the production delta (declaration **and** behaviour) together with any already-green regression guards as `TestU<unit>Guard_` functions. Expected red is stated per unit, and cycle 16 pins the exact per-unit `-run` selector and `-count=1` invocation that observes it, so "red" is verifiable rather than asserted. Cycle 20 removed the "declared regression guards inside the harness" device on fourteen units and the narrowed-red-selector device on U2g and U2h: P-004's precondition is expected failure markers **for every test function**, and it gates the harness commit rather than the selector or the prose. Cycle 20 withdrew the cycle-8 rule that every unit needs at least one failing assertion, on the grounds that a test failing only because a symbol does not exist is a build error and a test passing the moment the declared shape lands was never red. **Cycle 29 reinstated that rule for declaration units in corrected form** after PR #377 review (`workflow-policies.md:75`) held that the resulting `declaration-only` exemption admitted observable production surface — a serialized `CheckpointSummary` field, an exported `GetCheckpointResult` wrapper — with no observed failing test, which is a carve-out from a NON-NEGOTIABLE principle. **Cycle 31 closes the remaining half of that carve-out**: the cycle-20 "declaration step" — a production stub landed *ahead of* the unit's own harness so the package would compile — is **withdrawn plan-wide**. It was the same violation by a different route, and it was still live on U1, U1b, U1c, U2d, and most severely U11, whose stub was specified to carry real read/parse/mutate/marshal/atomic-replace behaviour. The corrected order is **harness first, declaration second**, with no stub in front of any harness; a seam or declaration whose body would absorb real behaviour is split into declaration → behaviour harness → implementation (the U11 → U12 → U13 shape), so U11's declaration is gated by its own `TestU11_` source-shape red and its behaviour by U12's contract harness. U1d and U15 are not exempt: each carries a **source-shape** harness that parses its own production file with `go/parser` and asserts the declared shape via `go/ast`, so it compiles before the declaration exists (no build error) and fails on an assertion (a genuine red), verified at HEAD `6a822ceb`. Both units' downstream behaviour REDs (U6/U6e/U6c/U16 for U1d; U6b/U8b for U15) are retained on top of that. **No Principle II deviation is recorded for any declaration unit** — the principle is satisfied outright. Test-first is preserved for behaviour with exactly one explicitly allowed, edge-backed carve-out: every behaviour-changing unit is backed by a failing harness observed red before its implementation lands, carried on the unit itself except for **U13**, whose harness is owned by its declared prerequisite **U12** under the `covered-by` class (invariant I4). No other unit may claim that shape. U2d owns a real production delta with a compiling-but-failing source-shape harness case. U8b lands in partition 3 against the U15/U1b/U1d/U2 declarations and fails on assertion behaviour before any partition-4 implementation lands; the cycle-15/16 batch-harness-generation framing is withdrawn because it made the red gate depend on implementer sequencing rather than the dependency graph. U5's withdrawn state-conflict rows never contributed to its red gate. Cycle-10 retired U5b, whose production delta contradicted the decision's scope boundary; cycle-16 corrected U7e's expected-red statement; and cycle-20 moved U3b's only red claim into the new harness unit U3c, because the resolve-verb conformance contract is delivered by U14's seam migration rather than by U3b. |
| III. Workspace Isolation and Security Boundaries | **pass** | No path handling changes. `ResolveDispositionTarget`, `ensurePathContained`, and `validateCheckpointFilename` are untouched. The new gates operate on already-read bytes. `Fields` carries key **paths** only, never values, so a refusal cannot leak checkpoint content. No secrets introduced. |
| IV. CLI Workspace Containment (NON-NEGOTIABLE) | **pass** | All edits are inside the repository tree. U10's scratch workspace is pinned to `.copilot/scratch/checkpoint-verification/` **inside** the working tree — never `%TEMP%`, never a sibling or parent — and the path is asserted to be repo-root-relative before any write. |
| V. Structured Observability | **deviation (documented)** | Refusals are typed and machine-readable: `unknown_fields` (raw paths plus structural truncation scalars) on MCP, named keys on CLI, `NeedsQuarantine` + a structured `RemediationIntent` on list and get. The audit-before-mutation ordering is **preserved** (not strengthened — the ordering already existed; U4 only moves the new gate to sit ahead of it). **Deviation**: no new counter, log line, or telemetry event is emitted when a refusal occurs, so a spike in refusals is observable only through agent-visible errors. Accepted for this scope; recorded as a follow-up. |
| VI. Single Responsibility | **pass** | No new dependencies. The helper reuses `decodeTopLevelEntries`, `isFoldKeyIn`, `modeledJSONTagKeys`, and `unknownNestedProgressKeys` already present in `internal/events`. |
| VII. Destructive Command Approval (NON-NEGOTIABLE) | **pass (conditional — see the approval condition below)** | Cycle 16 withdraws the two cycle-15 "documented deviations" against this principle. A NON-NEGOTIABLE principle cannot be satisfied by documenting a departure from it; either every destructive action is approval-gated or the plan fails the check. Both are now gated. **A4c is the single operative contract for every checkpoint-file-moving or checkpoint-file-overwriting command** — `quarantine`, `abandon`, `resolve`, and any operator-performed copy-back — and requires explicit operator approval **immediately before each execution batch**, never once at plan time. **A4b** (scratch teardown) requires its own approval immediately before execution. **A4d** confines live post-merge observation to the read verbs or a byte-copy mirror. **A5** (mutating the live corpus) stays forbidden without separate authorization. The change itself remains net **anti**-destructive: it removes a silent data-destruction path. **Condition**: this verdict is `pass` only while every live quarantine, resolve, abandon, or operator repair batch carries a recorded approval taken immediately before execution. An unapproved batch is a P-005 violation and a halt, not a documented exception. |
| VIII. Explicit Safety Modes | **pass** | Work executes under **freeze-scope**. Declared boundary: `internal/errors/`, `internal/events/`, `internal/core/`, `internal/mcp/`, `internal/cli/`, `docs/design-docs/checkpoint-administrative-disposition.md`, `docs/cli-reference/backlogit_checkpoint_*.md`, `.github/instructions/backlogit.instructions.md`, `.autoharness/backlog-registry.yaml`, `docs/closure/`, and `.copilot/scratch/checkpoint-verification/`. The nine live checkpoint files are explicitly **outside** the mutation boundary. |
| IX. Git-Friendly Persistence | **pass** | Checkpoint JSON stays human-readable; `jsonutil.MarshalReadable` and the atomic-write helpers are unchanged. |
| X. Agent Context Efficiency | **pass** | Refusals carry structured field lists so an agent does not parse message text to learn which keys were rejected. U6b closes the `list` / `get` disagreement that would otherwise cost an agent an extra round trip and a wrong verb. |
| XI. Merge Commit History Preservation (NON-NEGOTIABLE) | **pass** | Ships through a merge commit. Squash and rebase merge are forbidden and must be verified before merge. |

### Documented deviations

| Principle | Deviation | Justification | Simpler alternative rejected |
|---|---|---|---|
| V. Structured Observability | No refusal counter, log, or telemetry event. | The refusal is already agent-visible and typed; adding a telemetry surface pulls `internal/telemetry` into a freeze-scoped change and widens the blast radius past the defect. | "Emit a telemetry event per refusal" — rejected: nine known refusals on day one would immediately produce noise with no consumer defined. |
| II. Test-First Development | **Eight** units scaffold **zero** harness test functions and are recorded `harness-exempt`. | P-004's precondition quantifies over every scaffolded harness test function; when a unit scaffolds none, it holds vacuously. The exempt set is closed and enumerated below, each entry names where its behaviour's failing harness lives, and exactly one member changes behaviour — U13, the explicitly allowed `covered-by` carve-out whose red harness is owned by its prerequisite U12. **No declaration unit is in this set** (cycle 29). | "Give every unit a failing assertion" — rejected in cycle 20 *for docs and verification units*, where the only available assertion would be vacuous. It is **not** rejected for declaration units: cycle 29 reinstates a genuine failing assertion for U1d and U15 in source-shape form, which is neither a build error nor a tautology. Manufacturing *behaviour* purely to create a red assertion remains rejected, as in cycle 10 when U5b's invented delta reopened a scoped-out decision. |

**The `harness-exempt` set is closed and has exactly eight members (cycle 29).** No unit outside
this table may claim the exemption, and adding one requires a plan amendment. Cycle 20 enumerated
ten; cycle 29 removed **U1d (`147.032-T`)** and **U15 (`147.038-T`)** when the `declaration-only`
class was withdrawn, and both are now normal harness-required units carrying source-shape REDs.

| Unit | Task | Class | Where the failing harness lives |
|---|---|---|---|
| U13 | `147.036-T` | `covered-by` (`harness_owner: 147.035-T`) | U12 (`147.035-T`) — three failing seam-contract functions |
| U3b | `147.007-T` | `verification-only` | U3c (`147.042-T`), turned green by U14 (`147.037-T`) |
| U2f | `147.021-T` | `verification-only` | U12 (`147.035-T`) — I1 is enforced by the seam, not the enumeration |
| U9 | `147.017-T` | `docs-only` | not applicable — no behaviour |
| U9b | `147.018-T` | `docs-only` | not applicable — no behaviour |
| U10 | `147.019-T` | `verification-only` | not applicable — runtime evidence |
| U10b | `147.026-T` | `verification-only` | not applicable — runtime evidence |
| U10c | `147.041-T` | `verification-only` | not applicable — runtime evidence |

**Units removed from the exempt set in cycle 29** — both now carry `harness-ready` and a red
source-shape harness, and neither carries a `harness-exemption-contract` block or an
`EXEMPT_VERIFY_OK` gate any more:

| Unit | Task | Former class | Replacement red gate | Baseline probe at HEAD `6a822ceb` |
|---|---|---|---|---|
| U1d | `147.032-T` | `declaration-only` | `go test -count=1 -run '^TestU1d_' ./internal/events` | compiles (`go vet` exit 0; `-run='^$'` reports `[no tests to run]`), 2/2 assertion FAILs |
| U15 | `147.038-T` | `declaration-only` | `go test -count=1 -run '^TestU15_' ./internal/events` | compiles (same probe), 2/2 assertion FAILs |

**Every exempt unit carries an executable, must-fail-before-deliverable gate (cycle 23).** The
exemption removes the *scaffolded red harness*, not the *observed failure*. Each of the eight tasks
above carries a canonical `harness-exemption-contract` body block with five keys in identical
order — `harness_exemption_class`, `harness_exemption_reason`, `harness_owner`,
`exempt_verification_command`, `exempt_precondition` — plus `harness_owner_command` on U13 alone.
Every command was executed at HEAD `e8b974e` and observed **failing**, so no unit's gate is
vacuous:

| Unit | Gate shape | Why it cannot pass before the deliverable |
|---|---|---|
| U13 | seam-file precondition-set probe, then 3 named `--- PASS: TestU12_` lines | the seam file does not exist and U12's selector returns `[no tests to run]` |
| U3b | named test file exists, then 2 named `--- PASS: TestU3bGuard_` lines | the file does not exist; the bare selector exits 0 with `[no tests to run]` |
| U2f | named test file exists, then 2 named `--- PASS: TestU2fGuard_` lines | same |
| U9 | four required design-doc strings present, `disjoint by design` absent, then `docs lint` | `docs lint` alone already reports `violation_count: 0` and can never fail |
| U9b | six required instruction-file markers present, then `docs lint` | same, and this is the hard merge gate where a vacuous pass is most expensive |
| U10 | ≥5 `evidence_row: unit=U10 …` records plus 2 declared scalars | the closure artifact does not exist; an empty or partial artifact still fails |
| U10b | ≥5 `evidence_row: unit=U10b …` records plus 3 declared scalars | same |
| U10c | ≥3 `evidence_row: unit=U10c …` records plus 2 declared scalars | same |

**The `EXEMPT_VERIFY_OK:{task_id}` marker is required on `exempt_verification_command` only.**
`harness_owner_command` — which only U13 carries — is deliberately unmarked and is validated by its
named `--- PASS: TestU12_` count against U12's scaffolded harness (`workflow-policies.md` P-002.3,
Loop-command-versus-gate-command). Requiring a marker on it would mean editing U12's harness to
emit a string on U13's behalf, which every other rule in this contract forbids.

**Objective class boundary (P-002.4).** `covered-by` is the only exempt class that may modify
production behaviour, and U13 is this plan's only member of it. The check is mechanical: at the
completion gate the task's changed-file set must be a subset of its class delta surface —
`docs-only` changes zero `*.go` files, and `verification-only` changes zero non-test `*.go` files
**and zero repository-configuration files** (no `.gitignore`, no CI or build config — cycle 29;
`verification-only` is not a repository-hygiene class), and `covered-by` changes no `*_test.go`
file at all. Anything outside the surface is a halt (`EXEMPT_DELTA_EXCEEDS_CLASS`, or
`EXEMPT_BEHAVIOR_NO_OWNER` when a no-behaviour class produces production behaviour). An **empty**
changed-file set is also a halt, never a trivial pass. Fail closed on any unclassifiable delta.

**Ship ready-selection contract (cycle 21 adapter, generalized into global policy in cycle 22).**
Cycle 21 recorded this rule as a shipment-local adapter because the repository-wide ready-queue
policy (`.github/policies/workflow-policies.md`, `.github/agents/.ship.agent.md`) filtered strictly
on `harness-ready` and had no vocabulary for `harness-exempt`; read literally, that global policy
would have told Ship's harness-architect / build-feature selection to scaffold a red harness for
all eight units in the table above, contradicting the closed-set exemption this plan enumerates.
Cycle 22 closed that gap in the global policy itself — **P-002.1 (Harness-Exempt Alternative
Satisfaction, fail-closed)** and **P-002.2 (Harness-Exempt Halt Taxonomy)** — so this plan is now a
*conforming consumer* of a general contract rather than the carrier of a local exception:

* **Rule.** A `147.0xx-T` task is harness-satisfied — and therefore eligible for Ship's `queued`
  ready-selection and build-feature dispatch — when its `labels` field contains `harness-ready`,
  **or** contains `harness-exempt` and passes P-002.1 evaluation. Equivalent selection query,
  applied only after P-002.1 has validated each exempt task:
  `SELECT id FROM items WHERE parent_id = '147-F' AND status = 'queued' AND (labels LIKE
  '%harness-ready%' OR labels LIKE '%harness-exempt%')`.
* **Fail-closed evaluation.** The label alone is not admission. Each of the eight tasks declares a
  class from the closed P-002.1 vocabulary (`docs-only`, `verification-only`,
  `covered-by`, with the owner in `harness_owner`), a one-line reason, and membership in the closed set enumerated above,
  which is this plan's declared exempt contract. An unrecognized class, a task not in that closed
  set, or a behaviour-changing task without a valid predecessor harness owner is a **halt** and a
  reported P-002 gap — never a silent skip and never a trigger to scaffold a substitute harness.
* **Relationship to Principle II.** This is a P-002 *enforcement* contract, not a waiver of
  Constitution Principle II (Test-First Development), which is satisfied vacuously per the
  Documented deviations row above.
* **Behaviour requires red evidence; U13 is the single edge-backed carve-out.** Every
  behaviour-changing unit in this plan is backed by a failing harness that was observed red before
  its implementation lands. Seven of the eight exempt units carry no behaviour at all
  (`docs-only`, `verification-only`) and so owe no red of their own. Exactly
  one exempt unit does change behaviour: **U13 / `147.036-T`**, and it is the explicit, allowed
  `covered-by` carve-out — its failing harness is owned by its prerequisite **U12 /
  `147.035-T`** (three failing seam-contract functions), which is a declared dependency, must carry
  `harness-ready` at claim time — eligible only after harness generation runs — rather than an
  exemption, and lands red before U13 builds. No other unit may claim that shape: any further
  behaviour-changing unit needs `harness-ready` on itself. Invariant I4 (declaration → harness →
  implementation monotonicity) is the structural guarantee that the edge exists.
* **Application.** Ship (or its harness-architect / build-feature skill) MUST treat all eight tasks
  in the table above as already harness-satisfied once P-002.1 evaluation passes, and MUST NOT
  scaffold a red harness for any of them; it still schedules and implements each in dependency
  order like any other queued task. For U13, "in dependency order" additionally means after U12's
  red evidence is confirmed and its harness commit has landed.
* **Two gates, not one (cycle 23).** P-002.1 evaluation is split, because the cycle-22 form
  checked U12's red evidence at Ship Step 2 — which runs *before* harness generation — making U13
  unsatisfiable by construction and deadlocking this shipment at its own gate. **Static intake**
  (Ship Step 2a, harness-architect Step 1a) validates only what is knowable pre-scaffolding:
  fields, class, reason, closed-set membership, U12's existence / dependency edge / non-exempt
  type, and the declared commands. **Claim time** (Ship Step 4.1a) re-evaluates U12's
  `harness-ready` label, `Compilation: PASS` / `Red Phase: CONFIRMED`, and landed harness commit —
  and solely owns `EXEMPT_OWNER_NOT_RED`. `harness-architect` must scaffold U12 as an ordinary
  harness target and must never raise that code. Queue admission is not build admission.
* **Observed failure is still owed (cycle 23).** Each exempt unit runs its
  `exempt_verification_command` once before any work and MUST see it fail; a pre-work success is
  `EXEMPT_FALSE_GREEN` and a halt. `go test -run <selector>` exiting 0 with `[no tests to run]` is
  a **failure**, not a pass, so a selector-only command is never sufficient on its own.
* **Plugin bundle parity is out of scope (pre-existing, cycle-24 confirmatory review).** This
  contract governs `.github/policies/workflow-policies.md`, `.github/agents/.ship.agent.md`,
  `.github/skills/build-feature/SKILL.md`, and `.github/skills/harness-architect/SKILL.md` only.
  It is deliberately **not** propagated to `plugin/agents/ship.agent.md` or
  `plugin/skills/build-feature/SKILL.md`, and this is a pre-existing scope boundary that cycles
  22-24 did not introduce. Per
  `docs/decisions/2026-07-13-plugin-bundle-structural-verification-decision.md` (101-F), `plugin/`
  (the distributable product bundle) and `.github/` (backlogit's self-hosting harness) are not
  byte-identical or content-synchronized; `TestPluginBundleStructurallyValid`
  (`tests/integration/plugin_manifest_test.go`) validates the bundle's file set and frontmatter
  shape only, never cross-tree content parity. Direct inspection confirms
  `plugin/agents/ship.agent.md` and `plugin/skills/build-feature/SKILL.md` carry no P-002
  vocabulary at all — not merely missing the cycle 22-24 additions. Recorded as a dated note in
  `.autoharness/drift-ignore` and as follow-up stash `633818E1` (kind `task`, priority `low`); the
  plugin bundle is not edited by this or any prior cycle of this work.

**Principle I carries no deviation (cycle 17).** The row recorded here through cycle 16 —
`AbandonCheckpoint`'s `%v` validation wrap left in place — is withdrawn. The cycle-16 gate found
(H7) that the justification depended on the wrap living in code this plan does not touch, which
stopped being true when U4 placed a gate in that function and U14b moved its write onto the seam.
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

### Stage handoff checkpoint provenance

Before Stage hands shipment `130-S` to Ship, every checkpoint write or state update for this
initiative MUST use a validated backlogit checkpoint operation. New Stage checkpoints MUST be
created through the registered operation or workspace-bound CLI with `agent: stage`, then loaded
through `checkpoint get` so `ParseCheckpoint` and `ValidateCheckpoint` both succeed before the
checkpoint is cited as continuity evidence.

A direct edit to a checkpoint file is exceptional. It MUST be followed immediately by the complete
current-source checkpoint corpus gate in the final Plan Review section. Until that gate passes,
the direct edit is not persisted state and MUST NOT be used for recovery, review evidence, or the
Stage-to-Ship handoff. A failure blocks handoff. This is a shipment-local provenance rule; it
changes no global agent, policy, or production-code contract.

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
| `docs/compound/best-practices/windows-safe-atomic-rename-goos-gate-2026-04-23.md` (cycle 18) | Directly corroborates the pre-existing `os.Remove(dst)`-before-`os.Rename` hazard this plan already names in U9b item 7 and the Risks table's "Windows atomic-write regression" row: gating pre-remove on `runtime.GOOS == "windows"` is the shipped pattern the guarded seam (U11-U14) and `moveNoReplace` must not regress. |
| `docs/compound/best-practices/crash-safe-delete-rename-rollback-go-2026-04-23.md` (cycle 18) | Corroborates U10/U10b's no-clobber-and-rollback posture for the archive payload move: on a failed post-rename step, roll back to a discoverable canonical path rather than leaving evidence stranded under a temp name. |
| `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` (cycle 18) | Corroborates U9's `gen-docs`-then-lint sequencing: CLI reference docs must be regenerated from the Cobra command `Long` field, not hand-edited, before the drift check and `go run ./cmd/backlogit --cwd . docs lint` are run. |

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
| A4 | Create and seed the scratch verification workspace — `mkdir`, byte-copy fixtures in, build the verify binary, and run the **read** verbs (`list`, `get`, `version`) against it | `.copilot/scratch/checkpoint-verification/` only | local file creation, no checkpoint mutation | moderate | not required — creation and reads only; **this row grants no authority to run a disposition verb**. Cycle 15's wording ("run disposition verbs … not required") contradicted A4c and is removed: any command that moves or overwrites a checkpoint file, in scratch or anywhere else, is A4c | discard the scratch contents | planned |
| A4c | Execute a **disposition command that overwrites or moves a checkpoint file** — `checkpoint quarantine` (moves the file into `archive/checkpoints/` and writes a sidecar), `checkpoint abandon` (rewrites the document in place), `checkpoint resolve` (rewrites the document in place), and any operator-performed archive copy-back | scratch fixtures, scratch mirror, live corpus, and any archive/sidecar they create | file move / in-place overwrite | **destructive** | **required, and A4c is the sole operative contract for this class** — obtain explicit operator approval **immediately before each execution batch**, not once at plan time (Constitution VII). Approval covers the named files in the named directory only; a new directory or a new filename set needs a fresh approval. The batch request MUST carry the U10 execution contract: `--cwd` binding with bare filenames, per-file filename/hash/state/destination display, a pre-run preimage byte copy, the **operation-class-appropriate** destination precondition (cycle-17: absent-destination and no-clobber for an **archive move**; preimage plus a post-step SHA comparison for an **in-place rewrite**, where the destination is necessarily present and absent-destination must not be asserted; a declared intentional-collision row expects a refusal), and fail-closed halting | quarantine is reversible in principle by operator copy-back from `archive/checkpoints/`; an in-place rewrite is reversible only from the pre-run byte copy, so a pre-run byte copy of every mutating fixture is a **precondition** of the approval request | planned |
| A4b | Tear down the scratch verification workspace after closure (**owned by U10c** since cycle 17, only after its three rows pass — U10 and U10b hand the workspace over intact) | `.copilot/scratch/checkpoint-verification/` | directory deletion | **destructive** | **required** (Constitution VII) — obtained immediately before execution | none needed — contents are reproducible from the plan, and the durable evidence lives in the tracked closure artifact rather than in the scratch directory | planned |
| A4d | Post-merge observation of the live workspace | `.backlogit/checkpoints/` | **read-only** — `checkpoint list` and `checkpoint get` only | low | not required **while read-only**; any mutating verb against the live corpus leaves this row and becomes A4c or A5 | n/a | planned |
| A5 | Mutate the nine live legacy checkpoints or the stale `129-S` checkpoint | `.backlogit/checkpoints/` | destructive, irreversible | **destructive** | **FORBIDDEN in this work.** Out of scope; requires explicit operator approval in a separate unit of work. | n/a | **abandoned** |

**Live post-merge observation is read-only or mirror-based (cycle 15, reinforced cycle 16).** The
closure step "run one Stage and one Ship session-start recovery against the live workspace" MUST
NOT invoke `resolve`, `abandon`, or `quarantine` against `.backlogit/checkpoints/`. Session-start
recovery protocols do call `resolve` on leftover checkpoints, so the observation is performed either
(a) with the read verbs only, reading `needs_quarantine`, `conforming`, and `non_conforming_fields`
from `checkpoint list` / `checkpoint get`, or (b) against a byte-copy mirror under `.copilot/scratch/`,
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
  `.copilot/scratch/checkpoint-verification/` (Constitution IV; never `%TEMP%`), assert the resolved
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
  `.copilot/scratch/checkpoint-verification/mirror/.backlogit/checkpoints/` (workspace root:
  `.copilot/scratch/checkpoint-verification/mirror/`), a byte-copy of `.backlogit/checkpoints/`; all
  sweep CLI invocations use `--cwd .copilot/scratch/checkpoint-verification/mirror` with bare filename
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
  (`checkpoint list`, `checkpoint get`) or against a byte-copy mirror under `.copilot/scratch/`, per
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
> cycle 15, cycle 16, cycle 17, cycle 18, cycle 20, cycle 21, or cycle 24. The current gate state is
> the **last** `## Plan Review` record at the end of this document (`cycle: 24`, `decision:
> ADVISORY`).

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
cycle-16, cycle-17, cycle-18, cycle-20, cycle-21, and cycle-24 gate records that follow.** This
record remains the authoritative history of the cycle-14 dispatch and its findings; the current
gate state is `cycle: 24`, `decision: ADVISORY`. It supersedes the earlier `## Plan Review` record
in this document, which is scoped to cycles 1-13 and does **not** cover the cycle-14 dispatch. The
prior PASS is retained as history and must not be read as clearance for the plan in its cycle-14
state.

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

**This record is superseded by the cycle-16, cycle-17, cycle-18, cycle-20, cycle-21, and cycle-24
gate records that follow.** It supersedes both earlier `## Plan Review` records: the cycles 1-13
`PASS` and the `cycle: 14` `FAIL`. Neither may be read as clearance for the plan in its cycle-15
state. The cycle-16 remediation appendix beneath this record documents what changed in response;
it did **not** clear the gate. The required fresh, independent cycle-16 review follows later in
this document and also returned `FAIL`; it was in turn superseded by cycle 17 (`FAIL`), cycle 18
(`ADVISORY`), cycle 20 (`FAIL`), cycle 21 (`FAIL`), and then cycle 24 (`ADVISORY`, the current gate
state).

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

**This record was the current gate state through cycle 16; it is superseded by the `cycle: 17`
`FAIL` record, then the `cycle: 18` `ADVISORY` record, then the `cycle: 20` `FAIL` record, then the
`cycle: 21` `FAIL` record, and finally the `cycle: 24` `ADVISORY` record at the end of this
document, the current gate state.** It supersedes the cycles 1-13 `PASS`, the `cycle: 14` `FAIL`,
and the `cycle: 15` `FAIL`. None of the three earlier records may be read as clearance for the plan
in its cycle-16 state.

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
That appendix is remediation evidence, **not** a gate outcome: this `cycle: 16` record was the
current gate state at `decision: FAIL` until the fresh, independent `cycle: 17` plan review below
was dispatched against the decomposed plan, and it is now superseded in turn by the `cycle: 18`
`ADVISORY` record, the `cycle: 20` `FAIL` record, the `cycle: 21` `FAIL` record, and finally the
`cycle: 24` `ADVISORY` record at the
end of this document, the current
gate state. All eight blockers (1 P0, 7
P1) are dispositioned as closed in the appendix, and the decomposition passed both the cycle-17 and
cycle-18 reviews.

### PR #377 plan remediation, cycle 17 — formal decomposition

This appendix is **historical evidence of the cycle-17 remediation pass**. It does not override the
normative sections above; where this appendix and a normative section disagree, the normative
section governs. The `cycle: 16` `## Plan Review` record was the **current gate state** at the time
this appendix was written; it was `decision: FAIL`, and nothing in this appendix cleared it on its
own. It has since been superseded by the `cycle: 17` `FAIL` record, the `cycle: 18`
`ADVISORY` record, the `cycle: 20` `FAIL` record, the `cycle: 21` `FAIL` record, and the `cycle: 24`
`ADVISORY` record at the end of
this document, the current gate
state. Cycle 17 executed the
`restage_recommendation: formal-decomposition` that gate required; the result passed a fresh,
independent plan-review gate, as the `cycle: 17` record below records, and cycle 18 confirms the
topology and closes the remaining synchronization gaps.

**Method.** Unit-by-unit patching was rejected by the cycle-16 gate. The remaining work was
re-partitioned into the five DAG partitions the gate named, each independently reviewable, and the
plan's normative sections — Requirements Trace, Implementation Units, Dependency Graph, Decisions,
Risks, Constitution Check, Runtime Verification and Closure, gate sequence, and the I1/I2 invariants
— were rewritten against that structure rather than annotated.

#### Blocker disposition

| Gate ID | Severity | Cycle-17 disposition |
|---|---|---|
| H1 | P0 | **Closed.** `internal/events` no longer emits any command string for the new conformance branch. **U1d** declares a structured, non-executable `RemediationIntent` (`verb`, `target_filename`, `requires_approval`, `approval_class`, `reason`); U6, U6b, and U6c publish it; **U16** is the sole surface that renders an operator command, always with an explicit `--cwd` bound to the resolved storage root, a bare filename, and the A4c approval / preimage / no-clobber preamble, and refuses to render at all when quoting would be needed. U8 no longer asserts a runnable command; U9b gains a normative rule (item 9) forbidding executable remediation, repair, restore, or sweep text and an acceptance grep. The shipped `RemediationCommand` field is deprecated in place, not silently redefined; removal is stash `F350503F`. |
| H2 | P1 | **Closed.** Partition 3 exists for this: **U15** declares `CheckpointReadResult` / `GetCheckpointResult` ahead of every behavioural unit, and **U8b** is rewritten as a partition-3 harness whose prerequisites are declarations only (U1, U1b, U1d, U2, U15). Eighteen partition-4 units now depend on U8b instead of the reverse. The already-green schema-invalid `get` assertions were reclassified as declared regression guards and removed from U8b's red gate; the "batch harness generation phase" framing is withdrawn. |
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

**This record was the current gate state through cycle 17; it is superseded by the `cycle: 18`
`ADVISORY` record, then the `cycle: 20` `FAIL` record, then the `cycle: 21` `FAIL` record, and
finally the `cycle: 24` `ADVISORY` record at the end of this document, the current gate state.** It
supersedes the
`cycle: 16` `FAIL` record for gate-decision purposes; cycle 16 and every earlier record remain the
historical trace of how the plan reached its present shape. The "PR #377 plan remediation, cycle 17
— formal decomposition" appendix above is remediation evidence, not a gate outcome, and this record
supersedes it as *evidence of closure* wherever a finding below identifies a gap between that
appendix's disposition claims and the plan's actual normative text or the live backlog state.

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

<!-- copilot-review-remediation: pr-377-cycle-18 -->

## Plan Review

cycle: 18

**SUPERSEDED by cycle 20, in turn superseded by cycle 21, in turn superseded by cycle 24.** This
record is no longer the current gate state. It was written before the cycle-20 Copilot review of
the current head `3bcff086` on PR #377, which returned 20 unresolved threads — 17 P1 and 3 P2 —
against artifacts this record had declared clean. Its `decision: ADVISORY`, its `push_allowed:
yes`, and its `operator_authorization: approved` are all withdrawn for the purpose of gate state.
The current gate state is the **cycle 24** record at the end of this document. Read this record as
history only.

dispatch_mode: single-agent-declared-degradation

TOOL_DEGRADED: reviewer-subagent-dispatch

decision: ADVISORY (superseded — see cycle 20, then cycle 21, then cycle 24)

operator_authorization: approved (superseded — see cycle 20, then cycle 21, then cycle 24)

severity counts: P0=0, P1=0, P2=1, P3=5

push_allowed: yes (superseded — see cycle 20, then cycle 21, then cycle 24)

**This record was the current gate state through cycle 18; it is superseded by the `cycle: 20`
`FAIL` record, then the `cycle: 21` `FAIL` record, and finally the `cycle: 24` `ADVISORY` record at
the end of this document, the current gate state.** It supersedes the `cycle: 17`
`FAIL` record immediately above for gate-decision purposes; cycle 17 and every earlier record remain
the historical trace of how the plan reached its present shape.

### Authorization basis

`operator_authorization: approved` records that the operator explicitly directed autonomous
continuation until this bounded cycle is fully complete, so the advisory (P2/P3) corrections found
in this pass are accepted and applied in the same pass rather than deferred to a further review
cycle. **This is not merge approval.** It authorizes Stage to apply the corrections below, record
the gate outcome, and mark the branch cleared for push pending PR checks; it does not authorize
Ship to build, does not authorize a shipment claim, and does not authorize a merge. Those remain
separate, later gates owned by Ship and the operator respectively.

**Continuity note (added cycle 21).** Session memory
(`docs/memory/2026-08-25/stage-pr377-cycle-19-advisory-closure-memory.md`) labels the bounded
pass that applied the corrections below as **cycle 19**. It produced no separate `cycle: 19`
`## Plan Review` record because all six findings (N1, N2, N3a-c, N4) folded into this `cycle: 18`
record under the `operator_authorization` above, rather than opening a new gate. The next formal
`## Plan Review` record after this one is `cycle: 20`.

### Dispatch record — degraded to a single-agent sequential pass

This is the fresh, independent dispatch that follows the cycle-17 gate's synchronization fixes.
Reviewer sub-agent dispatch was unavailable for this pass; `TOOL_DEGRADED:
reviewer-subagent-dispatch` records the degradation, matching the cycle-16 and cycle-17 precedent.
The gate ran as a complete sequential, single-agent pass applying all seven personas' adapters over
the full plan text and the referenced backlog artifacts (`147-F` and its forty queued tasks),
achieving **complete seven-adapter coverage**: all seven selected personas completed; none was
skipped or partially dispatched.

| Persona | Coverage mode | Coverage assignment |
|---|---|---|
| Constitution | sequential (single-agent) | current-gate-state self-consistency; `operator_authorization` framing against the P-014 merge-approval boundary |
| Go | sequential (single-agent) | source-line and count claims (U8b dependent count) against the live dependency graph |
| Scope | sequential (single-agent) | topology and task-count claims against the live `147-F` queue; confirms no topology or task drift is introduced |
| Learnings | sequential (single-agent) | citation completeness in the consulted-learnings table against `docs/compound/` |
| Architecture | sequential (single-agent) | partition-preamble wording against the actual DAG dependency semantics |
| Agent-Native Parity | sequential (single-agent) | ambient-versus-current-source command form consistency across U9 Tests, U9b Tests, Gate 6/7, and the Risks table |
| Security | sequential (single-agent) | confirms no destructive or ambient-executable text is introduced by any correction |

### Gate rationale

The gate is **ADVISORY**. The five DAG partitions, the dependency graph, the 40-task /
98-executable-edge / 41-member topology, and the two-root ready set (`147.001-T`, `147.032-T`) are
confirmed **SOUND** and unchanged by this pass — this cycle finds no P0 and no P1. The six findings
below (one P2, five P3) are documentation-consistency and citation-completeness gaps: a stale
"current gate state" pointer chain that let more than one `## Plan Review` record read as
authoritative at once, an off-by-one dependent count in prose, a residual ambient-command form in
three normative locations whose sibling locations were already corrected in cycle 17, and an
optional same-file wording and citation polish. None of them bears on architecture, safety, or the
implementation contract, so the gate is advisory rather than blocking, and every finding is
corrected in this same pass under the `operator_authorization` above. `push_allowed: yes`.

### Findings by severity

**P0 — 0**

None.

**P1 — 0**

None.

**P2 — 1**

| ID | Finding | Disposition |
|---|---|---|
| N1 | Stale "current gate state" pointer chain: the cycles 1-13 admonition, the cycle-14 record, the cycle-15 record, the cycle-16 record's self-declaration and its "Cycle-17 status of this record" paragraph, the cycle-17 formal-decomposition appendix's intro, and the cycle-17 record's own self-declaration each either pointed at `cycle: 16` as current (already once-stale after cycle 17 landed) or independently declared themselves "the current gate state" with no forward pointer, so more than one record could be read as authoritative at once. | Fixed in this pass. Every earlier self-declaration is corrected to state which cycle superseded it and to point forward to `cycle: 18` as the sole current gate state; the cycles 1-13 admonition box no longer hardcodes an ordinal position for the current record. Exactly one current-gate-state claim now exists in this document. |

**P3 — 5**

| ID | Finding | Disposition |
|---|---|---|
| N2 | The U8b unit section's ordering-contract prose named "the eleven behavioural units it pins," and the cycle-17 appendix's H2 disposition said "Seventeen partition-4 units now depend on U8b." The live dependency graph's per-task prerequisite table shows **18** tasks — `147.006-T`, `147.007-T`, `147.008-T`, `147.009-T`, `147.011-T`, `147.012-T`, `147.013-T`, `147.014-T`, `147.015-T`, `147.017-T`, `147.022-T`, `147.023-T`, `147.024-T`, `147.025-T`, `147.027-T`, `147.029-T`, `147.039-T`, `147.040-T` (units U3, U3b, U4, U5, U6, U6b, U6c, U6d, U7, U7b, U7c, U7d, U7e, U8, U8c, U9, U16, U17) — declaring `147.016-T` as a prerequisite. | Fixed in this pass. Both the U8b section's unit list and the cycle-17 appendix's H2 row are corrected to the count of eighteen and the full unit list above; no dependency edge was added, removed, or renumbered. |
| N3a | U9's Tests bullet still invoked the ambient/installed `backlogit docs lint` rather than the current-source form Gate 6, the Dependency Graph regeneration note, and the U9/U9b red-verification row already use. | Fixed in this pass. Replaced with `go run ./cmd/backlogit --cwd . docs lint`. |
| N3b | U9b's Tests bullet carried the same ambient `backlogit docs lint` form. | Fixed in this pass. Replaced with `go run ./cmd/backlogit --cwd . docs lint`. |
| N3c | The Risks table's "CLI reference drift blocks the PR" row cited the ambient `backlogit docs lint` form. | Fixed in this pass. Replaced with `go run ./cmd/backlogit --cwd . docs lint`, preserving the surrounding `gen-docs` clause. |
| N4 | (Optional, same-file.) The cycle-17 formal-decomposition preamble stated "Partition order is a hard execution order" without grounding that in the DAG edges that are the actual binding constraint, reading as though partition number were itself an independent ordering rule rather than a label following from declared dependencies; separately, the consulted-learnings table was missing three directly applicable, already-shipped learnings on Windows-safe atomic rename, crash-safe rename rollback, and CLI reference drift, each bearing on mechanisms this plan already relies on. | Fixed in this pass, same file, no unit or topology change. The preamble now states the dependency graph is the binding constraint and partition number is a descriptive label following from it; the consulted-learnings table gained three rows: `docs/compound/best-practices/windows-safe-atomic-rename-goos-gate-2026-04-23.md`, `docs/compound/best-practices/crash-safe-delete-rename-rollback-go-2026-04-23.md`, and `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`. |

### Rejected / stale findings

| Claim | Disposition |
|---|---|
| The five-partition decomposition, the dependency graph, or the 40/98/41 topology needs restructuring | Rejected — this pass re-confirms the topology against the live `147-F` queue via the worktree-scoped CLI; unchanged by this gate. |
| The three P1 synchronization defects cycle 17 fixed (S1-S3) have regressed | Rejected — verified still corrected: the Deepened runtime verification recap names U10c, the Constitution Check Principle II narrative reads the corrected partition-3 framing, and stash `F350503F` / `A12BBAFA` / `3A33E404` are present and referenced correctly throughout. |

### Remediation queue and authorization

restage_recommendation: none (advisory corrections applied in-pass)

All findings in this cycle are P2/P3 and are fixed in the same pass under
`operator_authorization: approved`. No P0 or P1 finding exists. The plan, backlog, checkpoint, and
memory are updated to record this gate outcome and the corrections applied. Feature `147-F`'s
plan-review state is set to **ADVISORY authorized / cleared for push pending PR checks**; the
recorded next action is to push branch `chore/stage-130-s` and reconcile PR #377. This authorization
is scoped to *pushing the already-reviewed branch and letting PR checks run*; it is explicitly
**not** merge approval, not a shipment claim, and not authorization for Ship to begin
implementation — those remain separate gates owned by Ship and the operator.


### PR #377 Copilot review remediation, cycle 20 — one constitutional test lifecycle

**Trigger.** A fresh current-head review of PR #377 at `3bcff086` found CI 6/6 green and the
Copilot review fresh on that exact commit, but **20 unresolved Copilot threads**: 17 P1 and 3 P2.
The cycle-18 `decision: ADVISORY` record is therefore **superseded**; its `push_allowed: yes` and
`operator_authorization: approved` no longer describe the gate. The `cycle: 20` record below was
the current gate state when this appendix was written; it was superseded by the `cycle: 21` `FAIL`
record, which has itself since been superseded by the `cycle: 24` `ADVISORY` record at the end of
this document, the current gate state.

**Root causes, not samples.** The 20 threads reduce to four root causes. Every fix below is applied
to the whole class, not to the individual prose the reviewer happened to quote.

| Root cause | Threads | Fix |
|---|---|---|
| **A — green regression guards committed inside the red harness** | 14 P1: `147.002-T`, `147.003-T`, `147.005-T`, `147.009-T`, `147.011-T`, `147.012-T`, `147.014-T`, `147.016-T`, `147.022-T`, `147.023-T`, `147.024-T`, `147.025-T`, `147.027-T`, `147.028-T` | The "declared regression guards" device is withdrawn plan-wide. One three-step lifecycle now governs every unit: declaration step (no test function) → red harness step (only functions that fail) → green step (implementation plus `TestU<unit>Guard_` guards). P-004's precondition is expected failure markers *for every test function*, so a green function in the harness commit defeats the gate regardless of which `-run` regex is quoted. The cycle-17 narrowed-selector device on U2g and U2h is withdrawn for the same reason: P-002/P-004 gate the harness commit, not the selector. |
| **B — units with no genuine failing RED** | 3 P1: `147.007-T` (U3b), `147.032-T` (U1d), `147.038-T` (U15) | The cycle-8 rule "every unit still needs at least one assertion that fails" is withdrawn where it conflicts with declarations and documents; it is what forced the fabrications. U1d and U15 are `harness-exempt: declaration-only`, each naming the downstream unit whose harness genuinely fails. U3b's production delta — a second conformance call in `ResolveCheckpoint` — was already redundant after U14 routed the verb through the guarded seam, so it is withdrawn; U3b becomes `harness-exempt: verification-only` and the contract's failing harness is the new **U3c** (`147.042-T`), which lands before U14 and fails against the pre-migration verb. |
| **C — false published contract on the read surface** | 1 P2: `147.014-T` line 32 | `ListCheckpoints` sets `NeedsQuarantine: true` on three branches; U6 populated `RemediationIntent` on one. The published `backlogit_list_checkpoints` sentence promising a structured intent was therefore false for parse-failure and schema-invalid summaries. Closed by making the code total: new unit **U6e** (`147.043-T`) populates the intent with `Reason: "unparseable"` and `Reason: "schema_invalid"` on the remaining branches, and U7b now depends on it. |
| **D — stale halt path and stale archive ownership note** | 2 P2: `147.021-T` line 31, `.backlogit/archive/147.010-T.md` line 35 | U2f's halt path now names `return_blocked` explicitly: marking `147.021-T` blocked leaves it in the `130-S` manifest and the shipment cannot ship, so `ReturnBlockedItem` is the operation that must run. The archived U5b note's present-tense claim that U5 owns the state-conflict guards is corrected — cycle 16 withdrew them to stash `6FA45E69` and `147.009-T` no longer carries them. |

**New units.**

| Unit | Task | Partition | Role | Harness |
|---|---|---|---|---|
| U3c | `147.042-T` | 2 | Resolve-verb conformance refusal harness, test-only, no production change | 1 red function, `TestU3c_ResolveRefusesValidNonConformingDocument`; turned green by U14 |
| U6e | `147.043-T` | 4 | `RemediationIntent` on the parse-failure and schema-invalid quarantine branches | 2 red functions under `^TestU6e_` |

**Topology delta.** Six edges added: `147.042-T -> 147.001-T`, `147.042-T -> 147.004-T`,
`147.043-T -> 147.011-T`, `147.043-T -> 147.032-T`, `147.037-T -> 147.042-T`, and
`147.014-T -> 147.043-T`. No edge was removed or renumbered. Counts move from 40 tasks / 98 edges /
41 shipment members to **42 tasks / 104 edges / 43 shipment members**. The ready set is unchanged at
`{147.001-T, 147.032-T}` — both new tasks are interior nodes. The graph is verified acyclic by Kahn
topological sort, 42/42 nodes ordered from the two roots.

**New protected invariant I4 — declaration → harness → implementation monotonicity.** Every unit
whose delta changes behaviour has a failing harness that lands no later than its own harness step
and no earlier than the declarations it compiles against. The `harness-exempt` set is closed and
enumerated in Documented deviations, each entry naming where its behaviour's failing harness lives.
No behaviour-changing unit lacks a transitive harness prerequisite, and no harness-exempt unit
implements behaviour.

**Width.** No task in the plan exceeds 3 files or 3 scenarios after this pass. Relocating a guard
from the harness step to the green step does not change a task's file or scenario count — only the
commit in which the assertion lands.

## Plan Review

cycle: 20

**SUPERSEDED by cycle 21, in turn superseded by cycle 24.** This record is no longer the current
gate state. Its own remediation queue named "fresh local plan review of the cycle-20
test-lifecycle doctrine, U3c, U6e, and the recomputed topology" as `required before push`; the
`cycle: 21` record later in this document **is** that review, and the `cycle: 24` record at the
end of this document is the further independent confirmatory review cycle 21 itself required. The
root-cause remediation narrative above (root causes A-D, the new units U3c/U6e, and the topology
delta) stands unmodified as history — only this record's gate-state role (`decision`,
`operator_authorization`, `push_allowed`) is superseded. Read the gate fields below as history
only.

dispatch_mode: single-agent-declared-degradation

TOOL_DEGRADED: reviewer-subagent-dispatch

decision: FAIL (superseded — see cycle 21, then cycle 24)

operator_authorization: pending (superseded — see cycle 21, then cycle 24)

severity counts (inherited from the PR #377 current-head Copilot review at `3bcff086`): P0=0,
P1=17, P2=3, P3=0

push_allowed: no (superseded — see cycle 21, then cycle 24) — a fresh local plan review of the
cycle-20 artifacts must run before this branch is presented as ready again

### Authorization basis

This record supersedes cycle 18 in full. Cycle 18 evaluated a head that the Copilot reviewer
subsequently found to carry 17 P1 findings, so its ADVISORY decision cannot stand as the gate. All
17 P1 and 3 P2 findings are remediated in this pass at the root-cause level, but remediation is not
a gate: the corrected plan, the two new units, the six new edges, and the recomputed topology have
**not** been reviewed. The gate therefore returns `FAIL` with `restage_recommendation:
fresh-local-plan-review`.

### Remediation queue and restage recommendation

restage_recommendation: fresh-local-plan-review (fulfilled — see cycle 21)

| Item | Owner | State |
|---|---|---|
| Fresh local plan review of the cycle-20 test-lifecycle doctrine, U3c, U6e, and the recomputed topology | Stage | **complete** — performed as the `cycle: 21` record below |
| Reply to and resolve the 20 PR #377 threads once the fresh review passes | Ship / pr-lifecycle | blocked on cycle 21 (see below) |
| Operator merge approval (P-014) | operator | not requested |

This authorization covers **planning artifacts only**. It is not merge approval, not a shipment
claim, and not authorization for Ship to begin implementation.

## Plan Review

cycle: 21

**SUPERSEDED by cycle 24.** This record is no longer the current gate state. Its own remediation
queue named `restage_recommendation: confirmatory-review-of-cycle-21-fixes` as required before
push; the cycle-22 and cycle-23 remediation appendices and the cycle-24 remediation appendix that
follow widened that same obligation to also cover their own changes, and the `cycle: 24` record at
the end of this document **is** that confirmatory review. The root-cause remediation narrative
below (R1-R4: the ten-unit `harness-exempt` set fully labelled, the Ship ready-selection adapter,
the ambient docs-lint replacements, the cycle-19 continuity note, and the pointer-chain fix) stands
unmodified as history — only this record's gate-state role (`decision`, `operator_authorization`,
`push_allowed`) is superseded. Read the gate fields below as history only.

dispatch_mode: single-agent-declared-degradation

TOOL_DEGRADED: reviewer-subagent-dispatch

decision: FAIL (superseded — see cycle 24)

operator_authorization: pending (superseded — see cycle 24)

severity counts: P0=0, P1=1, P2=1, P3=2

push_allowed: no (superseded — see cycle 24) — this pass fixes every finding below at the
root-cause level, but a P1 finding was present, so a further independent confirmation pass is
required before push, matching the cycle-16/cycle-17 precedent that a same-pass fix does not
itself clear a P1 gate

### Authorization basis

This is the fresh local plan review of the cycle-20 test-lifecycle remediation artifacts that the
`cycle: 20` record's own remediation queue named `required before push`. It reviews the doctrine
rewrite, the guard-naming contract, U3c (`147.042-T`), U6e (`147.043-T`), the recomputed
42-task/104-edge/43-member topology, and the closed ten-unit `harness-exempt` set against the live
`147-F` backlog state (not against prose alone). This bounded session is planning-artifacts-only:
no Go source, test, or configuration file was touched; no build, test, or lint of Go source was
run beyond the read-only `go run ./cmd/backlogit` CLI verification gates; no push, PR action,
shipment claim, or Ship handoff occurred.

### Dispatch record — degraded to a single-agent sequential pass

Reviewer sub-agent dispatch was unavailable for this pass (this bounded session runs CLI-only,
worktree-bound, with no subagent delegation); `TOOL_DEGRADED: reviewer-subagent-dispatch` records
the degradation, matching the cycle-16/17/18/20 precedent. The gate ran as a complete sequential,
single-agent pass applying all seven personas' adapters over the full plan text and the referenced
backlog artifacts (`147-F` and its forty-two queued tasks), achieving **complete seven-adapter
coverage**: all seven selected personas completed; none was skipped or partially dispatched.

| Persona | Coverage mode | Coverage assignment |
|---|---|---|
| Constitution | sequential (single-agent) | Principle II vacuous-satisfaction framing against the closed `harness-exempt` set; whether the Ship ready-selection gap is a Principle II waiver (rejected) or a P-002 enforcement adapter (confirmed) |
| Go | sequential (single-agent) | live frontmatter `labels` field on all ten enumerated tasks against the plan's Documented deviations table; no Go source touched |
| Scope | sequential (single-agent) | topology and task-count claims against the live `147-F` queue; confirms no topology, edge, or ready-set drift is introduced |
| Learnings | sequential (single-agent) | citation completeness carried forward from cycle 19 unchanged; no new external learning required for a label/prose-only fix |
| Architecture | sequential (single-agent) | placement and blast radius of the Ship ready-selection adapter; confirms it is plan/task-local and does not touch `.github/policies/` or `.github/agents/` |
| Agent-Native Parity | sequential (single-agent) | ambient-versus-current-source command form consistency in `147.017-T` and `147.018-T` against every other corrected location (cycles 17, 19) |
| Security | sequential (single-agent) | confirms no destructive command, ambient-executable text, or credential is introduced by any correction |

### Gate rationale

The gate is **FAIL**, driven entirely by one P1 finding: the closed, ten-unit `harness-exempt` set
the cycle-20 doctrine enumerated was not actually applied to seven of its ten member tasks'
frontmatter, and no plan-local text told Ship's ready-selection query how to treat the label even
where it was present. The 42-task / 104-executable-edge / 43-shipment-member topology, the
two-root ready set (`147.001-T`, `147.032-T`), and the acyclic Kahn ordering are re-confirmed
**SOUND** and unchanged by this pass — this is not a topology or architecture finding. All findings
below are fixed in this same pass; the gate is `FAIL` rather than `ADVISORY` because a P1 existed
in the artifacts as reviewed, and, per the cycle-17 precedent, a same-pass fix is remediation
evidence, not a substitute for the independent confirmation pass a P1 gate requires.

### Findings by severity

**P0 — 0**

None.

**P1 — 1**

| ID | Finding | Disposition |
|---|---|---|
| R1 | The plan's Documented deviations table (and `147-F.md`'s test-lifecycle state) enumerate a closed, ten-unit `harness-exempt` set, and the cycle-20 checkpoint/memory recorded `harness_exempt_count: 10` / "exempt set = 10, matching the plan's closed table" as an already-satisfied claim. The committed frontmatter did not match: only `147.007-T`, `147.032-T`, and `147.038-T` carried the `harness-exempt` label; `147.017-T`, `147.018-T`, `147.019-T`, `147.021-T`, `147.026-T`, `147.036-T`, and `147.041-T` carried none. Separately, neither the global ready-queue policy (`.github/policies/workflow-policies.md`, `.github/agents/.ship.agent.md`), which filters strictly on `harness-ready`, nor any plan-local text told Ship's harness-architect/build-feature selection that `harness-exempt` is an alternative satisfied precondition — so even a fully-labelled set gave Ship no machine-readable signal to skip scaffolding a red harness for these ten tasks, silently risking a I4/Principle-II-vacuous-satisfaction violation on first Ship contact. | Fixed in this pass. All seven missing labels applied; the closed set now carries `harness-exempt` on exactly its ten enumerated members (verified by SQL query against the live index — see Validation). A machine-readable **Ship ready-selection adapter** was added to the plan's Documented deviations section (immediately after the closed-set table) and mirrored in `147-F.md`'s test-lifecycle state: Ship's ready-queue filter for shipment `130-S` accepts a task as harness-satisfied when `labels` contains `harness-ready` **or** `harness-exempt`, with the equivalent SQL predicate given inline. This is recorded explicitly as a shipment-scoped **P-002 enforcement deviation/adapter**, not a waiver of Constitution Principle II — Principle II remains satisfied vacuously per the existing Documented deviations row — and behaviour-changing units are unaffected: they still require `harness-ready` and cannot substitute `harness-exempt`. Global policy templates (`.github/policies/workflow-policies.md`, `.github/agents/.ship.agent.md`) are intentionally left unmodified in this staging pass; the adapter is plan/task-local. |

**P2 — 1**

| ID | Finding | Disposition |
|---|---|---|
| R2 | `147.017-T` (U9, docs-only) and `147.018-T` (U9b, docs-only, a HARD MERGE GATE task) invoked the ambient, PATH-resolved `backlogit docs lint` command 3 and 4 times respectively, across Tests/body prose and acceptance criteria, rather than the workspace-pinned `go run ./cmd/backlogit --cwd . docs lint` form that cycles 17 (Gate 6/7, the Dependency Graph regeneration note, the U9/U9b red-verification row) and 19 (N3a-c: the U9 Tests bullet, the U9b Tests bullet, the Risks table row) already standardized everywhere else in this plan. Because `147.018-T` is the hard merge gate blocking `147.007-T` / `147.008-T` / `147.009-T`, an ambient-binary reference inside its own acceptance criteria and classification prose carries the same self-hosted version-skew risk U10's binary-provenance discussion warns against, elevated above the cycles 17/19 P3 classification of the same defect class because this occurrence sits inside a hard merge gate rather than ordinary Tests-bullet prose. | Fixed in this pass. All 7 occurrences (3 in `147.017-T`, 4 in `147.018-T`) replaced with `go run ./cmd/backlogit --cwd . docs lint`, preserving surrounding args and context (the CLI Reference Drift clause, the merge-gate clause, and the acceptance-criteria wording). No other live (non-archived) task under `147-F` carries the ambient form. |

**P3 — 2**

| ID | Finding | Disposition |
|---|---|---|
| R3 | The plan narrates cycle 18 as directly superseded by cycle 20, with no mention that a bounded "cycle 19" advisory-closure session existed between them, appended the formal `cycle: 18` record, and fixed its N1/N2/N3a-c/N4 findings in-pass. A reader tracing the cycle numbering 14 → 15 → 16 → 17 → 18 → 20 could reasonably ask what happened to 19. | Fixed in this pass (optional per scope; low-risk, no topology or task change). A "Continuity note (added cycle 21)" paragraph was added to the cycle-18 record's Authorization basis, naming the cycle-19 memory artifact and stating explicitly that no separate `cycle: 19` gate record exists because its corrections folded into `cycle: 18`. |
| R4 | Stale current-gate-state pointer chain: the cycles-1-13 admonition box and the cycle-14/15/16/17/18/20 records' own self-declarations all named cycle 20 as "the current gate state" / "the end of this document", which becomes stale the moment this `cycle: 21` record is appended — reproducing the exact defect class cycle 19's N1 fixed for the cycle-16→17→18 transition. | Fixed in this pass. All six locations (the cycles-1-13 admonition, and the cycle-14, cycle-15, cycle-16, cycle-17, and cycle-18 self-declarations) updated to name `cycle: 21` as the current gate state. The `cycle: 20` record itself is marked `SUPERSEDED by cycle 21`, with its root-cause remediation narrative (A-D, U3c, U6e, the topology delta) preserved unmodified as history and only its gate-state role (`decision`, `operator_authorization`, `push_allowed`) superseded. Exactly one current-gate-state claim exists in this document after this pass. |

### Rejected / stale findings

| Claim | Disposition |
|---|---|
| The five-partition decomposition, the dependency graph, or the 42/104/43 topology needs restructuring | Rejected — this pass re-confirms the topology against the live `147-F` queue via the worktree-scoped CLI; unchanged by this gate. |
| The cycle-20 root-cause remediation (A-D, U3c, U6e) needs to be redone | Rejected — verified still correct and unmodified; only the cycle-20 record's gate-state role is superseded, not its remediation content. |
| The `harness-exempt` closed set itself (which ten units, which class) is wrong or incomplete | Rejected — the plan's Documented deviations table is authoritative and correct; the defect was the label application and the Ship query rule, not the enumeration. |

### Remediation queue and restage recommendation

restage_recommendation: confirmatory-review-of-cycle-21-fixes

All findings in this cycle (one P1, one P2, two P3) are fixed in the same pass. No P0 finding
exists. `push_allowed: no` because the one P1 finding, even though fixed here, still requires an
independent confirmation pass before push — this record does not itself authorize push, a merge,
or a shipment claim. The plan, feature, ten task frontmatter files, canonical memory, and
checkpoint are updated to record this gate outcome and the corrections applied.

| Item | Owner | State |
|---|---|---|
| Independent confirmation review of the cycle-21 label/adapter/docs-lint fixes | Stage | **complete** — performed as the `cycle: 24` record at the end of this document (widened to also cover cycles 22-24) |
| Reply to and resolve the 20 PR #377 threads | Ship / pr-lifecycle | unblocked — see the `cycle: 24` record |
| Operator merge approval (P-014) | operator | not requested |

This authorization covers **planning artifacts only**. It is not merge approval, not a shipment
claim, and not authorization for Ship to begin implementation.

### PR #377 plan remediation, cycle 22 — P-002 consumer contract generalized

**This appendix is remediation evidence, not a gate outcome.** It records a bounded
prompt/policy-artifact pass. It appends no `## Plan Review` record, claims no `PASS`, and does not
clear the `cycle: 21` `FAIL`. The `cycle: 21` record above was the current gate state when this
appendix was written; it has since been superseded by the `cycle: 24` `ADVISORY` record at the end
of this document, which closes the `restage_recommendation: confirmatory-review-of-cycle-21-fixes`
obligation this appendix still shows as outstanding. No Go source,
test, or configuration file was touched; no push, PR action, shipment claim, or Ship handoff
occurred.

**Trigger.** Cycle 21 recorded the `harness-exempt` ready-selection rule as a *shipment-local
adapter* and deliberately left `.github/policies/workflow-policies.md` and
`.github/agents/.ship.agent.md` unmodified. Those global artifacts accept only `harness-ready`, so
Ship would halt at its own Step 2 gate ("confirm every queued task now carries the `harness-ready`
label") before executing shipment `130-S` — with all forty-two tasks currently carrying neither
label and ten of them exempt by design. A plan-local paragraph cannot repair a global consumer
contract that halts first.

**Corrections applied.**

| ID | Finding | Fix |
|---|---|---|
| C1 | The global P-002 consumer accepts only `harness-ready`, so Ship halts before shipment `130-S` and, read literally, would scaffold red harnesses for all ten exempt units. | Generalized, not PR-specific: `.github/policies/workflow-policies.md` P-002 now admits a task that is **harness-satisfied** — `harness-ready` **or** fail-closed `harness-exempt` — with new **P-002.1** (closed exemption-class vocabulary, required metadata, predecessor-harness-owner conditions, evaluation order, producer no-scaffold obligation) and **P-002.2** (halt taxonomy and reporting). `.github/agents/.ship.agent.md` gains **Step 2a** and a three-way Step 2 partition; Step 3 selects on the harness-satisfied predicate. `.github/skills/harness-architect/SKILL.md` gains **Step 1a** plus guardrails forbidding fabricated REDs; `.github/skills/build-feature/SKILL.md` accepts harness-satisfied dispatch. P-004 gains the vacuous-satisfaction relationship. Amendment log entry `1.15.0`. |
| C2 | The Ship ready-selection adapter's "No exception for behaviour" bullet asserted that a behaviour-changing task *always* requires `harness-ready`, which contradicts U13 — an exempt, behaviour-changing unit — in the same table. | Reworded plan-wide. Behaviour requires **red evidence**, carried on the unit itself except for exactly one explicitly allowed, edge-backed carve-out: **U13 / `147.036-T`**, whose failing harness is owned by its declared prerequisite **U12 / `147.035-T`**, which carries `harness-ready` rather than an exemption and lands red before U13 builds. Corrected in the Documented deviations row, the Constitution Check Principle II row, the renamed "Ship ready-selection contract" bullets, and `147-F.md`. |
| C3 | The `147.018-T` / U9b hard merge gate was discoverable only from `147.018-T` and the plan's U9b bullet; the three constrained tasks carried no backreference. | `147.007-T`, `147.008-T`, and `147.009-T` each gain a `merge-gate-dependent` label and a "HARD MERGE GATE backreference" paragraph; `147-F.md` and `130-S.md` state the gate at feature and shipment level. Prose and labels only — **no dependency edge added**, executable ordering unchanged, topology unchanged. |
| C4 | Two current-gate pointers still named cycle 20 as "the current gate state" (the cycle-16 "Cycle-17 status of this record" paragraph and the cycle-17 formal-decomposition appendix preamble), and the cycle-20 remediation appendix asserted itself as current. | All three normalized to name `cycle: 21` as the current gate state. Exactly one current-gate-state claim remains. |
| C5 | `147-F.md` asserted in the present tense that "32 tasks carry a red harness". No harness has been scaffolded — Ship has not run `harness-architect` against this shipment. | Reworded to planned ownership: 32 tasks are **planned to own** a red harness, each naming the `TestU<unit>_` functions its harness step must land, none scaffolded yet. |

**Topology unchanged.** 42 queued tasks, 104 queued-to-queued executable edges, 43 shipment
members, ready roots exactly `{147.001-T, 147.032-T}`. No dependency edge, task count, shipment
membership, or unit definition changed in this pass.

**Still outstanding.** The `cycle: 21` remediation queue above is unchanged: the independent
confirmation review is still required before push, the PR #377 threads remain blocked on it, and
operator merge approval has not been requested.

### PR #377 plan remediation, cycle 23 — harness-exempt consumer contract made executable

**This appendix is remediation evidence, not a gate outcome.** It records a second bounded
prompt/policy-artifact pass over the cycle-22 contract. It appends no `## Plan Review` record,
claims no `PASS`, and does not clear the `cycle: 21` `FAIL`. The `cycle: 21` record above was the
current gate state when this appendix was written; it has since been superseded by the `cycle: 24`
`ADVISORY` record at the end of this document, which closes the
`restage_recommendation: confirmatory-review-of-cycle-21-fixes` obligation this appendix still
shows as outstanding, widened here to also cover cycle 23's own changes. No Go source, test, or
configuration file was touched; no subagent was invoked; no push, PR action, shipment claim, or
Ship handoff occurred.

**Trigger.** A fresh review of the cycle-22 artifacts found two P1 defects in the contract itself.
Cycle 22 gave Ship a vocabulary for `harness-exempt` but left the contract both unsatisfiable in
one direction and unfalsifiable in the other.

**Corrections applied.**

| ID | Finding | Fix |
|---|---|---|
| D1 | **Deadlock.** P-002.1 required the `covered-by` owner's red evidence (`harness-ready`, `Compilation: PASS`, `Red Phase: CONFIRMED`, harness commit landed) at Ship Step 2a — which runs *before* harness generation. The evidence cannot exist at that point, so `147.036-T` / U13 halts with `EXEMPT_OWNER_NOT_RED` on every run and shipment `130-S` deadlocks on its own gate. | P-002.1 split into **static intake** (fields, class, reason, closed-contract membership, owner existence / dependency edge / non-exempt type, declared commands) and a **claim-time** re-evaluation. `.ship.agent.md` gains **Step 4.1a**, an explicit claim-time gate that solely owns `EXEMPT_OWNER_NOT_RED` and runs the pre-work probe; Step 2a is now labelled static intake and forbidden from evaluating owner red evidence. `harness-architect` Step 1a is likewise static-only, must not raise `EXEMPT_OWNER_NOT_RED`, and must **scaffold** `covered-by` owners rather than excluding them. P-002's `Gate Point` row and the P-002.2 table now name the owning gate per code, reconciling the old "task claiming (Step 3)" wording with Ship's actual step numbering. |
| D2 | **False green.** Exempt dispatch could pass without any work. `go test -run <selector>` exits **0** with `[no tests to run]` when nothing matches (verified: `ok … [no tests to run]`, exit 0); the `declaration-only` "compile check" passes before the declaration exists; and `147.019-T`, `147.026-T`, `147.041-T` carried **no executable command at all**. | New **P-002.3**: every exempt task declares `exempt_precondition: must-fail-before-deliverable`; Ship Step 4.1a and `build-feature` Step 0 each run the command once pre-work and MUST observe failure, halting `EXEMPT_FALSE_GREEN` on a pre-work success. A false-green signal table makes `[no tests to run]`, `testing: warning: no tests to run`, `no test files`, and zero named `--- PASS:` lines failures regardless of exit status. Per-class authoring rules require `declaration-only` to probe the declared symbol (`go doc`, which exits non-zero when absent) and `docs-only` to probe required content before linting. The post-deliverable gate must pass **and** match declared evidence, else `EXEMPT_EVIDENCE_MISMATCH`. |
| D3 | The exemption class, reason, and owner lived only in prose (`**Test-lifecycle classification**: harness-exempt: docs-only`), with no stable machine-readable anchor and no command field. | All ten tasks gain a `<!-- BEGIN:harness-exemption-contract -->` block carrying five keys in identical order — `harness_exemption_class`, `harness_exemption_reason`, `harness_owner`, `exempt_verification_command`, `exempt_precondition` — plus `harness_owner_command` on U13. The class token is now bare `covered-by` with the owner in `harness_owner`. Body metadata, not frontmatter: `.backlogit/header-def.yaml` declares a closed per-type field set that would reject or drop these keys. Static intake requires all keys and an executable command (`EXEMPT_CONTRACT_INCOMPLETE`, `EXEMPT_COMMAND_MISSING`). |
| D4 | The three runtime-evidence units had no deterministic durable artifact, so "recorded runtime evidence" was unfalsifiable. | A single tracked human-readable manifest under the existing `docs/closure/` convention, `docs/closure/2026-08-checkpoint-disposition-runtime-verification.md` (already named by U10 and U10c), carries per-unit `evidence_row:` records (`filename`, `sha256`, `state`, `destination`, `outcome`) and `evidence_scalar:` fields. Probes require a minimum row count plus the declared scalars, so an absent, empty, or partially-populated artifact fails and only a complete run passes. |
| D5 | `build-feature` forbade modifying test files outright, which blocks `verification-only` and `declaration-only` deliverables whose *product* is a new guard file. | Narrowly amended: the skill MAY create the **new** test/evidence files a task names, and nothing else. Editing, deleting, relaxing, skipping, build-tagging, or narrowing the selector of any pre-existing assertion stays forbidden, as does authoring a failing assertion (a fabricated RED). The exception does not extend to `docs-only` (zero `*.go`) or `covered-by` (zero `*_test.go` — the owner owns the harness). |
| D6 | "Changes behavior" was a prose judgement, so class compliance was unverifiable after the fact. | New **P-002.4** makes it objective: a per-class changed-file delta surface, checked at the completion gate against `git diff --name-only`, with `EXEMPT_DELTA_EXCEEDS_CLASS` / `EXEMPT_BEHAVIOR_NO_OWNER` halts and an explicit fail-closed rule for unclassifiable deltas. `covered-by` is stated as the only exempt class that may modify production behaviour. U15's wrapper re-expression of `GetCheckpoint` is admitted only because its guards pin the pre-existing contract unchanged. |
| D7 | P-002's `Applies To` named only `ship`, with harness-architect mentioned parenthetically and `build-feature` absent, although both enforce the contract. | `Applies To` now reads `ship` (queue consumer and claim-time gate), `harness-architect` skill (producer / no-scaffold partition), `build-feature` skill (dispatch consumer and pre-work precondition probe), with the matching gate points. |

**No fabricated REDs.** No test was invented for any declaration-only or docs-only unit. Their
observed failure is the declared `exempt_verification_command` run against the pre-work tree, which
is a real probe of the declared symbol or the required content — not a build error and not a
manufactured assertion.

**Baseline evidence.** All ten `exempt_verification_command` values were extracted from the task
files and executed at HEAD `e8b974e`. All ten exited **1**, matching each task's declared baseline;
`147.036-T`'s `harness_owner_command` also exited 1. The command grammar was additionally proven in
the passing direction against existing symbols and existing test selectors, so the probes are
falsifiable in both directions rather than merely failing.

**Topology unchanged.** 42 queued tasks, 104 queued-to-queued executable edges, 43 shipment
members, ready roots exactly `{147.001-T, 147.032-T}`. No dependency edge, task count, shipment
membership, or unit definition changed in this pass.

**Still outstanding.** The `cycle: 21` remediation queue is unchanged, and cycle 23 adds to it: the
independent confirmation review must now cover the cycle-21, cycle-22, and cycle-23 changes
together. **Nine pre-existing P-003 gaps are recorded but not fixed here — corrected cycle 24 from
this pass's own undercount of two.** This pass's review scope was bounded to the harness-exemption
contract and did not re-audit every task's `acceptance-criteria` block, so it named only
`147.036-T` and `147.041-T` as carrying none. A fuller audit in cycle 24 found seven more:
`147.031-T`, `147.033-T`, `147.034-T`, `147.035-T`, `147.037-T`, `147.039-T`, and `147.040-T` also
carried no `acceptance-criteria` block at all, because closing any of the nine means authoring
acceptance criteria rather than correcting the exemption contract, which was out of scope here.
Every deliverable was declared in body prose — and, for `147.036-T` and `147.041-T`, in this
cycle's own contract and evidence-manifest requirements — so no task was ever undefined, but P-003
requires at least one recorded acceptance criterion per task and all nine were missing one. Cycle 24
authors the missing block for all nine against each task's existing contract; see the cycle-24
appendix below.

### PR #377 plan remediation, cycle 24 — acceptance-criteria gap closed, appendix count corrected, exempt contract hardened

**This appendix is remediation evidence, not a gate outcome.** It records a third bounded
prompt/policy/backlog-artifact pass over the cycle-23 contract, run in a dedicated worktree at HEAD
`35aac6c0` on branch `chore/cycle-24-remediation`. It appends no `## Plan Review` record, claims no
`PASS`, and does not clear the `cycle: 21` `FAIL`. The `cycle: 21` record above was the current
gate state when this appendix was written; it has since been superseded by the `cycle: 24`
`ADVISORY` record at the end of this document, which closes the
`restage_recommendation: confirmatory-review-of-cycle-21-fixes` obligation this appendix still
shows as outstanding, widened here to also cover cycles 22-24. No Go source, test, or production
configuration file was touched; no subagent was invoked; no push, PR action, shipment claim, or
Ship handoff occurred.

**Trigger.** A fresh review of the cycle-23 artifacts found the cycle-23 appendix's own P-003
undercount (it named two tasks lacking `acceptance-criteria`, not the actual nine), plus three P2
advisories against the exempt-execution contract cycle 23 introduced: the completion gate accepted
a bare `exit 0` with no positive signal, none of the ten declared commands was ever screened for a
destructive operation before execution, and `147.036-T` (plus two normative plan locations)
described U12 as "carries `harness-ready`" in the present tense where P-002.1's own two-gate split
only guarantees that at claim time. A fifth item — the closure-evidence filename's missing day
component — was raised as optional.

**Corrections applied.**

| ID | Finding | Fix |
|---|---|---|
| E1 | **Acceptance-criteria gap undercounted.** The cycle-23 "Still outstanding" paragraph named only `147.036-T` and `147.041-T` as carrying no `acceptance-criteria` block; a full audit of all 42 queued tasks found seven more with the same gap — `147.031-T`, `147.033-T`, `147.034-T`, `147.035-T`, `147.037-T`, `147.039-T`, `147.040-T` — a P-003 violation on all nine, not two. | All nine authored against their own existing unit contract (files, scenarios, harness/exempt lifecycle, dependencies, evidence paths) — no invented scope, none exceeding three scenarios. `147.036-T` and `147.041-T` (the two harness-exempt members) additionally cross-reference their own exemption class, owner, and evidence grammar. The cycle-23 "Still outstanding" paragraph is corrected in place from "two" to "nine", naming the fuller audit and all nine task IDs. All 42 queued tasks — 43 counting the archived `147.010-T`, which already carried one — now carry an `acceptance-criteria` block; re-verified independently by parsing every queue file rather than trusting the prior claim. |
| E2 | **Exempt completion gate accepted a bare `exit 0`.** All ten `exempt_verification_command` values terminated on a plain `exit 0` reached only through preceding `exit 1` guards, with no positive signal distinguishing a genuine pass from a cmd/shell wrapper that inverts the real exit code or a no-op that always returns 0. | New P-002.3 paragraph requires every `exempt_verification_command` to print `EXEMPT_VERIFY_OK:{task_id}` as its last statement before its own `exit 0`; a pass is now exit 0 **and** the exact marker. All ten task commands updated consistently — `Write-Output 'EXEMPT_VERIFY_OK:<task-id>'` inserted immediately before each command's final `exit 0`. `147.036-T`'s `harness_owner_command` is deliberately unchanged: it is the `build-feature` loop command, checked by its own named `--- PASS:` count against U12's harness, not the exempt completion gate. New halt code `EXEMPT_MARKER_MISSING`. `.ship.agent.md` Step 4.1a and Step 4.3, and `build-feature/SKILL.md` Step 0, Step 2, and the post-loop completion gate, all updated to require the marker before treating an exit 0 as a pass. |
| E3 | **No pre-execution screening for destructive operations.** Neither Ship nor `build-feature` screened a task-authored `exempt_verification_command` / `harness_owner_command` for a destructive pattern before running it. | New **P-002.5** requires a static Constitution Principle VII destructive-operation screen (deletion, force-overwrite, config/history mutation, data drops, package install/removal, untrusted code) immediately before either command is ever executed. A match is rejected outright — never executed by this gate — and routed to Principle VII operator approval instead; new halt code `EXEMPT_COMMAND_DESTRUCTIVE`. `.ship.agent.md` Step 4.1a and `build-feature/SKILL.md` Step 0 / post-loop gate screen both `exempt_gate_cmd` and, when distinct under `covered-by`, `harness_cmd`/`harness_owner_command`, before running either. All eleven live commands (ten `exempt_verification_command` plus `147.036-T`'s `harness_owner_command`) re-audited against a destructive-pattern list and confirmed read-only — probes only (`Test-Path`, `Get-Content`, `Select-String`, `go doc`, `go test -run`, `go run … docs lint`). |
| E4 | **U12's status was misstated in the present tense.** Three normative locations — `147.036-T`'s own body, the plan's U13 unit section, and the plan's "Ship ready-selection contract" bullets — said U12 "carries `harness-ready` rather than an exemption", contradicting the same two-gate split cycle 23 itself introduced: static intake never observes red evidence, only claim time does. | All three corrected to "must carry `harness-ready` at claim time — eligible only after harness generation runs — rather than an exemption." The cycle-22 historical appendix cell (`C2`) that used the original wording is left unmodified as the historical record of what cycle 22 said at the time; normative text, not historical evidence, carried the defect. |
| E5 | **Workflow policy `Version` header stale.** The registry header read `1.0.0` while the Amendment Log's own last row already recorded `1.16.0` (cycle 23's P-002.3/P-002.4 addition) — a documentation-sync defect independent of any content change. | Header corrected to `1.16.0`. The existing `1.16.0` amendment-log row is extended in place — rather than minting a further version number — to also describe this cycle's P-002.3 marker mandate, the new P-002.5, and the two new halt codes, and to note that the header itself was previously stale. |
| E6 | **Optional: closure-evidence filename did not follow the `YYYY-MM-DD` convention** every other `docs/closure/` artifact uses (for example `docs/closure/2026-08-24-130-s-adversarial-review.md`). | The not-yet-created planned path `docs/closure/2026-08-checkpoint-disposition-runtime-verification.md` is renamed to `docs/closure/2026-08-24-checkpoint-disposition-runtime-verification.md` (dated to the governing plan document) across every **live** reference: `147.019-T`, `147.026-T`, `147.041-T`, and the plan's U10/U10b/U10c unit sections. The two historical mentions (the cycle-17 `H3` blocker-disposition row and the cycle-23 `D4` finding row) keep the name as it was written at the time — renaming history would misstate what those cycles actually named. No topology, dependency, or task-count change; the file still does not exist. |

**No fabricated REDs; no behaviour change.** This pass touches no `*.go` file, adds no dependency
edge, and creates no new task. The nine acceptance-criteria blocks restate each task's own
already-declared contract; none extends scope beyond what the task body already specified.

**Baseline evidence, re-verified after the marker/screening changes.** All ten
`exempt_verification_command` values — now each carrying its `EXEMPT_VERIFY_OK:{task_id}` marker —
were re-executed against this worktree's pre-work tree (HEAD `35aac6c0`); all ten exited **1** and
none printed its marker, because none of the ten deliverables exists yet, so the marker branch is
never reached and the failing branch is unchanged. `147.036-T`'s `harness_owner_command` was
re-executed the same way and also exited **1**. All eleven commands were independently re-audited
against a destructive-operation pattern list with zero matches.

**Validation.**

| Gate | Result |
|---|---|
| All 10 `exempt_verification_command` values, re-executed at this worktree's HEAD | 10/10 exit 1, 0/10 marker printed (declared baseline unchanged) |
| `147.036-T` `harness_owner_command`, re-executed | exit 1 |
| Destructive-operation pattern audit, all 10 exempt commands + 1 owner command | 0 matches — all read-only |
| `sync` | 1208 artifacts indexed |
| `doctor` | 23 issues, all pre-existing orphans (`106.0xx-T`, `016.001-R`) outside `147-F`; 0 new |
| `docs lint` | `valid: true, violation_count: 0` |
| `scripts/md-lint.ps1` (markdownlint-cli2 0.23.1, repo-wide) | 2286 files, 0 issues, exit 0 |
| `go test ./tests/integration/... -count=1` | `ok` — existing structural prompt/plugin/CI guards pass |
| Frontmatter YAML validity + balanced section markers, all 21 changed Markdown files | 21/21 valid; the one raw-text match on a historical table cell's backtick-quoted example is not a real marker |
| Exemption-class vocabulary (`declaration-only`, `docs-only`, `verification-only`, `covered-by`) | present and consistent across policy, `.ship.agent.md`, `harness-architect/SKILL.md`, `build-feature/SKILL.md` |
| Halt-code cross-references, including the two new codes | all 12 codes referenced in policy; the 5 codes owned at execution time (including the 2 new ones) referenced in both `.ship.agent.md` and `build-feature/SKILL.md`; the 7 static-intake-only codes referenced in `.ship.agent.md` only, matching the pre-existing pattern |
| Topology, recomputed independently from queue frontmatter (not from the index) | 42 queued tasks / 104 queued-to-queued executable edges / 43 shipment members / ready roots exactly `{147.001-T, 147.032-T}` / acyclic (42/42 Kahn-ordered) |
| Acceptance criteria | 42/42 queued tasks carry an `acceptance-criteria` block (43/43 counting the archived `147.010-T`) |

**Topology unchanged.** 42 queued tasks, 104 queued-to-queued executable edges, 43 shipment
members, ready roots exactly `{147.001-T, 147.032-T}`. No dependency edge, task count, shipment
membership, or unit definition changed in this pass.

**Still outstanding.** The `cycle: 21` remediation queue is unchanged, and cycle 24 adds to it: the
independent confirmation review must now cover the cycle-21, cycle-22, cycle-23, and cycle-24
changes together. No P-003 gap remains: all 42 queued tasks (43 with the archived `147.010-T`)
carry at least one acceptance criterion. No push, merge approval, or shipment claim was performed,
and no subagent was invoked.

**Resolved by the cycle-24 confirmatory review below.** The `## Plan Review` `cycle: 24` record
immediately following this appendix is that independent confirmation review. It found five P2
advisories — none against the acceptance-criteria or exempt-execution work recorded above — and
fixed them in-pass; the topology, the ten-unit exempt set, and every correction recorded in this
appendix are re-confirmed unchanged and are not reopened.

## Plan Review

cycle: 24

dispatch_mode: single-agent-declared-degradation

TOOL_DEGRADED: reviewer-subagent-dispatch

decision: ADVISORY

operator_authorization: approved

severity counts: P0=0, P1=0, P2=5, P3=0

push_allowed: yes

**This is the independent confirmatory review that the `cycle: 21` record's own remediation queue
named `required before push`, widened by the cycle-22, cycle-23, and cycle-24 remediation
appendices above to also cover their own changes.** It supersedes `cycle: 21` `FAIL` for
gate-decision purposes; cycle 21 and every earlier record remain the historical trace of how the
plan reached its present shape, and cycle 21's own root-cause remediation (R1-R4: the ten-unit
`harness-exempt` set fully labelled, the Ship ready-selection adapter, the ambient docs-lint
replacements, the cycle-19 continuity note, and the pointer-chain fix) stands unmodified as history
— only its gate-state role (`decision`, `operator_authorization`, `push_allowed`) is superseded
here. **This record is the current gate state.**

### Authorization basis

`operator_authorization: approved` records that the operator explicitly directed autonomous
completion of this bounded cycle — a direction to finish the confirmatory review cycle 21 required
and apply any advisory corrections it found in the same pass, not to defer them to a further cycle.
**This is not merge approval.** It authorizes Stage to record this gate outcome, apply the five P2
corrections below, and mark the branch cleared for push pending PR checks; it does not authorize
Ship to build, does not authorize a shipment claim, and does not authorize a merge. Those remain
separate, later gates owned by Ship and the operator respectively.

### Dispatch record — degraded to a single-agent sequential pass

Reviewer sub-agent dispatch was unavailable for this pass (this bounded session runs CLI-only,
worktree-bound, with no subagent delegation); `TOOL_DEGRADED: reviewer-subagent-dispatch` records
the degradation, matching the cycle-16/17/18/20/21 precedent. The gate ran as a complete
sequential, single-agent pass applying all seven personas' adapters over the full plan text, the
cycle-21 through cycle-24 remediation record and appendices, and the referenced backlog artifacts
(`147-F` and its forty-two queued tasks), achieving **complete seven-adapter coverage**: all seven
selected personas completed; none was skipped or partially dispatched.

| Persona | Coverage mode | Coverage assignment |
|---|---|---|
| Constitution | sequential (single-agent) | Principle VII destructive-command approval framing against the new P-002.5 screen; `operator_authorization` framing against the P-014 merge-approval boundary; Principle II vacuous-satisfaction framing unaffected by the Step 4.1/4.1a reorder |
| Go | sequential (single-agent) | confirms no Go source, test, or production configuration file is touched this cycle; the exempt-command marker/screening prose describes CLI-observable behavior only |
| Scope | sequential (single-agent) | topology, task-count, and exempt-set claims against the live `147-F` queue; confirms no topology, edge, ready-set, or exempt-set drift across cycles 21-24 |
| Learnings | sequential (single-agent) | citation completeness carried forward unchanged; no new external learning required for this pass's P2-level corrections |
| Architecture | sequential (single-agent) | Ship Step 4.1/4.1a claim-time ordering against the actual state-mutation sequence; the plugin-bundle/`.github` boundary against the 101-F structural-verification decision; blast radius of every correction confirmed plan/task/policy-local |
| Agent-Native Parity | sequential (single-agent) | `.autoharness/drift-ignore` re-apply-obligation completeness against the live cycle-24 policy/agent/skill diffs; ambient-vs-workspace-pinned command forms re-checked, none found |
| Security | sequential (single-agent) | confirms the claim-time gate reorder leaves no task stranded `active` on a halt; confirms no destructive command, ambient-executable text, or credential is introduced by any correction |

### Gate rationale

The gate is **ADVISORY**. The 42-task / 104-executable-edge / 43-shipment-member topology and the
two-root ready set (`147.001-T`, `147.032-T`) are re-confirmed **SOUND** and unchanged across
cycles 21 through 24 — this cycle finds no P0 and no P1. The five findings below (all P2) are
sequencing-safety, scope-boundary, drift-bookkeeping, memory-persistence, and evidence-currency
gaps: a claim-time gate whose physical ordering could strand a task `active` on a halt even though
its own prose already said it ran "immediately before" the claim; an undeclared but real
plugin-bundle scope boundary; a drift-ignore re-apply obligation that had not caught up to cycle
24's own additions; two missing canonical memory entries; and a stale live-corpus illustrative
count in prose whose surrounding sentence already anticipated the drift. None of them bears on
architecture, safety, or the implementation contract, so the gate is advisory rather than blocking,
and every finding is corrected in this same pass under the `operator_authorization` above.
`push_allowed: yes`.

### Findings by severity

**P0 — 0**

None.

**P1 — 0**

None.

**P2 — 5**

| ID | Finding | Disposition |
|---|---|---|
| B1 | Ship's claim-time sequence set a task's status to `active` at **Step 4.1** before the harness-exempt gate at **Step 4.1a** ran, even though Step 4.1a's own text said it runs "immediately before it is claimed." A task that halted at Step 4.1a — `EXEMPT_CONTRACT_INCOMPLETE`, `EXEMPT_OWNER_NOT_RED`, `EXEMPT_COMMAND_DESTRUCTIVE`, `EXEMPT_FALSE_GREEN`, or `EXEMPT_MARKER_MISSING` — was therefore stranded `active` with no further step to move it, since a halt returns the task to the operator or to Stage rather than reversing the claim. | Fixed in this pass. `.github/agents/.ship.agent.md` reordered: the gate is now physically first as **Step 4.1a**, unchanged in name, number, and content, and the status mutation is renamed **Step 4.1b: Claim Task**, positioned after it and reached only once Step 4.1a has passed (or does not apply, for a `harness-ready` task). A new closing sentence makes the invariant explicit: a halt at Step 4.1a stops before Step 4.1b runs, so the task's status is untouched and it is never stranded `active`. No other file references the bare `Step 4.1` label for the claim action (checked repo-wide — the one historical hit, `docs/memory/2026-08-21/ship-129-s-pa8-pa3-approval-gate-memory.md`, predates the harness-exempt system and is left as history), so every existing cross-reference to `Step 4.1a` (the gate) in `workflow-policies.md`, `harness-architect/SKILL.md`, `build-feature/SKILL.md`, this plan, `147-F.md`, `130-S.md`, and all ten exempt task bodies remains correct, unchanged. |
| B2 | The P-002 harness-exempt consumer contract (cycles 22-24) amended `.github/policies/workflow-policies.md`, `.github/agents/.ship.agent.md`, and `.github/skills/build-feature/SKILL.md`, but no artifact stated whether `plugin/agents/ship.agent.md` or `plugin/skills/build-feature/SKILL.md` were also expected to carry it. Left unstated, a future reader could misread the omission as an uncaught gap rather than a deliberate, pre-existing boundary. | Declared explicitly in three places; the plugin bundle itself is untouched. **Plan**: a new bullet in the "Ship ready-selection contract" section (Documented deviations, above) states the boundary and its rationale. **Drift note**: a new dated note in `.autoharness/drift-ignore` records that `plugin/agents/ship.agent.md` and `plugin/skills/build-feature/SKILL.md` carry no P-002 vocabulary at all — confirmed by direct inspection, not merely missing the cycle 22-24 additions — per the pre-existing `docs/decisions/2026-07-13-plugin-bundle-structural-verification-decision.md` (101-F) decision that `plugin/` and `.github/` are not content-synchronized and that `TestPluginBundleStructurallyValid` validates structure only. **Follow-up**: stash entry `633818E1` (kind `task`, priority `low`) records the same boundary as a durable backlog artifact so it survives outside this plan's prose. `plugin/` was not edited. |
| B3 | `.autoharness/drift-ignore`'s re-apply-obligation note for the P-002.1/P-002.2 contract was corrected in place for cycle 23's split-gate form, but was never extended for cycle 24's positive-completion-marker mandate, the new P-002.5 destructive-command screen, or the two halt codes those additions introduced (`EXEMPT_MARKER_MISSING`, `EXEMPT_COMMAND_DESTRUCTIVE`). A template re-adoption after cycle 24 would have re-applied only the cycle-23 shape and silently dropped the cycle-24 additions. | Fixed in this pass. A new dated note appended immediately after the existing cycle-23 note extends the re-apply obligation to cover the `EXEMPT_VERIFY_OK:{task_id}` marker mandate and P-002.5 screening in `workflow-policies.md`, `.ship.agent.md` (including the Step 4.1/4.1a reorder from finding B1), and `build-feature/SKILL.md`, naming both new halt codes explicitly. `harness-architect/SKILL.md` is reconfirmed unaffected (it never executes either command) and still needs no entry. |
| B4 | Cycles 21 and 24 each produced a detailed `docs/memory/2026-08-25/...-memory.md` narrative, but neither cycle persisted a corresponding keyed entry to the canonical `.backlogit/memories.json` store — cycles 20, 22, and 23 each have one (`stage-pr377-cycle-20`, `stage-pr377-cycle-22`, `stage-pr377-cycle-23`), so the canonical store carried a two-entry gap relative to the narrative record. | Fixed in this pass using the worktree-bound CLI (`go run ./cmd/backlogit --cwd . memory save --key stage-pr377-cycle-21 --summary "..."` and the equivalent `--key stage-pr377-cycle-24`), matching the existing entries' density and no-invented-content standard: each summary is drawn only from that cycle's own plan record or memory doc, with no fabricated detail. |
| B5 | The plan's U10 runtime-verification section said "twelve files are present on this branch now that the staging checkpoint has landed" as the illustrative live-corpus count. The actual count had already drifted well past that, which the surrounding sentence anticipated ("that number drifts as sessions add checkpoints, so the guard enumerates the directory rather than a literal") but did not itself keep current. | Refreshed in this pass to the count observed at this session: twenty files (the same nine schema-invalid legacy files this plan's contract has always excluded, plus eleven conforming files, up from two when this section was first written), without changing the guard's own non-literal, directory-enumerating behavior, the nine-file expected-refusal set, or the closure artifact's pre-merge recapture requirement — both preserved verbatim. |

### Rejected / stale findings

| Claim | Disposition |
|---|---|
| The 42-task / 104-edge / 43-member topology, the ready set, or the ten-unit `harness-exempt` set needs restructuring | Rejected — re-confirmed against the live `147-F` queue via the worktree-bound CLI; unchanged by this gate. |
| Cycles 21-24's root-cause remediation (R1-R4, C1-C7, D1-D7, E1-E6) needs to be redone | Rejected — verified still correct and unmodified; only cycle 21's gate-state role is superseded, not its remediation content, and cycles 22-24 remain remediation evidence under this now-ADVISORY gate. |
| The nine-file expected-refusal set or the closure-artifact recapture requirement needs to change alongside the live-corpus count refresh (B5) | Rejected — the nine legacy filenames and the pre-merge recapture requirement are unaffected by ordinary conforming checkpoints accumulating; both are preserved verbatim. |

### Remediation queue and authorization

restage_recommendation: none (advisory corrections applied in-pass)

All findings in this cycle are P2 and are fixed in the same pass under `operator_authorization:
approved`. No P0 or P1 finding exists. The plan, `147-F.md`, `.backlogit/memories.json`, and a new
session checkpoint are updated to record this gate outcome and the corrections applied. Feature
`147-F`'s plan-review state is set to **ADVISORY authorized / cleared for push pending PR checks**;
the recorded next action is to push branch `chore/stage-130-s` and reconcile PR #377. This
authorization is scoped to *pushing the already-reviewed branch and letting PR checks run*; it is
explicitly **not** merge approval, not a shipment claim, and not authorization for Ship to begin
implementation — those remain separate gates owned by Ship and the operator.

| Item | Owner | State |
|---|---|---|
| Reply to and resolve the 20 PR #377 threads | Ship / pr-lifecycle | unblocked — ready once the branch is pushed |
| Push `chore/stage-130-s` and let PR #377 checks run | operator / Stage | ready |
| Operator merge approval (P-014) | operator | not requested |

This authorization covers **planning artifacts only**. It is not merge approval, not a shipment
claim, and not authorization for Ship to begin implementation.

---

### PR #377 review remediation, cycle 29 — declaration-only withdrawn, harness waves introduced

**Trigger.** Six threads remained unresolved on HEAD `6a822ceb` after the cycle 26-28 fix budget
was exhausted (`github-pr-automation` §1.8 limits review-fix cycles to 3 per HEAD; the limit stops
automated fixing but does not clear the merge gate). Each was accepted as valid at the time and
explicitly deferred rather than waived. Three of them — the `declaration-only` class challenge, the
topological-waves finding, and the U10 `.gitignore` conflict — were recorded as requiring a fresh
plan review because they change contracts rather than wording, and the first two were recorded as
having to be decided **together**. This cycle decides them together.

| Thread | Location | Classification | Disposition |
|---|---|---|---|
| T4 | `workflow-policies.md:75` | foundational | **`declaration-only` withdrawn.** Option (A) from the deferral note is taken: the class admitted observable production surface with no observed failing test, which is a carve-out from NON-NEGOTIABLE Principle II, and cycle 20's tension is resolved by a **source-shape** harness rather than by an exemption or a signed deviation. Closed vocabulary drops to three tokens; the exempt set drops from ten to **eight**. |
| T2 | `.ship.agent.md:254` | requires decomposition | **Waves adopted** as new policy **P-002.6**. Ship Step 2 becomes a per-wave procedure invoked from a new Step 4.0 wave-admission step; Step 3 builds the wave schedule; Step 4 is an outer loop over waves. The cycle-26 declaration-prerequisite early-execution pass is removed — it was only ever one wave and could not satisfy a chain of depth > 1. |
| T1 | `workflow-policies.md:261` | bounded | **Fixed.** P-002.3's must-fail signal list no longer admits an unmarked exit 0 as the required pre-work failure; the probe classifies three ways and `EXEMPT_MARKER_MISSING` is stated as a halt at both gates, matching Ship 4.1a item 4 and `build-feature` Step 0 item 4. |
| T3 | `build-feature/SKILL.md:213` | bounded | **Fixed.** The skill's completion gate diffs the baseline against the **working tree** (two-dot with no right-hand side, plus `--cached`) because it runs before its own Commit step; an empty changed-file set is now a halt. Ship Step 4.3 keeps `..HEAD` and now states why. |
| T6 | `.autoharness/drift-ignore:70` | bounded | **Fixed.** The cycle-24 re-apply note said the `EXEMPT_VERIFY_OK` marker applied to `harness_owner_command` "distinctly"; corrected to state the marker is required on `exempt_verification_command` only and that owner commands are validated by named `--- PASS:` count and are intentionally unmarked. |
| T5 | `147.019-T.md:83` | requires decomposition | **Fixed without decomposition.** The scratch workspace moves from the un-ignored `docs/scratch/checkpoint-verification/` to the already-ignored `.copilot/scratch/checkpoint-verification/` (`git check-ignore -v` → `.gitignore:5:.copilot/`; `git ls-files .copilot` → 0 entries), so U10 commits no ignore rule at all. The `verification-only` class contract is **narrowed**, not widened: P-002.4 now explicitly rejects `.gitignore` and other configuration files. Topology is unchanged. |

**Why T4 and T2 had to be decided together, and how the resolution composes.** The waves design
depends on whether declarations are exempt. With `declaration-only` gone, U1d and U15 become
harness-required, which *adds* two harness-generation obligations to the DAG — and it is precisely
the wave scheduler that makes those obligations satisfiable. The concrete chain the reviewer named
resolves as U1d (wave 1) → U1b (wave 2) → U15 (wave 3) → U8b (wave 4): each harness is scaffolded
only after every declaration it compiles against is `done`, so no waiver, no early-execution pass,
and no exemption is required anywhere in the chain.

**The source-shape harness is the third option cycle 20 missed.** Cycle 20 saw only two: a test
that references the missing symbol (a build error, which P-004 rejects) or a test that passes the
instant the shape lands (never red). A test that parses the production file with `go/parser` and
asserts the declared shape via `go/ast` is neither — it names no undeclared identifier, so it
compiles; and it fails on an assertion, so the red is genuine. Both harnesses were executed against
worktree HEAD `6a822ceb`: `go vet ./internal/events` exits 0 and `go test -run='^$' -count=1
./internal/events` reports `ok … [no tests to run]` (compiles without the declaration), while
`go test -count=1 -v -run '^TestU1d_'` and `-run '^TestU15_'` each exit 1 with two assertion
failures naming the absent shape. `RemediationIntent`, `remediation_intent`, `CheckpointReadResult`,
and `GetCheckpointResult` have 0 occurrences under `internal/events/` at that HEAD.

**Wave verification.** The 42-task / 104-edge DAG was simulated under the P-002.6 wave rule: **18
waves, 42 tasks scheduled, 0 stalls, 0 compile-order violations** (every dependency lands in a
strictly earlier wave than its dependent). The `covered-by` owner U12 lands in wave 6 and U13 in
wave 7, so `EXEMPT_OWNER_NOT_RED` is satisfiable by construction. As a negative control, injecting
the cycle `147.032-T → 147.038-T` halts the scheduler with `WAVE_NO_PROGRESS` after 9 waves with 26
tasks blocked, rather than looping — confirming the no-progress detector fires. The full schedule
is tabulated in the Execution Order section.

**Contract changes.**

| Artifact | Change |
|---|---|
| `workflow-policies.md` | 1.16.0 → **1.17.0**. P-002 postcondition restated per wave; P-002.1 vocabulary 4 → 3 tokens plus the source-shape rule; P-002.2 gains `WAVE_NO_PROGRESS`, `WAVE_CYCLE_DETECTED`, `WAVE_BUDGET_EXCEEDED`; P-002.3 three-way probe classification and marker scoping; P-002.4 per-consumer diff form, empty-delta halt, and the repository-hygiene prohibition; P-004 wave scoping and declaration-task applicability; **new P-002.6**. |
| `.ship.agent.md` | Step 2 wave-scoped (cycle-26 early pass removed); Step 3 → wave schedule; **new Step 4.0** wave admission; Step 4.2 routing; Step 4.3 diff-form rationale and empty-delta halt; two new circuit breakers; branch rule pinning all waves to one branch/worktree/PR (P-016). |
| `build-feature/SKILL.md` | Completion gate uses the working-tree diff form; empty delta halts; Commit step must not be reordered ahead of the gate; `declaration-only` removed from Inputs and the Step 5 exception. |
| `harness-architect/SKILL.md` | Wave-scoped Step 1; missing-dependency-edge reporting; source-shape harness rule in Step 1a with no production stub; Steps 4 and 5.2 aligned. |
| `.autoharness/drift-ignore` | Cycle-24 marker note corrected; cycle-26 notes marked SUPERSEDED; cycle-29 re-apply obligation appended. |
| Tasks | `147.032-T` and `147.038-T` lose `harness-exempt` and their contract blocks, gain source-shape harness specs and red selectors; `147.019-T` loses the `.gitignore` delta and relocates its workspace; `147.041-T` teardown path updated. |

**Gate state.** This is a plan-and-prompt-artifact correction only. No production Go code is
written here, and no push or merge is performed. The §1.9 readiness gate remains **FAIL on Check 3**
pending a fresh local review of these changes and re-review of PR #377 on the new HEAD; the six
threads are addressed but are not self-resolved by this session.

---

## Plan Review

cycle: 29

dispatch_mode: single-agent-declared-degradation

TOOL_DEGRADED: reviewer-subagent-dispatch

decision: ADVISORY

operator_authorization: approved

severity counts: P0=0, P1=0, P2=3, P3=2

push_allowed: yes

**This is the fresh local plan review that the cycle-29 remediation session's own gate state named
`required before push` ("the §1.9 readiness gate remains FAIL on Check 3 pending a fresh local
review of these changes"), run at canonical worktree HEAD `ef8bd954`.** It supersedes `cycle: 24`
for gate-decision purposes; cycle 24 and every earlier record remain the historical trace of how
the plan reached its present shape, and cycle 29's own remediation (the `declaration-only`
withdrawal, the source-shape harness design, new policy P-002.6, the three bounded fixes T1/T3/T6,
and the U10 workspace relocation T5) stands unmodified as history — only the gate-state role
(`decision`, `operator_authorization`, `push_allowed`) is superseded here. **This record is the
current gate state.**

### Authorization basis

`operator_authorization: approved` records that the operator explicitly directed autonomous
completion of this bounded review-and-fix cycle — a direction to run the confirmatory review cycle
29 required and apply any advisory corrections it found in the same pass, not to defer them to a
further cycle. **This authorization is autonomous completion of this bounded cycle, not merge
approval.** It authorizes Stage to record this gate outcome, apply the three P2 corrections below,
and mark the branch **ADVISORY authorized / ready for push**; it does not authorize a shipment
claim, does not authorize Ship to begin implementation, and does not authorize a merge. Those
remain separate, later gates owned by Ship and the operator respectively, and this record does not
itself resolve the §1.9 readiness gate's Check 3 — that requires PR #377's review threads to be
re-examined against the pushed HEAD.

### Dispatch record — degraded to a single-agent sequential pass

Reviewer sub-agent dispatch was unavailable for this pass (this bounded session runs CLI-only,
worktree-bound, with no subagent delegation, per the operator's explicit "no subagents" scope);
`TOOL_DEGRADED: reviewer-subagent-dispatch` records the degradation, matching the
cycle-16/17/18/20/21/24 precedent. The gate ran as a complete sequential, single-agent pass
applying all seven personas' adapters over cycle 29's remediation text, the 1.17.0 policy contract,
`build-feature/SKILL.md`, `.ship.agent.md`, `.autoharness/drift-ignore`, and the referenced backlog
artifacts (`147-F` and its forty-two queued tasks, re-verified live), achieving **complete
seven-adapter coverage**: all seven selected personas completed; none was skipped or partially
dispatched.

| Persona | Coverage mode | Coverage assignment |
|---|---|---|
| Constitution | sequential (single-agent) | Principle II framing re-confirmed for the `declaration-only` withdrawal (the P2-1 fix reclassifies a marker gap, it does not reintroduce a carve-out); Principle VII unaffected — no destructive command is touched by any of the three findings |
| Go | sequential (single-agent) | confirms no Go source, test, or production configuration file is touched this cycle; the P-002.6 snapshot recipe, the P2-1 wording, and the U1d/U15 fail-count question describe CLI-observable and documentation-only content |
| Scope | sequential (single-agent) | topology, task-count, wave-count, and exempt-set claims re-verified live against the `147-F` queue via `backlogit sync` / `query` / `list` / `shipment get` at this session's HEAD; confirms no drift from cycle 29's 42-task / 104-edge / 43-member / 18-wave / eight-unit figures |
| Learnings | sequential (single-agent) | the 117-F / A6A1B47E per-row-N+1-avoidance precedent (`internal/cli/list.go`) is cited for the new P-002.6 snapshot recipe; no other new external learning required for this pass's P2-level corrections |
| Architecture | sequential (single-agent) | the new one-snapshot-per-wave recipe's placement (P-002.6, cross-referenced from `.ship.agent.md` Step 4.0 item 1) and its interaction with P-012 degraded-tool handling; blast radius of every correction confirmed policy/skill/memory-local, no plan-topology or task-graph change |
| Agent-Native Parity | sequential (single-agent) | CLI (`backlogit query`, `backlogit list --json`) and MCP (`backlogit_query_sql`, `backlogit_list_items`) equivalence re-confirmed for the new recipe by direct invocation against the live index (one join call returned 42 items / 104 edges; one list call returned the same 104 edges embedded inline); `.autoharness/drift-ignore` re-apply obligation extended to cover this cycle's two changed files |
| Security | sequential (single-agent) | confirms the new recipe issues only read-only `SELECT`/list calls, introduces no destructive command, and that the P2-3 review-cycle-budget note narrows no policy limit and touches no circuit-breaker file |

### Gate rationale

The gate is **ADVISORY**. The 42-task / 104-executable-edge / 43-shipment-member topology and the
18-wave schedule (0 stalls, 0 compile-order violations) that cycle 29 introduced are re-confirmed
**SOUND** and unchanged — this cycle finds no P0 and no P1. The three P2 findings below are: a
residual misclassification left behind by cycle 29's own P-002.3 fix (the normative policy text and
`build-feature`'s Steps 0/2 were corrected, but one Behavioral Constraints bullet in the same file
still named a missing marker as a false-green signal); a missing efficient-computation recipe for
the wave admission and requery steps that same cycle introduced (P-002.6 defined the ordering rule
but not how `ready_k` is computed without a per-task-per-wave query pattern that would cost up to
756 calls on this release unit's own schedule); and a procedural note recording that this
shipment's remediation arc (cycles 26-30) has now run past the global `github-pr-automation` §1.8
three-review-fix-cycle budget, authorized by the operator as a one-shipment advisory exception
rather than a change to any policy file. Two further P3 items were investigated for accuracy: one
(the PowerShell-quote artifact in `.backlogit/memories.json`) was a genuine, mechanical defect and
is corrected; the other (a possible U1d fail-count discrepancy) was investigated in depth and found
**not** to be a defect — see P3-1 below. None of these findings bears on architecture, safety, or
the implementation contract, so the gate is advisory rather than blocking, and every P2 finding is
corrected (or, for P2-3, recorded as an authorized advisory exception) in this same pass under the
`operator_authorization` above. `push_allowed: yes`.

### Findings by severity

**P0 — 0**

None.

**P1 — 0**

None.

**P2 — 3**

| ID | Finding | Disposition |
|---|---|---|
| P2-1 | Cycle 29's own contract-changes table states P-002.3's must-fail signal list "no longer counts an unmarked exit 0 as the required pre-work failure" and that it "is `EXEMPT_MARKER_MISSING`", and `workflow-policies.md`'s normative P-002.3 text is already correct throughout (the pre-work-probe three-way classification, the false-green signal table, and the explicit sentence that a missing marker is "deliberately **absent**" from that table). But `build-feature/SKILL.md`'s Behavioral Constraints section still read "Never treat an exit-0 run carrying a P-002.3 false-green signal — including a missing `EXEMPT_VERIFY_OK:{task_id}` marker on `exempt_gate_cmd` — as success", which classifies the missing marker itself *as* a false-green signal — the exact conflation cycle 29 withdrew from the normative section, left standing in the one place that paraphrased it for the skill's own constraints. Steps 0 (item 4) and 2 of the same file already stated the two as separate conditions and needed no change. | Fixed in this pass. The bullet is split into two siblings: "Never treat an exit-0 run carrying a P-002.3 false-green signal as success" and "Never treat an exit-0 run on `exempt_gate_cmd` that is missing its declared `EXEMPT_VERIFY_OK:{task_id}` marker as success — this is the distinct `EXEMPT_MARKER_MISSING` evidence failure ..., explicitly **not** a P-002.3 false-green signal". A repository-wide sweep of "false-green" in the file confirmed this was the only occurrence needing correction. |
| P2-2 | P-002.6, introduced this same cycle-29 pass, defines wave admission (`ready_k`) and requery as an ordering rule but does not say how the underlying status/dependency snapshot is computed. The naive form — one dependency lookup per still-queued task, repeated at every wave's admission and requery — costs up to `count(T) × waves` calls; on this release unit's own 42-task / 18-wave schedule that is as many as 756 calls (the "avoid 750 calls" figure this cycle was scoped against) for information a single snapshot already contains, and nothing in the policy said not to compute it that way, nor what to do if the snapshot itself were unreliable. | Fixed in this pass. P-002.6 gains an "Efficient wave-set computation (snapshot recipe)" paragraph: one status+dependency snapshot call per wave — a single `SELECT i.id, i.status, d.depends_on, d.dep_type FROM items i LEFT JOIN item_deps d ON d.item_id = i.id WHERE ...` (`backlogit query` / `backlogit_query_sql`) when `.autoharness/backlog-registry.yaml` declares `features.sql_query: true` (true for this workspace), or the equivalent one-call `list_tasks` fallback (`backlogit list --type task --json` / `backlogit_list_items`, which already embeds each item's `dependencies` inline) preserving CLI/MCP parity when it is not. Both paths were exercised live against this worktree's index during this review (the join returned 42 items and 104 edges in one call; the list call returned the same 104 edges embedded inline in one call) to ground the recipe in the workspace's actual capabilities rather than invented tooling. A new halt code, `WAVE_SNAPSHOT_UNRELIABLE` (P-002.2), fails closed when the snapshot call errors, returns non-parseable output, or returns a task count that does not match the release unit's known task count — the snapshot is never reused across waves and never replaced by a per-task fallback. `.ship.agent.md` Step 4.0 item 1 gains a one-clause cross-reference to the recipe. `workflow-policies.md` 1.17.0 → **1.18.0**. |
| P2-3 | Cycles 26 through 30 form one continuous remediation arc against PR #377's review threads on a single HEAD lineage. `github-pr-automation.instructions.md` §1.8 (mirrored in `circuit-breaker.instructions.md`, `constitution.instructions.md`, `.ship.agent.md`, `AGENTS.md`, and `pr-lifecycle/SKILL.md`) limits review-fix cycles to 3 per HEAD; cycle 26's own memory record already named cycles 26-28 as "review-fix cycle 1 of 3" through "third review-fix cycle" and stated "[a]ny further Copilot finding is a backlog follow-up, not a fix in this session — and per §1.8 it remains merge-blocking rather than waived." Cycles 29 and 30 are, by that accounting, a fourth and fifth cycle against the same budget. | Recorded, not fixed by relaxing any limit. Closed as an operator-authorized procedural advisory, consistent with the cycle-26 precedent quoted above: the operator's `operator_authorization: approved` for this bounded cycle-29/30 pass extends the review-fix budget for **this shipment's remediation arc only**, by explicit direction, recorded here and in `.backlogit/memories.json`. `circuit-breaker.instructions.md`, `constitution.instructions.md`, `.ship.agent.md`, `AGENTS.md`, `github-pr-automation.instructions.md` §1.8, `pr-lifecycle/SKILL.md`, and the `plugin/` agent bundle are **unmodified** — the global 3-cycle limit stands for every other HEAD and every other shipment. This does not clear the §1.9 readiness gate's Check 3, which remains a separate, later gate, and does not waive the merge-blocking consequence §1.8 itself states. |

**P3 — 2**

| ID | Finding | Disposition |
|---|---|---|
| P3-1 | Investigated whether `147.032-T` (U1d)'s baseline-probe FAIL count is understated: the task's table names three functions and its "Expected red" and acceptance-criteria prose say "three ... source-shape harness functions" / "the three `TestU1d_*` ... fail on assertions, verified", while its own "Baseline probe" paragraph, the plan's "Units removed from the exempt set in cycle 29" summary table (`2/2 assertion FAILs`), this same cycle's own new remediation narrative above ("each exit 1 with two assertion failures"), the cycle-29 memory document, and `.backlogit/memories.json` all instead record **two**. | **No change made — investigated and found not to be a confident defect.** `147.038-T` (U15) shows the identical structure: three named functions, the third's pre-work failure message explicitly noted as coinciding with an earlier function's case ("fails with case 1 until the type exists" for U1d's third function; "fails with case 2 until the wrapper pair exists" for U15's), and only two named in its own baseline probe — a symmetric pattern across both sibling units, not a one-off slip. The cycle-29 memory document's own "Task widths" line independently records "3 scenarios" for U1d in the same document that records "`--- FAIL` ×2" three lines above it, showing the two figures were not conflated by that document's own author. Because `internal/events/checkpoint_remediation_test.go` does not exist at this HEAD (the task is still `queued`; no production Go was run this cycle), neither "the three functions all fail once scaffolded" nor "only two of the three fail" is mechanically verifiable now. Silently changing the recorded "2" to "3" would assert an observation nobody has made; silently leaving a real defect uncorrected would also be wrong. This record leaves every occurrence of both numbers exactly as found and defers the question to `harness-architect`'s actual wave-1 scaffolding of this harness, when `go test -v -run '^TestU1d_'`'s real output resolves it without guessing. |
| P3-2 | `.backlogit/memories.json`'s `stage-pr377-cycle-29` entry recorded the go test selector as `-run='^\$'` (one literal backslash before the `$`), while every other occurrence of this selector across the plan, the task files, and the cycle-29 memory document reads `-run='^$'` with no backslash. PowerShell's escape character is a backtick, not a backslash, so the stray character had no quoting function where it was written — it reads as a leaked shell-quoting artifact from whatever command persisted the entry. | Fixed in this pass. The stored value is corrected to `-run='^$'`, matching every other occurrence of the selector; the JSON file re-validated as parseable after the edit. No other file in the repository carries the malformed `^\$` form. |

### Rejected / stale findings

| Claim | Disposition |
|---|---|
| `147.038-T` (U15)'s baseline-probe/memory FAIL count should also move from two to three, matching U1d, since it shows the identical three-named/two-observed pattern | Rejected for this pass — P3-1 above applies equally to U15 and was resolved as "no confident defect" for both units together; there is accordingly nothing to change for U1d that would leave U15 inconsistent. |
| The circuit-breaker limit files (`circuit-breaker.instructions.md`, `constitution.instructions.md`, `.ship.agent.md`, `AGENTS.md`, `github-pr-automation.instructions.md`, `pr-lifecycle/SKILL.md`, `plugin/`) should be amended to reflect this shipment's longer remediation arc | Rejected — the operator's direction is explicit: do not weaken any limit globally. P2-3 is recorded as a one-shipment advisory exception only, and every one of these files is unmodified. |
| The 42-task / 104-edge / 43-member topology, the ready set, or the eight-unit `harness-exempt` set needs restructuring | Rejected — re-confirmed against the live `147-F` queue via the worktree-bound CLI (`sync`, `query`, `list`, `shipment get`) at this session's HEAD; unchanged by this gate. |

### Remediation queue and authorization

restage_recommendation: none (advisory corrections applied in-pass; one P3 item investigated and
left unchanged by design)

All findings in this cycle are P2 or P3. The three P2 findings are fixed, or — for P2-3 — recorded
as an authorized advisory exception, in this same pass under `operator_authorization: approved`. No
P0 or P1 finding exists. `build-feature/SKILL.md`, `workflow-policies.md` (1.17.0 → 1.18.0),
`.ship.agent.md`, `.autoharness/drift-ignore`, this plan, `147-F.md`, `.backlogit/memories.json`,
and a new session checkpoint are updated to record this gate outcome and the corrections applied.
Feature `147-F`'s plan-review state is set to **ADVISORY authorized / ready for push**;
`push_allowed: yes`. **This authorization is autonomous completion of this bounded review-and-fix
cycle, not merge approval** — it does not authorize a shipment claim, does not authorize Ship to
begin implementation, and does not itself resolve the §1.9 readiness gate's Check 3, which remains
a separate, later gate pending fresh PR-side review of the pushed HEAD. No subagents were
dispatched, nothing was pushed or merged, and no Go command was run this cycle.

| Item | Owner | State |
|---|---|---|
| Push `chore/stage-130-s` and let PR #377 checks run | operator / Stage | ready |
| Reply to and resolve the PR #377 review threads on the new HEAD | Ship / pr-lifecycle | unblocked — ready once the branch is pushed |
| Operator merge approval (P-014) | operator | not requested |

This authorization covers **planning, policy, and skill-prose artifacts only**. It is not merge
approval, not a shipment claim, and not authorization for Ship to begin implementation.

## Plan Review

<!-- BEGIN:plan-review -->
```yaml
cycle: 31
reviewed_at: 2026-08-26
reviewed_head: 5212ee45c0a36c9255fcf89b5fe61d4804057c45
dispatch_mode: single-agent-declared-degradation
tool_degraded: reviewer-subagent-dispatch
decision: FAIL
pending: fresh-local-review-required
operator_authorization: approved
severity_counts: "P0=0, P1=6 (all remediated in-pass), P2=0, P3=0"
topology: "42 tasks, 104 executable edges, 43 shipment members, 18 waves, acyclic"
push_allowed: no
restage_recommendation: none
```
<!-- END:plan-review -->

decision: FAIL — pending fresh review

**This record is the current gate state.** It supersedes `cycle: 29` `ADVISORY` for gate-decision
purposes. Cycle 29's record and every earlier record remain the historical trace of how the plan
reached its present shape; only the gate-state role (`decision`, `push_allowed`) is superseded here.

The gate is **FAIL** because this cycle remediated **six P1 review threads** that had been
acknowledged and deliberately HELD across cycles 29 and 30. A cycle that changes the harness
lifecycle plan-wide, the wave scheduler's completion semantics, and the per-task green gate is not
self-certifying: the corrected contract has not yet been reviewed by anything other than the
session that wrote it. `decision: FAIL` with `pending: fresh-local-review-required` is the honest
state, and it keeps §1.9 Check 3 blocking until a fresh local review of *these* changes is run.

### Authorization basis

`operator_authorization: approved` records that the operator explicitly directed autonomous
remediation of the six unresolved PR #377 P1 threads in one bounded cycle, with an explicit scope
of **no subagents, no push, no merge, no production Go**. That scope was honoured: zero `*.go`
files were modified, zero subagents were dispatched, and nothing was pushed or merged. The
authorization covers applying the corrections and recording this gate outcome. It is **not** merge
approval, **not** a shipment claim, and **not** authorization for Ship to begin implementation.

### Dispatch record — degraded to a single-agent sequential pass

Reviewer sub-agent dispatch was unavailable (the operator's explicit "no subagents" scope);
`TOOL_DEGRADED: reviewer-subagent-dispatch` records the degradation, matching the
cycle-16/17/18/20/21/24/29 precedent. The pass ran sequentially over the six thread anchors, the
1.18.0 policy contract, `.ship.agent.md`, `build-feature/SKILL.md`, `harness-architect/SKILL.md`,
`.autoharness/drift-ignore`, and the referenced backlog artifacts, re-verified live.

### Findings by severity — the three roots behind six threads

| ID | Sev | Root | Threads | Disposition |
|---|---|---|---|---|
| R1 | P1 | Declaration stub landing **before** the harness that gates it | `147.001-T:26`, `147.030-T:44`, `147.031-T:37`, `147.034-T:44` | **Fixed.** Withdrawn plan-wide; harness-first ordering is now normative in P-002.1 |
| R2 | P1 | Wave state machine had **no disposition for a `blocked` member**, permitting a false completion | `workflow-policies.md:534` | **Fixed.** Immutable manifest set `M`, total status partition, `WAVE_MEMBER_BLOCKED` |
| R3 | P1 | Wave order **unsatisfiable for `\|ready_k\| > 1`** — per-task loop required the full suite against sibling REDs | `.ship.agent.md:410` | **Fixed.** Scoped per-task loops; full suite relocated to new Step 4.6 convergence gate |

**R1 — declaration doctrine.** Cycle 29 withdrew the `declaration-only` exemption class but did not
propagate the corrected lifecycle into the staged tasks, leaving four of them instructing "land the
declaration stub so the package compiles, then the harness, then implement". That is the same
Principle II carve-out by another route — the stub is observable production surface admitted with
no observed failing test — and it was additionally *unschedulable*, because `harness-architect` no
longer fabricates the missing declaration. The plan's three-step lifecycle is replaced by a
**two-step** one: red harness first (source-shape whenever the delta introduces the symbol),
green step second. `147.034-T` / U11 was the most severe: its stub was specified to perform real
read/parse/mutate/marshal/atomic-replace behaviour, so it is corrected to the
declaration → behaviour-harness → implementation split (U11 → U12 → U13), with U11 gated by its own
`TestU11_` source-shape red and its behaviour red relocated to U12. The sweep also caught two
class-matched siblings the threads did not name — `147.005-T` / U2d (empty-map stub) and
`147.002-T` / U2 (`return nil` stub) — plus the plan's Implementation Units head, the
Green-step-guards paragraph, the U1/U1b/U1c/U2/U2d/U11/U12/U15 unit sections, invariant I4, and
Constitution Check II. Historical amendment-log rows are preserved unmodified as history.

**R2 — blocked accounting.** `queued` was overloaded between a derived residual (`M \ done`) and a
literal status, and neither reading is safe: the derived one silently re-admits a returned member,
the literal one leaves it in no examined set at all so `WAVE_NO_PROGRESS` never fires. P-002.6 now
freezes an **immutable manifest member set `M`** at Ship Step 0.5 — never re-derived from the
shipment's live `items` list, because `core.ReturnBlockedItem` (`internal/core/shipment.go:626-665`)
removes a returned member from that list — and defines the **total** partition
`M = queued ⊎ active ⊎ blocked ⊎ terminal` over the five recognized statuses. Completion is
`terminal = M` and nothing else; an empty frontier is never completion. Any blocked member halts
with the new `WAVE_MEMBER_BLOCKED` code, reporting the full census, each `blocked_reason`, the
transitive dependency impact, and whether `return_blocked` has been invoked/recorded. An `active`
residual halts with `WAVE_NO_PROGRESS`. A **blocked-injection validation** is now mandatory before
a wave schedule may be relied on.

**R3 — wave green semantics.** Step 4.0 left the whole wave red at once while `build-feature`'s
post-loop gate required `go test ./...`, so the first task of any multi-member wave could never
reach green. Eleven of this release unit's eighteen waves carry more than one non-exempt member, so
this failed on wave 2 of the worked schedule. `harness-architect` may still batch-scaffold the
whole wave; the per-task loop now uses only the task's **scoped** command, with an explicit, closed,
non-widenable `sibling_red_selectors` tolerated-red set; and the full suite is **relocated, not
skipped**, to a new mandatory **Ship Step 4.6 wave convergence gate** that must be fully green
before the next wave is admitted. Six task-scoped command requirements prevent the scoping from
becoming a weakening, and every branch-wide gate is preserved unchanged.

### Simulation evidence

Re-run at this cycle's HEAD against the live `.backlogit/queue` manifest; 17/17 assertions PASS.

| Simulation | Result |
|---|---|
| Baseline replay (42 tasks / 104 edges) | `COMPLETE`, **18 waves**, 42/42 scheduled, **0** stalls, **0** compile-order violations; sizes 2,2,4,5,4,4,3,2,3,3,1,2,2,1,1,1,1,1 |
| Blocked-member injection (`147.030-T` → `blocked`) | `WAVE_MEMBER_BLOCKED` at wave 1; **no** false completion; member retained in `M`; dependency impact **26** members |
| Blocked mid-run variant | `WAVE_MEMBER_BLOCKED` |
| Active-residual variant | `WAVE_NO_PROGRESS` (detail: `active residual`) |
| Unrecognized-status variant | `WAVE_MEMBER_BLOCKED` (fails closed) |
| Non-frozen-`M` control (member dropped) | `WAVE_NO_PROGRESS` at wave 10 — the failure mode the frozen-`M` contract exists to prevent |
| Sibling-red wave (wave 2, 2 non-exempt) | OLD repo-wide gate progressed **0/2** (original P1 deadlock reproduced); NEW scoped gate progressed **2/2**; 1 convergence full-suite run; 0 full-suite runs inside per-task loops |
| Negative control (failure outside tolerated set) | gate **HALTS** — the tolerated-red set is not a blanket escape hatch |

### Validation evidence

| Gate | Result |
|---|---|
| Markdown P-008 (`markdownlint-cli2` 0.23.1, repo-wide) | 0 issues in 2291 files |
| Docline frontmatter (`backlogit docs lint`) | `valid: true`, 0 violations |
| Index sync (`backlogit sync`) | 1209 artifacts indexed, 0 parse failures |
| Topology (`backlogit query`) | 42 queued + 1 archived task, 104 executable edges, 43 shipment members — unchanged |
| `go build ./...` / `go vet ./...` | exit 0 / exit 0 |
| `go test ./...` | **exit 0**, 29 `ok`, 0 `FAIL` |
| Production Go touched | **none** — 0 `*.go` files modified |

`gofmt -l .` lists every file in this checkout, a pre-existing CRLF artifact of the Windows
worktree rather than a finding: `git status` shows zero `*.go` modifications this cycle, so its
output is byte-identical to the baseline.

### Remediation queue and authorization

All six threads are remediated in-pass. The gate is nonetheless **FAIL pending fresh review**,
because the corrected contract has been reviewed only by the session that authored it.

| Item | Owner | State |
|---|---|---|
| Fresh local plan review of the cycle-31 contract | Stage / plan-review | **required before push** — this is what `decision: FAIL` blocks on |
| Push `chore/stage-130-s` | operator / Stage | blocked on the review above |
| Reply to and resolve the six PR #377 threads on the new HEAD | Ship / pr-lifecycle | blocked on push |
| Operator merge approval (P-014) | operator | not requested |

This authorization covers **planning, policy, and skill-prose artifacts only**. It is not merge
approval, not a shipment claim, and not authorization for Ship to begin implementation. The §1.9
readiness gate remains **FAIL on Check 3**.

## Plan Review

<!-- BEGIN:plan-review -->
```yaml
cycle: 32
reviewed_at: 2026-08-26
reviewed_head: d57bbd8a456383c0a98283f2637642b34ffa5c54
dispatch_mode: single-agent-declared-degradation
tool_degraded: reviewer-subagent-dispatch
decision: FAIL
pending: fresh-local-review-required
operator_authorization: approved
severity_counts: "P0=0, P1=2 (both remediated in-pass), P2=3 (all remediated in-pass), P3=0"
topology: "42 tasks, 104 executable edges, 43 shipment members, 18 waves, acyclic"
push_allowed: no
restage_recommendation: none
```
<!-- END:plan-review -->

decision: FAIL — pending fresh review

**This record is the current gate state.** It supersedes `cycle: 31` `FAIL` for gate-decision
purposes. Cycle 31's record and every earlier record remain the historical trace of how the plan
reached its present shape; only the gate-state role (`decision`, `push_allowed`) is superseded here.

This cycle is the **fresh local review of the cycle-31 contract** that cycle 31's `FAIL` blocked on.
It found **two P1 defects and three P2 defects in that contract** and remediated all five in-pass.
The gate stays `FAIL` for the same structural reason cycle 31 gave, now applied to this cycle's own
changes: the corrected contract has again been reviewed only by the session that authored it.

### Authorization basis

`operator_authorization: approved` records that the operator explicitly directed autonomous
remediation of the cycle-32 findings in one bounded cycle, with an explicit scope of **no
subagents, no push, no merge, no production Go**. That scope was honoured: zero `*.go` files were
modified, zero subagents were dispatched, and nothing was pushed or merged. The authorization
covers applying the corrections and recording this gate outcome. It is **not** merge approval,
**not** a shipment claim, and **not** authorization for Ship to begin implementation.

### Dispatch record — degraded to a single-agent sequential pass

Reviewer sub-agent dispatch was unavailable (the operator's explicit "no subagents" scope);
`TOOL_DEGRADED: reviewer-subagent-dispatch` records the degradation, matching the
cycle-16/17/18/20/21/24/29/31 precedent. The pass ran sequentially over the 1.19.0 contract
(P-002.2, P-002.6), `.ship.agent.md` Steps 0.5/3/4.0/4.2/4.3/4.5/4.6/5, `build-feature/SKILL.md`,
`harness-architect/SKILL.md`, the live status model (`.backlogit/config.yaml`,
`.autoharness/backlog-registry.yaml`, `internal/models/artifact.go`,
`internal/core/status_taxonomy.go`, `internal/core/shipment_gate.go`), and the 43 staged backlog
artifacts, re-verified live against the worktree.

### Findings by severity

| ID | Sev | Root | Anchor | Disposition |
|---|---|---|---|---|
| G1 | P1 | Step 4.6 asserted an **empty** tolerated-red set, which is false from wave 4 onward | `.ship.agent.md` Step 4.6 item 2; `workflow-policies.md` P-002.6 *Wave green semantics* | **Fixed.** `open_red_deliverables_k`, conditional unfiltered suite, explicit deferral, `WAVE_OPEN_RED_UNCLOSED` |
| G2 | P1 | Status model cited a **five-token** set that exists in no source | `workflow-policies.md` P-002.6 *Definitions* | **Fixed.** Configured executable / terminal-success / unsupported partition, `WAVE_STATUS_UNSUPPORTED`, `WAVE_STATUS_CATALOG_UNAVAILABLE` |
| G3 | P2 | `M`'s freeze anchor named Step 0.5, which never enumerates members | `workflow-policies.md` P-002.6; `.ship.agent.md` Step 4.0 item 1 | **Fixed.** Anchored to Step 3, with a defined non-shipment fallback and `WAVE_MANIFEST_UNAVAILABLE` |
| G4 | P2 | The mandatory scheduler validation was a **session claim**, not a re-runnable artifact | `workflow-policies.md` P-002.6 *Blocked-injection validation* | **Fixed.** Tracked read-only fixture + PowerShell runner, 84/84 assertions |
| G5 | P2 | "any package that was already green before this wave began" was discretionary | `.ship.agent.md` Step 4.3 item 3; `build-feature/SKILL.md` post-loop gate 3 | **Fixed.** Replaced by the task-declared closed list `green_regression_cmds` |

**G1 — open red across waves.** Cycle 31 relocated the full suite to a new Step 4.6 and justified an
empty tolerated-red set with "every current-wave harness is now green because every member has been
built". That premise holds only for waves whose members are all build-to-green tasks. Three members
of this release unit are **red deliverables** — tasks whose declared deliverable *is* a failing
harness, turned green by a later task — and two of them, `147.016-T` / U8b and `147.042-T` / U3c,
complete in **wave 4**. An unfiltered `go test ./...` at wave 4's convergence gate would fail on
exactly the artifacts the plan asked those tasks to produce, and Step 4.6 forbids advancing on a
failing gate, so the release unit would deadlock at wave 4 — the same class of unsatisfiable gate
cycle 31 fixed one level up. The correction defines `open_red_deliverables_k` as the closed set of
selectors belonging to completed red deliverables whose declared green-makers are not all `done`,
derived **mechanically** from a new `red-deliverable-contract` block in the task body and failing
closed with `WAVE_RED_MAPPING_UNRESOLVED` on a missing, empty, unknown, self-referential, or
not-strictly-later mapping. Convergence **always** runs the repo-wide compile check, `go vet`,
lint, format, and the wave's closed list of declared scoped commands; the **unfiltered** suite runs
if and only if the open-red set is empty, and is mandatory at final closure.

**Why deferral rather than a classified full run.** Both options in the finding were evaluated. A
classified full run (`go test ./...`, admitting only failures matching the open-red selectors) is
the *less* robust one: Go aborts a package on a build error, panic, or timeout, so a genuine
unrelated failure inside a package that also contains an open red produces no `--- FAIL:` line to
classify and would be silently absorbed by the tolerated set — precisely the hidden-unexpected-
failure mode the gate exists to prevent. Deferral has no classification step and therefore no such
mode: what stays repo-wide (compile, vet, lint, format) cannot be reddened by a designed red at
all, every declared scoped command is run and must pass, the deferral is recorded explicitly with
its selectors and unclosed green-makers, and it is bounded by `green_maker_closes_wave` and
`WAVE_OPEN_RED_UNCLOSED` rather than left open-ended. On this schedule the deferral spans waves
4–12 and discharges at **wave 13**, the wave that completes U8b's last green-maker.

**G2 — status model.** P-002.6 1.19.0 stated that `status(t)` "is exactly one of the five
recognized tokens `queued`, `active`, `blocked`, `done`, `archived`
(`internal/models/artifact.go:17-22`)". That citation is wrong: those lines declare **ten**
`ArtifactStatus` constants, and `.backlogit/config.yaml` `fields.status.values` enumerates the same
ten. Five real lifecycle tokens — `review`, `accepted`, `rejected`, `shipped`, `abandoned` — had no
stated disposition, and the one clause that mentioned unknown tokens folded them into
`WAVE_MEMBER_BLOCKED`, a code whose report format promises a `blocked_reason` and a `return_blocked`
record that none of them has. The model is now read from the **configured** sources at Step 3 — the
workspace catalog, the registry `status_values` mapping, and the actual archive lifecycle — and `M`
is partitioned into configured **executable** (`queued`, `active`, `blocked`), configured
**terminal-success** (`done`, `archived`), and **`unsupported`** defined as the complement, which
makes the partition total *by construction* rather than by enumeration. An unsupported member halts
with `WAVE_STATUS_UNSUPPORTED` naming each ID and its observed token; an unreadable, empty, or
self-inconsistent model halts with `WAVE_STATUS_CATALOG_UNAVAILABLE` rather than falling back to a
built-in vocabulary. A related sub-finding is fixed alongside: `archived` satisfies a *dependency*
but does **not** discharge a *green-maker* obligation, because a member may be archived as a
descope (`archivedFromDescopeEligibleStatus`, `internal/core/shipment_gate.go`).

**G3 — freeze anchor and the non-shipment path.** 1.19.0 froze `M` "at Ship Step 0.5 shipment
intake". Step 0.5 loads the shipment, validates membership, creates the branch, and claims — it
never enumerates members at every status, and it is explicitly conditional on
`features.shipments: true`. Step 3 is the step that lists every member; the anchor is moved there,
Step 0.5 now says in one sentence that it does *not* freeze `M`, and the non-shipment path is
defined rather than left implicit: freeze from the covering release unit's declared child
enumeration, recorded verbatim in the session record and checkpoint, with `WAVE_MANIFEST_UNAVAILABLE`
when neither source is available or the two disagree.

**G4 — the validation became an artifact.** P-002.6 requires a blocked-injection replay before a
wave schedule may be relied on, but cycle 31 satisfied it with an in-session simulation described
only in a review record. The next reader cannot re-run a paragraph. The replay is now carried by
`tests/simulation/wave-scheduler-contract.json` (a tracked fixture mirroring the live queue) and
`scripts/wave-scheduler-sim.ps1` (a pure, read-only runner). No Go was added: the repository's test
tree is Go-only and this cycle's scope forbids production Go, so the artifact uses the PowerShell
already present under `scripts/` plus a JSON fixture that CI, `jq`, and any reader can parse.

**G5 — discretionary package selection.** Both Ship Step 4.3 and `build-feature`'s post-loop gate
allowed "any package that was already green before this wave began". That set was never enumerated,
never recorded, and could be narrowed to nothing or widened into a sibling's red without any
artifact showing it — implementer judgement inside a gate that had just been tightened elsewhere.
It is replaced by `green_regression_cmds`: a task-declared, closed, diffable list, empty unless the
task declares otherwise.

### Simulation evidence

Committed artifact, re-runnable by anyone:
`pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue` → **`WAVE_SIM_OK:
84/84 assertions PASS across 16 scenario(s)`**, exit 0. The runner writes nothing, starts no
process, and runs no build, so it clears the P-002.5 read-only screen.

| Scenario | Result |
|---|---|
| `baseline` | `COMPLETE`, **18 waves**, 42/42 scheduled, sizes 2,2,4,5,4,4,3,2,3,3,1,2,2,1,1,1,1,1, **0** stalls, **0** compile-order violations, 19 snapshot calls (one per wave plus the completion snapshot) |
| `persistent_red_mapping` | Wave 4 **advances**; open red after wave 4 = `{147.016-T, 147.042-T}`, after wave 6 = `{147.016-T, 147.035-T, 147.042-T}`; entries close at waves 7 / 8 / 13; unfiltered full suite at waves 1,2,3,13,14,15,16,17,18; deferred at waves 4–12; compile gate at all 18; **0** hidden unexpected failures |
| `blocked_injection` | `WAVE_MEMBER_BLOCKED` at wave 1, dependency impact **26**, member retained in `M`, no completion claim |
| `blocked_mid_run` | `WAVE_MEMBER_BLOCKED` at wave 5 |
| `active_residual` | `WAVE_NO_PROGRESS` (detail `active residual`) at wave 1 |
| `unsupported_status_review` / `_abandoned` / `_off_catalog` | `WAVE_STATUS_UNSUPPORTED` at wave 1 in all three, naming the member and token; no false completion |
| `status_catalog_unavailable` / `status_catalog_disagrees` | `WAVE_STATUS_CATALOG_UNAVAILABLE` at schedule construction |
| `cycle_injection` | `WAVE_CYCLE_DETECTED` at schedule construction with a non-empty cycle path |
| `sibling_red_wave4` | 5 non-exempt members; withdrawn repo-wide gate progresses **0/5**; task-scoped gate progresses **5/5**; **0** full-suite runs inside per-task loops |
| `non_frozen_m_control` | `WAVE_NO_PROGRESS` at wave 10 with `147.030-T` dropped from the accounting universe — the failure the frozen-`M` contract prevents |
| `missing_green_maker` / `ambiguous_green_maker` | `WAVE_RED_MAPPING_UNRESOLVED` at schedule construction |
| `green_maker_descoped` | Green-maker `archived` instead of `done` → obligation stays open → `WAVE_OPEN_RED_UNCLOSED` at wave 8 |

### Validation evidence

| Gate | Result |
|---|---|
| Wave scheduler simulation | `WAVE_SIM_OK` 84/84, exit 0, fixture verified against the live queue |
| Markdown P-008 (`markdownlint-cli2`, repo-wide) | 0 issues |
| Docline frontmatter (`backlogit docs lint`) | `valid: true`, 0 violations |
| Index sync (`backlogit sync`) | 0 parse failures |
| Topology (`backlogit query`) | 42 tasks, 104 executable edges, 43 shipment members — unchanged |
| `go build ./...` / `go vet ./...` | exit 0 / exit 0 |
| `go test ./...` | exit 0 |
| Production Go touched | **none** — 0 `*.go` files modified |

### Remediation queue and authorization

All five findings are remediated in-pass. The gate is nonetheless **FAIL pending fresh review**:
this cycle rewrote the scheduler's status model, its manifest freeze anchor, and its convergence
semantics, and — exactly as in cycle 31 — the corrected contract has been reviewed only by the
session that authored it. The difference from cycle 31 is that the review obligation is now
partly mechanized: `scripts/wave-scheduler-sim.ps1` re-checks the behavioural claims on demand, so
the fresh review can concentrate on whether the *contract* is right rather than on whether the
*simulation* was really run.

| Item | Owner | State |
|---|---|---|
| Fresh local plan review of the cycle-32 contract | Stage / plan-review | **required before push** — this is what `decision: FAIL` blocks on |
| Push `chore/stage-130-s` | operator / Stage | blocked on the review above |
| Reply to and resolve the PR #377 threads on the new HEAD | Ship / pr-lifecycle | blocked on push |
| Operator merge approval (P-014) | operator | not requested |

This authorization covers **planning, policy, skill-prose, and simulation artifacts only**. It is
not merge approval, not a shipment claim, and not authorization for Ship to begin implementation.
The §1.9 readiness gate remains **FAIL on Check 3**.

## Plan Review

<!-- BEGIN:plan-review -->
```yaml
cycle: 33
reviewed_at: 2026-08-26
reviewed_head: c9fa17044cdd0585d0cfa7a2cca54f15adcb6f4d
dispatch_mode: single-agent-declared-degradation
tool_degraded: reviewer-subagent-dispatch
decision: FAIL
pending: independent-review-required
operator_authorization: approved
severity_counts: "P0=0, P1=2 (both remediated in-pass), P2=2 (both remediated in-pass), P3=0"
topology: "S=43 explicit shipment members; M=42 task-type IDs; excluded 147-F (feature); 104 executable edges; 18 waves; acyclic"
push_allowed: no
restage_recommendation: none
```
<!-- END:plan-review -->

decision: FAIL — pending independent review

**This record is the current gate state.** It supersedes `cycle: 32` `FAIL` for gate-decision
purposes. Cycle 32 and every earlier record remain the historical trace; their remediation content
stands except where this cycle explicitly narrows the manifest, verification, citation, and
green-regression declaration semantics.

### Baseline

Before edits,
`pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue` reported
`WAVE_SIM_OK: 84/84 assertions PASS across 16 scenario(s)`. That pass is the baseline and also
reproduces H2: verification globbed `147.*-T.md` rather than parsing `130-S.md`, compared only a
count plus fixture-member fields, and checked only `green_maker_tasks` from each red-deliverable
block. It could therefore pass while shipment membership, the excluded covering feature, the red
flag/reason/selector, or the close-wave declaration drifted.

### Findings and remediation

| ID | Sev | Finding | Remediation |
|---|---|---|---|
| H1 | P1 | Policy and Ship said every shipment member was `M`, while the fixture and plan scheduled 42 tasks from a 43-member manifest | Defined complete shipment set `S`; `M` is exactly task-type IDs in `S`; report excluded IDs/types; `130-S`: S=43, M=42, excluded `147-F` (`feature`) |
| H2 | P1 | The scheduler drift gate did not parse the shipment and omitted four of five red-contract keys | Parse `130-S.md`; resolve each item type; exact-set compare filtered task IDs with `M`; compare all five keys; nine data-driven in-memory mutations; selector and close-wave scheduler negatives |
| H3 | P2 | Descope provenance cited the lower-level `isDescopeEligibleStatus` predicate | Corrected to `archivedFromDescopeEligibleStatus` in `internal/core/shipment_gate.go` |
| H4 | P2 | `green_regression_cmds` had no declaration grammar, so “task declares” was not executable | Added one optional canonical JSON body block, exact absent-block default `[]`, fail-closed malformed-block handling, and explicit statement that no current task needs a block |

**H1 — two accounting universes.** `S` is the shipment artifact's complete explicit item-ID set.
Ship resolves every listed artifact type before filtering. `M = { id in S : artifact_type(id) =
task }` is the immutable scheduler universe. Non-task IDs remain visible in a stable excluded
report, but do not consume a wave, satisfy a task dependency, or appear in a per-wave snapshot.
For `130-S`, `count(S)=43`, `count(M)=42`, and the sole excluded member is `147-F` (`feature`).
The covering-unit fallback is compared with filtered task IDs, not raw `S`; snapshot reliability
expects `count(M)` rows.

**H2 — verification now tests what it claims.** The PowerShell runner reads `130-S.md` first and
loads only the artifacts named there, from queue or archive. It compares shipment IDs, task-filtered
`M`, excluded non-task IDs/types, statuses, dependencies, exemption metadata, the empty
green-regression projection, and every canonical red-deliverable field. Fixture-declared mutations
remove a shipment task, improperly add the feature to `M`, suppress the excluded report, mutate
each of the five red keys, and inject a green-regression command. Every mutation must produce its
specific drift code. Scheduler scenarios also prove an empty selector and a false close wave halt
with `WAVE_RED_MAPPING_UNRESOLVED`.

**H3 — provenance helper.** `isDescopeEligibleStatus` classifies a status value; it does not read
an archived artifact's provenance. The green-maker rule relies on the helper that does that read,
`archivedFromDescopeEligibleStatus`, so the policy, prior memory, plan, and checkpoint cite that
symbol.

**H4 — canonical optional array.** A present `green-regression-contract` is a delimited fenced JSON
object with exactly one key, `green_regression_cmds`, whose value is a non-empty array of unique
command strings satisfying the scoped-command constraints. Absence means exactly `[]`; an empty
block is not authored. Ship freezes the parsed array at Step 3 and `build-feature` consumes it
unchanged. No current `130-S` task needs the block.

### Simulation and validation evidence

The corrected read-only command reports
`WAVE_SIM_OK: 104/104 assertions PASS across 18 scenario(s)`. It also reports the parsed census:
43 total shipment members, 42 task members in `M`, excluded `147-F=feature`, and nine mutation
checks.

| Gate | Result |
|---|---|
| Scheduler simulator + live shipment drift + mutations | `WAVE_SIM_OK` 104/104, 18 scenarios, exit 0 |
| Docline frontmatter (`backlogit docs lint`) | `valid: true`, 0 violations |
| Markdown P-008 (`scripts/md-lint.ps1`, repo-wide) | 0 issues |
| Integration (`go test ./tests/integration/ -count=1`) | exit 0 |
| Index sync (`backlogit sync`) | 0 parse failures |
| Topology | S=43; M=42; excluded `147-F` (`feature`); 104 executable edges; 18 waves; acyclic |
| Production Go touched | **none** — 0 `*.go` files modified |

### Gate and next action

All four findings are remediated in-pass, but this remediation is not self-certifying. The gate
therefore stays **FAIL**, `pending: independent-review-required`, and `push_allowed: no`. The next
action is an independent review of the cycle-33 diff. Push, PR-thread reconciliation, shipment
claim, and merge remain blocked; operator merge approval has not been requested. No subagent was
used, and no push or merge occurred.

## Plan Review

<!-- BEGIN:plan-review -->
```yaml
cycle: 34
reviewed_at: 2026-08-26
reviewed_head: 47925de28b61a39ff3dd3860f8d90a12886a298a
dispatch_mode: single-agent-operator-constrained
subagents: prohibited-by-operator
decision: FAIL
pending: independent-review-required
operator_authorization: approved
severity_counts: "P0=0, P1=2 (both remediated in-pass), P2=2 (both remediated in-pass), P3=1 (remediated in-pass)"
topology: "S=43 explicit shipment members; M=42 exact task IDs; excluded 147-F (feature); forbidden historical sibling 147.010-T absent; 104 executable edges; 18 waves; acyclic"
push_allowed: no
restage_recommendation: none
```
<!-- END:plan-review -->

decision: FAIL - pending independent review

**This record is the current gate state.** It supersedes `cycle: 33` `FAIL` for gate-decision
purposes. Cycle 33 and every earlier record remain historical; their corrections stand except
where this cycle narrows snapshot membership, replaces child enumeration, and strengthens the
simulator's source verification.

### Baseline

At canonical HEAD `47925de28b61a39ff3dd3860f8d90a12886a298a`, before edits:

```text
pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1
WAVE_SIM_OK: 90/90 assertions PASS across 18 scenario(s)

pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue
WAVE_SIM_OK: 104/104 assertions PASS across 18 scenario(s)
```

Those passes reproduced the review gap. The policy's documented SQL selected task rows by
`parent_id = '147-F'`; the live query returned **43** tasks, including retired archived
`147.010-T`, although frozen `M` has 42 IDs. Its non-SQL path used `list --type task`, and its
non-shipment path inferred all task children. The simulator copied status sources from its fixture
without parsing either YAML file, and `frozen_m_counterpart` compared the expected value with
itself.

### Findings and remediation

| ID | Sev | Finding | Remediation |
|---|---|---|---|
| I1 | P1 | SQL snapshot selected by `parent_id`, widening 42-member `M` to 43 task children through archived `147.010-T` | Query `artifact_type = 'task'` plus exact frozen-M IDs; bind placeholders when supported or construct validated, escaped quoted literals for CLI SQL; never query `parent_id` |
| I2 | P1 | Non-SQL and non-shipment fallbacks could enumerate unrelated children | Direct-get each frozen M ID exactly once at every status; forbid `list --type task`; require explicit `frozen_task_ids` in non-shipment mode or halt `WAVE_MANIFEST_UNAVAILABLE` |
| I3 | P2 | `-VerifyAgainstQueue` did not verify the configured status sources or source-selection features | Parse and compare live config status values, registry status mapping, and `sql_query`/`shipments`; add three drift mutations |
| I4 | P2 | `frozen_m_counterpart` was a tautology and no control injected the archived sibling | Exact-compare manifest M with the explicit fallback set; add a `147.010-T` fallback-inclusion mutation; forbid both `147-F` and `147.010-T` |
| I5 | P3 | Green-regression parsing accepted a root array or scalar `green_regression_cmds` | Require a JSON object root and JSON array value; add valid/root-array/scalar parser controls |

**I1/I2 - a closed snapshot universe.** SQL receives only the already-frozen ID set. The result
must contain exactly the 42 distinct task IDs in `M`; dependency join multiplicity does not alter
that cardinality. The direct path issues 42 exact-ID reads and rejects a missing, duplicate, extra,
or non-task response. Both paths include all statuses and cannot discover scope. Covering feature
`147-F` and archived historical sibling `147.010-T` are absent by construction.

**I3/I4 - verification now checks independent sources.** The live drift gate reads
`.backlogit/config.yaml` and `.autoharness/backlog-registry.yaml`, compares the status catalog,
mapping, and snapshot/manifest features, then compares the shipment-filtered M with the explicit
non-shipment set. Thirteen in-memory mutations include each status source and archived-sibling
inclusion. The negative non-frozen-M scenario now carries a real set-equality assertion rather
than an expected-value tautology.

**I5 - bounded P3 folded in.** Shape enforcement was local to the existing parser and did not
require a new declaration grammar. Three synthetic read-only controls prove a valid array passes
and the two previously accepted wrong shapes fail.

### Simulation and validation evidence

| Gate | Result |
|---|---|
| Scheduler simulator | `WAVE_SIM_OK` 93/93, 18 scenarios, exit 0 |
| Live manifest/config/registry drift + 13 mutations | `WAVE_SIM_OK` 115/115, 18 scenarios, exit 0 |
| Actual exact-M SQL snapshot | 42 distinct task IDs / 104 dependency edges; `147-F` and `147.010-T` absent |
| Direct-get sample/full parity | sample PASS; 42/42 exact IDs read once, status/dependency parity with SQL, no extras |
| Markdown P-008 | 0 issues |
| Docline frontmatter (`backlogit docs lint`) | `valid: true`, 0 violations |
| Integration tests | `go test ./tests/integration/ -count=1`, exit 0 |
| Index sync | 0 parse failures |
| Topology | S=43; M=42; 104 executable edges; 18 waves; acyclic |
| Production Go touched | **none** |

### Gate and next action

All five findings are remediated in-pass, but cycle 34 cannot certify its own contract changes.
The gate remains **FAIL**, `pending: independent-review-required`, and `push_allowed: no`.
Independent review of this diff is required before any push. PR-thread reconciliation, shipment
claim, and merge remain blocked; operator merge approval has not been requested. No subagent, Go
source edit, push, or merge occurred.

## Plan Review

<!-- BEGIN:plan-review -->
```yaml
cycle: 35
reviewed_at: 2026-08-26
reviewed_head: c246eee3189485d77930a45327a1f24d5c1fbb2e
dispatch_mode: single-agent-operator-constrained
subagents: prohibited-by-operator
decision: ADVISORY
pending: none
operator_authorization: approved
severity_counts: "P0=0, P1=0, P2=2 (both remediated in-pass), P3=0"
topology: "S=43 explicit shipment members; M=42 exact task IDs; excluded 147-F (feature); forbidden historical sibling 147.010-T absent; 104 executable edges; 18 waves; acyclic"
checkpoint_validation: "18 V1-era checkpoints valid; 9 explicitly accepted pre-V1 legacy files"
push_allowed: yes
restage_recommendation: none
```
<!-- END:plan-review -->

decision: ADVISORY

**Cycle 36 supersedes this record as the current gate state.** Cycle 35 superseded `cycle: 34`
`FAIL` for gate-decision purposes. Cycle 34 and every earlier record remain historical; their
corrections stand. Cycle 35 changed only checkpoint planning data and the executable current-source
checkpoint corpus gate.

### Baseline and correction

At canonical HEAD `c246eee3189485d77930a45327a1f24d5c1fbb2e`, the current-source corpus check
enumerated 26 checkpoint files. Nine explicitly accepted pre-V1 legacy files were distinguished
from 17 V1-era files. Fifteen V1-era files passed, while these two failed `ValidateCheckpoint`
because `prompt-builder` is not an accepted V1 checkpoint agent:

* `checkpoint-20260826-064716.json`
* `checkpoint-20260826-072421.json`

Both files now record `agent: stage`. No other key, value, array member, or context field in either
file changed.

Cycle 36 normalizes J1 from P1 to P2. The invalid agent values made two staged continuity artifacts
unusable, but did not alter production code, the implementation contract, topology, or a persisted
runtime checkpoint mutation. With J1 and J2 both corrected in-pass, cycle 35 has P0=0/P1=0/P2=2
and is `ADVISORY` under `operator_authorization: approved`. That authorization completes the
bounded Stage review; it is not merge approval, not a shipment claim, and not authorization for
Ship to begin implementation.

### Current-source Stage checkpoint gate

Run this exact PowerShell command from the repository root before accepting this plan's Stage gate
or recording any later plan-review result. It uses the current source rather than a pinned binary,
enumerates the complete live checkpoint directory, distinguishes the explicitly accepted pre-V1
set, and fails on every unlisted legacy file, JSON parse error, or V1
`ParseCheckpoint`/`ValidateCheckpoint` error.

```powershell
$ErrorActionPreference = 'Stop'
$checkpointDir = '.backlogit\checkpoints'
$acceptedPreV1 = @(
  'checkpoint-20260406-171334.json',
  'checkpoint-20260411-051040.json',
  'checkpoint-20260421-164238.json',
  'checkpoint-20260424-162622.json',
  'checkpoint-20260424-174043.json',
  'checkpoint-20260424-204116.json',
  'checkpoint-20260426-031618.json',
  'checkpoint-20260426-045333.json',
  'checkpoint-20260801-051014.json'
)
$minimumV1Count = 18
$failures = @()
$legacyCount = 0
$v1Count = 0

if ($acceptedPreV1.Count -ne 9 -or
    @($acceptedPreV1 | Sort-Object -Unique).Count -ne 9) {
  throw 'checkpoint corpus declaration must contain exactly 9 unique pre-V1 filenames'
}

$jsonFiles = @(
  Get-ChildItem -LiteralPath $checkpointDir -File |
    Where-Object { $_.Extension -ieq '.json' } |
    Sort-Object Name
)

$unexpectedJson = @(
  $jsonFiles | Where-Object { $_.Name -notlike 'checkpoint-*.json' }
)
foreach ($unexpected in $unexpectedJson) {
  $failures += "$($unexpected.Name): JSON filename is outside checkpoint-*.json"
}

$checkpoints = @(
  $jsonFiles | Where-Object { $_.Name -like 'checkpoint-*.json' }
)
$checkpointNames = @($checkpoints | ForEach-Object { $_.Name })
foreach ($expected in $acceptedPreV1) {
  if ($expected -notin $checkpointNames) {
    $failures += "$expected`: expected pre-V1 file is missing"
  }
}

foreach ($checkpoint in $checkpoints) {
  try {
    $document = Get-Content -LiteralPath $checkpoint.FullName -Raw |
      ConvertFrom-Json -ErrorAction Stop
  } catch {
    $failures += "$($checkpoint.Name): JSON parse failed: $($_.Exception.Message)"
    Write-Host "JSON_FAIL: $($checkpoint.Name)"
    continue
  }

  if ($acceptedPreV1 -contains $checkpoint.Name) {
    if ($document.PSObject.Properties.Name -contains 'schema_version') {
      $failures += "$($checkpoint.Name): accepted pre-V1 file now declares schema_version"
      Write-Host "PRE_V1_FAIL: $($checkpoint.Name)"
    } else {
      $legacyCount++
      Write-Host "PRE_V1_ACCEPTED: $($checkpoint.Name)"
    }
    continue
  }

  if ($document.PSObject.Properties.Name -notcontains 'schema_version' -or
      $document.schema_version -ne 1) {
    $failures += "$($checkpoint.Name): unlisted file is not schema_version 1"
    Write-Host "UNLISTED_LEGACY_FAIL: $($checkpoint.Name)"
    continue
  }

  $v1Count++
  $output = @(
    & go run .\cmd\backlogit --cwd . --no-update-check checkpoint get $checkpoint.Name 2>&1
  )
  if ($LASTEXITCODE -eq 0) {
    Write-Host "V1_OK: $($checkpoint.Name)"
  } else {
    $failures += "$($checkpoint.Name): $($output -join ' | ')"
    Write-Host "V1_FAIL: $($checkpoint.Name)"
  }
}

if ($legacyCount -ne 9) {
  $failures += "accepted pre-V1 count is $legacyCount; expected exactly 9"
}
if ($v1Count -lt $minimumV1Count) {
  $failures += "V1 count is $v1Count; expected at least $minimumV1Count"
}
if ($failures.Count -gt 0) {
  $failures | ForEach-Object { Write-Host $_ }
  throw "checkpoint corpus validation failed for $($failures.Count) file(s)"
}
Write-Host (
  "CHECKPOINT_VALIDATION_OK: $v1Count V1, " +
  "$legacyCount named pre-V1, $($unexpectedJson.Count) unexpected JSON"
)
```

The cycle-35 post-correction run reported 18 V1 and 9 accepted pre-V1 files. After the validated
cycle-36 CLI checkpoint operations, the exact current result is
`CHECKPOINT_VALIDATION_OK: 20 V1, 9 named pre-V1, 0 unexpected JSON`. This honestly includes the
CLI-resolved pre-validation checkpoint and the final active closure checkpoint. The lower bound
remains 18 so future validated checkpoint creation does not make a healthy corpus fail solely
because it grew; the output always reports the exact observed count.

### Validation evidence

| Gate | Result |
|---|---|
| Current-source checkpoint corpus | 18 V1 valid; 9 explicitly accepted pre-V1; exit 0 |
| Markdown P-008 | 0 issues |
| Docline frontmatter (`go run ./cmd/backlogit --cwd . --no-update-check docs lint`) | `valid: true`, 0 violations |
| Index sync (`go run ./cmd/backlogit --cwd . --no-update-check sync`) | 0 parse failures |
| Topology and live source drift | `WAVE_SIM_OK` 115/115; S=43; M=42; 104 edges; 18 waves; acyclic |
| Production Go touched | **none** |

### Gate and next action

Both cycle-35 findings were remediated in-pass. Cycle 36 normalizes this review to **ADVISORY**,
`pending: none`, and `push_allowed: yes` under the operator's explicit bounded authorization.
That authorization permits a later push and PR-thread reconciliation; it is not merge approval,
not a shipment claim, and not authorization for Ship to begin implementation. No subagent, Go
source edit, push, or merge occurred in cycle 35.

## Plan Review

<!-- BEGIN:plan-review -->
```yaml
cycle: 36
reviewed_at: 2026-08-26
reviewed_head: fbbcc0d01d5da4e769f74225330bd3a01851d3db
dispatch_mode: single-agent-operator-constrained
subagents: prohibited-by-operator
decision: ADVISORY
pending: none
operator_authorization: approved
severity_counts: "P0=0, P1=0, P2=4 (all remediated in-pass), P3=1 (pre-existing advisory recorded and prevented)"
topology: "S=43 explicit shipment members; M=42 exact task IDs; excluded 147-F (feature); forbidden historical sibling 147.010-T absent; 104 executable edges; 18 waves; acyclic"
checkpoint_validation: "20 V1 valid; exactly 9 named pre-V1; 0 unexpected JSON files"
push_allowed: yes
push_performed: no
restage_recommendation: none
```
<!-- END:plan-review -->

decision: ADVISORY

**This record is the current gate state.** It supersedes cycle 35 for gate-decision purposes.
Cycle 35 is normalized to `ADVISORY`; its two planning-data corrections remain historical
foundations. The operator explicitly authorized final bounded cycle-36 closure, so these
documentation and provenance corrections do not open another self-review loop.

### Authorization boundary

`operator_authorization: approved` authorizes this bounded Stage review, its advisory corrections,
and a later push of the reviewed branch. It is **not merge approval**, not a shipment claim, and
not authorization for Ship to begin implementation. This cycle dispatched no subagent, ran no Go
command, changed no Go or global agent/policy file, and performed no push or merge.

### Findings and dispositions

| ID | Sev | Finding | Disposition |
|---|---|---|---|
| K1 | P2 | Cycle 35 classified corrected planning-data defects as one P1 and left its review `FAIL` despite the bounded operator authorization | Normalized cycle 35 to `ADVISORY`, `operator_authorization: approved`, P0=0/P1=0/P2=2/P3=0, and `push_allowed: yes`; the authorization is expressly not merge approval |
| K2 | P2 | Feature `147-F` retained cycle 34's full “Current gate” block beneath cycle 35, leaving two current blocks | Removed the duplicate block and made cycle 36 the sole current block; cycle 34 remains concise historical context |
| K3 | P2 | The checkpoint corpus command did not prove all nine named legacy files existed, impose a V1 floor, or notice JSON files outside `checkpoint-*.json` | Added exact unique nine-name declaration and presence checks, an exact post-loop legacy count, a minimum of 18 V1 files, and an all-JSON namespace check; every parsed V1 still runs current-source `checkpoint get` and therefore `ValidateCheckpoint` |
| K4 | P2 | The plan did not state when a directly edited checkpoint became trustworthy Stage handoff state | Added the normative Stage handoff provenance rule: validated backlogit operations own writes and updates; a direct edit is not persisted evidence until immediate full-corpus validation passes |
| K5 | P3 | At baseline, `147-F.updated_at` was `2026-08-26T08:02:14.3640000Z`, earlier than the CLI-created cycle-35 checkpoint at `2026-08-26T14:50:50.2684865Z` even though the feature text described that checkpoint | Recorded as a pre-existing timestamp-provenance concern. The historical checkpoint timestamp is unchanged; this feature update uses a captured UTC value, and future checkpoint timestamps remain CLI-owned |

K5 is advisory. The ordering proves the feature timestamp was not a reliable provenance marker,
but it does not prove the clock-conversion mechanism that produced it and does not invalidate the
CLI-created checkpoint. The prevention is procedural rather than global: use validated checkpoint
operations, do not hand-author checkpoint timestamps, and apply the immediate corpus gate after
any exceptional direct edit.

### Validation evidence

| Gate | Result |
|---|---|
| Checkpoint corpus through validated workspace-bound CLI | 20 V1 valid; exactly 9 named pre-V1; 0 unexpected JSON files |
| Cycle-36 checkpoint create/get | final `checkpoint-20260826-152441.json` created with `agent: stage`; `valid: true` |
| Markdown P-008 | 0 issues |
| Docline frontmatter | `valid: true`, 0 violations |
| Index sync | 0 parse failures |
| Topology and live source drift | `WAVE_SIM_OK` 115/115; S=43; M=42; 104 edges; 18 waves; acyclic |
| Go commands / Go source changes | none / none |

### Closure and next action

The Stage plan gate is **ADVISORY authorized / ready for later push**. No push occurred in this
cycle. After a later push, PR #377 checks and review threads must be reconciled against that pushed
HEAD before the separate §1.9 readiness gate can clear. Operator merge approval remains
unrequested and ungranted.

### PR #377 Copilot review remediation, cycle 37 — caller migration split by verb, convergence gate widened to earlier-wave open reds

Three Copilot review threads were raised against pushed HEAD `88ced429`. All three were reviewed
against the artifacts they cite; two required artifact changes and one required a pull-request
metadata refresh.

| Thread | Path | Finding | Classification | Disposition |
|---|---|---|---|---|
| `PRRT_kwDORzozKM6cjdUQ` | `.github/agents/.ship.agent.md` | The Step 4.6 convergence gate re-confirms RED only for red deliverables that completed *in the current wave*. Entries carried into `open_red_deliverables_k` from an earlier wave are never executed at any later gate, so an earlier deliverable can go green before its declared green-maker and still pass convergence — while the unfiltered suite that would catch it is deferred for exactly as long as the set is non-empty | Valid | Fixed. P-002.6's always-run list and `.ship.agent.md` Step 4.6 item 2 gain a fourth element: after the single per-wave recomputation of `open_red_deliverables_k`, the gate runs the `red_selector_command` of every entry still open — including entries carried in from earlier waves — and requires each to be observed RED. An entry the wave closed is deliberately not run there; its selector is expected green and is covered by the unfiltered suite the moment the set empties. `workflow-policies.md` moves to **1.23.0** |
| `PRRT_kwDORzozKM6cjdUy` | `.backlogit/queue/147.037-T.md` | U14 spanned three files (`internal/events/checkpoint_lifecycle.go`, `internal/core/checkpoint_disposition.go`, and a new test file), which is not *fewer than three* and therefore outside the 2-Hour Rule heuristic this plan has already enforced against `147.021-T` (cycle 3) and `147.014-T` (cycle 20) | Valid | Fixed by splitting the caller migration by verb. **U14** (`147.037-T`) keeps the resolve site in `internal/events`; new **U14b** (`147.044-T`) takes the abandon site in `internal/core`. Each unit is two files and two scenarios. The alternative — reducing U14 to two files by borrowing another unit's harness, as U13 borrows U12's — was rejected: it would leave the abandon migration with no red evidence of its own |
| `PRRT_kwDORzozKM6cjdVm` | `docs/memory/2026-08-26/stage-pr377-cycle-35-planning-data-correction-memory.md` | The pull-request description is stale relative to the branch: it names cycle 29 current, reports 108 changed files, and claims six unresolved threads | Valid | Fixed outside the repository: the PR description is refreshed after this cycle's review wave resolves, so cycle, changed-file count, thread state, and topology match the pushed HEAD. No repository artifact carries the PR description, so nothing in-tree changes for this thread |

**Second review wave (against `84d53b79`).** Five further threads were raised after the first wave
was fixed, replied to, and resolved.

| Thread | Path | Finding | Classification | Disposition |
|---|---|---|---|---|
| `PRRT_kwDORzozKM6cmKT6` | `.github/policies/workflow-policies.md` | The tracked scheduler simulation does not exercise the new always-run item: `Invoke-WaveScheduler` computes `open_red_after_wave` but never runs an open selector or models one becoming green, so the reported result would still pass with the item removed | Valid | Fixed. The early-green outcome gains a named fail-closed code, **`WAVE_RED_DELIVERABLE_EARLY_GREEN`** (P-002.2), used by both the in-wave case and the carried-in case. The runner models the re-confirmation step and records `open_red_reconfirmed_at_wave`; the fixture gains an `open_red_early_green` mutation and two scenarios — `open_red_early_green_carried_in` (a carried-in entry injected green three waves before its green-maker halts the loop at the wave-5 gate) and `open_red_closed_entry_not_reconfirmed` (an entry the wave closed is deliberately not re-run, so the same injection is not a violation). `persistent_red_mapping` additionally asserts the exact re-confirmed set at waves 3, 4, 7, 8, 12, and 13. The first scenario is the load-bearing control: with the always-run item removed it expects `WAVE_RED_DELIVERABLE_EARLY_GREEN` and observes `COMPLETE` |
| `PRRT_kwDORzozKM6cmKUm` | `.backlogit/queue/147.037-T.md` | The `TestU14_` RED can pass without performing the required migration — removing `syncWriteFileAtomic` and routing through any other helper satisfies a removal-only assertion | Valid | Fixed. The structural assertion now requires **both** halves: no direct atomic-write call under the checkpoint directory **and** a `RewriteCheckpointFile` call inside `ResolveCheckpoint` |
| `PRRT_kwDORzozKM6cmKU9` | `.backlogit/queue/147.044-T.md` | Same defect in the new `TestU14b_` harness | Valid | Fixed identically, requiring an `events.RewriteCheckpointFile` call inside `AbandonCheckpoint`. Both plan unit sections carry the same two-part assertion, so the task text and the plan cannot drift |
| `PRRT_kwDORzozKM6cmKVX` | `tests/simulation/README.md` | The `baseline` scenario row still reported the pre-split 42-task / 104-edge topology, contradicting the updated fixture and the same document's 43-task description | Valid | Fixed to 43 tasks / 106 edges; the two new scenario rows were added in the same table |
| `PRRT_kwDORzozKM6cmKV6` | this plan | The disposition claims the PR metadata was refreshed, but the live description still shows cycle 29, 42/104/43, and six unresolved threads | Invalid — stale premise | Declined with evidence. The description was refreshed before the second review wave completed; the review's PR-metadata snapshot predates the edit. The live description records cycle 37, 129 changed files, 43 tasks / 106 edges / 44 shipment members, and no open findings. Re-verified after the fact; no further change was needed for the claim to be true |

**Third review wave (against `44f4f078`).** Four threads, resolving to two defect classes.

| Thread(s) | Path | Finding | Classification | Disposition |
|---|---|---|---|---|
| `PRRT_kwDORzozKM6cmdQ0`, `PRRT_kwDORzozKM6cmdRN`, `PRRT_kwDORzozKM6cmdRb` | `workflow-policies.md`, `.ship.agent.md`, `wave-scheduler-sim.ps1` | The mirror of the wave-2 defect: an entry that *closes* at a gate leaves the open set without its selector ever being required to pass. While another entry keeps the set non-empty the unfiltered suite stays deferred, so a green-maker that landed without turning its selector green is not detected at the wave that was supposed to prove it | Valid | Fixed. The always-run list gains **item 5**: re-run the `red_selector_command` of every entry in `newly_closed_k` and require **GREEN**, halting with the new `WAVE_GREEN_MAKER_UNVERIFIED` code. Items 4 and 5 now **partition** the pre-recomputation open set exactly — still open → RED, newly closed → GREEN — so no entry is skipped. The runner tracks `newly_closed_k`, the fixture gains a `green_maker_leaves_red` mutation and the `green_maker_lands_but_selector_stays_red` control (halt at wave 7, six waves before the deferred suite would have surfaced it), and `open_red_closed_entry_not_reconfirmed` was rewritten from "not re-run" to "re-confirmed GREEN" |
| `PRRT_kwDORzozKM6cmdQK` | `.ship.agent.md` | Step 3's required manifest census still pinned **43 total / 42 in `M`**, and its green-regression paragraph still said all **42** arrays are empty — both pre-split figures that would make Ship reject the valid current manifest or freeze the wrong wave budget | Valid | Fixed to 44 total / 43 in `M` and 43 empty arrays. The reviewer named both occurrences; a repository-wide sweep for the same census shape found no others |

**Fourth review wave (against `c45a586f`) — cycle limit reached.** Four threads. Two are direct
consequences of this cycle's own edits and were closed as completion of that work rather than as a
fourth fix cycle; two are **pre-existing defects this PR did not introduce** and are recorded here
as open findings for the next Stage cycle, per the `github-pr-automation` §1.8 three-cycle limit.
The limit stops further automated fixing; it does **not** clear the merge gate, and the two open
threads keep §1.9 Check 3 failing by design.

| Thread | Path | Finding | Classification | Disposition |
|---|---|---|---|---|
| `PRRT_kwDORzozKM6cmx7G` | `workflow-policies.md` | The `S` / `M` census became internally impossible: `count(S) = 43` with `count(M) = 43` and one excluded feature cannot both hold, so an implementation following the policy would reject the valid frozen manifest | Valid — self-introduced | **Fixed.** `count(S) = 44`. The `count(M)` figure was updated when U14b landed but its `count(S)` counterpart three lines above was missed |
| `PRRT_kwDORzozKM6cmx7W` | `.autoharness/drift-ignore` | The template-adoption obligation still pinned the obsolete fixture size (84 assertions / 16 scenarios) and the pre-split `count(S)=43` / `count(M)=42` census, so a future template adoption could omit the new convergence controls | Valid — self-introduced | **Fixed** at all three cited lines: 150 assertions / 21 scenarios, `count(S)=44` / `count(M)=43`, and the obligation now names both convergence controls explicitly so an adopting template cannot silently drop them |
| `PRRT_kwDORzozKM6cmx8M` | `.github/skills/build-feature/SKILL.md` | The declared `red_deliverable` input has no implementing branch: the generic loop treats a passing harness as success and a failing harness as code to repair, so a red-deliverable task dispatched through the skill is driven toward green or exhausts the retry loop | **Valid — pre-existing, deferred** | Not fixed in this cycle. The defect predates cycle 37 (`red_deliverable` was declared in 1.20.0 and the loop was never branched), it is a behavioural contract change to the build loop rather than a text correction, and the three-cycle limit is reached. Recorded as an open finding; the thread is left **unresolved** so §1.9 Check 3 keeps failing |
| `PRRT_kwDORzozKM6cmx7t` | `.backlogit/queue/147.026-T.md` | U10 relocated the verification workspace to the already-ignored `.copilot/scratch/checkpoint-verification/`, but U10b recreates its mirror under unignored `docs/scratch`, contradicting the shared-workspace handoff and able to dirty the tracked tree | **Valid — pre-existing, deferred** | Not fixed in this cycle. The path predates cycle 37 (it survives the cycle-29 U10 relocation) and re-siting it touches U10b's and U10c's declared evidence paths and their `verification-only` exempt surfaces, which needs its own plan pass. Recorded as an open finding; the thread is left **unresolved** |

Both deferred findings are genuine and neither is dismissed. They are carried as open findings
because the honest options at the cycle limit are "fix and re-review" or "record and halt", and
both of these change execution contracts that deserve a fresh review rather than an edit made at
the boundary.

**Why the split, and why it needs no new mechanism.** Cycle 17 already used exactly this move when
it broke the original five-file seam unit into U11 (declaration), U12 (contract harness), U13
(implementation), and U14 (caller migration); cycle 20 used it again when it split U7b into U7b and
U7c on width-isolation grounds. The two verb migrations touch disjoint packages, use different
write helpers (`syncWriteFileAtomic` in `internal/events`, `atomicfile.WriteFileAtomic` in
`internal/core`), and share no symbol, so the split is clean and each half stays independently
verifiable.

**Edge retargeting.** `147.008-T` / U4 edits `internal/core/checkpoint_disposition.go`, so its
prerequisite moves from `147.037-T` to `147.044-T` — the migration that actually touches its file.
`147.021-T` / U2f enumerates the post-migration allow-list across **both** packages, so it now
depends on `147.037-T` **and** `147.044-T`. `147.006-T` / U3, `147.007-T` / U3b, and `147.042-T` /
U3c are resolve-verb units and keep their `147.037-T` edges unchanged; U3c's green-maker is still
`147.037-T`, closing at wave 8.

**Topology delta.** Two edges added net: `147.044-T -> 147.036-T` and `147.021-T -> 147.044-T`;
`147.008-T -> 147.037-T` was replaced by `147.008-T -> 147.044-T`. Counts move from
42 tasks / 104 edges / 43 shipment members to **43 tasks / 106 edges / 44 shipment members**. The
ready set is unchanged at `{147.001-T, 147.032-T}` — U14b is an interior node. The schedule stays
at **18 waves**: U14b depends only on U13 (wave 7), so it lands in wave 8 beside U14, and wave 8
grows from two members to three. No dependent's wave moves, because U4 and U2f already sat strictly
after wave 8.

### Validation evidence

| Gate | Result |
|---|---|
| Wave-scheduler simulation, live-queue verification | `WAVE_SIM_OK` 150/150 across 21 scenarios; S=44; M=43; 106 edges; 18 waves; acyclic; 0 stalls; 0 compile-order violations |
| Markdown P-008 | 0 issues |
| Docline frontmatter | `valid: true`, 0 violations |
| Index sync | 0 parse failures |
| Go commands / Go source changes | none / none |

## Plan Review

<!-- BEGIN:plan-review -->
```yaml
cycle: 37
reviewed_at: 2026-08-26
reviewed_head: 88ced429218f31ef424e24f149471522b771a6c6
dispatch_mode: single-agent-operator-constrained
subagents: prohibited-by-operator
decision: ADVISORY
pending: two deferred pre-existing findings (build-feature red_deliverable branch; 147.026-T scratch mirror path)
operator_authorization: approved
severity_counts: "P0=0, P1=3 (convergence gate open-red omission, its newly-closed mirror, and the missing simulation coverage — all remediated in-pass), P2=4 (U14 three-file width, removal-only structural harnesses, stale simulation README baseline row, stale Ship Step 3 manifest census — all remediated in-pass), P3=0"
topology: "S=44 explicit shipment members; M=43 exact task IDs; excluded 147-F (feature); forbidden historical sibling 147.010-T absent; 106 executable edges; 18 waves; acyclic"
push_allowed: yes
push_performed: yes
restage_recommendation: none
```
<!-- END:plan-review -->

decision: ADVISORY

**This record is the current gate state.** It supersedes cycle 36 for gate-decision purposes.
Cycle 36 remains the historical foundation for the checkpoint-provenance corrections it made.

### Authorization boundary

`operator_authorization: approved` authorizes this bounded Stage review, its remediations, a push
of the reviewed branch, and replies to and resolution of bot-authored review threads. It is **not**
merge approval, **not** a shipment claim, and **not** authorization for Ship to begin
implementation. This cycle dispatched no subagent, ran no Go build or test command against
production source, and changed no Go source file.

### Closure and next action

The Stage plan gate is **ADVISORY authorized / pushed with two open findings**. PR #377 checks and
review threads are reconciled against the pushed HEAD; the §1.9 readiness gate **fails Check 3**
with two deliberately unresolved threads. Operator merge approval remains unrequested and
ungranted, and the next Stage cycle should take the two deferred findings first:

1. `build-feature/SKILL.md` — add an explicit red-deliverable branch ahead of the generic loop:
   land the harness, require compilation plus assertion RED, then enter the inverted quality gates
   with no fix iteration. Today the declared `red_deliverable` input has no implementing branch,
   so `147.016-T`, `147.035-T`, and `147.042-T` would be driven toward green.
2. `147.026-T` / U10b — re-site the verification mirror beneath U10's canonical
   `.copilot/scratch/checkpoint-verification/` root, and re-check U10b's and U10c's declared
   evidence paths and `verification-only` exempt surfaces against the move.


## Plan Review

<!-- BEGIN:plan-review -->
```yaml
cycle: 38
reviewed_at: 2026-08-26
reviewed_head: ae28bec8e43409074be93417e753448f70ca920c
dispatch_mode: single-agent-operator-constrained
subagents: prohibited-by-operator
decision: ADVISORY
pending: none
operator_authorization: approved-additional-review-fix-cycles
severity_counts: "P0=0, P1=2 (the two findings cycle 37 deferred at the §1.8 cycle limit — both remediated in-pass), P2=0, P3=0"
topology: "S=44 explicit shipment members; M=43 exact task IDs; excluded 147-F (feature); forbidden historical sibling 147.010-T absent; 106 executable edges; 18 waves; acyclic"
push_allowed: yes
push_performed: yes
restage_recommendation: none
```
<!-- END:plan-review -->

decision: ADVISORY

**This record is the current gate state.** It supersedes cycle 37 for gate-decision purposes and
closes both findings cycle 37 recorded as deliberately unresolved. Cycle 37 remains the historical
record of the convergence-gate corrections it made; its fourth-wave disposition rows are left
verbatim, as every superseded row in this plan is.

### Authorization boundary

The operator authorized additional review-fix cycles beyond the `github-pr-automation` §1.8
three-cycle limit, so the deferral rationale recorded in cycle 37 no longer applies. This
authorization covers the two remediations below, a push of the reviewed branch, replies to and
resolution of the two bot-authored threads, and a fresh Copilot review. It is **not** merge
approval, **not** a shipment claim, and **not** authorization for Ship to begin implementation.
This cycle dispatched no subagent, ran no Go build or test command against production source, and
changed no Go source file.

### Finding 1 — `build-feature` had no red-deliverable execution branch

| Thread | Path | Classification | Disposition |
|---|---|---|---|
| `PRRT_kwDORzozKM6cmx8M` | `.github/skills/build-feature/SKILL.md` | Valid — pre-existing, now fixed | **Fixed.** New Step 0.5 branch; `workflow-policies.md` moves to **1.24.0** |

`red_deliverable` was declared as a `build-feature` input in policy 1.20.0, and 1.23.0 built the
whole cross-wave open-red accounting on top of it, but the loop underneath was never branched: a
passing `harness_cmd` read as success and a failing one as code to repair, and Step 2 advanced only
on green. A red-deliverable task dispatched through the skill was therefore driven toward green or
exhausted its 5-attempt budget, destroying exactly the red that `147.016-T` / U8b, `147.035-T` /
U12, and `147.042-T` / U3c exist to hold and starving Step 4.6 items 4 and 5 of the entries they
re-confirm.

`build-feature` gains **Step 0.5**, which replaces the generic loop for these tasks rather than
modifying it. In its final form — corrected in review wave 4, below — it **consumes and validates
the harness `harness-architect` already scaffolded with the wave, and writes nothing**. It is fail
closed at four boundaries:

* **Dispatch preconditions** — a `harness-exempt` pairing, a `harness_cmd` that is not the declared
  `red_selector_command` verbatim, or an absent `red_baseline_sha` halts
  `WAVE_RED_MAPPING_UNRESOLVED`.
* **Compilation (0.5a item 1)** — the repo-wide compile check must pass. A failure contradicts the
  P-002 compiling-but-failing harness contract and halts `RED_DELIVERABLE_HARNESS_UNCOMPILABLE`,
  returning the task to `harness-architect` rather than repairing the harness here.
* **Assertion RED (0.5a item 2)** — the run must fail on named `--- FAIL:` functions. Exit 0 in any
  form halts `WAVE_RED_DELIVERABLE_EARLY_GREEN`; a no-tests-to-run signal at any exit code halts new
  `WAVE_RED_DELIVERABLE_VACUOUS`, the exact mirror of the P-002.3 false-green rule; a panic,
  timeout, or package abort halts `RED_DELIVERABLE_NOT_ASSERTION_RED`.
* **Zero-delta gate (0.5b)** — the changed-file set against `red_baseline_sha` must be empty. A
  non-test `*.go` file halts `RED_DELIVERABLE_PRODUCTION_DELTA_REFUSED`; anything else halts
  `RED_DELIVERABLE_DELTA_OUT_OF_SURFACE`.

Step 0.5c then returns the evidence Ship Step 4.5 turns into the `open_red_deliverables` entry —
executed selector, failing function names and exit code, passing compile check and empty delta,
declared green-makers and closing wave — and 0.5d runs the inverted gates with no fix iteration,
tolerating exactly `sibling_red_selectors ∪ open_red_selectors ∪ {the task's own declared selector}`.
There is no commit, because the delta is empty by contract.

Coupled surfaces moved together: P-002.2 gains `WAVE_RED_DELIVERABLE_VACUOUS` and extends
`WAVE_RED_DELIVERABLE_EARLY_GREEN` to the new gate points; P-002.6 gains the execution rule and its
executable-coverage requirement; `.ship.agent.md` Step 4.2 passes `red_deliverable` and
`red_baseline_sha` explicitly and names the branch and its halt codes, and Step 4.5 builds its
record from the Step 0.5c report rather than re-deriving it.

**Executable coverage and a design correction (review waves 2-4, threads `PRRT_kwDORzozKM6cnqV4`,
`PRRT_kwDORzozKM6cn_pZ`, `PRRT_kwDORzozKM6coLsh`, `PRRT_kwDORzozKM6coLtY`).** Three follow-up waves
against this cycle's own edits corrected two coverage gaps and one design error, in that order.

*Wave 2 — no executable coverage.* The first form of the fix left Step 0.5 as prose with no control,
the same defect class cycle 37 recorded as a P1 follow-up when its convergence items had no
simulation coverage. The tracked simulation now carries a `red_deliverable_branch_controls` block
and a classifier applying this branch's ordering. Step 0.5's assertion check was tightened at the
same time: a panic, timeout, or package abort is an explicit `RED_DELIVERABLE_NOT_ASSERTION_RED`
halt, because a non-zero exit is not evidence that an assertion failed.

*Wave 3 — the delta rule was a heuristic.* The classifier rejected only non-test `*.go` files, so an
extra `*_test.go`, a configuration file, or a documentation file passed a rule whose stated scope
was "the harness file(s) the task names and nothing else".

*Wave 4 — the branch was unsatisfiable, and the delta set had no runtime source.* The reviewer
found the load-bearing error: Ship Step 4.0 item 10 and `harness-architect` item 9 scaffold **every**
non-exempt wave member, red deliverables included, and confirm RED before `build-feature` is
invoked. A branch that *lands* the harness therefore sees a matching `--- FAIL:` at its own baseline
and halts as "pre-landed" for exactly the tasks it exists to complete. The companion finding was
that the wave-3 declared-file set had no runtime source at all — the canonical
`red-deliverable-contract` has five keys, none naming harness files, and deriving the set from the
working tree would sweep in every sibling harness scaffolded in the same wave.

Both are resolved by inverting the branch: **Step 0.5 consumes and validates the already-scaffolded
harness and writes nothing.** Its expected dispatch state is *already RED*, which is what
`harness-architect` recorded. It runs the repo-wide compile check (failure →
`RED_DELIVERABLE_HARNESS_UNCOMPILABLE`, returned to `harness-architect`), requires named assertion
RED (`WAVE_RED_DELIVERABLE_EARLY_GREEN` on a pass, `WAVE_RED_DELIVERABLE_VACUOUS` when no test
matches the selector, `RED_DELIVERABLE_NOT_ASSERTION_RED` on a panic or timeout), and then applies a
**zero-delta gate** against a new `red_baseline_sha` that Ship captures at Step 4.1a item 5 —
*after* the wave's scaffolding commit, so a sibling's harness can never enter this task's delta. A
non-test `*.go` file there is `RED_DELIVERABLE_PRODUCTION_DELTA_REFUSED`; anything else is
`RED_DELIVERABLE_DELTA_OUT_OF_SURFACE`. The branch produces no commit, so Ship Step 4.5 completes
the task without one. `WAVE_RED_DELIVERABLE_PRELANDED` is withdrawn: the state it described is now
the expected one.

The control suite was rebuilt against the corrected model — 16 controls covering the confirmed
deliverable, an early-green selector, a selector matching no test, an uncompilable scaffolded
harness, panic and timeout rejection, both zero-delta refusals, all three dispatch-precondition
refusals including an absent `red_baseline_sha`, an incomplete evidence report, and both routing
directions. The load-bearing control is `red-deliverable-never-enters-generic-loop`, which feeds the
exact observation the generic loop reads as SUCCESS; with the branch removed it fails on both
assertions. Totals are 182 assertions with `-VerifyAgainstQueue` and 160 without.

### Finding 2 — U10b recreated its mirror under an unignored path

| Thread | Path | Classification | Disposition |
|---|---|---|---|
| `PRRT_kwDORzozKM6cmx7t` | `.backlogit/queue/147.026-T.md` | Valid — pre-existing, now fixed | **Fixed.** Mirror re-sited under U10's canonical ignored root |

Cycle 29 relocated U10's verification workspace to the already-ignored
`.copilot/scratch/checkpoint-verification/` precisely so `verification-only` would not have to
commit a `.gitignore` rule. The plan's U10b unit section moved with it, but the task artifact did
not: `147.026-T` still sited its mirror at `docs/scratch/checkpoint-verification/mirror/`, a path
`git check-ignore` does not match, which would dirty the tracked tree during verification and force
this unit to commit an ignore rule its own P-002.4 surface rejects.

The mirror is now `.copilot/scratch/checkpoint-verification/mirror/`, with the copied checkpoints at
`.copilot/scratch/checkpoint-verification/mirror/.backlogit/checkpoints/` and every sweep invocation
bound by `--cwd .copilot/scratch/checkpoint-verification/mirror`. Ignore state was proved rather
than assumed before the change was accepted:

```text
git check-ignore -v .copilot/scratch/checkpoint-verification/mirror/x.json
  -> .gitignore:5:.copilot/    (exit 0, ignored)
git check-ignore -v docs/scratch/checkpoint-verification/mirror/x.json
  -> (exit 1, NOT ignored — rejected)
git ls-files .copilot
  -> 0 entries
```

The task now carries that check as an acceptance criterion, and its class delta surface states the
no-configuration-file rule explicitly, matching U10. The three coupled surfaces were re-checked
against the move: the plan's U10b section already named the canonical root and needed no change;
`147.041-T` / U10c owns teardown of `.copilot/scratch/checkpoint-verification/`, which contains the
mirror as a child, so its teardown scope is unchanged; and both units' `exempt_verification_command`
values and evidence-manifest rows address the tracked closure artifact, never the scratch path, so
neither gate moved. The mirror root is a child of U10's root and inherits its containment proof
rather than establishing a second one.

### Closure and next action

The Stage plan gate is **ADVISORY authorized / pushed with no open findings**. Both threads cycle 37
left unresolved are fixed, replied to with the fixing SHA, and resolved, so §1.9 Check 3 no longer
fails on them. Operator merge approval remains unrequested and ungranted; Ship has not been
authorized to claim `130-S` or begin implementation.

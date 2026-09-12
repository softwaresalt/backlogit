---
chunk_strategy: h2
description: "Stage PR #377 cycle-16 remediation — cycle-15 plan-review FAIL gate recorded, repair runbook withdrawn, bounded diagnostic projection isolated, U7e narrowed, topology regenerated"
doc_type: memory
schema_version: "1.0"
source: cycle-16-session
title: "PR #377 Cycle 16 Remediation Memory"
---

# PR #377 Cycle 16 Remediation Memory

**Date**: 2026-08-25
**Agent**: Stage
**Branch**: `chore/stage-130-s`
**PR**: #377
**Cycle**: 16 (operator-authorized extension)
**Worktree**: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
**Shipment**: `130-S` (queued — **not** claimed, **not** shipped)

## Outcome

Recorded the formal cycle-15 plan-review gate as `decision: FAIL` and remediated every P0 and P1
finding in one bounded planning pass. No Go source, test, or configuration file was modified.
Stage's Role Boundary was observed throughout: no push, no PR action, no shipment claim, no build,
no lint of Go code.

The cycle-14 P0 is **closed** and was not re-raised: U2g carries one caller-invoked
conformance-helper contract, `ErrCheckpointContextDuplicateKey` stays withdrawn, and the open
`context` namespace is preserved. The cycle-15 gate fails on **new** findings, not residue.

## Gate record

Appended a third `## Plan Review` section carrying the literal fields `cycle: 15`,
`dispatch_mode: multi-agent-dispatch`, `decision: FAIL`. All seven selected personas returned:
Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture
Strategist, Agent-Native Parity Reviewer, Security Lens Reviewer. The cycles 1-13 `PASS` and the
`cycle: 14` `FAIL` are both annotated as historical. `147-F` now states plan-review state
**PENDING / FAIL** and blocks implementation until a fresh cycle-16 review returns.

Thirteen merged findings: two P0, nine P1, one P2, one P3.

## Decisions and rationale

### D1 — this shipment is quarantine-only (the central P0)

The paste-runnable hand-repair and post-quarantine restore runbook published in U9b and
`147.018-T` is **withdrawn**, not weakened. It performs rename, copy, in-place rewrite, and
conditional delete against a directory the codebase does not open with real-root / no-follow
semantics, with no handle-or-content CAS between the classifying read and the repairing write, and
no adversarial coverage. Publishing it in a file with `applyTo: '**'` claimed a safety property
this release cannot provide. It moves to stash `35A27CD0` alongside the containment work that makes
it safe. U10b's restore row is replaced by a quarantine evidence-integrity row (archived bytes
SHA-identical to preimage, sidecar names the original filename, a second quarantine of the same
filename is refused).

Refusal plus verbatim-move remains a complete, reversible disposition, so nothing this plan needs
is lost. Operators may still make an explicit raw-token-aware survivor decision by hand, outside
agent automation.

### D2 — the universal no-implicit-survivor claim is narrowed, not defended

Cycle 15 asserted the rule "universally … whether at the top level, inside `context`, inside
`progress`, or inside `legacy_top_level` itself". That is false at the create boundary, where
`CreateCheckpoint` still parses and re-marshals caller bytes and can collapse duplicate `context`
members before disk. The invariant is now scoped to the stored-checkpoint administrative-disposition
read and rewrite surfaces (`resolve`, `abandon`, `quarantine`, `list`, `get`), with create-boundary
hardening explicitly deferred under `E429A031`. A bounded create-boundary unit was considered and
rejected: the create boundary is 146-F's shipped surface, no pre-existing on-disk evidence is at
risk there, so it is not required for safe disposition and adding it is scope creep against the
origin decision.

### D3 — no spelling-based implicit survivor anywhere

U9b's repair table told the operator to move the unmodeled variant and leave the modeled member in
place for `duplicate:<key>` with one modeled member. That is a spelling-based implicit survivor
selection on a modeled field. Removed. Every duplicate equivalence class touching a modeled field
now requires an explicit, recorded, raw-token-aware operator selection or quarantine.

### D4 — U7e narrowed from three rows to one

Source audit: only `handleGetCheckpoint` (`internal/mcp/tools.go:1205`) and
`handleResolveCheckpoint` (`:1224`) route through `domainError`; abandon (`:1279`) and quarantine
(`:1310`) already use `checkpointDispositionError`. Get never emits `ErrCheckpointUseQuarantine` or
`ErrCheckpointNonConforming`, and U7d reroutes every `QuarantineIsRemedy` match, so rows 1 and 2 are
unreachable and were removed. Only `ErrCheckpointCannotResolveAbandoned` → `validation_failed`
remains — genuinely reachable, genuinely `default: InternalError` today.

The cycle-15 mandatory-ordering constraint is withdrawn with row 1, which is the only row it
guarded. Architecture's P1 asking for a named ordering invariant is therefore **closed by
removal** rather than by adding an invariant over a row the plan no longer contains.

### D5 — U7e's expected red was wrong and is corrected

Cycle 15 claimed all three rows currently fall to `default: InternalError`. False for row 1: the
multi-`%w` wrap `fmt.Errorf("%w: %w", ErrCheckpointUseQuarantine, valErr)` also carries
`ErrCheckpointInvalid`, which the combined case at `internal/mcp/errors.go:188-193` maps to
`validation_failed`. It shadows, it does not fall through. Correcting this removed the last reason
to keep row 1.

### D6 — one bounded diagnostic projection, isolated as U1b

Cycle 15 bounded `Error()` only. U7 populates `unknown_fields` via `errors.As` from the raw
`Fields` slice, so the MCP payload could be unbounded while the CLI was bounded. New task
`147.030-T` (U1b) owns `BoundedFieldPaths()` — quoted, 16 paths, 128 bytes per path, truncation
marker with omitted count. U6b, U7, and U8 all render through it and are forbidden from
re-deriving a list. Isolating it as its own task also removes the fourth effective flow that
bounding had added to U1.

### D7 — the offender source is now executable

U9b told operators to read offenders from `checkpoint get`, but `CheckpointReadResult` carried no
offender field. Chose the smaller truthful option (b): `NonConformingFields` added to the carrier
U6b already declares, projected as `non_conforming_fields` by U6c and U8c. Width-neutral — no new
file, no new scenario; the assertion rides on each unit's existing projection case.

### D8 — one destructive-approval contract

A4 said runtime disposition mutation needs no approval while A4c said the same commands are
destructive. A4 is narrowed to workspace creation and read verbs; A4c is the sole operative
contract for any checkpoint-file-moving or -overwriting command. Principle VII moved from
"deviation (documented)" to conditional **pass** — a NON-NEGOTIABLE principle cannot be satisfied
by documenting a departure from it — and both VII rows were removed from the documented-deviations
table. The six-point execution contract (approval batch immediately before execution, `--cwd`
binding with bare filenames, filename/hash/state/destination display, preimage byte copy,
absent-destination and no-clobber assertion, fail-closed halting) is propagated into U10, U10b, and
their tasks.

### D9 — executable red verification

The `go test ./<pkg>` placeholder named no package, no selector, and no cache-defeating flag. A
per-unit table now gives the exact `-run` regex and `-count=1` invocation for all 29 tasks, backed
by a mandatory `TestU<unit>_` harness naming contract so `^TestU2_` cannot match `TestU2b_`.

### D10 — width normalization

| Unit | Task | Before | After |
|---|---|---|---|
| U5 | `147.009-T` | 4 effective | 2 — byte-identity is a postcondition; `resolved` guards withdrawn |
| U2g | `147.028-T` | 4 effective | 2 red plus 1 combined green preservation guard |
| U7e | `147.029-T` | 3 (2 unreachable) | 1 |
| U1 | `147.001-T` | 3 plus in-line bounding | 3; bounding split to U1b |

### D11 — provenance and gates

`version --json` is not a flag this CLI has; the JSON selector is `--format json`
(`internal/cli/version_cmd.go`), and `--no-update-check` is required for a hermetic assertion. The
ldflags reference now cites the repository's own `Makefile:5-8` `LDFLAGS`. Gate 4 is `make fmt`
(`Makefile:38-39`), not a bare `gofmt -l .` — which exits 0 while listing unformatted files. Gate 9
(build and provenance) added. Gates declared mandatory for the release-unit branch regardless of any
individual unit being docs-only.

### D12 — rejected review claims

* **"Add a bounded create-boundary duplicate-detection unit."** Rejected as scope creep; narrowing
  the claim is smaller and honest.
* **"Keep the restore runbook but harden it here."** Rejected as scope expansion; that is a
  filesystem-hardening project (`35A27CD0`), not a disposition change.
* **"The linked worktree is a P-016 violation."** Rejected again, unchanged from cycle 15.

## Files modified

| File | Change |
|---|---|
| `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` | Per-unit red-verification table and harness naming contract added; U1 split, U1b added; U2g, U5, U5b, U6b, U6c, U7, U7d, U7e, U8, U8c, U9b, U10, U10b rewritten or corrected; requirements trace R7/R8/R10 updated; risks, constitution check, documented deviations, risky actions, runtime verification, closure, and gate sequence updated; dependency graph, edge table, execution order and counts regenerated; cycle-15 `FAIL` gate record and cycle-16 appendix appended |
| `.backlogit/queue/147-F.md` | Plan-review state set to PENDING / cycle-15 FAIL; task inventory and topology refreshed |
| `.backlogit/queue/147.030-T.md` | **new** — U1b bounded diagnostic projection |
| `.backlogit/queue/147.001-T.md` | Bounding scope moved out to U1b; red command added |
| `.backlogit/queue/147.009-T.md` | 2 scenarios; byte-identity postcondition; `resolved` guards withdrawn |
| `.backlogit/queue/147.012-T.md` | `NonConformingFields` on the read result, rendered through `BoundedFieldPaths()` |
| `.backlogit/queue/147.013-T.md` | `unknown_fields` rendered through the bounded projection |
| `.backlogit/queue/147.015-T.md` | CLI key list bounded; abandoned-resolve CLI coverage declared out of scope |
| `.backlogit/queue/147.018-T.md` | Rewritten quarantine-only; runbook withdrawn; invariant scoped; auto-survivor rule removed |
| `.backlogit/queue/147.019-T.md` | ldflags/version provenance corrected; six-point execution contract added |
| `.backlogit/queue/147.022-T.md` | `non_conforming_fields` projection |
| `.backlogit/queue/147.026-T.md` | Restore row replaced by quarantine evidence integrity |
| `.backlogit/queue/147.027-T.md` | `non_conforming_fields` projection |
| `.backlogit/queue/147.028-T.md` | Preservation assertions combined into one green guard |
| `.backlogit/queue/147.029-T.md` | Narrowed to the one reachable mapping; ordering constraint withdrawn |
| `.backlogit/stash.json` | Two new follow-ups; `35A27CD0` extended |
| `.backlogit/checkpoints/checkpoint-20260824-191617.json` | cycle-16 state |
| `.backlogit/memories.json` | canonical Stage handoff entry refreshed |
| `docs/memory/2026-08-24/stage-pr377-remediation-cycle-16-memory.md` | this file |

## Topology

Measured with `backlogit --cwd . query` after `backlogit --cwd . sync`.

| Measure | Cycle 15 | Cycle 16 |
|---|---|---|
| Queued tasks under `147-F` | 28 | **29** |
| Queued-to-queued executable edges | 48 | **52** |
| Shipment `130-S` members | 29 | **30** |
| Ready set | `{147.001-T}` | `{147.001-T}` (sole root) |
| Historical total edges | 49 | **53** (52 executable + archived `147.010-T -> 147.009-T`) |

One task added (`147.030-T`, U1b) and four edges added: `147.030-T -> 147.001-T`,
`147.012-T -> 147.030-T`, `147.013-T -> 147.030-T`, `147.015-T -> 147.030-T`. Graph verified
acyclic by Kahn topological sort — all 29 nodes ordered, sole root `147.001-T`.

## Validation

* `backlogit --cwd . sync` — 1193 artifacts indexed, 0 parse failures
* `backlogit --cwd . doctor` — 23 pre-existing orphans under `016-R` and `106-*`, **none** under
  `147-F`; no duplicate IDs
* `backlogit --cwd . docs lint` — `valid: true, violation_count: 0`
* `markdownlint-cli2@0.23.1` over the plan and all 30 `147*` artifacts — 0 issues in 31 files
  (`scripts/md-lint.sh` itself fails on this Windows checkout with a CRLF `$'\r'` shell error, a
  pre-existing environment condition unrelated to this change; the underlying linter was invoked
  directly)
* Kahn topological sort over the 52 executable edges — acyclic, 29/29 ordered
* Canonical checkpoint re-read and parsed after the update

## Follow-up stash entries created

| Stash | Item |
|---|---|
| `6FA45E69` | Pin the conforming + `resolved` double-refusal state-conflict class in a unit that owns it |
| `DBBA62AA` | CLI coverage for `checkpoint resolve` on an already-abandoned document |
| `35A27CD0` | Extended to absorb the withdrawn repair/restore runbook alongside no-follow and CAS hardening |

## Open items

No P0 or P1 finding remains open in this bounded scope. G12 (architecture advisory: dual
`GetCheckpoint`/`GetCheckpointResult` API surface, a CI gate for the conformance predicate) and G13
(appendix accumulation) are P2/P3 and were deliberately not expanded.

## Next safe action

The plan is ready for a **fresh, independent cycle-16 plan review**. Remediation does not clear the
cycle-15 `FAIL` gate — only a new dispatch returning `PASS` does. Ship or the operator must query
live PR #377 state — head, checks, reviews, unresolved threads — before any merge-readiness claim.
Stage stopped at its Role Boundary: push, review replies, thread resolution, shipment claim, and
merge remain forbidden to it. The hard merge gate is unchanged: `147.018-T` must land in the same
merge commit as `147.007-T`, `147.008-T`, and `147.009-T`, inside the single `130-S` PR. The halt
condition is unchanged: if `147.009-T`'s paired accept/refuse assertion cannot pass, halt rather
than weaken it.

---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for formal-gate unit F6: route commit association through one shared core function so CLI and MCP cannot diverge, and assert behavioral (not merely surface) parity for governed operations.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-07-f6-governed-op-cli-mcp-parity-plan.md
title: 'F6 — governed-operation CLI/MCP parity hardening'
---

# F6 — governed-operation CLI/MCP parity hardening

Source: `docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md`
(Q7, follow-up F6). No separate micro-decision was required — the spike states a
single unambiguous contract requirement and names the concrete divergence.

<!-- plan-review-attempt: 2 -->

## Problem Frame

`.autoharness/backlog-registry.yaml` plus `internal/cli/registry_parity_test.go`
enforce **surface** parity: every live MCP tool has a registry row with either a
resolvable `cli_command` or `mcp_only: true`, every `cli_command` resolves to a
real cobra command, and flag/positional presence matches
(`registry_parity_test.go:152-218`).

That is not behavioral parity. Two logically identical operations do different
things depending on which surface ran:

| Surface | What "associate a commit" actually writes |
|---|---|
| CLI `update --commit` | frontmatter scalar only (`internal/cli/update.go:194-196`) |
| MCP `track_commit` | `commit_links` row + best-effort JSONL event, **no** markdown (`internal/mcp/tools.go:1474` → `internal/core/commits.go:27-56`) |
| MCP `update_item(commit=…)` | frontmatter scalar only (`internal/mcp/tools.go:736-746`) |

All three pass the parity test today while producing three different states. The
registry even maps `track_commit`'s `cli_command` to
`backlogit update {{task_id}} --commit {{sha}}` — a command that does something
materially different.

Separately, `LinkCommit` warns and continues on JSONL append failure and still
returns `nil` (`internal/core/commits.go:50-56`), so a caller is told the
association succeeded when only one of three representations was written.

The genuine remaining *surface* gap is the operator-only gate control
`--gate-base` / `--force-gates`, which CLI `update` exposes
(`internal/cli/update.go:346`) and MCP `update_item` does not.

### Success Criteria

* One shared core function performs commit association and updates **all**
  representations: frontmatter scalar, `commit_links`, and the JSONL event.
* That function is expressed as an **ordered list of discrete steps** — the
  frontmatter scalar and `commit_links` upsert are genuinely idempotent and
  reversible; the JSONL append is append-only, sequenced last, and never itself
  compensated (see U2) — so F5's compensating envelope can wrap it later
  without rewriting it.
* CLI `update --commit`, MCP `update_item(commit=…)`, and MCP `track_commit` all
  route through it and produce identical observable state.
* A parity test asserts **behavioral** equivalence for governed operations, not
  just flag presence, and **fails when the governed set is empty**.
* The registry's `track_commit` → CLI mapping is honest, and `message` / `author`
  handling on the CLI fallback is explicitly defined.
* `--force-gates` is marked in the registry as intentionally human-terminal-only,
  so an agent can tell deliberate asymmetry from drift.
* Blast radius on the alternate surface is equal or lower — never higher.

### Scope Boundaries

**In scope:** the shared commit-association core function; routing all three call
sites through it; removing the best-effort append from inside the governed path;
a behavioral-parity assertion for governed operations; the registry row and its
documentation.

**Out of scope:** adding `--force-gates` to MCP. The gate broker contract is
explicit that forcing is a deliberate human-at-a-terminal action, and the
established rule is that a fallback surface must never be **more** dangerous than
the surface it mirrors. Defining "governed" for every operation in the registry
(this unit fixes commit association and establishes the mechanism; extending the
governed set is a follow-up). Any change to `internal/core/gate/*`. F5's failure
envelope — F6 lands first and F5 wraps it afterwards, which is why U2 specifies
discrete steps (with the JSONL append's honest append-only, never-compensated
semantics) rather than a monolithic function.

## Requirements Trace

| Requirement | Source | Unit |
|---|---|---|
| One shared core function for governed ops | Spike Q7 contract requirement | U2 |
| Expressed as discrete steps so F5 can wrap it | Review Architecture P1 | U2 |
| Behavioral parity asserted, not surface parity | Spike Q7 | U5 |
| Governed marker must exist before it is consumed | Review parity F6-1 | U4 |
| Do not mint an `EventWriter` inside shared core | `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md` | U2 |
| Reload from markdown at the re-persist seam | `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md` | U2 |
| Registry `cli_command` must be honest | `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` | U4 |
| Denylist-derived parity set, never an allowlist | `docs/compound/2026-07-23-cli-mcp-filter-param-denylist-parity-test.md` | U5 |
| Force-gates asymmetry legible to agents | Review parity F6-3 | U4, U6 |
| Blast radius on the alternate surface must not increase | Same | scope boundary |

## Implementation Units

### U1 — Failing parity characterization: three surfaces, three states (tests)

Add a red test that performs commit association through each of the three
surfaces against equivalent fixtures and asserts all three produce identical
observable state (frontmatter scalar, `commit_links` row, JSONL event). It must
be **observed failing at HEAD**. Also capture a characterization of today's
`LinkCommit` best-effort behavior before it is removed.

Files: `internal/cli/commit_association_parity_test.go`.
Scenarios: CLI vs `track_commit`; CLI vs `update_item(commit=…)`; JSONL append
failure currently returns `nil`.
Posture: characterization-first (RED).

### U2 — Shared commit-association core function, expressed as discrete steps (code)

Introduce `core.AssociateCommit` covering all three representations. It is built
as an **ordered list of discrete steps** (frontmatter scalar → `commit_links`
upsert → JSONL append, **append deliberately last**), **specifically so F5's
compensating envelope can wrap it later without rewriting it**. A monolithic
function would force a rewrite in F5; that structural coupling is resolved here
by design rather than by resequencing the operator-mandated F1 → F4 → F6 → F5
order.

**Honest step semantics (not all three steps are the same shape).** The
frontmatter scalar write and the `commit_links` upsert are genuinely idempotent
and reversible: re-applying them converges, and compensating them (reset the
scalar, delete the upserted row) is safe. The **JSONL append step is not** —
`events.EventWriter.AppendEvent` (`internal/events/stream.go:87-124`) is a plain
append with no deduplication key, and it explicitly documents that a partial
write or fsync failure is `ErrWriteIndeterminate`, which the writer itself
states is unsafe to blindly retry. The append step is therefore:

* sequenced **last** precisely so no later step's failure can ever require
  compensating it — there is nothing after it in the ordered list;
* its own `Compensate` is a **documented no-op**: an audit trail is never
  rewritten or deleted, only ever appended to;
* if the append itself returns `ErrWriteNotApplied` (failure before any bytes
  were written — e.g. `MkdirAll`/`OpenFile` failure), the whole `AssociateCommit`
  call is safe to retry because nothing was appended;
* if the append itself returns `ErrWriteIndeterminate`, it is **not** retried
  and does **not** trigger compensation of the two prior steps — this matches
  F5's existing commit-then-surface rule for indeterminate outcomes exactly, so
  no new deduplication or locking mechanism is introduced here.

Reload the artifact **from markdown** via `findArtifact` (never the DB fast path,
which is lossy for `item_links` and `archived_status`). Thread the caller's
`*events.EventWriter` as a parameter. **Both surfaces pass a real instance** —
MCP passes the server's shared instance; the CLI constructs a per-invocation
writer via the existing workspace writer constructor, mirroring the checkpoint
disposition plan's resolution of this same question
(`docs/exec-plans/2026-08-07-checkpoint-administrative-disposition-plan.md`).
**No caller passes `nil`**, because a typed-error-on-append-failure contract and
a nil writer are contradictory — a permanently-nil CLI writer would make every
CLI-side append fail. The core function never mints one itself. Return a typed
error on append failure instead of warning and continuing.

Files: `internal/core/commits.go`.
Scenarios: all three representations written; frontmatter and `commit_links`
steps are individually re-runnable and compensable; the JSONL append step's
`Compensate` is asserted to be a no-op and is never invoked because nothing
follows it; append returning `ErrWriteNotApplied` before any bytes are written
leaves the whole call safely retryable; append returning
`ErrWriteIndeterminate` is surfaced without retry and without compensating the
prior two steps; markdown reload used; both surfaces construct and pass a real
writer, never nil; append failure surfaces a typed error.
Posture: test-first.

### U3 — Route all three surfaces through the shared function (code)

CLI `update --commit` (`internal/cli/update.go:194-196`), MCP
`update_item(commit=…)` (`internal/mcp/tools.go:736-746`), and MCP
`track_commit` (`:1474`) all call `core.AssociateCommit`. Behavior-preserving
elsewhere: no other `update` field handling changes in this unit. Define
explicitly whether `message` and `author` are preserved, defaulted, or
intentionally unavailable on the CLI fallback, and document the chosen answer.

Files: `internal/cli/update.go`, `internal/mcp/tools.go`.
Scenarios: U1's red parity tests turn green for all three entry points;
`message`/`author` behavior matches the declared contract; unrelated `update`
fields unchanged; `--section`-only updates still take the raw-frontmatter path.
Posture: test-first.

### U4 — Registry governed markers and honest mapping (config)

Add `governed: true` markers for **all three** commit-association entry points
(`update_task` with `commit`, `track_commit`, and the CLI `update --commit`
mapping), and correct the `track_commit` `cli_command` mapping so it is honest.
Add explicit registry metadata marking `--force-gates` / `--gate-base` as
intentionally human-terminal-only and non-replicated, so an agent reading the
registry can tell deliberate asymmetry from drift.

This unit **precedes** the behavioral assertion so the assertion cannot run
against an empty governed set.

Files: `.autoharness/backlog-registry.yaml`.
Scenarios: all three entry points carry the marker; the `track_commit` mapping
resolves to a command with equivalent behavior; the force-gates metadata is
present.
Posture: configuration.

### U5 — Behavioral parity assertion for governed operations (tests)

Extend `registry_parity_test.go` with a governed-operation behavioral assertion:
for each operation declared `governed: true`, execute both surfaces against
equivalent fixtures and assert identical observable state. Derive the covered set
with a **denylist** of output-only concerns, so a newly added governed operation
enters the set automatically and fails rather than silently passing.

**An empty governed set is a test failure**, and the assertion additionally
requires the commit-association operation to be present by name, so the test can
never pass vacuously. Add a regression assertion that `--force-gates` is absent
from the MCP surface, pinning the safety boundary.

Files: `internal/cli/registry_parity_test.go`.
Scenarios: all three commit-association entry points match; a deliberately
divergent fixture fails the assertion; an empty governed set fails; a newly added
governed op is auto-covered; `--force-gates` remains CLI-only.
Posture: test-first.

### U6 — Document the governed-operation contract (docs)

Document the governed-operation contract, the three converged entry points, the
`message`/`author` decision, and the deliberate `--force-gates` asymmetry with
its blast-radius rationale, in terms an agent consumer can act on.

Files: `.github/instructions/backlogit-yaml-header-tooling.instructions.md`,
`docs/design-docs/governed-operation-parity.md`.
Posture: documentation.

## Dependency Graph

```text
U1 ──> U2 ──> U3 ──> U4 ──> U5 ──> U6
```

Strictly sequential. `U4` (registry markers) deliberately precedes `U5` (the
behavioral assertion) so the assertion cannot run against an empty governed set.
`U6` last so the documentation describes shipped behavior.

## Decisions and Rationale

* **One core function, three thin call sites** — the spike's contract requirement
  verbatim. It makes divergence structurally impossible rather than
  test-detected. It is built as discrete idempotent steps so F5 can wrap it
  without a rewrite.
* **Thread the caller's `EventWriter`** — `EventWriter.mu` only serializes callers
  sharing one instance. Minting a writer inside the core function would give each
  concurrent MCP call its own mutex and can interleave the per-item JSONL. This
  was a real reviewed defect, not a style preference.
* **Reload from markdown, not the DB fast path** — the DB path is lossy for
  `item_links` and `archived_status`, and a split load (DB for the guard,
  markdown for the persist) races. Both were explicitly discarded in review.
* **Remove the best-effort append inside the governed path** — silent partial
  success is the defect. This is a deliberate behavior change, characterized in
  U1 before removal.
* **Do not add `--force-gates` to MCP** — forcing is a deliberate
  human-at-a-terminal action; the alternate surface must never be more dangerous.
* **Denylist, not allowlist** — an allowlist produces a false green when a new
  governed operation appears.

## Risks and Caveats

| Risk | Severity | Mitigation |
|---|---|---|
| Behavior change: commit association now writes more than before | medium | Characterized in U1; documented in U6; all three surfaces converge on the strictest existing behavior |
| Callers now see errors where they previously saw success | medium | Intentional; the typed error is the point. Surfaced in closure notes |
| Fresh `EventWriter` minted inside core | **high** | Writer is a required parameter; a test asserts the passed instance is used |
| Lossy `loadArtifact` used at the re-persist seam | **high** | `findArtifact` mandated in the unit description and asserted |
| Parity assertion passes vacuously on an empty governed set | **high** | U4 lands the markers before U5; an empty set is a test failure and the commit operation is required by name |
| Parity test passes while behavior diverges | medium | Denylist-derived set plus a deliberately divergent negative fixture |
| Registry `cli_command` fabricated | medium | Honest-mapping rule; U4 corrects the existing dishonest row |
| Agent cannot distinguish deliberate asymmetry from drift | medium | U4 registry metadata plus a regression assertion pinning `--force-gates` as CLI-only |
| Monolithic function blocks F5's envelope | **high** | U2 specifies discrete steps up front, with honest idempotent-vs-append-only semantics per step |
| Scope creep into a general "governed" taxonomy | medium | Scope boundary limits this unit to commit association plus the mechanism |

## Constitution Check

| Principle | Assessment |
|---|---|
| I. Safety-First Go | No `unsafe`. Typed error replaces a swallowed warning. |
| II. Test-First | U1 is an explicit RED characterization observed failing at HEAD. |
| III. Workspace Isolation | No new paths. |
| IV. CLI Containment | No writes outside the workspace. |
| V. Structured Observability | Strictly improved — a previously silent failure becomes a typed error. |
| VI. Single Responsibility | No new dependencies; consolidates existing code. |
| X. Context Efficiency | No change to query shape. |

No violations.

## Plan Hardening Signals

* public API or contract change: the governed-operation contract and a changed
  error surface — **yes**
* migration, backfill, or destructive action — no
* auth, security, or permission-sensitive behavior — no
* high rollback risk — no

Requires plan hardening: yes

## Runtime Verification and Closure

* **Verification surface:** CLI `update --commit`; MCP `update_item(commit=…)`;
  MCP `track_commit`. **The MCP path must be exercised directly**, not inferred
  from the CLI — MCP defaults `RootPath` to `"."`, a relative-root path the CLI
  never exercises.
* **Scenarios:** all three surfaces converge; append failure surfaces a typed
  error; `--section`-only updates still take the raw-frontmatter path;
  `--force-gates` remains CLI-only; an empty governed set fails the parity test.
* **Rollback:** plain revert; no persistent state introduced.
* **Closure artifact:** must record the deliberate error-surface change, the
  `message`/`author` decision, and the deliberate `--force-gates` asymmetry with
  its blast-radius rationale.

## Plan Hardening

Hardening was required (contract change on a governed operation plus a
deliberate error-surface change on an agent-facing surface).

### Protected Invariants (must not regress)

1. `core.AssociateCommit` remains expressed as discrete steps — frontmatter and
   `commit_links` idempotent and reversible, JSONL append sequenced last and
   never compensated — so F5's
   envelope can wrap it without a rewrite.
2. The core function never mints an `events.EventWriter`; it always uses the
   caller's. Both the CLI and MCP construct and pass a real instance — no
   caller passes `nil`, mirroring the checkpoint disposition plan's resolution
   of the same question.
3. The re-persist seam reloads from **markdown** (`findArtifact`), never the DB
   fast path, and never splits the load between the guard and the persist.
4. `--force-gates` / `--gate-base` stay CLI-only; blast radius on the alternate
   surface is never increased.
5. `--section`-only updates keep their raw-frontmatter path.
6. The governed set used by the parity assertion is never empty.

### Learnings and Instructions Consulted

* `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md`
* `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`
* `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`
* `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`
* `docs/compound/2026-07-23-cli-mcp-filter-param-denylist-parity-test.md`
* `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`
* `docs/compound/2026-05-07-mcp-cli-config-parity.md`
* `.github/instructions/go.instructions.md`,
  `.github/instructions/strict-safety.instructions.md`

### Risky Actions (carry forward to Ship)

| # | ProposedAction | Targets | change_kind | ActionRisk | rollback | approval_required |
|---|---|---|---|---|---|---|
| A1 | Change what commit association writes on every surface | `internal/core/commits.go` | contract + behavior change | **high** | Plain revert | **yes** |
| A2 | Remove warn-and-continue so callers now see errors | `internal/core/commits.go` | error-surface change | moderate | Plain revert | no |
| A3 | Change the registry `track_commit` CLI mapping | `.autoharness/backlog-registry.yaml` | agent-facing contract | moderate | Plain revert | no |

`ActionResult` for every entry starts `planned`.

### Deepened Verification and Rollback (for Ship)

* **Characterize before removing.** U1 must record today's best-effort behavior
  before U2 removes it.
* **Assert the writer identity.** A test must confirm the passed
  `*events.EventWriter` instance is the one used, and that the CLI constructs
  and passes its own real writer for the whole call — never `nil`.
* **Negative fixture required.** The parity assertion must be proven to fail on a
  deliberately divergent fixture and on an empty governed set.
* **Exercise MCP directly**, including the relative `RootPath: "."` default.
* **Rollback trigger:** any observed loss of a previously written representation,
  or any agent workflow broken by the new error surface, in the first validation
  window → revert.
* **Validation window:** one full commit-association cycle across CLI and MCP,
  owned by the operator.

### Unresolved Operator Decisions

* Whether the governed set is later extended beyond commit association. Deferred;
  this unit establishes the mechanism.

## Plan Review

* **dispatch_mode: multi-agent-dispatch** (Constitution Reviewer, Scope Boundary
  Auditor, Architecture Strategist, Go Reviewer, Agent-Native Parity Reviewer,
  Learnings Researcher — cross-model).
* **Cycle 1 decision: FAIL.** P1: `core.AssociateCommit` was specified as a
  monolithic ordered operation, making it structurally impossible for F5's
  envelope to compensate a partial failure without rewriting it (Architecture);
  the unit that consumed `governed: true` was sequenced **before** the unit that
  added the first such marker, so the assertion could pass vacuously (parity
  F6-1, scope P2). P2: the plan declared `Requires plan hardening: no` while
  answering **yes** to a contract-change signal, leaving no `## Plan Hardening`
  section and no risky-action classification (Constitution F6-1); only two of
  three entry points were modelled and `message`/`author` handling on the CLI
  fallback was undefined (parity F6-2); the deliberate `--force-gates` asymmetry
  was documented only in prose, with nothing in the registry or tool contract to
  distinguish it from drift (parity F6-3).
* **Resolutions:** U2 now specifies discrete steps explicitly — frontmatter and
  `commit_links` idempotent and reversible; the JSONL append honestly
  characterized as append-only, sequenced last, and never itself compensated,
  matching F5's indeterminate-outcome rule rather than introducing new
  deduplication or locking machinery — so F5
  can wrap without a rewrite, resolving the coupling by design rather than by
  resequencing the operator-mandated order; the registry marker unit was split
  out and moved **before** the behavioral assertion, and an empty governed set is
  now a test failure with the commit operation required by name; all three entry
  points are modelled and the `message`/`author` contract must be declared;
  registry metadata plus a regression assertion now pin the `--force-gates`
  safety boundary; hardening flipped to **yes** with a full `## Plan Hardening`
  section, protected invariants, and a risky-action table; the nil-CLI-writer
  path was removed as self-contradictory with the typed-error-on-append-failure
  contract — both surfaces now construct and pass a real writer, mirroring the
  checkpoint disposition plan's resolution of the same question, and this is
  cross-referenced with a writer-identity test.

### Cycle 2 Decision

decision: PASS

* dispatch_mode: multi-agent-dispatch
* P0: 0 — P1: 0 — remaining P2/P3 accepted as advisory follow-ups.

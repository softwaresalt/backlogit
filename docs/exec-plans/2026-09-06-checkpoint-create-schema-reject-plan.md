---
chunk_strategy: h1-h2-h3
description: 'Make checkpoint create reject schema-less/non-V1 dumps by default across core, CLI, and MCP; gate legacy import behind an explicit migration-only opt-in with strict pre-write validation.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-06-checkpoint-create-schema-reject-plan.md
title: 'Checkpoint Create Rejects Schema-less Dumps by Default; Legacy Import Requires Explicit Opt-in'
---

# Checkpoint Create Rejects Schema-less Dumps by Default; Legacy Import Requires Explicit Opt-in

**Source deliberation**: `docs/decisions/2026-09-06-checkpoint-create-schema-reject-deliberation.md`
**Stash**: 6FDC4A49 (high-priority bug)

## Problem Frame

`internal/events/memory.go:CreateCheckpoint` (lines ~289-398) classifies the caller's
state dump by a `schema_version` probe and runs full CheckpointV1 validation **only** when
`schema_version == 1`. Any other syntactically valid JSON (missing / `0` / non-1
`schema_version`) skips validation and is written **verbatim** to disk via the
`legacyContextKeys` branch. The resulting file is subsequently rejected by
`ValidateCheckpoint` and flagged for quarantine by `GetCheckpoint`/`ListCheckpoints`,
inverting the write/read contract. The fix inverts the default: reject non-V1 dumps
pre-write, and gate the legacy verbatim path behind an explicit migration-only opt-in,
consistently across the shared core function, the CLI `checkpoint create` command, and the
MCP `backlogit_create_checkpoint` tool (a `governed: true` operation pair).

## Requirements Trace

| Requirement (from deliberation) | Implementation action | Unit |
|---|---|---|
| Reject schema-less/non-V1 by default | Invert classification in `CreateCheckpoint`; typed pre-write rejection | U2 |
| Legacy import behind explicit opt-in | Add `allowLegacyImport` to core signature (options value) | U1, U2 |
| Cover shared core function | `internal/events/memory.go` | U2 |
| Cover CLI `checkpoint create` | `--allow-legacy-import` flag, wired to core | U3 |
| Cover MCP `backlogit_create_checkpoint` | optional `allow_legacy_import` param + handler | U4 |
| Cover schemas/tool-descriptions/docs | tool description + CLI reference + design docs | U4, U5 |
| Strict pre-write validation; no file on reject | keep classification before `syncWriteFileAtomicHook`; assert no file | U1, U2 |
| Preserve size/secret/dup-key/closed-namespace | leave pre-branch and V1-branch guards intact | U2 |
| Explicit semantics: missing/zero/unsupported/malformed/legacy | encode matrix in tests + behavior | U1, U2 |
| CLI/MCP parity | single core option; registry-dispatching parity fixtures | U3, U4 |

### Semantics matrix (authoritative)

| `schema_version` | Default (no opt-in) | With opt-in (migration only) |
|---|---|---|
| `1` valid V1 | Accept (existing V1 path) | Accept (opt-in inert) |
| missing | **REJECT**, no file | **Upgrade→validate→write** (coerce to `1`, populate defaults, run V1 validation); else **REJECT**, no file |
| `0` | **REJECT**, no file | **Upgrade→validate→write** (same as missing); else **REJECT**, no file |
| unsupported/future (`2`, negative) | **REJECT**, no file | **REJECT**, no file |
| wrong-typed / ambiguous (`"0"`, `null`, non-integral, out-of-range, **duplicate** `schema_version` members) | **REJECT**, no file | **REJECT**, no file |
| malformed JSON | **REJECT**, no file | **REJECT**, no file |

Legacy eligibility is defined precisely: an **absent** `schema_version` member, or **exactly
one** `schema_version` member whose raw JSON token is the **integer literal `0`**. Anything
else (string `"0"`, `null`, `1.5`, overflow, or duplicate members) is NOT a legacy shape and
is rejected on both paths before the write site. Legacy import is **upgrade-or-reject, never
verbatim**: a successful import always produces a record readable by
`GetCheckpoint`/`ListCheckpoints` without quarantine.

Preserved unconditionally before the write site: input size limit, secret scan. Duplicate-
key and closed-namespace checks apply exactly as today on the V1 path (and on the upgraded
legacy-import path, which runs the same V1 validation).

## Implementation Units

### U1 — Declarations & test harness (execution posture: test-first / scaffolding)

* **What**: Introduce the create-time opt-in and rejection vocabulary and the failing test
  harness that encodes the semantics matrix, with **no behavior change yet**.
  * Add the create-time opt-in via **variadic functional options** so existing 3-arg
    callers compile unchanged and the opt-in is added incrementally:
    `CreateCheckpoint(ctx, dir, dump, opts ...CreateCheckpointOption)` with
    `WithAllowLegacyImport()` in `internal/events`. The zero-option call is strict-by-
    default. (Variadic options — not a mandatory struct param — avoid a repo-wide
    compilation break at U2; see Decisions.)
  * Add a dedicated create-time sentinel error (e.g. `ErrCheckpointSchemaRejected`) in
    `internal/errors/checkpoint_errors.go`. Its message is **surface-neutral** (e.g.
    "schema-less/non-V1 checkpoint rejected; legacy import must be explicitly enabled") —
    it MUST NOT hard-code a surface-specific token; each surface appends its own
    `--allow-legacy-import` / `allow_legacy_import` remediation in its error-mapping layer.
    When a cause is preserved, wrap with the two-`%w` discriminator form
    (`fmt.Errorf("...: %w: %w", ErrCheckpointSchemaRejected, cause)`) so the sentinel stays
    `errors.Is`-detectable AND the cause stays traversable. Reuse the existing
    malformed-input error for malformed JSON.
  * Add test fixtures + shared assertion scaffolding in a focused
    `internal/events/checkpoint_create_schema_test.go` that reuse the existing
    `assertNoCheckpointWritten` helper (first confirm it uses `t.Helper()` and asserts the
    checkpoint dir is **empty**, not merely a specific filename absent) and encode the
    matrix rows as a table (tests reference the new options/error and are expected to fail
    until U2). These tests override package-global seams, so they MUST NOT use
    `t.Parallel()`.
* **Files** (< 3 production): `internal/errors/checkpoint_errors.go`,
  `internal/events/checkpoint_create_options.go` (new; keeps the options + sentinel wiring
  out of `checkpoint_schema.go`); test file `internal/events/checkpoint_create_schema_test.go`
  (test file, excluded from the production-file budget).
* **Tests / verify**: package compiles; new sentinel + options exist; harness present
  (red). `go build ./...` and `go vet ./internal/events/...` pass.
* **Milestone**: contract + red harness in place.

### U2 — Core reject-by-default behavior (execution posture: test-first)

* **What**: Invert `CreateCheckpoint` classification. Require `schema_version == 1` by
  default; when the dump is missing/`0`/unsupported/wrong-typed/malformed, reject with the
  appropriate typed error **before** `syncWriteFileAtomicHook`. Classify `schema_version`
  from the **raw JSON token** (e.g. `json.RawMessage`/`json.Number`), not an `int`
  zero-value probe, so a present-but-wrong-typed or duplicated member is distinguished from
  an absent one and rejected rather than silently collapsing to `0`. When the
  `WithAllowLegacyImport()` option is set AND the dump is a legacy shape (absent member or a
  single integer-literal `0`), **upgrade** it: coerce `schema_version` to `1`, run the SAME
  V1 path (populate `created_at`/`updated_at`/`status`, closed-namespace, duplicate-key,
  `ValidateCheckpoint`, canonical marshal) and write only if it validates; otherwise reject
  with no file. Never write a legacy dump verbatim. Still reject unsupported/future
  versions, wrong-typed/ambiguous members, and malformed JSON even under the opt-in.
  Preserve the pre-branch size + secret guards and the V1-branch duplicate-key/closed-
  namespace checks unchanged, and keep the `TestSyncWriteFileAtomic_NoPreRemoveInAST` guard
  green (no Windows pre-Remove near the write site).
* **Files** (< 3): `internal/events/memory.go` and the U1 test file turning green. The
  variadic-options seam means **no** CLI/MCP call-site edits are required in U2 (they compile
  unchanged); the surfaces opt in during U3/U4. No helper split-out is planned (deterministic
  single-file core change).
* **Tests / verify** (table-driven, matrix rows as one focused scenario set): default
  rejects missing/`0`/unsupported/wrong-typed/malformed with **no file**; opt-in upgrades
  missing/`0` to a valid V1 that is then **readable by `GetCheckpoint`/`ListCheckpoints`
  without quarantine**; opt-in still rejects unsupported/future/wrong-typed/malformed and a
  legacy shape that cannot be made valid; valid V1 unchanged; size/secret/dup-key/closed-
  namespace regressions still fire; a producer-level `errors.Is(err, ErrCheckpointSchemaRejected)`
  assertion on core output (independent of surface tests). Tests overriding global seams do
  not use `t.Parallel()`. `go test ./internal/events/...` green.
* **Milestone**: core behavior correct; no-file on every rejection path; every successful
  legacy import is a valid, readable V1 record.
* **Depends on**: U1.

### U3 — CLI surface (execution posture: test-first)

* **What**: Add a migration-only `--allow-legacy-import` boolean flag to
  `newCheckpointCreateCmd` (`internal/cli/checkpoint.go`); pass `WithAllowLegacyImport()`
  into the core when set; keep `--state-dump` required; update the command long-help to state
  the new default-reject behavior and that the flag is for migration only (remove the
  "written verbatim" advertisement). The CLI error-mapping layer appends the
  `--allow-legacy-import` remediation to the surface-neutral sentinel message.
* **Files** (< 3): `internal/cli/checkpoint.go`, `internal/cli/checkpoint_create_test.go`
  (or `checkpoint_create_shape_test.go`).
* **Tests / verify**: CLI create without flag rejects a schema-less dump (non-zero exit,
  sentinel preserved through the `%w` wrap, no file); with `--allow-legacy-import` a legacy
  dump is imported and readable; flag defaults to false. Use real `cli.NewRootCommand()`
  dispatch. `go test ./internal/cli/...` green.
* **Milestone**: CLI parity with core; flag off by default.
* **Depends on**: U2.

### U4 — MCP surface & tool schema/description (execution posture: test-first)

* **What**: Add an optional `allow_legacy_import` boolean parameter to the
  `backlogit_create_checkpoint` tool schema; parse it in `handleCreateCheckpoint`
  (`internal/mcp/tools.go`) and pass `WithAllowLegacyImport()` into the core; update the
  tool description to (a) state the changed default ("default changed: non-V1 dumps now
  rejected"), (b) convey the opt-in scope compactly (accepts only legacy shapes — absent or
  integer-literal `0` `schema_version`; future/unsupported/wrong-typed/malformed remain
  rejected; import upgrades to valid V1), and (c) drop the "written verbatim with no schema
  validation" text. The MCP sentinel-to-error mapping is **mandatory (not "if needed")**:
  `ErrCheckpointSchemaRejected` maps to a bounded validation-class MCP error carrying the
  sentinel-equivalent code, and the MCP error-mapping layer appends the `allow_legacy_import`
  remediation (NOT the CLI flag spelling). Preserve the sentinel via `%w` so
  `errors.Is`/`errors.As` resolve through the handler.
* **Files** (< 3): `internal/mcp/tools.go`, `tests/contract/checkpoint_tools_test.go` (or
  `internal/mcp/checkpoint_create_strict_test.go`).
* **Tests / verify**: MCP create without the param rejects a schema-less dump and writes no
  file, surfacing the bounded sentinel-equivalent error code (parity assertion on error
  class, not just accept/reject outcome); with `allow_legacy_import: true` a legacy dump is
  imported and readable; default false. Use in-process registered-tool dispatch
  (`callToolForTest`) with a separate `t.TempDir()` per surface. `go test ./internal/mcp/...
  ./tests/contract/...` green.
* **Milestone**: MCP parity with core + CLI (accept/reject AND error class); governed-parity
  fixtures dispatch the registered handler.
* **Depends on**: U2. (Parallel with U3.)

### U5 — Documentation & compatibility (execution posture: docs)

* **What**: Update operator-facing docs to match the new contract: CLI reference
  (`docs/cli-reference/backlogit_checkpoint_create.md`) — document `--allow-legacy-import`,
  default-reject behavior, and the semantics matrix; design docs
  (`docs/design-docs/checkpoint-administrative-disposition.md` and/or
  `governed-operation-parity.md`) — record that create is now strict-by-default and the
  legacy path is opt-in; add a short compatibility note. Ensure `make docs-lint` passes.
* **Files** (< 3): `docs/cli-reference/backlogit_checkpoint_create.md`,
  `docs/design-docs/checkpoint-administrative-disposition.md` (+ optionally
  `governed-operation-parity.md` as a single grouped doc edit).
* **Tests / verify**: `go run ./cmd/backlogit docs lint` reports 0 violations; docs
  describe flag, default behavior, and matrix.
* **Milestone**: docs and compatibility guidance consistent with shipped behavior.
* **Depends on**: U3 and U4.

## Dependency Graph

```
U1 ─▶ U2 ─▶ U3 ─▶ U5
             └▶ U4 ─▶ U5
      (U3 and U4 both depend on U2 and may run in parallel; U5 depends on U3 and U4)
```

No cycles.

## Decisions and Rationale

* **Variadic functional options, not a mandatory struct parameter** — `CreateCheckpoint(ctx,
  dir, dump, opts ...CreateCheckpointOption)`. A mandatory 4th parameter would break
  repo-wide compilation (`go build ./...` / `go test ./...`) at the U2 milestone because
  every existing 3-arg caller (CLI, MCP, resumption call sites) would fail to compile until
  U3/U4. Variadic options keep the repo green at each unit boundary while giving the same
  extensibility and a strict-by-default zero-option call. (Raised by Go Reviewer and
  Architecture Strategist at plan review.)
* **Opt-in scoped to legacy shapes only, defined by raw JSON token** — a legacy shape is an
  **absent** `schema_version` member or **exactly one** member whose token is the integer
  literal `0`. Classify from the raw token (`json.RawMessage`/`json.Number`), never an `int`
  zero-value probe, so a present-but-wrong-typed (`"0"`, `null`, `1.5`, overflow) or
  **duplicated** member is rejected rather than silently collapsing to `0`. A
  future/unsupported version is "unknown", not "legacy". Malformed JSON is never importable.
  (Raised by Go Reviewer and Security Lens Reviewer.)
* **Legacy import upgrades to valid V1 — never verbatim** — the opt-in coerces a legacy
  shape to `schema_version:1` and runs the SAME V1 validation path, writing only a record
  that reads back without quarantine. Writing legacy JSON verbatim behind a flag would
  reintroduce the exact quarantine-poisoning the fix removes. Enforcing the invariant in the
  core write seam *before* schema resolution (not at the surfaces) follows the
  enforce-before-schema precedent (compound 2026-07-30
  `task-only-typed-metadata-seam-enforce-before-schema`). (Raised by Security Lens Reviewer.)
* **Dedicated create-time sentinel in `internal/errors`, surface-neutral message** —
  `internal/errors` is the only cycle-free leaf (CLI→MCP import direction), so a shared error
  there keeps both surfaces aligned (compound 2026-09-04 neutral-leaf ownership). The message
  is surface-neutral; each surface appends its own `--allow-legacy-import` /
  `allow_legacy_import` remediation token in its error-mapping layer, avoiding CLI/MCP message
  drift. When a cause is preserved, wrap with the two-`%w` discriminator
  (`%w: %w`, sentinel, cause) so the sentinel stays `errors.Is`-detectable and the cause
  stays traversable (compound 2026-09-04 `two-percent-w-discriminator`). (Raised by Go
  Reviewer, Agent-Native Parity Reviewer, Learnings Researcher.)
* **Mandatory bounded MCP error mapping** — the sentinel maps to a bounded validation-class
  MCP error (not "if needed"), with a parity assertion on error *class*, so the agent path is
  as legible and governed as the CLI path. (Raised by Agent-Native Parity Reviewer.)
* **Classification stays before the atomic write site** — structurally guarantees the
  no-file-on-reject property; reuse `assertNoCheckpointWritten` (confirmed to use `t.Helper()`
  and assert an empty dir) and the `checkpoint_writesite_test.go` static guard; keep the
  `TestSyncWriteFileAtomic_NoPreRemoveInAST` guard green (no Windows pre-Remove). Tests that
  override package-global seams do not use `t.Parallel()` (compound 2026-07-29
  durable-writes-test-seam; 2026-08-29 windows-preremove-rename). (Raised by Learnings
  Researcher.)

## Risks and Caveats

* **Behavior/contract change** (default now rejects previously-accepted schema-less dumps,
  including for agents on a cached tool schema): mitigated by the explicit, documented
  `--allow-legacy-import` / `allow_legacy_import` migration path, an explicit "default
  changed" signal in the MCP tool description, and doc updates on both surfaces. See Plan
  Hardening.
* **Quarantine-poisoning via the opt-in**: closed by upgrade-or-reject — a successful legacy
  import is always a valid, readable V1 record (asserted end-to-end in U2). The opt-in is not
  a schema-bypass.
* **CLI/MCP parity drift**: mitigated by a single shared core option and
  authoritative-registry-dispatching parity fixtures (compound 2026-08-15), with parity
  asserted on both accept/reject outcome AND error class. Separate `t.TempDir()` per surface
  avoids second-precision checkpoint filename collisions.
* **Preserved-guard regression**: mitigated by explicit regression assertions in U2 that
  size/secret/dup-key/closed-namespace still fire, and by keeping the write-site static guards
  green.
* **No on-disk migration in scope**: existing invalid files remain a quarantine concern
  (already handled by 136-F); this fix only stops manufacturing new ones.

## Constitution Check

Mapped against `.github/instructions/constitution.instructions.md`:

* **Safety-First Go (I)** — pass. Production code stays Go; new sentinel wraps with `%w`
  (two-`%w` when a cause is preserved); no `unsafe`. Typed errors, not string matching;
  `golangci-lint`/`go vet` expected clean.
* **Test-First Development (II, NON-NEGOTIABLE)** — pass. U1 lands the red harness; U2/U3/U4
  turn it green; each unit lands a failing test before production code; table-driven with
  `t.Run`, `testify`.
* **Workspace Isolation and Security Boundaries (III)** — pass. Checkpoint writes stay within
  the workspace checkpoint dir; the change *tightens* what is written; secret scanning is
  preserved on both paths; no path-traversal surface introduced.
* **CLI Workspace Containment (IV, NON-NEGOTIABLE)** — pass. No creation/modification outside
  the working tree; the CLI flag only changes acceptance of the provided `--state-dump`.
* **Structured Observability (V)** — pass. Rejections return a typed, `errors.Is`-detectable
  sentinel and (MCP) a bounded structured error; the closure section's monitoring signal
  tracks reject/quarantine outcomes so newly-rejected creates are diagnosable post-rollout.
* **Single Responsibility (VI)** — pass. No new external dependencies; uses stdlib
  (`encoding/json`) plus existing `internal/errors` and `internal/events`. No dependency-graph
  change.
* **Destructive Command Approval (VII, NON-NEGOTIABLE)** — N/A. No destructive data/config
  action; a rejected create writes nothing. Legacy import is additive, opt-in, and
  upgrade-or-reject.
* **Explicit Safety Modes for Elevated Risk (VIII)** — pass. This is a medium-risk change to a
  governed pair on the fail-closed crash-resumption boundary; the plan applies a careful
  posture — plan-harden performed, scope frozen to the checkpoint-create path, risky actions
  classified with `ProposedAction`/`ActionRisk`, and rollback = revert the single merge commit.
* **Git-Friendly Persistence (IX)** — pass. Checkpoints remain human-readable canonical JSON;
  the no-file-on-reject invariant depends on the preserved atomic-write ordering
  (`syncWriteFileAtomic`), which is not perturbed (Windows pre-Remove guard stays green).
* **Agent Context Efficiency (X)** — pass. Errors surface as typed/bounded structured
  responses (not raw content), and the MCP tool description compactly conveys the opt-in scope
  so agents get token-efficient decision context.
* **Merge Commit History Preservation (XI, NON-NEGOTIABLE)** — pass. Ships via merge commit
  (Stage stages only; Ship executes and merges with a merge commit).
* **Supplemental — CLI/MCP parity for governed operations** (workspace convention, not a
  numbered principle) — pass. Both surfaces route through the shared core option; parity
  fixtures dispatch the registered handlers and assert accept/reject AND error class.

Constitution Check: pass

## Plan Hardening Signals

* public API, schema, or contract change — **present** (core `CreateCheckpoint` signature +
  default-behavior inversion + MCP tool schema/description change).
* security, auth, permission, or compliance-sensitive behavior — **present (adjacent)**
  (checkpoint integrity / fail-closed crash-resumption trust; secret-scan guard must be
  preserved).
* migration, backfill, destructive data/config action, or irreversible step — **present**
  (explicit legacy-import migration opt-in; the change itself is non-destructive and
  reversible).
* external integration, operator checkpoint, or external dependency — absent.
* high runtime, rollout, or rollback risk — **present (low-moderate)** (behavior change to a
  governed pair consumed by Stage/Ship resumption).

Requires plan hardening: yes

## Runtime Verification and Closure

* **Changed runtime surfaces**: CLI (`backlogit checkpoint create`) and MCP
  (`backlogit_create_checkpoint`). Core `events.CreateCheckpoint` underlies both.
* **Runtime verification before absorbed**:
  * Default CLI create of a schema-less dump exits non-zero with the typed error and leaves
    **no** file in the checkpoint dir.
  * `--allow-legacy-import` create of a legacy dump succeeds and returns the path.
  * MCP create mirrors both outcomes via registered-tool dispatch.
  * A valid V1 create still succeeds unchanged (regression).
* **Operational closure**: monitoring signal = zero new quarantine-candidate files produced
  by create after rollout; rollback trigger = unexpected rejection of valid V1 creates or a
  parity divergence between CLI and MCP; rollback procedure = revert the merge commit
  (single feature PR, self-contained); owner = Stage→Ship for this shipment; validation
  window = first post-merge Stage/Ship crash-resumption cycle exercising checkpoint create.

## Plan Hardening

**Hardening required?** Yes. Triggered by: (1) public contract change — the governed
`events.CreateCheckpoint` signature and default behavior invert, and the MCP tool schema +
CLI flag surface change; (2) security-adjacent behavior — checkpoint integrity underpins
the fail-closed crash-resumption trust boundary that both Stage and Ship depend on, and the
secret-scan guard must be preserved on every path; (3) migration path — a new explicit
legacy-import opt-in.

**Learnings and instruction files consulted**:

* `docs/compound/2026-09-04-bounded-error-and-cli-mcp-parity-patterns.md` — neutral-leaf
  (`internal/errors`) ownership of shared error DTOs; shared validation helper called by
  both surfaces before delegating.
* `docs/compound/2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md`
  — governed fixtures must dispatch the registered CLI/MCP handler, not the core function
  directly; separate `t.TempDir()` per surface to avoid second-precision filename
  collisions in checkpoint-create fixtures.
* `docs/design-docs/checkpoint-administrative-disposition.md`,
  `docs/design-docs/governed-operation-parity.md` — CheckpointV1 read/lifecycle contract and
  the governed create parity requirement (identical observable state on both surfaces).
* `.github/instructions/constitution.instructions.md` — Safety-First Go, Test-First,
  workspace/security boundaries, merge-commit preservation.

**Protected invariants** (must hold after the change):

1. **No-file-on-reject** — every rejection path returns *before* `syncWriteFileAtomicHook`;
   `assertNoCheckpointWritten` asserts an empty checkpoint dir; the
   `checkpoint_writesite_test.go` static write-site guard remains intact.
2. **Preserved guards** — input size limit and secret scanning fire on both default and
   opt-in paths; duplicate-key and closed-namespace checks fire exactly as today on the V1
   path.
3. **Governed CLI/MCP parity** — identical observable acceptance/rejection and identical
   opt-in semantics across both surfaces; both route through the shared core option.
4. **Valid-V1 creates unaffected** — the existing happy path is a regression guard.
5. **Opt-in is narrow and non-poisoning** — accepts only legacy shapes (absent
   `schema_version` member or a single integer-literal `0`); never accepts
   future/unsupported versions, wrong-typed/ambiguous/duplicate members, or malformed JSON.
   Legacy import is **upgrade-or-reject**: every successful import is coerced to a valid
   CheckpointV1 and is readable by `GetCheckpoint`/`ListCheckpoints` without quarantine
   (asserted end-to-end); no legacy dump is ever written verbatim.

**Risky actions** (`ProposedAction` / `ActionRisk` / `ActionResult`):

* `ProposedAction`: invert default classification in `events.CreateCheckpoint` so non-V1
  dumps are rejected by default.
  * `ActionRisk`: **medium** — governed behavior change to a surface consumed by Stage/Ship
    resumption; a bug could reject valid V1 creates (availability) or accept invalid ones
    (integrity). Approval: covered by this plan + plan-review gate; no separate operator
    approval needed (non-destructive, reversible by merge revert).
  * `ActionResult` (expected): valid V1 unchanged; missing/`0`/unsupported/malformed
    rejected with no file; opt-in accepts legacy shapes only.
* `ProposedAction`: change the MCP tool schema (add optional `allow_legacy_import`) and the
  CLI flag surface (add `--allow-legacy-import`).
  * `ActionRisk`: **low-medium** — additive optional parameter/flag; default false preserves
    the new strict behavior. Approval: none beyond this plan.
  * `ActionResult` (expected): both surfaces expose the opt-in identically; agents/operators
    that never set it get strict-by-default.

**Deepened runtime verification**:

* Environment precheck: run against a fresh `t.TempDir()` checkpoint dir per surface.
* Target scenarios (per surface): schema-less reject + no file; `schema_version:0` reject +
  no file; unsupported/future reject + no file; malformed reject + no file; opt-in accepts
  missing and `0`; opt-in still rejects future + malformed; valid V1 accepted; size/secret/
  dup-key/closed-namespace regressions still fire.
* Blocked-path handling: if `--allow-legacy-import` is set but the dump is malformed or a
  future version, the create must still reject with a no-file guarantee.

**Rollback and closure**:

* Rollback trigger: any valid-V1 create rejected, or CLI/MCP divergence observed
  post-merge.
* Rollback procedure: revert the single self-contained feature merge commit.
* Owner: Stage→Ship for this shipment. Validation window: first post-merge crash-resumption
  cycle exercising checkpoint create.

**Review-gate capability risk**: plan review MUST emit literal `dispatch_mode:` and
`decision:` markers. This plan touches an MCP tool surface (→ Agent-Native Parity Reviewer)
and a security-adjacent integrity boundary (→ Security Lens Reviewer), so both cross-model
personas are triggered in addition to the always-on set and Architecture Strategist. If
sub-agent dispatch is unavailable, plan review must declare
`single-agent-declared-degradation` and carry `TOOL_DEGRADED: reviewer-subagent-dispatch`
rather than silently skipping.

**Unresolved operator decisions blocking safe execution**: none. The error-taxonomy choice
(dedicated create-time sentinel vs. reuse) is a plan-level detail with no scope or safety
impact and is resolved in Decisions above (dedicated sentinel recommended).

<!-- plan-review-attempt: 1 -->
## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

TOOL_OK: reviewer-subagent-dispatch

**Attempt**: 1 of max 3. **Gate**: FAIL (two P1 findings). Plan revised in place; see attempt 2 for the authoritative PASS.

**Personas dispatched (7 selected)**: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher (always-on); Architecture Strategist (always), Agent-Native Parity Reviewer (MCP tool change → triggered), Security Lens Reviewer (security-adjacent integrity boundary → triggered). Every selected persona completed and returned findings.

**Plan hardening**: required (`Requires plan hardening: yes`) and satisfied — the plan carries a `## Plan Hardening` section with ProposedAction/ActionRisk classification and rollback/closure detail.

### P1 findings (blocking)
- **P1 (Constitution Reviewer)** — `## Constitution Check` mapped only 6 of 11 principles; V, VI, VIII, IX, X unmapped. Governance completeness gap.
- **P1 (Security Lens Reviewer, conf 0.9)** — legacy opt-in wrote arbitrary legacy JSON verbatim, recreating quarantine-poisoning at the fail-closed crash-resumption trust boundary; a successful legacy import could remain unreadable by ValidateCheckpoint. Must upgrade/canonicalize to valid V1 (or reject) and assert imports are readable.

### P2 findings
- **P2 (Go + Security Lens)** — wrong-typed/ambiguous `schema_version` (string `"0"`, `null`, non-integral, overflow, duplicate members) collapses to int zero-value and is misclassified as legacy; classify from the raw JSON token and reject ambiguous shapes before the write site.
- **P2 (Go + Architecture Strategist)** — a mandatory 4th struct parameter to `CreateCheckpoint` breaks repo-wide compilation at the U2 milestone before U3/U4 update call sites; use variadic functional options.
- **P2 (Agent-Native Parity)** — MCP error mapping was "if needed" (optional) while CLI had a concrete typed error; make it mandatory/bounded and at parity. Shared sentinel message must stay surface-neutral (not hard-code the CLI flag token).

### P3 findings (advisory, folded into the revised plan)
- Scope Boundary Auditor: resolve borderline file-count/optionality hedges in U1/U2/U5 to deterministic values (no scope creep of P2+).
- Learnings Researcher (P0=0, P1=0, high confidence): cite the enforce-before-schema precedent (compound 2026-07-30); apply the two-`%w` discriminator for the new sentinel; avoid `t.Parallel()` on package-global-seam tests; keep the Windows `NoPreRemoveInAST` write-site guard green.
- Constitution Reviewer P2/P3: surface principles V and VIII explicitly (MUST-level) and add VI/IX/X rows.

### Resolution
Plan revised in place to clear both P1s and all P2s: full 11-principle Constitution Check; legacy import changed to upgrade-or-reject (never verbatim) with end-to-end readable-import assertion; raw-token `schema_version` classification rejecting ambiguous/duplicate members; variadic functional options; surface-neutral sentinel with two-`%w` wrap and per-surface remediation; mandatory bounded MCP error mapping with error-class parity assertion. P3 advisories folded into Decisions/Risks.

<!-- plan-review-attempt: 2 -->
## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

TOOL_OK: reviewer-subagent-dispatch

**Attempt**: 2 of max 3. **Gate**: PASS (no P0/P1/P2 remaining; only folded P3 advisories).

**Verification dispatch**: the four personas that raised blocking/P2 findings in attempt 1 (Constitution Reviewer, Security Lens Reviewer, Go Reviewer, Agent-Native Parity Reviewer) were re-dispatched against the revised plan. All four returned empty finding sets:
- Constitution Reviewer: "P1 CLEARED — no remaining P0/P1 governance gap"; all 11 principles mapped, NON-NEGOTIABLEs honored, verdict consistent.
- Security Lens Reviewer: no remaining findings — upgrade-or-reject + raw-token classification resolve the P1 and P2.
- Go Reviewer: both P2s and both P3s resolved; no remaining P0/P1/P2.
- Agent-Native Parity Reviewer: both P2s and both P3s resolved; parity restored across core/CLI/MCP.

**Carried forward (attempt 1 coverage, non-blocking)**: Scope Boundary Auditor (scope well-bounded, P3 advisories only), Architecture Strategist (architecture sound, single P3), Learnings Researcher (P0=0, P1=0, high confidence). Their P3 advisories are folded into the revised plan's Decisions and Risks sections.

**Plan hardening**: required and satisfied. Constitution Check: pass. Runtime verification and operational closure specified for the changed CLI and MCP surfaces.

**Rationale**: The plan is architecturally sound (single authoritative core behavior; neutral-leaf error ownership), scope-bounded to the checkpoint-create path, security-hardened (no quarantine-poisoning; secret/size guards preserved; no-file-on-reject structurally guaranteed), and Go-idiomatic (variadic options keep the repo green at every unit boundary). Cleared for harvest.

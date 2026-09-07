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
pre-write, and gate a legacy-**import** path (**upgrade-or-reject, never verbatim**) behind
an explicit migration-only opt-in, consistently across the shared core function, the CLI
`checkpoint create` command, and the MCP `backlogit_create_checkpoint` tool (a
`governed: true` operation pair).

## Requirements Trace

| Requirement (from deliberation) | Implementation action | Unit |
|---|---|---|
| Reject schema-less/non-V1 by default | Invert classification in `CreateCheckpoint`; typed pre-write rejection | U2 |
| Legacy import behind explicit opt-in | Add `allowLegacyImport` to core signature (variadic option) | U1, U2 |
| Actionable rejection taxonomy | Two sentinels — `ErrCheckpointSchemaRejected` (legacy-eligible → opt-in remediation) vs `ErrCheckpointSchemaUnsupported` (unsupported/malformed/non-upgradable → NO opt-in remediation) | U1, U2, U3, U4 |
| Cover shared core function | `internal/events/memory.go` | U2 |
| Cover CLI `checkpoint create` | `--allow-legacy-import` flag, wired to core | U3 |
| Cover MCP `backlogit_create_checkpoint` | bounded sentinel→MCP error mapping in `internal/mcp/errors.go` (U4); optional `allow_legacy_import` param + tool schema/description + registry entry + parity fixtures (U6) | U4, U6 |
| Cover schemas/tool-descriptions/docs | tool description + registry (U6) + CLI reference + design docs (U5) | U5, U6 |
| Strict pre-write validation; no file on reject | keep classification before `syncWriteFileAtomicHook`; assert no file | U2 |
| Preserve size/secret/dup-key/closed-namespace | leave pre-branch and V1-branch guards intact | U2 |
| Explicit semantics: missing/zero/unsupported/malformed/legacy | encode matrix as the U2 behavior harness + behavior | U2 |
| CLI/MCP parity | single core option; registry-dispatching parity fixtures | U3, U4, U6 |

### Semantics matrix (authoritative)

| `schema_version` | Default (no opt-in) | With opt-in (migration only) |
|---|---|---|
| `1` valid V1 | Accept (existing V1 path) | Accept (opt-in inert) |
| missing | **REJECT**, no file — `ErrCheckpointSchemaRejected` **if the record is in the Upgrade window** (remediation: enable opt-in — it will then succeed); else `ErrCheckpointSchemaUnsupported` (no opt-in remediation) | **Deterministic upgrade→validate→write** if in the Upgrade window; otherwise **REJECT**, no file — `ErrCheckpointSchemaUnsupported` |
| `0` (single integer-literal) | same as missing (window-conditioned sentinel) | same as missing |
| unsupported/future (`2`, negative) | **REJECT**, no file — `ErrCheckpointSchemaUnsupported` | **REJECT**, no file — `ErrCheckpointSchemaUnsupported` (opt-in does not apply) |
| wrong-typed / ambiguous (`"0"`, `null`, non-integral, out-of-range, **duplicate** `schema_version` members) | **REJECT**, no file — `ErrCheckpointSchemaUnsupported` | **REJECT**, no file — `ErrCheckpointSchemaUnsupported` |
| malformed JSON | **REJECT**, no file — malformed-input error | **REJECT**, no file — malformed-input error |

**Legacy eligibility (shape gate)** is defined precisely: an **absent** `schema_version`
member, or **exactly one** `schema_version` member whose raw JSON token is the **integer
literal `0`**. Because `encoding/json` collapses duplicate object keys (last-wins), the
"exactly one member" test and the top-level `schema_version` probe are done with a
**`json.Decoder.Token()` stream scan** (reuse the existing `objectMemberKeys` pattern in
`internal/events/checkpoint_schema.go`), NOT a `json.RawMessage`/struct decode — a
`json.RawMessage` capture cannot see a duplicated member. The scan MUST be **case-fold-aware**:
`encoding/json` and `checkClosedSchemaNamespace` match keys case-insensitively
(`strings.EqualFold`, `internal/events/checkpoint_strict.go:166-170`), so the probe matches
`schema_version` by fold-equality (recognizing `SCHEMA_VERSION`, `Schema_Version`, …) and
counts **fold-equivalent** occurrences — a fold-valid `SCHEMA_VERSION: 1` is correctly
classified as valid V1 (not misclassified as schema-less), and two or more fold-equivalent
members are rejected as duplicate/ambiguous. Anything else (string `"0"`, `null`, `1.5`,
overflow, or duplicate members) is NOT a legacy shape and is rejected on both paths before the
write site as `ErrCheckpointSchemaUnsupported`.

**Upgrade window (deterministic, fail-closed) — decided by a full V1 dry-run on BOTH paths.**
Passing the shape gate is necessary but not sufficient, and a field-level predicate (identity +
closed top-level namespace) is ALSO insufficient — a schema-less object can carry a wrong-typed
`context` (e.g. `"context":"bad"`), unknown nested `progress` fields, or duplicate `context`
members that satisfy those fields yet still fail the shared V1 pipeline. Eligibility is therefore
defined operationally: run the **complete upgraded-V1 pre-write pipeline as a dry-run** — coerce
`schema_version → 1`, apply create-time defaults (`created_at`/`updated_at`, `status`), then the
closed top-level + nested-`progress` namespace checks, duplicate-key checks, and full
`ValidateCheckpoint` — and treat the record as in-window **iff that dry-run would produce a valid
CheckpointV1**. The SAME dry-run decides both paths, so the default path emits
`ErrCheckpointSchemaRejected` (actionable — enabling the opt-in *will* succeed) ONLY when the
dry-run succeeds, and `ErrCheckpointSchemaUnsupported` when it would not (enabling the opt-in
cannot help). Necessary preconditions for the dry-run to have any chance of passing (helpful for
readers, but not a substitute for the dry-run):

* **required present**: `agent ∈ {ship,stage}`, non-empty `session_id`, non-empty `phase`, and
  a **closed** top-level namespace (no foreign top-level members)
  (`internal/events/checkpoint_schema.go:21-57`);
* **defaultable-when-absent (validated only if present)**: `created_at`/`updated_at` (default
  to now) and `status` (default to `active`; if present it must be an allowed non-`abandoned`
  value);
* but a record satisfying those can still be out-of-window if the full pipeline rejects it
  (wrong-typed `context`, invalid nested `progress`, duplicate `context` members). `agent`/
  `session_id`/`phase` are never synthesized.

Records requiring field remapping or identity synthesis are **fail-closed rejected**
(`ErrCheckpointSchemaUnsupported`). Grounded in the actual corpus:

* the repository legacy corpus
  (`internal/events/testdata/legacy-corpus/fixture-{a,b,c}.json`) uses `consumer` instead of
  `agent`, omits `session_id`, and carries top-level `shipment_id`/`feature_id`/`decisions`/… —
  these violate the closed namespace and lack required identity, so they are **not** upgradable
  and are rejected;
* the checkpoint archived by this PR
  (`.backlogit/archive/checkpoints/checkpoint-20260906-231751.json`) lacks
  `schema_version`/`session_id`/`status`/`created_at`/`updated_at` and carries top-level
  `findings`/`next_step` — likewise **not** upgradable, rejected.

**No identity is ever synthesized.** `agent` and `session_id` anchor the fail-closed
crash-resumption trust boundary, so a record missing either is rejected, never guessed. The
migration claim is therefore **narrow and honest**: the opt-in rescues a record that is already
V1-valid except for a missing/`0` `schema_version` (and defaultable timestamps/status); it does
**not** promise to migrate arbitrary historical dumps. A broader corpus-remapping migration
(aliasing `consumer→agent`, relocating top-level members into the open `context` namespace,
deterministically deriving `session_id`) is explicitly **out of scope** for this fix and is
captured as a separate deferred migration feature (see Risks). Every successful import is
readable by `GetCheckpoint`/`ListCheckpoints` without quarantine; no legacy dump is ever written
verbatim.

Preserved unconditionally before the write site: input size limit, secret scan. Duplicate-
key and closed-namespace checks apply exactly as today on the V1 path (and on the upgraded
legacy-import path, which runs the same V1 validation).

## Implementation Units

### U1 — Source-shape AST harness + declarations (execution posture: declaration; harness-first)

* **What**: Land the create-time opt-in and rejection **vocabulary** only — **no behavior**.
  Per the mandatory ordering (`workflow-policies.md:123-143`,
  `harness-architect/SKILL.md:221-234`): a **source-shape (`go/ast`) harness lands first** and
  gates the declaration; **no production stub** is written; the declaration lands second and
  turns the harness green. The semantics-matrix **behavior** harness is NOT in this task — it is
  owned by U2, in a strictly later wave, against the landed declaration.
  * Declarations to land (the deliverable):
    * a `CreateCheckpointOption` type + `WithAllowLegacyImport()` constructor in a new
      `internal/events/checkpoint_create_options.go`, AND the **signature-only** change to
      `CreateCheckpoint` in `internal/events/memory.go` — widen it to
      `CreateCheckpoint(ctx, dir, dump, opts ...CreateCheckpointOption)`. This U1 edit to
      `memory.go` is **declaration only**: the variadic parameter is added (and, if referenced
      at all, only stored/ignored) while the **existing body behavior is unchanged** — it is NOT
      a stub, and NO behavior branch on `opts` is added here (that is U2). Because Go has no
      overloading, the signature necessarily lives in `memory.go`; existing 3-arg callers compile
      unchanged (variadic); the zero-option call is strict-by-default (see Decisions).
    * **Two** create-time sentinels in `internal/errors/checkpoint_errors.go`, both
      **surface-neutral** (no hard-coded surface token):
      * `ErrCheckpointSchemaRejected` — a **legacy-eligible** dump rejected because the opt-in
        was off; each surface appends its own `--allow-legacy-import` / `allow_legacy_import`
        remediation in its error-mapping layer (remediation is **actionable**: enabling the
        opt-in is the correct next step).
      * `ErrCheckpointSchemaUnsupported` — an **unsupported / future / wrong-typed / ambiguous /
        non-deterministically-upgradable** dump rejected; this class is rejected on BOTH paths
        and carries **no** opt-in remediation (enabling the opt-in cannot help), so surfaces MUST
        NOT append the opt-in token to it.
      When a cause is preserved, wrap with the two-`%w` discriminator
      (`fmt.Errorf("...: %w: %w", sentinel, cause)`) so the sentinel stays `errors.Is`-detectable
      AND the cause stays traversable. Reuse the existing malformed-input error for malformed JSON.
  * **Source-shape harness** (red until the declaration lands): a `go/ast` test that parses
    `internal/events/checkpoint_create_options.go`, `internal/events/memory.go`, and
    `internal/errors/checkpoint_errors.go` and asserts the shapes are present — the
    `CreateCheckpointOption` type, the `WithAllowLegacyImport()` constructor, the variadic
    `CreateCheckpoint(..., opts ...CreateCheckpointOption)` signature (parsed from `memory.go`),
    and both sentinel identifiers. It asserts **shape only** (no behavior, no file I/O), so it
    compiles against the pre-declaration tree and needs no stub — and it gates the signature that
    U1 actually lands.
* **Files** (3 small declaration files): `internal/errors/checkpoint_errors.go`,
  `internal/events/checkpoint_create_options.go` (new), and `internal/events/memory.go`
  (**signature-only** widening — no behavior change; U2 owns the body). Test file
  `internal/events/checkpoint_create_shape_test.go` (source-shape AST harness; excluded from the
  production-file budget). The `memory.go` touch is a trivial declaration edit, so the unit stays
  within the 2-hour envelope; its behavior change is isolated to U2.
* **Tests / verify**: source-shape harness is red before the declaration and green after; `go
  build ./...` and `go vet ./internal/events/...` pass. **No behavior test in this task.**
* **Milestone**: declaration (options type, `WithAllowLegacyImport`, widened signature, two
  sentinels) + source-shape harness landed; no production stub ahead of the harness that gates it;
  no behavior admitted.

### U2 — Behavior harness + core reject-by-default / bounded upgrade (execution posture: test-first)

* **What**: In a strictly later wave than U1's declaration, land the **semantics-matrix behavior
  harness** (red) against the landed declaration, then invert `CreateCheckpoint` **behavior** in
  `internal/events/memory.go` to turn it green (the widened **signature** already landed in U1;
  U2 changes only the body/behavior). Behavior turned green by this task and retained as a
  green-step regression guard. Require `schema_version == 1` by default; when the dump is
  missing/`0`/unsupported/wrong-typed/malformed, reject **before** `syncWriteFileAtomicHook` with
  the appropriate typed sentinel, choosing it from a **full upgraded-V1 pre-write dry-run,
  evaluated the same way on both paths**: `ErrCheckpointSchemaRejected` for a legacy shape whose
  dry-run WOULD succeed (so the default-path remediation "enable the opt-in" is actionable),
  `ErrCheckpointSchemaUnsupported` for a legacy shape whose dry-run would NOT succeed and for
  unsupported/future/wrong-typed/ambiguous, and the existing malformed-input error for malformed
  JSON. Classify `schema_version` with a
  **`json.Decoder.Token()` stream scan** (reuse the `objectMemberKeys` pattern in
  `internal/events/checkpoint_schema.go`) capturing the raw token via `json.RawMessage` — **not**
  `json.Number`, and **not** an `int` zero-value probe or a struct/`map` decode (those collapse
  duplicate keys last-wins) — so a present-but-wrong-typed value OR a **duplicated** top-level
  `schema_version` member is rejected rather than collapsing to `0`. The scan is
  **case-fold-aware** (`strings.EqualFold`, matching `encoding/json` and
  `checkClosedSchemaNamespace` at `internal/events/checkpoint_strict.go:166-170`), so a valid
  `SCHEMA_VERSION: 1` is recognized as V1 (not misclassified as schema-less) and fold-equivalent
  duplicates are rejected as ambiguous; U2 covers this case. In-window / out-of-window is decided
  by the **complete upgraded-V1 pre-write dry-run** (coerce `schema_version → 1`; default
  `created_at`/`updated_at`/`status`; closed top-level + nested-`progress` namespace checks;
  duplicate-key checks; full `ValidateCheckpoint`) — NOT by a field-level identity+namespace
  predicate alone, because a schema-less object with valid `agent`/`session_id`/`phase` can still
  fail the pipeline via a wrong-typed `context`, invalid nested `progress`, or duplicate `context`
  members. When `WithAllowLegacyImport()` is set AND the dry-run succeeds, **upgrade**: write the
  canonical V1 the dry-run validated. A legacy shape whose dry-run fails (needing field remapping
  such as `consumer→agent`, identity synthesis, top-level relocation, or carrying bad
  context/progress) is **fail-closed rejected** with `ErrCheckpointSchemaUnsupported` and **no
  file** — identity (`agent`/`session_id`) is **never** synthesized. Never write a legacy dump
  verbatim. Preserve the pre-branch size + secret guards and
  the V1-branch duplicate-key/closed-namespace checks unchanged, and keep
  `TestSyncWriteFileAtomic_NoPreRemoveInAST` green (no Windows pre-Remove near the write site).
  * **Update existing tests that assumed schema-less / verbatim success** (they encode the OLD
    contract and MUST be revised to reject-by-default so `go test ./...` stays green):
    `internal/events/memory_test.go` (`TestCreateCheckpoint_WritesFile`, writes `{"state":"test"}`),
    `internal/events/checkpoint_readpath_test.go` (`TestCreateCheckpoint_LegacyDumpWrittenVerbatim`),
    `internal/events/checkpoint_u1_harness_test.go`
    (`TestCreateCheckpoint_U1_ValidLegacyJSON_PassesThrough`),
    `internal/cli/checkpoint_create_shape_test.go:12-82` (which must also pin arrays/scalars/`null`
    as unsupported **non-object** inputs that reject with no file), and
    `tests/contract/checkpoint_tools_test.go:219-237`; plus any legacy-corpus fixture relied on for
    verbatim success. Each becomes a reject-with-no-file assertion (or an opt-in dry-run success
    assertion where the fixture qualifies). Add cases for a schema-less object with valid identity
    but wrong-typed `context` / invalid nested `progress` / duplicate `context` members → default
    `ErrCheckpointSchemaUnsupported`, and opt-in fail-closed reject.
* **Files**: `internal/events/memory.go` (production) plus the U2 behavior test file
  (`internal/events/checkpoint_create_schema_test.go`, reusing the existing
  `assertNoCheckpointWritten` helper — first confirm it uses `t.Helper()` and asserts the
  checkpoint dir is **empty**) and the three existing test files above turning to the new contract.
  The variadic-options seam means **no** CLI/MCP call-site edits are required in U2 (they compile
  unchanged); the surfaces opt in during U3/U4/U6.
* **Tests / verify** (table-driven matrix, no `t.Parallel()` on global-seam tests): default rejects
  missing/`0` as `ErrCheckpointSchemaRejected`, unsupported/future/wrong-typed as
  `ErrCheckpointSchemaUnsupported`, and malformed as the malformed-input error — each with **no
  file**; opt-in upgrades an Upgrade-window record to a valid V1 that is then **readable by
  `GetCheckpoint`/`ListCheckpoints` without quarantine**; opt-in still fail-closed rejects an
  out-of-window legacy shape (e.g. a corpus fixture with `consumer`/no-`session_id`) as
  `ErrCheckpointSchemaUnsupported`; valid V1 unchanged; size/secret/dup-key/closed-namespace
  regressions still fire. **Producer-level** `errors.Is(err, ErrCheckpointSchemaRejected)` and
  `errors.Is(err, ErrCheckpointSchemaUnsupported)` (and `errors.As` where a cause is preserved) are
  asserted here, on the CORE return value — this is the only layer where the Go error chain is
  intact (see U4 for why the MCP layer asserts the mapped error instead). `go test ./internal/events/...`
  green.
* **Milestone**: core behavior correct; no-file on every rejection path; every successful legacy
  import is a valid, readable V1 record; the three legacy-success tests now encode the strict contract.
* **Depends on**: U1.

### U3 — CLI surface (execution posture: test-first)

* **What**: Add a migration-only `--allow-legacy-import` boolean flag to
  `newCheckpointCreateCmd` (`internal/cli/checkpoint.go`); pass `WithAllowLegacyImport()`
  into the core when set; keep `--state-dump` required; update the command long-help to state
  the new default-reject behavior and that the flag is for migration only (remove the
  "written verbatim" advertisement), and convey the **same compact opt-in scope** the MCP tool
  description carries (accepts only in-window legacy shapes; future/unsupported/wrong-typed/
  malformed and remap/identity records remain rejected; import upgrades to valid V1, never
  verbatim) so both governed-pair surfaces present symmetric in-surface decision context. The
  CLI error-mapping layer appends the `--allow-legacy-import` remediation **only** to
  `ErrCheckpointSchemaRejected` (legacy-eligible); it MUST NOT append the flag to
  `ErrCheckpointSchemaUnsupported` (enabling the flag cannot help).
* **Files** (≤ 2): `internal/cli/checkpoint.go`, `internal/cli/checkpoint_create_test.go`
  (or `checkpoint_create_shape_test.go`).
* **Tests / verify**: CLI create without flag rejects a schema-less dump (non-zero exit,
  `ErrCheckpointSchemaRejected` preserved through the `%w` wrap, no file); with
  `--allow-legacy-import` an Upgrade-window dump is imported and readable; an out-of-window
  legacy dump still fails (`ErrCheckpointSchemaUnsupported`, no flag remediation); flag defaults
  to false. Use real `cli.NewRootCommand()` dispatch. `go test ./internal/cli/...` green.
* **Milestone**: CLI parity with core; flag off by default.
* **Depends on**: U2.

### U4 — MCP sentinel→error mapping (execution posture: test-first)

* **What**: Map the core sentinels to bounded MCP errors in `internal/mcp/errors.go`, where
  `domainError` and its mapping already live. `handleCreateCheckpoint` converts a domain error
  into a `CallToolResult` (via `domainError`) and returns a **nil Go error**, so
  `errors.Is`/`errors.As` do **NOT** resolve "through the handler". The mapper MUST therefore
  detect the wrapped sentinel with `errors.Is` **before** serialization and choose the MCP error
  code from it. Map `ErrCheckpointSchemaRejected` → a bounded validation-class MCP error whose
  remediation appends `allow_legacy_import` (NOT the CLI flag spelling); map
  `ErrCheckpointSchemaUnsupported` → a bounded validation-class MCP error with **no** opt-in
  remediation. The two sentinels MUST be **programmatically distinguishable at the structured
  level** (distinct MCP error codes, or a structured machine-detectable remediation/retryable
  field — not merely differing free-text), so an agent can branch "retry with opt-in" vs "give
  up" the same way the CLI branches the two typed sentinels; a test asserts that structured
  distinction. Producer-level `errors.Is`/`errors.As` chain assertions stay in U2 (core), the only
  layer where the Go error chain is intact.
* **Files** (≤ 2 production): `internal/mcp/errors.go`, `internal/mcp/error_mapping_test.go`.
* **Tests / verify**: given a wrapped `ErrCheckpointSchemaRejected` / `ErrCheckpointSchemaUnsupported`
  produced by the core, the mapper yields the expected bounded MCP error **code/body** and the
  correct (or absent) remediation token; assert on the serialized `CallToolResult` error class, not
  on a Go error return (which is nil). `go test ./internal/mcp/...` green for the mapping.
* **Milestone**: both sentinels map to bounded, correctly-remediated MCP errors, detected before
  serialization.
* **Depends on**: U2.

### U6 — MCP tool opt-in parameter, schema/description, registry & parity fixtures (execution posture: test-first)

* **What**: Expose the opt-in on the MCP surface consistently. Add an optional `allow_legacy_import`
  boolean parameter to the `backlogit_create_checkpoint` tool schema; parse it in
  `handleCreateCheckpoint` (`internal/mcp/tools.go`) and pass `WithAllowLegacyImport()` into the
  core. Update the tool description to (a) state the changed default ("default changed: non-V1 dumps
  now rejected"), (b) convey the opt-in scope compactly (accepts only legacy shapes in the Upgrade
  window — absent or integer-literal `0` `schema_version` that is otherwise V1-valid;
  future/unsupported/wrong-typed/malformed and records needing remap/identity remain rejected;
  import upgrades to valid V1, never verbatim), and (c) drop the "written verbatim with no schema
  validation" text. Advertise the new parameter in the governed operation registry
  (`.autoharness/backlog-registry.yaml`, whose `create_checkpoint` entry currently lists only
  `state_dump` + a CLI command without the opt-in): **keep the base `cli_command` strict-by-default
  — do NOT add `--allow-legacy-import` as an unconditional literal in `cli_command`**, or every
  registry-derived CLI fallback would silently enable migration mode and violate the
  strict-by-default contract. Instead list `allow_legacy_import` in the entry's `params` while
  omitting the flag from the base `cli_command` — the established optional-flag convention
  (`internal/cli/registry_parity_test.go:321-329`, as used by `archive_item.commit_sha`) — and have
  the opt-in parity fixture append `--allow-legacy-import` only in the explicit `true` scenario.
  Update the existing governed **parity fixtures / tool-metadata** so generated metadata and
  registry-driven parity tests expose the parameter on both surfaces and dispatch the flag only for
  the explicit opt-in case.
* **Files** (≤ 2 production + config): `internal/mcp/tools.go`, `.autoharness/backlog-registry.yaml`;
  governed parity fixtures / tool-metadata test surface (`tests/contract/checkpoint_tools_test.go` or
  `internal/mcp/checkpoint_create_strict_test.go`) plus the existing governed-parity fixture.
* **Tests / verify**: MCP create without the param rejects a schema-less dump and writes no file,
  surfacing the bounded U4-mapped error code (parity assertion on error **class**, not just
  accept/reject); with `allow_legacy_import: true` an Upgrade-window dump is imported and readable;
  an out-of-window legacy dump still fails as `ErrCheckpointSchemaUnsupported`; default false.
  Registry-driven parity fixtures dispatch the **registered** handler (via `callToolForTest`), not
  the core directly, with a separate `t.TempDir()` per surface. Generated tool metadata includes
  `allow_legacy_import`. `go test ./internal/mcp/... ./tests/contract/...` green.
* **Milestone**: MCP parity with core + CLI (accept/reject AND error class); registry + generated
  metadata + parity fixtures all expose the opt-in consistently.
* **Depends on**: U4 **and U3** (U6's registry-driven parity fixture dispatches the CLI
  `--allow-legacy-import` flag introduced by U3, so U3 must land first).

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
* **Depends on**: U3, U4, and U6.

## Dependency Graph

```
U1 ─▶ U2 ─┬▶ U3 ─┬──────────────▶ U5
          │      └──────┐
          └▶ U4 ─▶ U6 ──┴────────▶ U5
   (U3 and U4 both depend on U2 and may run in parallel; U6 depends on U4 **and U3**;
    U5 depends on U3, U4, and U6)
```

No cycles.

## Decisions and Rationale

* **Mandatory harness ordering: source-shape AST harness → declaration → behavior harness →
  implementation** — per `workflow-policies.md:123-143` and `harness-architect/SKILL.md:221-234`,
  a declaration-bearing seam whose body will absorb real behavior MUST be split. U1 is a
  **declaration** task: its **source-shape (`go/ast`) harness lands first** and gates the
  declaration (options type, `WithAllowLegacyImport`, the variadic signature, both sentinels);
  **no production stub** is written ahead of that harness. The **behavior** harness (the semantics
  matrix, reading/mutating/writing) is owned by U2 in a strictly later wave, landing red against
  the landed declaration and turned green by U2's implementation. Combining the declaration with
  the behavior harness in one task (the earlier draft) is the plan defect this split removes.
  (Raised at PR review — mandatory red-before-production ordering.)
* **Variadic functional options, not a mandatory struct parameter** — `CreateCheckpoint(ctx,
  dir, dump, opts ...CreateCheckpointOption)`. A mandatory 4th parameter would break
  repo-wide compilation (`go build ./...` / `go test ./...`) at the U2 milestone because
  every existing 3-arg caller (CLI, MCP, resumption call sites) would fail to compile until
  U3/U4/U6. Variadic options keep the repo green at each unit boundary while giving the same
  extensibility and a strict-by-default zero-option call. (Raised by Go Reviewer and
  Architecture Strategist at plan review.)
* **Opt-in scoped to a deterministic Upgrade window; fail-closed; no identity synthesis** — a
  legacy **shape** is an **absent** `schema_version` member or **exactly one** member whose token
  is the integer literal `0`. Classification uses a **`json.Decoder.Token()` stream scan** (reuse
  the `objectMemberKeys` pattern in `internal/events/checkpoint_schema.go`) capturing the raw token
  via `json.RawMessage` — never `json.Number`, an `int` zero-value probe, or a struct/`map` decode
  (those collapse duplicate keys last-wins) — so `"0"`/`null`/`1.5`/overflow AND a **duplicated**
  top-level member are rejected, not silently accepted. Passing the shape gate is necessary but not
  sufficient, and a field-level identity+namespace predicate is ALSO insufficient; instead the
  **complete upgraded-V1 pre-write pipeline is dry-run on both paths** (coerce → defaults → closed
  top-level + nested-`progress` namespace → dup-key → full `ValidateCheckpoint`), and the record is
  in-window **iff that dry-run would produce a valid CheckpointV1** — so the default-path
  `ErrCheckpointSchemaRejected` (enable-opt-in advice) is emitted only when the opt-in would
  actually succeed. Necessary-but-insufficient preconditions are `agent ∈ {ship,stage}`, non-empty
  `session_id`/`phase`, and no foreign top-level members (`created_at`/`updated_at`/`status` are
  defaultable-when-absent); a record meeting those can still be out-of-window if the pipeline
  rejects a wrong-typed `context`, invalid nested `progress`, or duplicate `context` members.
  `agent`/`session_id` anchor the fail-closed crash-resumption trust boundary and are **never
  synthesized**, so a record needing field remapping (`consumer→agent`), identity synthesis, or
  top-level relocation is fail-closed rejected as `ErrCheckpointSchemaUnsupported`. This narrows the
  migration claim to what the fix can honestly deliver (grounded in the actual
  `internal/events/testdata/legacy-corpus` fixtures and this PR's archived checkpoint, all of which
  are correctly rejected). A broad corpus-remapping migration is out of scope and deferred (see
  Risks). (Raised by Go + Security Lens Reviewer; narrowed and made dry-run-based per PR review.)
* **Legacy import upgrades to valid V1 — never verbatim** — the opt-in coerces an in-window legacy
  shape to `schema_version:1` and runs the SAME V1 validation path, writing only a record that
  reads back without quarantine. Writing legacy JSON verbatim behind a flag would reintroduce the
  exact quarantine-poisoning the fix removes. Enforcing the invariant in the core write seam
  *before* schema resolution (not at the surfaces) follows the enforce-before-schema precedent
  (compound 2026-07-30 `task-only-typed-metadata-seam-enforce-before-schema`). (Raised by Security
  Lens Reviewer.)
* **Two create-time sentinels in `internal/errors`, surface-neutral, actionable remediation** —
  `internal/errors` is the only cycle-free leaf (CLI→MCP import direction), so shared errors there
  keep both surfaces aligned (compound 2026-09-04 neutral-leaf ownership). The taxonomy is split so
  remediation is **actionable**: `ErrCheckpointSchemaRejected` (a legacy-eligible dump rejected with
  the opt-in off) carries the `--allow-legacy-import` / `allow_legacy_import` remediation, while
  `ErrCheckpointSchemaUnsupported` (unsupported/future/wrong-typed/ambiguous/non-upgradable, rejected
  on BOTH paths) carries **no** opt-in remediation — appending "enable the opt-in" to a case the
  opt-in cannot rescue is non-actionable advice. Both messages are surface-neutral; each surface
  appends the correct remediation only for the eligible sentinel, avoiding CLI/MCP drift. When a
  cause is preserved, wrap with the two-`%w` discriminator (`%w: %w`, sentinel, cause) so the
  sentinel stays `errors.Is`-detectable and the cause stays traversable (compound 2026-09-04
  `two-percent-w-discriminator`). (Raised by Go Reviewer, Agent-Native Parity Reviewer, Learnings
  Researcher; taxonomy split per PR-review actionability.)
* **Mandatory bounded MCP error mapping, detected before serialization** — each sentinel maps to a
  bounded validation-class MCP error (not "if needed"), with a parity assertion on error *class*.
  Because `handleCreateCheckpoint` returns a `CallToolResult` with a **nil Go error**,
  `errors.Is`/`errors.As` cannot resolve "through the handler"; the mapper in
  `internal/mcp/errors.go` (U4) MUST detect the wrapped sentinel with `errors.Is` **before**
  serialization, and the test asserts the returned MCP error code/body after registered-handler
  dispatch. Producer-level `errors.Is`/`errors.As` chain assertions stay in the core (U2), the only
  layer where the Go error chain is intact. (Raised by Agent-Native Parity Reviewer; refined per
  PR-review.)
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
* **Narrow opt-in coverage / deferred broad migration**: the opt-in's Upgrade window is
  deliberately narrow (records already V1-valid except a missing/`0` `schema_version`), so it
  does **not** rescue the representative legacy corpus (`consumer`/no-`session_id`/top-level
  extras) or this PR's archived checkpoint — those are fail-closed rejected, by design, because
  `agent`/`session_id` cannot be safely synthesized at the crash-resumption trust boundary. A
  full corpus-remapping migration (alias `consumer→agent`, relocate top-level members into the
  open `context` namespace, deterministically derive `session_id`, tested against the
  `legacy-corpus` fixtures) is a **separate, deferred migration feature** — captured as a
  `DEFERRED SCOPE EXPANSION` stash entry (`5EF84EC4`, `requires deliberation`; P-021 C1/C2:
  different contract surface — a design decision, not a mechanical part of the reject-by-default
  fix). This caveat keeps the migration claim honest and the fix bounded.

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
  * `--allow-legacy-import` create of an **in-Upgrade-window** legacy dump (already V1-valid
    except a missing/`0` `schema_version` — NOT a `legacy-corpus` fixture or this PR's archived
    checkpoint, which are intentionally out-of-window and MUST fail) succeeds and returns the
    path; an **out-of-window** legacy dump still fails with no file even under the flag.
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
    rejected with no file; opt-in accepts only **in-window** legacy shapes (upgrade-or-reject).
* `ProposedAction`: change the MCP tool schema (add optional `allow_legacy_import`) and the
  CLI flag surface (add `--allow-legacy-import`).
  * `ActionRisk`: **low-medium** — additive optional parameter/flag; default false preserves
    the new strict behavior. Approval: none beyond this plan.
  * `ActionResult` (expected): both surfaces expose the opt-in identically; agents/operators
    that never set it get strict-by-default.

**Deepened runtime verification**:

* Environment precheck: run against a fresh `t.TempDir()` checkpoint dir per surface.
* Target scenarios (per surface): schema-less reject + no file; `schema_version:0` reject +
  no file; unsupported/future reject + no file; malformed reject + no file; opt-in accepts an
  **in-window** missing/`0` dump (out-of-window legacy dumps still reject + no file); opt-in
  still rejects future + malformed; valid V1 accepted; size/secret/
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

<!-- plan-review-attempt: 3 -->
## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

TOOL_OK: reviewer-subagent-dispatch

**Attempt**: 3 of max 3. **Gate**: PASS (no P0/P1/P2 remaining; only folded P3 advisories). This
re-review supersedes the attempt-2 PASS because the plan was materially revised to address the
Copilot review on PR #428 (harness-ordering split, MCP task split, narrowed migration claim,
two-sentinel taxonomy, corpus-grounded upgrade window).

**Personas dispatched (5) + self-verified (3)**: Correctness Reviewer, Security Reviewer, Go
Reviewer, Agent-Native Parity Reviewer, Scope Boundary Auditor were dispatched as independent
sub-agents against the revised plan; Constitution (unchanged 11-principle mapping still holds),
Architecture Strategist (single authoritative core; neutral-leaf error ownership; U1/U2 and
U4/U6 seams clean), and Learnings Researcher (enforce-before-schema, two-%w, no-parallel-seam,
Windows pre-Remove guard — all retained) were carried forward/self-verified.

### Findings raised and resolution (all cleared before this PASS)
- **Go P1 x2 (signature location)** — the variadic `CreateCheckpoint` signature necessarily lives
  in `internal/events/memory.go` (Go has no overloading) and must be gated by U1's source-shape
  harness. RESOLVED: U1 now owns the signature-only widening of `memory.go` (body/behavior deferred
  to U2) and its `go/ast` harness parses `memory.go`, `checkpoint_create_options.go`, and
  `checkpoint_errors.go`. No production surface lands ahead of the harness that gates it.
- **Go P2 (duplicate-member detection)** — `json.RawMessage`/struct decode collapse duplicate keys
  (last-wins). RESOLVED: classification now uses a `json.Decoder.Token()` stream scan (reuse
  `objectMemberKeys`) so a duplicated top-level `schema_version` is rejected as
  `ErrCheckpointSchemaUnsupported`.
- **Go P3 (json.Number imprecise)** — RESOLVED: dropped; `json.RawMessage` via the token scan is the
  single capture mechanism.
- **Correctness P2 (non-actionable default sentinel)** — RESOLVED: the Upgrade-window predicate is
  now evaluated on BOTH paths; an in-window legacy shape yields `ErrCheckpointSchemaRejected`
  (opt-in advice is actionable), an out-of-window shape yields `ErrCheckpointSchemaUnsupported`.
- **Correctness P3 (status determinism)** — RESOLVED: `status`/timestamps are defaultable-when-absent
  and validated-only-if-present; the required-present window set is `agent`/`session_id`/`phase`
  plus a closed top-level namespace.
- **Security Reviewer** — no findings; upgrade-or-reject + never-synthesize-identity + no-file-on-reject
  close quarantine-poisoning and identity-fabrication at the crash-resumption trust boundary.
- **Scope Boundary Auditor** — no findings; U4->U4+U6 and U1/U2 splits are policy-required (not
  fragmentation); the broad corpus migration is correctly OUT of scope and captured as
  `DEFERRED SCOPE EXPANSION` stash `5EF84EC4`.
- **Agent-Native Parity — 3 P3 advisories, folded**: CLI long-help now conveys the same compact
  opt-in scope as the MCP tool description; U4 requires the two sentinels to be programmatically
  distinguishable at the structured MCP level (distinct code / structured field), with a test; U6
  records the opt-in in the registry with BOTH surface spellings so registry-driven parity fixtures
  dispatch each surface.

**Plan hardening**: required (`Requires plan hardening: yes`) and satisfied. Constitution Check: pass.
Runtime verification and operational closure specified for the changed CLI and MCP surfaces.

**Rationale**: The revised plan is architecturally sound, scope-bounded (narrowed, honest migration
claim; deferred broad migration captured), security-hardened, and Go-idiomatic with mandatory
harness ordering now correctly specified (source-shape AST harness -> declaration -> behavior
harness -> implementation). No P0/P1/P2 remain. Cleared for harvest.

**Post-PASS PR-review refinements folded (gate NOT reset — same-surface consistency, not design change).** After this attempt-3 PASS, Copilot PR review raised same-contract-surface consistency items, all folded without altering the design or decision: (1) case-fold-aware `schema_version` classification (a fold-valid `SCHEMA_VERSION: 1` must not be misclassified as schema-less; `strings.EqualFold` per `checkpoint_strict.go:166-170`); (2) `U6 ← U3` dependency added (U6's registry-driven parity fixture dispatches the CLI flag from U3); (3) deliberation decomposition and semantics matrix aligned to six tasks and the narrowed Upgrade window; (4) task bodies (171.002/003/004-T) propagated the plan's structured-MCP-distinguishability, CLI in-surface-scope, and fold-aware requirements; (5) `171-F` carries `source_stash_id: 6FDC4A49` for durable stash→artifact linkage. These strengthen the reviewed plan; the attempt-3 PASS stands.

**Further PR-review refinements (cycle 5), folded — same-surface, gate not reset.** (6) The default-path `ErrCheckpointSchemaRejected` actionability is now decided by a **full upgraded-V1 pre-write dry-run** (not a field-level identity+namespace predicate), so a schema-less object with valid identity but wrong-typed `context` / invalid nested `progress` / duplicate `context` members is correctly `ErrCheckpointSchemaUnsupported`; (7) the governed registry keeps its base `cli_command` **strict-by-default** — `allow_legacy_import` lives in `params`, the flag is appended only in the explicit opt-in parity scenario (`archive_item.commit_sha` convention, `registry_parity_test.go:321-329`) — so registry-derived CLI fallback cannot silently enable migration mode; (8) U2 test-update scope extended to `internal/cli/checkpoint_create_shape_test.go:12-82` (pinning arrays/scalars/null as unsupported non-object inputs) and `tests/contract/checkpoint_tools_test.go:219-237`.

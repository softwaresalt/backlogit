---
chunk_strategy: h1-h2-h3
description: "Deliberation: make checkpoint create reject schema-less/non-V1 dumps by default and gate legacy import behind an explicit migration-only opt-in (stash 6FDC4A49)."
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-06-checkpoint-create-schema-reject-deliberation.md
title: "6FDC4A49 Deliberation — Checkpoint Create Rejects Schema-less Dumps by Default"
---

# 6FDC4A49 Deliberation — Checkpoint Create Rejects Schema-less Dumps by Default

**Date**: 2026-09-06
**Status**: Decided
**Stash**: 6FDC4A49 (high-priority bug)
**Trigger**: `events.CreateCheckpoint`'s implicit legacy path writes schema-invalid
records (`schema_version:0` + zero `created_at`; or records omitting
`schema_version`/`status`/`created_at`/`updated_at`) that `GetCheckpoint` /
`ListCheckpoints` later reject as `ErrCheckpointInvalid` and flag for quarantine,
violating the CheckpointV1 read/lifecycle contract. Surfaced during PR #404/#405
pre-136-S remediation.

## Problem Frame

`internal/events/memory.go:CreateCheckpoint` classifies the caller's state dump with a
`schema_version` probe. Only when `schema_version == 1` does it run parse →
closed-namespace → duplicate-key → default population → `ValidateCheckpoint` → canonical
marshal. **Any** syntactically valid JSON whose `schema_version` is absent, `0`, or not
`1` skips the V1 branch entirely and is written to disk **verbatim** via
`legacyContextKeys`. That verbatim write produces a file that the read/lifecycle contract
(`ValidateCheckpoint`, `checkpoint_lifecycle.go`) subsequently rejects and marks as a
quarantine candidate.

The create surface therefore **manufactures invalid checkpoints**: a write succeeds that
the read side is guaranteed to reject. This is a write/read contract inversion. The bug is
confined to the *create* path — reads (`GetCheckpoint`/`ListCheckpoints`) already reject
invalid records correctly, and the operator-approved quarantine flow already dispositions
pre-existing invalid files (evidence: quarantine of `checkpoint-20260906-231751.json`
succeeded during this session's setup).

**Who cares and why.** Stage and Ship both depend on the crash-resumption checkpoint
contract being fail-closed and trustworthy. A create path that silently emits
quarantine-bound records erodes that trust, pollutes the checkpoint namespace, and forces
downstream quarantine toil.

### Constraints and Requirements

* Cover **every** creation surface consistently: the shared `events.CreateCheckpoint`
  core function, the CLI `checkpoint create` command, and the MCP
  `backlogit_create_checkpoint` tool. These are a governed operation pair
  (`create_checkpoint`, `governed: true`) and must retain CLI/MCP parity.
* Strict **pre-write** validation: a rejected create MUST leave no checkpoint file behind.
* Legacy import must remain **possible but explicit** — an opt-in flag/parameter intended
  only for migration, never the default.
* Preserve existing **size limit, secret scanning, duplicate-key, and closed-namespace**
  behavior.
* Define explicit semantics for: `schema_version` missing, zero, unsupported/future,
  malformed JSON, and the legacy opt-in.

### Success Criteria

* Default create rejects schema-less/non-V1 dumps with a typed error and writes no file.
* Legacy-shaped dumps are accepted **only** when the explicit migration opt-in is set.
* CLI and MCP expose the opt-in identically and route through the shared core function.
* Existing preserved guards continue to fire; existing valid-V1 creates are unaffected.

### Out of Scope

* Changing read-side validation (`GetCheckpoint`/`ListCheckpoints`) — already correct.
* Changing the quarantine / abandon disposition workflow (136-F) — already correct.
* Bulk migration of existing on-disk legacy files — the opt-in enables case-by-case
  import; a batch migrator is not part of this bug fix.
* Introducing a JSON Schema file under `schemas/` — the CheckpointV1 schema is Go
  struct/tag based and stays that way.

## Research Findings

* **Shared pipeline**: `internal/events/memory.go:289-398` `CreateCheckpoint(ctx, dir,
  stateDump)`. Order today: `MkdirAll` → build path → `json.Valid` (malformed guard) →
  size guard → secret scan → `schema_version` probe → **V1 branch only if == 1** → second
  secret scan → atomic write → legacy context keys if V1 branch did not run. The write
  site is `syncWriteFileAtomicHook(path, data, 0o644)` at ~`memory.go:385-388`
  (`internal/events/fsutil.go:67-117`), statically guarded by
  `checkpoint_writesite_test.go`.
* **Schema/validation**: `internal/events/checkpoint_schema.go` — `CheckpointV1` with
  `SchemaVersion int validate:"eq=1"`; `ParseCheckpoint` (→ `ErrCheckpointCorrupt` on
  malformed) and `ValidateCheckpoint` (→ `ErrCheckpointInvalid`). Closed-namespace and
  duplicate-key logic live in `internal/events/checkpoint_strict.go`; sentinel errors in
  `internal/errors/checkpoint_errors.go`.
* **CLI**: `internal/cli/checkpoint.go:54-119` `newCheckpointCreateCmd` — only
  `--state-dump` (required); help text currently *advertises* verbatim legacy write.
* **MCP**: `internal/mcp/tools.go:154-180` registration + `:1156-1171`
  `handleCreateCheckpoint` — only `state_dump`; description currently says a legacy dump
  "is written verbatim with no schema validation."
* **Docs**: `docs/cli-reference/backlogit_checkpoint_create.md`,
  `docs/design-docs/checkpoint-administrative-disposition.md`,
  `docs/design-docs/governed-operation-parity.md`.
* **Prior learnings** (`docs/compound/2026-09-04-bounded-error-and-cli-mcp-parity-patterns.md`):
  shared structured-error DTOs belong in `internal/errors` (the only cycle-free leaf, since
  CLI→MCP); a shared input-validation helper called by **both** surfaces before delegating
  keeps CLI/MCP behavior identical.
* **Governed fixtures** (`docs/compound/2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md`):
  a behavioral fixture for a `governed:true` operation must dispatch the **registered**
  handler (real CLI dispatch / in-process MCP), not the core function directly, and use
  separate `t.TempDir()` per surface to avoid second-precision filename collisions.

## Options Evaluated

### Option A: Default-reject in core, explicit `allowLegacyImport` opt-in threaded through all three surfaces

Invert the classification in `CreateCheckpoint`: require `schema_version == 1` by default;
reject anything else (missing/0/unsupported/malformed) with a typed error **before** the
write site. Add a single boolean opt-in (`allowLegacyImport`) to the core signature (via a
small options value to keep the governed signature stable and extensible). CLI adds a
`--allow-legacy-import` migration-only flag; MCP adds an optional `allow_legacy_import`
parameter. When the opt-in is set, a legacy-shaped dump (absent or integer-literal
`0` `schema_version`) is **upgraded to a canonical CheckpointV1 and re-validated —
never written verbatim**: the upgrade coerces `schema_version` to `1` and defaults
timestamps/status, and the record is written only if it then passes full V1
validation. Records that cannot be deterministically upgraded (missing required V1
identity such as `agent`/`session_id`, `consumer`-style legacy fields needing
remapping, or foreign top-level members that violate the closed namespace) are
**fail-closed rejected** even under the opt-in — as are unsupported/future versions,
wrong-typed/ambiguous `schema_version`, and malformed JSON.

* **Pros**: One authoritative behavior change in the shared core; CLI/MCP stay thin and in
  parity; opt-in is narrow and legible; preserves all existing guards; rejection stays
  pre-write so no-file is structurally guaranteed.
* **Cons**: Touches the governed core signature (contract change) → requires hardening and
  careful parity fixtures.
* **Effort**: Medium. **Fit**: Strong — matches every stated constraint.

### Option B: Reject only at the CLI/MCP surfaces; leave core permissive

Add validation in each surface handler before calling the unchanged core.

* **Pros**: Core signature untouched.
* **Cons**: Duplicated validation across two surfaces (drift risk); core still able to
  manufacture invalid files for any other caller; violates the "single authoritative
  behavior" and neutral-leaf learnings. Rejected.
* **Effort**: Low-Medium. **Fit**: Weak.

### Option C: Remove the legacy path entirely (no opt-in)

Always require `schema_version == 1`; delete the verbatim branch.

* **Pros**: Simplest possible semantics.
* **Cons**: Violates the explicit requirement to keep legacy import possible for migration;
  removes a migration capability with no replacement. Rejected.
* **Effort**: Low. **Fit**: Fails a hard requirement.

## Trade-off Comparison

| Criterion | Option A | Option B | Option C |
|---|---|---|---|
| Single authoritative behavior | strong | weak (duplicated) | strong |
| CLI/MCP parity risk | low | high (drift) | low |
| Preserves migration capability | yes (opt-in) | yes | no |
| Blast radius of core change | medium (governed sig) | low | medium |
| Aligns with compound learnings | yes | no | partial |

## Decision

**Adopt Option A.** Make `events.CreateCheckpoint` reject non-V1 dumps by default with
pre-write typed errors, and gate the legacy path behind an explicit `allowLegacyImport`
opt-in surfaced identically on CLI (`--allow-legacy-import`) and MCP (`allow_legacy_import`).
Keep the opt-in narrow (legacy = an absent `schema_version` member or exactly one
integer-literal `0` only). Legacy import is **upgrade-or-reject**: a legacy-shaped dump is
coerced to `schema_version:1`, has defaults populated, and is run through full V1
validation; it is written **only if it validates**, otherwise rejected with no file. Legacy
import never writes an invalid record verbatim. Preserve size/secret/duplicate-key/closed-
namespace guards.

### Covering feature

This is task-shaped (a single bug) but spans five surfaces and exceeds one 2-hour task, so
it is harvested under a synthesized covering **feature**: *"Checkpoint create rejects
schema-less dumps by default; legacy import requires explicit opt-in."* Work is decomposed
into five width-isolated, dependent tasks: (1) declarations/harness, (2) core reject-by-
default behavior, (3) CLI flag, (4) MCP parameter/schema, (5) documentation/compatibility.

### Explicit semantics matrix

| `schema_version` state | Default (no opt-in) | With `allow_legacy_import` (migration only) |
|---|---|---|
| `1` (valid V1) | Accept via existing V1 path (parse → closed-namespace → dup-key → defaults → validate → canonical write) | Same (opt-in has no effect on valid V1) |
| Missing (absent) | **REJECT** — typed schema-rejection error, **no file written** | **Upgrade → validate → write**: coerce to `schema_version:1`, populate defaults, run full V1 validation; write canonical V1 only if it validates, else **REJECT, no file** |
| `0` (zero) | **REJECT** — no file | **Upgrade → validate → write** (same as missing); else **REJECT, no file** |
| Unsupported/future (e.g. `2`, negative) | **REJECT** — no file | **REJECT** — no file (opt-in is scoped to legacy shapes, not future/unknown versions) |
| Wrong-typed / ambiguous `schema_version` (string `"0"`, `null`, non-integral, out-of-range, or **duplicate** `schema_version` members) | **REJECT** — no file | **REJECT** — no file (only an absent member or exactly one integer-literal `0` counts as a legacy shape) |
| Malformed JSON | **REJECT** — malformed-input error, no file | **REJECT** — no file (malformed is never importable) |

Preserved unconditionally (both default and opt-in paths, before the write site):
input **size limit** and **secret scanning**. The **duplicate-key** and
**closed-namespace** structural checks continue to apply exactly as today on the V1 path
(and now also on the upgraded legacy-import path, since it runs the same V1 validation).

**Legacy import is upgrade-or-reject, never verbatim.** A successful legacy import always
produces a record that `GetCheckpoint`/`ListCheckpoints` can read without quarantine — the
opt-in migrates a legacy-shaped dump into a valid CheckpointV1 rather than writing an
invalid record behind a flag. This closes the quarantine-poisoning vector that a verbatim
opt-in would have reintroduced.

## Rejected Alternatives

* **Option B** (surface-only validation): duplicated logic and core remains able to
  manufacture invalid records for any caller — contradicts the neutral-leaf/shared-helper
  learnings.
* **Option C** (drop legacy entirely): removes a required migration capability.

## Unresolved Questions

* Error taxonomy: whether default rejection reuses `ErrCheckpointInvalid` or introduces a
  dedicated `create`-time sentinel (e.g. `ErrCheckpointSchemaRejected`) for a clearer
  operator message. **Recommendation carried into planning**: introduce a dedicated
  create-time sentinel in `internal/errors/checkpoint_errors.go` so the CLI/MCP message can
  point the operator at `--allow-legacy-import`; reuse `CheckpointMalformedInputError` for
  malformed JSON. Final choice is a plan-level detail with no scope impact.

## Risks and Mitigations

* **Contract/behavior change** (default now rejects what previously succeeded): mitigate by
  keeping the opt-in as the documented migration escape hatch and by explicit doc updates on
  both surfaces. This is the crux hardened in the plan.
* **CLI/MCP parity drift**: mitigate by threading a single core option and using
  authoritative-registry-dispatching parity fixtures (compound 2026-08-15).
* **Accidental no-file regression**: mitigate by keeping all classification before the
  atomic write site and reusing `assertNoCheckpointWritten` +
  `checkpoint_writesite_test.go` static guard.

## Promotion

Promoted to **plan**: `docs/exec-plans/2026-09-06-checkpoint-create-schema-reject-plan.md`.

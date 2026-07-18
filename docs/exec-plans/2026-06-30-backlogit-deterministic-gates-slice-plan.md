---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "backlogit Deterministic-Gates slice — implementation plan"
description: "Doctor target-mode, per-task locking, task size schema, and body-preserving size mutation CLI (+ MCP parity)"
source: "docs/decisions/2026-06-30-backlogit-deterministic-gates-slice-deliberation.md"
doc_type: "plan"
ingested_at: "2026-06-30T00:00:00Z"
design_doc: "docs/design-docs/autoharness-evals-gates-design.md"
stash_id: "AE0838A9"
slug: "backlogit-deterministic-gates-slice"
date: "2026-06-30"
tags:
  - deterministic-gates
  - doctor
  - concurrency
  - header-def
  - mdfront
  - cli-mcp-parity
---

# Implementation Plan — backlogit Deterministic-Gates slice

## Problem Frame

Implement the **backlogit-owned** slice of the Deterministic Gates initiative
(design `docs/design-docs/autoharness-evals-gates-design.md` §3–§6). backlogit is
the `.backlogit/` work-state authority (design §2). autoharness consumes two
backlogit CLI contracts:

* `pre_task_completion` gate → `backlogit doctor --target {file_path}` (5s timeout, exit-code gated)
* `pre_execution` sizing hook → `backlogit update {task_id} --size {value}` (body-preserving)

Grounded code locations (verified):

* Doctor CLI: `internal/cli/doctor.go:13-97` (flags; no `--target`).
* Doctor core: `internal/core/doctor.go:125-387`; canonical scan
  `internal/core/canonical_scan.go:30-115`; search dirs
  `internal/core/artifacts.go:553-598` (already `.backlogit/`-scoped, no `docs/`).
* Header-def loader/validation: `internal/config/headerdef.go:43-82`,
  `internal/core/field_validation.go:11-34`; task type at
  `.backlogit/header-def.yaml:51-63` (only `priority` today).
* Body-preserving codec: `internal/mdfront/codec.go` (`Markdown`, `Decode`, `Encode`);
  atomic write helper `internal/atomicfile`.
* Update CLI/core: `internal/cli/update.go:20-212`; `core.UpdateArtifact`
  (`internal/core/artifacts.go:434-543`) → `persistArtifact` → `WriteArtifactFile`
  (`internal/core/artifacts.go:637-690`, currently a full-file rebuild — NOT body-preserving).
* Lock precedent: `internal/core/stash_lock.go:11-61` (`.lock` sidecar).
* MCP tool handlers: `internal/mcp/tools.go` (parity surface).
* Registry op mapping: `.autoharness/backlog-registry.yaml` (`doctor`, `update_task`).
* Telemetry (unaffected): `internal/db/telemetry_schema.go`.

## Requirements Trace

| Source requirement | Implementation action | Unit(s) |
|---|---|---|
| Item 1a: doctor validates only `.backlogit/` against header-def | Characterization/guard test on `artifactSearchDirs` scope; header-def validation reused in target mode | U1, U2 |
| Item 1b: `doctor --target {file}` single-file, 5s timeout | New core single-file validator + CLI flag with `context.WithTimeout(5s)` + exit-code contract | U1, U2 |
| Item 1 MCP parity | `backlogit_doctor` gains `target` param + registry op | U3 |
| Item 2: per-task lock during validation | Advisory `.lock` sidecar primitive (stale-TTL); acquired around target validation | U4, U5 |
| Item 3: header-def `size` (T-shirt) attribute | Optional `size` enum field on `task` type in header-def.yaml + default template | U6 |
| Item 4: `update --size` body-preserving | Dedicated body-preserving `core.SetArtifactSize` (mdfront + atomicfile + db.UpsertItem, enum-validated, lock-guarded) (U7); `--size` CLI flag routed to it (U8) | U7, U8 |
| Item 4 MCP parity | `backlogit_update_item` gains `size` param + registry op | U9 |
| Q1: gate-failure escape hatch | Document existing `move --status blocked/queued`; no new code (folded into U2 help text/docs) | U2 |
| Q3: telemetry DB location | No backlogit change (no unit) | — |

## Implementation Units

Granularity: each unit targets ≤2 source files (+ tests), a single skill domain,
and one verifiable milestone. Execution posture is **test-first** for all code
units unless noted. Suggested T-shirt sizes are advisory (the `size` schema they
introduce cannot be applied to these tasks until U6 ships).

### U1 — core: single-file header-def validation for doctor target mode  [test-first · S]

* **Change**: Add `core.DoctorTarget(ws, filePath) (DoctorTargetResult, error)`
  (new `internal/core/doctor_target.go`, returns a **concrete result struct**)
  that: (a) confines the path to the workspace storage root by **reusing the
  existing scope authority** — resolve against `WorkspaceStorageRoot(ws.RootPath)`
  / the same directory set as `artifactSearchDirs` (`artifacts.go:553-598`), NOT a
  bespoke prefix string check (single source of truth for invariant #2);
  (b) `mdfront.Decode`s the file and builds the artifact via
  `models.ArtifactFromFrontmatter`; (c) resolves the artifact type and runs
  `ValidateArtifactFields` via `ResolveFieldSchema`/`LoadHeaderDef` for
  **required-field presence** (existing semantics — this unit does NOT add global
  enum enforcement, which would regress legacy artifacts); (d) returns a typed
  pass/fail carrying field-level errors and a classified error kind
  (`scope`/`io`/`validation`) for the exit-code mapping in U2. `ctx`-less at the
  core layer; the timeout is applied by the caller (U2).
* **Files**: `internal/core/doctor_target.go`, `internal/core/doctor_target_test.go`.
* **Tests (≤3 scenarios)**: valid `.backlogit/` task → pass; a task missing a
  required field → validation-fail with the field name; **scope-guard** — a path
  outside the workspace storage root (incl. `../` traversal and a `docs/` path) is
  rejected with the `scope` error kind, and the guard is derived from
  `WorkspaceStorageRoot`/`artifactSearchDirs` (locks in item 1a's boundary).
* **Milestone**: `go test ./internal/core/ -run DoctorTarget` green.
* **Runtime surface**: none directly (library); feeds the CLI.

### U2 — CLI: `backlogit doctor --target {file}` flag + correct 5s timeout + versioned exit-code contract  [test-first · M · dep: U1]

* **Change**: Register `--target string` on the doctor cobra command
  (`internal/cli/doctor.go`). When set, run `core.DoctorTarget` in a goroutine and
  `select` on `ctx.Done()` (from `context.WithTimeout(ctx, 5*time.Second)`) so the
  deadline is actually enforced — a plain wrapper does NOT interrupt synchronous
  I/O (cooperative-context finding). Use a **buffered** result channel (cap 1) for
  the goroutine so that on timeout (exit 2) the still-running goroutine can send and
  exit rather than leaking blocked on the channel. Note the timeout↔lock
  interaction: on exit 2 the orphaned goroutine may still hold the U5 per-task
  sidecar, which is then reclaimed by the 60s stale-TTL (invariant #5). Map the
  outcome to an **explicit, versioned exit-code table**, set via `SilenceErrors` +
  an explicit `os.Exit`/returned code (Cobra `RunE` otherwise collapses to 1):

  | Code | Meaning |
  |---|---|
  | 0 | pass |
  | 1 | validation fail (field errors) |
  | 2 | timeout (`errors.Is(ctx.Err(), context.DeadlineExceeded)`) |
  | 3 | scope / IO / decode error (path outside storage root, unreadable) |
  | 4 | busy (task lock held — see U5) |

  Honor existing `--format json` but emit a **versioned, target-mode-specific JSON
  schema** (a `mode: "target"` discriminator + stable field names) so a downstream
  parser is not surprised by the full-scan `DoctorReport` shape. Document in help
  text: the exit-code table, that autoharness's subprocess `timeout_seconds: 5`
  (design §5) is the authoritative outer bound, and the **Q1 escape hatch**
  (`backlogit move {id} --status blocked` / `--status queued` after repeated gate
  failure — an existing transition, no new command).
* **Files**: `internal/cli/doctor.go`, `internal/cli/doctor_target_test.go`.
* **Tests (≤4)**: valid file → exit 0; missing-required-field → exit 1 with field
  errors; **timeout** → exit 2 (inject a near-zero deadline against a blocked/slow
  stub so the goroutine+select path is exercised non-vacuously); out-of-scope path
  → exit 3.
* **Milestone**: CLI test green; `backlogit doctor --target .backlogit/queue/<file>`
  returns 0 within the 5s bound; `--format json` emits the versioned target schema.
* **Runtime surface**: CLI (autoharness gate consumer). Verify the exit-code table
  and the 5s bound are honored deterministically.

### U3 — MCP parity: `backlogit_doctor` `target` param + registry op mapping  [test-first · S · dep: U1 · deferrable]

* **Change**: Add a `target` parameter to the `backlogit_doctor` MCP tool handler
  in `internal/mcp/tools.go`, routing to the same `core.DoctorTarget`; add the
  `target` param to the `doctor` op in `.autoharness/backlog-registry.yaml`. The
  MCP handler only threads the param into the shared core function (near-free once
  U1 exists); prevents the documented CLI/MCP drift class. **Scope note:** the
  design specifies CLI-only contracts — this unit is the explicit deferrable if
  the operator wants to trim to strictly the four items.
* **Files**: `internal/mcp/tools.go` (+ its test), `.autoharness/backlog-registry.yaml`.
* **Tests (≤2)**: MCP `backlogit_doctor` with `target` returns a pass result on a
  valid file; returns a fail result with the classified error kind on an invalid
  file. (MCP returns a structured result, not a process exit code.)
* **Milestone**: MCP tool test green; `backlogit manifest` lists the `target` param.
* **Runtime surface**: MCP tool.

### U4 — core: per-task `.lock` sidecar primitive (per-path advisory + stale TTL)  [test-first · S]

* **Change**: Add `internal/core/task_lock.go` implementing
  `lockTaskFile(taskFilePath) (unlock func() error, err error)` reusing the
  crash-safe sidecar pattern from `advisory-file-lock-stale-ttl-go-2026-04-08.md`,
  but **per-path** rather than a single package-global mutex: a registry-guarded
  `map[string]*sync.Mutex` keyed by the resolved task path (so task A does not
  serialize task B), plus the on-disk `O_CREATE|O_EXCL` sidecar `.<name>.lock`
  for cross-process safety, a stale-TTL (60s) recovery with a single retry, and a
  WARN on stale removal. On a live sidecar, return a distinct **busy** error
  (do not block indefinitely). Callers MUST `defer` the returned `unlock` so every
  error path releases both the sidecar and the mutex; discard the warn-only unlock
  error (`defer func(){ _ = unlock() }()`). Sidecars are ephemeral/gitignored
  (`.*.lock`), naming per `concurrency.instructions.md`. **Long-lived-process note**:
  in the MCP server the per-path mutex map is never pruned (bounded by distinct task
  paths — acceptable for the CLI-subprocess gate; document it). Cross-process busy
  (live sidecar) returns the distinct busy error; genuine non-blocking in-process
  busy, if ever required, would use `Mutex.TryLock`.
* **Files**: `internal/core/task_lock.go`, `internal/core/task_lock_test.go`.
* **Tests (≤3)**: acquire→release round-trip; **busy path is observable** by
  pre-creating a fresh sidecar → `lockTaskFile` returns the busy error (does not
  block); a manually-aged stale sidecar is recovered after one WARN.
* **Milestone**: `go test ./internal/core/ -run TaskLock` green.
* **Runtime surface**: none directly.

### U5 — core: acquire the per-task lock in doctor `--target`; define busy semantics  [test-first · S · dep: U4, U1]

* **Change**: In `core.DoctorTarget`, acquire the per-task lock (U4) before reading
  the target file and `defer`-release it, so a concurrent mutation cannot modify a
  task while it is undergoing validation (item 2). Lock acquisition is
  **non-blocking**: if the task lock is already held, return the `busy` result
  kind → U2 maps it to exit code 4 (this closes the lock↔timeout↔exit-code hole in
  the gate contract). Do not hold the lock across unrelated I/O.
* **Files**: `internal/core/doctor_target.go`, `internal/core/doctor_target_lock_test.go`.
* **Tests (≤2)**: validation acquires+releases the lock (sidecar present during,
  gone after); when the sidecar is pre-held, target validation returns `busy`
  (not a mid-write read, not a block).
* **Milestone**: lock-integration test green; busy → exit 4 asserted end-to-end via U2.
* **Runtime surface**: none directly (defines U2's busy exit path).

### U6 — config: add optional `size` (T-shirt) field to `header-def.yaml` task schema  [config · XS]

* **Change**: Add an optional `size` enum field (`XS, S, M, L, XL`) to the `task`
  type in `.backlogit/header-def.yaml` (and the embedded default header-def
  template, if one exists, so `init` emits it). Optional, **no default**, so
  existing tasks without `size` stay valid. `size` is a **logical** task field
  whose value is **physically stored under `custom_fields.size`** — consistent
  with how `severity`/`harness_status` are stored and read
  (`ArtifactFromFrontmatter` only round-trips known keys + the nested
  `custom_fields` map; a top-level `size` would be silently dropped).
  `artifactFieldValue`'s default case already reads `CustomFields["size"]`.
* **Files**: `.backlogit/header-def.yaml` (+ embedded default template if present).
* **Tests (≤3)**: `ResolveFieldSchema("task")` exposes the `size` enum with the
  five values; an artifact carrying `custom_fields.size: M` round-trips through
  `ArtifactFromFrontmatter`→`WriteArtifactFile`→re-parse; an artifact with **no**
  `size` still passes `ValidateArtifactFields` (backward-compat). (Pure
  schema/model unit — a true root; no `--target` dependency.)
* **Milestone**: `go test ./internal/config/ ./internal/models/` green; full
  `backlogit doctor` clean (no regression across existing 645 indexed artifacts).
* **Runtime surface**: schema/config; verify no existing-artifact regressions.

### U7 — core: dedicated body-preserving `core.SetArtifactSize` (single seam)  [test-first · M · dep: U4, U6]

* **Change**: Add `core.SetArtifactSize(ctx, ws, id, size)` (new
  `internal/core/artifact_size.go`) as the **single** body-preserving field
  mutation seam for `size`. It: (a) **enum-validates** `size ∈ FieldDef.Values`
  for the task schema (a targeted check — do NOT retrofit global enum enforcement
  into `ValidateArtifactFields`, which would regress legacy artifacts); (b)
  acquires the per-task lock (U4) and `defer`-releases it; (c) reads the on-disk
  file and `mdfront.Decode`s it; (d) sets `custom_fields.size` in the decoded
  frontmatter map, leaving all other frontmatter and the **entire body bytes
  untouched**; (e) `mdfront.Encode` → **`atomicfile.WriteFileAtomic`** (already
  short-write-guarded + Windows-rename-safe — do not reimplement); (f) keeps the
  SQLite index in sync via **`db.UpsertItem(ctx, db, artifact)`**. **Critical**:
  `UpsertItem` executes `INSERT OR REPLACE` on the **full item row**, so it MUST
  be handed a **fully-populated `*models.Artifact`** — reconstructed from the
  *same* decoded frontmatter+body via `models.ArtifactFromFrontmatter` (or
  `findArtifact`) with `custom_fields.size` set — NOT a partial `{ID, CustomFields}`
  stub, which would null out `title`/`status`/`priority`/etc. in the index and
  silently re-open the markdown↔DB drift class. Derive both representations from
  one decode so the file and the index cannot diverge (the DB has always stored
  only the modeled subset, so no new drift is introduced). The generic
  `UpdateArtifact`/`WriteArtifactFile` rebuild path is left unchanged (bounded
  blast radius; no interim non-body-preserving `--size`).
* **Files**: `internal/core/artifact_size.go`, `internal/core/artifact_size_test.go`
  (+ a golden fixture generated through the codec).
* **Tests (≤4)**: valid `size` persists to `custom_fields.size` and the DB row
  reflects it **while `title`/`status`/`priority` index columns are unchanged**
  (guards the full-row-REPLACE reconstruction — a partial-artifact upsert MUST
  fail this); invalid value (`XXL`) rejected before any write; **golden** — after
  the mutation, **body bytes are byte-identical** and the frontmatter is
  **semantically equal** to the original plus `size` (parse+compare maps, not a raw
  line diff); **idempotency** — re-applying the same `size` yields no body change
  and a stable frontmatter map; the mutation acquires+releases the per-task lock.
* **Milestone**: `go test ./internal/core/ -run ArtifactSize` green (body-byte +
  idempotency + full-row-preservation DB-sync assertions).
* **Runtime surface**: core write path; the golden test is the anti-corruption gate.
* **Hook-event note**: `SetArtifactSize` intentionally bypasses the generic
  `HookUpdateArtifact` chain, so size-only mutations emit **no** `emit_hook_event`
  mutation event. This is acceptable: the only pre-hook (`ValidateStatusTransition`)
  is a no-op when `status` is unchanged, and post-hooks are best-effort. Documented
  as intentional (see Decisions); revisit only if autoharness needs a size-change
  event.

### U8 — CLI: `backlogit update --size {value}` flag → `SetArtifactSize`  [test-first · S · dep: U7]

* **Change**: Register `--size string` on `internal/cli/update.go`. When set, route
  to `core.SetArtifactSize` (NOT the generic `updates` map — a bare `updates["size"]`
  is dropped by `UpdateArtifact`'s type-switch). **`--size` is mutually exclusive**
  with other frontmatter-mutating flags (`--status`/`--priority`/etc.): if combined,
  error out **before** any write, because those flags route through the generic
  `UpdateArtifact`→`WriteArtifactFile` **rebuild** path, and running both in one
  invocation would double-write and negate body preservation. Keep behavior
  single-purpose: `--size` performs the body-preserving size mutation only. On a
  **busy** task lock (a concurrent `doctor --target`/mutation holds it), surface the
  **same non-zero busy exit code as the doctor table (4)** — do not block — so the
  autoharness sizing hook sees deterministic behavior under contention. Emit a clear
  validation error message for out-of-enum values.
* **Files**: `internal/cli/update.go`, `internal/cli/update_size_test.go`.
* **Tests (≤3)**: `backlogit update {id} --size M` persists and `backlogit get {id}`
  shows `size: M` with the on-disk body unchanged (end-to-end body-preservation
  smoke); `--size XXL` exits non-zero with an enum error; `--size M --status done`
  (combined) errors before writing (mutual-exclusion guard).
* **Milestone**: CLI test green; live dogfood on a scratch task shows `size` set
  with body preserved.
* **Runtime surface**: CLI (autoharness sizing-hook consumer). Busy → exit 4
  matches the doctor contract.

### U9 — MCP parity: `backlogit_update_item` `size` param + registry op mapping  [test-first · S · dep: U7 · deferrable]

* **Change**: Add a `size` parameter to the `backlogit_update_item` MCP tool
  handler (`internal/mcp/tools.go`) routing to the **same** `core.SetArtifactSize`
  seam (validated, body-preserving, DB-synced); add `size` to the `update_task` op
  params in `.autoharness/backlog-registry.yaml`. Near-free once U7 exists;
  prevents CLI/MCP drift. **Scope note:** deferrable alongside U3 if trimming to
  strictly the four items.
* **Files**: `internal/mcp/tools.go` (+ its test), `.autoharness/backlog-registry.yaml`.
* **Tests (≤2)**: MCP `backlogit_update_item` with `size` persists (body preserved,
  DB synced); invalid `size` is rejected.
* **Milestone**: MCP tool test green; `backlogit manifest` lists the `size` param.
* **Runtime surface**: MCP tool.

## Dependency Graph

```text
Roots:  U1            U4            U6
        │ │           │             │
        │ └▶U3        ├────▶U5◀── U1 │
        ▼             │             │
        U2◀── U5      └────▶U7◀─────┘
                            │
                            ├─▶U8
                            └─▶U9
```

Edges (authoritative — prose governs, the diagram is illustrative):
* U1 → U2, U3, U5
* U4 → U5, U7
* U6 → U7   (U6 does NOT feed U5)
* U7 → U8, U9

* Roots (parallelizable): **U1, U4, U6**.
* Wave 2: **U2** (◀U1), **U3** (◀U1), **U5** (◀U1,U4), **U7** (◀U4,U6).
* Wave 3: **U8** (◀U7), **U9** (◀U7).
* Suggested serial order for Ship: **U1 → U4 → U5 → U2 → U3 → U6 → U7 → U8 → U9**
  (U5 before U2 so the busy→exit-4 path exists when U2 wires exit codes).
* No cycles. Deferrable-if-trimming: **U3, U9** (MCP parity).

## Decisions and Rationale

* **Reuse header-def presence validation + workspace scope authority for target
  mode (B1)** — one validation path (`ValidateArtifactFields`), one scope
  definition (`WorkspaceStorageRoot`/`artifactSearchDirs`); no duplication, no new
  source of truth for the `.backlogit/` boundary.
* **`size` stored under `custom_fields` (not a top-level key)** — the model only
  round-trips known keys + the nested `custom_fields` map; this mirrors
  `severity`/`harness_status` and is what `artifactFieldValue` reads. A top-level
  `size` would be silently dropped on re-parse.
* **One body-preserving seam: `core.SetArtifactSize` (A1)** — a dedicated,
  intentional mutation using `mdfront.Decode/Encode` + `atomicfile.WriteFileAtomic`
  + `db.UpsertItem`, enum-validated and lock-guarded. It does **not** modify the
  generic `UpdateArtifact`/`WriteArtifactFile` rebuild path (bounded blast radius)
  and it does **not** bypass the DB index (no markdown↔DB drift). Body bytes are
  preserved byte-for-byte; frontmatter is re-marshaled as valid YAML (semantic
  equality asserted, not raw-line equality — the 068-S golden pattern).
* **Enum validation is targeted to the written value, not global** — checking
  `size ∈ FieldDef.Values` at the mutation entry point avoids retrofitting enum
  enforcement onto `ValidateArtifactFields` (which would suddenly reject legacy
  artifacts with out-of-enum values elsewhere). Prefer reusing a shared
  value-membership helper (e.g., extend `core.ValidateFields`/a `FieldDef`
  membership method) rather than hand-rolling a third enum path, so there is one
  enum-membership authority.
* **Size mutations emit no hook event (intentional)** — `SetArtifactSize` bypasses
  the generic `HookUpdateArtifact` chain (its only pre-hook is a status-transition
  validator that no-ops when `status` is unchanged; post-hooks are best-effort).
  Size-only edits therefore produce no `emit_hook_event` mutation event. Accepted
  for now; revisit only if autoharness requires a size-change signal.
* **Correct 5s timeout via goroutine + `select(ctx.Done())`** — a bare
  `context.WithTimeout` cannot interrupt synchronous I/O; autoharness's subprocess
  `timeout_seconds: 5` (design §5) is the authoritative outer bound.
* **Versioned exit-code table + target-mode JSON schema** — the CLI is a cross-repo
  gate contract; codes (0/1/2/3/4) and JSON shape are pinned and set explicitly
  (Cobra `RunE` otherwise collapses to exit 1).
* **Per-path lock (map[path]*sync.Mutex) + sidecar + stale TTL (C1)** — genuine
  per-task granularity, crash-safe, Windows-safe; busy is non-blocking → exit 4.
* **CLI/MCP parity kept but flagged deferrable (U3/U9)** — near-free via the shared
  `core` seam; prevents the documented drift class; the design's consumer is
  CLI-only, so these are the explicit trim points.
* **Q1 uses existing `move` transitions; no new command** — correct ownership
  boundary (autoharness owns retry policy); documented, not built.
* **Q3 no telemetry change** — metrics DB stays in `.autoharness/metrics`.

## Risks and Caveats

* **Queue-file corruption** on write → single `SetArtifactSize` seam with
  `mdfront` + `atomicfile.WriteFileAtomic` + golden body-byte-equality/idempotency
  tests + per-task lock + `db.UpsertItem` (U7).
* **markdown↔DB drift** → `SetArtifactSize` always `db.UpsertItem`s; never a
  markdown-only write.
* **Cross-repo CLI contract** (flag names / exit-code table / 5s timeout / JSON
  schema consumed by autoharness) → pinned and versioned (U2); a change breaks the
  external gate.
* **Path-traversal on `--target`** → scope guard derived from the workspace storage
  root (U1).
* **Stale lock after crash** → 60s stale-TTL recovery (U4).
* **CLI/MCP drift** → explicit parity units (U3, U9).
* **T-shirt vocabulary churn** → fix the enum in header-def (U6) as the single
  source of truth; autoharness must emit a member of that set.

## Plan Hardening Signals (REQUIRED)

* **Public API / schema / contract change** — **PRESENT.** New `header-def.yaml`
  `size` field (task frontmatter contract); new CLI flags `doctor --target`,
  `update --size`; new MCP params; **cross-repo CLI contract** (exit codes, flag
  names, 5s timeout) consumed by autoharness.
* **Security / auth / permission / compliance** — **MINOR.** `doctor --target`
  accepts an arbitrary file path → path-traversal consideration (mitigated by the
  `.backlogit/` scope guard, U1). No auth/secret surface.
* **Migration / backfill / destructive / irreversible** — **PRESENT.** The
  `--size` mutation writes into existing queue markdown files; an incorrect write
  can irreversibly corrupt frontmatter/body. Mitigated by the single
  `core.SetArtifactSize` seam (mdfront body-preserving codec +
  `atomicfile.WriteFileAtomic` + golden body-byte-equality/idempotency test +
  per-task lock + `db.UpsertItem`, U7). `size` is additive and optional — no
  backfill of existing tasks required.
* **External integration / operator checkpoint / external dependency** —
  **PRESENT.** autoharness (separate repo) depends on both CLI contracts.
* **High runtime / rollout / rollback risk** — **MODERATE.** 5s timeout behavior,
  concurrent locking, and body-preserving writes carry runtime-correctness risk;
  all are covered by tests and are individually revertible (additive, feature-flagged
  by presence of the new flags/params).

**Requires plan hardening: yes**

## Runtime Verification and Closure

| Unit | Runtime surface | Verify before absorbed | Closure artifact |
|---|---|---|---|
| U2 | CLI `doctor --target` | exit-code table (0/1/2/3/4) honored; timeout→2 via goroutine+select; ≤5s wall time; versioned target JSON | contract note in command help + plan |
| U3 | MCP `backlogit_doctor` | `target` param in manifest; result parity with CLI | manifest check |
| U5 | lock during validation | sidecar created/removed; held→busy→exit 4 (non-blocking) | test evidence |
| U7 | core `SetArtifactSize` | enum-validated; body bytes byte-identical; frontmatter semantically equal; DB row synced; idempotent | golden fixture + closure note |
| U8 | CLI `update --size` | routes to `SetArtifactSize`; `get` shows size; body unchanged E2E | test evidence |
| U9 | MCP `backlogit_update_item` | `size` param in manifest; parity with CLI | manifest check |
| U6 | schema/model | `size` round-trips via `custom_fields`; no regression on existing tasks (`backlogit doctor` clean) | doctor run evidence |

Rollback: each unit is additive (new flag/param/field/file). Revert = remove the
flag/param and the `size` enum line; no data migration to unwind (existing tasks
never required `size`). Owner: backlogit maintainers. Validation window: one
green CI run + one live dogfood (`doctor --target` on a real queue file, `update
--size` on a scratch task with golden check).

## Plan Hardening

**Hardening required: YES.** Triggered by (a) a public/cross-repo **contract**
change (CLI flags, exit codes, 5s timeout, header-def schema, MCP params consumed
by autoharness), and (b) a **destructive-potential** write path that mutates
existing `.backlogit/` queue markdown in place.

### Learnings and instructions consulted

* `docs/compound/2026-06-28-codec-extraction-leaf-packages.md` (068-S) — body-preserving
  `mdfront` codec + `internal/atomicfile` + **differential golden byte-equality**
  and **idempotency** tests as the proof obligation for "nothing else changed".
* `docs/compound/best-practices/advisory-file-lock-stale-ttl-go-2026-04-08.md` —
  `sync.Mutex` + `O_CREATE|O_EXCL` sidecar + 60s stale-TTL + single retry; Windows
  PID-liveness is unreliable → use TTL, not PID.
* `docs/compound/best-practices/go-file-write-short-write-guard-2026-04-23.md` and
  `windows-safe-atomic-rename-goos-gate-2026-04-23.md` — short-write guard and
  GOOS-gated atomic rename for the write path.
* `docs/compound/2026-05-07-mcp-cli-config-parity.md` — CLI/MCP parity is a P2
  review-finding class (drives U3, U9).
* `.github/instructions/concurrency.instructions.md` — `.<filename>.lock`,
  ephemeral, gitignored, do not force-break, warn on locks older than ~1h.
* `.github/instructions/strict-safety.instructions.md` — risky-action classification below.

### Protected invariants

1. **Body-byte preservation**: any `--size` write MUST leave the markdown **body
   bytes byte-identical**; non-`size` frontmatter MUST be **semantically equal**
   (parse+compare — `mdfront.Encode` re-marshals the frontmatter block via yaml.v3,
   so raw-line identity is not guaranteed, but no key/value is lost or changed
   except `size`). Asserted by the U7 golden test (`body_bytes_changed: false` +
   frontmatter map compare).
2. **Scope confinement**: `doctor` (full and `--target`) MUST never validate or
   read outside `.backlogit/`.
3. **Backward compatibility**: existing tasks without `size` MUST remain valid;
   `size` is optional with no default.
4. **Deterministic exit contract**: `doctor --target` exit codes are a stable
   external contract (0 pass / non-zero fail / distinct non-zero timeout).
5. **No lock deadlock/orphan**: locks release on every path; stale locks recover
   within the TTL window.

### Risky actions (ProposedAction / ActionRisk / ActionResult)

* **PA-1 — In-place frontmatter mutation of existing queue files (`--size` write, U7).**
  * `ActionRisk`: **HIGH** (irreversible corruption of live work-item files if the
    codec drops body bytes, reorders/loses comments, or a partial write occurs).
  * Approval: not gated (covered by tests), but MUST NOT merge without the golden
    byte-equality + idempotency tests green.
  * `ActionResult` (expected): only the `size` frontmatter key added/changed;
    body and other frontmatter byte-identical; write is atomic (temp + rename);
    short-write guarded.
  * Mitigations: `mdfront.Decode`/`Encode` (no model rebuild) + `internal/atomicfile`
    + golden fixture + idempotency test + per-task lock held during the write.

* **PA-2 — Editing `.backlogit/header-def.yaml` (schema contract, U6).**
  * `ActionRisk`: **MEDIUM** (a malformed edit or a non-optional `size` would
    invalidate every existing task and break `doctor`).
  * `ActionResult` (expected): `size` added as **optional** enum (`XS,S,M,L,XL`),
    no default; `backlogit doctor` stays clean across the existing 645 indexed
    artifacts.
  * Mitigations: optional field; full `doctor` regression run in U6 milestone
    before dependents proceed.

* **PA-3 — Reading an operator-supplied path in `doctor --target` (U1/U2).**
  * `ActionRisk`: **LOW-MEDIUM** (path traversal / reading outside workspace).
  * `ActionResult` (expected): paths resolved and confined to `.backlogit/`;
    out-of-scope paths rejected with a clear error, non-zero exit.
  * Mitigations: scope-guard test (U1) asserting rejection of `../` and non-`.backlogit` paths.

* **PA-4 — Concurrent lock acquisition / stale-lock removal (U4/U5).**
  * `ActionRisk`: **MEDIUM** (a wrongly-removed live lock re-introduces the race).
  * `ActionResult` (expected): only sidecars older than the 60s TTL are removed,
    with a single retry and a WARN; live locks are respected; agents never
    force-break a lock they did not create.
  * Mitigations: stale-TTL pattern from the advisory-lock learning; test with a
    manually-aged sidecar.

### Deepened verification

* **Reuse `atomicfile.WriteFileAtomic`** for the U7 write (it already implements
  the short-write guard via `writeAll` and the Windows-safe rename) — do NOT
  reimplement a temp+rename in the size path; a second divergent atomic writer is
  the anti-pattern.
* U6 milestone MUST run a full `backlogit doctor` (not just `--target`) to prove
  zero regression on existing artifacts before U7/U9 build on the schema.
* U2 must assert the **timeout branch** deterministically against a blocked/slow
  stub (goroutine+select, near-zero deadline) so the 5s contract is tested
  non-vacuously — a bare `context.WithTimeout` would pass the test without
  actually interrupting synchronous work.
* Parity (U3/U9): assert the new param appears in `backlogit manifest` output so
  MCP/CLI drift is caught mechanically.

### Rollback / monitoring / ownership

* **Rollback**: additive and independently revertible per unit (remove flag/param/
  `size` enum line). No backfill to unwind. If PA-1 regresses in the field, revert
  U7/U8 (removes `--size`/`SetArtifactSize`) — existing files are unaffected
  because `size` is optional.
* **Monitoring signal**: post-merge, `backlogit doctor` clean and
  `doctor --check-archived-from` at 0 self-referential (the 068-S body-preservation
  canary) confirm no corruption from the new write path.
* **Owner**: backlogit maintainers. **Validation window**: one green CI run + one
  live dogfood per U2/U7 milestones before the shipment is marked done by Ship.

### Unresolved operator decisions (non-blocking for harvest)

* Confirm the T-shirt enum membership (`XS,S,M,L,XL`) — plan assumes this set;
  Ship/impl can adjust the U6 enum without reshaping the hierarchy.
* Optional: whether `doctor --target` should also emit `--format json` for the
  gate (CLI already supports `--format`; reuse, no new contract).

<!-- plan-review-attempt: 2 -->

## Plan Review

### Attempt 1 — Gate: FAIL

Multi-persona review (Scope Boundary Auditor, Go Reviewer, Architecture
Strategist). Multiple convergent **P1** findings on the persistence/model/
validation seam force a FAIL and a plan revision before harvest.

**P1 — model/round-trip reconciliation.** The original U8 wrote `size` as a
top-level frontmatter key via mdfront, but `models.ArtifactFromFrontmatter`
(`internal/models/frontmatter.go:39-116`) only maps a fixed key allowlist plus a
nested `custom_fields:` map; an unknown top-level `size` is dropped on re-parse.
*Resolution:* store `size` under `custom_fields` (like `severity`/`harness_status`);
`artifactFieldValue` already reads `CustomFields["size"]` (`field_validation.go:80-87`).

**P1 — DB index sync.** A raw mdfront+atomicfile write bypasses `persistArtifact`,
which is what calls `db.UpsertItem`; the SQLite index would go stale.
*Resolution:* the body-preserving size mutation MUST call `db.UpsertItem` (reuse
the persist seam), never a markdown-only write.

**P1 — enum validation not enforced.** `ValidateArtifactFields`
(`field_validation.go:21-33`) checks required-field *presence* only and `continue`s
on `Optional`; it never checks `FieldDef.Values`. An invalid `--size XXL` would
NOT be rejected by the existing path. *Resolution:* add a targeted
`size`-value enum check at the mutation entry point (scoped to the written field —
do NOT retrofit global enum enforcement onto all existing artifacts, which would
regress legacy values).

**P1 — U7/U8 contradiction + dropped key.** `core.UpdateArtifact`
(`artifacts.go:462-509`) type-switches known keys; a bare `updates["size"]` routes
nowhere (only `harness_status`/`custom_fields` reach CustomFields). And U7 routed
through `WriteArtifactFile` (full rebuild) while U8 said "instead of
WriteArtifactFile" — contradictory. *Resolution:* one seam — a dedicated
body-preserving `core.SetArtifactSize` used from the start (no interim
rebuild-based `--size`); the generic update path is left unchanged (bounded blast radius).

**P1 — cooperative context timeout.** `context.WithTimeout(5s)` does not interrupt
synchronous `os.ReadFile`/`yaml.Unmarshal` or a blocking `Mutex.Lock`; the 5s
contract and a distinct timeout code cannot be honored by wrapping alone, and the
injected-deadline test would pass vacuously. *Resolution:* run the validation in a
goroutine and `select` on `ctx.Done()`; distinguish via `errors.Is(err,
context.DeadlineExceeded)`. Note autoharness's subprocess `timeout_seconds: 5`
(design §5) is the authoritative outer bound.

**P1 — exit-code contract under-specified.** "non-zero fail / distinct timeout"
pinned no integer values and did not disambiguate validation-fail vs IO/scope vs
busy; Cobra `RunE` also collapses errors to exit 1. *Resolution:* publish a
versioned exit-code table (0 pass / 1 validation-fail / 2 timeout / 3 scope-or-IO
/ 4 busy) and a versioned target-mode JSON schema; set codes explicitly.

**P2 — per-task lock is process-global.** Modeling verbatim on `stash_lock.go`
uses one package-global mutex (whole-process serialization) and takes the mutex
before the `O_EXCL` check, so an in-process second acquire blocks rather than
observing the sidecar-busy path (the U4/U5 "busy while held" test would not be
observable in-process). *Resolution:* use a guarded `map[string]*sync.Mutex`
keyed by task path; test the sidecar-busy path via a pre-created sidecar; `defer`
the unlock so every error path releases both the sidecar and the mutex.

**P2 — golden fixture canonicalization.** mdfront.Encode re-marshals frontmatter
(sorted keys) whereas WriteArtifactFile emits a fixed key order, so a raw
"only the size line changed" diff holds only for already-canonical fixtures.
*Resolution:* assert **body bytes byte-identical** (strong) + frontmatter
**semantically equal** (parse+compare maps), and generate the fixture through the
codec. Reuse `atomicfile.WriteFileAtomic` (already short-write-guarded and
Windows-rename-safe) — do not reimplement atomic write.

**P2 — U6 mislabeled as a verifiable root.** Its acceptance depended on U1/U2.
*Resolution:* U6 acceptance is a pure schema + `ValidateArtifactFields` +
custom_fields round-trip check (true root); `--target` runtime verification moves
to U2's wave.

**P2 — U1 scope guard second source of truth.** *Resolution:* derive the
`.backlogit/` guard from the existing workspace storage root / `artifactSearchDirs`
(`artifacts.go:553-598`) rather than a bespoke prefix check.

**P2 — MCP parity (U3/U9) beyond the declared four items.** The design specifies
CLI-only contracts; autoharness consumes via CLI subprocess. *Resolution:* KEEP
U3/U9 but scope-flag them — because `--size`/`--target` route through shared
`core` functions, the MCP handlers only pass the param through, making parity
near-free and preventing the documented CLI/MCP drift class
(`2026-05-07-mcp-cli-config-parity.md`). U3/U9 are the explicit **deferrable**
units if the operator wants to trim this slice to strictly the four items.

**P3 (advisory)** — reuse `atomicfile` verbatim; name the helper for its scope
(`SetArtifactSize`, not a generic `writeFrontmatterField`); return concrete result
structs. Accepted into the revision.

**Gate action:** Plan revised to address every P1 and the P2 architecture
findings (see revised Implementation Units, Decisions, and Risks). Re-review as
Attempt 2.

### Attempt 2 — Gate: PASS (residual P1 closed by spec-tightening; P2/P3 incorporated)

Two independent re-reviewers ran against the revised plan.

* **Go Reviewer — PASS.** Confirmed all six Attempt-1 P1 fixes are correctly and
  consistently incorporated (size→custom_fields round-trip; single `SetArtifactSize`
  seam calling `db.UpsertItem` without touching the generic path; targeted enum
  check; goroutine+select 5s timeout with explicit 0–4 exit table; per-path mutex
  with observable busy; body-byte + semantic golden). No residual P0/P1.
* **Architecture Strategist — one residual P1 + P2/P3 nits.** Endorsed the seam and
  dependency architecture (clean core→leaf direction, no cycles, no serializer
  divergence) but flagged one **P1 spec-completeness gap**: because `db.UpsertItem`
  runs `INSERT OR REPLACE` on the **full item row**, U7 must reconstruct a
  *fully-populated* `*models.Artifact` before upsert — a partial `{ID, CustomFields}`
  stub would null out `title`/`status`/`priority` in the index and silently re-open
  the markdown↔DB drift class, and the U7 test as written would not catch a
  collateral column wipe.

**Residual P1 — resolution (incorporated).** U7 now specifies (a) reconstruct the
full `*models.Artifact` from the *same* decoded frontmatter+body via
`models.ArtifactFromFrontmatter`/`findArtifact` with `custom_fields.size` set, and
(b) a DB-sync assertion that non-`size` index columns (`title`/`status`/`priority`)
are **unchanged** after the mutation (a partial-artifact upsert MUST fail this test).
Signature updated to `SetArtifactSize(ctx, ws, id, size)`.

**P2/P3 — resolutions (all incorporated):**

* **P2 (both) — busy-semantics asymmetry / combined-flag double-write.** U8 now
  makes `--size` **mutually exclusive** with other frontmatter flags (error before
  write — prevents the SetArtifactSize + UpdateArtifact double-write that would
  negate body preservation) and defines U8's **busy → exit 4**, mirroring the doctor
  table so the sizing hook has deterministic contention behavior.
* **P2 (Go) — hook-event gap.** Documented as intentional in U7 + Decisions:
  size-only mutations emit no `emit_hook_event` (only pre-hook is a no-op on
  unchanged status; post-hooks best-effort). Revisit only if autoharness needs a
  size-change signal.
* **P3 — timeout goroutine leak.** U2 now specifies a **buffered** (cap 1) result
  channel and notes the timeout↔stale-lock reclaim (60s TTL, invariant #5).
* **P3 — unbounded mutex map / TryLock.** U4 documents the long-lived-process map
  growth (bounded by distinct task paths; acceptable for the CLI-subprocess gate)
  and names `Mutex.TryLock` as the option if in-process non-blocking busy is needed.
* **P3 — third enum path.** Decisions now prefer a shared value-membership helper
  over a hand-rolled third enum check.
* **P3 — dependency-graph diagram.** Redrawn so U6→U7 only (no spurious U6→U5) with
  an authoritative edge list; prose already governed.
* **P3 — Invariant #1 overstated.** Restated as body bytes byte-identical +
  non-`size` frontmatter semantically equal (matches the U7 golden test contract).

**Gate outcome:** The sole residual P1 was a plan-specification completeness gap
(the `SetArtifactSize` seam itself is endorsed by both reviewers); it is closed by
adopting the reviewer's exact remediation as additive spec text with a new guarding
assertion — no design change. All P2/P3 items incorporated. No design-level defect
remains and the cycle budget (max 2 re-entry cycles) is respected. **PASS →
proceed to harvest.**

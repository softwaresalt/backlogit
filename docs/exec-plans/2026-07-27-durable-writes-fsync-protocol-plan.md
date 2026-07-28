---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for an opt-in repo-wide durable_writes fsync protocol across the two shared write primitives (atomicfile.WriteFileAtomic and events.EventWriter.AppendEvent): temp+parent-dir fsync on POSIX, an unconditional Windows atomic-replace that fixes the pre-existing remove-before-rename canonical data-loss window, a two-class outcome-based error contract (not-applied vs indeterminate), and platform-asymmetric durability docs plus regression harnesses.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-27-durable-writes-fsync-protocol-plan.md
title: 'Repo-wide durable_writes fsync protocol (atomicfile + events)'
---

## Source

- Feature: `123-F` (queued, priority medium) — "Repo-wide durable_writes fsync
  protocol (atomicfile+events)". Labels: `durability`, `filesystem`, `deferred`.
- Spike findings (conclusion **proceed**, confidence **high**):
  `docs/decisions/2026-07-23-atomicfile-events-fsync-durability-protocol-spike.md`.
- Origin spike: `120-F` (`spike_ref` link on `123-F`; `promoted_to: ["123-F"]`).
- Prior art (repo patterns to reuse):
  - `internal/events/fsutil.go` — `syncAppendLine` (file fsync) and
    `syncWriteFileAtomic` (temp fsync + Windows pre-remove, "acceptable for
    regenerable files" only).
  - Typed-error leaf pattern: `internal/errors/gate_errors.go` +
    `internal/cli/gate_exit.go` (route classification via `errors.Is/As`
    without importing `core`).
  - Regression-harness pattern: 040-F/041-S
    (`hook_checkpoint_harness_041_test.go`, `shipment_041_harness_test.go`).

## Problem Frame

backlogit's canonical write path runs through a small set of shared write
functions, none of which currently fsyncs. Two are leaf primitives; a third is
the exported artifact-rewrite choke point that today **bypasses** the primitive
entirely:

1. `internal/atomicfile/atomicfile.go` — `WriteFileAtomic(path, data)` writes a
   same-directory temp file then `os.Rename`. It performs **no** `Sync()` on the
   temp file and **no** parent-directory fsync, so a crash/power-loss can lose a
   just-"written" canonical artifact even after the call returns. Worse, its
   Windows fallback (lines ~52-60) does `os.Remove(path)` then retries the rename
   on any rename error: a process crash **or a second rename failure between the
   remove and the retry leaves the canonical artifact MISSING** — a real,
   pre-existing data-loss window, not mere durability polish. Every canonical
   write inherits this, including the size seam
   (`internal/core/artifact_size.go:165` calls `WriteFileAtomic` directly).
2. `internal/events/stream.go` — `EventWriter.AppendEvent` runs
   `os.MkdirAll(logsDir)` and opens each per-item log with `O_APPEND|O_CREATE`
   (lines ~56-64), writing via `fmt.Fprintf` with a deferred `Close()` and **no**
   `Sync()`. The **first** append to a new log creates a new dirent (and possibly
   the `logs/` dir), which a file-only fsync does not persist.
3. `internal/core/artifacts.go` — `WriteArtifactFile(artifact, filePath)` is the
   **exported, single choke point for every production artifact rewrite** (its
   own doc comment says so) and has **8 callers** across `cli` and `core`
   (status transitions, `move`, reference rewrites, `migrate`, shipment
   lifecycle, and `persistArtifact`). Critically, it does **not** call the
   `atomicfile` primitive — it inlines its own `os.WriteFile(tmp)` + `os.Rename`
   with no fsync. So `atomicfile.WriteFileAtomic` is today reached directly only
   by the size seam (`internal/core/artifact_size.go:165`); the dominant
   artifact-persist path bypasses it. A repo-wide guarantee therefore requires
   routing `WriteArtifactFile` through the durable primitive (U8), not just
   hardening `WriteFileAtomic`.

Additionally, `events.EventWriter` is **constructed independently at six sites**
(`core/archive.go:275`, `core/commits.go:38` + `:82`, `core/gate_evidence.go:60`,
`core/shipment.go:307`, `mcp/server.go:76`). Because the durable option lives at
writer construction (U5), a default-off option does **not** propagate to these
existing writers unless each construction site is explicitly rewired — a dedicated
wiring unit (U9), not a single composition-root change.

The spike verified the premise fully and recommends promoting the existing
`fsutil.go` file-content fsync patterns into both primitives, adding the missing
POSIX parent-directory fsync, gating directory fsync on
`runtime.GOOS != "windows"`, replacing the Windows remove-before-rename fallback
with a real atomic replace, shipping behind an opt-in `durable_writes` mode
(default off), and documenting the platform-asymmetric guarantee.

## Requirements Trace

Each scope item from `123-F` / the spike maps to at least one unit below.

| # | Source requirement | Unit(s) |
|---|---|---|
| 5 | Opt-in `durable_writes` mode (default off; gates fsync additions only) | U1 |
| 4 | Two-class outcome-based error contract (`ErrWriteNotApplied` / `ErrWriteIndeterminate`) | U2 (types), U3/U5 (emit), U6 (consume) |
| 1 | `WriteFileAtomic`: temp fsync + POSIX parent-dir fsync **after** rename, gated `GOOS != "windows"` | U3 |
| 3 | Canonical-write Windows atomic replace (real data-loss fix, **unconditional**) | U4 |
| 2 | `AppendEvent` durability: file fsync + conservative POSIX logs-dir (and level-by-level MkdirAll) fsync on durable appends; Windows best-effort | U5 |
| 1/2 | Route the central artifact-rewrite choke point `WriteArtifactFile` (8 callers) through the durable primitive so artifact persists are durable repo-wide, not just the size seam | U8 |
| 2/5 | Rewire the six independent `EventWriter` construction sites to pass the durable option (else U5 is inert for every existing writer) | U9 |
| 4 | Consume the indeterminate error class in critical callers (size seam, status transitions) without a reconciliation engine; retry scoped to the atomic write (never the composite event-then-write op) | U6 |
| 6 | Document platform-asymmetric guarantee; regression harnesses; micro-benchmark fsync latency | U3/U4/U5 (harnesses, test-first), U3 (benchmark), U7 (docs) |

## Implementation Units

Each unit is a single-domain atomic milestone scoped to roughly two hours of
effort (the binding constraint). The `< 3 files / < 5 functions / < 4 scenarios`
figures are heuristics, not hard limits: U4 touches four files and U3, U6, and U8
touch three, but in every case they are **tightly-coupled files within one
package** (a build-tagged platform seam plus its test, or one writer plus its
test) that must change together and still fit the two-hour rule. These are
explicit, reviewed exceptions to the file-count heuristic, not scope violations;
each unit remains a single skill domain with one verifiable milestone. Execution
posture is **test-first** for all code units (harness lands failing before
production code).

### U1 — `durable_writes` config flag + primitive options (config)

- **Change**: Add `DurableWrites bool` to `WorkspaceConfig`
  (`internal/config/schema.go`) as `yaml:"durable_writes,omitempty"` (default
  `false`); expose a resolver/accessor. The flag is read at the composition root
  and passed to the primitives as an **option value** (see U3/U5) — `atomicfile`
  and `events` never import `config`, and no existing function signature changes.
- **Files**: `internal/config/schema.go` (+ defaults/loader if centralized),
  `internal/config/*_test.go`.
- **Tests**: (a) default load → `durable_writes == false`; (b) explicit
  `durable_writes: true` round-trips; (c) `omitempty` keeps serialized output
  clean when false.
- **Posture**: test-first. No deps.

### U2 — Two-class durability error contract (errors leaf)

- **Change**: Add typed errors in the `internal/errors` leaf (new
  `durability_errors.go`), mirroring `gate_errors.go`, named by **outcome** (not
  by write phase):
  - `ErrWriteNotApplied` — the mutation **definitely did not apply** (canonical
    file/log untouched); the **failed atomic write** is safe to retry. This is a
    property of the single write, not of a composite operation: a caller that has
    **already appended an audit event** before the file write (e.g. the size
    seam, see U6) must retry only the atomic write, never the whole op, or it
    double-appends the event.
  - `ErrWriteIndeterminate` — the mutation is **possibly-applied / outcome
    uncertain**; callers **MUST NOT blindly retry** (a retry may double-apply a
    status transition or duplicate an audit event).
  Provide `errors.Is/As` predicates (`IsWriteNotApplied`, `IsWriteIndeterminate`)
  so `atomicfile`, `events`, `core`, `cli`, and `mcp` classify without import
  cycles. Both wrap the underlying cause with `%w`.
- **Files**: `internal/errors/durability_errors.go`,
  `internal/errors/durability_errors_test.go`.
- **Tests**: (a) not-applied matches its predicate but not indeterminate; (b)
  indeterminate matches its predicate but not not-applied; (c) both preserve
  `%w` unwrap to the cause.
- **Posture**: test-first. No deps.

### U4 — Windows canonical atomic replace, unconditional safety fix (atomicfile, Windows)

- **Change**: Replace the Windows remove-before-rename fallback with a real
  atomic replace via `golang.org/x/sys/windows.MoveFileEx` with
  `MOVEFILE_REPLACE_EXISTING|MOVEFILE_WRITE_THROUGH` (the temp file is
  same-directory / same-volume, so **no** `MOVEFILE_COPY_ALLOWED`). The
  destination is **never** removed before the replacement is in place; on failure
  return U2 `ErrWriteNotApplied` (destination untouched). This fix is
  **unconditional** — it applies whether or not `durable_writes` is on, because
  the pre-remove is a real data-loss defect, not a durability nicety.
  `ReplaceFileW` is **rejected** (its write-through story is unreliable and
  documented failure modes can leave the destination absent). Build tags:
  `atomicfile_windows.go` (MoveFileEx) + `atomicfile_other.go` (POSIX `os.Rename`
  passthrough). Promote `golang.org/x/sys` from indirect to direct.
- **Files**: `internal/atomicfile/atomicfile_windows.go`,
  `internal/atomicfile/atomicfile_other.go`, `internal/atomicfile/atomicfile.go`
  (call the platform seam), `internal/atomicfile/atomicfile_windows_test.go`.
- **Tests**: (a) Windows replace over an existing dest succeeds and the dest is
  never observed missing; (b) replace failure returns `ErrWriteNotApplied` and
  leaves the original intact; (c) absent-dest and locked-dest cases behave
  fail-closed; POSIX build uses plain rename (parity).
- **Posture**: test-first. **Deps: U2.**

### U3 — `WriteFileAtomic` fsync sequence + POSIX parent-dir fsync (atomicfile)

- **Change**: Add `WriteFileAtomicWithOptions(path, data, Options{DurableWrites bool})`
  and keep the existing `WriteFileAtomic(path, data)` as a thin wrapper passing
  the zero value (durable off) — **no existing caller signature changes**. When
  durable is on: `Sync()` the temp file before close; open the **parent-dir
  handle before the rename**, and after a successful rename, on
  `runtime.GOOS != "windows"`, `Sync()` that dir handle so the rename is durable.
  Classification (U2): any failure **before** the rename commits →
  `ErrWriteNotApplied`; a parent-dir fsync failure **after** the rename →
  `ErrWriteIndeterminate`. With durable **off**, no fsync is added (fast path);
  the OFF contract is "no added fsync" — the U4 Windows atomic-replace safety fix
  is unconditional and applies in both modes. Add an `fsync` micro-benchmark.
- **Files**: `internal/atomicfile/atomicfile.go`,
  `internal/atomicfile/atomicfile_test.go` (+ `atomicfile_bench_test.go`).
- **Tests**: (a) durable-on write fsyncs temp + parent dir (injected dir-fsync
  seam); (b) **Windows dir-fsync skip assertion** (no dir fsync attempted); (c)
  post-rename dir-fsync failure returns `ErrWriteIndeterminate` while the file is
  already replaced.
- **Posture**: test-first. **Deps: U1, U2, U4** (builds on U4's platform-replace
  seam).

### U5 — `AppendEvent` fsync + directory durability (events)

- **Change**: Add durable options at **writer construction**
  (`NewEventWriter(logsDir, WithDurableWrites(true))`; writer stays immutable) —
  the existing `NewEventWriter(logsDir)` is unchanged (durable off). When durable
  is on: `Sync()` the log file after write; **conservatively** fsync the parent
  `logs/` dir on **every** durable append on `runtime.GOOS != "windows"`
  (first-create detection is unsafe under concurrency, so do not rely on it);
  when the dir tree must be created, create and fsync **level-by-level** so each
  new ancestor dirent is durable. A partial write, or a post-write file/dir fsync
  failure, returns U2 `ErrWriteIndeterminate` (an append is not atomic — a failed
  append is not safe to blindly retry). Reuse `fsutil.go`'s `syncAppendLine`
  file-fsync pattern; do **not** reuse `syncWriteFileAtomic` (canonical-unsafe
  Windows pre-remove). Windows dirent durability is best-effort (file-only).
- **Files**: `internal/events/stream.go`,
  `internal/events/stream_test.go` (+ reuse/extend `fsutil.go`).
- **Tests**: (a) durable append fsyncs the file; (b) durable append fsyncs the
  parent dir on POSIX (seam), Windows skip assertion; (c) a partial-write /
  dir-fsync failure returns `ErrWriteIndeterminate`.
- **Posture**: test-first. **Deps: U1, U2.**

### U6 — Consume the error contract in critical callers (core)

- **Change**: Route the critical single-item mutation paths (size seam
  `SetArtifactSizeWithProvenance` → `artifact_size.go:165`; status-transition
  writers) through the durable options and define concrete handling of the U2
  classes — **no automatic state-reconciliation engine is introduced**:
  - `ErrWriteNotApplied` → the **failed atomic write** may be retried once, but
    **not** by re-invoking the composite operation. The size seam
    `SetArtifactSizeWithProvenance` appends its audit event **first**
    (`artifact_size.go:124-139`) and only then writes the artifact (`:161-167`),
    so re-running the whole op would append a duplicate event. Retry is therefore
    scoped to the atomic file write using the already-encoded bytes **under the
    same lock**; alternatively the composite op is marked non-retryable. Either
    way the event count must remain exactly one.
  - `ErrWriteIndeterminate` → the caller **does not retry the mutation**; it
    surfaces/logs the operation as possibly-committed and relies on the existing
    invariants that make this safe: the durable **frontmatter value is the single
    source of truth**, and advisory/orphan crash-residue events are already
    **ignored on read** (`size-estimation-contract.md:97-103`). Where a cheap
    idempotent re-verify is available (re-open + re-fsync the already-visible
    file), the caller performs it; it does **not** reconstruct intent.
  Audit-event/status writers must not emit duplicate events on the indeterminate
  path. Bulk regeneration stays best-effort (durable option not set).
- **Files**: `internal/core/artifact_size.go`, the status-transition writer
  (e.g. `internal/core/gate_transition.go`), and one `_test.go`.
- **Tests**: (a) not-applied → the atomic write is retried but the audit event
  count stays **exactly one** (no duplicate); (b) indeterminate → no double-apply
  and no duplicate event; (c) durable-off path unchanged.
- **Posture**: test-first. **Deps: U2, U3, U4, U5, U8.**

### U7 — Document platform-asymmetric guarantee (docs)

- **Change**: Update `internal/atomicfile/doc.go` and the 108-F
  `size-estimation-contract.md` to describe: the opt-in `durable_writes` mode,
  the **unconditional** Windows atomic-replace safety fix, the two-class
  outcome-based error contract, the platform-asymmetric guarantee (full POSIX
  power-loss durability incl. new-file dirents; Windows file-content durability
  with best-effort dirent), and the recorded fsync micro-benchmark +
  critical-vs-bulk policy.
- **Files**: `internal/atomicfile/doc.go` (package doc comment — docs-only edit),
  `docs/product-specs/*size-estimation-contract*.md`.
- **Tests**: `make docs-lint` + markdownlint P-008; no runtime test.
- **Posture**: docs. **Deps: U3, U4, U5, U6, U8, U9** (documents shipped
  behavior).

### U8 — Route `WriteArtifactFile` through the durable primitive (core)

- **Change**: Make `WriteArtifactFile` (`internal/core/artifacts.go:735`) delegate
  its write to the durable primitive instead of its inline `os.WriteFile(tmp)` +
  `os.Rename`: call `atomicfile.WriteFileAtomicWithOptions(filePath, content,
  atomicfile.Options{DurableWrites: cfg})` where `cfg` is threaded from the
  workspace `durable_writes` flag (U1). This makes the durability guarantee reach
  **all 8 artifact-rewrite callers** (status transitions, `move`, reference
  rewrites, `migrate`, shipment lifecycle, `persistArtifact`) through the single
  documented choke point, honoring its own "single choke point" contract. The
  existing archive-provenance guard at the top of `WriteArtifactFile` is
  preserved unchanged. On failure the U2 classes propagate to callers
  (`ErrWriteNotApplied` before commit; `ErrWriteIndeterminate` after). Because the
  U4 Windows atomic-replace safety fix is unconditional, this routing also removes
  the last inline `os.Rename` on the artifact-persist path.
- **Files**: `internal/core/artifacts.go`, `internal/core/artifacts_test.go`.
- **Tests**: (a) durable-on artifact rewrite fsyncs temp + parent dir (via the U3
  seam) and the provenance guard still rejects an archived artifact without
  provenance; (b) durable-off rewrite is unchanged behaviorally (no added fsync)
  and still atomic; (c) a post-rename dir-fsync failure surfaces
  `ErrWriteIndeterminate` to the caller.
- **Posture**: test-first. **Deps: U1, U3, U4.**

### U9 — Event-writer durability wiring across construction sites (core, mcp)

- **Change**: Rewire the **six** independent `NewEventWriter(logsDir)`
  construction sites to pass the durable option from U5 when the workspace
  `durable_writes` flag (U1) is on:
  `core/archive.go:275`, `core/commits.go:38` + `:82`,
  `core/gate_evidence.go:60`, `core/shipment.go:307`, and `mcp/server.go:76`.
  Prefer a single internal helper (e.g. `core.newWorkspaceEventWriter(ws)`) that
  reads the flag once and applies `WithDurableWrites`, so the five core sites
  funnel through one construction point and only the MCP server site is wired
  separately; this avoids re-scattering the flag read. Without this unit, U5's
  option is inert for every existing writer.
- **Files**: `internal/core/events_writer.go` (new helper) + the touched core
  call sites, `internal/mcp/server.go`, and one `_test.go`.
- **Tests**: (a) with `durable_writes` on, an event appended through a core path
  fsyncs the log file (seam); (b) with the flag off, construction is unchanged;
  (c) the MCP server path constructs a durable writer when the flag is on.
- **Posture**: test-first. **Deps: U1, U5.**

## Dependency Graph

Edges (unit → its dependencies):

- U1 → ∅
- U2 → ∅
- U4 → {U2}
- U5 → {U1, U2}
- U3 → {U1, U2, U4}
- U9 → {U1, U5}
- U8 → {U1, U3, U4}
- U6 → {U2, U3, U4, U5, U8}
- U7 → {U3, U4, U5, U6, U8, U9}

Topological execution order: **U1, U2 → U4, U5 → U3, U9 → U8 → U6 → U7**. No
cycles. U4 (the unconditional Windows safety fix) lands before U3 because U3's
rename goes through U4's platform-replace seam; U8 (routing the artifact-rewrite
choke point) lands after U3/U4 so it delegates to the finished durable primitive;
U9 (event-writer wiring) only needs U1/U5; U6 consumes the error contract from the
now-durable size seam and `WriteArtifactFile` (U8). `atomicfile` never imports
`config` or `core`: the durable-mode option and error types flow *in*;
classification flows *out* via the `errors` leaf, preserving the leaf-primitive
dependency direction.

## Decisions and Rationale

1. **Opt-in default-off `durable_writes`** — fsync per critical write adds
   latency; bulk regeneration (hundreds of artifacts on sync) must keep current
   throughput. Default-off makes the fsync path a zero-risk no-op until explicitly
   enabled and benchmarked (spike recommendation). Note this governs only the
   **fsync** additions — the Windows atomic-replace safety fix (decision 3) is
   unconditional.
2. **Options-based API, not signature changes** — `WriteFileAtomic(path, data)`
   has many callers repo-wide; adding a positional `bool` would be a hidden
   scope explosion and make default-off non-inert. Instead add
   `WriteFileAtomicWithOptions(...)` (existing wrapper delegates with durable off)
   and construct `EventWriter` with durable options
   (`NewEventWriter(logsDir, WithDurableWrites(true))`, writer immutable). Existing
   callers are untouched.
3. **Windows atomic replace is an unconditional safety fix** — the current
   remove-before-rename can leave a canonical file missing on a crash between
   remove and retry. `MoveFileEx(REPLACE_EXISTING|WRITE_THROUGH)` replaces
   atomically and never deletes the destination first. This is a correctness fix
   for a real data-loss defect, so it applies in **both** durable-on and
   durable-off modes — it is not gated by the flag. `ReplaceFileW` is rejected
   (unreliable write-through; failure modes can leave the destination absent).
4. **Build-tagged platform seam** (`_windows.go` / `_other.go`) — keeps the
   Windows syscall surface isolated and the POSIX path a plain `os.Rename`,
   testable per platform.
5. **Outcome-based two-class error contract** — a blanket "fail-closed" is
   impossible once a rename commits (the file is already visibly replaced), and
   phase-based naming ("pre/post-commit") mislabels non-atomic appends. Classify
   by **outcome**: `ErrWriteNotApplied` (definitely-not-applied, safe to retry —
   e.g. any `WriteFileAtomic` failure before the rename commits) vs
   `ErrWriteIndeterminate` (possibly-applied, must not blindly retry — e.g. a
   post-rename dir-fsync failure, **or an `AppendEvent` partial write / post-write
   fsync failure**, since an append is not atomic).
6. **No reconciliation engine** — automatic state-reconciliation with dedup would
   require persisted intent, operation IDs, an owner, and a startup trigger, all
   out of scope. Instead, the indeterminate error is **surfaced**; safety rests on
   existing invariants (frontmatter value is the single source of truth; advisory
   and orphan crash-residue events are ignored on read,
   `size-estimation-contract.md:97-103`). A cheap idempotent re-verify (re-open +
   re-fsync the already-visible file) is allowed; intent reconstruction is not.
7. **Reuse `fsutil.go` file-fsync patterns, not `syncWriteFileAtomic`** — the
   latter's Windows pre-remove is explicitly "acceptable for regenerable files"
   only and is canonical-unsafe.
8. **Conservative event-dir durability** — first-create detection is unsafe under
   concurrency and `MkdirAll` cannot report which ancestors it created, so on
   durable appends fsync the parent dir unconditionally and create+sync new
   directories level-by-level.
9. **Route `WriteArtifactFile`, don't narrow the guarantee** — the plan-review
   surfaced that `atomicfile.WriteFileAtomic` is reached directly only by the
   size seam; the dominant artifact-persist path is the exported
   `WriteArtifactFile` choke point (8 callers) which inlines its own
   `os.WriteFile` + `os.Rename`. Rather than narrowing the "repo-wide" claim, U8
   routes `WriteArtifactFile` through the durable primitive so all callers inherit
   durability through the single documented choke point — a small, high-leverage
   change that makes the repo-wide guarantee actually true.
10. **Explicit event-writer wiring unit** — `EventWriter` is constructed at six
    independent sites, so a default-off option at construction (U5) is inert for
    existing writers unless each site is rewired. U9 centralizes core construction
    behind one helper and wires the MCP server path, so enabling `durable_writes`
    actually reaches event appends instead of silently no-op'ing.

## Risks and Caveats

- **Blast radius (highest risk)**: `WriteFileAtomic` is the shared canonical
  writer, and `WriteArtifactFile` (U8) is the exported artifact-rewrite choke
  point for every artifact. A regression corrupts or loses writes repo-wide.
  *Mitigations*: the fsync additions are opt-in default-off (inert until enabled)
  and delivered via new `*WithOptions` entrypoints so existing callers' signatures
  are untouched; U8 changes only `WriteArtifactFile`'s internals (delegates to the
  durable primitive) and U9 rewires the six event-writer construction sites, both
  gated by the same default-off flag so the entire expanded surface is inert until
  `durable_writes` is enabled; the durable-off fast path adds no fsync; test-first
  harnesses per unit; the Windows atomic-replace lands behind a build tag with its
  own tests; git is the recovery backstop for `.backlogit/` (Principle IX).
- **Windows safety fix changes default-mode failure behavior**: removing the
  remove-before-rename fallback is unconditional, so durable-off is "no added
  fsync," **not** byte-identical to today on the Windows failure path — but the
  new behavior is strictly safer (never deletes the destination first).
  *Mitigation*: U4 tests assert never-missing + fail-closed on existing, absent,
  and locked destinations.
- **Windows atomic-replace semantics**: `MoveFileEx` ACL/attribute and
  locked-file behavior must be confirmed. *Mitigation*: pin to same-volume
  `MoveFileEx(REPLACE_EXISTING|WRITE_THROUGH)` (no `COPY_ALLOWED`); U4 tests cover
  the failure cases; `ReplaceFileW` is rejected outright.
- **Non-atomic appends**: an `AppendEvent` partial write is not safe to blindly
  retry. *Mitigation*: classify partial-write / post-write fsync failures as
  `ErrWriteIndeterminate`; U6 callers surface rather than retry.
- **Indeterminate handling is subtle**: callers must not double-apply.
  *Mitigation*: no reconciliation engine is promised; U6 leans on the existing
  frontmatter-source-of-truth + advisory-events-ignored invariants and asserts
  no double-apply / no duplicate event.
- **fsync latency** may be too high for some "critical" paths. *Mitigation*: U3
  micro-benchmark finalizes the critical-vs-bulk policy before U6 wiring decides
  which callers opt in.
- **Directory-fsync portability**: POSIX-only; Windows has no directory-handle
  flush. *Mitigation*: `runtime.GOOS != "windows"` gate + explicit skip
  assertion; Windows dirent durability documented as best-effort (U7).
- **New direct dependency** `golang.org/x/sys` — already an indirect dep at
  v0.39.0; promotion is low-cost and justified by the Windows syscall need
  (Principle VI).

## Constitution Check

- **I. Safety-First Go** — pass. All code is Go; errors wrap with `%w` (U2 types
  preserve unwrap); no `unsafe`. `golang.org/x/sys/windows` syscalls are the
  standard, non-`unsafe` Windows API surface.
- **II. Test-First Development (NON-NEGOTIABLE)** — pass. Every code unit
  (U1–U6, U8, U9) is test-first: a failing harness lands before production code,
  per the 040-F/041-S harness pattern; U7 is docs-only.
- **III. Workspace Isolation / Security Boundaries** — pass. All writes resolve
  within the workspace; no new path-traversal surface; no secrets. The Windows
  replace operates on the same resolved paths as today.
- **IV. CLI Workspace Containment (NON-NEGOTIABLE)** — pass. No out-of-tree
  writes; the feature only changes *how* in-tree files are written durably.
- **V. Structured Observability** — pass. The indeterminate error class is
  explicitly surfaced (not swallowed) so callers and logs can record
  possibly-committed operations.
- **VI. Single Responsibility** — pass with note. One dependency promotion
  (`golang.org/x/sys`, already indirect) justified by the Windows atomic-replace
  requirement; no speculative additions.
- **VII. Destructive Command Approval (NON-NEGOTIABLE)** — N/A at plan time; no
  destructive terminal commands. The Windows change *reduces* destructive
  filesystem behavior (removes a delete-before-replace).
- **IX. Git-Friendly Persistence** — pass. Output remains human-readable
  Markdown/YAML; durability strengthens atomic-write guarantees.
- **XI. Merge Commit History Preservation (NON-NEGOTIABLE)** — pass. Ships via a
  merge commit; no squash/rebase.

Constitution Check: pass

## Plan Hardening Signals

- Public API / schema / contract change — **present**: new `durable_writes`
  config field, new public error classes, and a changed durability contract on a
  shared primitive.
- Security / auth / permission / compliance-sensitive behavior — **absent**.
- Migration / backfill / destructive / irreversible step — **present (net-safe)**:
  changes a shared canonical write primitive and removes a Windows
  delete-before-replace data-loss window; no data migration, but high blast
  radius on the write path.
- External integration / operator checkpoint / external dependency — **present
  (minor)**: promotes `golang.org/x/sys` to a direct dependency.
- High runtime / rollout / rollback risk — **present**: `WriteFileAtomic` is the
  repo-wide canonical writer; a regression is high-impact. Rollback is clean
  because the feature is opt-in default-off.

Requires plan hardening: yes — **satisfied inline** in this revision (see
_Plan Hardening — Acceptance Criteria_ below). This plan was revised in response
to an independent plan-review (rubber-duck) pass and a second GitHub plan-review
pass; the high-blast-radius hardening depth is now folded into concrete
acceptance criteria rather than deferred.

## Plan Hardening — Acceptance Criteria

Concrete, testable gates that must be green before U-level work is accepted:

- **Crash-simulation (POSIX)** — a harness kills between the temp-fsync+rename and
  the parent-dir fsync and asserts, after reopen, the artifact is present and
  complete (U3/U8); a second harness kills across an event append and asserts the
  line is durable or the caller received `ErrWriteIndeterminate` (U5/U9).
- **Windows atomic replace** — `MoveFileEx(REPLACE_EXISTING|WRITE_THROUGH)` tests
  assert the destination is **never observed missing** across existing-dest,
  absent-dest, and locked-dest cases; `ReplaceFileW` is not used (U4).
- **Retry safety** — the size-seam test asserts the audit-event count stays
  **exactly one** across an `ErrWriteNotApplied` retry (U6), proving the composite
  op is not blindly re-run.
- **Wiring reach** — enabling `durable_writes` constructs durable writers on at
  least one core path and the MCP server path; disabling leaves construction
  byte-for-byte unchanged (U9).
- **Rollback** — `durable_writes: false` is a zero-migration instant disable, and
  reverting the merge restores the prior primitive with no data conversion.

## Runtime Verification and Closure

Changed runtime surfaces: the shared write path (no CLI/API/UI surface changes;
`durable_writes` is a config toggle).

- **U1**: verify config load with the flag both off (default) and on; confirm
  `omitempty` keeps existing configs unchanged.
- **U3/U4/U5**: on a POSIX host, enable `durable_writes` and confirm a canonical
  write + a new-log event append survive a simulated crash (kill between write
  and next open); on Windows, confirm the atomic replace never leaves the
  destination missing and dirent durability is best-effort. The per-unit
  harnesses (Windows dir-fsync skip, durable-append dir-fsync, post-rename
  dir-fsync-failure indeterminate class) are the primary proof.
- **U6**: verify a critical mutation receiving `ErrWriteIndeterminate` does not
  double-apply and does not emit a duplicate event (it surfaces + optionally
  cheap-re-verifies, per decision 6); a mutation receiving `ErrWriteNotApplied`
  retries only the atomic write, keeping the audit-event count at exactly one.
- **U8**: with `durable_writes` on, an artifact status transition / move persisted
  through `WriteArtifactFile` fsyncs temp + parent dir and the provenance guard
  still fires; durable-off stays atomic with no added fsync.
- **U9**: enabling `durable_writes` constructs durable event writers on a core
  path and the MCP server path; disabling leaves construction unchanged.
- **Operational closure**: rollback trigger = "any write-path regression or
  fsync-latency regression on bulk sync" → set `durable_writes: false` (instant,
  no data migration) and revert the merge. Monitoring = fsync micro-benchmark
  result recorded in U7; owner = Ship agent during the validation window. The
  high-blast-radius hardening depth (crash-simulation, Windows replace, retry
  safety, wiring reach, rollback) is specified in the _Plan Hardening — Acceptance
  Criteria_ section above and is a build-gate for the affected units.

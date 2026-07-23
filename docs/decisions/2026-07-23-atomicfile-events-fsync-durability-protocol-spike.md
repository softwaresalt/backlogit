---
title: "Repo-wide fsync durability protocol for atomicfile + events"
source: docs/decisions/2026-07-23-atomicfile-events-fsync-durability-protocol-spike.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
description: "Design-only spike for an OS-crash/power-loss durability protocol across backlogit's two shared write primitives (atomicfile.WriteFileAtomic and events.AppendEvent)."
docline:
    type: spike
    date: 2026-07-23
    time_box: "4h"
    conclusion: "proceed"
    confidence: "high"
    linked_parent_work_item: "120-F"
    promoted_to: ["none"]
    tags:
        - "durability"
        - "filesystem"
        - "atomicfile"
        - "events"
        - "windows"
---

## Goal

Design (do not implement) a repo-wide fsync durability protocol for backlogit's
two shared write primitives — `atomicfile.WriteFileAtomic` and
`events.EventWriter.AppendEvent` — so the whole repository write path (including
the 108-F size event+write seam) is durable against **OS-crash / power-loss**,
not merely process-crash. As a release gate, answer whether the currently-merged
size feature (108-F, shipped in 099-S) contains a data-loss/corruption **defect**
that must block cutting release **v1.7.0**.

## Success Criteria

* The sync-free premise is verified against current code with exact line refs.
* Every production call site of both primitives is enumerated (blast radius).
* A concrete durability sequence is specified for each writer, including the
  cross-writer ordering guarantee and Windows directory-fsync behavior.
* Cost, rollout staging, and failure-mode (fail-closed vs best-effort) stance are
  assessed.
* A crisp release-gating verdict distinguishes a pre-existing documented
  limitation from a new defect.

## Scope Constraints

* **Investigation/design only.** No `*.go` edits, no prototype, no commits.
* No git mutations and no backlog mutations (120-F status untouched; read-only
  `backlogit`/`grep` only).
* Principle IV: no writes outside `C:\Source\GitHub\backlogit`. The only file
  created is this findings artifact.
* Time-box: 4h-equivalent. Converge; record residual unknowns rather than
  expanding scope.

## Investigation Approach

1. Read the spike skill and both writer primitives; confirm the sync-free
   premise with exact current line refs.
2. Search prior work (`docs/compound`, `docs/decisions`, `docs/design-docs`,
   `docs/exec-plans`, `.backlogit`) for 099-S, 108-F, 040-F/041-S, atomicfile,
   events, and fsync.
3. Enumerate every production call site of `WriteFileAtomic` and `AppendEvent`
   to make the blast radius concrete.
4. Design the per-writer durability sequence and the cross-writer ordering
   guarantee; ground the Windows directory-fsync limitation in authoritative
   sources.
5. Assess cost, rollout staging, backward compatibility, and failure modes; then
   render the release-gating verdict with a limitation-vs-defect distinction.

## Findings

### What Was Discovered

**1. The sync-free premise is TRUE and is a deliberate, documented design.**

* `internal/atomicfile/atomicfile.go` — `WriteFileAtomic` spans **lines 15-63**.
  The sequence is `os.CreateTemp` (25) -> `writeAll` (34) -> `Chmod` (38) ->
  `Close` (42) -> `os.Rename` (46). There is **no `Sync()` of the temp file
  before rename** and **no parent-directory fsync after rename**.
* `internal/atomicfile/doc.go:36-42` states this explicitly under
  "**Sync-free by design**": "`WriteFileAtomic` deliberately does NOT fsync the
  temp file or its directory. os.Rename provides atomic VISIBILITY ... durability
  and rollback for docs and archive records are provided by git, not by fsync."
* `internal/events/stream.go` — `AppendEvent` spans **lines 43-67** (the stash
  cited 40-64). It does `os.OpenFile(..., O_APPEND|O_CREATE|O_WRONLY, 0644)` at
  **line 60** and `fmt.Fprintf(f, "%s\n", data)` at **line 65**, then a deferred
  `f.Close()` (64). There is **no `f.Sync()`** on the append path.

**2. The 108-F size seam wires both sync-free primitives and already documents
the OS-crash limitation.** `internal/core/artifact_size.go` performs
event-before-write, fail-closed: it appends the `estimate_history` audit event
(line 137, via `appendItemEventWithActorErr` -> `AppendEvent`) and refuses the
write if the append fails, then calls `atomicfile.WriteFileAtomic(ioPath, out)`
at **line 165**. The shipped contract
`docs/design-docs/2026-07-19-size-estimation-contract.md:104-106` states: "The
policy is **process-crash-safe only**. OS-level crash reconciliation,
exactly-once semantics, operation IDs, and doctor reconciliation are **out of
scope**." The event stream is explicitly "an advisory audit trail, not the
authority" (line 98); orphan crash-residue events are ignored on read (99-103).

**3. A reference durability implementation already exists in-repo (040-F/041-S)
— and it, too, omits parent-dir fsync.** Shipment 041-S
(`docs/archive/closure/2026-04-23-041-s-write-durability-closure.md`) added
fsync-before-close/rename to the JSONL *queue/checkpoint* paths but deliberately
did **not** touch the two canonical shared writers. `internal/events/fsutil.go`
provides two ready patterns:
   * `syncAppendLine` (12-33): open `O_APPEND` -> `Write` (with short-write
     guard) -> **`f.Sync()`** (21) -> `Close`.
   * `syncWriteFileAtomic` (39-74): create tmp -> `Write` (short-write guard) ->
     **`f.Sync()`** (49) -> `Close` -> Windows pre-remove (`runtime.GOOS ==
     "windows"`, 66-68) -> `os.Rename` (69).
   Both fsync the *file* but **neither fsyncs the parent directory**, so even the
   "durable" helpers do not guarantee the rename/dirent survives power loss.
   These helpers are consumed by `hook_checkpoint.go:96`,
   `checkpoint_lifecycle.go:178`, and `memory.go:80`.

**4. Blast radius — production call sites of the two shared primitives.**

`atomicfile.WriteFileAtomic` (5 production callers):
   * `internal/docline/service.go:349` (canonical docline codec apply path)
   * `internal/core/archive.go:487` (archive record write)
   * `internal/core/artifact_size.go:165` (108-F size seam)
   * `internal/core/doctor.go:605` and `:667` (doctor repair writes)

`EventWriter.AppendEvent` (7 production callers):
   * `internal/hooks/builtin_post.go:108`
   * `internal/core/archive.go:284`
   * `internal/core/commits.go:50` and `:91`
   * `internal/mcp/tools.go:590`
   * `internal/core/gate_evidence.go:61`
   * `internal/core/shipment.go:308`

Because both primitives are shared leaves, a durability change is genuinely
cross-cutting: it alters every markdown/archive write and every event append,
not just the size seam.

**5. The designed durability protocol.**

*Append writer (`AppendEvent`):* `OpenFile(O_APPEND|O_CREATE|O_WRONLY)` -> write
(retain the short-write guard already present in `writeAll`/`syncAppendLine`) ->
**`f.Sync()`** -> `Close`, surfacing any sync error fail-closed. This mirrors
`syncAppendLine` exactly; the append target already exists on disk (no new
dirent), so a parent-dir fsync is only needed on first file creation.

*Atomic writer (`WriteFileAtomic`):* create temp -> write (short-write guard) ->
**`Sync()` temp** -> `Close` -> `Rename` -> **`Sync()` parent directory** (POSIX
only; see Windows finding). This mirrors `syncWriteFileAtomic` plus the missing
parent-dir fsync that makes the *rename itself* durable.

*Cross-writer ordering (the 099-S invariant under power loss):* the size seam's
event-before-write ordering only holds across an OS crash if the event append is
made durable **before** the durable frontmatter write returns. Sequence:
`AppendEvent`(sync) returns durably -> then `WriteFileAtomic`(sync temp + sync
dir). With both writers synced, the only power-loss outcomes are "event only"
(a harmless orphan, already ignored on read) or "event + write" (consistent).
The forbidden "write without event" outcome is prevented, preserving the
event-before-write guarantee across power loss — which today's sync-free code
does **not** do (the OS may persist the rename but drop the buffered append).

**6. Windows directory fsync is the hard, partially-unsolvable part.** On POSIX,
`fsync(dirfd)` after rename durably persists the new dirent — the standard
"atomic + durable rename" idiom. On Windows there is **no reliable userspace
equivalent**: `(*os.File).Sync()` on a directory handle calls
`FlushFileBuffers`, which does not support directory handles and returns
`ERROR_ACCESS_DENIED` regardless of privilege (confirmed via Microsoft/Go issue
tracker, FlushFileBuffers API docs, and cross-platform libraries such as
`renameio` that no-op directory fsync on Windows). Consequently:
   * The protocol must **gate parent-dir fsync on `runtime.GOOS != "windows"`**
     (the exact idiom already used for the Windows pre-remove in
     `atomicfile.go:52` and `fsutil.go:66`).
   * On Windows the achievable guarantee is **file-content durability
     (`FlushFileBuffers` on the temp/append file) with best-effort dirent
     durability** — strictly weaker than POSIX. modernc.org/sqlite (the repo's DB
     layer) faces the same asymmetry and likewise flushes files, not
     directories, so the SQLite index inherits the same Windows behavior; this is
     acceptable because the index is a rebuildable cache.
   The durability guarantee is therefore inherently **platform-asymmetric** and
   must be documented as such, not advertised as uniform power-loss safety.

**7. Cost and rollout.** An fsync per write is expensive (a real disk flush; can
be 1-10ms+ on spinning media, and non-trivial even on SSDs). Applied
unconditionally across ~12 call sites (every doc/archive write and every event
append) it would materially slow bulk operations — `internal/db/rehydration.go:372`
already comments on avoiding "O(n) fsyncs that make rehydration extremely slow."
Mitigations: (a) an **opt-in durability mode / config flag**
(`durable_writes`), defaulting off, so latency-sensitive bulk paths (rehydrate,
doctor sweeps, sync) are unaffected; (b) fsync only on **critical single-item
mutations** (the size seam, status transitions) rather than bulk regeneration;
(c) reuse the proven `fsutil.go` helpers rather than re-deriving the sequence.
Backward compatibility is high: on-disk formats are unchanged (append-only JSONL
and atomic rename are already forward/backward compatible), so no migration is
needed and rollback is a plain revert.

### What Was Tried and Failed

* **Full POSIX-parity power-loss durability on Windows** — rejected as
  infeasible. `FlushFileBuffers` on a directory handle returns
  `ERROR_ACCESS_DENIED` with no userspace workaround (verified via external
  sources). The best Windows can offer is file-content flush with best-effort
  dirent durability; a uniform cross-platform guarantee is not achievable.
* **Unconditional fsync-on-every-write as the default** — rejected on
  performance grounds. The repo already avoids O(n) fsyncs on the hot
  rehydration path (`internal/db/rehydration.go:372`); making every shared write
  synchronous by default would regress bulk operations. An opt-in mode is the
  correct posture.
* **Treating this as a size-seam-local fix** — rejected. Both writers are shared
  leaves with 12 production callers; the change is unavoidably repo-wide and must
  be its own reviewed release unit (as 120-F already scopes it).

### Remaining Unknowns

* Quantitative fsync latency on the target Windows/NTFS + SSD hardware
  (qualitative only here; a micro-benchmark is deferred to implementation).
* Whether parent-dir fsync should be applied to **all** POSIX atomic writes or
  only critical single-item mutations — a policy call best made with benchmark
  data.
* Exact default posture of the proposed `durable_writes` flag and its
  interaction with the MCP server vs one-shot CLI invocations.
* Whether `doctor` should gain an OS-crash reconciliation pass (currently
  out-of-scope per the size contract) to detect/repair orphan-write states, which
  would relax the durability requirement on the write path itself.

## Recommendation

**Conclusion**: proceed
**Confidence**: high

The design is feasible and the premise is fully verified against current code.
Proceed with the protocol **as its own future release unit** (not part of
v1.7.0), implemented by promoting the existing `internal/events/fsutil.go`
patterns into the two shared primitives, adding the missing POSIX parent-dir
fsync, and gating directory fsync on `runtime.GOOS != "windows"`. Ship it behind
an **opt-in `durable_writes` mode** (default off) so bulk paths keep their
current performance, and document the **platform-asymmetric guarantee** (full
POSIX power-loss durability; Windows file-content durability with best-effort
dirent). Handle fsync errors **fail-closed** on the critical single-item mutation
paths (size seam, status transitions) and best-effort on bulk regeneration.

**Release-gating verdict (v1.7.0): DO NOT hold the release.** The sync-free
behavior is a **pre-existing, documented durability *limitation* shared by the
entire repo write path**, not a *defect* introduced by the size feature:

* The sync-free design predates 108-F and is deliberate and documented
  (`atomicfile/doc.go:36-42`).
* 108-F's own shipped contract explicitly declares the seam "process-crash-safe
  only" with OS-level crash handling "out of scope"
  (`size-estimation-contract.md:104-106`).
* The size seam adds **no new** corruption vector: its event stream is advisory,
  the durable frontmatter value is the single source of truth, and orphan
  crash-residue events are already ignored on read
  (`size-estimation-contract.md:97-103`).
* Recovery for a lost `.backlogit/` write is git (Principle IX; the workspace is
  git-tracked), consistent with the atomicfile durability rationale.

The size feature is **safe to ship as-is** provided the OS-crash limitation stays
documented (it already is). This spike is the follow-up already promised in
099-S / Copilot G3, and it should be scheduled as an independent, de-prioritized
(120-F priority: low) release unit.

## Next Steps

1. Leave 120-F queued as its own release unit; do not fold it into v1.7.0.
2. When scheduled, promote to `impl-plan`: (a) refactor `AppendEvent` and
   `WriteFileAtomic` to the synced sequences, reusing `fsutil.go`; (b) add POSIX
   parent-dir fsync with the `runtime.GOOS != "windows"` gate; (c) add a
   `durable_writes` config flag (default off); (d) micro-benchmark fsync latency
   to finalize the critical-vs-bulk policy.
3. Add regression harnesses mirroring the existing 040-F/041-S harness pattern
   (`hook_checkpoint_harness_041_test.go`, `shipment_041_harness_test.go`) for
   the two shared writers, including a Windows directory-fsync skip assertion.
4. Update `atomicfile/doc.go` and the size contract to describe the new opt-in
   durable mode and the platform-asymmetric guarantee once implemented.

## References

Verified code:

* `internal/atomicfile/atomicfile.go:15-63` — `WriteFileAtomic`: temp -> Chmod
  -> Close -> Rename; no temp fsync, no parent-dir fsync.
* `internal/atomicfile/doc.go:36-42` — "Sync-free by design" rationale.
* `internal/events/stream.go:43-67` — `AppendEvent`; `OpenFile` at :60,
  `Fprintf` at :65; no `f.Sync()`.
* `internal/events/fsutil.go:12-33` (`syncAppendLine`), `:39-74`
  (`syncWriteFileAtomic`) — in-repo reference fsync patterns; both omit parent-dir
  fsync; Windows pre-remove at :66.
* `internal/core/artifact_size.go:124-167` — 108-F event-before-write seam;
  `AppendEvent` at :137, `WriteFileAtomic` at :165.
* `internal/db/rehydration.go:372` — comment on avoiding O(n) fsyncs on the hot
  path.
* Windows pre-remove idiom: `internal/atomicfile/atomicfile.go:52`.

`WriteFileAtomic` production callers: `internal/docline/service.go:349`;
`internal/core/archive.go:487`; `internal/core/artifact_size.go:165`;
`internal/core/doctor.go:605`, `:667`.

`AppendEvent` production callers: `internal/hooks/builtin_post.go:108`;
`internal/core/archive.go:284`; `internal/core/commits.go:50`, `:91`;
`internal/mcp/tools.go:590`; `internal/core/gate_evidence.go:61`;
`internal/core/shipment.go:308`.

Prior work:

* `docs/design-docs/2026-07-19-size-estimation-contract.md:88-106` — audit-event
  durability policy; "process-crash-safe only", OS-crash out of scope.
* `docs/archive/closure/2026-04-23-041-s-write-durability-closure.md` — 040-F /
  041-S write-durability shipment (added fsync to JSONL/checkpoint paths, not the
  shared writers).
* `docs/exec-plans/2026-04-22-write-durability-hook-reliability-plan.md` — 040-F
  plan.
* `.backlogit/queue/120-F.md` — this spike's work item and routing note.

External (Windows directory fsync):

* Microsoft/Go issue tracker — `(*os.File).Sync()` fails on directory handles on
  Windows (`ERROR_ACCESS_DENIED`).
* Microsoft Learn — `FlushFileBuffers` (fileapi.h): no directory-handle support.
* `renameio` and similar libraries — no-op directory fsync on Windows.

---
title: "Is crash-window exactly-once size mutation feasible for backlogit's size feature?"
source: docs/decisions/2026-07-23-crash-window-exactly-once-size-mutation-spike.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
description: "Feasibility spike (121-F) for crash-window exactly-once size mutation deferred from 099-S (Option B2): confirms none of the three prerequisites exist today, that exactly-once is achievable only by building substantial new machinery, and that it is NOT needed for correctness because the shipped durable-field-as-truth + fail-closed-read model is already safe — so v1.7.0 is not release-blocked."
docline:
    type: spike
    date: 2026-07-23
    time_box: "4h"
    conclusion: "defer"
    confidence: "high"
    linked_parent_work_item: "121-F"
    promoted_to: ["none"]
    tags:
        - "size-estimation"
        - "durability"
        - "exactly-once"
        - "crash-safety"
        - "doctor"
        - "cli-mcp-parity"
---

## Goal

Assess the **feasibility** of **crash-window exactly-once size mutation** for
backlogit's size-estimation feature (feature `108-F`, shipment `099-S`, already
merged), deferred from `099-S` per Copilot cycle-3 "Option B2" and filed as stash
`9D5BB492` → feature `121-F`.

Precise question: *Given the current architecture, is exactly-once semantics for
`custom_fields.size` mutation across a crash window achievable, and if so, what
would it require?* Sub-question (release-critical): *Does the currently-merged
108.x feature have any known crash-window data-loss / corruption / incorrect-size
defect that should block cutting release v1.7.0?*

## Success Criteria

A sufficient answer:

1. Confirms (with current code references) whether each of the three named
   prerequisites exists today:
   (a) stable transport-visible OpID ingress on CLI/MCP reaching the core size
   writer; (b) deterministic multi-orphan event ordering; (c) a reachable offline
   doctor reconciliation routed through the real `internal/core/doctor.go`
   `Doctor()` workspace scan.
2. Enumerates the concrete building blocks exactly-once would require, with rough
   scope/risk.
3. Determines whether exactly-once is even **needed** — i.e., whether the shipped
   durable-field-as-truth model is already correct.
4. Delivers a crisp release-gating verdict on v1.7.0 and a conclusion
   (proceed | pivot | defer | abandon) with a confidence rating.

## Scope Constraints

* **Feasibility assessment ONLY.** No implementation of exactly-once semantics.
* Read-only: no `*.go` edits, no prototype commits, no git mutations, no backlog
  mutations (121-F status untouched; read-only `backlogit query`/`get` only).
* Principle IV: no writes outside `C:\Source\GitHub\backlogit`. The only file
  created is this findings artifact.
* Time-box: ~4h-equivalent focused investigation.

## Investigation Approach

1. Read the spike skill; gather prior decisions on `099-S` / `108-F` / Option B2
   from `docs/design-docs/`, `docs/exec-plans/`, `docs/decisions/`, and archived
   backlog artifacts.
2. Verify prerequisite 1 (OpID ingress) by inspecting the size seam
   (`internal/core/artifact_size.go`), the CLI size surface
   (`internal/cli/update.go`), the MCP size surface (`internal/mcp/tools.go`), and
   a repo-wide search for any `op_id` / `operation_id` / `idempotency` /
   `PrevOpID` symbol.
3. Verify prerequisite 2 (event ordering) by inspecting the event append/read path
   (`internal/core/gate_evidence.go`, `internal/events/stream.go`,
   `internal/events/reader.go`) and the `estimate_history` event shape.
4. Verify prerequisite 3 (doctor reconciliation) by reading the full-workspace scan
   (`internal/core/doctor.go` `Doctor()`) and the single-file validator
   (`internal/core/doctor_target.go`), and tracing every caller of `DoctorTarget`.
5. Synthesize feasibility, necessity, and a release-gating verdict.

## Findings

### What Was Discovered

#### Prior work (the deferral is well-documented and deliberate)

* The **shipped durable contract** (`docs/design-docs/2026-07-19-size-estimation-contract.md:89-106`)
  states the model explicitly: *"Every size mutation appends an `estimate_history`
  event to the item log before the durable frontmatter write. The append is
  fail-closed… The durable `custom_fields.size` (with its provenance) is the single
  source of truth. The event stream is an advisory audit trail, not the authority.
  Orphan crash-residue events… are ignored on read… The policy is
  process-crash-safe only. OS-level crash reconciliation, exactly-once semantics,
  operation IDs, and doctor reconciliation are out of scope."*
* The **Option B2 descope** is recorded authoritatively in
  `docs/exec-plans/2026-07-18-108-F-size-estimation-impl-plan.md:1012-1048`. Cycle-3
  finding **H1+H5** removed the exactly-once ambition at the root rather than
  patching the recurring review magnet (F4 → G3 → H1):
  * H1: *"the seam generates the OpID internally and no CLI/MCP caller carries one,
    so a client retry submits a NEW id and the orphan is never deduplicated — the
    'exactly-once crash-window retry' claim is unsupported."*
  * H5: *"SE-3c's CAS reconcile is nondeterministic when two crash residues share a
    `PrevOpID`."*
  * Disposition: dropped OpID dedup, exactly-once, `size_op_id`, `PrevOpID`, OpID
    transport ingress; SE-3b (`108.006-T`) narrowed to best-effort
    append-before-write with fail-closed read; **SE-3c (`108.011-T`) removed and
    archived**; the full ambition filed as stash `9D5BB492` requiring "stable /
    transport-visible OpID ingress + deterministic multi-orphan ordering + reachable
    offline doctor via the real `internal/core/doctor.go` `Doctor()`, not
    `doctor_target.go`."
* The archived SE-3c task (`.backlogit/archive/108.011-T.md`) confirms the removed
  design owned `internal/core/doctor_target.go` reconciliation and depended on an
  OpID / `PrevOpID` chain and a `custom_fields.size_op_id` field that were all
  removed.

The 121-F stash text and the impl-plan agree precisely with what the code shows
below — the deferral note is accurate, not stale.

#### Prerequisite 1 — Stable transport-visible OpID ingress: **DOES NOT EXIST (confirmed)**

* The single size seam `SetArtifactSizeWithProvenance` takes a `SizeMutation`
  (`internal/core/artifact_size.go:34-39`) whose only fields are `Size`, `Source`,
  `RulesetVersion`, `Actor`. **There is no OpID / operation-id / idempotency-key
  field.**
* The `estimate_history` event delta assembled at
  `internal/core/artifact_size.go:127-136` carries only `actor`, `size`,
  `size_source`, `size_ruleset_version`. **No OpID and no `PrevOpID`.**
* Both transports construct the mutation with only those four fields and no
  operation id:
  * MCP: `internal/mcp/tools.go:782-806` reads only `size`, `size_source`,
    `size_ruleset_version` (arg allow-list at `tools.go:848-849`) and builds
    `core.SizeMutation{Actor: core.ActorContextAgent}`.
  * CLI: `internal/cli/update.go:106-119` reads only `--size`, `--size-source`,
    `--size-ruleset-version` and builds `core.SizeMutation{Actor: core.ActorContextHuman}`.
* A repo-wide search across `internal/` for `op[_-]?id | operation[_-]?id |
  idempoten | prevopid | prev_op` returns **zero** matches related to size
  mutation — every `idempoten*` hit is about migrations, links, checkpoints,
  telemetry, or DB schema, none of which is an operation-id on the size write path.

Conclusion: no operation-id / idempotency-key is accepted or propagated from any
CLI/MCP caller into the size write path. The deferral note's claim ("SE-5 exposes
none and no caller carries one") is **verified true**.

#### Prerequisite 2 — Deterministic multi-orphan event ordering: **NOT AVAILABLE for reconciliation (confirmed)**

* The `Event` struct (`internal/events/stream.go:16-24`) carries `Timestamp`
  (wall-clock `time.Now()`), `Actor`, `ItemID`, `EventType`, `Delta`, `CommitSHA`.
  **There is no sequence number, no monotonic counter, and no OpID / `PrevOpID`
  causal-chain key.**
* `AppendEvent` (`internal/events/stream.go:43-67`) appends one JSON line per event
  guarded by a **per-`EventWriter`-instance mutex** (`internal/events/stream.go:26-34`);
  `ReadAllEvents` (`internal/events/reader.go:26-55`) returns events
  in **file append order only** — no sort by timestamp, no de-dup, no causal
  reconstruction.
* Consequently:
  * The per-instance lock only serializes appends made through **one shared writer
    instance**. Core paths construct a **fresh `EventWriter` per append** (e.g.
    `appendItemEventWithActorErr`, `internal/core/gate_evidence.go:48-61`), so
    concurrent in-process appends are **NOT serialized** by this lock and there is
    **no process-wide, deterministic append order**. The on-disk order is whatever
    the OS interleaves; even within a single writer the recorded order is
    *insertion* order, not a *causal* order that identifies which orphan reflects
    the intended final durable state.
  * Wall-clock `Timestamp` is non-monotonic (clock skew/adjustment) and can collide
    at equal values, so it cannot serve as a reliable tie-breaking total order across
    a crash window.
  * There is no field linking an event to the specific durable write it was meant to
    accompany, so multiple orphan `estimate_history` events cannot be
    deterministically reconciled to "the last intended mutation." This is exactly the
    H5 nondeterminism the descope cited: *two crash residues sharing a `PrevOpID`*
    (which do not even exist today) could not be ordered.
* Additionally, the shipped read path does **not** consume `estimate_history` events
  for size at all — orphans are ignored on read (design contract
  `…size-estimation-contract.md:99-103`), so there is currently no reconciliation
  logic to be deterministic about.

Conclusion: trivial append ordering exists, but the **deterministic multi-orphan
reconciliation ordering** that exactly-once requires does not — the necessary causal
key (OpID/PrevOpID or a durable monotonic sequence) is absent.

#### Prerequisite 3 — Reachable offline doctor reconciliation via the real `Doctor()` scan: **DOES NOT EXIST; the only reconciliation-shaped entry point is disconnected (confirmed)**

* The full-workspace scan `Doctor()` (`internal/core/doctor.go:137-303`) performs a
  single canonical artifact scan (`scanCanonicalArtifacts`, `doctor.go:172`) feeding
  only three check families: orphaned-artifact (`doctor.go:204-282`), duplicate/root-ID
  collisions, and `archived_from` repair. A grep of `doctor.go` for
  `size | estimate | reconcil | EventEstimate` finds **no** size/estimate/orphan-event
  reconciliation logic and **no hook** where size reconciliation could run. "Orphan"
  in `doctor.go` means an orphaned *artifact* (no parent), never an orphan *event*.
* `internal/core/doctor_target.go` is **single-file validation-only**:
  `DoctorTarget` / `PrepareDoctorTarget` / `ValidateDoctorTargetResolved`
  (`doctor_target.go:112-226`) confine one path to the storage root, acquire the
  per-task lock, decode, and run header-def **required-field** validation. It performs
  **no** size reconciliation, no orphan-event replay, no CLEAR-recovery.
* Call-graph trace: the only callers of `DoctorTarget` / `PrepareDoctorTarget` /
  `ValidateDoctorTargetResolved` are the CLI `--target` mode
  (`internal/cli/doctor.go:97-98,174`), the MCP `target` argument
  (`internal/mcp/tools.go:1870`), and tests. **`internal/core/doctor.go`'s `Doctor()`
  scan never calls any of them.** The two doctor surfaces are fully disconnected: the
  scan (`Doctor()`) and the single-file validator (`doctor_target.go`) share no code
  path.

Conclusion: the deferral note's claim is **verified true** — `doctor_target.go` is
single-file validation-only and is never called by the `Doctor()` scan, and the real
`Doctor()` scan has no hook where size reconciliation could run. The reconciliation
entry point SE-3c would have needed (`108.011-T`) was removed and archived.

#### Is exactly-once even needed? The shipped model is already correct

The shipped design is **durable-field-as-truth + fail-closed-write-ordering +
orphan-ignored-read**:

* A mutation validates first (`artifact_size.go:120`), then appends the audit event
  fail-closed (`:137-139`), then does the atomic frontmatter write (`:165-167`), then
  best-effort re-upserts the SQLite index (`:175-179`).
* If the process dies **before** the audit append: nothing changed — the durable size
  is unchanged and correct.
* If it dies **after** append but **before** the durable write (or the write itself
  fails, `:143-147` fault-injection models this): an orphan `estimate_history` event
  remains, but the durable `custom_fields.size` is unchanged. On read the orphan is
  ignored, so the observed size is the last committed value — **correct, not
  corrupt**.
* If it dies **after** the durable write but before the index upsert: the canonical
  file is correct; the index is a derived cache reconciled by the next `sync`.

In every crash position the durable field is authoritative and internally consistent.
The only thing the system does **not** provide is automatic *replay* of an
un-committed intended mutation — i.e., a client that crashed mid-mutation must
**re-issue** the mutation. That re-issue is **state-idempotent** (re-setting the
same size converges to the same durable size/bytes value, per
`TestSetArtifactSize_Idempotent`, `internal/core/artifact_size_test.go:146`) **but
produces at-least-once audit records**: `SetArtifactSizeWithProvenance` appends an
`estimate_history` entry before **every** write, including a same-size retry
(`internal/core/artifact_size.go:124-139`), so a retry **duplicates** the
`estimate_history` audit record. The durable *state* is safe; only the audit trail
is at-least-once. That is a *retry-visibility* property, not a *correctness*
property. Size estimation is an advisory planning signal, not
transactional financial state, so **state-idempotent-with-at-least-once-audit** is
the appropriate and sufficient semantics.

### Feasibility: achievable, but it is a multi-part build with real risk

Exactly-once crash-window size mutation is **technically achievable** but requires
building four new, coupled pieces that do not exist today:

1. **OpID transport contract on CLI + MCP** (new public interface surface). Add a
   caller-supplied operation-id argument to the CLI size flags and the MCP size args,
   thread it through `SizeMutation` into the seam, and specify client-retry semantics
   (a retry MUST reuse the same OpID). *Scope: medium. Risk: medium-high — it is a
   permanent CLI/MCP contract expansion; agents/humans must generate and persist an
   OpID across a crash, which pushes correctness responsibility onto every caller.*
2. **Idempotent OpID-keyed write** in the seam: persist the applied OpID durably
   (e.g. a reserved `custom_fields.size_op_id`) and short-circuit a re-applied OpID as
   a no-op. *Scope: medium. Risk: medium — reserved-key discipline already exists, but
   adds a compare-and-swap step and a new durable field to the contract.*
3. **Deterministic multi-orphan ordering + reconciliation**: add a durable monotonic
   sequence or a `PrevOpID` predecessor chain to events, and a reconciler that
   reconstructs the exact intended mutation (including field-CLEAR) from event payload
   alone. *Scope: large. Risk: high — this is precisely the H5 nondeterminism that
   sank SE-3c; two same-predecessor residues remain ambiguous without a globally
   ordered log, and the event log is currently per-item append-only wall-clock JSONL
   with no sequence.*
4. **Wire reconciliation into the real `Doctor()` scan**: add a size-reconciliation
   pass to `internal/core/doctor.go`'s canonical scan under the per-task lock, distinct
   from the disconnected single-file `doctor_target.go`. *Scope: medium-large. Risk:
   medium — the scan is a shared hot path; a new mutating pass must respect locking,
   fail-closed reads, and determinism.*

Even fully built, exactly-once here is only *process-crash* exactly-once; true
power-loss/OS-crash durability is a separate deferral (stash `131CEAE4`,
impl-plan `:1047-1048`). The composite risk is dominated by piece 3 (the reconciler
determinism problem that already caused three review cycles).

### What Was Tried and Failed

* Searched for any latent OpID / idempotency-key plumbing that might already partly
  satisfy prerequisite 1 (in case the deferral note was stale). None exists — the
  `idempoten*` hits are all unrelated subsystems (migration, links, checkpoints,
  telemetry, DB schema). The negative result is itself confirmation.
* No prototype was attempted — out of scope (feasibility only), and the historical
  record (three Copilot cycles culminating in Option B2) already establishes the
  nondeterminism failure mode, so a prototype would add cost without changing the
  conclusion.

### Remaining Unknowns

* **Product demand.** Whether any consumer will ever require automatic replay of an
  un-committed size mutation (vs. idempotent client retry) is a product question, not
  a code question. No such requirement exists today.
* **Power-loss durability** (`fsync`/OS-crash) is a distinct, larger concern tracked
  separately (stash `131CEAE4`); this spike scoped only process-crash exactly-once.
* **Global event ordering.** If a future exactly-once effort proceeds, the design of a
  durable monotonic sequence (per-item vs. workspace-global) is unresolved and is the
  highest-risk open design point.

## Recommendation

**Conclusion**: defer
**Confidence**: high

All three prerequisites are confirmed absent in current code, matching the deferral
note exactly. Exactly-once is **achievable** but only by building an OpID transport
contract (CLI+MCP), an OpID-keyed idempotent write, a deterministic multi-orphan
reconciler (the high-risk piece that already failed review three times), and a new
reconciliation pass wired into the real `Doctor()` scan — a large, coupled effort with
a permanent public-interface expansion.

Critically, exactly-once is **not needed for correctness**. The shipped
durable-field-as-truth + fail-closed-write-ordering + orphan-ignored-read model is
already safe: every crash position leaves the durable `custom_fields.size`
authoritative and internally consistent, and re-issuing a mutation is naturally
idempotent. For an advisory size-**estimation** signal,
at-least-once-with-idempotent-retry is the correct semantics; exactly-once would add
substantial machinery and a caller-facing OpID burden for no correctness gain.

Therefore **defer** (keep stash `9D5BB492` / `121-F` parked): revisit only if
exactly-once crash-window semantics become an explicit product requirement — at which
point piece 3 (deterministic ordering) and the OpID transport contract should be the
first design gates.

### RELEASE-GATING VERDICT (v1.7.0)

**Do NOT hold v1.7.0. No NEW release blocker introduced by this work.** Scope of
this verdict: the **exactly-once size-*value* question**. On that axis the
currently-merged 108.x size feature has **no known incorrect-size or
size-value-corruption defect** — a crash at worst leaves an **orphan
`estimate_history` audit event that is ignored on read**, while the durable
`custom_fields.size` field remains authoritative and correct. Exactly-once is a
**future enhancement gated on a non-existent product requirement, not a
correctness gap**, and can ship later if actually required.

**Accuracy correction — a separate, pre-existing Windows data-loss window DOES
exist (do not over-claim "no data loss").** The shared `atomicfile.WriteFileAtomic`
removes the destination before retrying the rename on Windows
(`internal/atomicfile/atomicfile.go:52-60`): a process crash between the
`os.Remove(path)` and the successful retry can leave the canonical artifact
**missing**. `SetArtifactSizeWithProvenance` calls `WriteFileAtomic` directly, so
this affects the size seam and every canonical write. This is **pre-existing
shared-writer behavior, NOT introduced or worsened by v1.7.0 or the 053-DL MCP
change**, so v1.7.0 is no riskier than v1.6.0 on this axis and remains
**unblocked** — but the "no crash-window data-loss defect" phrasing is corrected:
the durable size *value* is safe from *this* spike's exactly-once concern, while
the atomicfile Windows remove-before-rename data-loss window is a real,
pre-existing defect tracked separately by feature **`123-F` (raised to priority
medium)**, whose scope now explicitly includes replacing that fallback with a
Windows atomic-replace / fail-closed strategy.

## Next Steps

* `121-F` is **complete** (spike concluded `defer`) and has been moved to `done`
  and archived; **no** follow-up work item was created (nothing to implement this
  cycle). `promoted_to: ["none"]` is therefore correct for this artifact.
* Cut v1.7.0 without waiting on exactly-once; this spike is the traceable evidence
  that 108.x is safe-to-ship as-is.
* **Trigger for any future work:** exactly-once crash-window mutation becomes a
  work item only if and when it is an explicit **product requirement**. At that
  point, open a NEW feature (do not reopen the archived `121-F`), promote this
  artifact to `impl-plan`, and design the durable global event ordering (piece 3)
  and the OpID transport contract (piece 1) first, since they gate the other two
  and carry the dominant risk.

## References

* `.backlogit/archive/121-F.md` — this spike's parent work item and deferral text.
* `docs/design-docs/2026-07-19-size-estimation-contract.md:89-106` — shipped audit
  durability policy (durable-field-as-truth; exactly-once explicitly out of scope).
* `docs/exec-plans/2026-07-18-108-F-size-estimation-impl-plan.md:1012-1048` — Option B2
  descope (H1/H5), SE-3c removal, deferred-ambition prerequisites.
* `docs/decisions/2026-07-18-108-F-wave7-staging-review-followups.md:83-87,123` —
  crash-window stash `9D5BB492` kept separate as a distinct failure class.
* `.backlogit/archive/108.011-T.md` — the removed SE-3c doctor-reconcile task
  (`doctor_target.go`, OpID/`PrevOpID`, CLEAR-recovery).
* `internal/core/artifact_size.go:34-39,120-181` — the single size seam;
  `SizeMutation` with no OpID; fail-closed event-before-write; atomic write; index
  upsert.
* `internal/cli/update.go:106-122` and `internal/mcp/tools.go:778-813,848-849` — CLI
  and MCP size surfaces; neither accepts or propagates an operation id.
* `internal/core/gate_evidence.go:44-70` — `appendItemEventWithActorErr` event append.
* `internal/events/stream.go:16-67` — `Event` shape (no sequence/OpID) and append-only
  writer; `internal/events/reader.go:26-83` — append-order read, no causal ordering.
* `internal/core/doctor.go:137-303` — `Doctor()` full-workspace scan (orphaned-artifact
  / duplicate / archived_from only; no size reconciliation hook).
* `internal/core/doctor_target.go:112-226` — single-file validator; callers only at
  `internal/cli/doctor.go:97-98,174` and `internal/mcp/tools.go:1870` (never `Doctor()`).
* `internal/core/artifact_size_test.go:146` — `TestSetArtifactSize_Idempotent`
  (re-issued mutation is idempotent → at-least-once-with-idempotent-retry is safe).

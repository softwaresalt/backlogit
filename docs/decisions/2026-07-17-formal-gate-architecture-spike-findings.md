---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Formal-gate architecture spike findings'
source: docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md
doc_type: decision
description: 'Time-boxed read-only findings for the formal PASS-only planning-gate trust/atomicity contract: a first-pass contract sketch across seven questions with a PIVOT conclusion at medium confidence.'
docline:
    type: spike
    date: 2026-07-17T00:00:00Z
    time_box: "2h"
    conclusion: "pivot"
    confidence: "medium"
    linked_parent_work_item: 105.001-T
    review_state: concluded
    charter: docs/decisions/2026-07-14-formal-gate-architecture-spike.md
    supersedes: "PR #239 (closed, unmerged) formal-gate implementation loop"
    tags:
        - governance
        - architecture
        - planning-gate
---

# Formal-gate Architecture Spike Findings

## Conclusion

**PIVOT — medium confidence.**

A coherent first-pass trust/atomicity contract sketch is achievable, but not on
the design that collapsed PR #239 (an "exact canonical plan digest, PASS-only"
gate whose review iterations churned through and ultimately **discarded** waiver /
reservation / session machinery — see "Why PIVOT" below). The current
substrate already supplies most of the right primitives through the shipped
**gate-evidence log architecture** (082-F/083-F). The correct direction is to
build the formal gate **on that existing log substrate** and close five specific
foundational gaps first — not to reintroduce a plan-digest + waiver contract.

This is a `pivot` (a materially different gate design is required) rather than a
`proceed` (the prior design is coherent enough to plan) or a `defer` (no coherent
contract emerged). The contract sketch below is coherent; the confidence is
`medium` because two of the five foundations — evidence authenticity and
transactional multi-mutation — each need their own bounded design decision before
implementation.

## Method

Read-only investigation within the 2-hour box, per charter
`docs/decisions/2026-07-14-formal-gate-architecture-spike.md`. Inspected the
gate-evidence, shipment-gate, codec, status, dependency, atomic-write, and
MCP/CLI-parity paths in `internal/`, plus the closed PR #239 record. No code,
schema, or CLI changes were made.

## Trust/atomicity contract sketch (seven questions)

### Q1 — Evidence trust / forgery model

**Current mechanism.** Gate evidence is **logs-only**: appended to per-item
JSONL event streams, never to frontmatter
(`internal/core/gate_evidence.go:13-18`, `internal/gateevidence/gateevidence.go:13-15`).
On the gated completion path the append happens **before** the durable status
write, and under `evidence_required` a failed append refuses the transition
(`internal/core/gate_transition.go:234-257`, where `completeGatePass` appends
evidence and refuses on error before calling `updateArtifactUngated`). The derived
`gate_evidence` SQLite table is an explicitly disposable projection rebuilt from
the logs (`internal/db/rehydration.go:264-269`). The single shared `Latest`
predicate selects the most recent `EventGateForced` (any `ran`) or
`EventGatePassed` with `ran==true`; a fail-open no-run pass is skipped
(`internal/gateevidence/gateevidence.go:69-120`). Task evidence events carry a
`gate_report_hash` (sha256 of the gate report, `internal/core/gate_evidence.go:71-79`)
and a `head_sha` **only when non-empty** (`internal/core/gate_transition.go:406-411`),
while shipment-level passing evidence records **neither**
(`internal/core/shipment_gate.go:490-499`) — so today these binding fields are
optional metadata, not guaranteed primitives. The broker executes the gate binary
argv-array only with an allowlisted `MinimalEnv`
(`internal/core/gate/runner.go:25-56`, `:103-127`) and validates the binary as a
bare PATH name; the broker itself performs no durable write
(`internal/core/gate/broker.go:41-42`).

**Gap.** Trust is **structural, not cryptographic**. Evidence events are stamped
`Actor: "backlogit"` for attribution only
(`internal/core/gate_evidence.go:40-48`); there is no signature, MAC, or
append-only hash-chain. A hand-authored or replayed JSONL record that fits the
`Latest` predicate is accepted as genuine.

**Contract requirement.** Define the threat boundary explicitly: the actor can
hand-edit the JSONL log, so any proof kept **only inside that log is forgeable** —
a hand-editor can rewrite a per-item hash-chain end-to-end, and a previously valid
chain segment stays replayable. A formal PASS-only gate therefore needs an
**authenticated proof anchored outside the log** (e.g. an HMAC or signature over a
canonical digest of the complete event, keyed by material the mutating actor does
not control, or a trusted head/freshness state persisted outside the item log)
**plus explicit anti-replay state** (a monotonic counter or nonce bound into the
proof). A bare append-only hash-chain is neither necessary nor sufficient on its
own — it is only one candidate integrity structure among the HMAC/signature
options above, which do not require an in-log chain at all.

### Q2 — Mutation-manifest replay / binding

**Current mechanism.** Member-to-shipment binding is by **terminal status +
log-derived evidence + git ancestry**, resolved against a single shipment head
(`internal/core/shipment_gate.go:336-343`). The member scan compares each
member's recorded `head_sha` to the shipment head and fails closed on an empty
head under enforcement, a malformed SHA, an ancestry timeout/error, or head drift
across the evaluation window (`internal/core/shipment_gate.go:466-618`,
`:62-110`). Ancestry uses `git merge-base --is-ancestor` under a bounded helper
timeout with argv-array + minimal env (`:47-82`, `:112-135`).

**Gap.** There is **no cryptographic binding of a manifest to the exact evidence
or plan state that authorized it** — only current-HEAD ancestry. A stale or
replayed manifest whose `head_sha` remains an ancestor of the current head is not
independently rejected.

**Contract requirement.** Bind the manifest to the evidence with a **new digest
over the complete authenticated event plus the exact manifest/plan state** — do
not reuse today's `EvidenceSHA`. That field is copied verbatim from
`delta["gate_report_hash"]` (`internal/gateevidence/gateevidence.go:46`, `:118-120`),
and that hash covers only the raw gate report
(`internal/core/gate_evidence.go:71-78`); it does not bind event type, `ran`,
`head_sha`, item identity, or plan state, so distinct or replayed events can share
the same value. A bare digest is **not** a binding on its own: an actor who edits
the manifest can recompute an ordinary hash over the still-valid signed event plus
the altered plan, so a self-covering digest still permits manifest substitution.
The binding must therefore be **covered by Q1's authenticity proof** — the
HMAC/signature is computed over a canonical payload that *includes* the
plan/manifest digest, and the proof itself is stored **outside** that payload so
the mutating actor cannot recompute it. Verified at ship time, this upgrades
replay resistance from "still an ancestor" to "same authenticated evidence and
plan state that authorized it."

### Q3 — Exact-byte / CRLF semantics

**Current mechanism.** `mdfront` preserves body bytes verbatim (including CRLF
line endings) but normalizes CRLF→LF **inside the frontmatter block** and emits
LF-only fences; its `Encode` `yaml.Marshal`s the frontmatter map, which yields
**deterministic sorted-key** output — encoding is byte-stable, pinned by
`TestEncode_SortedKeysStable` (`internal/mdfront/codec.go:66-85`,
`internal/mdfront/codec_test.go:108-122`). `internal/docline/codec.go:7-28`
forwards to it. The typed model path normalizes CRLF→LF across the **whole input**
before parsing and reconstructs with a `---\n\n` (double-newline) separator, also
`yaml.Marshal`-ing a sorted-key map (`internal/models/frontmatter.go:21-42`,
`:126-133`). `atomicfile` writes exact bytes without normalization
(`internal/atomicfile/atomicfile.go:15-63`). sha256 is used only on **derived
payloads** (gate reports, release binaries), never on raw markdown file bytes.

**Gap.** Key ordering is **not** the problem — both codecs emit sorted keys. The
real cross-platform instability is **line-ending and separator handling**:
`mdfront` preserves body line endings (a CRLF body stays CRLF) while canonicalizing
only the frontmatter block, whereas the typed path normalizes the entire input to
LF and emits a different (`---\n\n`) separator. The same logical content therefore
hashes differently depending on which codec last wrote it and whether the body
carried CRLF. Any formal digest built naively over on-disk bytes is therefore
**cross-platform-brittle** on the current substrate, and any scheme that embeds its
own digest inside the hashed file must define an explicit **exclude-the-digest-block**
canonicalization rule to avoid self-reference.

**Contract requirement.** Never hash file-on-disk bytes. Define **one canonical
serialization** (LF, sorted keys, explicit trailing-newline rule) and hash that
canonical form for all evidence and manifest binding — decoupled from the bytes
the OS wrote. Note that `gate_report_hash` does **not** already do this: it
SHA-256s the broker-provided report bytes directly, with no canonicalization
(`internal/core/gate_evidence.go:71-78`), so platform- or producer-dependent report
bytes can still hash inconsistently. The new canonicalizer must therefore also
cover gate reports, unless the report producer contract guarantees canonical bytes
at the source.

### Q4 — Status model reconciliation

**Current mechanism.** Item statuses are defined at
`internal/models/artifact.go:11-25`
(`queued/active/blocked/review/done/accepted/rejected/archived/shipped/abandoned`)
with validation at `:37`. Shipment statuses are a **separate** enum
(`queued/active/shipped/abandoned`) with restricted transitions
(`internal/core/shipment.go:22-34`, `:97-169`). Archive adds a **second status
rule**: it writes `status: archived` plus a helper `archived_status: <previous>`
and restores from that helper on unarchive (`internal/core/archive.go:214-217`,
`:252-270`). Terminality is computed differently again: the gate defaults terminal
to `done` (`internal/core/gate_transition.go:120-133`), while queue dependency
resolution uses the shared `TerminalStatuses` list
(`internal/core/blocking_cascade.go:14`, applied in
`internal/core/queue.go:406-413`) and shipment release uses a separate
`isTerminalReleaseStatus` predicate (`internal/core/shipment_lifecycle.go:897`).

**Gap.** `archived` is simultaneously a real persisted status and a temporary
marker; the gate-terminal, queue-terminal, and shipment-terminal sets are not the
same. A formal gate reading "is this item complete?" can get different answers
from different call sites.

**Contract requirement.** One **authoritative status taxonomy** with explicitly
named, context-specific predicates — "no-longer-blocking" for dependency resolution
(where `shipped`/`abandoned` stop blocking), "releasable" for shipment (only
`done`/`accepted`/`rejected`/`archived`), and the gate's configurable completion
target — rather than a single shared boolean that would collapse those distinct
questions; plus a single documented rule for how `archived` composes with
`archived_status`.

### Q5 — Dependency-type durability

**Current mechanism.** The SQLite `item_deps(item_id, depends_on, dep_type)`
schema **contains** a `dep_type` column (`internal/db/schema.go:262-269`) and stores
it on every row (`internal/db/dependencies.go`) — but that SQLite table is a
**disposable projection**, not a durable store. Rehydration rebuilds edges from
**markdown frontmatter, which carries only target IDs**: it reads
`artifact.Dependencies` (a `[]string`) and upserts with the target ID only, after
deleting all `item_deps` (`internal/db/rehydration.go:165-170`, `:217-223`). Core
confirms frontmatter is source of truth and that `dep_type` is **not durable
there**; `RemoveDependency` recovers the type only from the SQLite cache
(`internal/core/dependencies.go:10-23`, `:45-73`). `models.Artifact.Dependencies`
is `[]string` and cannot express per-edge type (`internal/models/artifact.go:45-47`).

**Gap.** `dep_type` lives **only in the disposable index**. After a sync/rehydrate
or an out-of-band branch change, every non-`blocks` edge (`relates_to`,
`parent_of`) collapses to `blocks`. A gate that reasons over dependency
*semantics* cannot trust them across rehydration.

**Contract requirement.** Promote `dep_type` into the markdown source of truth
(typed dependency objects rather than a bare ID list) so edge semantics survive
rehydration.

### Q6 — Partial core-mutation rollback

**Current mechanism.** File writes are atomic per file via temp+rename **on
POSIX**, where `rename(2)` atomically replaces an existing destination. On Windows
the helper falls back to `os.Remove(dest)` then rename
(`internal/atomicfile/atomicfile.go:11-63`), which is **not atomic**: between the
remove and a successful rename the destination is absent, so a crash in that
window loses the file rather than leaving either the old or the new bytes. The
ship path is **sequential, not transactional**: it persists the artifact and then
calls `LinkCommit` with no rollback wrapper
(`internal/core/shipment_lifecycle.go:345-357`). `LinkCommit` updates SQLite first,
then best-effort appends the JSONL event (warn-and-continue, still returns nil)
and does not rewrite markdown (`internal/core/commits.go:25-57`).

**Gap.** There is **no journaling or all-or-nothing guarantee across a multi-file /
multi-store mutation** (frontmatter + DB + logs). Atomicity holds only at the
single-file boundary — and even there the Windows remove-then-rename fallback
opens a crash window in which the destination is absent, so a formal rollback
contract must treat single-file replacement as non-atomic on that platform.

**Contract requirement.** A governed mutation that creates items + dependencies +
shipment membership needs a **journaled or idempotent-replay wrapper** so a
partial failure is detectable and recoverable, or the gate must treat multi-step
mutations as advisory-only.

### Q7 — CLI / MCP parity contract

**Current mechanism.** `.autoharness/backlog-registry.yaml` is the parity contract,
and `internal/cli/registry_parity_test.go` enforces **surface** parity: every live
MCP tool has a registry row and either a resolvable `cli_command` or
`mcp_only: true`; every `cli_command` resolves to a real cobra command; and
flag/positional parity holds (`registry_parity_test.go:152-267`, `:314-387`).
Intentional asymmetries are declared: MCP-only `log_telemetry` and `merge_sync`;
a CLI-only allow-list (`init`, `mcp`, `manifest`, `migrate`, `status`,
`docs classify`, `queue bulk-status`, `queue move`, `telemetry *`)
(`registry_parity_test.go:60-75`).

**Gap.** The parity test catches **surface drift but not semantic drift**. Two
logically "same" operations do different things per surface: CLI `update --commit`
writes only the frontmatter scalar (`internal/cli/update.go:146-148`) while MCP
`track_commit` calls `LinkCommit` and inserts `commit_links`
(`internal/mcp/tools.go:1298-1320`). MCP `update_item` already exposes and
implements `sections` and `size` (`internal/mcp/tools.go:57-70`, `:739-770`); the
genuine surface gap is the operator-only gate controls `--gate-base/--force-gates`,
which CLI `update` exposes but MCP `update_item` does not
(`internal/cli/update.go:43-300` vs `internal/mcp/tools.go:722-777`).

**Contract requirement.** For every **governed** operation, both surfaces must
route through one shared core function (so `update --commit` and `track_commit`
cannot diverge), and the parity contract must assert **behavioral** equivalence
for governed ops, not just flag presence.

## Why PIVOT

PR #239 collapsed because implementation and review-fix patching proceeded while
these foundations were open, so each review round surfaced a new architectural
contradiction — waiver schemas and reservation/session handles were added then
**discarded** across iterations, leaving a final "exact canonical plan digest,
PASS-only" design. The findings above show that even that final digest-based
contract is **not implementable on the current substrate** without first resolving
Q1–Q6 (no evidence authenticity, no manifest↔evidence binding, no single
canonicalizer). But they
also show the substrate **already has the right shape** in the gate-evidence log
architecture: logs-as-source-of-truth, append-before-commit ordering,
`evidence_required` fail-closed refusal, a single shared evidence predicate, and
optional `gate_report_hash` + `head_sha` metadata on task evidence events (which
the replacement contract must promote to mandatory authenticated fields). The
materially different (and
cheaper) design is to **extend that substrate** rather than build a parallel
plan-digest/waiver contract.

## Replacement contract direction

A fail-closed formal PASS-only gate is viable if, and only if, it:

1. keeps the per-item JSONL evidence log as the durable **event source**, while
   evidence **validity** additionally depends on trusted freshness/anti-replay
   state persisted outside the mutable log (Q1) — the log stays authoritative for
   events, but is not the sole arbiter of validity;
2. adds an **authenticity proof** to evidence events (Q1) and binds the
   authorizing evidence hash to the manifest (Q2);
3. hashes a **single canonical serialization**, never file-on-disk bytes (Q3);
4. reads completion through **one authoritative status taxonomy** with
   context-specific predicates (Q4);
5. reasons only over dependency semantics that are **durable in markdown** (Q5);
6. treats any multi-store mutation as advisory unless wrapped in a **journaled /
   idempotent** operation (Q6);
7. routes every governed operation through **one shared core function** with
   behavioral parity asserted across MCP and CLI (Q7); and
8. admits the current gate decision through a **dedicated formal-admission
   predicate**, distinct from the existing shared `Latest` evidence predicate
   (which accepts `EventGateForced` regardless of `ran` and keeps an earlier pass
   even after a later block or requeue) — the formal predicate must require an
   authenticated, non-forced **real** PASS and must treat any later block/requeue
   as invalidating a prior pass (Q1/Q4).

## Recommended bounded follow-up (Stage to harvest; not created here)

Per the charter's non-goals, no implementation backlog items are created by this
spike. Recommended decomposition into bounded (~2h) units for a follow-up plan:

* **F1 — Evidence authenticity primitive (Q1, Q2).** Decide the externally
  anchored proof (HMAC/signature keyed outside the log, plus anti-replay state);
  add the authenticated proof to the evidence event and a **new manifest↔evidence
  binding digest** (over the full event + plan/manifest state, not the reused
  `EvidenceSHA`) verified at ship time. *Needs its own micro-decision → medium
  confidence.*
* **F2 — Canonical serialization + hash (Q3).** One canonicalizer (LF, sorted
  keys, trailing-newline rule) reused by evidence and manifest hashing.
* **F3 — Authoritative status taxonomy (Q4).** One taxonomy with named
  context-specific predicates (no-longer-blocking / releasable / gate-target) used
  by gate, queue, and shipment; document `archived`/`archived_status` composition.
* **F4 — Durable dependency type (Q5).** Typed dependency objects in frontmatter
  so `dep_type` survives rehydration.
* **F5 — Journaled multi-mutation wrapper (Q6).** All-or-nothing (or
  idempotent-replay) envelope for governed create+link mutations. *Non-trivial →
  medium confidence.*
* **F6 — Governed-op parity hardening (Q7).** Introduce **one commit-association
  operation** that updates *all* representations (frontmatter scalar + `commit_links`
  + JSONL) and route both CLI `update --commit` and MCP `update_item(commit=…)` /
  `track_commit` through it — today `LinkCommit` writes only `commit_links` + JSONL
  (`internal/core/commits.go:25-56`) while the CLI writes only the frontmatter
  scalar; assert behavioral parity for governed ops.

Recommended ordering: F2 and F3 first (cheap, unblock F1); F1 next; F4/F6 in
parallel; F5 last.

## Residual / unresolved questions

* The exact authenticity mechanism is **not resolved** — deferred to F1's
  micro-decision. A keyless in-log hash-chain alone is **not** viable under the Q1
  threat model (an actor who can edit the log can rewrite the chain end-to-end); the
  viable options are HMAC/signature key management **or an externally persisted
  trusted chain head** anchored outside the mutable item log, per the Q1 contract
  requirement above.
* The journaled-mutation design (write-ahead journal vs idempotent replay) is
  **not resolved** — deferred to F5's micro-decision.
* Machine-waiver / ADVISORY admission remains explicitly out of scope (charter
  non-goal); it must not re-enter via the F-series.

## Evidence inspected

* Closed PR #239 (record of the divergence): retained/removed waiver, reservation,
  and session machinery; two blocked plan digests; `review_state: blocked`, formal
  review not run.
* `internal/core/gate_evidence.go`, `internal/core/gate_transition.go`,
  `internal/gateevidence/gateevidence.go`, `internal/core/gate/{broker,runner,probe}.go`.
* `internal/core/shipment_gate.go`, `internal/core/shipment_lifecycle.go`,
  `internal/core/shipment.go`.
* `internal/mdfront/codec.go`, `internal/docline/codec.go`,
  `internal/models/{frontmatter,artifact}.go`, `internal/atomicfile/atomicfile.go`.
* `internal/db/{schema,rehydration,dependencies}.go`,
  `internal/core/dependencies.go`, `internal/core/archive.go`,
  `internal/core/queue.go`.
* `.autoharness/backlog-registry.yaml`, `internal/cli/registry_parity_test.go`,
  `internal/cli/update.go`, `internal/mcp/tools.go`.

## References

* Charter: `docs/decisions/2026-07-14-formal-gate-architecture-spike.md`
* Closed PR #239: https://github.com/softwaresalt/backlogit/pull/239
* Gate-evidence lineage: features 082-F / 083-F (shipped).

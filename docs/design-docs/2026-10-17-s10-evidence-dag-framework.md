---
chunk_strategy: h1-h2-h3
description: 'Design note for the S10 shipment-level fault-line evidence DAG framework (144-S / 162-F): the DAG model, evaluation core, detector executors, authenticity checks, review-ready gate, signed waivers and tamper-evident ledger'
doc_type: design
title: "S10 Shipment-Level Evidence DAG"
source: docs/design-docs/2026-10-17-s10-evidence-dag-framework.md
schema_version: "1.0"
docline:
    date: 2026-10-17T00:00:00Z
    status: accepted
    tags:
        - fault-line
        - evidence-dag
        - review-ready-gate
        - waivers
---

# S10 Shipment-Level Evidence DAG

`internal/evidencedag` turns the known fault-line detectors (S4-S9) into a
machine-enforced shipment completeness proof. It is a library; wiring it into
shipment assembly or S11 is a follow-up, as for the S9 gate. Rollout is
report-only first (`ModeReportOnly`), then `ModeEnforce`.

## Model (U1, task 162.001-T)

A declaration (JSON or YAML, `schema_version: 1`) holds nodes and edges.
`Load` is strict: unknown fields, duplicate keys, multiple documents and trailing
data are refused; input is capped (256 KiB, 512 nodes, 4096 edges).

A node declares `id`, `family`, an explicit `applicability` predicate (data, never
code: `always: true` or `requires: {attribute: [values]}`), the `validator`
identity, optional `required_evidence`, `producing_task`, `producing_commit`
(`head` or a pinned SHA), `waiver_family`, `on_unmet` (`block` or `report`) and
`terminal`. Families: `parity`, `mutation-state`, `compat-error`, `docs-audit`,
`plan-task-soundness`, `red-test-honesty`. `Load` also refuses YAML anchors,
aliases and merge keys and compares object keys case-insensitively, so a
document cannot hide a duplicate from one parser that another would honour.

`Declaration.Validate` is deterministic and fail-closed: duplicate IDs, dangling,
self and duplicate edges, cycles (reported as `a -> b -> a`, rotated to the
smallest ID), unknown families, no terminal node, and nodes unreachable from any
terminal node. Output is sorted.

`Evaluator` evaluates in topological order (ties by ID) and sets each node to
`verified`, `unmet`, `waived`, `not-applicable` or `error` (unverifiable):

* Applicability resolves from the shipment's declared attributes. A required
  attribute the shipment does not declare is `error`, never `not-applicable`.
* A node verifies only with evidence that decodes and validates through
  `faultline.DecodeAndValidate`, whose `node_family` matches the family's S4
  evidence family, and whose envelope status is `pass`. `report_only` and `fail`
  are `unmet`.
* An unmet or erroring upstream propagates transitively; own evidence never
  masks it. A `not-applicable` upstream is neutral: it neither blocks nor
  satisfies its dependents, which evaluate on their own evidence and their
  other upstreams. A node that is `waived` is decided before its upstreams are
  consulted, so a waiver also masks that node's unmet or erroring upstreams.
* Blocking semantics (`on_unmet`) cannot be laundered. `report` is only valid on
  terminal nodes (a non-terminal node must declare `block`; `Validate` refuses
  anything else). When a node is unmet because an upstream is unmet, its
  effective `on_unmet` is `block` if its own declaration is `block` or any unmet
  upstream's effective value is `block`; only a report-mode terminal whose
  unmet upstreams are all report-mode stays advisory. `NodeResult.OnUnmet`
  carries the effective value and the roll-up uses it. Because `Compose` adds
  nodes only, a contributed report-mode terminal can neither weaken a base node
  nor hide a base blocker it depends on.
* `Report.Rollup` is `ready`, `blocked` or `unverifiable`. An empty set of
  applicable terminal nodes is `unverifiable`, never "all satisfied". `nil` and
  empty evidence maps are equivalent; a zero `Report` is never ready.
* Incremental evaluation caches per node, keyed by a SHA-256 over the node,
  shipment, head, resolved attributes, presented evidence/waiver and the keys of
  every dependency, so a changed upstream invalidates all transitive dependents.
  The cache is per `Evaluator`, in memory, keyed by shipment, head and node and
  bounded; there is no global state. The presence of an executor failure is part
  of the key, and a node whose inputs cannot be digested is never cached.
* `Input.Failures` follows the same rules as evidence and waivers: an undeclared
  node ID or an empty reason is rejected. `Node.RequiredEvidence` is only an
  assertion checked against the S4 family bound to `Family`; it adds no
  behaviour of its own.
* `Shipment.Attributes` are trusted-producer input: applicability is resolved
  from them verbatim, so whoever supplies them can decide which nodes apply.
  They must come from the shipment-assembly pipeline, never from an
  agent-writable source.
* `Report`, `NodeResult` and `Rollup` marshal with snake_case JSON tags and
  never emit `null` slices.

## Detector executors and binding integrity (U2, task 162.002-T)

One authoritative pattern: a `NodeExecutor` returns an S4 U4 artifact;
`RunExecutors` wraps it in a bound `EvidenceRecord` (shipment, node, head commit,
SHA-256 of the canonical artifact); the evaluator verifies it. Executors never set
statuses. Adapters:

* `RedHonestyExecutor` runs the S9 gate against the shipment head commit. It
  refuses a node with a pinned `producing_commit`, because the gate cannot judge
  any other commit; the node then fails as unverifiable.
* `ParityExecutor` wraps a **caller-supplied** `ParitySource` in the shared S4
  envelope. It does not run S4 detectors; it trusts the source and only enforces
  the shared contract on the wrapped artifact.
* `NewFuncExecutor` is the generic adapter.

U2 coverage is partial: only the `parity` and `red-test-honesty` families have an
adapter and an S4 evidence family. Nodes of every other family evaluate as
unverifiable until executors and S4 families exist for them.

A panic (in `Identity` or `Execute`), error, nil or typed-nil binding, identity
mismatch, non-conformant artifact or (with `RunExecutorsWith` and
`ExecOptions.NodeTimeout`) a timeout is a bounded, sanitized failure for that
node, never evidence. A timed-out executor that ignores its context is
abandoned and keeps running until it returns. A nil context is rejected. A run
with no applicable bound node is vacuous: it produces no evidence and no
failures, and the evaluator reports the nodes as unmet, so the gate is never
ready on an empty run.

Evidence "authenticity" here means **binding and digest self-consistency**, not
cryptographic provenance. The evaluator checks, before applicability is
consulted: record version; shipment, node and head binding; digest equals the
canonical artifact digest; producing task, producing commit (`head` or pinned)
and validator identity equal the node declaration. The same digest presented for
two nodes rejects both, including the legitimate node of the pair. Anything able
to write the evidence root (or to call `NewEvidenceRecord`) can mint a record that
passes every one of these checks. Evidence roots must therefore be writable only
by trusted producers; cryptographic evidence signing is a deferred follow-up.
Evidence on a non-applicable node that fails these checks is `error`, not
`not-applicable`.

`LoadEvidenceFile` accepts only a relative path of visible components: no `..`,
absolute, rooted, drive or stream forms; no `.`- or `_`-prefixed components; no
empty components in any mix of `/` and `\`; no control characters, Windows
reserved device names, trailing dots or spaces, or 8.3 short-name-like (`~N`)
components. The root is canonicalised, every component is `Lstat`ed through
`os.Root` (symlinks and reparse points refused), the target must be a regular
file and the opened descriptor must be the file that was inspected, the open is
non-blocking where supported, the read is capped and decoding is strict. A
time-of-check/time-of-use window remains for a writer that can swap directories
inside the evidence root.

## Gate, waivers and ledger (U3, task 162.003-T)

`Gate.Check` is the single authority. `Compose` is the documented extension
point: a contribution (for example S11 policy nodes) adds nodes and edges whose
source node it owns; contributed terminals join the same terminal set. Each
contributed node records its `Source`, a source may be supplied only once, the
base declaration is never mutated and the result is re-validated.

A waiver is an ed25519-signed token verified against a `Trust` loaded by
`LoadTrust` from a path outside the agent-writable roots. At least one forbidden
root is required (an empty list is an error), symlink leaves are refused and the
directory is resolved first. The token binds shipment, node, head commit, a nonce
and a validity window of at most 24 hours. It is denied unless: the key is
trusted and human-held (`automation` keys are always denied); the signature
verifies over the exact claims; the claimed principal equals the key's principal;
the policy explicitly allows that principal to waive the node's family; the
token is inside its window (the clock is read again under the ledger lock); the
kind is `waiver`; and the nonce was never consumed by a different token. The
`not-applicable` kind is denied as not implemented. There is no interactive or
environment shortcut and no issuance API: tokens are minted out of band.

`Waiver` has only unexported fields, sealed with a per-process HMAC at
acceptance. `WaiverService.Accept` is the only producer, and the evaluator
rejects a zero value or any copy whose node, shipment, head or principal was
altered, so a waiver cannot be retargeted. A waiver covers its node as a whole,
masking that node's unmet or erroring upstreams.

`Gate.Check` verifies every token, and checks input validity, node applicability
and one-waiver-per-node, before consuming any nonce; a token that cannot take
effect in that check is denied without being burned. A nonce is consumed when
the anchored ledger record is committed, even if the gate then reports the
shipment as not ready. Re-presenting the identical token is idempotent: the
ledger returns the existing record and the waiver is granted again until the
token expires; a different token with a consumed nonce is a replay.

The `Ledger` is a JSONL hash chain (`prev_hash`, `hash` over canonical bytes)
appended under a single-writer lock file, with the chain re-verified under the
lock, plus a `.head` sidecar. The lock file carries an owner token; unlock
verifies it still owns the lock; a lock older than 30 seconds is reaped by an
atomic rename to a unique name (put back if it turns out to be a fresh lock);
rename and remove errors are checked; and contention is distinguished from
genuine permission errors, with a bounded back-off that honours the 10 second
deadline. `Records` and `Head` take the same lock, so they never observe a
half-finished append (callers need write access to the ledger directory). A
write or sync failure truncates the file back to its pre-append size, and the
next append discards a single torn trailing line. The sidecar is written through
an exclusive random temp file and a rename retried with back-off; if it still
fails the record is committed and `ErrSidecarStale` is returned, and the next
append heals it. Ledger, sidecar, lock and trust files are opened beneath their
canonical directory through `os.Root`, refusing symlink and non-regular leaves.

Waiver acceptance is **anchored**. `NewWaiverService` requires an
`AnchorStore` (`FileAnchorStore` outside the forbidden roots, or
`MemoryAnchorStore`). Under the lock the ledger must equal the anchored head
exactly: a missing, deleted, rolled-back or extended ledger is `ErrLedgerTampered`
and the waiver is denied. With no anchor yet, only an empty ledger is accepted
(genesis); a populated ledger with no anchor is refused. `OpenLedger` requires
forbidden roots and refuses a ledger inside them. If the record commits but the
anchor cannot be advanced (`ErrAnchorStale`), the waiver is denied and anchored
appends stay closed until an operator inspects the ledger and runs
`Ledger.AdoptHead`. The one-record-behind sidecar tolerance remains, but it only
affects unanchored reads: anchored operations do not rely on the sidecar.

`Report.Text` and `Declaration.DOT` give deterministic, bounded, sanitized
output (control and bidi characters become spaces); truncated DOT output is still
a closed graph with a truncation comment. Versioned records are refused when
unsupported.

## Documented limitations

* Only `parity` (v1/v2) and `red-test-honesty` (`red_green`, v2) have S4 U4
  evidence families and executor adapters. Nodes of `mutation-state`,
  `compat-error`, `docs-audit` and `plan-task-soundness` evaluate as `error`
  (unverifiable) until the S4 manifest gains those families (the S8 soundness
  gate returns a decision, not an S4 artifact). This fails closed by design.
* Evidence is not cryptographically signed. Binding and digest checks prove
  self-consistency only: an actor who can write the evidence root can mint a
  passing record for the right node, task, commit and validator. The trust
  boundary is the producer pipeline; signing is a deferred follow-up.
* `Shipment.Attributes` are trusted-producer input for applicability.
* Identical artifacts for two nodes are rejected, which also fails the
  legitimate node of the pair; producers must make each node's artifact distinct
  (for example by scenario ID).
* The external anchor is only as strong as its storage: if an attacker can
  rewrite both the anchor and the ledger (or delete both, which reads as a fresh
  genesis), nothing is detected. Keep the anchor where the constrained party
  cannot write. A crash between the record commit and the anchor update fails
  closed until an operator re-anchors, and `AdoptHead` on a missing anchor trusts
  the current ledger on first use. A torn tail is discarded by the next append,
  including a deliberately appended unterminated fragment.
* Stale-lock reaping is best effort: a holder paused past 30 seconds can lose its
  lock. Any resulting fork or lost lock is caught by chain verification and
  anchoring on the next operation, which fails closed; there is no heartbeat.
* There is no key revocation list: remove the key from the trust file.
* `LoadTrust`, `OpenLedger` and `NewFileAnchorStore` can only exclude the roots
  the caller passes; they cannot discover agent-writable locations. Ancestors of
  the configured directory are trusted and resolved. Expiry uses the verifier
  host's clock.
* Directory swaps inside an evidence root between inspection and open are not
  fully closed (no `openat`-style single-call component walk exists in the
  standard library). Junction and reparse-point detection relies on
  `fs.ModeIrregular` and `os.OpenRoot`, so it needs Go 1.24 (the module's
  minimum).
* `RunExecutorsWith` timeouts abandon, but cannot kill, an executor that ignores
  its context.
* The cache is in-memory per `Evaluator`; it is not persisted.

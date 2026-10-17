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
`plan-task-soundness`, `red-test-honesty`.

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
  masks it.
* `Report.Rollup` is `ready`, `blocked` or `unverifiable`. An empty set of
  applicable terminal nodes is `unverifiable`, never "all satisfied". `nil` and
  empty evidence maps are equivalent; a zero `Report` is never ready.
* Incremental evaluation caches per node, keyed by a SHA-256 over the node,
  shipment, head, resolved attributes, presented evidence/waiver and the keys of
  every dependency, so a changed upstream invalidates all transitive dependents.
  The cache is per `Evaluator`, in memory; there is no global state.

## Detector executors and authenticity (U2, task 162.002-T)

One authoritative pattern: a `NodeExecutor` returns an S4 U4 artifact;
`RunExecutors` wraps it in a bound `EvidenceRecord` (shipment, node, head commit,
SHA-256 of the canonical artifact); the evaluator verifies it. Executors never set
statuses. Adapters: `RedHonestyExecutor` (S9 gate), `ParityExecutor` (S4 parity
detectors) and the generic `NewFuncExecutor`. A panic, error, identity mismatch or
non-conformant artifact is a bounded failure for that node, never evidence.

Authenticity runs before applicability is consulted: record version, shipment,
node and head binding; digest equals the canonical artifact digest; producing
task, producing commit (`head` or pinned) and validator identity equal the node
declaration. The same digest presented for two nodes rejects both. Evidence on a
non-applicable node that fails authenticity is `error`, not `not-applicable`.

`LoadEvidenceFile` accepts only a relative path of visible components: no `..`,
absolute, rooted, drive or stream forms, no `.`- or `_`-prefixed components. The
root is canonicalised, every component is `Lstat`ed through `os.Root` (symlinks
and reparse points refused), the target must be a regular file, the open is
non-blocking where supported, the read is capped and decoding is strict.

## Gate, waivers and ledger (U3, task 162.003-T)

`Gate.Check` is the single authority. `Compose` is the documented extension
point: a contribution (for example S11 policy nodes) adds nodes and edges whose
source node it owns; contributed terminals join the same terminal set. The base
declaration is never mutated and the result is re-validated.

A waiver is an ed25519-signed token verified against a `Trust` loaded by
`LoadTrust` from a path outside the agent-writable evidence roots (symlinks
resolved). The token binds shipment, node, head commit, a nonce and a validity
window of at most 24 hours. It is denied unless: the key is trusted and
human-held (`automation` keys are always denied); the signature verifies over the
exact claims; the claimed principal equals the key's principal (self-asserted
identity is denied); the policy explicitly allows that principal to waive the
node's family; the token is inside its window; and the nonce was never consumed
by a different token. Re-presenting the identical token is idempotent. The only
constructor of a verified `Waiver` is `WaiverService.Accept`; the evaluator
refuses any other. There is no interactive or environment shortcut, and the
library ships no issuance API: tokens are minted out of band.

The `Ledger` is a JSONL hash chain (`prev_hash`, `hash` over canonical bytes)
appended under a single-writer lock file with the chain re-verified under the
lock, plus a `.head` sidecar. `Records` detects edited, deleted, reordered and
truncated records, appended garbage, partial lines and a missing sidecar; an
`Anchor` retained elsewhere also detects truncation when both files are rewritten.

`Report.Text` and `Declaration.DOT` give deterministic, bounded text and Graphviz
output. `SchemaVersion`, the trust, token and ledger records each carry a version
and unsupported versions are refused.

## Documented limitations

* Only `parity` (v1/v2) and `red-test-honesty` (`red_green`, v2) have S4 U4
  evidence families. Nodes of `mutation-state`, `compat-error`, `docs-audit` and
  `plan-task-soundness` evaluate as `error` (unverifiable) until the S4 manifest
  gains those families; no executor adapter exists for them (the S8 soundness
  gate returns a decision, not an S4 artifact). This fails closed by design.
* Evidence artifacts are not cryptographically signed by S4. Authenticity here
  is binding, digest, producer identity and anti-replay consistency; an actor
  who can write a self-consistent record for the right node, task, commit and
  validator is not stopped. The trust boundary is the producer pipeline.
* Identical artifacts for two nodes are rejected; producers must make each
  node's artifact distinct (for example by scenario ID).
* Waiver and ledger tamper evidence cannot detect truncation when an attacker
  rewrites both the ledger and its sidecar and no external `Anchor` is retained;
  with a single record the sidecar may be absent (crash window). A lock file
  older than 30 seconds is reclaimed. `Records` does not take the lock and
  tolerates a sidecar one record behind. No key revocation list: remove the key
  from the trust file.
* `LoadTrust` can only exclude the roots the caller passes; it cannot discover
  agent-writable locations itself. Expiry uses the verifier host's clock.
* A waiver covers its node as a whole, including unmet upstream nodes, which
  remain separately gating only if they are terminal.
* Junction detection relies on `fs.ModeIrregular` (Go 1.23 and later).
* The cache is in-memory per `Evaluator`; it is not persisted.

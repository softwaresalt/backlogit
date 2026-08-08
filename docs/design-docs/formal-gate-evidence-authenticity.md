---
chunk_strategy: h1-h2-h3
description: "Design for backlogit's HMAC-authenticated formal gate evidence primitive: a domain-separated proof envelope, an externally anchored enforcement requirement, a dedicated formal-admission predicate, and fail-closed manifest binding at ship time."
doc_type: design
docline:
    date: 2026-08-07T00:00:00Z
    status: accepted
    tags:
        - gate-broker
        - evidence-authenticity
        - hmac
        - manifest-binding
        - security
schema_version: "1.0"
source: docs/design-docs/formal-gate-evidence-authenticity.md
title: "Formal Gate Evidence Authenticity and Manifest Binding"
---

# Formal Gate Evidence Authenticity and Manifest Binding

## Overview

The pre-task-completion gate broker
(`docs/design-docs/2026-07-04-pre-task-completion-gate-broker.md`) records
gate outcomes as plain JSONL evidence events. That evidence is only as
trustworthy as the log it lives in: anyone with write access to
`.backlogit/logs/` can hand-edit a `pre_task_completion_gate_passed` event
into existence. Formal gate evidence (106-F F1) closes that gap with an
**optional, backward-compatible** HMAC-authenticated proof bound into every
gate evidence event, plus a manifest-binding digest for shipment-level
evidence, so a verifier can distinguish "the gate broker actually ran and
produced this evidence" from "someone wrote a plausible-looking JSON line."

Formal admission is opt-in: a workspace that never sets
`BACKLOGIT_GATE_EVIDENCE_KEY` and never enables `formal_gate` in config sees
byte-identical evidence to today. Every unit below is additive.

## Envelope Field Names and Formats

The signed unit is `gateproof.Envelope` (`internal/gateproof/gateproof.go`),
serialized via `internal/canonical.Canonicalize` before HMAC-SHA256:

| Field | Type | Purpose |
|---|---|---|
| `magic` | string | Protocol constant (`backlogit.gate-evidence.v1`); prevents cross-protocol reuse of a MAC. |
| `purpose` | string | `task` or `shipment`; prevents cross-purpose replay (a task proof can never pass as a shipment proof). |
| `schema` | int | Envelope schema version (currently `1`); an unknown version is rejected rather than partially trusted. |
| `alg` | string | MAC algorithm identifier (`HMAC-SHA256`). |
| `key_id` | string | Non-secret identifier for the active key, so key rotation is auditable without ever carrying key material. |
| `workspace_id` | string | Trusted workspace identity, derived deterministically from the absolute workspace root path (`internal/core.workspaceIdentity`) — no new persisted state. |
| `item_id` | string | The task/subtask ID (`purpose: task`) or shipment ID (`purpose: shipment`). |
| `event_type` | string | The gate event type (`pre_task_completion_gate_passed`, etc.). |
| `ran` | bool | Whether the gate actually executed (mirrors the existing evidence `ran` field). |
| `actor` | string | Always `backlogit` today (the evidence appender's fixed actor). |
| `timestamp_utc` | string | RFC 3339 UTC timestamp. **Audit data only — never used for ordering or replay decisions.** |
| `head_sha` | string | The resolved HEAD SHA at evidence time. |
| `report_digest` | string | `internal/canonical.Hash` of the **validated** formal report (task proofs) or empty (shipment proofs, which bind `manifest_digest` instead). |
| `counter` | int64 | Monotonic per-item counter (rollback/duplicate detection — see Anti-Replay below). |
| `manifest_digest` | string | **Required** for `purpose: shipment`, **forbidden** for `purpose: task` — see Manifest Binding below. |

Delta persistence: every envelope field that is not already derivable from
the base evidence delta (`ran`, `head_sha`) or supplied by the verifier as
context (`workspace_id`, `item_id`) is persisted verbatim in the event delta
— `proof`, `key_id`, `proof_schema`, `counter`, `timestamp_utc`,
`report_digest`, and (shipment only) `manifest_digest` — so a verifier can
reconstruct the exact signed envelope later.

Reconstruction strictness: `key_id`, `timestamp_utc`, and `ran` are written
unconditionally by every signing path, so a persisted event missing any of
them (or holding the wrong type) can only be unsigned, corrupted, or
tampered — reconstruction refuses with `ErrProofUnverifiable` rather than
silently defaulting to a zero value. `head_sha` and `report_digest` remain
legitimately absent for genuine no-repo / non-`EventGatePassed` outcomes, so
their outright absence is not refused — only a present-but-wrong-typed value
is, since a genuine signer never writes anything but a string for either.

## Domain Separation Rules

* `magic` + `purpose` + `schema` are bound inside the MAC, not just checked
  out-of-band, so a proof cannot be reinterpreted under a different protocol
  version or purpose by an attacker who controls the surrounding JSON.
* `workspace_id` and `item_id` are supplied by the **verifier**, not read back
  from the delta, when reconstructing the envelope for verification
  (`gateevidence.FormalContext`). A proof's MAC only proves it was signed by
  someone holding the key — it does not by itself prove it belongs to the
  item/workspace being checked. Binding the verifier's own expected values
  into the reconstructed envelope means a proof valid for one item can never
  be accepted as evidence for a different item, and a proof from one
  workspace can never be replayed into another.
* `EventGateForced` is **never** formally admissible, regardless of proof
  validity — the formal-admission predicate (`gateevidence.FormalAdmit`)
  only ever considers `EventGatePassed` candidates.

## Key Provisioning

The HMAC key is resolved **exclusively** from the `BACKLOGIT_GATE_EVIDENCE_KEY`
environment variable (`config.ResolveFormalGateKey`) — never from workspace
config or a CLI flag; `config.FormalGateConfig` has no key field at all. The
value must decode as strict base64 (standard encoding) or hex to at least 32
bytes. Encoding precedence is deliberate: a value matching the hex charset
with an even length is always decoded as hex (hex is a strict subset of the
base64 alphabet, so an unpadded hex-shaped string could otherwise decode
successfully — and differently — under both encodings).

`key_id` (workspace config, non-secret) is bound inside the MAC so a future
key rotation is auditable: a verifier can tell which key era produced a given
proof without the key value itself ever appearing in a log, error string, or
persisted artifact.

## Enforcement Anchor

Enforcement is anchored **outside** the workspace via
`BACKLOGIT_FORMAL_GATE_REQUIRED` (`config.FormalGateEnforced`), which is
authoritative. Workspace config (`formal_gate.enabled`) may only **raise**
strictness, never lower it: when the environment anchor requires
enforcement, an explicit `formal_gate.enabled: false` in config is ignored
and enforcement still applies. This prevents a compromised or misconfigured
in-workspace config from silently disabling authentication that an operator
or CI pipeline requires.

## Unknown-Key Behavior

* **Key absent** (`BACKLOGIT_GATE_EVIDENCE_KEY` unset or empty): resolution
  returns `ErrGateKeyAbsent`. Under enforcement this refuses the transition
  entirely — there is no unauthenticated fallback.
* **Key invalid** (present but fails to decode, or decodes short): resolution
  returns `ErrGateKeyInvalid`, refusing identically under enforcement.
* **Wrong key at verify time**: `gateproof.Verify` computes the expected MAC
  under the verifier's own key and compares with `hmac.Equal`; a mismatch
  (wrong key, or any tampered field) returns `ErrProofInvalid`. There is no
  partial-trust state — verification is binary.

## Fail-Closed Matrix

| Condition | Outcome |
|---|---|
| Formal admission not enforced (no env anchor, config disabled) | Evidence delta byte-identical to pre-F1 behavior; no proof attempted. |
| Enforced, key absent/invalid | Refuse the transition (`ErrFormalGateRequired` wrapping `ErrGateKeyAbsent`/`ErrGateKeyInvalid`); no status write. |
| Enforced, `EventGatePassed` report fails schema validation | Refuse (`ErrFormalGateRequired` wrapping `ErrFormalReportInvalid`) — a bare exit-0 pass with an empty/non-conforming report is not sufficient evidence for a formal proof. |
| Enforced, shipment gate broker nil/unwired | Refuse (`ErrFormalGateRequired`) rather than silently preserving the pre-gate ship path. |
| Enforced, shipment gate auto fail-open | Refuse (`ErrFormalGateRequired`) rather than silently skipping member-evidence/shipment-diff enforcement. |
| Verification: tampered field, wrong key, replayed/non-maximal counter, or a later block/requeue/escalate after the candidate pass | Refuse (`ErrProofInvalid`) — definitively wrong. |
| Verification: missing proof fields, or canonicalization failure | Refuse (`ErrProofUnverifiable`) — could not be evaluated at all. |
| Verification: manifest changed since a shipment proof was signed | Refuse (`ErrProofInvalid`) — the recomputed `manifest_digest` no longer matches. |

Every refusal above surfaces through the MCP structured error contract
(`internal/mcp/formal_gate_errors.go`) with a stable `error` code,
`retryable: false`, and specific remediation — including the exact missing
environment variable name and the 32-byte minimum when the cause is
key-related.

## Guarantee Statement (honest, load-bearing — verbatim from the plan)

> The counter provides **rollback and duplicate detection within an intact
> log**, not unconditional anti-replay. The mutating actor can delete or
> truncate log records, so an actor who removes later block/requeue events
> can lower the observed counter floor. Full anti-replay requires a
> high-water anchor **outside** the actor's write set; this plan therefore
> supports an **optional verifier-side high-water ledger** (path supplied by
> environment, e.g. held by CI) and enforces it strictly when configured.
> When it is not configured, the guarantee is exactly the narrower one stated
> in this paragraph and must be documented as such. No unit in this plan may
> claim more.

**A symmetric key means CI cannot be verify-only.** Because the same HMAC key
both signs and verifies, any party capable of verifying a proof is also
capable of forging one. This is an accepted, explicitly documented tradeoff
for v1 (see the source deliberation), not an oversight: there is no
asymmetric-signature primitive in this design. A CI system that only needs to
*verify* still holds a key that could sign, so key distribution to CI is a
trust decision, not a cryptographic guarantee of read-only capability.

## Anti-Replay Counter and Locking

The counter is allocated under a heartbeat-refreshed advisory lock
(`internal/core.nextGateEvidenceCounter`, reusing the same
`lockTaskFileWithHeartbeat` mechanism the long-lived per-task completion lock
uses), so two concurrent transitions can never allocate — and therefore sign
and append — the same counter for the same item. A plain fixed-TTL sidecar
with no heartbeat was tried first and rejected: it could be reclaimed out
from under a holder whose sign+durable-append step legitimately stalled past
the TTL, letting a second caller rescan the same max counter and allocate the
identical value. The heartbeat refreshes the sidecar's ModTime on a fixed
interval strictly under the stale-TTL for as long as the holder is alive, so
a live holder's lock is never mistaken for crash residue regardless of how
long the guarded sequence takes.

The formal-admission predicate additionally requires the candidate's counter
to be strictly greater than every *other* **`EventGatePassed`** counter
recorded anywhere in the same event log (the intact-log guarantee) and, when
an external high-water ledger is configured, strictly greater than that
ledger value too. Only other `EventGatePassed` events count toward this
floor: `EventGateForced` is documented as never itself admissible and does
not invalidate a prior genuinely-signed pass either (only a later
`EventGateBlocked`/`EventGateRequeued`/`EventGateEscalated` does), yet a
Forced completion is itself signed and therefore legitimately and
unavoidably receives a *higher* counter than any earlier pass simply by being
later in time — counting it toward the floor would make every pass followed
by any later forced completion permanently unadmissible. **backlogit only
ever reads the ledger for comparison; it never writes to it** — updating the
ledger after a successful admission is the external verifier/service's
responsibility, so no out-of-workspace write ever originates from this
codebase (Constitution Principle III/IV).

## Manifest Binding (Shipment-Purpose Envelopes)

`manifest_digest` is `internal/canonical.Hash` over the shipment's **ordered**
manifest membership, its derived covering feature, and the resolved shipment
head (`internal/core.computeManifestDigest`) — reordering members, dropping a
member, or swapping the covering feature each change the digest. It is bound
into the `purpose: shipment` envelope at gate-pass time
(`augmentShipmentDeltaWithFormalProof`) and, additively to the existing
`head_sha` ancestry and head-drift guards (both unchanged), re-verified from
live state immediately after signing (`verifyShipmentManifestBinding`),
refusing on either `ErrProofInvalid` (manifest changed) or
`ErrProofUnverifiable` (proof missing or malformed) — never silently skipped.

Membership mutation (`AddItemToShipment`, `ReturnBlockedItem`) and the
locked window `ShipShipment` holds while re-checking and signing the manifest
share one advisory lock, `internal/core.lockShipmentMembership`, extended
through the shipment's own status transition so no unlocked window remains
for a concurrent membership change to ride inside a signed proof. The lock
key is a stable synthetic path derived only from the workspace root and the
shipment ID — never the shipment's current markdown file path, which
`persistArtifact` relocates on some status transitions (always at archival)
— and is validated to stay contained within the dedicated
`.backlogit/.locks` directory before use, since the shipment ID reaching this
function is caller-controlled and not yet validated by an upstream
`GetShipment` call.

## Formal-Admission Predicate

`gateevidence.FormalAdmit` (distinct from the existing `gateevidence.Latest`,
which accepts `EventGateForced` regardless of `ran` and keeps an earlier pass
even after a later block/requeue) admits a candidate only when **all** of the
following hold:

1. it is the chronologically latest `EventGatePassed` event;
2. no `EventGateBlocked`/`EventGateRequeued`/`EventGateEscalated` event
   appears after it;
3. its `ran` field is `true`;
4. its proof verifies against the verifier-supplied workspace/item context
   and key;
5. its counter is strictly greater than every other `EventGatePassed`
   counter in the log, and the external high-water ledger value when
   configured.

At ship time, `internal/core.validateMemberGateEvidence` reassigns its
lineage-check event to `FormalAdmit`'s own `res.Event` under enforcement,
never the legacy `Latest`-selected event: `Latest` prefers whichever
qualifying event is chronologically latest (Forced included), so a later,
completely unsigned `EventGateForced` carrying an arbitrary forged `head_sha`
could otherwise silently override the real, cryptographically-verified
commit binding for the ancestry check — making lineage pass for a commit that
was never actually authenticated.

## Scope Boundaries

Out of scope for 106-F F1 (unchanged from the reviewed plan): waivers,
reservations, session handles, and any speculative verifier abstraction
beyond the single `Sign`/`Verify` pair. Key rotation ergonomics beyond a
MAC-bound `key_id` are deferred until a second key exists. Provisioning an
external high-water ledger is an operations decision, not a code blocker.

## Related Reading

* `docs/design-docs/2026-07-04-pre-task-completion-gate-broker.md` — the
  underlying gate broker this proof authenticates evidence for.
* `docs/decisions/2026-08-07-f1-evidence-authenticity-mechanism-deliberation.md`
  — the source deliberation and micro-decisions.
* `docs/exec-plans/2026-08-07-f1-evidence-authenticity-manifest-binding-plan.md`
  — the reviewed implementation plan (F1, units U1–U9).
* `docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md` —
  the authoritative research spike this plan implements.

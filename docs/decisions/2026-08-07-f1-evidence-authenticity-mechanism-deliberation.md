---
title: "F1 evidence authenticity mechanism and key custody"
description: "Micro-decision for formal-gate unit F1: which authenticity mechanism, which key custody model, and which anti-replay state make an authenticated gate-evidence proof implementable on the current substrate without unsafe secret storage."
source: docs/decisions/2026-08-07-f1-evidence-authenticity-mechanism-deliberation.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
topic: "F1 (Q1+Q2): the externally anchored authenticity proof, its key custody, and its anti-replay state for formal-gate evidence and manifest binding"
depth: "deep"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-08-07-f1-evidence-authenticity-manifest-binding-plan.md"
  - "docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md"
tags:
  - "governance"
  - "formal-gate"
  - "security"
  - "evidence"
  - "shipment"
  - "stage"
---

## Problem Frame

Formal-gate spike `docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md`
concluded PIVOT at **medium** confidence specifically because unit **F1** was left
open: "the exact authenticity mechanism is **not resolved** — deferred to F1's
micro-decision." F1 cannot be planned or harvested until that decision exists,
and planning it while the question is open is exactly the failure mode that
collapsed PR #239.

### What must be decided

1. **Mechanism** — what makes an evidence event unforgeable (Q1).
2. **Key custody** — where the keying material lives, given that the mutating
   actor has the operator's filesystem permissions (Q1).
3. **Anti-replay state** — what stops a previously valid event from being
   re-presented (Q1).
4. **Manifest binding** — what digest binds a shipment manifest to the exact
   authenticated evidence that authorized it (Q2).

### Constraints

- **Constitution III / IV** — all state must resolve inside the workspace root;
  no out-of-tree writes.
- **Principle VI** — no speculative dependencies. Prefer the Go standard library.
- **Operator policy** — reliability and security first; simplicity over
  complexity; fail closed rather than choose unsafe secret or key storage.
- **Substrate** — evidence is logs-only, appended before the durable status write,
  with `evidence_required` fail-closed refusal already in place
  (`internal/core/gate_transition.go:223-249`).
- **F2 and F3 already shipped** — `internal/canonical.Canonicalize` / `.Hash` and
  `internal/core/status_taxonomy.go` predicates exist and must be reused.

### Success criteria

- A gate-evidence event carries a proof that a *verifier without write access to
  the log* can check.
- A replayed or hand-authored event is rejected, not accepted.
- A shipment manifest cannot be substituted under a still-valid proof.
- The absence or malformation of key material under enforcement **refuses**; it
  never silently degrades to unauthenticated evidence.
- No secret is ever written into the workspace, the config file, the index, the
  JSONL logs, or any log line.

### Explicitly out of scope

Machine waivers and ADVISORY admission (charter non-goal — must not re-enter via
the F-series). Multi-tenant key distribution. Key rotation tooling beyond
accepting a key identifier. Remote attestation services.

## Research Findings

### Substrate facts (HEAD, verified this session)

| Fact | Evidence |
|---|---|
| Evidence events are stamped `Actor: "backlogit"` for attribution only; no signature, MAC, or chain | `internal/core/gate_evidence.go:35-65` |
| `EvidenceSHA` is copied verbatim from `Delta["gate_report_hash"]`, which SHA-256s raw report bytes with no canonicalization | `internal/gateevidence/gateevidence.go:114-120`, `internal/core/gate_evidence.go:80-86` |
| Shipment-level passing evidence records only `level`, `outcome`, `base_ref`, `head_ref`, `ran` — no hash, no head binding | `internal/core/shipment_gate.go:490-497` |
| Member binding is by recorded `head_sha` + `git merge-base --is-ancestor` against a single shipment head, with a head-drift re-read | `internal/core/shipment_gate.go:62-109,581-632` |
| A canonical serializer + hash already exists (F2) | `internal/canonical/canonical.go:33-67` |
| Authoritative status predicates already exist (F3) | `internal/core/status_taxonomy.go:69-114` |
| **No** `crypto/hmac`, `crypto/ed25519`, keyring, or credential storage exists anywhere under `internal/` | repository-wide grep, this session |
| The only secret-adjacent precedent **rejects literal secrets in config and permits environment expansion only** | `internal/config/loader.go:188-196` |
| Item JSONL events have no sequence field, but a monotonic `Seq` already exists for hook events | `internal/events/hook_events.go:35-54,149` |
| The broker maps exit 0 to `DecisionProceed` even with empty or non-JSON stdout | `internal/core/gate/decision.go:56-60` |

### Prior learnings applied

- **Fail-closed trichotomy** — `docs/compound/2026-07-07-empty-head-fail-closed-repo-presence-probe.md`:
  a verifier must distinguish *valid* / *definitively invalid* / *unverifiable*
  and refuse on the third. Proof verification inherits this shape verbatim.
- **"Data must not choose the code"** — `docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md`:
  the honest-boundary discipline. State what the control does and does not
  guarantee rather than overclaiming.
- **Machine-readable governance fields** — `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`:
  the proof, key id, counter, and binding digest are machine-consumed, so their
  exact field names and value formats must be specified at the producer.
- **Bounded helper hard cap** — `docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md`:
  any ship-time verification helper sits on the lock-holding critical path.

### The uncomfortable fact that shapes the whole decision

In this deployment the mutating actor (an agent or the operator) runs with the
operator's own filesystem permissions. **No purely local storage is outside that
actor's write set.** Therefore no local scheme can produce a proof that is
unforgeable *by the actor who currently holds the key*. Any design that claims
otherwise is dishonest. What a local scheme *can* deliver is real and useful:

- an event written **without** the key can never be made to look authentic;
- an event authenticated at time *T* cannot be **replayed** later or **rebound**
  to a different manifest;
- an **independent verifier** (CI, review, a later agent run, a human) can check
  the proof without any ability to forge it.

That is the honest boundary this decision adopts and documents.

## Options Evaluated

### Option A — HMAC-SHA256 over a canonical payload, key from environment only

Sign a canonical payload (built with `internal/canonical`) containing event type,
`ran`, item ID, actor, timestamp, `head_sha`, the *schema-validated* gate report
digest, a monotonic per-item counter, and — for shipment evidence — the manifest
digest. Store the resulting MAC and key identifier as delta fields **outside**
the hashed payload. Key material comes from a single environment variable and is
never persisted, never echoed, never logged.

- **Pros:** stdlib only (`crypto/hmac`, `crypto/sha256`); reuses the shipped F2
  canonicalizer directly; symmetric verification is trivially available to CI;
  matches the existing env-expansion-only secret precedent; smallest possible
  change surface; naturally fail-closed when the variable is absent.
- **Cons:** symmetric — a verifier that can check can also forge. Does not defend
  against an actor who currently holds the key. Requires the operator to provision
  a variable.
- **Effort:** medium. **Fit:** high.

### Option B — Detached signature via an external signer command

Model on `git`'s `gpg.program`: shell out to an operator-configured signer so the
private key never enters the backlogit process.

- **Pros:** asymmetric — verifiers cannot forge; key can live in an agent, HSM, or
  smart card genuinely outside the actor's write set.
- **Cons:** adds a second exec surface, which the bare-path-validated-binary
  learning constrains tightly; adds an external tooling dependency (Principle VI);
  signer availability becomes a new failure and DoS class on the gate critical
  path; substantially more than one bounded unit of work.
- **Effort:** high. **Fit:** medium.

### Option C — Trusted chain head anchored in git

Persist the evidence chain head as a git object (note or trailer) so the anchor
lives outside the mutable item log.

- **Pros:** no key management at all; the anchor becomes durable once pushed to a
  protected branch.
- **Cons:** locally the actor can rewrite git as easily as JSONL, so the guarantee
  only materializes after push; couples the gate to remote branch protection and
  to network state; the spike already warns a chain is "neither necessary nor
  sufficient on its own"; adds a git-write side effect to an evidence append.
- **Effort:** high. **Fit:** low.

### Option D — Ship the binding digest only, defer authenticity

Implement the canonical manifest↔evidence binding digest with no proof.

- **Pros:** cheapest; strictly better than today for accidental drift.
- **Cons:** the spike is explicit that a self-covering digest is **not** a binding —
  an actor who edits the manifest recomputes the digest. The gate stays advisory,
  so F1 does not actually unblock the formal PASS-only gate. Fails the success
  criteria.
- **Effort:** low. **Fit:** low.

## Trade-off Comparison

| Criterion | A (HMAC, env key) | B (external signer) | C (git anchor) | D (digest only) |
|---|---|---|---|---|
| Complexity | medium | high | high | low |
| New dependencies | none (stdlib) | external signer binary | git write path | none |
| Blocks forgery *without* key | **yes** | yes | after push only | **no** |
| Blocks replay | yes (counter in payload) | yes | partial | no |
| Blocks manifest substitution | **yes** (binding covered by MAC) | yes | partial | **no** |
| Independent verifier can check | yes | yes | yes | n/a |
| Fail-closed when unconfigured | yes | yes | ambiguous | n/a |
| Honest boundary statable | yes | yes | hard | n/a |
| Bounded to ~2h units | yes | no | no | yes |
| Reuses shipped F2/F3 | yes | yes | partial | yes |

## Decision

**Adopt Option A**, with the trust boundary stated explicitly and with an
extension point that admits Option B later without redesign.

### Mechanism

`HMAC-SHA256` computed over `internal/canonical.Canonicalize(payload)`. The
canonical payload is a new, purpose-built structure — it explicitly does **not**
reuse `EvidenceSHA` / `gate_report_hash`, which covers only raw report bytes and
therefore cannot distinguish distinct or replayed events.

Payload fields (exact names fixed at the producer, per the machine-readable
governance-field contract):

| Field | Purpose |
|---|---|
| `schema` | proof payload schema version |
| `event_type` | pins `gate_passed` vs `gate_forced` |
| `ran` | pins that the gate actually executed |
| `item_id` | pins identity |
| `actor` | pins attribution |
| `timestamp_utc` | pins time |
| `head_sha` | pins the reviewed tree |
| `report_digest` | canonical hash of the **schema-validated** formal report |
| `counter` | monotonic per-item anti-replay counter |
| `manifest_digest` | shipment evidence only; canonical hash of the ordered manifest membership + covering feature + shipment head |

The MAC, the key identifier, the counter, and the payload schema version are
written as delta fields **outside** the hashed payload, so the actor cannot
recompute the proof by editing the payload.

### Key custody

- Single source: the environment variable `BACKLOGIT_GATE_EVIDENCE_KEY`.
- **Never** read from `config.yaml`, `registry.yaml`, `hooks.yaml`, any workspace
  file, or any CLI flag. This extends the existing rule at
  `internal/config/loader.go:188-196` (literal secrets rejected; env expansion only).
- **Never** written to the index, the JSONL logs, any error message, or any log
  line. Config may carry only a non-secret `key_id` for rotation bookkeeping.
- Minimum length enforced; malformed or short key is a hard error.

### Fail-closed rule (non-negotiable)

When formal admission is enforced and the key is absent, malformed, or the MAC
does not verify, the operation **refuses**. There is no unauthenticated fallback
and no "warn and continue". Verification implements the three-way outcome —
*valid* / *definitively invalid* / *unverifiable* — and refuses on both of the
last two. When formal admission is **not** enforced, behavior is unchanged from
today, so the change is opt-in and non-breaking.

### Anti-replay

A monotonic per-item counter, modeled on the existing hook-event `Seq`
(`internal/events/hook_events.go:35-54,149`), is bound **inside** the signed
payload. A verifier rejects any admissible event whose counter is not strictly
greater than the highest counter already admitted for that item. Counter state is
derived from the log itself; because the counter is inside the MAC, an actor
without the key cannot fabricate a higher one.

**Intact-log limitation (honest, load-bearing).** The log-derived counter
provides rollback and duplicate detection **within an intact log**, not
unconditional anti-replay: the mutating actor can delete or truncate log
records, which lowers the observed counter floor for a later admission check.
Full anti-replay requires a high-water anchor **outside** the actor's write
set. The reviewed implementation plan therefore supports an **optional
verifier-owned high-water ledger** (path supplied by environment, e.g. held by
CI) and enforces it strictly when configured; backlogit only **reads** the
ledger for comparison and never writes to it — the external verifier that owns
the ledger is solely responsible for updating it, so no out-of-workspace write
originates from this codebase. When the ledger is not configured, the
guarantee is exactly the narrower intact-log guarantee stated above. No unit
may claim a stronger guarantee than this.

### Formal admission predicate

A **new, dedicated** predicate — not `gateevidence.Latest`. It requires all of:
a verified MAC; `event_type == gate_passed`; `ran == true`; a schema-validated
formal report whose digest matches `report_digest`; a counter strictly greater
than every previously admitted counter; and **no** later block or requeue event
for the item. `EventGateForced` is never formally admissible.

### Manifest binding

`manifest_digest` is the canonical hash of the ordered manifest membership plus
the covering feature plus the resolved shipment head. It is a field *inside* the
signed payload, so it is covered by the MAC. At ship time the verifier recomputes
it from live state and refuses on mismatch. This is additive to — not a
replacement for — the existing `head_sha` ancestry and head-drift guards.

### Honest boundary (must be documented in code and in the shipped docs)

> This control guarantees that gate evidence produced **without** the key cannot
> be made to look authentic, that an authenticated event cannot be replayed or
> rebound to a different manifest, and that an independent verifier can check the
> proof. It does **not** defend against an actor who currently holds the key.
> Because the key is symmetric, any party able to verify is also able to sign.

## Rejected Alternatives

- **Option B (external signer)** — right long-term shape, wrong size now. It adds
  an exec surface and an external dependency for an assurance gain that a
  single-operator local workflow does not yet realize. The verifier is defined as
  an interface so B can be added later as an additional proof kind without
  redesigning the payload.
- **Option C (git anchor)** — the anchor is only trustworthy after push to a
  protected branch, which couples the gate to network and remote-policy state.
  The spike independently rejects a bare chain as "neither necessary nor
  sufficient".
- **Option D (digest only)** — explicitly rejected by the spike: a self-covering
  digest is not a binding. It would leave the gate advisory and fail every
  success criterion.
- **Keyless in-log hash chain** — rejected by the spike itself: an actor who can
  edit the log can rewrite the chain end to end.
- **Reusing `EvidenceSHA` as the binding digest** — rejected: it covers only raw
  report bytes, so distinct or replayed events can share the same value.

## Unresolved Questions

- Key rotation ergonomics beyond a `key_id` field are deferred until a second key
  actually exists.
- Whether CI should hold a verify-only key is an operations decision, recorded as
  an advisory follow-up rather than backlog work.
- The formal report schema itself is specified as part of F1 but its *content*
  requirements (which personas, which fields) are inherited from the existing
  review contract and are not re-litigated here.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Symmetric key implies verifier can forge | Documented honest boundary; verifier defined as an interface so an asymmetric proof kind can be added later |
| Operator forgets to provision the key | Formal admission is **opt-in**; when it is off, behavior is unchanged. When it is on, the refusal message names the exact variable |
| Secret leaks into logs or index | Key never enters any struct that is serialized; only `key_id` is persisted; a regression test asserts the key value never appears in written artifacts |
| Verification cost on the ship critical path | HMAC-SHA256 over a small canonical payload; the bounded-helper hard-cap discipline applies to any helper added |
| Counter state derived from a mutable log | The counter is inside the MAC, so it cannot be raised without the key; the derivation is the *floor*, the MAC is the *proof* |
| Scope creep back toward PR #239 | Waivers, reservations, and session handles remain charter non-goals and are named as out of scope here |

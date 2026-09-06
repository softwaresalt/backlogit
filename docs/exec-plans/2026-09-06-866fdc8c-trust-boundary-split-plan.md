---
chunk_strategy: h1-h2-h3
description: "Execution plan: decompose 866FDC8C into three isolated trust-boundary features (A trust-anchor lifecycle, B signed attestation verification, C authenticated operator approval)"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-06-866fdc8c-trust-boundary-split-plan.md
title: "Execution Plan — 866FDC8C trust-boundary decomposition (crypto authenticity / authenticated authorization)"
---

# Execution Plan — 866FDC8C trust-boundary decomposition

**Covering scope:** the #423 deferred residual `866FDC8C`, split into THREE
sibling features.
**Source deliberation:** `docs/decisions/2026-09-06-866fdc8c-trust-boundary-split-crypto-authz.md` (`065-DL`)
**Upstream delivery this builds on:** feature `167-F` / shipment `148-S` (`shipment reconcile-shipped`, #423)
**Stash:** 866FDC8C (high, feature)

## Problem Frame

Close the two #423 residuals — (1) cryptographic authenticity of delivery
evidence and (2) authenticated per-invocation operator authorization — WITHOUT
conflating the three trust boundaries they span. backlogit today has integrity
binding but no external trust anchor, no signature verification, and no
machine-authenticated authorization. This plan defines three independently
shippable features and their harvest into 2-hour, single-domain tasks. It does
NOT implement source code (Stage boundary).

## Constitution Check

Go-only; test-first (TDD per unit); workspace-isolated (all key/token/attestation
file reads are handle-bound, real-path contained, no-follow); destructive/
security-sensitive approval respected (verification is fail-closed and opt-in;
unconfigured anchors preserve #423 v1 behavior); structured observability (verified
attestation/authorization identity folded into the durable event); single-
responsibility (each feature = one trust boundary; general `reconcile` allowlist
unchanged); public-verification-material-only (no private-key custody, no operator
credential minting). No new *mandatory* runtime dependency on the happy path.

Constitution Check: pass

## Feature Decomposition (three release units)

| Role | Working title | Ships as | Depends on |
|---|---|---|---|
| A | External trust-anchor / verification-key lifecycle | own shipment | — |
| B | Signed delivery-evidence attestation verification | own shipment | A, 167-F |
| C | Authenticated per-invocation operator approval | own shipment | A, 167-F |

B and C are mutually independent (fan-out after A). Feature-level dependency
edges to create: `A blocks B`, `A blocks C`, `167-F blocks B`, `167-F blocks C`.

## Implementation Units

> Harvest ordering per feature follows the workspace TDD/harness policy
> (`.github/policies/workflow-policies.md`): a go/ast source-shape harness lands
> before each behavior-bearing declaration; behavior harness (RED) precedes each
> implementation. Every task below is single-domain and ≤2h.

> **Root-of-trust anchoring constraint (review remediation P1, Security Lens).**
> Because the deliberation's own adversary is a workspace writer who can hand-edit
> files, the trusted-key ROOT must not be solely an in-workspace, attacker-writable
> value, or B and C are defeated by config tampering (add an attacker key / flip
> `revoked`→`active` / repoint `public_key_ref`). Feature A therefore anchors the
> ROOT OF TRUST OUTSIDE the workspace: the set of trusted key fingerprints is pinned
> via an external reference (env/CI-injected pin OR an out-of-band operator-supplied
> fingerprint), and the in-workspace `trust_anchors` config is validated AGAINST that
> external pin at load time (a config entry whose fingerprint is not in the external
> pin set is rejected fail-closed). Mutation of anchors (add/rotate/revoke) is a
> distinct, audited, out-of-workspace-pin-gated operation. When no external pin is
> configured, the feature is DISABLED (fail-closed, not fail-open) and `reconcile-shipped`
> retains #423 v1 behavior.

* **A1 — `trust_anchors` config schema + loader + validation + external-pin gate.**
  Add `WorkspaceConfig.trust_anchors []TrustAnchor{ id, role, algo, public_key_ref,
  fingerprint, status(active|revoked), not_before, not_after }`; loader; validation
  (id-uniqueness, algo allowlist ed25519/ecdsa-p256, ref non-empty); and the
  EXTERNAL-PIN GATE (each anchor's `fingerprint` MUST appear in the external pin set;
  absent pin ⇒ feature disabled fail-closed). Domain: config. Deps: none. Size: S.
  Acceptance: pinned anchor loads; unpinned/mismatched anchor rejected; no-pin ⇒ disabled.
* **A2 — verification-key material loader (public-only).** Load public key bytes
  from `public_key_ref` — inline PEM or handle-safe real-path-contained file
  (no-follow) — parse per `algo`, reject private keys / malformed material, and
  confirm the loaded key's fingerprint equals the pinned `fingerprint`.
  Domain: core (fs + parse). Deps: A1. Size: S.
* **A3 — trusted-key resolver primitive.** Pure `ResolveVerificationKey(role|id,
  at time) (VerificationKey, error)` honoring rotation (`not_before`/`not_after`)
  and revocation (`status==revoked` fails closed); ambiguous/expired/revoked →
  typed error. The single reusable primitive B and C consume. Domain: core
  (pure). Deps: A2. Size: S.
* **A4 — read-only `backlogit trust-anchor` inspection CLI.** `list` / `show` /
  `verify-key <id>` over A3 — no writes. Domain: cli. Deps: A3. Size: S.
* **A5 — trust-anchor mutation CLI (`add`/`rotate`/`revoke`).** Edits config +
  `status` only, gated on the external pin (a mutation introducing an unpinned
  fingerprint is rejected), each mutation writes a durable audit event. Domain:
  cli. Deps: A3, A1. Size: S. (Split from the old A4 per architecture + scope P2:
  read inspection vs audited mutation are distinct single-responsibility units.)
* **A6 — operator docs: trust-anchor lifecycle + threat model.** Public-only
  custody, external-pin root of trust, rotation, revocation, and the residual it
  does/does not close. Domain: docs. Deps: A4, A5. Size: S.
  Acceptance: documents external-pin bootstrap, rotation, revocation, and states
  it closes NEITHER residual alone (it is the foundation B and C consume).

### Feature B — signed delivery-evidence attestation verification

* **B1 — attestation envelope declarations + parser.** Declare the attestation
  type (DSSE/in-toto envelope over a canonical evidence statement:
  shipment-id + merge-sha + manifest-digest + closure-content-hash) + source-shape
  harness + envelope parser (reject malformed / duplicate-member JSON, mirroring
  the #423 raw-token validator discipline). Domain: core. Deps: own declarations
  only (NOT A3 — parsing needs no key; review remediation P3). Size: M.
* **B2 — attestation verification primitive (fail-closed).** Verify the envelope
  signature over the canonical statement using an A3-resolved key; verify the
  statement's evidence fields equal the reconcile request's computed evidence
  (bind to `evidence_digest` inputs); any mismatch/expired-key/bad-sig → typed
  fail-closed error. Domain: core. Deps: B1, A3. Size: M.
* **B3 — event binding + doctor recognition of signed repair.** Add
  `attestation{ key_id, signer_identity, sig_algo, statement_digest }` to the
  `shipment_reconciled_shipped` delta, thread it through the transaction's
  prepared-event + `evidence_digest`/`event_digest` construction, and add a
  doctor-LOCAL branch distinguishing signed vs unsigned reconcile. Domain: core
  (event). Deps: B2, `167.002-T` (event schema) AND `167.008-T` (the transaction
  that constructs/persists the prepared event + digests — review remediation P1,
  Architecture). **Replay-digest decision:** the attestation is folded into the
  audit `event_digest` but is NOT part of the `request_identity_digest`, so a
  same-key replay with an added/removed attestation is a `conflict` (not a
  silent no_op) — preserving the #423 classifier's determinism. Size: M.
* **B4 — `reconcile-shipped --attestation <path>` CLI wiring + dry-run.**
  Opt-in flag; dry-run prints verification outcome; unconfigured/absent =
  #423 v1 behavior unchanged. Domain: cli. Deps: B2, `167.004-T` (CLI). Size: S.
* **B5 — integration + negative tests (signed 048-S-shaped fixture).** Happy
  path (valid signed attestation) + negatives: tampered statement, wrong-key,
  expired/revoked key, attestation for another shipment. Domain: tests.
  Deps: B3, B4. Size: M.
* **B6 — operator docs: attestation generation (CI) + verify.**
  Documents the opt-in verify path ONLY; any future policy-enforce mode is
  explicitly marked NOT-YET-AVAILABLE (review remediation P2, Scope). Domain:
  docs. Deps: B4. Size: S. Acceptance: documents CI-side attestation generation,
  the `--attestation` verify path, and that enforce-mode is future/not-shipped.

> **Shared-surface serialization (review remediation P2, Architecture — B⊥C).**
> B and C are independent at the trust-boundary/release-unit level but both extend
> the SAME `167.004-T` CLI, the SAME `shipment_reconciled_shipped` delta, and the
> SAME `167.008-T` transaction. Their added event fields (`attestation{}` vs
> `approver{}`) and flags (`--attestation` vs `--authorization-token`) MUST be
> strictly non-overlapping/additive; whichever shipment lands second rebases onto
> the first's additive changes to the shared surface. Ship serializes the shared
> `167.008-T`/event-delta touch-point.

### Feature C — authenticated per-invocation operator approval

* **C1 — authorization-token declarations + parser.** Declare the operator
  authorization token (signed binding of `{shipment_id, request_identity_digest,
  not_after, nonce}`) + source-shape harness + parser. Domain: core. Deps: own
  declarations only (NOT A3 — parsing needs no key; review remediation P3). Size: M.
* **C2 — token verification primitive (fail-closed).** Verify signature via an
  A3-resolved key; verify binding to the target shipment-id AND the reconcile
  request-identity digest; enforce `not_after`; typed fail-closed on any
  mismatch. Domain: core. Deps: C1, A3. Size: M.
* **C3 — nonce single-use ledger (anti-replay).** Durable, handle-safe,
  append-then-fsync record of consumed nonces; reject a replayed token
  fail-closed. Domain: core. Deps: C2. Size: M.
  **Justify-or-fold gate (review remediation P1, Scope):** Feature-C planning MUST
  record why request-identity binding + `not_after` alone is INSUFFICIENT before
  committing this durable store. Rationale on file: a captured token is bound to a
  `request_identity_digest`, and the reconcile op is idempotent (same-key ⇒ no_op),
  so replay of the identical request is already neutralized; C3 defends the residual
  window where a token is captured before its first legitimate use. **Ledger
  integrity + growth (review remediation P2, Security Lens):** the ledger's root of
  trust is the SAME external pin as A (a ledger reset without the pinned key cannot
  forge un-consumption of a validly-signed nonce because verification still requires
  C2); consumed nonces are pruned past their token `not_after` to bound growth.
  If planning finds the binding sufficient, C3 folds into C2 and this task is dropped.
* **C4 — core authenticated-approval gate integration.** Inside the `167.008-T`
  locked critical section, require a C2/C3-verified token in place of the
  self-suppliable `--confirm`/TTY when an authorization policy is configured, and
  persist the authenticated approver identity (`approver{...}`) into the prepared
  event. Domain: core (transaction seam). Deps: C2, C3, `167.008-T`. Size: M.
  (Split from the old C4 per architecture P1 — core gate vs CLI surface are
  distinct single-domain units mirroring the #423 `167.008`/`167.004` boundary.)
* **C5 — `reconcile-shipped --authorization-token <path>` CLI surface + dry-run.**
  Opt-in flag; dry-run shows the authorization verification outcome; unconfigured
  = #423 v1 confirmation behavior. Domain: cli. Deps: C4, `167.004-T`. Size: S.
* **C6 — integration + negative tests.** Valid token authorizes; self-minted
  (no trusted key), expired, wrong-shipment, wrong-request-identity, and
  replayed tokens each rejected. Domain: tests. Deps: C5. Size: M.
* **C7 — operator docs + `167.015-T` linkage note.** Documents that Feature C is
  the mechanism that upgrades the confirmation-only v1 to authenticated
  authorization; does NOT itself close `167.015-T`. Domain: docs. Deps: C5. Size: S.
  Acceptance: documents token issuance/verification flow and states it is the
  mechanism for, but does not by itself satisfy, `167.015-T`.

## Dependency Graph (harvest ordering)

```
A1 → A2 → A3 → A4 (read CLI) → A6 (docs)
              A3 → A5 (mutation CLI) → A6
B1 → B2 (also needs A3) → B3 → B5
                          B2 → B4 → B5, B6
C1 → C2 (also needs A3) → C3 → C4 → C5 → C6, C7
                          C2 → C4
167-F / 148-S delivered → B3 (167.002-T + 167.008-T), B4 (167.004-T),
                          C4 (167.008-T), C5 (167.004-T)
```

Feature-level: `A blocks B`, `A blocks C`, `167-F blocks B`, `167-F blocks C`;
`B` and `C` are independent. Note: B1/C1 (parsers) depend only on their own
declarations, not A3, so they may proceed in parallel with A2/A3.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| Introduce an external trust anchor / verification-key store | High — a mis-scoped anchor store could hold or leak private key material, or trust an attacker-supplied key | Public verification material ONLY (A2 rejects private keys); keys resolved by explicit config (A1), never trust-on-first-use; revocation + validity window enforced fail-closed in A3; no network fetch (offline verification only) |
| Verify a signed delivery attestation and mark a repair "authentic" | High — a verification bug would let a forged attestation pass, worse than today's honest "integrity-only" posture | B2 fail-closed on any signature/binding/expiry mismatch; statement fields must EQUAL the reconcile-computed evidence (bound to the existing `evidence_digest` inputs); duplicate-member JSON rejected (raw-token scan, mirroring #423); negative tests for tampered/wrong-key/expired/cross-shipment (B5) |
| Verify an operator authorization token and treat the invocation as authorized | High — a self-mintable or replayable token would recreate exactly residual (2) | Token must be signed by an A3-trusted key backlogit does NOT hold the private half of (no self-minting); bound to shipment-id + request-identity digest + expiry; single-use nonce ledger (C3) prevents replay; negatives for self-minted/expired/wrong-shipment/wrong-identity/replay (C5) |
| Extend the `shipment_reconciled_shipped` event schema | Medium — event/doctor consumers could misread signed vs unsigned repairs | Additive `attestation{...}` delta folded into the event digest; doctor-LOCAL recognition branch (does not broaden the shared strict presence check); unsigned repairs still valid (backward-compatible) |
| Opt-in verification layered on the shipped #423 surface | Medium — could regress the #423 v1 happy path | Unconfigured anchors/policies preserve EXACT #423 v1 behavior (confirmation + audit); verification is additive and only fail-closed when an anchor/policy IS configured; general `reconcile` allowlist untouched |

Rollback: features are additive and opt-in; disabling the config restores #423
v1 behavior. Ownership: A owns `internal/config` trust-anchor + `internal/core`
key resolver; B owns attestation verification + event binding; C owns token
verification + nonce ledger + gate wiring.

### Plan Hardening Signals (REQUIRED)

* destructive / state-repair on delivered records: PRESENT — B/C gate the
  `reconcile-shipped` state-repair path.
* security/audit/permission/compliance-sensitive: PRESENT — cryptographic
  verification, trust-anchor custody, authenticated authorization, audit-event
  schema.
* concurrency-sensitive: PRESENT — C3 nonce ledger is a durable append under the
  reconcile lock discipline.
* public API/schema/contract change: PRESENT — new config block, new CLI surface
  + flags, new event delta field.

Requires plan hardening: yes

## Plan Review

<!-- plan-review-attempt: 1 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

personas (cross-model multi-persona plan review, 1 cycle + remediation):
* Security Lens Reviewer (`gpt-5.6-sol`, high) — ADVISORY (P1 trust-anchor config
  circularity / root-of-trust anchoring; P2 nonce-ledger integrity+growth; P2
  request-identity-digest under-defined).
* Architecture Strategist (`gemini-3.8-flash`, high) — ADVISORY (P1 missing
  B3→167.008-T edge; P1 C4 single-domain/2h violation; P2 A4 too wide; P2 B⊥C
  shared-surface coupling; P3 over-tight A3→B1/C1 edges; P3 B3 event/doctor split).
* Scope Boundary Auditor (`claude-opus-4.8`, high) — ADVISORY (split faithful, A
  justified by two real consumers, NOT a D6.1-style over-scope; P1 C3 justify-or-fold;
  P2 A4 mutation scope; P2 B6 enforce-mode doc; P3 docs-task acceptance criteria).

Remediation applied this cycle (verdict upgraded ADVISORY → PASS):
* Root-of-trust anchoring constraint added (external pin gates the in-workspace
  `trust_anchors`; feature disabled fail-closed when unpinned) — closes Security P1.
* A4 split into read-only inspection (A4) + audited mutation (A5) — closes Arch/Scope P2.
* B3 now depends on `167.008-T` (transaction) and records the attestation/replay-digest
  decision (attestation in audit digest, NOT in request-identity → replay=conflict) — closes Arch P1.
* Old C4 split into core gate integration (C4, in the 167.008-T locked section) +
  CLI surface (C5) — closes Arch P1 single-domain violation.
* B1/C1 re-pointed to depend only on their own declarations (parallel with A2/A3) — closes Arch/Scope P3.
* C3 carries an explicit justify-or-fold gate + ledger-integrity (external-pin-rooted)
  + pruning-past-not_after note — closes Scope P1 / Security P2.
* B6 enforce-mode marked NOT-YET-AVAILABLE; docs tasks (A6/B6/C7) given checkable
  acceptance lines — closes Scope P2/P3.
* Shared-surface serialization note added for B⊥C (additive non-overlapping fields;
  Ship serializes 167.008-T touch-point) — closes Arch P2.

Residual (accepted, non-blocking, carried into feature-level detailed planning):
request-identity-digest issuance flow (what it commits to + how the operator
pre-commits) is a Feature-C planning decision (D5 open question 3), not a plan-gate blocker.

Scope disposition (P-021): this plan decomposes `866FDC8C` only; it neither closes
nor removes the #423 `167.015-T` authorization ratification gate (D3). Feature C is
the mechanism that later closes residual (2); Feature B closes residual (1).

Readiness: HARVEST-READY — three coherent release units (Feature A = 6 tasks,
Feature B = 6 tasks, Feature C = 7 tasks = 19 single-domain ≤2h tasks total),
feature-level dependency edges A→B, A→C, 167-F→B, 167-F→C, B⊥C. Cleared for harvest.

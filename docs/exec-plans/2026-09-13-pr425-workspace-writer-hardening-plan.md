---
chunk_strategy: h1-h2-h3
description: "Hardening-addendum implementation plan amending 168-F/169-F/170-F so the PR #425 workspace-writer threat-model findings (3B661FCE, BFF76433, BF18DA1D, 4E210DB4) are genuinely resolved before Ship claims 149-S/150-S/151-S"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-13-pr425-workspace-writer-hardening-plan.md
title: "Hardening Addendum Plan: 168-F/169-F/170-F workspace-writer resistance"
docline:
    stash_id: 3B661FCE,BFF76433,BF18DA1D,4E210DB4
    status: approved
    created_at: 2026-09-13T17:31:00Z
---

## Objective

Amend the already-PASSED 866FDC8C trust-boundary split (`168-F`/`169-F`/`170-F`)
with hardening tasks that close four PR #425 review threads under the workspace-
writer threat model, and add those tasks to the existing queued shipments
`149-S`/`150-S`/`151-S` so they are resolved **before** Ship.

Source: `docs/decisions/2026-09-13-pr425-workspace-writer-hardening-deliberation.md`.
Origin plan: `docs/exec-plans/2026-09-13-866fdc8c-trust-boundary-split-decided-plan.md`.

## Requires plan hardening: yes

Security-architecture changes to trust-anchor status binding, attestation
persistence, and anti-replay state. See `## Plan Hardening`.

## Implementation Units (new hardening tasks)

### 168-F (shipment 149-S) — trust-anchor external binding [3B661FCE]

* **168.010-T (RED harness — predecessor)** Write the failing negative tests
  BEFORE `168.007-T`/`168.008-T`/`168.009-T` and observe them RED: (1) a
  workspace-local edit flipping a revoked/de-pinned key to `active` currently
  resolves as trusted (must FAIL after the fix); (2) an in-workspace role-widening
  / validity-extension currently resolves as trusted (must FAIL after the fix);
  (3) an advisory-only `revoke` currently reports success without authoritative
  external revocation (must FAIL after the fix). Domain: tests. Depends on
  `168.001-T`, `168.003-T`. ≤3 scenarios.
  * AC: all three tests exist and reproduce the vulnerable behavior against the
    pre-hardening resolver/CLI, giving a genuine RED baseline.
* **168.007-T** Make external-pin **status** authoritative over in-workspace
  config: the resolver treats in-workspace `active`/`revoked` status as advisory
  only; an externally-pinned key that is de-pinned (removed from the external pin
  set) resolves as **not trusted** and a workspace-local edit CANNOT flip a
  de-pinned/revoked key back to active. Revocation-via-external-de-pin is
  monotonic. Fail closed if the external pin set is absent. Domain: code.
  Depends on `168.001-T`, `168.003-T`, `168.010-T` (RED).
  * AC: a workspace-local edit flipping a revoked/de-pinned key to `active` does
    NOT resolve it as trusted; negative test proves reactivation is rejected.
  * Scope note: this closes the **named** 3B661FCE vector (revoked→active
    reactivation). Because `168.001-T`'s external-pin gate is a **fingerprint-
    membership set only** (no per-key role or validity), role and validity-window
    are NOT yet externally authoritative — see `168.008-T`.
* **168.008-T** Extend the external trust-root representation to carry **role**
  and **validity-window** (`not_before`/`not_after`) per key, and bind the
  resolver to them so a workspace writer cannot widen a key's role or un-expire a
  key by editing in-workspace config. A **documentation-only residual is NOT an
  acceptable outcome** for role/validity (consistent with the 170.008-T and
  Decision-1 no-document-only-closure stance). If extending the external
  representation to carry role/validity is genuinely out of 149-S's bounded
  scope, the resolver MUST instead enforce **concrete fail-closed** behavior:
  treat any in-workspace-asserted role/validity that is NOT externally
  authoritative as **non-authoritative and deny** — i.e. a key whose role or
  validity is not confirmed by the external root resolves as NOT trusted for the
  widened role / extended window (fail closed), rather than recording a tracked
  sub-vector and deferring enforcement. Domain: code. Depends on `168.007-T`.
  * AC: role-widening and validity-extension via in-workspace edit are rejected —
    either because role/validity are externally authoritative, OR because the
    resolver fails closed (denies) on any role/validity not confirmed by the
    external root. No documentation-only deferral of enforcement is accepted; a
    negative test proves the widened role / extended validity is denied.
* **168.009-T** Make `trust-anchor revoke` (168.005-T) **authoritatively
  revoke**: a revoke that only flips in-workspace `status: revoked` (now advisory
  per 168.007-T) MUST NOT report success as if the key were revoked. Revoke MUST
  perform an **externally authoritative revocation** — write/append to the
  external revocation/tombstone set (the same external pin root of trust), or
  explicitly surface the required **external operation** to the operator and
  **fail closed** (non-success) until it is confirmed — so the resolver's
  monotonic external de-pin/tombstone actually denies the key. Domain: code.
  Depends on `168.005-T`, `168.007-T`.
  * AC: `revoke` returns success ONLY when the external revocation/tombstone is
    recorded (or the explicit external operation is confirmed); a revoke that
    could only change advisory in-workspace status returns a non-success / fails
    closed and does not claim the key is revoked; a negative test proves an
    advisory-only status flip does not resolve the key as revoked.

### 169-F (shipment 150-S) — durable signed material [BFF76433 + BF18DA1D]

* **169.008-T (RED harness — predecessor)** Write the failing negative tests
  BEFORE `169.007-T` and observe them RED: (1) a forged unsigned-metadata-only
  event is **rejected**; (2) the removed presence-based metadata-presence doctor
  branch **no longer accepts** (asserts the forgeable accept path is gone).
  Domain: tests. Depends on `169.003-T` (base event surface). ≤2 scenarios.
  * AC: both tests exist and fail against the pre-hardening code (which still
    accepts unsigned metadata), demonstrating a genuine RED baseline.
* **169.007-T** Persist a durable **signed envelope** in the `169.003-T` event
  binding, replacing unsigned-metadata+digest, enabling independent
  `doctor`/auditor re-verification against the external root; fail closed when
  signed material is absent. If a durable **protected reference** is used instead
  of the inline envelope, the reference MUST be **digest-pinned** so a swapped or
  tampered target fails closed rather than re-verifying a substituted envelope.
  This task **supersedes** (removes/upgrades) the presence-based, forgeable
  `169.003-T` metadata-presence doctor branch — it is NOT an additive parallel
  accept path — and updates the `169-F` / `169.003-T` contract text so the stale
  "doctor distinguishes by presence of attested metadata fields" acceptance is
  removed. Domain: code. Depends on `169.002-T`, `169.003-T`, `169.008-T` (RED).
  * AC: `doctor` re-verifies a persisted attestation offline against the external
    root; unsigned-metadata-only events are rejected; a swapped/tampered
    protected reference fails closed; the 169-F/169.003-T presence-only acceptance
    text is removed.
* **169.009-T (GREEN integration)** After impl: persisted signed envelope
  re-verifies against the external root; tampered envelope / swapped reference
  fails closed. Domain: tests. Depends on `169.007-T`. ≤2 scenarios.

### 170-F (shipment 151-S) — rollback-resistant nonce ledger [4E210DB4]

* **170.010-T (RED harness — predecessor)** Write the failing negative tests
  BEFORE `170.008-T`/`170.009-T` and observe them RED: (1) deleting/truncating
  the in-workspace nonce ledger permits replay of a still-valid token (this test
  must FAIL after the fix); (2) removing/malforming the workspace auth policy
  downgrades 170-F enforcement to self-suppliable confirmation (this test must
  FAIL after the fix). Domain: tests. Depends on `170.003-T`. ≤2 scenarios.
  * AC: both tests exist and reproduce the vulnerable behavior against the
    pre-hardening code, giving a genuine RED baseline.
* **170.008-T** Move nonce single-use consumption state to **externally-protected,
  rollback-resistant, atomic compare-and-consume (test-and-set)** storage bound to
  the external pin root of trust — a single atomic check-then-mark on an
  independently protected store — so ledger deletion/truncation cannot re-enable
  replay; fail closed if the external consumption state is unavailable.
  **Option A is REQUIRED before Ship claims 151-S.** Document-only closure —
  binding nonce lifetime and merely documenting the residual replay window — is
  **NOT an acceptable Ship outcome** for this authorization-replay /
  privilege-escalation vector (consistent with Decision 1). If externally-protected
  atomic compare-and-consume state is genuinely infeasible within 151-S, the only
  choices are: (i) **block Ship for 151-S** and re-plan, or (ii) retain a fold
  ONLY behind a hard-reviewed **non-exploitability proof that itself rests on
  independently protected state** — e.g. the replay reduction to a
  no-op/conflict must be guaranteed by an *externally/independently protected*
  idempotency or request-identity record (not merely request-identity binding +
  short `not_after` asserted from workspace-resident state), reviewed and approved
  as genuinely non-exploitable, NOT a documentation checkbox. **Explicitly
  excluded as a fold basis:** `archived_status` and any other workspace-resident
  reconcile/ledger frontmatter (the concurrency CAS guards it only against
  concurrent stale overwrite, not against a deliberate workspace-writer reset),
  and any reconcile-idempotency record not itself externally protected and bound
  to the same external pin root of trust as the nonce store. Domain: code.
  Depends on `170.003-T`, `170.010-T` (RED).
  * AC: deleting/truncating the in-workspace ledger does not permit replay of a
    still-valid token (atomic external compare-and-consume, Option A); OR (only if
    A infeasible) a hard-reviewed non-exploitability proof grounded in
    independently protected state is recorded and approved, otherwise 151-S is
    blocked. Document-only closure is rejected.
* **170.009-T** Make **required-auth enablement externally authoritative /
  fail-closed**: a workspace writer who removes or malforms the in-workspace auth
  policy config MUST NOT downgrade 170-F enforcement back to the self-suppliable
  `--confirm`/TTY v1 behavior. The "authenticated-authorization required" state
  must be asserted by the external pin root of trust (or an externally protected
  enablement flag); when required-auth is externally enabled, a missing/malformed
  in-workspace policy **fails closed (denies the invocation)** rather than
  silently falling back to confirmation-only. Domain: code. Depends on
  `170.002-T`, `168.007-T` (external authority), `170.010-T` (RED).
  * AC: with required-auth externally enabled, removing/malforming the workspace
    auth policy denies the invocation (fail closed) and does NOT downgrade to
    self-suppliable confirmation; a negative test proves the downgrade is
    rejected.

## Constitution Check

* **Test-first ordering (NON-NEGOTIABLE)**: each behavior-changing security task
  is preceded by a RED harness that is written and observed failing first —
  168.010-T (RED) → 168.007/168.008/168.009; 169.008-T (RED) → 169.007-T →
  169.009-T (GREEN integration); 170.010-T (RED) → 170.008/170.009. Dependency
  edges enforce the ordering. Pass.
* **Single-domain tasks**: 168.010 tests, 168.007 code, 168.008 code, 168.009
  code, 169.008 tests, 169.007 code, 169.009 tests, 170.010 tests, 170.008 code,
  170.009 code. Pass.
* **2-hour rule**: each task bounded to its feature's existing surface; test
  tasks ≤3 scenarios. Pass.
* **Fail-closed security posture**: every hardening task fails closed on missing
  external material; no documentation-only closure for role/validity (168.008),
  revocation (168.009), replay (170.008), or auth enablement (170.009). Pass.
* **Workspace containment (P-017)**: edits confined to 168/169/170 feature
  surfaces; no autoharness change. Pass.
* **Dependency integrity (P-003)**: each hardening task depends on the base task
  it hardens and its RED harness (168.010→168.001/168.003; 168.007→168.001/168.003/
  168.010; 168.008→168.007; 168.009→168.005/168.007; 169.008→169.003;
  169.007→169.002/169.003/169.008; 169.009→169.007; 170.010→170.003;
  170.008→170.003/170.010; 170.009→170.002/168.007/170.010). Parents 168-F/169-F/
  170-F precede their children. The new hardening tasks are inserted into
  149-S/150-S/151-S at shipment assembly (Stage Step 5.5); Ship MUST confirm each
  hardening task is present in its shipment AND sequenced after its base
  dependency before claiming 149-S/150-S/151-S. Pass (membership added at
  assembly; Ship-gate confirmation required).

Constitution Check: pass

## Plan Hardening

* **ProposedAction**: make external-pin revocation authoritative over in-workspace
  status; make `revoke` perform authoritative external revocation/tombstone.
  **ActionRisk**: high (auth/authz trust decision). Mitigation: advisory-only
  in-workspace status + monotonic external revocation; `revoke` fails closed
  (non-success) unless the external revocation/tombstone is recorded; negative
  tests for reactivation and advisory-only revoke.
* **ProposedAction**: bind role/validity to the external root or fail closed.
  **ActionRisk**: high. Mitigation: no documentation-only residual — role/validity
  not externally confirmed resolves as NOT trusted (deny).
* **ProposedAction**: persist signed envelope in event schema. **ActionRisk**:
  high (evidence-integrity / event-schema change). Mitigation: fail-closed on
  absent signed material; independent re-verification test; supersedes the
  forgeable unsigned-metadata path rather than adding a desyncable sidecar.
* **ProposedAction**: externally-protected atomic compare-and-consume nonce state;
  externally authoritative required-auth enablement. **ActionRisk**: high
  (anti-replay / authz bypass). Mitigation: Option A required before Ship or block
  151-S; any fold requires a non-exploitability proof grounded in independently
  protected state (no document-only closure); auth enablement fails closed on
  policy removal/malformation.
* **Trust-boundary invariant** (from origin plan): never treat the workspace as
  its own root of trust — every task binds to / verifies against the external
  root.
* **Shared external-root-of-trust boundary (design coherence)**: the four
  externally-protected stores this plan relies on — the external pin/fingerprint
  set (168.001-T), the external revocation/tombstone set (168.009-T), the atomic
  nonce-consume store (170.008-T), and the required-auth enablement flag
  (170.009-T) — are NOT four independently-invented protection models. They all
  derive their anti-tamper guarantee from the SINGLE external root-of-trust
  boundary defined by the decided 866FDC8C trust-boundary split
  (`docs/decisions/2026-09-06-866fdc8c-trust-boundary-split-crypto-authz.md`):
  state that resides OUTSIDE the workspace-writer's write capability, holding
  PUBLIC verification material only (no credential/private-key custody), with
  monotonic revocation against that root. `170.009-T`'s dependency on `168.007-T`
  is for this shared external-anchor **boundary/mechanism** — 168.007-T's resolver
  is the in-repo consumer of the external anchor — NOT because 168.007-T itself
  exposes the enablement flag; the required-auth enablement flag is a distinct
  externally-protected datum anchored to the SAME root. Before Ship, the concrete
  external-store surface for the tombstone set, the atomic nonce-consume store, and
  the enablement flag MUST be confirmed to reside on that same boundary (a single
  shared external-authority surface is preferred over per-feature stores); the
  rollback-resistance and monotonicity guarantees of 168.009-T/170.008-T are void
  if that boundary is not enforced.
* **Rollback (SECURITY-SAFE — never restore forgeable/replayable behavior)**:
  reverting an individual hardening task MUST NOT silently restore the
  pre-hardening forgeable/replayable behavior. If a hardening task must be backed
  out, the affected security feature is **disabled (fails closed)** OR the whole
  shipment (149-S/150-S/151-S) is rolled back as a unit; in either case the
  **external revocation/tombstone state (168) and the external nonce-consumption
  state (170) are preserved** so a revoked key stays revoked and a consumed nonce
  stays consumed. A revert that would re-enable a workspace-forgeable trust flip,
  an unsigned-metadata accept path, replay of a consumed token, or a downgrade to
  self-suppliable confirmation is prohibited; disable-the-feature (deny) is the
  only acceptable degraded state.

## Verification

* `go test ./internal/core/... ./cmd/... -race` for the RED harnesses
  (168.010-T, 169.008-T, 170.010-T), the GREEN integration (169.009-T), and
  per-task unit coverage.
* `doctor` independent re-verification path exercised for 169.007-T.

## Traceability

Threads: `PRRT_kwDORzozKM6fvp_k` (3B661FCE), `PRRT_kwDORzozKM6fvp_X` (BFF76433),
`PRRT_kwDORzozKM6fvp_1` (BF18DA1D), `PRRT_kwDORzozKM6fvp_g` (4E210DB4); PR #425;
features 168-F/169-F/170-F; shipments 149-S/150-S/151-S.

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

Attempt 1. Personas: Security Lens / Security Reviewer (specialist), Correctness
+ Architecture Strategist (cross-cut). Coverage complete.

**Gate rationale**: one P1 finding → FAIL per rubric.

Findings:
* **P1 (Security)** — 170.008-T justify-or-fold permitted document-only closure
  of an authorization-replay / privilege-escalation vector, inconsistent with
  Decision 1 (which rejects document-only closure for the analogous 168
  reactivation vector). Not "genuinely resolved before Ship."
* **P2 (Security)** — 168.007-T claimed to bind role+status+validity externally,
  but 168.001-T external-pin is fingerprint-membership only; role/validity remain
  in-workspace-mutable.
* **P2 (Security)** — hardening tasks not yet in 149-S/150-S/151-S manifests;
  Ship-gate must confirm insertion + sequencing.
* **P3 (Security)** — 169.007-T protected reference should be digest-pinned and
  must supersede (not parallel) the forgeable metadata-presence branch.

Plan hardening required: yes; present. Remediation applied in this revision
(170.008-T Option A now REQUIRED before Ship or Ship blocked; new 168.008-T for
role/validity external binding or tracked residual; 169.007-T digest-pinned +
supersedes forgeable branch; Constitution Check P-003 note on assembly-time
membership + Ship-gate confirmation). Re-review below.

<!-- plan-review-attempt: 1 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Attempt 2. Personas: Security Lens / Security Reviewer (specialist, re-review).
Coverage complete. All four prior findings confirmed resolved at plan level:
(1) 170.008-T now REQUIRES Option A before Ship or blocks 151-S, fold gated behind
hard-reviewed non-exploitability proof (P1 resolved); (2) 168.007-T honestly
scoped to the status vector + new 168.008-T for role/validity external binding or
bounded tracked residual (P2 resolved); (3) Constitution Check states
assembly-time membership + Ship-gate confirmation (P2 resolved); (4) 169.007-T
digest-pinned + supersedes forgeable branch (P3 resolved). The attempt-2 P3
(168.008-T residual bound) was tightened to require a fail-closed/monotonic
mitigation + Ship-gate acknowledgement.

**Gate rationale**: no P0/P1/P2 remain. Plan hardening required: yes; present and
adequate. Security findings are genuinely resolved before Ship (hardening tasks
added to 149-S/150-S/151-S at assembly; Ship-gate must confirm sequencing).

<!-- plan-review-attempt: 2 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Attempt 3 (review-fix cycle 1). Personas dispatched (7, coverage complete):
Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher,
Architecture Strategist, Security Lens Reviewer, Agent-Native Parity Reviewer.

**Findings and remediation:**
* **No P0/P1 (Security Lens)** — all six mandated security invariants (170.008
  atomic compare-and-consume / no doc-only closure; 168.009 authoritative revoke;
  168.008 fail-closed role/validity; 170.009 externally-authoritative auth
  enablement; 169.007 durable signed material superseding the forgeable branch;
  security-safe rollback) confirmed fully and consistently enforced across plan,
  tasks, and manifests. Learnings Researcher confirmed the four threads are the
  in-scope, consistent continuation of the decided 866FDC8C split (P3, no
  contradiction).
* **P2 (Security Lens)** — the 170.008 fold-route example ("reconcile idempotency")
  could be misread to admit self-suppliable `archived_status`/workspace-resident
  state as the non-exploitability basis. **REMEDIATED**: plan 170.008-T and task
  170.008-T now explicitly EXCLUDE `archived_status` and any workspace-resident
  idempotency record not externally protected and bound to the same external pin
  root; the fold basis must rest solely on externally-protected state.
* **P2 (Architecture Strategist)** — the four external stores asserted "same root"
  without a defined shared primitive, and 170.009's dependency on 168.007 read as
  a proxy for an enablement surface 168-F does not expose. **REMEDIATED**: added a
  "Shared external-root-of-trust boundary" hardening note binding all four stores
  to the single 866FDC8C external-anchor boundary, clarifying 170.009→168.007 is
  the shared external-anchor mechanism dependency (not an enablement-flag source),
  and requiring the concrete external-store surface be confirmed on that boundary
  before Ship.

**Gate rationale**: after remediation no P0/P1/P2 remain; residual items are P3
advisories (168.008 fallback baseline detection; external-store write-boundary
assertion; 168.007-T prose dependency traceability — the last fixed in this
revision). Plan hardening required: yes; present and adequate. Security findings
genuinely resolved before Ship (hardening tasks in 149-S/150-S/151-S; Ship-gate
confirms sequencing).

<!-- plan-review-attempt: 3 -->

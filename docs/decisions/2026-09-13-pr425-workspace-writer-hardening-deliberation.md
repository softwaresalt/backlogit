---
chunk_strategy: h1-h2-h3
description: "Deliberation for the PR #425 workspace-writer threat-model hardening group: 3B661FCE (168-F trust-anchor external binding), BFF76433+BF18DA1D (169-F durable signed material), 4E210DB4 (170-F rollback-resistant nonce ledger)"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-13-pr425-workspace-writer-hardening-deliberation.md
title: "Deliberation: PR #425 workspace-writer threat-model hardening (168-F/169-F/170-F)"
docline:
    stash_id: 3B661FCE,BFF76433,BF18DA1D,4E210DB4
    status: decided
    created_at: 2026-09-13T17:31:00Z
---

## Deliberation: PR #425 workspace-writer threat-model hardening

**Depth**: deep (security architecture). **Route**: `deliberate` (all four
entries carry `requires deliberation: true`). These four PR #425 review threads
share ONE threat model — an adversary who can **write the workspace** — and each
targets a *distinct* trust surface of the already-reviewed 866FDC8C trust-
boundary split (`168-F`/`169-F`/`170-F`). They are deliberated together because
the common principle is identical: **security state that gates a decision must be
bound to / verifiable against the external trust root, never forgeable by a
workspace writer.** They are harvested as per-feature hardening tasks so each
ships inside its own existing shipment (149-S / 150-S / 151-S).

Origin: `docs/exec-plans/2026-09-13-866fdc8c-trust-boundary-split-decided-plan.md`
(PASS-after-remediation) — these threads are follow-ups that surfaced in PR #425
review AFTER that plan passed, each ruled out-of-scope of the exact authorized
change under P-021 C1 but explicitly `requires deliberation: true`.

### Shared Threat Model

The 866FDC8C decided plan already establishes: *keep the trust-anchor root
externally pinned; do not treat the workspace as its own root of trust.* The four
findings are places where the current design still lets a workspace writer
influence a security decision because the security state lives (or is verifiable
only) **inside** the workspace.

---

### Decision 1 — 3B661FCE: 168-F trust-anchor external binding

**Finding**: a workspace writer can flip a pinned **revoked** verification key
back to **active** by editing the in-workspace `trust_anchors` config, because
role/status/validity are asserted by in-workspace config rather than bound to the
external trust root.

**Options**:
* (A, CHOSEN) Bind role + status + validity-window to the external trust root:
  the resolver (`168.003-T`) treats in-workspace status as **advisory only** and
  requires the external-pin gate (`168.001-T`) to confirm active/revoked; a
  workspace-local `active` flag cannot override an externally-pinned revocation.
  Revocation is **monotonic** against the external root.
* (B) Sign the in-workspace status blob. Rejected: still workspace-resident;
  moves the forgery target to key custody without an external re-verification.
* (C) Document the risk only. Rejected: the operator requires the finding
  **genuinely resolved before Ship**, and status-reactivation is a privilege
  escalation, not an acceptable documented residual.

**Decision**: Option A. New hardening task `168.007-T` binds the trust-anchor
**status** to the external root and makes revocation non-reversible by a
workspace-local edit (depends on `168.001-T` pin gate and `168.003-T` resolver).
Because `168.001-T`'s external pin is fingerprint-membership only, `168.008-T`
extends the external representation to carry **role + validity** OR fails closed
(denies) any in-workspace-asserted role/validity not confirmed by the external
root — a documentation-only residual is **not** accepted. `168.009-T` makes
`trust-anchor revoke` (168.005-T) **authoritatively revoke**: since in-workspace
status is now advisory, a revoke that could only flip advisory status MUST NOT
report success — it performs an external revocation/tombstone (or surfaces the
explicit external operation) and fails closed otherwise. All three are gated
behind the `168.010-T` RED harness (written and observed failing first).

---

### Decision 2 — BFF76433 + BF18DA1D: 169-F durable signed material

**Finding (both threads, same concern)**: `169.003-T` event binding persists
only **unsigned metadata + digest**, which a workspace writer can forge; there is
no durable **signed** material for `doctor`/auditors to independently re-verify
against the external root. BF18DA1D (B3 event binding) and BFF76433 (169.003-T
schema) are the **same** persistence gap phrased from feature-level and task-
level views — deduplicated into one decision.

**Options**:
* (A, CHOSEN) Persist a durable **signed envelope** (or a durable protected
  reference to one) in the event, replacing unsigned-metadata-only, so `doctor`
  and auditors re-verify the attestation against the external trust root offline.
* (B) Keep unsigned metadata + add a detached signature file. Rejected: two
  artifacts that can desync; a workspace writer can drop the detached file and
  fall back to the forgeable path.
* (C) Verify only at write time, don't persist signed material. Rejected: defeats
  independent re-verification, which is the whole point.

**Decision**: Option A. New hardening task `169.007-T` persists the signed
envelope / digest-pinned protected reference (fail closed when absent) and
**supersedes** the forgeable presence-based metadata-presence doctor branch —
removing the stale "doctor distinguishes by presence of attested metadata fields"
acceptance from the `169-F` / `169.003-T` contract text rather than adding a
parallel accept path. It is preceded by the `169.008-T` **RED harness** (forged
unsigned metadata rejected; removed presence branch no longer accepts — written
and observed failing first), and followed by `169.009-T` GREEN integration
(persisted envelope re-verifies; tampered/swapped reference fails closed).
`169.007-T` depends on `169.002-T` (verification primitive), `169.003-T` (event
binding), and `169.008-T` (RED); `169.009-T` depends on `169.007-T`.

---

### Decision 3 — 4E210DB4: 170-F rollback-resistant nonce ledger

**Finding**: the `170.003-T` nonce single-use ledger keeps consumption state
in-workspace; deleting/truncating the ledger allows **replay** of still-valid
authorization tokens under the workspace-writer threat model.

**Options**:
* (A, CHOSEN) Move nonce-consumption state to an **externally protected,
  rollback-resistant, atomic compare-and-consume (test-and-set)** store bound to
  the external pin root of trust, so ledger deletion cannot re-enable replay; fail
  closed if the external consumption state is unavailable.
* (B) Bind nonce lifetime tightly + **document** the residual replay window.
  **REJECTED** as a document-only closure of an authorization-replay /
  privilege-escalation vector (consistent with Decision 1's rejection of
  document-only closure for the analogous 168 reactivation vector). The stash's
  "or explicitly document the replay risk" phrasing is explicitly overridden here:
  documentation alone does not resolve the vector before Ship.
* (C) Do nothing. Rejected.

**Decision**: Option A (externally protected atomic compare-and-consume state).
`151-S` **MAY be claimed** so its members implement this external
compare-and-consume control — claiming a shipment to build its own hardening
tasks is the normal, acyclic path. The hard gate is **PRE-COMPLETION /
PRE-SHIPPING / merge-readiness, NOT claim-time**: `151-S` MUST NOT be marked
complete, shipped, archived, or presented as merge-ready unless `170.008-T` has
landed Option A (externally protected atomic compare-and-consume) **or** the
independently-protected non-exploitability proof below is approved. If Option A is
genuinely infeasible within the bounded shipment, the ONLY alternatives are
(i) **block `151-S` from completion/shipping/merge-readiness** and re-plan, or
(ii) a hard-reviewed **non-exploitability proof that itself rests on
independently protected state** (the replay-to-no-op/conflict reduction must be
guaranteed by an externally/independently protected idempotency or
request-identity record, not by workspace-resident state alone), reviewed and
approved as genuinely non-exploitable. A document-only replay-risk fold is NOT an
acceptable outcome. New hardening task `170.008-T` depends on `170.003-T` (and its RED predecessor
`170.010-T`); a new
`170.009-T` makes required-auth enablement externally authoritative / fail-closed
so removing the workspace auth policy cannot downgrade 170-F enforcement.

---

### Cross-cutting Decision: resolve before completion/shipping (claim is permitted)

All three shipments (149-S/150-S/151-S) are still **queued** (not claimed). The
hardening tasks are added to the existing features and their shipments now, and
the amended plans are re-hardened and re-reviewed **before Ship claims** them —
that is a *planning-gate* precondition on claim (the plan must be review-PASS
before the shipment is claimed) and is fully acyclic. The **security controls
themselves** (170.008-T Option A, 168/169 hardening) are implemented *after*
claim and gate **completion/shipping/merge-readiness**, NOT claim: a shipment is
claimed precisely so its members can be built. This satisfies "genuinely resolved
before the shipment ships" without requiring any member task to be complete
before its own containing shipment is claimed. Existing security-relevant
dependency edges (149-S root; 150-S/151-S depend on 148-S + 149-S) are preserved
unchanged.

### Scope Boundary

In-repo redesign of the attestation/trust/nonce data models within 168/169/170's
own feature surface. No autoharness change. No new shipments for these — hardening
folds into the existing queued shipments.

---
chunk_strategy: h1-h2-h3
description: "Deliberation for the PR #425 workspace-writer threat-model hardening group: 3B661FCE (168-F trust-anchor external binding), BFF76433+BF18DA1D (169-F durable signed material), 4E210DB4 (170-F rollback-resistant nonce ledger)"
doc_type: learning
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

**Decision**: Option A. New hardening task `168.007-T` binds
role/status/validity to the external root and makes revocation non-reversible by
a workspace-local edit; depends on `168.001-T` (pin gate) and `168.003-T`
(resolver).

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

**Decision**: Option A. New hardening tasks `169.007-T` (persist signed envelope
/ protected reference; fail closed when absent) and `169.008-T` (negative tests:
forged unsigned metadata rejected; envelope re-verifies against external root).
`169.007-T` depends on `169.002-T` (verification primitive) and `169.003-T`
(event binding); `169.008-T` depends on `169.007-T`.

---

### Decision 3 — 4E210DB4: 170-F rollback-resistant nonce ledger

**Finding**: the `170.003-T` nonce single-use ledger keeps consumption state
in-workspace; deleting/truncating the ledger allows **replay** of still-valid
authorization tokens under the workspace-writer threat model.

**Options**:
* (A, CHOSEN) Move nonce-consumption state to an **externally protected,
  rollback-resistant** store (monotonic external counter / protected reference),
  so ledger deletion cannot re-enable replay; fail closed if the external
  consumption state is unavailable.
* (B) Bind nonce lifetime tightly + document the residual replay window.
  Acceptable ONLY as a fallback if externally-protected state is infeasible in
  this shipment; recorded as the explicit documented-risk alternative the stash
  itself names ("or explicitly document the replay risk").
* (C) Do nothing. Rejected.

**Decision**: Option A as the target; the task is written **justify-or-fold**
(matching the existing `170.003-T` marker) — if externally-protected state
cannot be delivered within the bounded shipment, fold to Option B (documented
replay risk under the stated threat model) with an explicit operator-visible
residual-risk record, NOT a silent gap. New hardening task `170.008-T` depends on
`170.003-T`.

---

### Cross-cutting Decision: resolve before Ship

All three shipments (149-S/150-S/151-S) are still **queued** (not claimed), so
the hardening tasks are added to the existing features and their shipments now,
and the amended plans are re-hardened and re-reviewed **before** Ship claims
them. This satisfies "genuinely resolved before Ship." Existing security-relevant
dependency edges (149-S root; 150-S/151-S depend on 148-S + 149-S) are preserved
unchanged.

### Scope Boundary

In-repo redesign of the attestation/trust/nonce data models within 168/169/170's
own feature surface. No autoharness change. No new shipments for these — hardening
folds into the existing queued shipments.

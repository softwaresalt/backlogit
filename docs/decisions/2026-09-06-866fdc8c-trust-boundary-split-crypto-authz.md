---
chunk_strategy: h1-h2-h3
description: "Deliberation: split 866FDC8C (#423 crypto-authenticity + authenticated-authorization follow-up) into isolated trust-boundary features"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-06-866fdc8c-trust-boundary-split-crypto-authz.md
title: "Deliberation — Split 866FDC8C into isolated trust-boundary features (crypto authenticity / authenticated authorization)"
---

# Deliberation — Split 866FDC8C into isolated trust-boundary features

**Stash:** 866FDC8C (high, feature) — DEFERRED SCOPE EXPANSION from #423 adversarial Security review
**Deliberation artifact:** `065-DL`
**Source residual owner:** `docs/decisions/2026-09-05-423-archived-shipment-reconciliation-to-shipped.md` (D1 authorization framing; Plan Review cycle 3 SEC-01 + doctor-spoof)
**Related delivery:** feature `167-F` / shipment `148-S` (#423), PR #424 review round 13
**Date:** 2026-09-06

## Problem Frame

`866FDC8C` bundles the TWO deferred residuals that #423's Plan Review (cycle 3
Security, PASS-with-out-of-scope-residual) consciously carved out of the
`shipment reconcile-shipped` v1. BOTH residuals require an **external trust
anchor backlogit does not currently have**:

1. **Cryptographic authenticity of delivery evidence** — a workspace writer who
   can hand-edit files can forge a consistent frontmatter + `shipment_reconciled_shipped`
   event (as they could forge any file-based state). #423's evidence gate provides
   strong *integrity/binding* (trusted-ref-reachable merge commit + shipment-bound
   closure + content-hash in `evidence_digest`) but no *authenticity* proof.
   Closable only by verifying a **CI/release-system signed delivery attestation**
   against an external verification key.
2. **Authenticated per-invocation operator authorization** — #423's issue text
   asks for an *operator-authorized* repair, but an autonomous caller can
   self-supply the `--confirm` phrase or allocate a TTY, so the v1 gate is
   CONFIRMATION (anti-accident) + ATTRIBUTION (audit), NOT machine-authenticated
   authorization. Closable only by an **operator-issued per-invocation credential
   the agent cannot mint** (signed authorization token / out-of-band approval
   channel / hardware-backed operator key), verified against an external anchor.

Preserving these as **one broad feature** conflates THREE distinct trust
boundaries — (i) the lifecycle of the external anchor/verification key itself,
(ii) authenticity verification of *evidence*, (iii) authorization of the
*invocation* — each with a different threat model, blast radius, and independent
delivery value. The #423 deliberation already anticipated this split by tracking
both residuals in a single follow-up stash pending Stage triage.

## Code / Contract Grounding (current reality)

| Fact | Location |
|---|---|
| v1 trust model = explicit-confirmation + audited approval; NO signing/attestation or operator trust anchor exists | `docs/exec-plans/2026-09-05-423-...-plan.md` U2 "Trust model"; Plan Hardening "Evidence binding" |
| Residual (1) crypto authenticity = non-gating, documented, accepted follow-up | #423 Plan Review "Scope disposition (P-021)" item (1) |
| Residual (2) authenticated authorization = CONSCIOUS SCOPE NARROWING, a BLOCKING acceptance gate recorded as `167.015-T` | #423 Plan Review "Scope disposition (P-021)" item (2); D1 |
| Evidence today binds via `evidence_digest` = hash(shipment-id + merge-sha + manifest-digest + closure content-hash + evidence_refs) — integrity, not authenticity | #423 plan U2 (6) |
| Durable event `shipment_reconciled_shipped` carries `evidence{...}` but no signature field | #423 plan U3 / D2 |
| `reconcile.trusted_refs` config field (git-ref trust) exists as of `167.012-T`; there is NO verification-key config | #423 plan U2 (5); `167.012-T` |
| Doctor recognizes the reconcile marker locally but cannot distinguish forged from genuine | #423 plan U3 / doctor-spoof residual |

## Decisions

### D1 — Three isolated features, not one broad feature (Option (c))

**Options considered:**
- **(a) One broad security feature.** Rejected: conflates 3 trust boundaries;
  wide blast radius (touches config, verification core, CLI, event schema, and
  the reconcile transaction at once); cannot ship or review incrementally;
  reproduces exactly the width problem the operator flagged.
- **(b) Two features (crypto-vs-authz).** Rejected: BOTH a crypto-authenticity
  feature and an authorization feature independently need to bootstrap, store,
  and verify against an external key/anchor. Splitting only crypto-vs-authz
  duplicates the anchor/key lifecycle into two divergent implementations and
  leaves two verification cores that will drift.
- **(c) Three isolated features with a shared anchor foundation.** **CHOSEN.**

**Decision:** decompose `866FDC8C` into THREE sibling features:

- **Feature A — External trust-anchor / verification-key lifecycle.** The shared
  security foundation: how backlogit configures, stores, loads, rotates, and
  revokes external *verification* keys / trust anchors (public keys only — no
  private-key custody in backlogit). Exposes a single reusable
  "resolve a trusted verification key by id/role, honoring rotation + revocation"
  primitive plus `backlogit trust-anchor` config/admin surface. Delivers value
  independently (any future signature-verifying capability reuses it) and is the
  hard prerequisite for both B and C.
- **Feature B — Signed delivery-evidence attestation verification.** Closes
  residual (1). Verifies a CI/release-system **signed attestation** over the
  delivery evidence (merge SHA + manifest digest + closure content-hash) against
  a Feature-A verification key, and binds the verified attestation identity into
  the `shipment_reconciled_shipped` event so `doctor` can distinguish a genuine
  signed repair from a forged one. Extends `reconcile-shipped` with an
  `--attestation <path>` verification path (opt-in v1, enforced-by-policy later).
- **Feature C — Authenticated per-invocation operator approval.** Closes
  residual (2) and satisfies the `167.015-T` authorization narrowing. Replaces
  self-suppliable `--confirm`/TTY with verification of an operator-issued,
  per-invocation credential the agent cannot mint (e.g. a short-lived signed
  authorization token bound to the specific shipment-id + request-identity
  digest), verified against a Feature-A verification key.

### D2 — Dependency ordering (explicit, feature-level)

**Decision:**
- **B depends on A** (`blocks`: A → B) — B cannot verify an attestation without
  A's verification-key resolution.
- **C depends on A** (`blocks`: A → C) — C cannot verify an operator credential
  without A's verification-key resolution.
- **B and C are mutually INDEPENDENT** — evidence-authenticity and
  invocation-authorization are distinct trust boundaries; either may ship first
  after A, and they may ship in parallel. No B↔C edge.

Delivery order: **A → { B, C }** (fan-out). Each feature is its own shipment so
the trust boundaries stay isolated at the release-unit level.

### D3 — Relationship to #423 residual gating (no change to 167.015-T status)

**Decision:** this split does NOT close #423 by itself and does NOT alter the
existing gate semantics:
- Residual (1) remains a **non-gating** accepted residual until Feature B ships;
  Feature B is the mechanism that finally closes it.
- Residual (2) remains the **BLOCKING** `167.015-T` acceptance gate on `148-S`
  closure. Feature C is the mechanism that upgrades confirmation-only v1 to
  authenticated authorization; until then Ship still needs explicit operator
  ratification of the confirmation-only narrowing to close #423. **This
  deliberation neither satisfies nor removes `167.015-T`.**

### D4 — Scope guardrails

- backlogit stores/verifies **public verification material only**; it never holds
  operator private keys or mints operator credentials (that would re-introduce
  the exact self-minting weakness residual (2) is about).
- Do NOT broaden the general `reconcile` allowlist; all new surface stays on the
  shipment-scoped `reconcile-shipped` path plus the new `trust-anchor` admin
  surface.
- No new *mandatory* runtime dependency for the reconcile happy-path in v1:
  attestation (B) and authenticated-authorization (C) are **opt-in** verification
  layers; when unconfigured, `reconcile-shipped` behaves exactly as the #423 v1
  (confirmation + audit) — so this is additive, backward-compatible, and
  fail-closed only when an anchor/policy IS configured.
- Stage produces artifacts only; NO source implementation in this pipeline.
- Preserve P-001 (single active implementation unit) — no active shipment is
  interrupted.

### D5 — Open questions carried into planning

- Attestation format/tooling for B (cosign/sigstore keyless vs minisign vs
  in-toto/DSSE) — settled in Feature-B planning; must be a single, auditable,
  offline-verifiable format with no network trust-on-first-use.
- Verification-key storage & rotation model for A (workspace `trust-anchor`
  config block vs external keyring reference) — settled in Feature-A planning.
- Operator-credential channel for C (short-lived signed token vs out-of-band
  approval record vs hardware-backed key) — settled in Feature-C planning.

## Done Looks Like

`866FDC8C` is decomposed into three sibling features A/B/C with explicit
feature-level dependency edges (A→B, A→C), each harvested into 2-hour,
single-domain, acceptance-criteria-bearing tasks and assembled into three
dependency-ordered shipments. The original stash entry is archived with forward
references to all three features. #423 residual gating is preserved unchanged.

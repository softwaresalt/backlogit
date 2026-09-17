---
chunk_strategy: h1-h2-h3
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-16-866fdc8c-decomposition-correction-plan.md
title: "Correction Plan: 866FDC8C trust-chain decomposition (149-S/150-S/151-S)"
---

## Correction Plan: 866FDC8C trust-chain decomposition

**Source deliberation:** `066-DL` (linked stash `AC5346BC`; covers all six
deferred-scope-expansion entries).
**Decided base plan:** `docs/exec-plans/2026-09-13-866fdc8c-trust-boundary-split-decided-plan.md`.
**Scope:** Stage-owned planning correction of the already-harvested 168-F / 169-F /
170-F decomposition. No source code is written by Stage. No new release units are
created; the queued shipments 149-S / 150-S / 151-S and their features/tasks are
amended in place. The 149-S -> 150-S and 149-S -> 151-S dependency edges are
preserved.

### Problem Frame

149-S (168-F) passes the topology pre-claim gate but carries Stage-owned planning
defects that block unconditional HARVEST-READY:

1. Several tasks conflate a source-shape/type **declaration** with
   **behavior** (loader / validation / parser / policy-gate), violating P-002.1
   (source-shape harness before declaration; behavior harness before
   implementation).
2. The 19 original 168-F/169-F/170-F tasks lack explicit, checkable pass/fail
   acceptance criteria.
3. Feature C (170-F) has no blocking design/declaration task pinning the
   request-identity-digest contract before token declaration/verification.

150-S (169-F) and 151-S (170-F) depend on 149-S and inherit the same defects.
The nine security-hardening tasks (168.007-010, 169.007-009, 170.008-010) already
carry explicit AC and RED-first ordering and are left unchanged.

### Six deferred-scope-expansion entries addressed

| Stash ID | Priority | Correction |
|---|---|---|
| AC5346BC | high | Split 168.001-T into declaration-only + behavior (loader/validation, external-pin gate) |
| 71F5C21F | high | Split 168.005-T into core security-state mutation + audit event, and a thin CLI wiring task |
| 01D8515F | high | Split 169.001-T attestation declaration from behavior-bearing parser |
| B9BA8751 | high | Split 170.001-T authorization-token declaration from behavior-bearing parser |
| 9BD58471 | high | Add a blocking design/declaration task (170.011-T) before 170.001-T/170.002-T |
| E8D4ED66 | medium | Backfill checkable pass/fail acceptance criteria across all 19 original tasks |

P-021 C5(A) duplicate scan: **CLEAN** (no duplicate entries target these
expansions; `B633E9B9` is a distinct, deferred authorization-boundary feature).
P-021 C5(B) late-identifier reconciliation: **NO-OP** for all six (every entry
carries fully populated refs: task/feature/shipment/pr=#425/thread/review-comment;
no `N/A`).

## Feature A — 168-F (shipment 149-S)

### 168.001-T (rewritten, declaration-only)

`trust_anchors` config type declaration + go/ast source-shape harness ONLY. No
loader/validation/gate behavior. Domain: config (declaration).
**AC:** (1) go/ast source-shape harness passes asserting the `TrustAnchor` struct
and all fields (id, role, algo, public_key_ref, fingerprint, status, not_before,
not_after) are declared on `WorkspaceConfig`; (2) `go build` compiles; (3) the
task diff contains no loader/validation/pin-gate logic.

### 168.011-T (new, behavior)

`trust_anchors` loader + validation. Parse config into the 168.001-T structs;
validate id-uniqueness, algo allowlist (ed25519/ecdsa-p256), non-empty
public_key_ref. Domain: core. TDD: RED table-driven harness before impl.
Depends on 168.001-T.
**AC:** (1) RED tests written first and observed failing; (2) a valid config loads
into structs; (3) duplicate id, disallowed algo, and empty public_key_ref are each
rejected with a typed validation error; (4) tests GREEN post-impl.

### 168.012-T (new, behavior)

`trust_anchors` external-pin gate (fail-closed). Each anchor fingerprint MUST be
in the external pin set; absent pin set => feature disabled / resolver denies.
Domain: core. TDD: RED harness first. Depends on 168.011-T.
**AC:** (1) RED test first; (2) an anchor whose fingerprint is not pinned is
rejected; (3) an absent external pin set fails closed (feature disabled), proven by
negative test; (4) tests GREEN.

### 168.002-T (AC backfill; dep retargeted 168.001 -> 168.012)

Verification-key material loader (public-only).
**AC:** (1) a valid public key (inline PEM or no-follow real-path-contained file)
parses per algo; (2) a private key or malformed material is rejected with a typed
error; (3) a loaded-key fingerprint that mismatches the pinned fingerprint fails
closed; (4) negatives cover each rejection.

### 168.003-T (AC backfill)

Trusted-key resolver primitive (rotation + revocation), pure.
**AC:** (1) returns the active key inside not_before/not_after with status=active;
(2) expired/not-yet-valid, revoked, and ambiguous inputs each return a typed error;
(3) pure (no I/O), proven by table-driven tests.

### 168.004-T (AC backfill)

Read-only trust-anchor inspection CLI (list/show/verify-key).
**AC:** (1) list/show/verify-key print resolver output; (2) commands perform no
writes; (3) verify-key exits non-zero on an untrusted/unknown key.

### 168.005-T (rewritten, core mutation only; split from CLI)

Trust-anchor mutation core: config+status mutation, external-pin policy
enforcement, durable audit event. No CLI parsing. Domain: core. TDD: RED harness
first. Depends on 168.003-T (preserved from the pre-correction DAG), 168.012-T
(added). **2-hour / audit-seam rationale (review P3 remediation):** add/rotate/revoke
share ONE parametrized mutation path (table-driven test, single core function), and
the durable audit event is kept INSIDE this core task — not split out — because audit
persistence must be transactionally atomic with the state change it records. This
faithfully follows stash 71F5C21F, which groups "config mutation, external-pin policy
enforcement, audit-event creation/persistence" as one core task and separates only
the CLI wiring. The task stays within the 2-hour envelope via the shared parametrized
path.
**AC:** (1) RED test first; (2) add/rotate/revoke mutate config+status; (3) a
mutation introducing an unpinned fingerprint is rejected (fail-closed); (4) each
successful mutation persists a durable audit event; (5) tests GREEN.

### 168.013-T (new, thin CLI wiring)

`backlogit trust-anchor add|rotate|revoke` CLI: parse args/flags and delegate to
the 168.005-T core. No security-state logic in the CLI layer. Domain: cli.
Depends on 168.005-T.
**AC:** (1) each subcommand invokes the core mutation and surfaces its typed
result/exit code; (2) the CLI layer contains no pin-gate/audit logic; (3) a
rejected core mutation yields a non-zero exit with no partial write.

### 168.006-T (AC backfill; dep retargeted 168.005 -> 168.013)

Operator docs: trust-anchor lifecycle + threat model.
**AC:** (1) covers public-only custody, external-pin root of trust, rotation,
revocation, and the residual it does/does not close; (2) references the CLI surface
(168.013-T); (3) markdownlint passes.

## Feature B — 169-F (shipment 150-S)

### 169.001-T (rewritten, declaration-only)

Attestation envelope type declaration (DSSE/in-toto over shipment-id + merge-sha +
manifest-digest + closure-content-hash) + go/ast source-shape harness ONLY. No
parser behavior. Domain: core (declaration).
**AC:** (1) source-shape harness asserts the envelope + statement types/fields are
declared; (2) `go build` compiles; (3) no parsing/verification behavior in the
diff (parser is 169.010-T).

### 169.010-T (new, parser behavior)

Attestation envelope parser rejecting malformed/duplicate-member JSON via
raw-token scan (mirroring the #423 validator). Domain: core. TDD: RED harness
first. Depends on 169.001-T.
**AC:** (1) RED test first; (2) a well-formed envelope parses; (3) malformed JSON
and duplicate-member JSON are each rejected with a typed error; (4) tests GREEN.

### 169.002-T (AC backfill; dep retargeted 169.001 -> 169.010)

Attestation verification primitive (fail-closed).
**AC:** (1) a valid signature over the canonical statement using a 168.003-resolved
key verifies; (2) statement evidence fields must equal the reconcile-computed
evidence or fail; (3) any signature/binding/expiry mismatch returns a typed
fail-closed error.

### 169.003-T (AC backfill)

Event binding + doctor recognition of signed repair.
**AC:** (1) attestation{key_id,signer_identity,sig_algo,statement_digest} added to
the `shipment_reconciled_shipped` delta and threaded through
evidence_digest/event_digest; (2) attestation folds into event_digest but NOT
request_identity_digest; (3) a doctor-LOCAL branch distinguishes signed vs unsigned
reconcile; (4) the presence-based branch is explicitly the RED baseline superseded
by 169.007-T.

### 169.004-T (AC backfill)

`reconcile-shipped --attestation` flag wiring + dry-run.
**AC:** (1) opt-in `--attestation <path>` flag; (2) dry-run prints verification
outcome; (3) absent/unconfigured = #423 v1 behavior unchanged (test-proven).

### 169.005-T (AC backfill)

Integration + negative tests (signed 048-S-shaped fixture).
**AC:** (1) signed happy path over a 048-S-shaped fixture passes; (2) tampered
statement, wrong-key, expired/revoked key, and wrong-shipment attestation are each
rejected.

### 169.006-T (AC backfill)

Operator docs: attestation generation (CI) + verify.
**AC:** (1) covers CI-side attestation generation and the `--attestation` verify
path; (2) any future policy-enforce mode marked NOT-YET-AVAILABLE; (3) markdownlint
passes.

## Feature C — 170-F (shipment 151-S)

### 170.011-T (new, blocking design/declaration predecessor)

Design: request-identity-digest contract + operator pre-commit issuance flow.
Define the request-identity-digest field set, canonical byte encoding, and operator
pre-commit token-issuance flow so issuance and verification are fully specified
before Feature C implementation. Domain: docs (design-doc). Blocks 170.001-T and
170.002-T.

**Producer linkage (Architecture-review P1 remediation).** `request_identity_digest`
is *produced* by the existing reconcile/event path inside the locked 167.008-T
critical section (shipped via 148-S). 170.011-T does NOT redefine that producer's
encoding; it references the producer as the authoritative source of the digest and
layers the operator token issuance/verification contract on top of it. A `references`
edge to 167.008-T (not a blocking dependency, since 167-F is already shipped) records
the linkage so the contract cannot drift from the producer. Because verification
(170.002-T) binds to this digest fail-closed, the contract MUST assert byte-for-byte
alignment with the producer.

**Cross-feature decoupling (Architecture-review P2 resolution).** 169-F does NOT
consume this token-issuance contract: 169.003-T explicitly keeps attestation OUT of
`request_identity_digest` (attestation folds into event_digest only). 169-F therefore
relies only on `request_identity_digest` existing as a separate reconcile-path digest,
not on 170.011-T's issuance encoding. This is why 170.011-T is correctly scoped inside
170-F / 151-S with NO ordering edge into 150-S; the sibling-shipment independence is
intentional, not an unstated gap.

**AC:** (1) design doc enumerates the exact request-identity-digest input fields
and their order, byte-for-byte aligned with the 167.008-T reconcile-path producer;
(2) specifies a canonical, deterministic byte encoding and asserts it matches the
producer's encoding (no independent re-canonicalization); (3) specifies the operator
pre-commit issuance flow (who mints, what is signed, TTL and nonce handling); (4) is
referenced by 170.001-T and 170.002-T and references the 167.008-T producer; (5)
markdownlint passes.

### 170.001-T (rewritten, declaration-only)

Authorization-token type declaration (signed binding of {shipment_id,
request_identity_digest, not_after, nonce}) + go/ast source-shape harness ONLY. No
parser behavior. Domain: core (declaration). Depends on 170.011-T.
**AC:** (1) source-shape harness asserts the token struct + fields are declared per
the 170.011-T contract; (2) `go build` compiles; (3) no parsing/verification
behavior in the diff (parser is 170.012-T).

### 170.012-T (new, parser behavior)

Authorization-token parser over the 170.001-T declared shape. Domain: core. TDD:
RED harness first. Depends on 170.001-T.
**AC:** (1) RED test first; (2) a well-formed token parses into the declared
struct; (3) malformed/duplicate-member token input is rejected with a typed error;
(4) tests GREEN.

### 170.002-T (AC backfill; dep retargeted 170.001 -> 170.012; +170.011)

Token verification primitive (fail-closed).
**AC:** (1) the token signature verifies via a 168.003-resolved key; (2) binding to
the target shipment-id AND the reconcile request-identity digest (per the 170.011-T
contract) is enforced; (3) not_after is enforced; (4) any mismatch returns a typed
fail-closed error.

### 170.003-T (AC backfill)

Nonce single-use ledger (anti-replay) [justify-or-fold].
**AC:** (1) durable, handle-safe append-then-fsync record of consumed nonces; (2) a
replayed token is rejected fail-closed; (3) consumed nonces are pruned past token
not_after; (4) a planning note records why request-identity binding + not_after is
insufficient (else the store folds into 170.002-T).

### 170.004-T (AC backfill)

Core authenticated-approval gate integration (in 167.008-T locked section).
**AC:** (1) inside the 167.008-T locked section, a 170.002/170.003-verified token is
required in place of self-suppliable --confirm/TTY when a policy is configured; (2)
the authenticated approver identity persists into the prepared event; (3)
unconfigured = v1 confirmation (test-proven).

### 170.005-T (AC backfill)

`reconcile-shipped --authorization-token` surface + dry-run.
**AC:** (1) opt-in `--authorization-token <path>` flag; (2) dry-run shows the
authorization verification outcome; (3) unconfigured = #423 v1 confirmation
behavior.

### 170.006-T (AC backfill)

Integration + negative tests (authenticated approval).
**AC:** (1) a valid token authorizes; (2) self-minted (no trusted key), expired,
wrong-shipment, wrong-request-identity, and replayed tokens are each rejected.

### 170.007-T (AC backfill)

Operator docs + 167.015-T linkage note.
**AC:** (1) documents the token issuance/verification flow; (2) states Feature C
upgrades confirmation-only v1 to authenticated authorization but does NOT itself
close 167.015-T; (3) markdownlint passes.

## Corrected dependency graph (deltas only)

Edges removed:

* 168.002-T depends-on 168.001-T -> retarget to 168.012-T
* 168.007-T depends-on 168.001-T -> retarget to 168.012-T
* 168.010-T depends-on 168.001-T -> retarget to 168.012-T
* 168.006-T depends-on 168.005-T -> retarget to 168.013-T
* 169.002-T depends-on 169.001-T -> retarget to 169.010-T
* 170.002-T depends-on 170.001-T -> retarget to 170.012-T

Edges added:

* 168.011-T depends-on 168.001-T
* 168.012-T depends-on 168.011-T
* 168.002-T depends-on 168.012-T
* 168.007-T depends-on 168.012-T
* 168.010-T depends-on 168.012-T
* 168.005-T depends-on 168.012-T
* 168.013-T depends-on 168.005-T
* 168.006-T depends-on 168.013-T
* 169.010-T depends-on 169.001-T
* 169.002-T depends-on 169.010-T
* 170.001-T depends-on 170.011-T
* 170.012-T depends-on 170.001-T
* 170.002-T depends-on 170.012-T
* 170.002-T depends-on 170.011-T

Edges preserved (restated for delta completeness, Correctness-review P3):

* 168.005-T depends-on 168.003-T (pre-existing; retained through the 168.005-T rewrite)

Reference-only linkage (not a blocking edge; 167-F is shipped via 148-S):

* 170.011-T references 167.008-T (request_identity_digest producer)

Release-unit edges preserved: 150-S depends-on 149-S; 151-S depends-on 149-S;
150-S/151-S depends-on 148-S.

## Shipment membership deltas

* 149-S: add 168.011-T, 168.012-T, 168.013-T (feature 168-F already present)
* 150-S: add 169.010-T (feature 169-F already present)
* 151-S: add 170.011-T, 170.012-T (feature 170-F already present)

## Plan Hardening Assessment

**Requires plan hardening: no.** This is a decomposition-and-acceptance-criteria
correction of an already-hardened, already-reviewed plan (the base plan passed
review PASS-after-remediation; the security vectors 3B661FCE / BFF76433 / 4E210DB4
are already encoded in the untouched hardening tasks 168.007-010, 169.007-009,
170.008-010 with fail-closed AC). The corrections split declaration from behavior,
add checkable AC, and add a design predecessor; none introduce new security surface
or relax an existing fail-closed control. RED-first ordering is preserved for every
behavior-bearing split. No hardening signals are introduced; proceed to plan review.

## Plan Review

<!-- plan-review-attempt: 1 -->

* **dispatch_mode:** multi-agent-dispatch
* **decision:** PASS (after remediation)
* **reviewers:** Scope Boundary Auditor, Correctness Reviewer, Architecture Strategist
  (three independent agents, dispatched in parallel on this plan document).

### Raw reviewer verdicts

| Reviewer | Verdict | Findings |
|---|---|---|
| Scope Boundary Auditor | PASS | All six entries traced; ACs checkable; DAG-rewire justified; no YAGNI. Two P3 advisories (168.003-T "ambiguous" wording; confirm 170.011-T reference enforcement). |
| Correctness Reviewer | ADVISORY | Reconstructed DAG is ACYCLIC; all predecessor chains valid; no dangling refs; declaration/behavior split sound; RED-first preserved. P3: 168.005->168.003 edge absent from delta list; 168.005-T / 168.011-T press the 2h test-scenario ceiling. |
| Architecture Strategist | ADVISORY | Declaration/behavior seams and core/CLI boundary cut correctly; no behavior leaks. P1: 170.011-T "No deps" vs request_identity_digest producer (drift risk). P2: cross-feature contract scoping 169-F/170-F. P3: audit-seam rationale note. |

### Remediations applied before gate close

1. **Architecture P1** — 170.011-T rewritten: removed unqualified "No deps"; added a
   `references` linkage to the 167.008-T request_identity_digest producer; AC now
   asserts byte-for-byte alignment with the producer's encoding, closing the
   contract-vs-producer drift risk that would otherwise fail-close authenticated
   approvals.
2. **Architecture P2** — recorded the explicit cross-feature decoupling: 169-F does
   NOT consume the token-issuance contract (169.003-T keeps attestation out of
   request_identity_digest), so 170.011-T correctly stays in 151-S with no ordering
   edge into 150-S. Sibling-shipment independence is now intentional and documented.
3. **Architecture P3 / Correctness P3 (2h)** — added the audit-seam atomicity
   rationale to 168.005-T: add/rotate/revoke share one parametrized mutation path and
   audit persistence is intentionally in-core for transactional atomicity, faithful to
   stash 71F5C21F. Task stays within 2h.
4. **Correctness P3 (delta completeness)** — added the preserved 168.005->168.003 edge
   and the 170.011->167.008 reference to the dependency-delta section.
5. **Scope P3 advisories** — non-blocking; 170.011-T reference enforcement is wired via
   the added 170.002->170.011 edge and the 167.008 reference; 168.003-T "ambiguous"
   wording left as-is (testable via the enumerated expired/revoked negatives).

No reviewer returned FAIL. All P1/P2 findings resolved; residual items are
non-blocking P3 advisories. Gate decision: **PASS**.

* **operator_authorization:** approved (operator delegated this Stage session with
  explicit direction to resolve blocking plan findings within Stage scope and hand back
  a reviewed trust-chain; no blocking/FAIL findings remained after remediation).

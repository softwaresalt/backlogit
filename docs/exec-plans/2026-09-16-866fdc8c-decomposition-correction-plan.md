---
chunk_strategy: h1-h2-h3
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-16-866fdc8c-decomposition-correction-plan.md
title: "Correction Plan: 866FDC8C trust-chain decomposition (149-S/150-S/151-S)"
---

# Correction Plan: 866FDC8C trust-chain decomposition

## Overview

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
The **ten** security-hardening tasks (168.007-010 [4], 169.007-009 [3],
170.008-010 [3] = 10) already carry explicit AC and RED-first ordering. Their
security *behavior* is unchanged, but two of them — **168.007-T and 168.010-T** —
had their upstream declaration dependency retargeted (168.001-T -> 168.012-T) as a
consequence of the declaration/behavior split, and 168.010-T's RED-ordering prose
was synchronized with its frontmatter (see item-5 remediation). All other
hardening tasks are unchanged.

## Constitution Check

This correction is validated (mandatory, NON-NEGOTIABLE)
against `.github/instructions/constitution.instructions.md`:

| Principle | Check | Result |
|---|---|---|
| I. Safety-First Go | No source written by Stage; typed-error/fail-closed AC preserved on every behavior task | PASS |
| II. Test-First (NON-NEGOTIABLE) | Every behavior-bearing split declares a RED harness observed failing before impl; declaration tasks carry a source-shape (go/ast) harness | PASS |
| III. Workspace Isolation / Security Boundaries | `trust_anchors` parsing/schema validation placed in `internal/config`; policy/resolution/mutation in `internal/core` (item 10); external-pin gate fail-closed | PASS |
| IV. CLI Workspace Containment (NON-NEGOTIABLE) | CLI split into thin wiring tasks (168.013-T) delegating to core; no security-state logic in CLI layer. REASSESSED (cycle-3 item 4) for external protected state: the external-pin ROOT OF TRUST is a read-only trust reference, not a filesystem write target; every mutable filesystem WRITE (config, status, durable audit, nonce ledger projection, indeterminate marker) stays within the workspace `.backlogit` tree; external revocation/tombstone (168.009-T) and rollback-resistant nonce compare-and-consume (170.008-T) authority is modeled through an INJECTED NON-FILESYSTEM provider (or a signed/read-only authority) — NO mutable filesystem write outside the workspace is authorized; the workspace holds only a contained projection. The `public_key_ref` structural contract (168.018-T) rejects absolute paths, `..`, and URL schemes so no ref escapes containment | PASS |
| V. Structured Observability | Durable audit event required on every trust-anchor mutation (168.005-T); write-outcome taxonomy + fail-closed audit + operator-visible indeterminate marker, incl. denied-path `ErrAuditNotPersisted` (168.017-T; 168.019-T withdrawn and folded, cycle-3 item 13) | PASS |
| VI. Single Responsibility | Declaration split from behavior; parser split from verification; every task single-domain (config OR core OR cli OR docs) | PASS |
| VII. Destructive Command Approval (NON-NEGOTIABLE) | REASSESSED (cycle-3 item 3): trust-anchor REVOKE / ROTATE / tombstone ARE destructive security-state mutations. Explicit operator approval is ALWAYS required; any safety-mode gate is ADDITIONAL, never an alternative to approval. The core refuses the mutation unless presented with VERIFIABLE authorization EVIDENCE (a signed/verifiable operator-approval token or capability the core validates) — an unproven caller boolean is NOT authorization evidence — recorded as a binding AC on the mutation surface (168.013-T CLI produces the evidence + 168.005-T core validates it). Stage itself runs no destructive commands; governed backlogit mutations only; append-only archive untouched | PASS (classification recorded) |
| VIII. Explicit Safety Modes | No elevated-risk mode introduced; destructive security-state mutations gated by the VII approval classification; no relaxation of existing fail-closed controls | PASS |
| IX. Git-Friendly Persistence | All backlog artifacts, plan, and memory are line-oriented Markdown/JSONL written by governed writers with deterministic ordering; append-only archive history is never rewritten | PASS |
| X. Agent Context Efficiency | Every task bounded to <=3 executable scenarios and a single domain; the scenario matrix (item 9) keeps leaves small; plan/memory kept concise | PASS |
| Task Granularity (NON-NEGOTIABLE) | Every task re-scoped to <3 files / <5 funcs / <4 test scenarios; tasks confirmed at 4+ scenarios split (item 9 matrix) | PASS |
| XI. Merge Commit History Preservation (NON-NEGOTIABLE) | NOT N/A: 149-S -> 150-S -> 151-S ship serially over shared reconcile/event surfaces. Downstream Ship PRs for these shipments MUST merge via MERGE COMMITS; squash and rebase merges are PROHIBITED so per-shipment history is preserved | PASS (constraint recorded) |

No principle fails; the IV / VII / XI reassessments are recorded constraints, not deviations.

Constitution Check: pass

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
not_after) are declared on `WorkspaceConfig`; (2) `go build ./cmd/backlogit` compiles and
`go test ./internal/config/ -run TestTrustAnchorSourceShape` passes (source-shape
harness); (3) the
task diff contains no loader/validation/pin-gate logic.

### 168.011-T (new, behavior)

`trust_anchors` loader + validation. Parse config into the 168.001-T structs;
validate id-uniqueness, algo allowlist (ed25519/ecdsa-p256), non-empty
public_key_ref. **Domain: config (`internal/config` — parsing/schema validation
only, no trust policy; item-10 + Architecture-P1 remediation).** TDD: RED
table-driven harness before impl. Depends on 168.001-T.
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

Trusted-key resolver primitive (rotation), pure.
**AC:** (1) returns the active key inside not_before/not_after with status=active;
(2) expired and not-yet-valid inputs each return a typed error; (3) pure (no I/O),
proven by table-driven tests; (4) revoked and ambiguous (multiple-active) resolver
paths are delegated to **168.015-T**; (5) role-scoped resolution (Security-F1) and
pinned-algo binding (Security-F4) are delegated to **168.020-T** (cycle-2 width
split) — this origin task no longer enumerates that behavior.

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
persistence is the final step of the same sequenced commit that records the state
change. This faithfully follows stash 71F5C21F, which groups "config mutation,
external-pin policy enforcement, audit-event creation/persistence" as one core task
and separates only the CLI wiring. The task stays within the 2-hour envelope via the
shared parametrized path.

**Deterministic ordering (item-3 cycle-1 + item-4 cycle-2 correction).** The core
mutation path executes a fixed sequence: (a) validate args -> (b) external-pin policy
gate (fail-closed) -> (c) apply config mutation -> (d) apply status mutation -> (e)
persist durable audit event. **Cycle-2 atomicity correction (Architecture-P2):** this
sequence is NOT a true cross-file atomic commit — a config file + status + separate
audit log cannot be atomically committed on a filesystem. 168.005-T owns the
deterministic ORDERING and returns the base `WriteOutcome` type; the marker-protected
sequenced-commit protocol, the durable **write-outcome taxonomy**
(`ErrWriteNotApplied` no state changed / retry-safe vs `ErrWriteIndeterminate` state
may be partially applied / retry-unsafe), retry/compensation boundaries, and the
recovery-visibility marker are OWNED by **168.017-T**; denied-path
audit-write-failure handling is folded into **168.017-T** (cycle-3 item 13: the cycle-2
168.019-T is withdrawn; its minimal denied-path `ErrAuditNotPersisted` fail-closed
outcome moves into 168.017-T, and the broad crash-recovery / doctor auto-reconciliation
subsystem is deferred to P-021 as out of scope, not implemented in 149-S/150-S/151-S).
This keeps
each task within the 2-hour/single-domain limit and removes the atomic-vs-indeterminate
contradiction (guarantee language lives with the mechanism in 168.017-T).

**Constitution VII (cycle-3 item 3) — destructive-op authorization.** rotate and revoke
are destructive security-state mutations. Explicit operator approval is ALWAYS required,
and any safety-mode gate is ADDITIONAL, never an alternative. The core refuses to apply a
rotate/revoke unless it is presented with VERIFIABLE authorization EVIDENCE — a
signed/verifiable operator-approval token or capability the core itself validates — NOT a
bare caller-supplied boolean the core would have to trust; an unproven caller boolean is
not authorization evidence, so even a programmatic core caller cannot forge approval.
**AC:** (1) RED test first; (2) add/rotate/revoke mutate config+status in fixed
order (a)->(e); (3) an unpinned-fingerprint mutation is rejected (fail-closed) at
step (b) before any state change **AND the denied mutation persists a durable audit
event (Security-F2 remediation: denials are audited, not only successes)**; (4) each
successful mutation persists a durable audit event as the final ordered step (NOT
claimed cross-file atomic; recoverability owned by 168.017-T); (5) a rotate/revoke
without VERIFIABLE operator-approval evidence (bare boolean, absent, invalid, or forged)
is refused with a typed error and no state change (Constitution VII); (6) the core
declares and returns the base `WriteOutcome` type consumed by 168.017-T; (7) tests GREEN.

### 168.013-T (new, thin CLI wiring)

`backlogit trust-anchor add|rotate|revoke` CLI: parse args/flags and delegate to
the 168.005-T core. No security-state logic in the CLI layer. Domain: cli.
Depends on 168.005-T.
**AC:** (1) **RED test first (Correctness-P3 remediation)**; (2) each subcommand
invokes the core mutation and surfaces its typed result/exit code; (3) **rotate and
revoke are destructive security-state operations and require an explicit operator
approval / safety-mode gate BEFORE the core mutation is invoked — without approval the
CLI refuses with a non-zero exit and invokes no core mutation (Constitution VII,
cycle-2), proven by a RED-first negative test;** (4) the CLI layer contains no
pin-gate/audit logic (delegates to core); (5) a rejected core mutation yields a
non-zero exit with no partial write.

### 168.006-T (AC backfill; dep retargeted 168.005 -> 168.013)

Operator docs: trust-anchor lifecycle + threat model.
**AC:** (1) covers public-only custody, external-pin root of trust, rotation,
revocation, and the residual it does/does not close; (2) references the CLI surface
(168.013-T); (3) **documents external pin-set provenance/integrity expectations —
either the authenticity control on pin-set contents or an explicit statement that
pin-set integrity is an out-of-scope external trust assumption (Security-F6
remediation);** (4) markdownlint passes.

## Feature B — 169-F (shipment 150-S)

### 169.001-T (rewritten, declaration-only)

Attestation envelope type declaration (DSSE/in-toto over shipment-id + merge-sha +
manifest-digest + closure-content-hash) + go/ast source-shape harness ONLY. No
parser behavior. Domain: core (declaration).
**AC:** (1) source-shape harness asserts the envelope + statement types/fields are
declared; (2) `go build ./cmd/backlogit` compiles and
`go test ./internal/core/ -run TestAttestationEnvelopeSourceShape` passes; (3) no parsing/verification behavior in the
diff (parser is 169.010-T).

### 169.010-T (new, parser behavior)

Attestation envelope parser rejecting malformed JSON and EXACT-BYTE duplicate
members via raw-token scan (mirroring the #423 validator). Domain: core. TDD: RED
harness first. Depends on 169.001-T.
**AC:** (1) RED test first; (2) a well-formed envelope parses; (3) malformed JSON is
rejected AND EXACT-BYTE duplicate JSON members are rejected with a typed error at
EVERY concrete object depth (envelope root, statement, subject, predicate/evidence),
proven by negative tests per depth; (4) decoder-equivalent (case-fold / Unicode-escape)
duplicate collisions at every depth are delegated to **169.015-T** (cycle-2 split);
(5) tests GREEN.

### 169.002-T (AC backfill; dep retargeted 169.001 -> 169.010)

Attestation verification primitive (fail-closed).
**AC:** (1) a valid signature over the canonical statement using a 168.003-resolved
key verifies; (2) statement evidence fields must equal the reconcile-computed
evidence or fail; (3) any signature/evidence mismatch returns a typed fail-closed
error; (4) role-scoped resolution + pinned-algo enforcement (Security-F1/F4) and the
fail-closed rejection audit (Security-F2) are delegated to **169.013-T** (cycle-2
width split) — this origin task no longer enumerates that behavior; (5) binding/expiry
mismatch delegated to **169.011-T**.

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
170.002-T. **Exact output path (item-6 remediation):**
`docs/design-docs/2026-09-16-866fdc8c-request-identity-digest-contract.md` — a
forward-reference deliverable produced by Ship during 170.011-T execution (Stage
does NOT create it now); referenced by 170.001-T and 170.002-T.

**Digest/lock sequencing (item-6 cycle-1 + item-3 cycle-2 correction).** Grounded in
authoritative code, the contract MUST specify this TARGET ordering:
`shipmentReconcileRequestIdentityDigestForNormalized` is computed as PRE-LOCK,
pure/deterministic work over the normalized delta BEFORE any membership or
persistence lock is taken (`internal/core/shipment_reconcile_transaction.go` ~line
76). Then Phase-A membership locking is acquired; then per-artifact persistence locks
are taken inside the critical section; the pre-computed digest is threaded THROUGH
the critical section. **Cycle-2 code-grounding correction:** the current code does
NOT yet match this target — `prepareShipmentReconcileEvidence` REDUNDANTLY recomputes
the digest under the Phase-A membership lock at
`internal/core/shipment_reconcile_evidence.go:186`. The earlier plan claim that the
digest is "never recomputed under lock" was FALSE. This docs task specifies the
drift-free target; per cycle-3 item 12 the redundant under-lock recomputation is left in
place and documented as drift-free (see the WITHDRAWN note below).
**170.015-T is WITHDRAWN**: because the under-lock recomputation is
DETERMINISTIC over the same normalized delta it yields a byte-identical digest and
introduces NO drift, so the design DOCUMENTS the current authoritative
producer/recompute behavior accurately rather than mandating a code change. A docs-only
task does not claim behavior no task owns; it describes existing behavior only.

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
pre-commit issuance flow (who mints, what is signed incl. not_before/issued-at, TTL
and nonce handling); (4) is
referenced by 170.001-T and 170.002-T and references the 167.008-T producer; (5)
markdownlint passes.

### 170.001-T (rewritten, declaration-only)

Authorization-token type declaration (signed binding of {shipment_id,
request_identity_digest, not_before, not_after, nonce} — **not_before/issued-at
added per Security-F5**) + go/ast source-shape harness ONLY. No
parser behavior. Domain: core (declaration). Depends on 170.011-T.
**AC:** (1) source-shape harness asserts the token struct + fields are declared per
the 170.011-T contract; (2) `go build ./cmd/backlogit` compiles and
`go test ./internal/core/ -run TestAuthorizationTokenSourceShape` passes; (3) no parsing/verification
behavior in the diff (parser is 170.012-T).

### 170.012-T (new, parser behavior)

Authorization-token parser over the 170.001-T declared shape. Domain: core. TDD:
RED harness first. Depends on 170.001-T.
**AC:** (1) RED test first; (2) a well-formed token parses into the declared
struct; (3) malformed token input is rejected AND EXACT-BYTE duplicate JSON members
are rejected with a typed error at EVERY concrete object depth (token root, header,
claims/payload, binding/evidence), proven by negative tests per depth; (4)
decoder-equivalent (case-fold / Unicode-escape) duplicate collisions at every depth
are delegated to **170.018-T** (cycle-2 split); (5) tests GREEN.

### 170.002-T (AC backfill; dep retargeted 170.001 -> 170.012; +170.011)

Token verification primitive (fail-closed).
**AC:** (1) the token signature verifies via a 168.003-resolved key; (2) binding to
the target shipment-id is enforced (wrong shipment fails closed with a typed error);
(3) the token VALIDITY WINDOW is enforced — both not_before/issued-at (not-yet-valid)
and not_after (expired) fail closed with typed errors (cycle-3 item 14: not_before
folded here into the validity-window seam); (4) role-scoped resolution +
pinned-algo enforcement (Security-F1/F4) and the fail-closed rejection audit
(Security-F2) are delegated to
**170.016-T** (cycle-2 width split; not_before no longer delegated there); (5) the request-identity-digest binding AND the
producer GOLDEN cross-check against actual 167.008-T output are delegated to
**170.013-T** (cycle-2 item 5) — this origin task no longer enumerates that behavior.

### 170.003-T (AC backfill)

Nonce single-use ledger (anti-replay) — **MANDATORY (Security-F3 remediation;
request-identity + not_after binding does not prevent replay within the TTL
window).** TDD: RED harness first.
**AC:** (1) RED test first; (2) durable, handle-safe append-then-fsync record of
consumed nonces; (3) a replayed token whose nonce is already recorded is rejected
fail-closed with a typed replay error; (4) consumed nonces are pruned past token
not_after; (5) the ATOMIC check-then-consume seam and the consume-audit event are
delegated to the single nonce-consume owner **170.008-T** (cycle-3 item 11; the cycle-2
carve 170.017-T is WITHDRAWN and folded into 170.008-T).

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

Integration + POSITIVE/issuer/expiry partition (paired with 170.014-T). TDD: RED
harness first.
**AC:** (1) RED test first; (2) a VALID token from a trusted issuer bound to the
target shipment authorizes; (3) a SELF-MINTED token (untrusted/unpinned issuer) is
rejected fail-closed with a typed untrusted-issuer error; (4) an EXPIRED token is
rejected fail-closed with a typed expired-token error; (5) the {wrong-shipment,
wrong-request-identity, replay} negatives are owned by **170.014-T** under the exact
partition — no case overlap.

### 170.007-T (AC backfill)

Operator docs + 167.015-T linkage note.
**AC:** (1) documents the token issuance/verification flow; (2) states Feature C
upgrades confirmation-only v1 to authenticated authorization but does NOT itself
close 167.015-T; (3) markdownlint passes.

## Scenario-driven leaf splits (item 9)

Executable-scenario recount for the ten flagged tasks. Every task confirmed at **4+
executable scenarios** is split by carving the over-the-line scenario cluster into a
pure **leaf** task that depends only on its origin (nothing depends on the leaf), so
the existing DAG edges are untouched and acyclicity is structurally preserved. Only
shipment membership grows.

### Scenario matrix

| Task | Pre-count | Verdict | Leaf carved | Post-count (origin / leaf) |
|---|---|---|---|---|
| 168.002-T | 4 (parse-per-algo, reject-private/malformed, fingerprint-mismatch, negatives) | SPLIT | 168.014-T (fingerprint pin-binding + mismatch fail-closed) | 3 / 2 |
| 168.003-T | 4 (active-in-window, expired/not-yet, revoked, ambiguous) | SPLIT | 168.015-T (revoked + ambiguous resolver paths) | 2 / 2 |
| 168.004-T | 4 (list, show, verify-key, no-writes) | SPLIT | 168.016-T (verify-key untrusted-key non-zero exit CLI) | 3 / 2 |
| 168.005-T | 5 (add, rotate, revoke, pin-reject, audit) | SPLIT (item 3) | 168.017-T (write-outcome taxonomy + recovery marker + injected-failure) | 3 / 3 |
| 168.011-T | 4 (valid-load, dup-id, bad-algo, empty-key-ref) | SPLIT | 168.018-T (public_key_ref shape/scheme validation) | 3 / 2 |
| 168.013-T | 2 (delegate+exit-code, no-partial-write on reject) | NO SPLIT | — | 2 |
| 169.002-T | 4 (valid-verify, evidence-equality, sig-mismatch, binding/expiry) | SPLIT | 169.011-T (binding + expiry fail-closed) | 3 / 2 |
| 169.005-T | 5 (happy, tampered, wrong-key, expired/revoked, wrong-shipment) | SPLIT | 169.012-T (expired/revoked + wrong-shipment negatives) | 3 / 2 |
| 170.002-T | 4 (sig-verify, shipment+digest binding, not_after, mismatch) | SPLIT | 170.013-T (request-identity-digest binding fail-closed) | 3 / 2 |
| 170.006-T | 5 (valid, self-minted, expired, wrong-shipment, wrong-request-identity/replay) | SPLIT | 170.014-T (authenticated-approval negative battery) | 3 / 2 |

**168.013-T rationale (NO split).** Recounted at exactly 2 executable scenarios: one
parametrized delegation assertion (add/rotate/revoke each invoke core and surface the
typed exit code) and one no-partial-write-on-reject assertion. Below the 4-scenario
ceiling; splitting would create an artificial single-scenario leaf. Retained whole.

### New leaf tasks (each a pure leaf; origin-only dependency)

| Leaf | Origin (depends-on) | Domain | Scope |
|---|---|---|---|
| 168.014-T | 168.002-T | core | Verification-key fingerprint pin-binding + mismatch fail-closed |
| 168.015-T | 168.003-T | core | Resolver revoked + ambiguous typed-error paths |
| 168.016-T | 168.004-T | cli | `verify-key` untrusted/unknown-key non-zero exit |
| 168.017-T | 168.005-T | core | Write-outcome taxonomy (`ErrWriteNotApplied`/`ErrWriteIndeterminate`), retry/compensation boundary, recovery-visibility marker, injected-failure tests |
| 168.018-T | 168.011-T | config | `public_key_ref` shape/scheme validation (internal/config) |
| 169.011-T | 169.002-T | core | Attestation binding + expiry fail-closed |
| 169.012-T | 169.005-T | core | Expired/revoked-key + wrong-shipment attestation negatives |
| 170.013-T | 170.002-T | core | Authorization-token request-identity-digest binding fail-closed |
| 170.014-T | 170.006-T | core | Authenticated-approval negative battery (self-minted/expired/wrong-shipment/replay) |

Each leaf carries `size: S`, `size_source: agent`, `size_ruleset_version:
stage-2h-rule-v1`, at least one checkable AC, RED-first ordering for the
behavior-bearing leaves, and a source-provenance comment where applicable.

**Note on scenario conservation (Correctness-P3).** Several splits are *refinements*
rather than strict partitions: carving a scenario cluster into a leaf sometimes adds
one focused negative scenario (e.g., 168.002 pre-4 -> origin-3 / leaf-2 = 5). The
governing invariant is that every resulting task lands at <=3 executable scenarios,
which every row satisfies; the pre/post columns are not a conservation identity.

### Enumerated leaf acceptance criteria (Scope-P2 remediation)

| Leaf | Checkable AC |
|---|---|
| 168.014-T | RED test first; a loaded key whose fingerprint equals the pinned fingerprint binds; a mismatch fails closed with a typed error; GREEN post-impl |
| 168.015-T | RED test first; a revoked anchor returns a typed revoked error; an ambiguous (multiple-active) input returns a typed ambiguity error; pure (no I/O) |
| 168.016-T | `verify-key` exits zero for a trusted key and non-zero for an untrusted/unknown key; performs no writes; error text names the untrusted key |
| 168.017-T | RED test first; `ErrWriteNotApplied` returned when no state changed (retry-safe) and `ErrWriteIndeterminate` when partial (retry-unsafe); a recovery-visibility marker is emitted on indeterminate; injected write-failure tests prove each branch |
| 168.018-T | RED test first; a well-formed `public_key_ref` (inline PEM or contained real-path) validates; empty/malformed/scheme-violating refs are each rejected with a typed error (internal/config) |
| 169.011-T | RED test first; an attestation bound to the target shipment within validity verifies; a wrong-shipment binding and an expired attestation each fail closed with a typed error |
| 169.012-T | RED test first; expired-key, revoked-key, and wrong-shipment attestations are each rejected fail-closed; each rejection is negative-tested |
| 170.013-T | RED test first; a token whose request-identity-digest matches the reconcile-computed digest binds; a mismatched digest fails closed with a typed error; a GOLDEN cross-check asserts the enforced digest equals actual 167.008-T producer output so encoding drift breaks THIS test not production |
| 170.014-T | RED test first; self-minted (untrusted-key), expired, wrong-shipment, and replayed tokens are each rejected fail-closed with distinct typed errors |

### Scenario matrix — cycle 2 recount (honest width, item 6)

Cycle-2 re-review recounted the security-expanded and newly surfaced tasks. Every
task confirmed at 4+ executable scenarios or carrying multiple distinct
implementation seams is split; the carved task is a pure leaf depending only on its
origin, so DAG acyclicity is structurally preserved and only shipment membership
grows.

| Task | Cycle-2 pre-count | Verdict | Carved task | Post (origin / carved) |
|---|---|---|---|---|
| 168.003-T | 5 (base 3 + F1 role-scope + F4 algo-pin) | SPLIT | 168.020-T (role-scoped + pinned-algo) | 3 / 3 |
| 168.017-T | 3 (pre-mut NotApplied, post-mut Indeterminate, denied-path audit-fail) | REDUCED (cycle-3 item 13) | 168.019-T WITHDRAWN/folded; broad auto-reconciliation deferred to P-021 | 3 / — |
| 169.002-T | 5 (base 3 + role/algo + fail-closed audit) | SPLIT | 169.013-T (role/algo + audit) | 3 / 3 |
| 169.007-T | 4 (persist-envelope, offline re-verify, tamper/swap, stale-branch removal) | SPLIT | 169.014-T (tamper/swap fail-closed + stale-branch absence) | 2 / 3 |
| 169.010-T | 4 (well-formed, malformed, exact-byte dup all depths, decoder-equiv dup all depths) | SPLIT | 169.015-T (case-fold/escape dup all depths) | 3 / 3 |
| 170.002-T | 5 (base 3 + role/algo + not_before + audit; golden moved out) | SPLIT | 170.016-T (role/algo + audit); not_before FOLDED into 170.002 validity window (cycle-3 item 14); golden -> 170.013-T | 3 / 3 |
| 170.003-T | 4 (durable record, replay-reject, prune, atomic-consume+audit) | FOLDED (cycle-3 item 11) | 170.017-T WITHDRAWN; atomic-consume+audit seam folded into nonce owner 170.008-T | 3 / — |
| 170.006-T / 170.014-T | 6 across the pair | RE-PARTITIONED (no new leaf) | exact partition {valid, self-minted, expired} / {wrong-shipment, wrong-request-identity, replay} | 3 / 3 |
| 170.012-T | 4 (well-formed, malformed, exact-byte dup all depths, decoder-equiv dup all depths) | SPLIT | 170.018-T (case-fold/escape dup all depths) | 3 / 3 |

**170.015-T WITHDRAWN (cycle-3 item 12).** The cycle-2 de-dup behavior task is REMOVED
as outside the authorized digest-design scope: `prepareShipmentReconcileEvidence`
recomputes `shipmentReconcileRequestIdentityDigestForNormalized` under the Phase-A
membership lock (`internal/core/shipment_reconcile_evidence.go:186`), but that recompute
is DETERMINISTIC over the same normalized delta and therefore byte-identical/drift-free,
so 170.011-T now DOCUMENTS the current authoritative behavior accurately rather than
mandating its removal. No behavior task is needed; the removal reduces scope.

**Hardening recount.** The ten hardening tasks (168.007-010, 169.007-009,
170.008-010) were recounted; 169.007-T exceeded four scenarios and was split
(169.014-T). 168.008-T and 170.008-T remain single-seam at <=3 scenarios (no
split). All ten now carry `size: S` / `size_source: agent` /
`size_ruleset_version: stage-2h-rule-v1`.

### New cycle-2 tasks (each a pure leaf/behavior; origin-only dependency)

| Task | Depends-on | Domain | Scope | Shipment |
|---|---|---|---|---|
| 168.020-T | 168.003-T | core | Resolver role-scoped (F1) + pinned-algo (F4) binding | 149-S |
| 169.013-T | 169.002-T | core | Attestation role-scoped + pinned-algo + fail-closed audit (F1/F4/F2) | 150-S |
| 169.014-T | 169.007-T, 169.008-T | core | Doctor attestation tamper/swap fail-closed + legacy-branch removal (single owner, core-only, cycle-3 item 10) | 150-S |
| 169.015-T | 169.010-T | core | Attestation JSON case-fold/escape duplicate rejection at every depth | 150-S |
| 170.016-T | 170.002-T | core | Token role-scoped + pinned-algo + fail-closed rejection audit + audit-write-failure contract (not_before folded to 170.002, cycle-3 item 14) | 151-S |
| 170.018-T | 170.012-T | core | Token JSON case-fold/escape duplicate rejection at every depth | 151-S |

Three cycle-2 tasks were WITHDRAWN in cycle 3 to remove over-expansion: **168.019-T**
(denied-path audit-fail folded into 168.017-T; broad recovery deferred to P-021, item 13),
**170.015-T** (digest de-dup out of scope; 170.011-T now documents the drift-free
behavior, item 12), and **170.017-T** (nonce atomic-consume+audit folded into the single
owner 170.008-T, item 11).

Each cycle-2 task carries `size: S`, `size_source: agent`,
`size_ruleset_version: stage-2h-rule-v1`, at least one checkable AC, and RED-first
ordering (a compiling failing harness observed before implementation) for the
behavior-bearing tasks. None is a docs-only exemption.

## Domain placement (item 10)

`trust_anchors` **parsing and schema validation** live in `internal/config`
(168.001-T declaration, 168.011-T loader/validation, 168.018-T `public_key_ref`
validation). **Policy, resolution, and mutation** live in `internal/core`
(168.003-T resolver, 168.005-T mutation, 168.012-T external-pin gate, 168.015-T
resolver negatives). The external-pin gate is a policy decision (core); config only
parses and structurally validates the declared anchors. This keeps the config layer
free of trust-policy logic and the core layer free of file-format concerns.

## Confirmed docs-only task set (item 8)

The **closed set** of confirmed docs-only, P-002.1-harness-exempt tasks is exactly:

| Task | Deliverable | Exemption basis |
|---|---|---|
| 168.006-T | Operator docs: trust-anchor lifecycle + threat model | Prose only; markdownlint is its gate; no Go source/behavior |
| 169.006-T | Operator docs: attestation generation + verify | Prose only; markdownlint gate |
| 170.007-T | Operator docs + 167.015-T linkage note | Prose only; markdownlint gate |
| 170.011-T | Design doc: request-identity-digest contract | Design/spec artifact under `docs/design-docs/`; markdownlint gate; no executable behavior |

These four carry the canonical label `harness-exempt` (plus the descriptive `docs-only`
label) and each embeds the exact P-002.1 harness-exemption-contract block
(delimited by `<!-- BEGIN:harness-exemption-contract -->` / `<!-- END:... -->`) with the
five canonical keys `harness_exemption_class: docs-only`, `harness_exemption_reason`,
`harness_owner: none`, `exempt_verification_command` (an exact runnable command), and
`exempt_precondition: must-fail-before-deliverable`. **Contract:**
a docs-only harness-exempt task MUST (1) produce only markdown/design artifacts, (2)
carry a runnable `exempt_verification_command` that fails before the deliverable exists
and passes after, and (3) contain NO Go source,
test, or config change. **No behavior- or test-bearing task is exempt** — every task
outside this closed set retains its RED/source-shape harness requirement. 170.011-T
qualifies solely because its deliverable is a specification document; the behavior it
specifies is implemented and harnessed by 170.001-T/170.012-T/170.002-T.

**Verification-only exempt task (P-002.1, separate from the docs-only set).** 169.009-T
is a `verification-only` harness-exempt task (cycle-3 item 10): it records green
re-verify/fail-closed evidence for behavior delivered by 169.007-T (persist) and
169.014-T (tamper/swap owner) and commits NO new red assertion. It carries the
`harness-exempt` label and the exact exemption-contract block with
`harness_exemption_class: verification-only` and `harness_owner: none`. Its previous
claim of OWNING tamper/swap detection is removed; the single owner is 169.014-T.

**Cycle-2 exemption note (item 7/8).** None of the six surviving cycle-2 tasks (168.020-T,
169.013-T, 169.014-T, 169.015-T, 170.016-T, 170.018-T) is docs-only; every one is
behavior-bearing and carries a RED-first compiling-harness requirement observed before
implementation. (The withdrawn 168.019-T, 170.015-T, and 170.017-T are removed in cycle
3.) The closed docs-only set above is unchanged at exactly four tasks.



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

Leaf-split edges added (item 9; each leaf is a pure sink — nothing depends on it, so
acyclicity is structurally preserved):

* 168.014-T depends-on 168.002-T
* 168.015-T depends-on 168.003-T
* 168.016-T depends-on 168.004-T
* 168.017-T depends-on 168.005-T
* 168.018-T depends-on 168.011-T
* 169.011-T depends-on 169.002-T
* 169.012-T depends-on 169.005-T
* 170.013-T depends-on 170.002-T
* 170.014-T depends-on 170.006-T

Edges preserved (restated for delta completeness, Correctness-review P3):

* 168.005-T depends-on 168.003-T (pre-existing; retained through the 168.005-T rewrite)

Cross-feature key-resolution edges (restated for delta completeness,
Correctness-review P3 cycle-1; satisfied at release-unit level via 150-S/151-S ->
149-S and enforced as task edges):

* 169.002-T depends-on 168.003-T (attestation verification uses a resolved key)
* 170.002-T depends-on 168.003-T (token verification uses a resolved key)

Reference-only linkage (not a blocking edge; 167-F is shipped via 148-S):

* 170.011-T references 167.008-T (request_identity_digest producer)

Release-unit edges preserved: 150-S depends-on 149-S; 151-S depends-on 149-S;
150-S/151-S depends-on 148-S.

**Cycle-2 serial-execution edge (item 2).** Added 151-S depends-on 150-S. Although
150-S (169-F attestation) and 151-S (170-F authorization) are dependency-independent
in feature logic, they touch shared reconcile/event surfaces
(`shipment_reconcile_evidence.go` / `shipment_reconcile_event.go` — e.g. the
request-identity digest recomputed deterministically in the reconcile evidence path and
the event binding upgraded by
169.007-T), so overlapping Ship work would race those surfaces. The chain is now
strictly serial 149-S -> 150-S -> 151-S through merge and closure under P-001.
Verified in-backlog: `151-S depends-on {149-S, 148-S, 150-S}`.

### Cycle-3 dependency corrections (item 9 — frontmatter/body/plan synchronized)

The cycle-2 leaf-split edges above are superseded for four tasks; the following are the
authoritative final task edges, matching each task's frontmatter and body prose:

* 169.012-T depends-on {169.005-T, 169.011-T, 168.015-T} (its expired/revoked/wrong-shipment
  cases require the attestation-verify behavior owner 169.011-T and the resolver-negative
  owner 168.015-T, added to the retained carve-origin edge 169.005-T).
* 170.006-T depends-on {170.002-T, 170.003-T, 170.013-T} (matches its required
  validity/nonce/digest-binding prerequisites; the edge to 170.005-T is removed).
* 170.014-T depends-on {170.002-T, 170.003-T, 170.013-T, 170.008-T} (depends on the
  digest-binding owner 170.013-T and the final nonce owner 170.008-T, not only 170.006-T).
* 170.004-T depends-on 170.008-T (wired to the single nonce compare-and-consume owner).
* 169.009-T depends-on 169.014-T (verification-only guard depends on the tamper/swap owner).

All other cycle-1/cycle-2 edges are unchanged. A full Kahn topological sort over the 53
in-scope nodes with these final edges yields a complete ordering (sorted = 53) => ACYCLIC.

## Cycle-2 shipment membership deltas (item 6) — revised by cycle 3

* 149-S: cycle-2 added 168.019-T, 168.020-T; cycle-3 WITHDREW 168.019-T -> net +1 (168.020-T)
* 150-S: add 169.013-T, 169.014-T, 169.015-T -> **16 items** (unchanged in cycle 3)
* 151-S: cycle-2 added 170.015-T, 170.016-T, 170.017-T, 170.018-T; cycle-3 WITHDREW
  170.015-T and 170.017-T -> net +2 (170.016-T, 170.018-T)

**Final verified membership after cycle 3: `149-S = 20, 150-S = 16, 151-S = 17`**
(feature + all children, confirmed via `backlogit` membership read and a Kahn DAG check
of 53 in-scope nodes = ACYCLIC). The stale cycle-1/cycle-2 final counts (149-S = 21 /
151-S = 19) are superseded by these cycle-3 figures.

## Shipment membership deltas

* 149-S: add 168.011-T, 168.012-T, 168.013-T, and leaf-splits 168.014-T, 168.015-T,
  168.016-T, 168.017-T, 168.018-T (feature 168-F already present) -> 19 items
* 150-S: add 169.010-T and leaf-splits 169.011-T, 169.012-T (feature 169-F already
  present) -> 13 items
* 151-S: add 170.011-T, 170.012-T and leaf-splits 170.013-T, 170.014-T (feature
  170-F already present) -> 15 items

Final verified membership: **149-S = 19, 150-S = 13, 151-S = 15** (feature + all
children, confirmed via `backlogit shipment get`). **[SUPERSEDED — cycle-1 snapshot;
the authoritative final counts are 149-S = 20, 150-S = 16, 151-S = 17 in the cycle-3
revised section above.]**

## Plan Hardening Assessment

**Requires plan hardening: yes.** Although this is a decomposition-and-acceptance-criteria
correction of an already-reviewed base plan, the corrected work directly governs
**authentication/authorization** (trust-anchor key resolution, attestation and token
verification), **destructive security-state mutation** (anchor rotate/revoke/tombstone),
**external integration** (non-filesystem revocation and nonce authorities), and
**rollback-resistant state** (single-consume nonce ledger). Under the plan-harden skill
these signals REQUIRE explicit hardening. The `## Plan Hardening` section below records the
ProposedAction / ActionRisk / approval / rollback / verification contracts for every
destructive or security-sensitive seam; only after it is complete does the plan proceed to
the plan-review gate.

## Plan Hardening

Every high-risk seam is modeled as a governed action with an explicit approval and
rollback/compensation contract. Destructive security-state operations (rotate, revoke,
tombstone) ALWAYS require explicit operator approval; a safety-mode classification is an
ADDITIONAL guard, never an alternative to approval. The core MUST treat authorization as
verifiable evidence threaded from the CLI boundary — never a bare caller-supplied boolean.

| ProposedAction | ActionRisk | Approval / classification | Rollback / compensation | Verification | Owner task |
|---|---|---|---|---|---|
| Rotate trust anchor (replace active key material) | HIGH — a wrong rotation can lock out valid signers or admit a rogue key | Explicit operator approval REQUIRED + destructive-safety classification (additional) | Prior anchor retained as revoked-not-erased; rotation is append-only so the previous active anchor is recoverable | Durable audit event on approve AND deny; RED test asserts unapproved rotate fails closed | 168.005-T (mutation), 168.013-T (CLI evidence) |
| Revoke trust anchor | HIGH — removes a trusted signer; irreversible trust withdrawal | Explicit operator approval REQUIRED + safety classification (additional) | Tombstone marker (revoked, not physically deleted) preserves audit lineage; no external-path erasure | Fail-closed resolver negative (168.015-T); audit event on deny and success | 168.005-T, 168.009-T |
| Tombstone / external revocation authority read | MEDIUM — external authority unavailability must fail closed, not open | No mutation of external protected state; authority modeled as a **non-filesystem, signed/read-only provider** injected into core; workspace holds only a contained projection | Workspace-contained projection is regenerable; no mutable write outside the workspace (Principle IV) | Injected-provider tests prove unreachable authority => fail closed | 168.009-T, 170.008-T |
| Atomic nonce compare-and-consume | HIGH — replay/double-spend if consume is non-atomic; rollback-resistant once consumed | No operator approval (automated verification path) but consume is single-writer atomic via injected provider | Consumed-but-audit-missing is an explicit INDETERMINATE outcome (no rollback that could re-enable replay); recovery marker emitted, replay stays blocked | RED tests: valid consume, replay reject, consumed-but-audit-missing indeterminate | 170.008-T (single owner) |
| Config/status mutation + durable audit commit | HIGH — a mutation whose audit is lost is unauditable security state | Ordered: mutate-then-audit; `ErrWriteNotApplied` (nothing changed, retry-safe) vs `ErrWriteIndeterminate` (partial, retry-unsafe) | On indeterminate, emit recovery-visibility marker; denied-path audit failure surfaces `ErrAuditNotPersisted`; broad auto-reconciliation deferred to P-021 (stash 6749D311) | Injected write-failure tests prove each branch (168.017-T) | 168.005-T, 168.017-T |

**Containment (Principle IV).** No planned operation writes mutable filesystem state outside
the workspace. External trust/revocation/nonce authority is modeled through an injected
non-filesystem provider or a signed read-only authority plus a workspace-contained projection;
168.009-T and 170.008-T are explicitly constrained to reject external-path writes.

**Approval evidence (Principle VII).** rotate/revoke/tombstone are destructive
security-state operations. The CLI (168.013-T) captures explicit operator approval and threads
it to core as verifiable evidence; core (168.005-T) MUST NOT trust an unproven caller boolean
as authorization. Safety-mode classification is additional and never substitutes for approval.

**Merge-history (Principle XI).** Downstream Ship PRs for 149-S/150-S/151-S MUST land as merge
commits; squash and rebase-merge are prohibited so the serial 149->150->151 history is preserved.

After hardening, RED-first ordering is preserved for every behavior-bearing split and no
existing fail-closed control is relaxed; the plan proceeds to the plan-review gate below.

## Plan Review

<!-- plan-review-attempt: 0 (cycle-0, superseded by BLOCKED verdict) -->
<!-- plan-review-attempt: 1 -->

* **dispatch_mode:** multi-agent-dispatch
* **decision:** PASS (after remediation)
* **reviewers:** Scope Boundary Auditor, Correctness Reviewer, Architecture
  Strategist, **Security Reviewer** (four independent agents dispatched in parallel
  on this plan document; the Security Reviewer is the mandatory security-sensitive
  persona for this trust-chain plan — item 16).
* **cycle:** review-fix cycle 1 (prior cycle-0 record superseded by the BLOCKED
  verdict; this is the authoritative final record).

### Raw reviewer verdicts (cycle-1)

| Reviewer | Verdict | Headline findings |
|---|---|---|
| Scope Boundary Auditor | ADVISORY | All changes trace to the six entries; no new release units; membership growth confined to 149-S/150-S/151-S. P2: 168.017-T recovery machinery / leaf-AC enumeration; P3: 168.005-T 2h ceiling, case-fold duplicate scope. |
| Correctness Reviewer | PASS | DAG reconstructed ACYCLIC; all 9 leaves are pure sinks; retargets consistent; membership arithmetic reconciles (149-S=19, 150-S=13, 151-S=15); split coverage correct (168.013-T@2 not split). Only P3 advisories. |
| Architecture Strategist | ADVISORY | Seams cut correctly. P1(high): 168.011-T "core" label vs item-10 "internal/config" (config->core inversion risk via 168.018-T). P2: write-outcome type-ownership inversion; 170.011 drift guard is prose-only. P3: audit-writer coupling. |
| Security Reviewer | ADVISORY | Fail-closed spine well-specified; no FAIL-class gap. High-conf P2: F1 confused-deputy (no role/key-purpose separation), F2 audit blind to denials/verification failures, F3 nonce fold + no atomic check-consume. Medium: F4 algo-pinning, F5 not_before. Low: F6 pin-set integrity. |

### Remediations applied before gate close (cycle-1)

1. **Architecture P1 (high) — RESOLVED.** The 168.011-T task artifact already declared
   `Domain: config (internal/config)`; the contradiction was plan-vs-body. The plan's
   168.011-T section is corrected to `internal/config`, and item-10 domain placement is
   restated. Verified in-backlog: 168.018-T depends-on 168.011-T with both in config —
   no config->core inversion.
2. **Architecture P2 / Correctness P3 (type ownership) — RESOLVED.** 168.005-T now
   DECLARES the base `WriteOutcome` type (its return signature/tests compile without its
   dependent); 168.017-T adds only variant semantics/compensation/recovery + tests.
3. **Architecture P2 (drift guard) — RESOLVED.** 170.002-T AC now requires a golden
   cross-check test against actual 167.008-T producer output so encoding drift breaks a
   test rather than fail-closing production.
4. **Security F1 (high) — RESOLVED.** Role-scoped key resolution added to 168.003-T,
   169.002-T, 170.002-T (attestation-signer vs token-issuer), closing the cross-protocol
   confused-deputy.
5. **Security F2 (high) — RESOLVED.** Durable audit event now required on DENIED
   mutations (168.005-T) and on every fail-closed verification rejection (169.002-T,
   170.002-T), not only successes.
6. **Security F3 (high) — RESOLVED.** The nonce ledger (170.003-T) is now MANDATORY
   (removed justify-or-fold) with atomic check-then-consume inside the 167.008-T locked
   section (no TOCTOU double-spend).
7. **Security F4 (medium) — RESOLVED.** Verification pinned to the anchor's algo,
   rejecting payload-declared algo incl. `none` (168.003-T/169.002-T/170.002-T).
8. **Security F5 (medium) — RESOLVED.** `not_before`/issued-at added to the signed token
   binding (170.001-T declaration) and enforced (170.002-T).
9. **Security F6 (low) — RESOLVED.** Pin-set provenance/integrity documented in the
   168.006-T threat model (control or explicit out-of-scope trust assumption).
10. **Scope P2 (leaf AC) — RESOLVED.** Enumerated checkable pass/fail AC added for all
    nine leaf tasks; scenario-conservation framing corrected (refinement, not partition).
11. **Correctness P3 — RESOLVED.** 168.013-T gains a RED-first AC; cross-feature
    169.002->168.003 / 170.002->168.003 edges restated (verified present in-backlog).
12. **Scope P2/P3 traceability — NON-BLOCKING.** 168.017-T write-outcome recovery and the
    case-fold-at-every-depth duplicate rejection are directly operator-mandated (items 3
    and 13 of the correction directive), tracing to review findings on stash 71F5C21F and
    the #423 validator respectively; retained by design.

The material AC changes above are propagated to the affected task artifacts as governed
append-only addendum comments (plan-review cycle-1), so the hardened contract travels
with each task into Ship execution.

No reviewer returned FAIL. The single high-confidence P1 and all high/medium-confidence
security P2/P3 integrity findings are resolved; residual items are non-blocking P3
advisories or operator-mandated by design. Gate decision: **PASS**.

* **operator_authorization:** approved (operator delegated this Stage review-fix session
  with explicit direction to resolve all high/medium P0/P1 and tightly-coupled P2/P3
  integrity findings and rerun the gate; no blocking/FAIL findings remained after
  remediation).

### Plan Review — cycle 2

<!-- plan-review-attempt: 2 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

* **reviewers:** Scope Boundary Auditor, Correctness Reviewer, Architecture
  Strategist, **Security Reviewer** (four independent agents dispatched in parallel on
  this plan document; the Security Reviewer is the mandatory security-sensitive persona
  for this trust-chain plan). A focused Security RE-REVIEW was dispatched after
  remediation to confirm the single FAIL-class finding was closed.
* **cycle:** review-fix cycle 2 (this is the authoritative final record; supersedes the
  cycle-1 record above).

#### Raw reviewer verdicts (cycle-2)

| Reviewer | Verdict | Headline findings |
|---|---|---|
| Scope Boundary Auditor | ADVISORY | No new release units; all 9 cycle-2 tasks land only in 149-S/150-S/151-S (deltas +2/+3/+4 -> 21/16/19); every task traces to a remediation item; only new edge is the declared serial 151-S->150-S. Advisories: 170.016-T/169.013-T scenario-count near ceiling (P2/P3); 170.015-T is a code de-dup behavior (P3); VII gate AC (P3, since resolved). |
| Correctness Reviewer | ADVISORY | DAG CONFIRMED ACYCLIC (all 9 cycle-2 tasks are pure sinks; shipment edges 149<-150<-151 serial). Item-3 code-grounding CONFIRMED ACCURATE: prepareShipmentReconcileEvidence recomputes the digest under the Phase-A lock at shipment_reconcile_evidence.go:186; 170.015-T removal is behavior-preserving and correctly owned. Membership arithmetic reconciles. P3: stale-body double-ownership risk (resolved by de-listing carved ACs). |
| Architecture Strategist | ADVISORY | No config->core or docs->behavior inversion; 170.011-T docs forward-ref vs 170.015-T behavior owner clean. Two P2 ownership ambiguities (golden cross-check placement; 168.005-T atomicity terminology) — both RESOLVED in remediation. |
| Security Reviewer | FAIL -> PASS (after remediation) | Initial: one P1 FAIL-class — Constitution VII rotate/revoke approval gate classified but unowned by a checkable AC. All other controls (F1-F6, JSON duplicate-member exact + decoder-equivalent) verified owned by behavior tasks with RED-first typed fail-closed ACs. Focused re-review after remediation: **PASS — P1 CLOSED**, no remaining FAIL-class finding. |

#### Remediations applied before gate close (cycle-2)

1. **Security P1 (Constitution VII) — RESOLVED.** The destructive rotate/revoke approval
   gate is now owned by checkable RED-first ACs on 168.013-T (CLI: operator approval /
   safety-mode gate before core invocation, else non-zero exit with no core mutation) and
   168.005-T (core: refuses rotate/revoke without an explicit approved-authorization
   argument, bypass-resistant). Confirmed CLOSED by focused Security re-review.
2. **Architecture P2 (atomicity terminology) — RESOLVED.** 168.005-T reworded from "one
   atomic in-core commit" to a deterministic sequenced commit returning the base
   WriteOutcome type; the marker-protected recoverability semantics
   (ErrWriteNotApplied/ErrWriteIndeterminate) live with the mechanism in 168.017-T, and
   denied-path audit-failure recovery in 168.019-T (**withdrawn cycle-3 item 13; folded
   into 168.017-T as the minimal `ErrAuditNotPersisted` denied-path contract, broad
   recovery deferred to P-021 stash 6749D311**). The atomic-vs-indeterminate
   contradiction is removed.
3. **Architecture P2 / Correctness P3 (golden cross-check ownership) — RESOLVED.** The
   producer golden cross-check moved from 170.002-T to the digest-binding owner 170.013-T
   (plan section, leaf-AC row, and task body); 170.002-T no longer enumerates it.
4. **Correctness P3 (stale-body double-ownership) — RESOLVED.** The origin plan sections
   168.003-T, 169.002-T, 170.002-T, 169.010-T, 170.012-T now DELEGATE their carved
   behavior (role/algo/audit/not_before/decoder-equivalent duplicates) to the cycle-2
   split tasks instead of re-listing it, so a Ship executor cannot implement it twice.
5. **Scope P2 (scenario count) — NON-BLOCKING.** 170.016-T and 169.013-T combine
   role+algo into one parametrized scenario, leaving each task at exactly three executable
   scenarios (role/algo, not_before or audit, fail-closed audit). Within the ceiling;
   retained by design.

No reviewer returned FAIL after remediation. Zero open P0/P1 findings remain (the single
P1 is closed and confirmed by re-review); residual items are non-blocking P2/P3 advisories
or operator-mandated by design. Remediation context is recorded above, separate from the
literal `decision:` line.

operator_authorization: approved (operator delegated this cycle-2 Stage review-fix session
on branch chore/stage-149-s-trust-chain-corrections with explicit direction to resolve the
consolidated residuals and rerun the full plan-review gate; no blocking/FAIL findings
remain after remediation).

### Plan Review — cycle 3 (FINAL)

<!-- plan-review-attempt: 3 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

* **reviewers:** Scope Boundary Auditor, Correctness Reviewer, Architecture
  Strategist, Constitution Reviewer, and **Security Reviewer** (five independent
  persona agents dispatched in parallel on this plan document after plan-harden; the
  Security Reviewer and Constitution Reviewer are the mandatory security-sensitive and
  standards personas for this destructive-mutation trust-chain plan — items A2, 16).
* **cycle:** review-fix cycle 3 (FINAL permitted cycle; this is the authoritative final
  record and supersedes the cycle-1 and cycle-2 records above).

#### Raw reviewer verdicts (cycle-3)

| Reviewer | Verdict | Open P0/P1 | Headline |
|---|---|---|---|
| Scope Boundary Auditor | PASS | none | Scope REDUCED as directed: 168.019-T/170.015-T/170.017-T withdrawn and consistently reflected; P-021 stash 6749D311 captures deferred recovery subsystem; no new release units; membership confined to 149-S/150-S/151-S; all tasks single-domain <=3 scenarios. Three P3 doc-staleness nits. |
| Correctness Reviewer | ADVISORY | none | Removed-task refs CLEAN; membership 20/16/17 MATCH; serial chain confirmed; DAG ACYCLIC; 170.011-T correctly documents drift-free under-lock recompute (code-verified at shipment_reconcile_evidence.go:186). One P2 plan-vs-artifact edge-text inconsistency on 169.012-T. |
| Architecture Strategist | PASS | none | No config->core or docs->behavior inversion; single-owner integrity for nonce (170.008-T) and tamper/swap (169.014-T); WriteOutcome base/variant layering correct. Three P3 (one medium: 168.017-T body/frontmatter edge mismatch). |
| Constitution Reviewer | PASS | none | `## Constitution Check` H2 + `Constitution Check: pass` present; I-XI incl. IX/X explicit; XI not N/A (merge-commit constraint); IV containment and VII destructive-approval owned by checkable ACs; `## Plan Hardening` present with ProposedAction/ActionRisk/approval/rollback; `Requires plan hardening: yes`. Two low P3. |
| Security Reviewer | ADVISORY | none | Fail-closed spine fully owned: role-scoped resolution (F1/F4), denial+rejection audit (F2), single atomic nonce consume with no-rollback indeterminate (F3), workspace containment, exact + decoder-equivalent JSON duplicate rejection at every depth. Cycle-2 P1 (VII approval gate) CLOSED and confirmed. One P3 (approval-evidence trust-root provenance). |

#### Remediations applied before gate close (cycle-3)

1. **Correctness P2 (169.012-T edge text) — RESOLVED.** The dependency-correction section
   now states the final set `{169.005-T, 169.011-T, 168.015-T}` (169.005-T retained as the
   carve origin), matching the task frontmatter and body; the incorrect "169.005-T replaced"
   wording is removed.
2. **Architecture P3 (medium) (168.017-T body/frontmatter mismatch) — RESOLVED.** The
   explicit edge `168.017-T -> 168.012-T` (external-pin gate, used by the denied-path audit
   contract) was added to the frontmatter to match the body; re-verified ACYCLIC.
3. **Scope/Security P3 (stale references) — RESOLVED.** The superseded cycle-1 "Final
   verified membership" block is annotated as a cycle-1 snapshot pointing to the
   authoritative cycle-3 counts; the cycle-2 remediation record's 168.019-T ownership
   mention is annotated `withdrawn cycle-3 item 13`.

The residual advisories (Security P3 approval-evidence trust root, Constitution low P3
tombstone/nonce wording legibility, Scope P3 documentation-staleness) are non-blocking
defense-completeness / legibility items with no reachable exploit or scope impact; they are
recorded for Ship execution consideration, not gate blockers.

**Zero open P0/P1 findings** across all five personas (the cycle-2 P1 is closed and
re-confirmed). No reviewer returned FAIL. Remediation context is recorded above, separate
from the literal `decision:` line.

operator_authorization: approved (operator delegated this cycle-3 FINAL Stage review-fix
session on branch chore/stage-149-s-trust-chain-corrections with explicit direction to
resolve the consolidated in-scope blockers, run plan-harden, and rerun the full plan-review
gate with architecture, scope, correctness, constitution/standards, and security coverage;
zero open in-scope P0/P1 remain after remediation).

### Plan Review — cycle 4 (operator-authorized extra bounded cycle)

<!-- plan-review-attempt: 4 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

* **reviewers dispatched this cycle:** Scope Boundary Auditor, Correctness Reviewer, and
  **Security Reviewer** (three independent persona agents dispatched in parallel over the
  cycle-4 change surface). The Security Reviewer is the mandatory security-sensitive persona
  for this trust-chain work.
* **personas carried forward (cycle-3 PASS inherited):** Constitution Reviewer and
  Architecture Strategist. Cycle-4 edits are confined to P-002.3 `exempt_verification_command`
  text, additive P-002.6 `red-deliverable-contract` metadata blocks, dependency-prose
  alignment, and one task-title rename. These touch neither the plan's `## Constitution Check`
  / principle coverage nor any task-ownership or dependency-DIRECTION surface (the prose fixes
  bring prose INTO alignment with pre-existing frontmatter; the red blocks are additive
  scheduling metadata). The cycle-3 Constitution `pass` and Architecture PASS therefore carry
  forward unchanged and unweakened.
* **cycle:** operator-authorized extra bounded review-fix cycle 4 (beyond the normal 3-cycle
  limit; narrowly scoped to the P1-1 / P1-2 / P2 residual list). This record supersedes the
  cycle-3 FINAL record for the affected contract surfaces only; all cycle-3 verdicts otherwise stand.

#### Scope of cycle-4 change surface

* **P1-1 (P-002.3 harness-exempt commands):** 168.006-T, 169.006-T, 169.009-T, 170.007-T,
  170.011-T. Docs-only commands now probe required document CONTENT and run the doc lint gate,
  emitting `EXEMPT_VERIFY_OK:<task-id>` only after all guards; content probes extended to the
  AC-required security terms (`rotation` on 168.006-T, `not_after` on 170.007-T).
  Verification-only 169.009-T runs verbose `go test`, rejects `no tests to run`, and asserts
  both named `--- PASS: TestAttestationReVerifyGuard_OfflineReverify` and
  `TestAttestationReVerifyGuard_TamperSwapFailClosed` guards by name and by count (>=2).
* **P1-2 (P-002.6 red-deliverable contracts):** 168.010-T (closes wave 8; green-makers
  168.007/168.008/168.009), 169.008-T (closes wave 6; green-maker 169.007), 170.010-T (closes
  wave 7; green-makers 170.008/170.009). Each block carries the five canonical keys in order
  with a persistent named `red_selector_command`; closing waves independently recomputed against
  the M-restricted per-shipment DAG.
* **P2 (consistency):** dependency prose aligned to frontmatter in 169.012-T
  (`169.005-T, 169.011-T, 168.015-T`), 170.013-T (`170.002-T`, with 170.011-T labelled a
  forward reference), 170.014-T (adds `170.008-T`); 170.016-T title renamed to
  `Token verify role-scoped + pinned-algo + rejection audit` (drops `not_before` ownership,
  which the body already delegates to 170.002-T).

#### Raw reviewer verdicts (cycle-4)

| Reviewer | Verdict | Open P0/P1 | Headline |
|---|---|---|---|
| Scope Boundary Auditor | PASS | none | No scope creep: no new tasks, no behavior beyond the authorized P1-1/P1-2/P2 contract text; changed-file set confined to the 12 authorized tasks. One P3 low-confidence (git-diff not runnable in agent session — closed by Stage: `git diff --stat` shows only the 12 authorized task files + the governed `hooks_queue.jsonl` side-effect of the 170.016 title update). |
| Correctness Reviewer | PASS | none | All eight P-002.3/P-002.6 contracts EXACTLY match authoritative policy; `EXEMPT_VERIFY_OK` marker last-before-exit on all five; red-block key order, selectors, green-maker closed-set membership, and closing waves (8/6/7) self-consistent and matching independently verified waves. Zero findings, high confidence. |
| Security Reviewer | PASS | none | Fail-closed contracts PRESERVED: docs-only commands emit the marker only after existence + security-content probe + doc lint gate (no short-circuit, no swallowed exit); red harnesses still assert fail-closed rejection of revoked/role-widened, unsigned-metadata, and replayed/downgraded inputs. Three P3 advisories (probe-coverage `rotation`/`not_after`, named-guard assertion) — two applied this cycle (168.006-T `rotation`, 170.007-T `not_after`), the 169.009-T named-guard assertion applied this cycle. |

#### Remediations applied before gate close (cycle-4)

1. **Security P3 (168.006-T probe coverage) — RESOLVED.** Added `rotation` to the content
   probe set so the exempt command validates the AC-required key-rotation custody content.
2. **Security P3 (170.007-T probe coverage) — RESOLVED.** Added `not_after` to the content
   probe set so the command validates the AC-required token-expiry (validity-window) content.
3. **Security P3 (169.009-T named-guard assertion) — RESOLVED.** The verification-only command
   now asserts both specific guard names (`OfflineReverify`, `TamperSwapFailClosed`) in
   addition to the `>=2` count and the `no tests to run` rejection, closing the "two arbitrary
   prefix PASS lines" gap.
4. **Scope P3 (git-diff coverage gap) — CLOSED by Stage.** `git --no-pager diff --stat`
   confirms exactly the 12 authorized task files changed plus one appended line in the
   governed `.backlogit/hooks_queue.jsonl` (automatic index/hook side-effect of the governed
   `backlogit update --title` on 170.016-T); no source, config, or out-of-scope task files touched.

**Zero open P0/P1 findings** across all dispatched personas; the two carried-forward personas
(Constitution, Architecture) remain PASS from cycle 3 with no cycle-4 surface impact. No
reviewer returned FAIL. Remediation context above is recorded separately from the literal
`decision:` line.

operator_authorization: approved (operator explicitly authorized ONE narrowly bounded extra
Stage review-fix cycle beyond the normal limit on branch
chore/stage-149-s-trust-chain-corrections to apply exactly the P1-1 P-002.3 harness-exempt
contract fixes, P1-2 P-002.6 red-deliverable contract blocks, and P2 consistency fixes, then
run focused policy validation and a final plan review; zero open in-scope P0/P1 remain after
remediation, so Step 1.5 may proceed).

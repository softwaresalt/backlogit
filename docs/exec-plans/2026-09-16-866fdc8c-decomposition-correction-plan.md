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
The **ten** security-hardening tasks (168.007-010 [4], 169.007-009 [3],
170.008-010 [3] = 10) already carry explicit AC and RED-first ordering. Their
security *behavior* is unchanged, but two of them — **168.007-T and 168.010-T** —
had their upstream declaration dependency retargeted (168.001-T -> 168.012-T) as a
consequence of the declaration/behavior split, and 168.010-T's RED-ordering prose
was synchronized with its frontmatter (see item-5 remediation). All other
hardening tasks are unchanged.

**Constitution Check (mandatory, NON-NEGOTIABLE).** This correction is validated
against `.github/instructions/constitution.instructions.md`:

| Principle | Check | Result |
|---|---|---|
| I. Safety-First Go | No source written by Stage; typed-error/fail-closed AC preserved on every behavior task | PASS |
| II. Test-First (NON-NEGOTIABLE) | Every behavior-bearing split declares a RED harness observed failing before impl; declaration tasks carry a source-shape (go/ast) harness | PASS |
| III. Workspace Isolation / Security Boundaries | `trust_anchors` parsing/schema validation placed in `internal/config`; policy/resolution/mutation in `internal/core` (item 10); external-pin gate fail-closed | PASS |
| IV. CLI Workspace Containment (NON-NEGOTIABLE) | CLI split into thin wiring tasks (168.013-T/168.016-T) delegating to core; no security-state logic in CLI layer | PASS |
| V. Structured Observability | Durable audit event required on every trust-anchor mutation (168.005-T); write-outcome taxonomy + recovery marker (168.017-T) | PASS |
| VI. Single Responsibility | Declaration split from behavior; parser split from verification; every task single-domain (config OR core OR cli OR docs) | PASS |
| VII. Destructive Command Approval (NON-NEGOTIABLE) | No destructive commands; governed backlogit mutations only; archive append-only history untouched | PASS |
| VIII. Explicit Safety Modes | No elevated-risk mode introduced; no relaxation of existing fail-closed controls | PASS |
| Task Granularity (NON-NEGOTIABLE) | Every task re-scoped to <3 files / <5 funcs / <4 test scenarios; tasks confirmed at 4+ scenarios split (item 9 matrix) | PASS |
| XI. Merge Commit History Preservation | N/A (no merges performed by Stage) | N/A |

No principle fails. Proceed to plan review.

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

Trusted-key resolver primitive (rotation + revocation), pure.
**AC:** (1) returns the active key inside not_before/not_after with status=active;
(2) expired/not-yet-valid, revoked, and ambiguous inputs each return a typed error;
(3) pure (no I/O), proven by table-driven tests; (4) **resolution is role-scoped
(Security-F1 remediation): a caller requests a key for a specific purpose
(attestation-signer vs token-issuer) and the resolver rejects an anchor whose
`role` does not match, preventing cross-protocol key reuse;** (5) **the resolved
key carries the anchor's pinned algo so callers verify with the pinned algorithm,
never a caller/payload-declared one (Security-F4 remediation).**

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

**Deterministic ordering (item-3 remediation).** The core mutation path executes a
fixed sequence: (a) validate args -> (b) external-pin policy gate (fail-closed) ->
(c) apply config mutation -> (d) apply status mutation -> (e) persist durable audit
event, where (c)+(d)+(e) form ONE atomic in-core commit. The durable
**write-outcome taxonomy** — `ErrWriteNotApplied` (no state changed; retry-safe) vs
`ErrWriteIndeterminate` (state may be partially applied; retry unsafe) — plus
retry/compensation boundaries, the recovery-visibility marker, and injected-failure
tests are split into **168.017-T** to preserve the 2-hour/single-domain limit.
168.005-T owns the deterministic ordering, atomic in-core commit, and it
**DECLARES the base `WriteOutcome` result type** (so its own return signature and
tests compile without depending on its downstream leaf); **168.017-T** adds only the
variant *semantics* (`ErrWriteNotApplied` retry-safe vs `ErrWriteIndeterminate`
retry-unsafe), retry/compensation boundaries, the recovery-visibility marker, and
injected-failure tests (Architecture-P2 / Correctness-P3 ownership-inversion
remediation). This split preserves the 2-hour/single-domain limit.
**AC:** (1) RED test first; (2) add/rotate/revoke mutate config+status in fixed
order (a)->(e); (3) an unpinned-fingerprint mutation is rejected (fail-closed) at
step (b) before any state change **AND the denied mutation persists a durable audit
event (Security-F2 remediation: denials are audited, not only successes)**; (4) each
successful mutation persists a durable audit event atomically with the state change;
(5) the core declares and returns the base `WriteOutcome` type consumed by
168.017-T; (6) tests GREEN.

### 168.013-T (new, thin CLI wiring)

`backlogit trust-anchor add|rotate|revoke` CLI: parse args/flags and delegate to
the 168.005-T core. No security-state logic in the CLI layer. Domain: cli.
Depends on 168.005-T.
**AC:** (1) **RED test first (Correctness-P3 remediation)**; (2) each subcommand
invokes the core mutation and surfaces its typed
result/exit code; (3) the CLI layer contains no pin-gate/audit logic; (4) a
rejected core mutation yields a non-zero exit with no partial write.

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

Attestation envelope parser rejecting malformed/duplicate-member JSON via
raw-token scan (mirroring the #423 validator). Domain: core. TDD: RED harness
first. Depends on 169.001-T.
**AC:** (1) RED test first; (2) a well-formed envelope parses; (3) malformed JSON is
rejected AND duplicate JSON members — both exact-byte duplicates and
decoder-equivalent case-fold duplicates — are rejected with a typed error at EVERY
object depth (top-level statement, nested predicate, subject, and evidence objects),
proven by negative tests per depth; (4) tests GREEN.

### 169.002-T (AC backfill; dep retargeted 169.001 -> 169.010)

Attestation verification primitive (fail-closed).
**AC:** (1) a valid signature over the canonical statement using a 168.003-resolved
key verifies; (2) statement evidence fields must equal the reconcile-computed
evidence or fail; (3) any signature/binding/expiry mismatch returns a typed
fail-closed error; (4) **the resolver is asked for an attestation-signer-role key
and verification uses the anchor's pinned algo, rejecting a role mismatch and any
envelope-declared algo (incl. `none`) (Security-F1/F4 remediation);** (5) **every
fail-closed verification rejection persists a durable audit event, not only
successes (Security-F2 remediation).**

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

**Digest/lock sequencing (item-6 remediation).** The contract MUST specify this
ordering to match the existing producer:
`shipmentReconcileRequestIdentityDigestForNormalized` is computed as PRE-LOCK,
pure/deterministic work over the normalized delta BEFORE any membership or
persistence lock is taken. Then Phase-A membership locking is acquired; then
per-artifact persistence locks are taken inside the critical section. The
pre-computed digest is threaded THROUGH the critical section but is NEVER
recomputed or re-canonicalized under lock (recomputation under lock is a defect the
contract must forbid), guaranteeing the digest is a stable function of the request
inputs independent of lock-ordering or concurrent membership changes.

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
struct; (3) malformed token input is rejected AND duplicate JSON members — both
exact-byte duplicates and decoder-equivalent case-fold duplicates — are rejected
with a typed error at EVERY object depth, proven by negative tests per depth;
(4) tests GREEN.

### 170.002-T (AC backfill; dep retargeted 170.001 -> 170.012; +170.011)

Token verification primitive (fail-closed).
**AC:** (1) the token signature verifies via a 168.003-resolved token-issuer-role
key using the anchor's pinned algo (rejects role mismatch and any token-declared
algo incl. `none`; Security-F1/F4); (2) binding to
the target shipment-id AND the reconcile request-identity digest (per the 170.011-T
contract) is enforced, verified by a **golden cross-check test against actual
167.008-T producer output so producer-encoding drift breaks a test rather than
fail-closing production (Architecture-P2 remediation);** (3) both not_after AND
not_before/issued-at are enforced (Security-F5 remediation); (4) any mismatch
returns a typed fail-closed error; (5) every fail-closed rejection persists a
durable audit event (Security-F2 remediation).

### 170.003-T (AC backfill)

Nonce single-use ledger (anti-replay) — **MANDATORY (Security-F3 remediation;
no longer fold-eligible: request-identity + not_after binding does not prevent
replay within the TTL window).**
**AC:** (1) durable, handle-safe append-then-fsync record of consumed nonces with
an **atomic check-then-consume inside the 167.008-T locked section (no TOCTOU
double-spend window under concurrency);** (2) a
replayed token is rejected fail-closed; (3) consumed nonces are pruned past token
not_after; (4) a replayed/double-spend attempt persists a durable audit event.

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
| 170.013-T | RED test first; a token whose request-identity-digest matches the reconcile-computed digest binds; a mismatched digest fails closed with a typed error |
| 170.014-T | RED test first; self-minted (untrusted-key), expired, wrong-shipment, and replayed tokens are each rejected fail-closed with distinct typed errors |

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

These four carry the canonical labels `docs-only,p002.1-harness-exempt`. **Contract:**
a P-002.1-harness-exempt task MUST (1) produce only markdown/design artifacts, (2)
declare markdownlint (or design-review) as its gate, and (3) contain NO Go source,
test, or config change. **No behavior- or test-bearing task is exempt** — every task
outside this closed set retains its RED/source-shape harness requirement. 170.011-T
qualifies solely because its deliverable is a specification document; the behavior it
specifies is implemented and harnessed by 170.001-T/170.012-T/170.002-T.



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

## Shipment membership deltas

* 149-S: add 168.011-T, 168.012-T, 168.013-T, and leaf-splits 168.014-T, 168.015-T,
  168.016-T, 168.017-T, 168.018-T (feature 168-F already present) -> 19 items
* 150-S: add 169.010-T and leaf-splits 169.011-T, 169.012-T (feature 169-F already
  present) -> 13 items
* 151-S: add 170.011-T, 170.012-T and leaf-splits 170.013-T, 170.014-T (feature
  170-F already present) -> 15 items

Final verified membership: **149-S = 19, 150-S = 13, 151-S = 15** (feature + all
children, confirmed via `backlogit shipment get`).

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

---
chunk_strategy: h1-h2-h3
description: "Design contract for the reconcile request-identity-digest (field set, canonical byte encoding, producer sequencing) and the operator pre-commit authorization-token issuance flow (170.011-T)."
doc_type: design
status: draft
created: 2026-09-16
schema_version: "1.0"
source: docs/design-docs/2026-09-16-866fdc8c-request-identity-digest-contract.md
title: "Request-Identity-Digest Contract and Operator Pre-Commit Issuance Flow"
---

# Request-Identity-Digest Contract and Operator Pre-Commit Issuance Flow

## Summary

This document fixes the request-identity-digest contract that the operator
authorization token (feature 170-F) binds to. It specifies the digest input
fields and their order, the canonical byte encoding, the real lock and
digest sequencing of the producer, and the operator pre-commit token
issuance flow. Issuance and verification are therefore fully specified
before Feature C implementation (170.001-T, 170.002-T).

Sources and context:

* `docs/exec-plans/2026-09-13-866fdc8c-trust-boundary-split-decided-plan.md`
* `docs/exec-plans/2026-09-16-866fdc8c-decomposition-correction-plan.md`
* Producer: 167.008-T (shipped via 148-S), the reconcile transaction in
  `internal/core/shipment_reconcile_transaction.go`

## Authoritative producer

The request-identity-digest is produced by the reconcile path in the locked
167.008-T critical section. It is persisted as the frontmatter field
`shipment_reconciliation_request_identity_digest` (constant
`shipmentReconcileRequestIdentityDigestField` in
`internal/core/shipment_reconcile_classifier.go`) and recorded in the
durable event as `request_identity_digest`
(`internal/core/shipment_reconcile_event.go`).

167.008-T is the single authoritative producer. This contract is
byte-for-byte aligned with it. Neither the token issuer nor the verifier
re-canonicalizes: they treat the digest as an opaque lowercase hex string
produced by `shipmentReconcileRequestIdentityDigestForNormalized` and never
derive it by a second encoder.

## Request-identity-digest input fields

The digest is computed over the NORMALIZED reconcile request
(`shipmentReconcileNormalizedRequest`, produced by
`normalizeShipmentReconcileRequest`). The domain tag is the constant
`backlogit/shipment-reconcile/request-identity/v1`. The fields are encoded
in exactly this order:

| Order | Tag | Kind | Normalization |
| ----- | --- | ---- | ------------- |
| 1 | `idempotency_key` | scalar | whitespace-trimmed, required |
| 2 | `merge_sha` | scalar | trimmed, full git object name, lowercased |
| 3 | `reason` | scalar | whitespace-trimmed, required |
| 4 | `actor` | scalar | whitespace-trimmed, required |
| 5 | `second_approver` | scalar | trimmed, empty string when absent, must differ from actor |
| 6 | `evidence_refs` | list | normalized by `normalizeShipmentReconcileRequestEvidenceRefs` |
| 7 | `closure_path` | scalar | the trimmed `closure_evidence` request value |

The shipment id is NOT a digest input (the request is normalized with
`requireShipmentID` false for the digest). Shipment binding is therefore a
separate token claim and is verified separately from the digest. Closure
file contents are not read for the digest; the closure content hash belongs
to the evidence digest, which is a different value with the domain tag
`backlogit/shipment-reconcile/evidence/v1`.

## Canonical byte encoding

The encoder is `shipmentReconcileDigestHex`. It is deterministic and
domain-separated:

1. Write the domain tag as a string record.
2. For each field in the order above, write one kind byte: `0x01` for a
   scalar, `0x02` for a list.
3. Write the field tag as a string record.
4. Write the value count as a uvarint (1 for a scalar, the list length for
   a list).
5. Write each value as a string record.

A string record is a uvarint byte length followed by the raw bytes. The
length prefixes prevent boundary shifts and list-item concatenation from
colliding under a different logical tuple. The payload is hashed with
`canonical.HashBytes` and the result is the lowercase hex digest (64
characters).

This encoding is normative for the digest. The token never re-encodes it:
the token carries the digest as a string and the verifier compares it by
exact byte equality against the producer value.

## Digest and lock sequencing in the producer

This section describes the ACTUAL behavior of the code. It mandates no code
change.

1. Pre-lock computation. `reconcileShipmentToShippedImpl` in
   `internal/core/shipment_reconcile_transaction.go` (around line 76)
   calls `shipmentReconcileRequestIdentityDigest(req)` BEFORE any lock. The
   digest is computed lock-free from the normalized reconcile delta, and
   the same call validates the request scalars fail-fast.
2. Phase A. The lifecycle-global lock and then the shipment membership lock
   are acquired. The membership lock stays held through every later phase.
3. Phase C evidence. With only the membership lock held,
   `prepareShipmentReconcileEvidence`
   (`internal/core/shipment_reconcile_evidence.go:186`, the function
   declaration region; the digest recompute is the
   `shipmentReconcileRequestIdentityDigestForNormalized` call within it)
   recomputes the digest under the Phase-A membership lock from the same
   normalized request. The recompute is deterministic over the same
   normalized delta, so it yields a byte-identical digest. It is
   drift-free: it introduces no drift between the pre-lock value and the
   value placed in the prepared event.
4. Phase D persistence locks. Inside the critical section the item-log lock
   (C) and then the sorted artifact-mutation batch lock (B) are acquired
   (`lockShipmentReconcileCThenB`). The pre-lock digest value is persisted
   to the frontmatter field and the prepared event is appended under
   those held locks.

Because the recompute is drift-free by determinism, no behavior change is
mandated and no de-duplication task exists. The cycle-2 de-dup task
170.015-T was withdrawn as outside the authorized digest-design scope
(cycle-3 item 12).

## Operator pre-commit issuance flow

### Roles

* The operator is the human or operator-controlled signer that mints the
  token. The agent requesting the reconcile never mints or holds signing
  material.
* The verifier is the reconcile path, which resolves the operator key via
  the 168.003-T trust anchor and fails closed.

### Flow

1. The operator (or an operator tool) computes the request-identity-digest
   for the intended reconcile request. It uses the producer algorithm
   above through the same code path and never a second implementation.
2. The operator mints the token BEFORE the reconcile commit. The token is
   a pre-commit authorization, so it is issued before the locked critical
   section runs and is verified inside it.
3. The operator signs the token with the operator key.

### Signed content

The signed claims are:

| Claim | JSON tag | Meaning |
| ----- | -------- | ------- |
| shipment id | `shipment_id` | target shipment the token authorizes |
| request-identity-digest | `request_identity_digest` | digest of the normalized reconcile request |
| issued-at / not-before | `not_before` | earliest instant the token is valid |
| expiry | `not_after` | latest instant the token is valid |
| nonce | `nonce` | per-invocation unique value |

The token is an object with four concrete object-bearing levels, each of
which rejects duplicate members: the token root, the `header` object
(algorithm and key id), the `claims` object (`not_before`, `not_after`,
`nonce`), and the `binding` object (`shipment_id`,
`request_identity_digest`). The `signature` is a root member over the
canonical bytes of the header, claims, and binding.

### Validity window and TTL

* `not_before` is the issued-at instant in UTC, RFC 3339.
* `not_after` is the expiry instant in UTC, RFC 3339.
* TTL is `not_after - not_before` and must be positive and bounded by a
  short operator-configured maximum. The verifier rejects a token that is
  not yet valid (now before `not_before`) and one that has expired (now
  after `not_after`), each with a typed error.

### Nonce handling

* The nonce is at least 128 bits from a cryptographically secure source,
  encoded as lowercase hex.
* Each token is single-invocation. The nonce is recorded so that replay of
  the same token is detectable. Consumption and replay audit belong to the
  later verification tasks, not to this design.

## Out of scope

* Role-scoped and pinned-algorithm enforcement and the fail-closed
  rejection audit (170.016-T).
* Request-identity-digest binding verification and the producer golden
  cross-check (170.013-T).
* Any change to the 167.008-T producer.

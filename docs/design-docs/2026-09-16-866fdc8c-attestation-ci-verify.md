---
chunk_strategy: h1-h2-h3
description: 'Operator guide for CI-side attestation generation and the reconcile-shipped --attestation verify path, including doctor offline re-verification.'
doc_type: design
schema_version: "1.0"
source: docs/design-docs/2026-09-16-866fdc8c-attestation-ci-verify.md
title: Signed Delivery-Evidence Attestation - CI Generation and Verify
---

# Signed Delivery-Evidence Attestation - CI Generation and Verify

This guide covers how a CI or release system emits a signed delivery-evidence
attestation and how an operator verifies it with `shipment reconcile-shipped
--attestation`. The trust model is described in
`2026-09-16-866fdc8c-trust-anchor-custody.md`.

## CI-side attestation generation

CI-side attestation generation produces a DSSE-style envelope over an in-toto
statement. The envelope is a JSON object with three members:

| Member | Content |
|---|---|
| `payloadType` | `application/vnd.in-toto+json` |
| `payload` | Base64 of the in-toto statement JSON |
| `signatures` | One or more entries of `keyid`, `sig` (base64) and `sig_algo` |

The statement `predicate` carries the delivery evidence the reconcile request
computes, and every field is required:

| Predicate field | Must equal |
|---|---|
| `shipment_id` | The shipment being reconciled. A subject `name` must also equal it. |
| `merge_sha` | The trusted merge commit of the shipment. |
| `manifest_digest` | The shipment manifest digest computed by reconcile. |
| `closure_content_hash` | SHA-256 of the closure evidence file. |
| `signer_identity` | The identity of the CI signer. |
| `issued_at`, `expires_at` | RFC 3339 validity window. A statement without a parsable expiry is rejected. |

The signature covers the DSSE v1 pre-authentication encoding
`DSSEv1 <len(payloadType)> <payloadType> <len(payload)> <payload>` computed over
the decoded payload bytes. Sign with an Ed25519 or ECDSA P-256 key. The
`keyid` must name a configured trust anchor, and `sig_algo` must equal that
anchor's pinned algorithm; a declared algorithm is never trusted on its own.

Provision the signer as a trust anchor with role `attestation-signer` in the
workspace config, and pin its fingerprint outside the workspace with
`BACKLOGIT_TRUST_PINS` or `BACKLOGIT_TRUST_PINS_FILE`:

```text
sha256:<fingerprint>=attestation-signer
```

The pin set is the external root of trust. Workspace config alone never makes a
key trusted.

## Verify with --attestation

Pass the envelope file to the governed reconcile command:

```text
backlogit shipment reconcile-shipped <shipment-id> \
  --reason <reason> --actor <actor> --second-approver <approver> \
  --idempotency-key <key> --merge-sha <sha> --closure-evidence <path> \
  --attestation <envelope.json> --confirm "reconcile-shipped <shipment-id>"
```

The flag is opt-in. Without it, reconcile behaves exactly as before. With it,
the command fails closed before any write unless all of these hold:

1. The envelope parses, with no duplicate or decoder-equivalent members.
2. The signing key resolves to an `attestation-signer` anchor that is pinned,
   within its validity window, and not revoked.
3. The signature verifies over the payload with the pinned algorithm.
4. The statement evidence equals the reconcile-computed evidence, and the
   statement names the target shipment.
5. The statement has not expired.

Use `--dry-run` to see the verification outcome without writing. A live run
that fails verification records an `attestation_rejected` audit event on the
shipment log.

## Persistence and doctor re-verification

A verified reconcile records the attestation claim and the durable signed
envelope in the `shipment_reconciled_shipped` event. `backlogit doctor` with the
shipped-event completeness audit re-verifies that persisted envelope offline
against the external pin set. An event whose envelope is missing, tampered with,
or signed by an untrusted key is not treated as shipped evidence, so the
missing-shipped-event finding is kept.

## Policy-enforce mode

A policy-enforce mode that would require an attestation for every reconcile is
NOT-YET-AVAILABLE. Attestation verification remains opt-in per invocation.

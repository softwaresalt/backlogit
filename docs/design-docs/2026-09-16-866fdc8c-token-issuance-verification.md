---
chunk_strategy: h1-h2-h3
description: "Operator guide to authorization token issuance and verification for governed shipment reconciliation, including the 167.015-T linkage note (170.007-T)."
doc_type: design
status: draft
created: 2026-09-16
schema_version: "1.0"
source: docs/design-docs/2026-09-16-866fdc8c-token-issuance-verification.md
title: "Authorization Token Issuance and Verification"
---

# Authorization Token Issuance and Verification

## Summary

Feature 170-F (Feature C) adds an operator pre-commit authorization token to
the governed `shipment reconcile-shipped` transaction. This document explains
how a token is issued, how it is verified inside the locked reconcile
critical section, and how it relates to task 167.015-T.

Related contracts:

* `docs/design-docs/2026-09-16-866fdc8c-request-identity-digest-contract.md`
  fixes the digest the token binds to.
* `docs/design-docs/2026-09-16-866fdc8c-trust-anchor-custody.md` describes
  the external pin set that authenticates the issuer key.

## Relationship to 167.015-T

Confirmation-only v1 guards a live reconcile with a self-suppliable
`--confirm` phrase or an interactive TTY prompt. Any caller that can run the
command can supply it, so it is not authenticated authorization.

Feature C upgrades confirmation-only v1 to authenticated authorization for
runs that opt in with `--authorization-token`.

Feature C does NOT itself close 167.015-T. The token path is opt-in at the
CLI, and 167.015-T stays open until its own acceptance criteria are met
independently of this feature.

When the flag is absent the behavior is exactly the confirmation-only v1
behavior, and the event records no approver.

## Token shape

A token is a JSON object with four members:

| Member | Content |
| ------ | ------- |
| `header` | signature algorithm (`alg`) and the issuer anchor id (`key_id`) |
| `claims` | `not_before`, `not_after` and `nonce` |
| `binding` | `shipment_id` and `request_identity_digest` |
| `signature` | base64 signature over the header, claims and binding |

The parser rejects malformed JSON and duplicate members at every depth,
including case-folded and escaped duplicates.

## Issuance flow

1. The operator computes the request-identity digest for the intended
   reconcile request with the producer code path
   (`core.ReconcileRequestIdentityDigest`), never a second encoder.
2. The operator chooses a nonce of at least 128 bits from a cryptographically
   secure source, encoded as lowercase hex.
3. The operator sets `not_before` to the issued-at instant and `not_after` to
   a short expiry, both UTC RFC 3339.
4. The operator signs the token with the private key of a `token-issuer`
   trust anchor. The agent that requests the reconcile never holds signing
   material.
5. The token is written to a file and passed with `--authorization-token`.

The token is issued before the reconcile commit. It is a pre-commit
authorization and is verified inside the critical section.

## Verification flow

The checks run inside the locked 167.008-T critical section, after the final
re-classification and before any write:

1. The issuer anchor is resolved against the external pin set. A key that is
   unknown, unpinned, revoked, outside its window or lacking the
   `token-issuer` role fails closed as an untrusted issuer, so a self-minted
   token is rejected.
2. The header algorithm must equal the anchor's pinned algorithm.
3. The signature must verify under the resolved public key.
4. The token `shipment_id` must equal the shipment being reconciled.
5. The verification instant must lie inside the inclusive window
   `not_before` to `not_after`. A token that is not yet valid and one that has
   expired are rejected with distinct typed errors.
6. The token `request_identity_digest` must equal the digest the reconcile
   computes, by exact byte equality.
7. On a live run the `nonce` is consumed through the atomic
   compare-and-consume provider.

Every failure is fail-closed: nothing is written and the nonce of a rejected
token is not consumed. Rejections are audited.

### Distinct negative errors

| Condition | Typed error |
| --------- | ----------- |
| token bound to another shipment | `ErrApprovalWrongShipment` |
| token bound to another request | `ErrApprovalWrongRequestIdentity` |
| nonce already consumed | `ErrApprovalReplay` |

Each wraps `ErrAuthorizationRejected`.

## Nonce single use

A `nonce` is valid for exactly one invocation. The authority is an external,
rollback-resistant compare-and-consume provider that lives outside the
workspace, so deleting or truncating the in-workspace projection cannot
re-enable replay. The check and the consume are one atomic operation, so
concurrent invocations cannot both succeed. If the consume succeeds but the
audit or projection write fails, the nonce stays consumed and the run reports
an indeterminate error.

The CLI does not ship a nonce provider. A live `--authorization-token` run
therefore fails closed with `ErrNonceProviderUnavailable` before any write
until a provider is supplied.

## Dry run

`--dry-run` verifies the token, including the digest and shipment binding,
and prints an `authorization` object with the `outcome`, `key_id` and
`nonce`. It never consumes the nonce and never writes.

## Recorded approver identity

A successful authenticated reconcile persists an `approver` object in the
prepared `shipment_reconciled_shipped` event with the issuer `key_id`, the
pinned `algo` and the `nonce`. Confirmation-only runs omit the object.

## Enforcement mode

`core.ResolveAuthEnforcement` decides whether authenticated authorization is
required. The external provider is authoritative and a workspace policy can
never downgrade it. The CLI flag is the opt-in surface for this feature.

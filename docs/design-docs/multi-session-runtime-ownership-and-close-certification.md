---
title: "Multi-session runtime ownership and close certification"
doc_type: reference
source: docs/design-docs/multi-session-runtime-ownership-and-close-certification.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-11"
shipment_id: 212-S
feature_id: 211-F
---

## Ownership boundaries

The shipped 211-F runtime separates workspace singleton ownership, per-client
stdio sessions and channel operation coordination. Preserve canonical
workspace/storage containment, isolated responses and durable serialized
mutations. Disconnect or cancellation of one session must not terminate
unrelated clients. Evidence and limits remain in the
[runtime report](../closure/2026-11-10-212-s-211-f-runtime-verification.md).

An API-owned stream is not automatically a test-framework-owned descriptor.
Re-executed coverage fixtures need private duplicates while retaining
normal close/EOF assertions. Pending Windows I/O likewise needs Go buffer
liveness through completion and cancellation drain, not merely race tests.
Completed request cancellation bridges must be unregistered without
reintroducing observable normal-completion cancellation.
Pinned executable identity, not library-host version or untrusted sidecar
metadata, governs daemon compatibility.

These are distinct ownership contracts, not reasons for a broad common
abstraction. The [compound refresh](../closure/2026-11-11-212-s-compound-refresh.md)
preserves their separate evidence and useful counterexamples.

## Lifecycle result and evidence are separate

A lost MCP reply means unknown transport outcome. The single original
212-S close continued and reached a verifiable terminal archive state.
The [certified report](../../.backlogit/reconcile/212-S-post-20261111T020701Z.md)
establishes completion from quiescence, exact manifest/control records,
native events, hook and doctor, with GI/GR/P-007 and parentage preserved.
It does not establish a received native reply or justify replay.

Keep the pre-mutation snapshot separate from the later certification time.
Do not backdate an evidence document or treat hook presence alone as return.
Preserve process deviations and historical provisional halts rather than
silently converting them to successful execution.

## Merge is not runtime adoption

Hosted race/coverage CI and approved merge prove the recorded source release,
not replacement of a running daemon. This closure is merge-only.
Monitoring baselines, acknowledged ownership, compatible rollback image
and rehearsal are future-adoption evidence, explicitly not activated here.
Artifact readiness does not authorize a closure PR merge or successor claim.
P-001 remains active until the separately approved closure PR merges.

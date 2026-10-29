---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 150-S and covering feature 169-F: signed delivery-evidence attestation verification (866FDC8C-B)."
doc_type: closure
schema_version: "1.0"
shipment_id: 150-S
feature_id: 169-F
source: docs/closure/150-S-169-F-post-merge-closure.md
title: "150-S / 169-F Post-Merge Closure"
---

# 150-S / 169-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `150-S` (866FDC8C-B) |
| Covering feature | `169-F` |
| Tasks | `169.001-T` to `169.015-T` (15 tasks, 8 waves) |
| Implementation PR | [#611](https://github.com/softwaresalt/backlogit/pull/611) (merge commit) |
| Merge commit | `b549518cfa6429d251c7d68ea7b1a41ddb2e8855` |
| Reviewed implementation HEAD | `184806958957221f94cd0bfca478212dc4a6b047` |
| Closure branch | `post-merge/150-s-signed-attestation-verification` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `target: all` invoked; memory compacted, 38 originals archived) |

## Delivered

* Core: attestation envelope types and parser (exact-byte and case-fold duplicate-member rejection at every depth), fail-closed verification (role-scoped anchor, pinned algorithm, binding and expiry checks, typed expired, revoked and wrong-shipment errors), rejection audit events with bounded diagnostics.
* Event binding: attestation metadata and the durable signed envelope persisted in `shipment_reconciled_shipped`; doctor re-verifies offline against the external pin set; the presence-only branch was removed.
* CLI: opt-in `reconcile-shipped --attestation <path>` with write-free dry-run; MCP doctor receives the same pin provider; CLI reference regenerated.
* Docs: `docs/design-docs/2026-09-16-866fdc8c-attestation-ci-verify.md`.

## Gates

* Local review: four passes, final `READY_WITH_FOLLOWUPS` (P0=0, P1=0 after the held-lock audit fix).
* CI: all required checks green on the reviewed HEAD; P-018 Copilot gate `SATISFIED` after 7 threads fixed, replied to and resolved over two rounds.
* Known environment notes: flaky `TestU175_*_LintFix` under load (pass in isolation); local `golangci-lint` binary incompatible with the toolchain.
* 169.005-T and 169.014-T had no natural RED phase; mutation checks stood in.

## Follow-ups (stash)

* `1D3EB9B7`: require a verifier whenever a core caller sets an attestation.
* `4DDF227B`: wire the revocation authority into the doctor reverifier.
* `0D468DC7`: validate payloadType, `_type`, `predicateType`, `issued_at`, and bind subject digests to the merge SHA.
* `D51C2537`: enforcement mode (deferred B6) and tooling notes.

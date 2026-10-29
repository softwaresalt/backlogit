---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 149-S and covering feature 168-F: external trust-anchor / verification-key lifecycle (866FDC8C-A)."
doc_type: closure
schema_version: "1.0"
shipment_id: 149-S
feature_id: 168-F
source: docs/closure/149-S-168-F-post-merge-closure.md
title: "149-S / 168-F Post-Merge Closure"
---

# 149-S / 168-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `149-S` (866FDC8C-A) |
| Covering feature | `168-F` |
| Tasks | `168.001-T` to `168.018-T`, `168.020-T` (19 tasks, 8 waves) |
| Implementation PR | [#609](https://github.com/softwaresalt/backlogit/pull/609) (merge commit) |
| Merge commit | `c7063679db8bd342eb547c3f3700ba8498e94af3` |
| Reviewed implementation HEAD | `01dff529d785f7333918b4ea72f0aec189629041` |
| Closure branch | `post-merge/149-s-trust-anchor-lifecycle` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `target: all` invoked; 37 memory files, below the 40-file threshold, no candidates) |

## Delivered

* Config: `trust_anchors` types, loader validation (id uniqueness, algo allowlist, strict status), structural `public_key_ref` contract.
* Core: external-pin gate, public-only key loader, fingerprint pin binding, pure resolver (validity, revocation, ambiguity, role-scoped, pinned algo), external-pin authority with bound role/validity, injected revocation authority with tombstones, mutation core with signed approval evidence, write-outcome marker protocol.
* CLI: `backlogit trust-anchor list|show|verify-key|add|rotate|revoke`; CLI reference regenerated.
* Docs: `docs/design-docs/2026-09-16-866fdc8c-trust-anchor-custody.md`.

## Gates

* Local review: three passes, final `READY_WITH_FOLLOWUPS` (P0=0, P1=0).
* CI: all required checks green on the reviewed HEAD; P-018 Copilot gate `SATISFIED` after 3 threads fixed and resolved.
* Known environment note: parallel full-suite runs show flaky `TestU175_*_LintFix` failures from golangci-lint contention; they pass in isolation.

## Follow-ups

* CLI rotate/revoke fail closed until a concrete `RevocationAuthority` is wired.
* Operator recovery or doctor path for an outstanding trust-anchor marker (out of scope, P-021).
* Combined resolve + load + pin-verify entry point.
* `FileTrustAnchorMarkerStore` nonce is store state, safe only under the per-root mutex.

---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 151-S and covering feature 170-F: authenticated per-invocation operator approval (866FDC8C-C)."
doc_type: closure
schema_version: "1.0"
shipment_id: 151-S
feature_id: 170-F
source: docs/closure/151-S-170-F-post-merge-closure.md
title: "151-S / 170-F Post-Merge Closure"
---

# 151-S / 170-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `151-S` (866FDC8C-C) |
| Covering feature | `170-F` |
| Tasks | `170.001-T` to `170.014-T`, `170.016-T`, `170.018-T` (16 tasks, 10 waves) |
| Implementation PR | [#613](https://github.com/softwaresalt/backlogit/pull/613) (merge commit) |
| Merge commit | `cf5a0ede52483b57db75cfcd3dd4391b067bd960` |
| Reviewed implementation HEAD | `9621da0f2dae22be285b130ed4617191bec7f0a1` |
| Closure branch | `post-merge/151-s-authenticated-per-invocation-operator-approval` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `compact-context` invoked) |

## Delivered

* Core: authorization token types, strict parser (exact-byte, case-fold and escape duplicate-member rejection at every depth), fail-closed verifier (signature via resolved anchor, role-scoped key, pinned algorithm, shipment binding, token and anchor validity windows, request-identity-digest binding with producer golden check), typed rejection audit.
* Anti-replay: durable nonce ledger, atomic compare-and-consume through an injected external rollback-resistant provider, typed indeterminate outcome on consume-without-audit, nonce bound to the request-identity digest for consumed-token resume.
* Enforcement: `ResolveAuthEnforcement` externally authoritative required-auth resolver (missing or malformed workspace policy fails closed); authenticated gate inside the locked reconcile critical section including the resume-append path; approver identity persisted into the prepared event.
* CLI: opt-in `reconcile-shipped --authorization-token <path>` with write-free dry-run verification outcome.
* Docs: request-identity-digest contract and token issuance/verification design docs under `docs/design-docs/`.

## Gates

| Gate | Result |
|---|---|
| Local review (P-014) | `READY_WITH_FOLLOWUPS` at the final HEAD (P0 0, P1 0, P2 1 deferred) |
| Copilot review (P-018) | `SATISFIED`; two rounds of findings fixed and every thread resolved |
| CI | All required checks passed for the merged HEAD |
| Merge strategy (P-009) | Merge commit |
| Local main sync | `main == origin/main` at `cf5a0ede52483b57db75cfcd3dd4391b067bd960` |

## Runtime verification and releasability

Releasability: `READY_WITH_CONDITIONS`. The feature adds no running service. The core primitives and the opt-in flag ship, but a live `--authorization-token` run fails closed with `ErrNonceProviderUnavailable` until a concrete external nonce provider exists, so only dry-run verification is operational.

* Monitoring: watch for `ErrNonceProviderUnavailable`, `ErrNonceConsumeIndeterminate` and token rejection outcomes in reconcile output.
* Rollback: revert the merge commit; the flag is opt-in, so unconfigured behavior stays confirmation-only v1.
* Owner: backlogit maintainers.
* Validation window: until the follow-up provider work lands.

## Follow-ups (stash)

* `2E8435DD`: wire the required-auth resolver into the reconcile-shipped CLI.
* `D1A14391`: supply a workspace-backed token rejection auditor in the CLI.
* `C887F4FE`: concrete external nonce provider, fail-fast when absent, operator digest helper.
* `D8B9C6DA`: canonicalize the resume digest inputs so crash-resume accepts non-canonical operator paths.

## Provenance

No source stash or deliberation artifact was archived or mutated by this closure; Stage owns any later triage.

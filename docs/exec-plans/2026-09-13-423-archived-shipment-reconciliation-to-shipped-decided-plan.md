---
chunk_strategy: h1-h2-h3
decided_from: docs/archive/plans/2026-09-05/2026-09-05-423-archived-shipment-reconciliation-to-shipped-plan.md
decided_at: 2026-09-13T20:55:00Z
doc_type: learning
schema_version: "1.0"
source: docs/exec-plans/2026-09-13-423-archived-shipment-reconciliation-to-shipped-decided-plan.md
title: "Decided Plan: Governed archived-shipment reconciliation to shipped (#423)"
---

## Decision Record: Governed archived-shipment reconciliation to shipped (#423)

**Review outcome**: PASS after multi-cycle remediation. **Release unit**:
148-S / 167-F. The plan became the basis for the shipped archive-backed
reconciliation flow now recorded in the final closure artifact.

### Executed Scope

* Use an in-place atomic frontmatter edit of the archived shipment file.
  Reject the naive `UnarchiveItem → mark shipped → ArchiveItem` round-trip,
  which would re-enter the live queue and require the forbidden normal shipped
  event.
* Hold the lock discipline in the order `C → B` for clobber safety while the
  membership lock remains held for the transaction.
* Keep the state classifier total and mutually exclusive across the
  no-op/resume/conflict/indeterminate branches.
* Persist and validate the prepared event, event digest, idempotency key, and
  request-identity digest so a replay can resume safely without re-reading the
  closure evidence.
* Treat the CLI surface as confirmation-gated and auditable rather than as a
  machine-authenticated operator boundary.

### Accepted Constraints

* Cryptographic authenticity and machine-authenticated invocation authorization
  were deferred as explicit follow-up scope, not silently absorbed into #423.
* `167.015-T` remained the blocking ratification gate for the authenticated
  authorization narrowing and no-descoping decision.
* The implementation stayed within one release unit and preserved merge-commit
  history.

### Review and Remediation History

* The first pass found a consensus P0 on the naive archive round-trip and forced
  the switch to the in-place atomic edit model.
* Later passes hardened the idempotency ordering, member-set validation,
  digesting, and rollback semantics until the classifier became total and the
  remaining residuals were explicitly scoped out.
* Two remaining follow-ups were explicitly scoped out rather than silently
  absorbed: the cryptographic/authenticated-authorization residual
  (`866FDC8C`, tracked as stash `B633E9B9`) and the verifiable legacy
  descope-provenance residual (tracked as stash `2B4E5AC3`) — both recorded
  on `167.015-T`'s ratification gate.

### Final Decisions

* Reconciliation is an in-place archive-file mutation, not a normal archive flow.
* The event log and artifact batch must be coordinated under the clobber-safe
  lock order already used by the surrounding transaction work.
* Review residuals are handled through explicit follow-up scope, not by mutating
  the shipped plan.

### Traceability

`148-S`, `167-F`, `167.015-T`, `866FDC8C`, issue #423, implementation PR #440, final
closure record `docs/closure/2026-09-13-148-s-operational-closure.md`.

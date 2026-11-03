---
chunk_strategy: h1-h2-h3
decided_from: docs/archive/plans/2026-09-06/2026-09-06-866fdc8c-trust-boundary-split-plan.md
decided_at: 2026-09-13T20:55:00Z
doc_type: learning
schema_version: "1.0"
source: docs/exec-plans/2026-09-13-866fdc8c-trust-boundary-split-decided-plan.md
title: "Decided Plan: 866FDC8C trust-boundary split"
---

## Decision Record: 866FDC8C trust-boundary split

**Review outcome**: PASS after remediation. The plan decomposed the trust-
boundary follow-up into three isolated release units and provided the planning
basis for the later feature work that depended on 167-F.

### Executed Scope

* External pin gating controls the in-workspace trust-anchor set; if the pin is
  absent, the feature fails closed.
* Read-only inspection and audited mutation were split into separate tasks so
  the surface stayed single-domain and reviewable.
* The attestation path and replay-digest decision were tied to the transaction
  boundary instead of being mixed into the request-identity digest.
* The core gate integration and CLI surface were separated to avoid a single
  task carrying both behavioral and operator-facing concerns.

### Accepted Constraints

* Feature B and Feature C stayed intentionally distinct, with B carrying the
  opt-in verification surface and C carrying the later enforcement / approval
  work.
* `167-F` remained the prerequisite dependency for the later trust-boundary
  release units.
* The request-identity-digest issuance flow remained a planning decision rather
  than an implementation blocker.
* Several task bodies still required acceptance criteria backfill before the plan
  could be treated as unconditional harvest-ready work.

### Review and Remediation History

* Security, architecture, and scope review all found real overreach risks until
  the plan was split around consumer boundaries and the trust root was pinned
  externally.
* The remediated plan resolved the major review concerns and was marked PASS
  with the remaining work explicitly tracked as follow-up or task-level detail.

### Final Decisions

* Keep the trust-anchor root externally pinned; do not treat the workspace as
  its own root of trust.
* Preserve the dependency edges so the later feature work composes correctly
  with `167-F`.
* Treat enforcement mode as a separately gated capability, not as a hidden
  assumption inside the plan.

### Traceability

`866FDC8C`, `167-F`, `168-F`, `169-F`, `170-F`, and the associated review
cycles captured in the source plan.

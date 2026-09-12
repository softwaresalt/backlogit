---
chunk_strategy: h1-h2-h3
compacted_at: 2026-09-12T02:40:00Z
compacted_from:
  - docs/memory/2026-08-21/ship-129-s-pa8-pa3-approval-gate-memory.md
  - docs/memory/2026-08-21/success-shaped-evidence-loss-stage-memory.md
  - docs/memory/2026-08-22/circuit-break-pr373-copilot-review-cycles.md
  - docs/memory/2026-08-22/ship-129-s-post-merge-closure-memory.md
doc_type: learning
schema_version: "1.0"
source: docs/memory/compacted/2026-09-11-129s-success-shaped-evidence-loss-compacted.md
source_shipment: 129-S
title: "Compacted Session: 129-S Success-Shaped Evidence Loss"
---

## Outcome

Shipment `129-S` and feature `146-F` delivered governed diagnostic-path
hardening through PR #373. The feature PR merged with merge commit
`15ab30a2a394439f52e5338fc94d1c50e3f395ae`. A ship-time evidence blocker was
captured as `DD957688`, repaired through the governed evidence path, and later
closed. The newest release-unit checkpoint remains in place at
`docs/memory/2026-08-24/ship-129-s-closure-memory.md`.

## Final Decisions

* The 23-task graph retained explicit PA-8 and PA-3 approval gates; Ship stopped
  rather than starting any gated task without approval
* Stage preserved one-to-one dependency and provenance evidence when harvesting
  `146-F` and assembling `129-S`
* Review findings were fixed only within the authorized contract surface;
  out-of-scope work remained separate follow-up scope
* The shipment-level evidence failure was repaired through the governed
  operation rather than bypassed or force-completed
* Merge history remained merge-commit based; no admin fallback or history
  rewrite was used

## Failed and Blocked Approaches

* The initial Ship run halted after 15 of 23 tasks because every remaining task
  required PA-8 or PA-3 approval
* PR #373 exhausted ordinary Copilot review-fix cycles and required an
  operator-authorized exceptional cycle
* The first post-merge shipment transition was blocked because task
  `146.006-T` lacked required delivery evidence

## Durable Learnings

* Approval-gated tasks must remain visibly blocked until the exact approval
  contract is satisfied
* Review-cycle exhaustion does not authorize scope expansion
* A merged feature and a shipped backlog shipment are separate closure states;
  governed evidence repair may be required between them

## Traceability IDs

Work IDs:

`129-S`, `146-F`, `146.001-T`, `146.002-T`, `146.004-T`, `146.005-T`,
`146.006-T`, `146.007-T`, `146.008-T`, `146.009-T`, `146.010-T`,
`146.011-T`, `146.012-T`, `146.013-T`, `146.014-T`, `146.015-T`,
`146.016-T`, `146.017-T`, `146.018-T`, `146.019-T`, `146.020-T`,
`146.021-T`, `146.022-T`, `146.023-T`, `146.024-T`.

Operational, stash, and commit tokens retained from the originals:

`DCC9B0C7`, `ECEBE820`, `6CD5B5EF`, `3FE9652F`, `AF5C6A1B`, `D0F81903`,
`36A7C055`, `64929F73`, `A47C624C`, `D5DC866C`, `BECA759E`, `95CE1375`,
`6508E8DF`, `AF67CE9C`, `924A941A`, `67F15DE4`, `83D5E272`, `EF0D9C3B`,
`EC3FE430`, `3BF7E207`, `F9A41311`, `36B38011`, `6D03554D`, `F89CADB7`,
`FA466658`, `8009EFA7`, `894DAAB4`, `DD957688`, `15AB30A2`, `4579F45C`,
`90911050`, `F00FB5D0`, `EA0B3724`, `5E86657E`, `D182B119`, `D3CE9E81`,
`EA1F5912`.

## Archived Originals

The four verbose originals are preserved byte-for-byte under
`docs/archive/memory/2026-08-21/` and `docs/archive/memory/2026-08-22/`.

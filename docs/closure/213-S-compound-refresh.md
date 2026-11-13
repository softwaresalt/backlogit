---
title: "213-S compound refresh"
doc_type: closure
source: docs/closure/213-S-compound-refresh.md
ingested_at: "2026-11-13T07:16:50Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
shipment_id: 213-S
feature_id: 212-F
---

## Invocation and evidence

Invoked `compound-refresh`, scope `recent` restricted to `213-S` / `212-F`,
mode `apply`. Known-file reads were used under the parent's declared
Engram/Graphtor degradation; no public lookup or index write occurred.

Evidence: merged PR 647, the actual published Markdown CLI invocation required
for this pass, the current Ship role boundary, operational-closure provenance
contract, and immutable
[public cascade evidence](evidence/213-S-212-F-close-evidence.json).

## Classifications

| Selected learning | Classification | Maintenance |
| --- | --- | --- |
| [Markdown library entrypoint false green](../compound/workflow-issues/markdownlint-library-entrypoint-false-green-2026-11-13.md) | keep | Accurate, distinct lesson; use the actual package bin and nonempty scan, not library-load exit 0 |
| [Historical source archival pattern](../compound/workflow-issues/source-artifact-archival-pattern-2026-04-20.md) | update; mark stale | Added current explicit-membership/provenance boundary and citations; historical examples retained, explicitly non-executable |

The old source-archival recommendation contradicted current authorization:
Ship records source IDs but does not mutate referenced stash/deliberation
artifacts outside explicit membership. The `213-S` public evidence retains
shared `082-DL`; `212-F` has neither source field. Narrative `ECF33C79` does
not authorize cleanup. All historical citations were preserved.

Reviewed: 2; kept: 1; updated/marked stale: 1; consolidated: 0; replaced: 0;
deleted/archived: 0. No unrelated compound entry was rewritten.

## Knowledge graduation and limitations

The U10-delivered [architecture](../ARCHITECTURE.md) and
[runtime/toolchain design](../design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md)
already describe the read-only library, Level 1 limits, unimplemented writing
APIs and later live consumption. No update is needed.
Plans, spikes, reviews and AGENTS are protected and unchanged.

Continuous-learning discovery was attempted by the parent: the named `learn`
SKILL read failed; `scripts/search.ps1 learn` returned compound/refresh only.
The configured pack therefore lacks callable named `learn`/`observe`/`evolve`
skills. No instincts, promotions, new skills or configuration were invented.
No new finding requiring capture arose; all 21 existing follow-ups remain
unchanged. Parent retains policy handling for any later finding.

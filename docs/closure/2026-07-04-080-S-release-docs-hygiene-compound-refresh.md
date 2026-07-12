---
chunk_strategy: h1-h2
description: 'Compound refresh report for shipment 080-S — reviewed the docs/compound/ entries adjacent to the shipped hygiene scope (workflow SHA-pinning F013, docline frontmatter contract, npm hybrid resolver, shipment/stash patterns f015) and classified every one as keep: none was superseded, invalidated, or duplicated by the 080-S changes, which preserved (did not alter) existing invariants. No new hard-won learning warranted capture — the work was routine hygiene (an additive env-indirection secret-presence guard, a characterization test against an isolated copy, and a docs wording correction). The one mildly-novel environmental gotcha (gofmt -l flags all files under a Windows CRLF working tree; CI on Linux/LF is authoritative) is recorded in the runtime-verification and closure artifacts rather than promoted to a compound learning.'
doc_type: closure
docline:
    ms.date: 2026-07-04T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-04T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-04-080-S-release-docs-hygiene-compound-refresh.md
title: 'Compound refresh — 080-S release pipeline & docs hygiene'
---

# Compound Refresh — 080-S release pipeline & docs hygiene

- **Context**: post-merge closure for shipment 080-S (feature 080-F, PR #174, merge `d0ebb4f`)
- **Scope**: `docs/compound/` entries adjacent to the shipped surfaces (CI workflow, docline docs, npm packaging, shipment/stash lifecycle)
- **Mode**: propose (review existing) — no new capture
- **Generated**: 2026-07-04

## Entries reviewed and classifications

| Entry | Classification | Rationale |
|---|---|---|
| `F013-workflow-sha-pinning.md` | **keep** | 080.001-T's guard is purely additive and **preserved** every third-party action's full-SHA pin (verified by the Security Reviewer and actionlint). The learning is neither exercised-against nor contradicted — it remains the governing rule and the change complies with it. |
| `2026-06-26-docline-frontmatter-contract.md` | **keep** | 080.003-T's docs wording fix and all five 080-S closure artifacts were authored to satisfy this contract (each passes the Docline frontmatter gate, 0 violations). The contract is applied, not superseded. |
| `npm-hybrid-go-binary-resolver-2026-04-28.md` | **keep** | 080.002-T only *characterizes* `scripts/package-npm.sh` output (valid package.json, version stamping, synced `optionalDependencies`); it does not change the resolver logic this entry documents. Orthogonal — kept verbatim. |
| `f015-shipment-stash-patterns.md` | **keep** | Shipment/stash lifecycle handling in 080-S (pre-archived items, `shipment ship` SHA stamping, source stashes retired by Stage) matched the documented patterns; nothing to update. |

No entries were classified `update`, `consolidate`, `replace`, or `delete`. Nothing in the
shipped scope superseded or invalidated an existing learning.

## New capture

**None.** 080-S was routine hygiene: an additive env-indirection secret-presence guard, a
characterization test run against an isolated copy, and a docs wording correction. No novel
error resolution, gotcha, or reusable pattern rose to the bar for a compound learning.

The one environmental note worth recording — `gofmt -l .` flags every `.go` file under a Windows
CRLF working-tree checkout (no `.gitattributes`, `core.autocrlf` on, blobs stored LF), so it is a
line-ending artifact rather than a content issue, and CI on Linux/LF is authoritative — is
captured in the runtime-verification and pre-merge closure artifacts. It is a known local-tooling
caveat, not a durable engineering learning, so it is intentionally **not** promoted to
`docs/compound/`.

## Follow-up items

- None requiring manual review. The deferred/out-of-scope stashes (`34F11E5A` external
  npm/NPM_TOKEN provisioning, `EED25928` external `.tmpl` parity, `21E17BFC` singleton-MCP
  contingency) are Stage-owned and out of Ship's P-010 boundary — left untouched.

## Evidence

- Shipped, merged code on `main` at `d0ebb4f` (feature PR #174).
- Review gate: 4 personas (Constitution / Go / Security / Scope-Boundary), **zero P0/P1/P2**.
- Security Reviewer confirmed SHA pins intact, `contents: read` + `persist-credentials: false`
  preserved, secret handled via boolean-only `$GITHUB_OUTPUT`.
- Runtime verification PASS WITH FOLLOW-UP:
  `docs/closure/2026-07-04-080-S-release-docs-hygiene-runtime-verification.md`.

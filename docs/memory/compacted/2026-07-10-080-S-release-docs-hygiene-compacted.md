---
chunk_strategy: h1-h2-h3
description: Compacted Stage and Ship memory for shipment 080-S release pipeline and documentation hygiene.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-080-S-release-docs-hygiene-compacted.md
title: Compacted memory - 080-S release and docs hygiene
---
## Summary

Shipment `080-S` shipped release-pipeline and docs hygiene. Stage bundled `9140F65C` and `B55985DD` into `080-F`; Ship guarded npm publish steps on `NPM_TOKEN`, characterized npm package output, fixed docs-lint wording, merged PR #174, and completed post-merge closure.

## Archived originals

* `docs/archive/memory/2026-07-04-stage-080-S-release-docs-hygiene-session.md`
* `docs/archive/memory/2026-07-04-ship-080-S-session.md`

## Decisions and outcomes

* Release workflow token presence is checked through env-indirection and boolean output; the token is never echoed and publish steps retain `continue-on-error: true`.
* `retired packaging script` received characterization through a thin Go wrapper; the script itself stayed unchanged and `npm pack` remained optional.
* Docs were clarified to distinguish repo-wide `make docs-lint` from scoped `go run ./cmd/backlogit docs lint --path <file>`.
* PR #174 merged by true merge commit `d0ebb4f`; post-merge `shipment ship 080-S` archived all scoped tasks, feature, and shipment with clean reconcile.

## Files and verification

* `.github/workflows/release.yml`, `retired packaging characterization test`, and two docs/backlog wording surfaces were updated.
* `actionlint`, YAML parse, scoped and repo-wide docs lint, Go tests, vet, lint, and CI all passed; Copilot produced no inline threads.
* Runtime verification was PASS WITH FOLLOW-UP: observe the guard on the next real tagged release.
* Compound refresh kept existing learnings and did not promote the Windows CRLF/gofmt gotcha.

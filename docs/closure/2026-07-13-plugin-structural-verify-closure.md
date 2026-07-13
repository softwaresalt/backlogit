---
chunk_strategy: h1-h2-h3
description: "Closure for 101-F — structural verification of the distributable plugin bundle."
doc_type: closure
docline:
  ms.date: 2026-07-13T00:00:00Z
  ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-13-plugin-structural-verify-closure.md
title: "101-F plugin structural verification closure"
---

## Outcome

Feature `101-F` replaced the broken plugin byte-identity drift check with
structural verification of the distributable plugin bundle. PR #227 merged with
merge commit `4cf30e22e2608c2f2e535fe8d4517568253c69e1`.

The decision is recorded in
`docs/decisions/2026-07-13-plugin-bundle-structural-verification-decision.md`.
`plugin/` is treated as the product bundle installed into user workspaces;
`.github/` remains backlogit's self-hosting harness.

## Verification

Local gates passed before merge:

```text
go test ./tests/integration/ -run 'TestPluginBundleStructurallyValid' -count=1
.\make.ps1 verify-plugin
go build ./cmd/backlogit
go test ./...
go vet ./...
golangci-lint run
gofmt -l .
go run ./cmd/backlogit docs lint --path docs\decisions\2026-07-13-plugin-bundle-structural-verification-decision.md
go run ./cmd/backlogit docs lint --path docs\plugin-guide.md
```

Negative-proof guardrail: temporarily renaming `plugin\skills\spike` to
`spike.__negative-proof` made `TestPluginBundleStructurallyValid` fail on the
missing expected skill and stray directory. The directory was restored before
implementation continued.

CI checks on PR #227 passed:

```text
Detect code changes: pass
Docline frontmatter gate: pass
CLI Reference Drift: pass
test: pass
```

## Review and Copilot loop

The local review skill reported no accepted P0/P1 findings after remediation.
A false-positive scope finding referenced `internal/core/shipment_gate.go`, but
`git diff` confirmed that file was not changed.

Copilot review iterations:

1. Initial review on `e1691787594de4ef56ffd879ab5ef5c985416649`.
   - Found CI fail-open behavior for plugin Markdown-only edits.
   - Fixed in `20baf96e854ad98066981be09423398e5cde9311`.
   - Replied to both threads and resolved them.
2. Follow-up review on `20baf96e854ad98066981be09423398e5cde9311`.
   - Found structural metadata validation was too weak.
   - Fixed in `937c7ec4e80c52e06dcf6170669ce7aa18d06c90`.
   - Replied to the thread and resolved it.
3. Final review on `937c7ec4e80c52e06dcf6170669ce7aa18d06c90`.
   - Latest Copilot review covered the PR head.
   - Zero unresolved Copilot threads remained.

## Backlog state

Stash `84B73A39` was harvested into feature `101-F` with provenance preserved
through `stash_links` and `source_stash_*` fields. After merge, `101-F` was
marked done, associated with merge commit
`4cf30e22e2608c2f2e535fe8d4517568253c69e1`, and archived.

Follow-up stash `E75605E5` tracks the pre-existing plugin `spike` example
frontmatter issue identified during review.

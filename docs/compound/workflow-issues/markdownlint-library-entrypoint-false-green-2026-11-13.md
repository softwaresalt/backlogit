---
title: Markdownlint library entrypoint can produce a false-green exit
description: Use the published CLI executable and verify a nonempty lint scan.
doc_type: learning
source: docs/compound/workflow-issues/markdownlint-library-entrypoint-false-green-2026-11-13.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  category: workflow_issue
  component: markdown_lint
  date: "2026-11-13T00:00:00Z"
  problem_type: workflow_issue
  root_cause: incorrect_entrypoint
  resolution_type: workaround
  resolved: true
  severity: medium
  message: A library-only markdownlint-cli2.mjs invocation exits zero without linting.
  file_path: .backlogit/archive/212.003-T.md
  tags:
    - markdownlint
    - ci
    - false-green
    - cli
citations:
  - https://github.com/softwaresalt/backlogit/pull/647
  - docs/memory/2026-11-13-213-s-pr-readiness.md
---

## Problem

Local invocation of the package's `markdownlint-cli2.mjs` file exited zero
without a scan. CI's real `make md-lint` command subsequently found an MD001
heading-level error in the U10 execution notes.

## Root Cause

The package includes a library entrypoint and a separate executable CLI
entrypoint. Loading the library with Node does not run the command-line scanner.
Process success alone therefore did not establish successful linting.

## Resolution

Use the executable declared by the installed package's `bin` metadata.
For the observed markdownlint-cli2 0.23.1 package, it was
`markdownlint-cli2-bin.mjs`.

```powershell
node <installed-package>/markdownlint-cli2-bin.mjs "**/*.md" "!.git/**"
```

The real CLI reproduced the failure. Removing one excess heading marker fixed
it without changing the task contract, Go code, or schemas. A full local scan
of 4146 Markdown files and the subsequent hosted Markdown check passed.

## Prevention

* Require an actual CLI banner, scan count, and summary before calling lint PASS
* Verify that the selected corpus is nonempty, as well as checking the exit code
* Use the package's published executable rather than guessing from module names
* Preserve the configured ignore rules; do not weaken them to hide failures
* When working offline, use the already-installed executable and its actual
  version rather than installing or assuming a newer cached package

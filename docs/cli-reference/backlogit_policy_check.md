---
chunk_strategy: h1-h2-h3
description: Evaluate one task's commit-bound evidence against the workflow-policy graph
doc_type: reference
ingested_at: "2026-10-17T21:55:52Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_policy_check.md
title: backlogit policy check
---

## backlogit policy check

Evaluate one task's commit-bound evidence against the workflow-policy graph

### Synopsis

Evaluate the prerequisite graph harness-scaffold -> red-phase-verify ->
green-phase-verify -> review-complete -> ci-pass -> merge-ready for one task.

This is the same evaluation the MCP and agent surfaces run (the shared
policyengine Engine.Check); no surface has a separate path. Every gate fails
closed: absent, ambiguous, unverifiable or out-of-order evidence blocks.
Evidence commits are verified against the protected remote-tracking refs
(--protected-ref), never only a local branch. review-complete and ci-pass are
derived from the pull request in the evidence bundle (requires --repo, which
must match the github.com owner/name of the origin remote; when origin does
not resolve to one, enforce mode fails closed and report-only mode warns);
self-asserted records for them are refused. Workspace-relative paths resolve
against the git toplevel, not the current directory.

The registry's mode applies (the shipped registry is report-only: violations
are reported and the exit status stays zero). --mode may tighten a report-only
registry but never relax an enforce one. A malformed request is always an
error.

Overrides (P-002.1 harness-exempt) need a signed operator token verified
against --trust, recorded in the hash-chained --ledger and anchored in
--anchor. All three must be absolute paths outside the git toplevel, and the
ledger and anchor must be in different directories. Tokens are
refused outside an interactive terminal; operator identity is never read from
the evidence or the command line.

```text
backlogit policy check [flags]
```

### Examples

```text
  backlogit policy check --evidence evidence/163.003-T.json --protected-ref refs/remotes/origin/main --protected-ref refs/remotes/origin/feat/x --repo owner/name
  backlogit policy check --evidence bundle.json --mode enforce --format json
```

### Options

```text
      --anchor string                absolute path of the ledger anchor (outside the repository)
      --evidence string              workspace-relative JSON evidence bundle (required)
      --format string                output format: text, json (default: text on TTY, json otherwise)
  -h, --help                         help for check
      --ledger string                absolute path of the override ledger (outside the repository)
      --mode string                  enforcement mode: report-only, enforce (may only tighten the registry mode)
      --override-token stringArray   workspace-relative signed operator override token file (repeatable)
      --protected-ref stringArray    remote-tracking ref the head commit must be reachable from (repeatable) (default [refs/remotes/origin/main])
      --registry string              workspace-relative workflow-policy registry (default ".github/policies/workflow-policy-registry.yaml")
      --repo string                  owner/name repository used to derive review and CI evidence
      --trust string                 absolute path of the operator trust configuration (outside the repository)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit policy](backlogit_policy.md)	 - Enforce workflow-policy gates (harness, red/green, review, CI, merge-ready)


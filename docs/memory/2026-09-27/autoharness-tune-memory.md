---
title: Autoharness tune 2026-09-27 session memory
doc_type: memory
source: docs/memory/2026-09-27/autoharness-tune-memory.md
---

## Outcome

An Auto-Tune 1.5.0 pass ran on branch `chore/autoharness-tune-2026-09-27`. The
full report is at `.autoharness/tuning-reports/2026-09-27-tuning-report.md`.
Pre-edit backups are in `.autoharness/backups/2026-09-27/`.

## Changes

* Refreshed the lock scripts (`scripts/acquire_lock.*`, `scripts/release_lock.*`)
  to the capability-token plus containment model. Also updated the concurrency
  instruction, the file-lock skill, and the Stage/Ship lock commands.
* Propagated the `config.model_routing` tier routes into the frontmatter of 18
  agents. Updated the anchor route to `gpt-6-sol` and the escalation route to
  `claude-opus-5.5`/anthropic/xhigh, and refreshed the orchestrator routing table.
* Fixed the Ship closure-sequence phrase, which now passes the verifier.
* Added the backlog-integration registry rows and the security-reviewer Scope
  Disposition section.
* Merged the constitution P-016/P-020 text and the spike docline/graphtor-docs
  guidance.
* Updated the manifest: LF checksums, variables, `tuned_at`, and a new
  `tuning_history` entry.

## Decisions

* Staged renders were not adopted wholesale. They drop `model_tier`, demote the
  H1, and leave `{{QUALITY_GATE_n}}` unresolved.
* The 79 checksum `user-modified` hits are CRLF false positives. I confirmed each
  one by LF re-hashing.

## Failed Approaches

* A multiline PowerShell regex using `.*$` consumed `\r`. Use `[^\r\n]*`.
* The ordinal sort of table rows misplaced a backtick-prefixed row. Strip
  backticks from the sort key.
* The manifest artifact indent is 4 spaces, not 6.

## Next Steps

* Review the diff and open a PR against `main`.
* Hand-merge TUNE-009 through TUNE-012 (runtime-verification/closure,
  shipment-reconcile, workflow-policies, backlogit checkpoint contract).
* Operator decisions: choose a cross-family escalation route, and decide on the
  custom `model:` keys in `sqlite-reviewer` and `go-mcp-expert`.

## Second Session — Review Follow-Up

* Applied TUNE-009 and TUNE-010, and parts of TUNE-011 and TUNE-012. See the
  tuning report's Follow-Up Review Pass section.
* Fixed the release/acquire lock-script bugs: default root, containment, and
  exit codes.
* The registry parity test rejects bracketed alternatives in `cli_command`.
  Use plain `{{param}}` flags and document alternatives in `rationale`.
* The verifier requires the literal `Source artifact cleanup` label in
  operational-closure.
* Next: open the PR, rebuild `backlogit.exe` from `main`, and upstream the lock
  fixes and template defects.

## Third Session — Operator Follow-Up

* Escalation route is now `gpt-6-sol`/openai/xhigh, a cross-family route
  approved by the operator.
* The P3 lock-script fix dropped 11 stderr messages; restored and verified
  under PowerShell 5.1 and 7.
* Upstream proposals: `docs/decisions/2026-09-27-autoharness-upstream-fix-proposals.md`.
* Built `bin\backlogit-main.exe` from `main` with a Go pseudo-version stamp. A
  `git describe` stamp sorts as a prerelease and falsely reports an available
  update.
* Installing to `C:\Tools` is operator-owned: it is outside the workspace and
  the executable is locked by running MCP servers.

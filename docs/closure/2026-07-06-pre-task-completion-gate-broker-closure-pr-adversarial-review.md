---
chunk_strategy: h1-h2-h3
description: 'Mandatory pre-push adversarial multi-model review of the DOCS-ONLY post-merge closure changeset for shipment 082-S (feature 082-F, pre-task-completion gate broker) on branch post-merge/082-S. Scope: backlog archival + 3 compound learnings + post-merge operational-closure artifact + session memory — no Go code changed. Reviewer-A (GPT-5.4, high effort) verified every technical claim against the merged source and returned BLOCK with 1 CRITICAL (MinimalEnv PATH/PATHEXT overclaim), 4 MAJOR (probe-timeout retryable misclassification; shipment ran=false invariant overclaim vs known F4 gap; pass->done vs requested-terminal-status; missing terminal_statuses config field), and 1 MINOR (`autoharness --version` vs `version` subcommand). All 6 remediated in the docs. Reviewer-B (Gemini 3.1 Pro) failed to return a response and was replaced by a fresh independent verification reviewer (Claude Opus 4.8, high effort) run on the corrected docs, which confirmed all 6 fixes accurate against merged code with correct citations and returned PROCEED (no remaining CRITICAL/MAJOR/MINOR; 1 discretionary NIT on an approximate line number, fixed). Gate decision: remediate-then-PROCEED.'
doc_type: closure
docline:
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-06-pre-task-completion-gate-broker-closure-pr-adversarial-review.md
title: 082-S post-merge closure PR — Adversarial Multi-Model Review
---

# Adversarial Review — 082-S Post-Merge Closure PR (docs-only)

- **Date:** 2026-07-06
- **Branch:** `post-merge/082-S` (base `main`)
- **Scope:** the post-merge closure changeset — backlog archival under
  `.backlogit/` + 3 compound learnings + post-merge operational-closure artifact
  + session memory. **No Go source changed** (the feature itself already merged in
  PR #178, merge `e47e1291c49f906a4b257c60f117a2cd05107db7`).
- **Review objective:** every technical claim in the closure/compound docs must be
  factually accurate against the *actually-merged* code. Inaccurate invariants,
  exit codes, function names, or file paths are defects.
- **Reviewers:**
  - Reviewer-A — GPT-5.4 (high effort) — full claim-vs-source verification.
  - Reviewer-B — Gemini 3.1 Pro — **failed to return a response**; replaced by:
  - Verification reviewer — Claude Opus 4.8 (high effort) — independent re-check of
    the corrected docs.

## Verdict

**Remediate-then-PROCEED.** Reviewer-A returned **BLOCK** with 6 findings; all 6
were verified against the merged source by the orchestrator and remediated in the
docs. The independent verification reviewer then returned **PROCEED** on the
corrected docs.

## Findings by confidence tier and disposition

| # | Finding | Doc | Severity (Reviewer-A) | Verified vs code | Disposition |
|---|---|---|---|---|---|
| 1 | `MinimalEnv` claimed to make PATH resolution "not steerable" — but it forwards ambient `PATH`/`PATHEXT` (`runner.go:109,114,121`) | RCE compound | **CRITICAL** | Confirmed | **Fixed** — reworded to "drops arbitrary injected vars but forwards ambient PATH/PATHEXT; guarantee = untrusted *config* cannot choose the path" |
| 2 | Probe timeout claimed "retryable" — actually `failProbe`→setup (fail-closed under `enabled:true`)/fail-open under `auto` (`probe.go:36-59`); only `ExecRunner` maps `ErrGateTimeout` (`runner.go:86-88`) | timeout compound + contract | **MAJOR** | Confirmed | **Fixed** — "retryable" scoped to the gate-check runner; probe classification documented |
| 3 | Shipment gate claimed to reject `ran=false` members — code never inspects `delta.ran` (`shipment_gate.go:124-139`); this is the stashed F4 gap | contract | **MAJOR** | Confirmed | **Fixed** — reframed as a documented known gap (F4 / `9822F787`), not an active invariant |
| 4 | "pass → move task → done" — code writes the **requested** terminal status via `completeGatePass`/`updateArtifactUngated` (`gate_transition.go:226,249`) | contract | **MAJOR** | Confirmed | **Fixed** — "pass → requested terminal status (default `[done]`)" |
| 5 | Owned-config list omitted `terminal_statuses` (`schema.go:143`, default `[done]`) | contract | **MAJOR** | Confirmed | **Fixed** — `terminal_statuses` added to owned-config list |
| 6 | Probe command written as `autoharness --version` — code runs `<binary> version` (`probe.go:112`) | timeout compound | **MINOR** | Confirmed | **Fixed** — `--version` → `version` |
| 7 | `validateGateBinary` cited at `~:234`; declaration at `schema.go:240` | RCE compound | **NIT** (verification reviewer) | Confirmed | **Fixed** — bumped to `~:240` |

## Independent verification cross-checks (all accurate)

Confirmed by the verification reviewer against merged source: backlogit gate exit
codes (`ExitGateBlocked=6`, `ExitGateConfig=7`, `ExitGateRetryable=8`);
`MinAutoharnessVersion="1.4.7"`; `--no-count` present in `BuildArgs`; base-ref
precedence `config → --gate-base → origin/HEAD → origin/main → main`; exit 2 =
config error; MinimalEnv injected into all three runners; docline `source:` fields
match file paths; archival arithmetic (24 archived = 082-F + 5 tasks + 17 subtasks
+ 082-S; 23-item manifest); no internal contradictions across the four docs.

## Security invariants (unchanged by this docs-only change)

The merged code's security invariants are unaffected — this review only corrected
how the docs *describe* them. As merged and re-confirmed: argv-array-only exec
(never a shell string), bare-PATH `autoharness_binary` validation
(`validateGateBinary` rejects absolute/separator/`..`), MinimalEnv allowlist,
timeout-bounded probe + gate check, logs-only evidence, CLI-only audited force with
no MCP force field, one-way `core → gate` boundary.

## Notes on adversarial value

The docs-only closure changeset carried real risk: durable compound learnings that
misstate a security invariant (finding 1) or an enforcement guarantee (finding 3)
would have graduated *wrong* institutional knowledge. The multi-model review caught
a self-contradiction (the contract doc asserted an invariant the same closure doc
listed as a deferred F4 gap) that a single-pass author review missed. Reviewer-B's
non-response was handled by substituting a fresh independent model rather than
proceeding on a single reviewer.

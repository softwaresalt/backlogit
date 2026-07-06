---
chunk_strategy: h1-h2-h3
description: 'Pre-merge runtime verification for shipment 082-S — pre-task-completion gate broker (082-F). The changed runtime surface is the CLI move/update completion path and the shipment completion path, which now synchronously invoke autoharness gate check before a task/subtask enters a terminal status and before a shipment is marked shipped. Verified against the installed autoharness 1.4.7 (version probe target). Evidence: (1) autoharness version = 1.4.7; gate check --base <ref> --json contract confirmed (documented exit codes 0 pass / 1 blocked / 2 config; --no-count and --force present and mutually exclusive). (2) Real-repo composition — no validation gates in .autoharness/config.yaml -> exit 0 "no validation gates configured" (fail-open). (3) Scratch e2e PASS under enabled:true (fail-closed config): backlogit invoked the real autoharness binary, resolved base_ref auto->main (no remote in scratch), got exit 0, moved the task active->done, recorded gate_report_hash, wrote a logs-only pre_task_completion_gate_passed event (ran:true, NOT frontmatter), and emitted the structured JSON GateOutcome; move exit 0. (4) Scratch e2e FAIL-CLOSED: missing autoharness_binary under enabled:true -> refused with backlogit exit 7 (setup error), task RETAINED active (no partial completion), matching the locked exit mapping. (5) Copilot-review fix for update --json + --section was verified e2e: the section write now lands AND the gate JSON is emitted (no silent drop). Verdict PASS — the real composed gate path passes and fails closed correctly; the exit-1 block/requeue/escalate decision mappings are covered by the deterministic unit suite (23 packages green) since exercising a real failing autoharness gate requires an autoharness gate-config schema out of scope for this shipment.'
doc_type: closure
docline:
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-06-pre-task-completion-gate-broker-runtime-verification.md
title: 082-S pre-task-completion gate broker — Pre-Merge Runtime Verification
---

# Runtime Verification — 082-S pre-task-completion gate broker

**Surface**: `cli` (move/update completion) + core shipment completion — the broker shells out to `autoharness gate check`.
**Mode**: `manual` (real-binary integration in an isolated scratch workspace).
**Target**: `backlogit move/update <task> --status done`; `autoharness gate check`.
**Context**: PR #178, branch `feat/pre-task-completion-gate-broker`, shipment 082-S / feature 082-F.
**Installed autoharness**: 1.4.7 (matches the minimum contract-probe target `>= 1.4.7`).

## Verdict: PASS

The real composed gate path was exercised end-to-end against the installed
autoharness 1.4.7 binary. Both the fail-open (auto/no-gates) and fail-closed
(missing binary under `enabled:true`) branches behaved exactly as the locked
design specifies. The exit-1 refusal family (block / requeue / escalate) decision
mapping is covered by the deterministic unit suite (injected fake runners), since
constructing a *real* failing autoharness gate requires authoring an autoharness
gate-config (`gates:` schema) that is out of scope for this shipment.

## Evidence

### E1 — Binary + contract probe

* `autoharness version` → `1.4.7` (equals the broker's minimum probe target).
* `autoharness gate check --help` confirms the contract backlogit maps:
  * `--base <ref>` required; `--task`, `--head`, `--workspace`, `--json`, `--force`, `--no-count` present.
  * Exit codes: `0` all matched gates passed / no gates / no files; `1` blocked (unless advisory); `2` invalid args or gate config.
  * `--force` and `--no-count` documented mutually exclusive (matches `BuildArgs` invariant).

### E2 — Real-repo composition (fail-open)

* `autoharness gate check --base origin/main --json` in the backlogit repo →
  `No validation gates configured; nothing to check.` exit `0`.
* Confirms the `auto` composition: when autoharness reports no gates, completion proceeds.

### E3 — Scratch e2e PASS (enabled:true, fail-closed config, gate actually runs)

Isolated temp git repo + `backlogit init` + copied `.autoharness/config.yaml` +
`hooks.yaml` `pre_task_completion_gate.enabled: true`:

* `backlogit move 001.001-T --status done --json` →
  `{"id":"001.001-T","outcome":"passed","old_status":"active","new_status":"done","state_changed":true,"base_ref":"main","head_ref":"HEAD","head_sha":"…","gate_report_hash":"8ac746…"}` exit `0`.
* Base-ref resolved `auto → main` (no remote → `origin/HEAD`/`origin/main` absent → `main`) — exactly the documented precedence tail.
* Item log recorded a single **logs-only** evidence event:
  `pre_task_completion_gate_passed` with `delta.ran=true`, `outcome=passed`, `base_ref=main`, `gate_report_hash=…`, `head_sha=…` — confirming evidence lands in `.backlogit/logs/*.jsonl`, never frontmatter.

### E4 — Scratch e2e FAIL-CLOSED (missing binary under enabled:true)

* `hooks.yaml` `autoharness_binary: autoharness-nonexistent`, `enabled: true`.
* `backlogit move 001.002-T --status done --json` → **exit 7 (setup error)**; the task **retained `active`**.
* Confirms the locked mapping: binary-not-found → setup error → fail-closed under `enabled:true`; no partial completion (old status re-read and retained).

### E5 — Copilot fix regression (update --json + --section)

* After the fix, `backlogit update 001.001-T --status done --section notes=FINAL_MARKER --json` (real gate, `enabled:true`):
  * Emitted the gate JSON (`outcome:passed`), **and**
  * Wrote `<!-- BEGIN:notes -->FINAL_MARKER` into the relocated `archive/001.001-T.md`.
* Confirms the section write is no longer silently dropped behind the early `--json` return.

## Invariants confirmed at runtime

* argv-array exec only (no shell string); autoharness invoked with discrete args.
* Logs-only evidence (item `.jsonl`), never frontmatter.
* No partial completion — a refusal retains the re-read prior status.
* Fail-open under `auto` with no usable gate; fail-closed under `true` with a missing binary.
* Structured JSON GateOutcome emitted for machine callers on the pass path.

## Follow-up risks

* Exit-1 block/requeue/escalate against a *real* failing autoharness gate is unit-tested
  (fake runners) but not exercised with a live failing gate here (requires autoharness gate-config schema — out of scope). Low risk: the decision logic is pure and fully covered.
* Deferred hardening items (adversarial F1/F4/F5/F7) are stashed as low-priority follow-ups (see closure artifact).

## Recommended next action

Proceed to operational closure and present PR #178 as merge-ready. HALT for operator P-014 merge approval.

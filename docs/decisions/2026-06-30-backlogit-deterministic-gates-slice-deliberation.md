---
title: "backlogit slice of Deterministic Gates, Telemetry & Evaluation Engine"
description: "Scope, design decisions, and open-question resolutions for the backlogit-owned portion of the Deterministic Gates initiative"
topic: "Deterministic Gates state authority in backlogit: doctor target-mode, per-task locking, task size schema + mutation CLI"
depth: "standard"
decision_status: "decided"
promoted_to: "plan"
stash_id: "AE0838A9"
source_design_doc: "docs/design-docs/autoharness-evals-gates-design.md"
linked_artifacts:
  - "docs/exec-plans/2026-06-30-backlogit-deterministic-gates-slice-plan.md"
tags:
  - deliberation
  - deterministic-gates
  - doctor
  - concurrency
  - header-def
  - mdfront
  - cli-mcp-parity
---

## Problem Frame

The **Deterministic Gates, Telemetry & Evaluation Engine** initiative
(`docs/design-docs/autoharness-evals-gates-design.md`, 2026-06-30, Proposed/Active)
transitions autoharness into a deterministic, closed-loop execution engine. The
design deliberately partitions responsibilities (§2): autoharness owns
orchestration/telemetry, **backlogit owns the `.backlogit/` work-state authority
and state-machine transitions**, and agent-engram/docline own document/AST
integrity.

This deliberation covers **only the backlogit-owned slice**. autoharness
(orchestrator/observer) tasks and agent-engram/docline (structural authority)
tasks live in their own repos and are **explicitly OUT OF SCOPE** here.

The backlogit slice is four concrete work items across two design phases:

**Phase 1 — Deterministic Gates & State Synchronization (§3):**
1. **Restrict Doctor Scope** — ensure `backlogit doctor` only validates
   `.backlogit/` artifacts against `header-def.yaml`, and add a **single-file
   target mode** (`backlogit doctor --target {file_path}`, 5s timeout) so
   autoharness `pre_task_completion` gates can validate one queue file at a time
   (design §5 config: `command: "backlogit doctor --target {file_path}"`,
   `timeout_seconds: 5`).
2. **State Locking** — a per-task lock (`.lock` sidecar) preventing concurrent
   modification to a task while it is undergoing validation, aligned with
   `concurrency.instructions.md` conventions.

**Phase 2 — Telemetry, Estimations & Compound Learning (§4):**
3. **Schema Update** — allow an optional `size` (T-shirt) attribute in task
   frontmatter via `header-def.yaml`.
4. **Mutation CLI** — expose `backlogit update {task_id} --size {value}` so
   autoharness can inject an estimated size back into the `.backlogit` markdown
   **without destroying the frontmatter/AST** (body-preserving write via
   `internal/mdfront`).

### Who cares and why

* **autoharness** is the primary consumer: its `pre_task_completion` validation
  gate shells out to `backlogit doctor --target …` and its `pre_execution`
  sizing hook shells out to `backlogit update … --size …` (design §5). Both are
  **CLI contracts across a repo boundary** — flag names, exit codes, and the 5s
  timeout are the interface.
* **backlogit** must remain the sole authority for `.backlogit/` state and must
  not corrupt queue files when mutated by an automated hook.

### Success criteria

* `backlogit doctor --target {file}` validates exactly one `.backlogit/` artifact
  against `header-def.yaml`, returns a deterministic exit code (0 pass / non-zero
  fail), completes within a 5s timeout, and never scans `docs/` or non-`.backlogit`
  paths.
* Concurrent modification of a task while it is being validated is prevented by a
  crash-safe per-task lock.
* `task` frontmatter may carry an optional `size` field validated against
  `header-def.yaml`.
* `backlogit update --size {value}` writes the size field while preserving the
  rest of the frontmatter and the entire body **byte-for-byte** (provable via
  golden test).
* CLI and MCP entry points stay at parity (documented hard convention).

### Scope boundaries (explicitly OUT of scope)

* autoharness lifecycle hooks, git-diff discovery, subprocess interceptor,
  telemetry emitter, SQLite aggregator, reviewer matrix, headless eval runner (§3, §4).
* agent-engram/docline `verify` CLI, reactive sync daemon, CozoDB telemetry schema (§3, §4).
* Any new telemetry storage inside backlogit (see Q3 resolution below).
* Any retroactive rewrite of existing task files' frontmatter ordering/comments.

## Research Findings

Grounded in the current repository (all paths verified):

* **Doctor scope is already correct.** `core.Doctor` (`internal/core/doctor.go:125-387`)
  scans only canonical `.backlogit/` dirs via `artifactSearchDirs`
  (`internal/core/artifacts.go:553-598`) plus `.backlogit/archive`; it does **not**
  scan `docs/`. So item 1's "restrict scope" is largely a **verify-and-guard**
  task (characterization test), not a rewrite. What is genuinely new is the
  `--target` single-file **header-def validation** mode — doctor does not
  currently validate any file against `header-def.yaml` (it does orphan /
  duplicate / archived-from checks).
* **header-def validation already exists** and is reusable: `LoadHeaderDef`
  (`internal/config/headerdef.go:43-61`), `ResolveFieldSchema`
  (`:64-82`), and `ValidateArtifactFields` (`internal/core/field_validation.go:11-34`).
  The `task` type (`.backlogit/header-def.yaml:51-63`) currently declares only
  `priority`; adding an optional `size` field flows through the existing
  validation path (required = `Optional==false && Immutable==false`).
* **mdfront is a stdlib-only leaf codec** (`internal/mdfront/codec.go`:
  `Markdown`, `Decode`, `Encode`) that is body-preserving. But `backlogit update`
  currently does **not** use it — it routes through `core.UpdateArtifact` →
  `persistArtifact` → `WriteArtifactFile` (`internal/core/artifacts.go:637-690`),
  which **rebuilds** the whole file from the model. That is the exact
  "destroys frontmatter/AST" risk item 4 warns about. Prior learning
  `2026-06-28-codec-extraction-leaf-packages.md` (068-S) established the
  body-preserving pattern (`mdfront` + `internal/atomicfile`) and proved it with
  **differential golden byte-equality tests**.
* **Locking**: only `internal/core/stash_lock.go` exists (a `.lock` sidecar).
  Prior learning `advisory-file-lock-stale-ttl-go-2026-04-08.md` documents the
  crash-safe pattern: `sync.Mutex` + `O_CREATE|O_EXCL` sidecar + **stale TTL**
  (default 60s) + single retry, with Windows-safe behavior (PID-based liveness is
  unreliable on Windows). A per-task lock should reuse this pattern.
* **Telemetry** tables (`telemetry_sessions`, `telemetry_tool_usage` in
  `internal/db/telemetry_schema.go`) are unaffected — no backlogit storage change
  is required (design §4/§6 keeps the metrics DB in `.autoharness/metrics`).
* **CLI/MCP parity is a documented hard convention.**
  `2026-05-07-mcp-cli-config-parity.md` records a P2 review finding (049-S) where
  a CLI option was not mirrored in the MCP handler. backlogit is a `tool_type:
  "both"`, so new CLI capabilities (`doctor --target`, `update --size`) MUST be
  mirrored in `backlogit_doctor` / `backlogit_update_item` MCP tools and the
  backlog registry op mapping.

## Options Evaluated

### Decision A — Body-preserving `--size` write

**Option A1 — Route the `--size` mutation through `internal/mdfront`
(Decode → set `size` → Encode via `internal/atomicfile`).** Preserves the rest of
the frontmatter and the entire body byte-for-byte; provable with a golden test;
reuses the 068-S pattern doctor repair already uses.

* Pros: satisfies the explicit "without destroying frontmatter/AST" requirement;
  proven pattern; testable as byte-equality; atomic write.
* Cons: introduces a second write path in `update` (must be scoped narrowly to
  the size mutation to avoid divergence with `WriteArtifactFile`).

**Option A2 — Keep the existing `WriteArtifactFile` model-rebuild path.**

* Pros: no new write path; least code.
* Cons: rebuilds the whole file from the model, risking frontmatter key
  reordering, comment loss, and body re-serialization — the exact failure item 4
  forbids. **Rejected.**

### Decision B — Doctor `--target` validation source

**Option B1 — Reuse `LoadHeaderDef` + `ValidateArtifactFields` on the decoded
single file.** No new validation semantics; one code path for field validation.

* Pros: zero duplication; consistent with `update`'s validation; small surface.
* Cons: must decode a single arbitrary file (use `mdfront.Decode`) and confine
  scope to `.backlogit/` (path guard).

**Option B2 — Write a bespoke single-file validator.**

* Pros: fully decoupled from artifact-field validation.
* Cons: duplicates schema logic; drift risk. **Rejected.**

### Decision C — Per-task lock mechanism

**Option C1 — Advisory `.lock` sidecar + `sync.Mutex` + stale TTL**, modeled on
`stash_lock.go` and the advisory-file-lock learning.

* Pros: crash-safe (TTL recovery); Windows-safe; matches existing convention and
  `concurrency.instructions.md` (`.<name>.lock`, ephemeral, gitignored).
* Cons: advisory only (does not stop an external editor that ignores the
  convention) — acceptable; the consumer set is backlogit + autoharness, both
  cooperative.

**Option C2 — OS-level `flock`/`LockFileEx`.**

* Pros: mandatory locking.
* Cons: platform-divergent; Windows semantics differ; heavier; PID-liveness
  unreliable on Windows per prior learning. **Rejected.**

## Trade-off Comparison

| Criterion | A1 mdfront write | A2 rebuild | B1 reuse validation | C1 advisory sidecar | C2 flock |
|---|---|---|---|---|---|
| Meets explicit requirement | ✅ | ❌ | ✅ | ✅ | ✅ |
| Reuses proven repo pattern | ✅ | partial | ✅ | ✅ | ❌ |
| Cross-platform (Windows) | ✅ | ✅ | ✅ | ✅ | ❌ |
| Duplication risk | low | n/a | none | none | n/a |
| Blast radius | low (scoped) | high (whole file) | low | low | medium |

## Decision

* **A1** — body-preserving `--size` write via `internal/mdfront` +
  `internal/atomicfile`, proven with a differential golden byte-equality test.
* **B1** — doctor `--target` reuses `LoadHeaderDef` + `ValidateArtifactFields`
  with a `.backlogit/`-scope path guard and a 5s `context.WithTimeout`.
* **C1** — per-task advisory `.lock` sidecar with `sync.Mutex` + stale TTL,
  reusing the `stash_lock.go` pattern; the lock is acquired around the doctor
  `--target` validation read and around the `--size` mutation write.
* **CLI/MCP parity is mandatory** for both new capabilities (documented
  convention). New CLI flags are mirrored in the MCP tool handlers and the
  backlog registry op mapping.
* **Backbone preserved**: the four numbered work items remain the plan backbone;
  Phase 1 = items 1 & 2, Phase 2 = items 3 & 4.

### Backlogit-relevant open questions — resolutions (NOT dropped)

* **Q1 — Infinite correction loops** (design §6.1): "when an agent fails a
  validation gate 3+ times, should backlogit force the task to `blocked` and
  return it to the queue?" **Resolution:** backlogit **already owns and exposes**
  the required state transitions — `backlogit move {id} --status blocked` and
  `backlogit move {id} --status queued` (`.backlogit/header-def.yaml` status enum
  includes `blocked`, `queued`). No new backlogit state-machine code is required.
  The **policy** (when to trigger after N failures, or escalate to a heavier
  model instead) is **autoharness's** responsibility, consistent with the §2
  separation of concerns. Action: **document** the existing transition as the
  supported gate-failure escape hatch in the plan; **do not** build a new
  command. (Adding a bespoke `--force-blocked-after-N` command was considered and
  rejected as YAGNI / wrong ownership.)
* **Q3 — Telemetry DB location** (design §6.3): autoharness recommends local
  `.autoharness/metrics/execution_epochs.db` for Phase 1. **Resolution:** no
  backlogit storage change. Existing `telemetry_sessions` / `telemetry_tool_usage`
  tables are untouched. Confirmed: **no task** in this slice.

## Rejected Alternatives

* A2 (model-rebuild write) — corrupts frontmatter/AST; violates the requirement.
* B2 (bespoke validator) — duplicates schema logic.
* C2 (OS flock) — platform-divergent, Windows-unreliable.
* New Q1 state-machine command — wrong ownership boundary; existing `move`
  transitions suffice.

## Unresolved Questions

* Exact allowed T-shirt value set for `size`. Proposed: `XS, S, M, L, XL`
  (enum in `header-def.yaml`). To be confirmed during planning; autoharness only
  needs a stable, validated vocabulary.
* Whether `doctor --target` should also emit machine-readable JSON for the gate
  (design shows exit-code-driven gating; JSON is a nice-to-have, deferred unless
  the CLI already supports `--format json` — it does; reuse it).

## Risks and Mitigations

* **Queue-file corruption** on `--size` write → mitigate with mdfront body-preserving
  codec + atomic write + golden byte-equality test (A1).
* **Cross-repo CLI contract drift** (autoharness depends on flag names / exit
  codes / 5s timeout) → mitigate by treating flag name, exit-code semantics, and
  timeout as a documented contract in the plan and command help text.
* **Stale lock after crash** → mitigate with stale-TTL recovery (C1).
* **CLI/MCP drift** → mitigate with explicit MCP-parity tasks (documented P2 class).
* **Scope creep** into autoharness/engram territory → guarded by the explicit
  out-of-scope list above.

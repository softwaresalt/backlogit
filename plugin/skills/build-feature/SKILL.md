---
name: build-feature
description: "Usage: Build feature {task-id} with harness {harness-cmd}. Implements a requested feature by continuously looping a fast worker agent against a strict, passing-import but failing test harness until success is achieved."
version: 2.0
maturity: stable
input:
  properties:
    task-id:
      type: string
      description: "The unique backlogit task or subtask ID."
    harness-cmd:
      type: string
      description: "The go test command defining the strict test harness boundary."
  required:
    - task-id
    - harness-cmd
---

# Build Feature Skill

Implements a requested feature by continuously looping a fast worker agent against a strict, importable but failing test harness until success is achieved. The harness defines the contract; go test is the critic.

## Subagent Execution Constraint (NON-NEGOTIABLE)

This skill is a leaf executor. It MUST NOT spawn additional subagents via runSubagent, Task, or any other agent-spawning mechanism. Perform all work using direct tool calls (read, edit, search, terminal, MCP tools) and return results to the parent agent (build orchestrator). If you encounter work that seems to require a subagent, report it as a finding and let the parent decide.

## Agent-Intercom Communication (NON-NEGOTIABLE)

If agent-intercom is available (determined by the parent agent's intercom state), broadcast at every step. Broadcasting is not optional.

## Stall Detection

Every terminal command gets a watchdog timeout:

| Operation | Timeout | Action |
|---|---|---|
| go test / golangci-lint / go vet | 10 minutes | Kill process, broadcast stall error, clean up |
| Non-Go terminal commands | 5 minutes | Kill, broadcast, proceed with error handling |

If a command exceeds its timeout, broadcast `[STALL] {command} exceeded {timeout}`, kill the process, clean up any stale build cache artifacts, and count toward the parent orchestrator's stall limit.

## Prerequisites

* The test harness defined by `${input:harness-cmd}` imports without error (green imports, red tests)
* The structural stubs in `internal/` exist with `panic("not implemented")` markers
* The project imports cleanly before starting (`go build ./cmd/backlogit` passes)
* All test files follow go test discovery conventions (`*_test.go` in the same package or `_test` suffix package)

## Shell Session Hygiene

Before starting any test run, verify no previous go test processes are still running from prior iterations. On Windows: `Get-Process -Name go -ErrorAction SilentlyContinue`. Stale processes can hold file locks and cause silent hangs. Stop them before proceeding.

## Remote Operator Integration (agent-intercom)
When the agent-intercom MCP server is reachable, status updates and file modifications route through it so the remote operator can follow progress via Slack.
### Availability Detection
At the start of execution, call `ping` with a brief status message. If the call succeeds, agent-intercom is active and you must follow all remote workflow rules below, then verify messaging with the first `broadcast` before reading files or running the harness. If it fails or times out, print a prominent CLI warning that agent-intercom is unavailable and Slack status updates will not be delivered for this task, then fall back to local-only operation. Silent fallback is forbidden.
### Status Broadcasting
Use `broadcast` (non-blocking) throughout execution to keep the operator informed.
| When | Tool | Level | Message Pattern |
|---|---|---|---|
| Skill start | `broadcast` | `info` | `[BUILD] Starting task {task-id}: {harness-cmd}` |
| Each iteration start | `broadcast` | `info` | `[LOOP] Attempt {N}/5 — running harness` |
| File created | `broadcast` | `info` | `[FILE] created: {file_path}` — include full content in body |
| File modified | `broadcast` | `info` | `[FILE] modified: {file_path}` — include unified diff in body |
| Harness passes | `broadcast` | `success` | `[BUILD] Harness passed on attempt {N}` |
| Harness fails | `broadcast` | `warning` | `[LOOP] Attempt {N} failed — {error_summary}` |
| Circuit breaker hit | `broadcast` | `error` | `[BUILD] Circuit breaker — 5 attempts exhausted, task blocked` |
| Workspace test pass | `broadcast` | `success` | `[BUILD] Workspace tests pass — task {task-id} complete` |
| Instruction re-read | `broadcast` | `info` | `[REINFORCE] Coding standards refreshed for attempt {N}/5` |
| Task complete | `broadcast` | `success` | `[BUILD] Task {task-id} complete — commit {short_hash} — {N} attempt(s)` |
Post the first `broadcast` as a new top-level message and capture the returned `ts`. Use that `ts` as `thread_ts` for all subsequent messages. That first `broadcast` is an intercom verification gate and must happen before reading files, editing code, or running the harness. If it fails after a successful `ping`, print a prominent CLI warning, mark agent-intercom unavailable for the remainder of the task, and continue in local-only mode instead of assuming the operator received the update.
### File Change Workflow
File creation and modification proceed with direct writes. After each file write, call `broadcast` at `info` level with the change details.

**Protected file awareness**: When modifying core harness configuration files (`.github/agents/*.agent.md`, `.github/skills/*/SKILL.md`, `.github/instructions/*.instructions.md`, `AGENTS.md`), `broadcast` at `info` level: `[PROTECTED] Modifying harness config: {file_path}`. This alerts the operator without blocking modification.

For **destructive operations** (file deletion, directory removal), route through the approval workflow:
1. `auto_check` — Check if workspace policy allows the operation.
2. `check_clearance` — Submit proposal and block until operator responds.
3. `check_diff` — Execute only after `status: "approved"`.
## Execution Steps

### Step 1: Context Isolation

1. Read the test file targeted by the `${input:harness-cmd}`. Carefully read the embedded `# GIVEN`, `# WHEN`, `# THEN` BDD comments to fully internalize the human intent behind the test.
2. Use grep/glob tools to understand the codebase context before reading raw files:
   * Search for each domain class and function found in the test to locate the source files in `src/` containing the `panic("not implemented")` stubs that require attention.
   * Search for the feature's key concepts to find related code and prior decisions that inform the implementation.
   * Use glob to discover available modules in specific packages.
3. Read `.github/copilot-instructions.md` and `.github/agents/subagents/go-engineer.agent.md` (if it exists) for project coding standards and Python-specific conventions.
4. `broadcast` at `info` level: `[BUILD] Starting task {task-id}: {harness-cmd}` with a summary of the test scenarios and stub files.

### Step 2: Mechanical Feedback Loop (Actor-Critic)

The harness loop inherently satisfies the **atomic milestone validation** requirement: every task exits with either a passing test (verifiable state) or a circuit breaker (blocked state). No task completes without a concrete verification artifact.

Execute the following loop with a **hard limit of 5 attempts**:
1. **Run** the targeted `${input:harness-cmd}`.
2. **If it passes** (exit code 0): proceed to Step 3.
3. **If it fails** (exit code != 0):
   a. Capture the raw output (import errors, type errors, or assertion failures).
   b. `broadcast` the failure summary at `warning` level.
   c. **Instruction reinforcement**: Read `.github/agents/subagents/go-engineer.agent.md` (if it exists) or `.github/copilot-instructions.md` coding standards section to refresh project conventions before implementing the fix. `broadcast` at `info` level: `[REINFORCE] Coding standards refreshed for attempt {N}/5`.
   d. Analyze the error output and implement the fix:
      * **Import errors**: Fix missing packages, incorrect imports, circular imports in the `internal/` stubs.
      * **Panic (not implemented)**: Implement the underlying logic inside the `internal/` stubs to make the harness pass. Replace the `panic("not implemented")` markers with real logic.
      * **Test assertion failures**: Fix the implementation logic (not the test itself, unless the test setup has an import error).
   d. Apply all project coding standards:
      * All exported functions have GoDoc comments.
      * Use backlogit error hierarchy (`internal/errors/`), not bare `error` values.
      * Follow Effective Go naming and GoDoc conventions.
      * Run `go build ./cmd/backlogit` after each fix to verify the module imports cleanly before re-running the harness.
   e. After each file write, `broadcast` the change at `info` level with the unified diff.
   f. **Do not modify the test file itself** unless fixing an import error in the test setup.
   g. Return to step 1 of this loop.

4. **Circuit breaker**: If 5 attempts are exhausted without the harness passing:
   * `broadcast` at `error` level: `[BUILD] Circuit breaker — 5 attempts exhausted, task blocked`.
   * Call `backlogit_move_item` with `id: ${input:task-id}` and `status: "blocked"`.
   * Call `backlogit_append_comment` with `item_id: ${input:task-id}`, `actor: "build-feature"`, and a note indicating the task is blocked pending human review.
   * Halt execution. Do not retry automatically.
### Step 3: Verification & State Update

Once the isolated harness passes:
1. **Workspace verification — tiered strategy**: Do NOT run the full test suite (`go test ./...`) after every harness pass in the feedback loop. Use this order:
   a. Run `go test {harness_test_path} -v` — confirms the harness still passes after any cleanup changes.
   b. Run `go test ./internal/... -v` — fast check for internal package regressions.
   c. Run `go test ./...` (full suite) exactly once before committing.
   * If new failures appear in the full suite, diagnose and fix them before committing.
   * `broadcast` at `success` level: `[BUILD] Workspace tests pass — task {task-id} complete`.
2. **Lint verification**: Run `golangci-lint run` and `gofmt -l .`. Run `go vet ./...`. Fix any violations.
3. **Commit**: Stage and commit validated changes for the current work item only:
   * Stage only the files required for `${input:task-id}`. Use explicit paths when unrelated local changes exist. Use `git add -A` only when the full working tree belongs to this same work item.
   * `git commit -m "feat: implement passing harness for ${input:task-id}"`
   * Capture commit metadata with:
     * `git rev-parse HEAD` for the full hash
     * `git rev-parse --short HEAD` for the short hash
     * `git log -1 --pretty=%s HEAD` for the commit subject
     * `git log -1 --pretty=%an HEAD` for the commit author
   * Do not batch unrelated backlogit items into this implementation commit. If the working tree contains changes for sibling, parent, or future items that are not directly completed by this work, split them into a different commit before proceeding.
   * `broadcast` at `success` level: `[BUILD] Task {task-id} complete, commit {short_hash}, {N} attempt(s)`. The attempt count is used by the invoking workflow, typically Ship, to decide whether to invoke compound knowledge capture.
4. **State update**: Record commit traceability only for the items directly affected by this commit:
   * Start `affected_item_ids` with `${input:task-id}`.
   * Add another task, subtask, or review item only when this exact commit directly implements or finalizes that item. Never attach the commit to the full feature or to untouched descendants.
   * For each item in `affected_item_ids`, call `backlogit_track_commit` with `item_id`, `sha: {full_commit_hash}`, `message: {commit_subject}`, and `author: {commit_author}` so the event log captures the precise work-to-commit link.
   * Call `backlogit_move_item` with `id: ${input:task-id}` and `status: "done"`.
   * Move any additional affected item to `done` only when its acceptance criteria and Definition of Done are fully satisfied by this commit. Otherwise leave its status unchanged and only record the commit link.

## Troubleshooting

### Compilation errors after adding new packages

Verify that all new packages have correct `package` declarations matching the directory name and that imports use the full module path (`github.com/softwaresalt/backlogit/internal/{package}`). Check that `go.mod` has the correct module declaration.

### Go vet reports suspicious constructs

Review `go vet ./...` output for common issues: unused variables, unreachable code, mismatched struct tags. Go vet findings must be fixed before committing.

### SQLite locking in tests

SQLite allows only one writer at a time. If parallel tests share a database file, use `t.TempDir()` to create isolated databases per test. Avoid `go test -parallel` for database-touching tests unless each test uses its own database file.

### Tests pass locally but fail in CI

Verify the Go version in `go.mod` matches the CI matrix in `.github/workflows/ci.yml`. Check that all test dependencies are properly declared in `go.mod` and resolved via `go mod tidy`.

### Circuit breaker triggered (5 failed attempts)

When the 5-attempt hard limit is reached, the task is marked as blocked in backlogit. Review the error output from each attempt to identify the root cause. Common issues include incorrect function signatures in stubs, missing interface implementations, or test assumptions that conflict with the codebase architecture.
---

Proceed by reading the harness test file and isolating context for the given task.




---
name: fix-ci
description: "Usage: Fix CI. Detects CI pipeline failures and GitHub Copilot review comments on the current branch's PR, reproduces and fixes errors locally, addresses review comments, runs all CI gates, then pushes and polls until the pipeline passes and comments are resolved."
version: 2.0
input:
  properties:
    pr-number:
      type: integer
      description: "PR number to fix. Auto-detected from current branch if omitted."
    owner:
      type: string
      description: "Repository owner. Auto-detected from git remote if omitted."
    repo:
      type: string
      description: "Repository name. Auto-detected from git remote if omitted."
    max-iterations:
      type: integer
      description: "Maximum fix-push-poll cycles before halting (default: 5)."
    poll-interval:
      type: integer
      description: "Seconds between CI status polls (default: 30)."
    max-wait:
      type: integer
      description: "Maximum seconds to wait for CI checks per cycle (default: 600)."
  required: []
---

# Fix CI Skill

Automates the cycle of detecting CI pipeline failures AND GitHub Copilot review comments on a PR, reproducing errors locally, applying fixes, addressing review comments, running all CI gates, then pushing and polling until the remote pipeline passes and all review comments are resolved.

## Agent-Intercom Communication (NON-NEGOTIABLE)

Call `ping` at session start. If agent-intercom is reachable, broadcast at every step. If unreachable, warn the user that operator visibility is degraded.

| Event | Level | Message prefix |
|---|---|---|
| Session start | info | `[FIX-CI] Starting for PR #{pr_number}` |
| CI status checked | info | `[FIX-CI] CI status: {passed}/{failed}/{pending} checks` |
| Copilot comments found | info | `[FIX-CI] Found {count} Copilot review comments` |
| Reproducing failure | info | `[FIX-CI] Reproducing: {check_name}` |
| Fix applied | info | `[FIX-CI] Fixed: {description}` |
| Comment addressed | info | `[FIX-CI] Addressed comment on {file}: {summary}` |
| Comment declined | info | `[FIX-CI] Declined comment on {file}: {reason}` |
| Local gate passed | success | `[FIX-CI] Local CI gates pass` |
| Push and poll | info | `[FIX-CI] Pushed, polling cycle {N}/{max}` |
| All checks pass | success | `[FIX-CI] All CI checks pass, all comments resolved` |
| Circuit breaker | error | `[FIX-CI] Max iterations ({max}) reached — halting` |
| Stall detected | error | `[STALL] {operation} exceeded timeout` |
| Waiting for input | warning | `[WAIT] Blocked on: {what}` |

## Subagent Execution Constraint (NON-NEGOTIABLE)

When invoked as a subagent, you MUST NOT spawn additional subagents. You are a leaf executor.

## Loop Limits (NON-NEGOTIABLE)

| Counter | Limit | Action |
|---|---|---|
| Fix-push-poll cycles | 5 (configurable via max-iterations) | Halt, broadcast error, leave PR open for manual intervention |
| Stalls in session | 3 | Halt, broadcast error, exit |

## Prerequisites

* The workspace is a Git repository with a remote configured on GitHub.
* The current branch has an open pull request (or `pr-number` is provided).
* The project imports cleanly before starting (`python -c "import backlogit"` passes).
* GitHub MCP tools are available (`mcp_github_pull_request_read`).
* The `.github/copilot-instructions.md` coding standards are accessible.

## Quick Start

Invoke the skill from the current branch:

```text
Fix CI
```

To target a specific PR:

```text
Fix CI pr-number 42
```

The skill runs autonomously through all required steps, halting only when CI passes or the maximum iteration count is reached.

## Parameters Reference

| Parameter        | Required | Type    | Default | Description                                                   |
| ---------------- | -------- | ------- | ------- | ------------------------------------------------------------- |
| `pr-number`      | No       | integer | —       | PR number to fix. Auto-detected from current branch if omitted |
| `owner`          | No       | string  | —       | Repository owner. Auto-detected from git remote if omitted     |
| `repo`           | No       | string  | —       | Repository name. Auto-detected from git remote if omitted      |
| `max-iterations` | No       | integer | 3       | Maximum fix-push-poll cycles before halting                     |
| `poll-interval`  | No       | integer | 30      | Seconds between CI status polls                                |
| `max-wait`       | No       | integer | 600     | Maximum seconds to wait for CI checks per cycle                |

## Required Steps

### Step 1: Identify Target PR

Determine the current branch and locate the associated pull request.

1. Run `git branch --show-current` to get the active branch name.
2. Run `git remote get-url origin` to extract the repository owner and name from the remote URL. Parse the `owner/repo` segment from the URL (handles both HTTPS and SSH formats). Use these values when `owner` and `repo` are not provided as input.
3. If `pr-number` is provided, use it directly. Otherwise, search for an open PR matching the current branch using `mcp_github_search_pull_requests` with a query filtering by head branch and repository.
4. Use `mcp_github_pull_request_read` with method `get` to retrieve the PR details, including the head branch name and head SHA.
5. Report the PR number, branch, and head SHA before proceeding.

### Step 2: Check CI Status

Poll the PR's check run statuses to determine which checks need attention.

1. Use `mcp_github_pull_request_read` with method `get_status` to retrieve all check run statuses for the PR.
2. If all checks have passed, report success and stop — no fixes are needed.
3. If any checks are still *pending*, wait for the configured `poll-interval` (default 30 seconds) and re-poll. Continue polling until all checks have completed or `max-wait` is exceeded.
4. If `max-wait` is exceeded with checks still pending, report the pending checks and halt.
5. When checks have completed, identify which specific checks failed. The CI pipeline runs these checks in order: *ruff check*, *ruff format*, *mypy*, *pytest*.
6. Also use `mcp_github_pull_request_read` with method `get_comments` to check for CI bot failure summaries that may provide additional diagnostic context.
7. Report the list of failed checks before proceeding to local reproduction.

### Step 2b: Check for Copilot Review Comments

Detect and classify GitHub Copilot review comments on the PR.

1. Use `mcp_github_pull_request_read` with method `get_reviews` to retrieve all reviews on the PR.
2. Filter for Copilot-authored review comments (author is `github-actions[bot]` or `copilot` or similar automated reviewer).
3. For each Copilot comment, extract:
   - File path and line number
   - Comment text and suggestion
   - Whether the comment thread is resolved or unresolved
4. Filter to only unresolved Copilot comments.
5. If no unresolved Copilot comments and no CI failures, report success and stop.
6. `broadcast` at `info` level: `[FIX-CI] Found {count} unresolved Copilot review comments`
7. Report the list of comments before proceeding.

### Step 3: Reproduce Failures Locally

Run the failing CI checks locally in CI pipeline order to capture detailed error output.

* Run each check as a separate terminal command (one command per invocation — project rule).
* If a command produces output that may exceed the terminal buffer, redirect to a file using `Out-File` (e.g., `python -m pytest 2>&1 | Out-File .pytest-output.txt`).
* Use workspace-relative paths for any output files (e.g., `.pytest-output.txt`).

The CI checks in pipeline order:

1. `ruff check src/ tests/` — lint verification.
2. `ruff format --check src/ tests/` — format verification.
3. `mypy src/` — type checking.
4. `python -m pytest --cov=backlogit --cov-report=xml` — test execution.

Run only the checks that failed remotely (and any earlier checks in the pipeline that gate them). Capture and parse the error output from each failing command to identify specific errors, file locations, and error codes.

### Step 4: Diagnose and Fix

Analyze each failing check's error output, apply targeted fixes, and verify the fix resolves the specific failure.

For each failing check, working in CI pipeline order:

1. Parse the error output to identify root causes — specific file paths, line numbers, error codes, and error messages.
2. Use grep/glob tools to understand the affected code before reading raw files:
   * Search for each failing symbol to understand its usage, callers, and dependencies.
   * Search with error-related concepts to find related code and context.
   * Fall back to broader file reads only when search results are insufficient.
3. Read the affected source files to understand the context around each error.
4. Apply the minimal fix that resolves the error while following the project's coding standards from `.github/copilot-instructions.md`.
5. After applying fixes for a specific check, re-run that check locally to verify the fix.
6. If the re-run reveals new failures introduced by the fix, diagnose and fix those as well.
7. Continue iterating on the specific check until it passes locally.
8. Move to the next failing check in pipeline order and repeat.

Common fix patterns by check type:

* *ruff check*: Address each lint violation individually — common issues include unused imports, missing type annotations, line length, and naming conventions. Auto-fix with `ruff check --fix src/ tests/` where safe.
* *ruff format*: Run `ruff format src/ tests/` to auto-fix formatting, then verify with `ruff format --check src/ tests/`.
* *mypy*: Address type errors individually — common issues include missing type annotations, incompatible types, and missing return types. Fix the source code types rather than adding `# type: ignore`.
* *pytest*: Investigate test assertion failures, import errors in test code, and missing test fixtures. Fix the implementation rather than weakening the test, unless the test itself contains a bug.

### Step 4b: Address Copilot Review Comments

For each unresolved Copilot review comment (from Step 2b):

1. Read the comment and the referenced code at the specified file and line.
2. Use grep/glob tools to understand the context:
   - Search for the affected symbol's usage across the codebase
   - Search for the module structure around the comment
3. Evaluate whether the suggestion is valid:
   - **Valid suggestion**: Apply the fix. `broadcast` at `info` level: `[FIX-CI] Addressed comment on {file}: {summary}`. Resolve the comment thread if the GitHub API supports it.
   - **Partially valid**: Apply the applicable portion, leave a reply explaining what was and was not applied.
   - **Invalid or disagreeable**: Do NOT apply. Leave a reply comment explaining why the suggestion was declined with technical rationale. `broadcast` at `info` level: `[FIX-CI] Declined comment on {file}: {reason}`
4. After addressing all comments, re-run local CI gates to verify no regressions were introduced.

### Step 5: Local CI Gate

This is a **hard gate**. All checks pass locally before proceeding to push. Run all CI checks in pipeline order regardless of which ones originally failed, since fixes may have introduced regressions in previously passing checks.

1. Run `ruff check src/ tests/`. If violations are found, run `ruff check --fix src/ tests/` to auto-fix, then re-run the check to confirm it passes.
2. Run `ruff format --check src/ tests/`. If violations are found, run `ruff format src/ tests/` to auto-fix, then re-run the check to confirm it passes.
3. Run `mypy src/`. Fix any type errors, then re-run until the command exits cleanly.
4. Run `python -m pytest --cov=backlogit`. Fix any failures, then re-run until all tests pass.
5. If fixes applied in steps 2–4 cause an earlier check to fail, restart from step 1 and repeat the full cycle.
6. All four checks exit 0 before proceeding.
7. Report results: ruff check exit code, ruff format exit code, mypy exit code, test counts and pass rate.

### Step 6: Stage, Commit, and Push

Stage all changes, compose a descriptive commit message, and push to the remote.

1. Run `git add -A` to stage all modified, created, and deleted files.
2. Compose a commit message following *Conventional Commits* format:
   * Subject: `fix(ci): resolve {check-names} failures`
   * Body: itemized list of fixes applied with brief descriptions of each change
   * Footer: `Refs: #{pr-number}`
3. Run `git commit` with the composed message.
4. Run `git push` to push the commit to the remote branch.
5. Report the commit hash before proceeding to remote polling.

### Step 7: Poll Remote CI

After pushing, poll the PR's check statuses until all checks complete, then decide whether to iterate or finish.

1. Wait for `poll-interval` seconds (default 30) after pushing to allow CI to start.
2. Use `mcp_github_pull_request_read` with method `get_status` to retrieve updated check run statuses.
3. If checks are still *pending*, wait for `poll-interval` seconds and re-poll. Continue until all checks complete or `max-wait` is exceeded.
4. If all checks pass, re-check for new Copilot review comments (Step 2b). If no new unresolved comments, proceed to Step 8 with a success status. If new comments appeared, address them (Step 4b), re-run local gates, push, and re-poll.
5. If any checks fail, increment the iteration counter.
   * If the counter is below `max-iterations` (default 5), loop back to Step 3 to reproduce the new failures locally and begin another fix cycle.
   * If the counter has reached `max-iterations`, proceed to Step 8 with a failure status and the accumulated findings. `broadcast` at `error` level: `[FIX-CI] Max iterations ({max}) reached — halting`

### Step 8: Completion Report

Summarize the outcome of the fix cycle.

Report the following:

* **Final status**: all checks passed and all Copilot comments resolved, or maximum iterations reached with remaining issues.
* **Iterations completed**: how many fix-push-poll cycles ran.
* **Commits made**: list of commit hashes produced during the fix cycle.
* **Fixes applied**: summary of all changes made across all iterations.
* **Copilot comments**: {addressed_count} addressed, {declined_count} declined with rationale.
* If checks are still failing after reaching `max-iterations`:
  * The specific checks that remain broken.
  * The latest error output from the failing checks.
  * Recommendations for manual review, including affected files and error patterns.

## Troubleshooting

### PR not found for current branch

Verify the current branch has been pushed to the remote and an open PR exists. If the branch was recently pushed, the PR may need to be created first. Provide `pr-number` explicitly to bypass auto-detection.

### CI checks stay pending beyond max-wait

The CI runner may be queued or slow. Increase `max-wait` and re-invoke the skill. Alternatively, check the GitHub Actions tab for the repository to see if runners are available.

### Fixes pass locally but fail in CI

Verify the local Python version matches CI. The CI pipeline uses `actions/setup-python` with a version matrix — run `python --version` locally to confirm. Check that `pyproject.toml` `requires-python` matches the CI configuration in `.github/workflows/ci.yml`.

### max-iterations reached without resolution

Some failures may require architectural changes or upstream dependency fixes that fall outside the scope of automated repair. Review the accumulated error output in the completion report and consider manual intervention.

### Terminal output truncated

When a CI check produces extensive output, redirect to a file:

```text
python -m pytest 2>&1 | Out-File .pytest-output.txt
```

Then read the output file to review the full error details.

---

Proceed with the user's request following the Required Steps.

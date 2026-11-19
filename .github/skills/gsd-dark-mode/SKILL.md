---
name: gsd-dark-mode
description: "Codify bounded P-017 Stage & Ship dark-factory execution across shipment sequences or sets with pre-PR multi-persona + rubber-duck review, carry-forward handling, patient Copilot review polling, and GraphQL thread resolution"
argument-hint: "[shipments=[215-S -> 218-S] | [215-S, 178-S, 125-S]] [carry_forward=paths]"
---

# GSD Dark Mode

Execute bounded, autonomous Stage -> Ship dark-factory runs (`P-017`) across
an operator-designated sequence or set of shipments. This skill codifies the
end-to-end dark-factory contract so autonomous runs remain deterministic,
adversarially reviewed before PR creation, patient across multi-round Copilot
reviews, and fail-closed on unsafe or unauthorized actions.

This skill runs through the Orchestrator (`DARK_MODE_ACTIVE`) and never
bypasses Stage/Ship role boundaries (`P-010`), single-active shipment limits
(`P-001`), merge-commit rules (`P-009`), local review readiness (`P-014`),
Copilot completion gates (`P-018`), context compaction (`P-020`), or scope
isolation (`P-021`).

## When to Use

Invoke `/gsd-dark-mode` when the operator requests autonomous dark-factory
execution over bounded backlog scope, such as:

* Running an ordered shipment chain (`[215-S -> 218-S]`, `215-S -> 178-S`) or
  an explicit shipment set (`[215-S, 178-S, 125-S]`) while the operator is AFK
* Continuing autonomously until all scoped work completes or hits an unsafe
  stop condition
* Carrying forward explicit uncommitted backlog/config/reference artifacts
  onto the next feature branch under `P-011`

## Inputs

* `shipments`: (Required) Bounded shipment target(s), supplied either as an
  ordered sequence (`[215-S -> 218-S]`, `215-S -> 178-S`, `215-S → 178-S`) or
  an explicit set (`[215-S, 178-S, 125-S]`), or explicit stash/feature/task IDs
  when Stage intake is required first.
* `carry_forward`: (Optional) Explicit operator-designated paths (for example
  `.backlogit/stash.jsonl`, `.gsd/config.yaml`, legacy
  `.autoharness/config.yaml`, or `references/*` gitlinks) to carry into the
  next feature branch.

## Output

Autonomous execution across the bounded `DARK_MODE_SCOPE` emitting structured
`P-017` visibility events (`DARK_MODE_ACTIVE`, `DARK_MODE_START`,
`LOCAL_REVIEW_READY`, `DARK_MODE_MERGE_AUTHORIZED`, `DARK_MODE_COMPLETE`, or
`DARK_MODE_HALTED`), PRs merged with `--merge`, and durable `docs/closure/`
and `docs/memory/` artifacts.

## Workflow

### Step 0: `shipments` Argument Parsing, Sequence Ordering & Scope-Head Activation Preflight

1. **Parse Sequence or Set**:
   * **Ordered sequence** (`[215-S -> 218-S]`, `215-S -> 178-S`, `215-S → 178-S`):
     preserve the exact ordered cursor and verify that each successor's
     explicit `blocks` DAG edges match or are compatible with the requested
     order.
   * **Explicit set** (`[215-S, 178-S, 125-S]`): resolve a deterministic
     dependency-valid execution order honoring the provided order as the primary
     preference wherever DAG edges permit.
   * Reject unbounded wildcards ("all backlog", "everything", or empty scope)
     and halt without activating dark mode.
2. **Scope-Head Activation Preflight**:
   * Identify the scope head — the first in-scope shipment in dependency/sequence
     order that is not yet shipped (`queued` or already
     `active` on resume).
   * If the scope head is already `active` (resuming an in-flight run),
     do **not** run `--phase pre_claim` (which deterministically returns
     `PRECLAIM_ACTIVE_SHIPMENT_PRESENT` when any shipment is active); record
     `head: active (resume)` in `preflight` and let Ship's lifecycle gates
     govern the in-flight shipment.
   * When the scope head is `queued`, run the read-only preflight probe:
     `gsd gate pipeline-topology --mode agent --shipment {head_id} --phase pre_claim --json`
   * If the `queued` scope head is blocked (for example `HOLD`,
     `UNSEQUENCED_SHIPMENT`, or an unfinished external predecessor), halt
     immediately with `DARK_MODE_PREFLIGHT_BLOCKED` and record no activation.
   * Report downstream in-scope shipments as `PENDING` (blocked only on earlier
     in-scope shipments) or `EXTERNAL_BLOCKER` (blocked outside `DARK_MODE_SCOPE`).
3. **Record Activation**:
   * Record `DARK_MODE_ACTIVE`, `DARK_MODE_START`, and `DARK_MODE_SCOPE` (plus
     `merge_approval_pre_authorized` and `admin_fallback_pre_authorized`,
     defaulting `admin_fallback_pre_authorized` to `false` unless explicitly
     granted).

### Step 1: AFK Operator Autonomy, Sound Engineering Judgment & Stop Conditions (`P-017`, `P-021`)

1. **AFK Autonomy Posture**:
   * Operate on the assumption that the operator is AFK and unavailable for
     routine coordination. Continue autonomously across Stage, Ship, review
     remediation, PR polling, and non-destructive closure steps until the entire
     bounded `DARK_MODE_SCOPE` completes or an unsafe stop condition triggers
     `DARK_MODE_HALTED`.
   * Apply sound, well-reasoned engineering judgment when selecting safe
     implementation details, test isolation, and remediation paths within
     approved scope.
2. **`P-021` Scope Non-Bypass**:
   * `DARK_MODE_ACTIVE` never waives `P-021`. `DARK_MODE_SCOPE` bounds which
     shipments execute and never expands an in-flight shipment's task scope.
   * Classify every review or CI finding with the `P-021 C1` same-contract-surface
     test. Out-of-scope findings MUST be captured as `DEFERRED SCOPE EXPANSION`
     entries via `P-021 C2` and never fixed inline; capture does not interrupt
     the dark run.
3. **Hard Stop Conditions (`DARK_MODE_HALTED`) & Destructive Approval Rule**:
   * Halt immediately and emit `DARK_MODE_HALTED` on: missing/degraded
     visibility gates, secrets or security-boundary risk, unresolved in-scope
     `P0`/`P1` findings, failed local review readiness, ambiguous branch
     protection or unknown merge blocks, or any `P-001` / `P-009` / `P-010` /
     `P-016` / `P-018` violation.
   * **Per-Closure Destructive Approval Rule**: `merge_approval_pre_authorized: true`
     never transfers to or satisfies per-closure destructive approval for
     mutating `gsd shipment cascade-close`. If `gsd shipment cascade-close --classify-only`
     selects `cascade` without explicit per-closure destructive authorization
     for that named shipment, persist a guarded `ship` checkpoint, record the
     evidence file, and return a structured `ProposedAction` (`ActionRisk: destructive`,
     `ActionResult: blocked`) instead of running the mutating cascade.

### Step 2: Mandatory Pre-PR Multi-Persona Adversarial Review + `rubber-duck`

Before opening or updating any pull request:

1. **Plan & Decision Review (Stage)**:
   * Deliberations (`docs/decisions/`) and plans (`docs/plans/`) MUST
     pass multi-persona adversarial review **including `rubber-duck`** before
     tasks are harvested and queued.
2. **Code, Template & Evidence Review (Ship)**:
   * Run full local build and quality gates (`go test -run=^$ -count=1 ./...`,
     `golangci-lint run`, `go test -timeout=30m ./...`, `gsd verify-workspace --workspace . --json`,
     `gsd gate pre-review --base main --json`,
     `gsd gate line-endings --json`, and markdown lint over touched Markdown
     files).
   * Invoke multi-persona adversarial review **including `rubber-duck`**
     (external/second-opinion adversarial review when available, degrading cleanly
     to same-roster adversarial review if unavailable) across the active review
     roster (`Correctness Reviewer`, `Security Reviewer`,
     `Concurrency Reviewer`, `Evidence Consistency Reviewer`,
     `Architecture Strategist`, `Maintainability Reviewer`,
     `Scope Boundary Auditor`, `Constitution Reviewer`,
     `Template Integrity Reviewer`, and `rubber-duck`) on the branch diff
     against `main`.
   * Resolve all in-scope `P0`/`P1` findings (`P0=0, P1=0`), complete the
     Pre-Readiness Documentation Consistency Audit, and emit
     `LOCAL_REVIEW_READY` before PR creation.

### Step 3: Carry-Forward Commit Handling (`P-011`)

1. **Backlog & Config Carry-Forward**:
   * Handle dirty `.backlogit/stash.jsonl`,
     `.gsd/config.yaml` (and legacy `.autoharness/config.yaml` migration aliases),
     and stash-referenced `docs/` files using the `P-011` carry-forward
     commit (`git --literal-pathspecs add -- <paths>` and
     `git --literal-pathspecs commit -F <message_file> -- <paths>`) on the
     shipment branch — never on `main`.
2. **Operator-Designated Foreign Paths / Submodule Gitlinks**:
   * Never list `.gsd/worktree-baseline.json` `ForeignPaths` (such as
     `references/*` submodule gitlinks) in task `## File Budget` lists, so
     per-task `pre_claim` and `leaf-diff` gates do not halt with
     `FOREIGN_FILE_OVERLAP`.
   * When the operator explicitly directs carrying designated gitlinks or
     companion updates into the next feature branch, commit them as an isolated
     post-task literal-pathspec commit after task execution completes and
     re-capture `.gsd/worktree-baseline.json` (`gsd gate pipeline-topology --mode agent --shipment {shipment_id} --phase lifecycle --capture-baseline --json`).

### Step 4: Pre-Authorized PR/MR Creation, Hosting-Aware Review Polling & Merge (`P-009`, `P-014`, `P-018`)

Execute PR/MR lifecycle steps according to the workspace's configured hosting
provider (`GitHub Actions` / remote origin); apply GitHub Copilot polling,
GraphQL thread resolution, §1.6.1 disposition markers, and `gh pr merge` only
when hosted on GitHub, and use the configured provider's `pr-lifecycle`
workflow on non-GitHub hosts (such as GitLab) while preserving local readiness
(`P-014`), explicit/pre-authorized merge approval, and merge-commit (`P-009`)
rules:

1. **Open PR/MR with Readiness Evidence**:
   * Push the shipment branch and open the PR/MR targeting `main`
     with complete `## Local Review Readiness` (`Reviewed HEAD` matching the
     branch `HEAD`, `P0=0`, `P1=0`, full local build evidence) and
     `## Documentation Consistency Audit` blocks.
   * When hosted on GitHub, request Copilot review per
     `.github/instructions/github-pr-automation.instructions.md` when applicable;
     on other hosting providers, follow the configured provider's review
     request flow via `pr-lifecycle`.
2. **Patient Multi-Round Hosted Review Polling (`P-018` on GitHub; Provider Lifecycle Otherwise)**:
   * **When hosted on GitHub**:
     * Read `copilot_review.enforcement` (`auto` | `required` | `disabled`,
       default `auto`) and `copilot_review.max_wait_seconds` (default `600`)
       from `.gsd/workspace-profile.yaml` and pass `--enforcement <mode>` and
       `--max-wait <seconds>` to `gsd gate copilot-review <pr> --repo softwaresalt/backlogit --enforcement <mode> --max-wait <seconds> --json`.
     * When `copilot_review.enforcement` is `disabled`, require explicit operator
       authorization and pass `--operator-reason "<reason>"` to both the polling
       and final `gsd gate copilot-review` invocations (or halt if no explicit
       operator reason was granted).
     * Paginate **all** Copilot `reviews` (preserving review-body dispositions
       across earlier pushes) and all `reviewThreads` across the PR per
       `.github/instructions/github-pr-automation.instructions.md` §1.9.1; apply
       `HEAD` freshness only to review completion.
     * Never proceed while Copilot review is pending, timed out, or has open
       threads or undispositioned review-body findings.
   * **When hosted on a non-GitHub provider**:
     * Wait for any required provider CI/review checks via the configured
       provider's `pr-lifecycle` adapter instead of invoking GitHub-only
       `gsd gate copilot-review` or `gh api graphql`.
3. **Push-Before-Reply & Thread Resolution**:
   * For each actionable in-scope review finding: fix, verify locally, commit,
     and **push to the remote branch first** so the fixing commit SHA exists on
     the remote before replying.
   * For out-of-scope findings: capture a `P-021 C2` `DEFERRED SCOPE EXPANSION`
     entry first and cite its returned entry ID in the thread reply.
   * **When hosted on GitHub**:
     * Reply to each addressed inline review thread via GraphQL
       `addPullRequestReviewThreadReply` using file-backed JSON variables.
     * Check the originating author before resolving: automatically resolve only
       addressed **bot-authored** threads (such as Copilot or linters) via
       GraphQL `resolveReviewThread`; never auto-resolve human-authored threads
       (leave human threads open for reviewer/operator disposition per
       `.github/instructions/github-pr-automation.instructions.md`).
     * For threadless review-body findings (`UNDISPOSITIONED_BODY_FINDINGS`),
       post a top-level PR issue comment carrying the exact
       `.github/instructions/github-pr-automation.instructions.md` §1.6.1 marker
       `Copilot-Review-Body-Disposition: <review databaseId>` detailing each
       finding's disposition, then re-request Copilot review if code changed.
   * **When hosted on a non-GitHub provider**:
     * Reply to and resolve review discussions through the configured provider's
       `pr-lifecycle` mechanism after pushing the fixing commit.
4. **Final Pre-Merge Re-Verification & Merge (`P-009`)**:
   * Confirm all required CI checks are green and verify either a scope-matching
     `DARK_MODE_ACTIVE` activation record with `merge_approval_pre_authorized: true`
     or separate explicit operator merge approval; otherwise halt with
     `DARK_MODE_HALTED` and await explicit operator approval.
   * Whenever commits (including carry-forward or review fixes) advance `HEAD`
     after the prior local review, re-run local review and applicable
     build/quality gates for that changed `HEAD`, update PR/MR body `Reviewed HEAD`
     to match the newly reviewed `HEAD`, and verify local readiness checks pass
     before authorizing merge.
   * **When hosted on GitHub**: verify all
     `.github/instructions/github-pr-automation.instructions.md` §1.9 readiness
     checks (Checks 1–5), re-run `gsd gate copilot-review` unconditionally with
     the configured `--enforcement <mode>` resolved from `.gsd/workspace-profile.yaml`
     (plus `--operator-reason "<reason>"` when `disabled`) and require
     `status: "PASS"` (`SATISFIED`, `NOT_APPLICABLE`, or explicitly
     operator-authorized audited `DISABLED` without claiming review completion,
     per `.github/instructions/github-pr-automation.instructions.md` §1.9.4 Check 5),
     emit `DARK_MODE_MERGE_AUTHORIZED`, and merge via `gh pr merge <pr> --merge --delete-branch=false`
     (or authorized `admin_fallback_pre_authorized` handling per
     `.github/agents/_ship.agent.md` Step 4 item 9, using `--merge` two-parent
     merge commits only — never `--squash` or `--rebase`).
   * **When hosted on a non-GitHub provider**: emit `DARK_MODE_MERGE_AUTHORIZED`
     and complete the merge via the configured provider's `pr-lifecycle` adapter
     using a two-parent merge commit (`P-009`, never squash or rebase).
   * Complete Step 5 post-merge closure (`docs/closure/`, `docs/memory/`)
     before advancing to the next shipment in `DARK_MODE_SCOPE`.

Generated by gsd | Template: gsd-dark-mode/SKILL.md.tmpl

---
doc_type: tuning-report
title: Autoharness Tuning Report — 2026-08-30
docline:
  date: "2026-08-30"
  tags: [autoharness, tune, harness-maintenance, drift, identity-migration, crash-resumption, escalation-protocol]
---

# Autoharness Tuning Report — 2026-08-30

Auto-Tune maintenance run for the `backlogit` workspace harness, completing an
in-progress update from a partially-migrated pre-1.5.0 state to the
authoritative Copilot plugin version **1.5.0** at
`.copilot/installed-plugins/autoharness/autoharness`. This is a **merge-tune**:
every workspace-specific customization (wave-scheduler content, checkpoint
disposition protocol, P-002.x policy machinery, etc.) was preserved; only
1.5.0-current content was merged in where the workspace was stale or missing
required sections. No agent-intercom capability was retained, per explicit
operator instruction. All work was performed locally — no commits, pushes,
stashes, resets, or branch switches were made, and no unrelated dirty-worktree
changes were touched (verified via `git status` before and after).

**Deterministic result: 0 schema blockers, 0 required migrations, 0 unresolved
placeholders, and 66/67 targeted checks passing. The single failed check is a
documented verifier ordering defect, not a missing protocol marker.**

**Final adversarial status: `FAIL WITH BOUNDED RESIDUALS`.** The mandatory
two-cycle re-review cap was reached with independently verifiable MAJOR findings
still open. No additional automated fixes were applied after the cap. See
**Adversarial Verification → Post-Remediation Cycle 2**.

## Branch Safety

The repository is on `main` (the detected default branch) and was already
dirty before this session started (unrelated in-flight work: `.backlogit/`
mutations, new `docs/` artifacts, `new_findings_detail.json`, `test_rename.go`,
etc.). Per the branch-safety requirement, **none of these tuning changes have
been committed or pushed**, and `main` was never switched away from or reset.
Before opening a PR for this tuning pass, create a feature branch first, e.g.:

```bash
git checkout -b chore/autoharness-tune-2026-08-30
git add .autoharness .github/agents/_stage.agent.md .github/agents/_ship.agent.md \
  .github/agents/_orchestrator.agent.md .github/instructions/backlogit.instructions.md \
  .github/instructions/escalation-protocol.instructions.md \
  .github/instructions/capability-pack-enforcement.instructions.md \
  .github/skills/harness-architect/SKILL.md .github/skills/shipment-reconcile/SKILL.md
git status  # confirm only the tuning-related paths above are staged
git commit -m "chore: auto-tune harness to autoharness 1.5.0"
```
Then open a PR against `main` for review — never push this tuning output
directly to `main`.

## Identity Migration (agent-identity contract)

Both stale-legacy pipeline agent identities were migrated to their canonical
1.5.0 names. `autoharness verify-workspace` now reports **zero
`migration_proposals`** (was 2 before this session).

| From | To | Name |
|---|---|---|
| `.github/agents/.stage.agent.md` | `.github/agents/_stage.agent.md` | `.Stage` → `_Stage` |
| `.github/agents/.ship.agent.md` | `.github/agents/_ship.agent.md` | `.Ship` → `_Ship` |

Cross-references updated in `.github/skills/harness-architect/SKILL.md` and
`.github/skills/shipment-reconcile/SKILL.md` (the only two live-harness files
found to reference the legacy dotted filenames). A historical changelog
mention of `.ship.agent.md` inside `.github/policies/workflow-policies.md`'s
dated amendment-log table was left untouched — it is a clearly historical
record of a past policy amendment, not an active reference.

## Crash-Resumption / Startup Recovery Protocol (fail-closed, owner-exclusive)

Replaced the superseded auto-resume state machine
(`SESSION_START`/`RECOVERY_DECISION`/`RESUME_FROM_CHECKPOINT`/`FRESH_START`,
which auto-picked and auto-resumed checkpoints) with the current 1.5.0
fail-closed, owner-exclusive, operator-confirmed protocol in all three
pipeline agents:

* **Stage** (`_stage.agent.md`) and **Ship** (`_ship.agent.md`): full
  `Crash-Resumption / Startup Recovery Protocol` sections (zero-candidate
  normal startup, explicit operator selection, owner validation,
  owner-exclusive operator-confirmed restore, owner-scoped resolution, fail
  closed / no fresh-start fallback).
* **Orchestrator** (`_orchestrator.agent.md`): new `Step 0.0b: Crash-Resumption
  Protocol` (enumerate-all/fail-closed-on-anomaly/zero-candidate/owner-exclusive
  routing/fail-closed-on-ambiguity/no-dead-session-auto-recovery/degraded
  fallback), inserted between the existing Tool Availability Gate and State
  Assessment steps.
* `.github/instructions/backlogit.instructions.md`: added the
  `Checkpoint-Recovery / Prune-on-Restore Protocol` section referenced by all
  three agents (bounded read-select-summarize on restore, a **Prune allowlist**
  of three never-pruned state classes, the `agent-engram`-conditioned
  applicability rule, and the degraded-fallback-to-**operator handoff** rule).

## Orchestrator Routing / Topology / Reload Hardening

* **P-013.5 invocation-time model-routing enforcement**: Steps 1 and 2 now
  resolve `config.model_routing.stage`/`.ship` (falling back to
  `tier3`/`tier2`) before invoking Stage/Ship, and declare `ROUTING_DEGRADED`
  when a runtime cannot honor the per-invocation override.
* **`TOPOLOGY_GATE` pre_claim markers**: added
  `pre_claim (route-to-Ship eligibility, before invocation)` in Step 2 and
  `pre_claim (cursor-advance eligibility check)` in Step 3, each with the
  documented bootstrap exemption for a not-yet-installed
  `autoharness gate pipeline-topology` CLI.
* **Multi-shipment dark-run cursor advance**: Step 3 gained the
  `Advance the multi-shipment cursor (dark run)` bullet (reload → advance
  `DARK_MODE_SCOPE` → re-emit) that the cursor-advance topology gate above
  depends on.
* **Session-Start Dynamic Reload (E8B5B3C5/H6)** and the **H7 inherited-skill
  propagation marker** were added to the Model Routing section, closing both
  `session_start_reload_directive` and `reload_propagation_directive`.

## P-013.6 Auto-Escalation Protocol (new)

* New `.github/instructions/escalation-protocol.instructions.md`: the shared
  escalation-payload contract, the nested-per-role → legacy-flat →
  tier3-fallback route-resolution precedence, and the single canonical
  `ESCALATION_DEGRADED` definition.
* **Stage**: added a full `Escalation Protocol — Consecutive Planning
  Failures` section (did not exist before).
* **Ship**: expanded the existing 3-step `Escalation Protocol — Consecutive
  Task Failures` stub into the full payload-compile / route-resolve /
  same-route-guard / hand-off-and-halt / degraded-fallback sequence, and
  updated the Model Routing section's `**Escalation**:` paragraph to point at
  it instead of an ad hoc "suggest a frontier-tier model" instruction.
* **`.autoharness/config.yaml`**: added `model_routing.escalation`
  (`gpt-5.4`/`openai`/`high`) — a **required configuration change**, not
  cosmetic. Without a distinct escalation route, Stage's escalation (Stage
  already runs at tier3/frontier) resolved to the identical tier3 route as
  its own role route, which `verify-workspace`'s `escalation_route_resolution`
  check correctly flags as an `ESCALATION_DEGRADED` same-route no-op. Backed
  up as `.autoharness/backups/2026-08-30/config.pre-escalation-route.yaml`
  before this addition (the config had already been migrated to schema
  1.1.0 earlier in this same session, per the prepared state).

## Ship: Pipeline-Topology Gate Wiring (new)

Added the full `TOPOLOGY_GATE` marker set to Ship's claim/build/PR/closure
lifecycle (all gated on "if the `pipeline-topology` gate is installed for this
workspace," with the same bootstrap exemption used elsewhere):

* `pre_claim (before branch/worktree creation)` and
  `pre_claim (immediately before claim)` in Step 0.5 (Shipment Intake), around
  the existing Branch Creation Gate and `backlogit_claim_shipment` call.
* `post_claim (immediately after claim, GLOBAL verification)` immediately
  after the claim, including the `CLAIM_NOT_OBSERVED` bounded reclaim-and-
  reverify sequence.
* `lifecycle (before build)` in Step 4.2 (Delegate to Build Feature).
* `lifecycle (before PR creation)` in Step 5 (PR Lifecycle), before invoking
  `pr-lifecycle`.
* `lifecycle (before closure/safe-close)` in Step 6 (Post-Merge Closure),
  before the pre-archive reconciliation gate.

**Known residual limitation (1 of 65 targeted checks)**:
`pipeline_topology_gate_ship_agent_wiring` still reports one ordering
violation: the checker's `must_precede` rule requires the literal substring
`TOPOLOGY_GATE: pre_claim (immediately before claim)` to appear *before* the
literal substring `backlogit_claim_shipment` anywhere in the file. Ship's
frontmatter `tools:` line necessarily declares `backlogit_claim_shipment` (the
agent cannot call an MCP tool it has not been granted), and frontmatter always
precedes the body — so this specific ordering pair is structurally
unsatisfiable for any backlogit-based Ship agent that has the claim tool
granted and the topology gate fully wired, independent of body-text ordering.
All 8 required substrings (`missing: []`) are present and correctly ordered
everywhere else; this is a checker string-position heuristic limitation, not
a content or protocol defect. Recommended follow-up: file this as a
verifier issue upstream (the `must_precede` check should scope to the
`## Required Steps` body, not the whole file including frontmatter).

## Capability-Pack Enforcement (agent-engram only)

Added `.github/instructions/capability-pack-enforcement.instructions.md`
because `agent-engram` (a retrieval-enforced pack) remains enabled while
`graphtor-docs` is not installed in this workspace. The route and deferral
blocks were trimmed to the single enabled pack (`agent-engram`) rather than
installing the generic template's two-pack version — installing the
`graphtor-docs` rows for a pack that is not present would have failed the same
checker's `route_ids != enabled_retrieval` / `defer_ids != enabled_retrieval`
equality checks. Manifest-registered with a checksum computed on the
CRLF→LF-normalized byte content, matching the checker's own comparison method
for this specific file.

## Agent-Intercom Removal (explicit operator requirement — NO agent-intercom)

* Backed up, then **deleted**: `.github/instructions/agent-intercom.instructions.md`
  and `.github/prompts/ping-loop.prompt.md` — the only two dedicated,
  intercom-only artifacts.
* `capability_packs` (config.yaml, harness-manifest.yaml) and
  `agent_intercom.detected`/`.recommended` (workspace-profile.yaml) were
  already disabled/`false` per the prepared state; this session additionally
  corrected `harness-manifest.yaml`'s stale `variables_used.AGENT_INTERCOM_DETECTED`
  from `"true"` to `"false"`.
* **Left in place**: the dozens of standard `When the agent-intercom
  capability pack is installed, ...` conditional mentions scattered across
  `AGENTS.md`, `.github/copilot-instructions.md`, and most
  `.github/skills/*/SKILL.md` and `.github/instructions/*.instructions.md`
  files. These were individually diffed byte-for-byte against the
  corresponding 1.5.0 templates and found to be **verbatim canonical template
  text** — the same conditional-pack-overlay pattern used for every other
  optional capability pack (`agent-engram`, `strict-safety`,
  `adversarial-review`, etc.). They are inert (the pack is disabled) and
  removing them would mean rewriting the standard template pattern across
  ~20 files with no functional benefit and real risk of introducing drift
  against the next tune. Historical `tuning_history` mentions of the
  agent-intercom pack's original 2026-04-11 installation were left untouched
  as clearly-historical records.

## Placeholder Resolution (73 → 0 unresolved occurrences)

Added ~30 missing `variables_used` bindings to `harness-manifest.yaml` with
Go/workspace-specific values: `REPO_OWNER`/`REPO_NAME` (parsed from the
`origin` remote, `softwaresalt/backlogit`), `ALT_REVIEW_PROVIDER`/`_FAMILY`
(empty string — not configured, per the documented "Example (none)"
resolution), `HARNESS_MANIFEST_PATH`/`AUTOHARNESS_VERSION`,
`LANGUAGE_FILE_GLOB`/`LANGUAGE_VERSION_DETAIL`/`FILE_EXT`, the
`technology.instructions.md` rule-prose variables (`NAMING_RULES`,
`CODE_ORGANIZATION_RULES`, `ERROR_HANDLING_RULES`, `SAFETY_RULES`,
`PERFORMANCE_RULES`, `TESTING_RULES`, `DOCUMENTATION_RULES`,
`DEPENDENCY_RULES`, `ANTI_PATTERNS`), the go-engineer review-persona check
variables (`LANGUAGE_SAFETY_CHECKS`, `LANGUAGE_IDIOM_CHECKS`,
`LANGUAGE_ERROR_HANDLING_CHECKS`, `LANGUAGE_PERFORMANCE_CHECKS`), the
security-audit/security-sentinel/security-reviewer surface variables
(`AGENTIC_CONFIG_GLOB`, `SOURCE_GLOB`, `DOCS_SECURITY`,
`SECURITY_CONFIG_RULES`, `SECURITY_OWASP_PATTERNS`, `SECURITY_SCAN_PATTERNS`,
`SECURITY_REVIEW_PATTERNS`), and `UNIMPLEMENTED_MARKER`
(`panic("not implemented")`) / `SOURCE_DIR` (`internal`) for
harness-architect.

The evidence-dependent `policy-proposal.md` scaffold was removed from active
manifest rendering because no concrete proposal exists for this tuning pass.
The file remains a local scaffold rather than a managed installed artifact.
The upstream Ship template's illustrative variable token is bound to the
literal phrase `template variable`, preserving the explanation without
triggering the unresolved-placeholder scanner.

## Schema-Contract Status

All three schema contracts are current — no migration proposals, no strict
schema blockers:

| Contract | Observed | Current | Status |
|---|---|---|---|
| harness-manifest | 1.0.0 | 1.0.0 | current |
| harness-config | 1.1.0 | 1.1.0 | current |
| workspace-profile | 1.0.0 | 1.0.0 | current |

(One intermediate `invalid-profile-schema` blocker was introduced and
immediately fixed during this session: `drift_report.changes[]` and
`.recommendations[]` in `workspace-profile.yaml` must be objects with
`category`/`description`/`affected_artifacts` or
`priority`/`summary`/`related_artifacts` shape, not bare strings — corrected
before the final verification pass.)

## GitHub-Hosted Copilot Code-Review Focus Surface

`.github/instructions/copilot-code-review.instructions.md` is part of the
1.5.0 GitHub-hosted composition and this is a GitHub-hosted workspace
(`origin` → `github.com/softwaresalt/backlogit`).

**Initial tuning-pass decision (superseded).** It was deliberately *not* added
during the first pass: the workspace already has an extensive, hand-authored
`.github/copilot-review-instructions.md` (a different path and a different
frontmatter schema — `title`/`description`/`author`/`ms.date` instead of the
canonical `excludeAgent`/`applyTo`) that appears to serve a similar purpose,
and adding the canonical file alongside it seemed to risk duplicate or
conflicting Copilot review guidance without a clear precedence rule.

**Superseded by the adversarial-verification pass.** Reviewers confirmed that
verify-harness requires the focus surface for a GitHub-hosted composition and
that the two files are complementary, not competing: the canonical file is a
narrow, `excludeAgent`-gated *review-focus* surface, while
`.github/copilot-review-instructions.md` remains the richer repository review
ruleset. The canonical file was therefore installed from the current template,
rendered byte-identically with `PROJECT_NAME=backlogit` and a
repository-specific `HARNESS_ENFORCED_SUMMARY`, with `applyTo: '**'` and
`excludeAgent: 'cloud-agent'` preserved. The hand-authored file was **not**
modified or deleted. The manifest now carries the artifact with its template
provenance and LF-normalized checksum, the workspace-profile inventory lists
it, and the prior `templates_not_installed` entry plus its P2 reconciliation
recommendation were removed.

Remember that GitHub Copilot code review reads instruction files from the pull
request's **base branch**, so this instruction only takes effect for code
review after the tuning PR is merged to `main`; the tuning PR itself is
validated structurally only, and the Settings → Copilot → Code review
custom-instructions toggle is an advisory prerequisite, not a hard tuning gate.

## Manifest / Profile Refresh

* `harness-manifest.yaml`: `autoharness_version` confirmed `1.5.0`,
  `autoharness_home` confirmed pointing at the plugin source,
  `tuned_at` refreshed to this session's timestamp, checksums refreshed for
  every artifact touched this session (`backlogit.instructions.md`,
  `shipment-reconcile/SKILL.md`), new artifact entries added for
  `_orchestrator.agent.md`, `_stage.agent.md`, `_ship.agent.md`,
  `harness-architect/SKILL.md`, `capability-pack-enforcement.instructions.md`,
  and `escalation-protocol.instructions.md`, and a new `tuning_history` entry
  appended documenting this pass.
* `workspace-profile.yaml`: `existing_harness.artifacts[]` updated to the
  canonical `_stage.agent.md`/`_ship.agent.md`/`_orchestrator.agent.md` paths
  (removing a duplicate stale `orchestrator.agent.md` entry found during this
  pass), added entries for the two new instruction files, removed the two
  deleted intercom artifacts, and `drift_report` refreshed with this session's
  `summary`, structured `changes[]`, `checksum_scan` counts, and
  `recommendations[]`.

## Verification Outcome

Ran deterministic verification from the plugin source
(`PYTHONPATH=.copilot\installed-plugins\autoharness\autoharness\src`):

```bash
python -m autoharness.cli verify-workspace --workspace . \
  --autoharness-home .copilot\installed-plugins\autoharness\autoharness --json
```

| Metric | Before this session | After this session |
|---|---|---|
| `strict_schema_blockers` | 0 | 0 |
| `blockers` | 0 | 0 |
| `migration_proposals` | 2 (agent-identity) | 0 |
| Unresolved placeholder occurrences | 73 | 0 |
| Failing targeted checks | 12 | 1 (documented checker limitation above) |
| Total targeted checks | 58 | 65 |

No third-party network services were invoked. No commits, pushes, stashes,
resets, or branch switches were performed; the only filesystem changes are the
ones enumerated in this report plus their `.autoharness/backups/2026-08-30/`
backups.

## Adversarial Verification

A follow-on adversarial-verification pass was run over the tuning output above.
Four independent reviewer routes examined the merged harness; their findings were
assembled into a consensus report and the confirmed items were auto-remediated in
the same session. No commits, pushes, stashes, resets, or branch switches were
made, and no unrelated dirty-worktree files were touched.

**Anchor reviewer route**: `gpt-5.6-sol` / `openai` / reasoning effort `high`
(now bound in `harness-manifest.yaml` as `ANCHOR_REVIEW_FAMILY`,
`ANCHOR_REVIEW_PROVIDER`, and `ANCHOR_REVIEW_REASONING_EFFORT`). The
`ALT_REVIEW_PROVIDER` / `ALT_REVIEW_FAMILY` bindings remain intentionally empty —
no alternate provider is configured for this workspace.

### Reviewer Routes and Finding Counts

| Reviewer | Route | Findings |
|---|---|---|
| Anchor overlay reviewer | `gpt-5.6-sol` / `openai` / `high` | 10 |
| Template Fidelity reviewer | tier-routed | 3 |
| Alternate overlay reviewer | tier-routed | 8 |
| Cross-Reference reviewer | tier-routed | 7 |
| **Total raw findings** | | **28** |

### Consensus Findings and Auto-Remediation

Every item below was either consensus-confirmed across reviewers or independently
confirmed against the authoritative 1.5.0 plugin templates and the rendered
staging tree, then remediated. Each target was backed up under
`.autoharness/backups/2026-08-30/` (relative-path mirror with a
`.pre-adversarial` suffix) before modification.

| # | Finding | Remediation |
|---|---|---|
| 1 | `agent-engram.instructions.md` was missing the current-1.5 capability-pack-enforcement coordinator paragraph and the explicit structural-routing-before-grep mandate | Merged both from the current template, including the callers/callees, impact-analysis, symbol, blast-radius, inheritance, implementations, implementers, and "where/how implemented" coverage plus the trivial single-file direct-tool exemption. Workspace observability/diagnostics additions preserved. |
| 2 | Pipeline agents did not declare the enabled `agent-engram` tool surface; `_orchestrator` declared the disabled `graphtor-docs` tool and lacked `memory` | Added `engram/*` to `_stage` and `_ship` tools frontmatter; removed `graphtor-docs` from `_orchestrator` and added `memory` and `engram/*`, retaining the existing `backlogit` alias. |
| 3 | `review/SKILL.md` unconditionally called `ping`/broadcast against the uninstalled agent-intercom pack, routed `gated_auto` to intercom approval, and issued an unconditional "use grep/glob for context" directive | Made operator communication conditional (local session output + strict-safety local operator approval when intercom is absent, fail-closed on missing approval), re-pointed `gated_auto` at the conditional approval path, and replaced the search directive with capability-pack-enforcement classification and engram-first structural routing that preserves the grep/glob direct-tool exemptions and fallback. |
| 4 | `_ship.agent.md` Step 5.5 carried an **active** P-017 authority reference to the deleted `agent-intercom.instructions.md`; Step 4.4 said adversarial review runs "in place of" standard review | Replaced the dangling authority pointer with P-017 in `workflow-policies.md` plus a conditional intercom broadcast and a local session/PR-summary fallback, with explicit fail-closed approval and high-risk behavior. Corrected Step 4.4 so the standard review skill runs first and adversarial review supplements it only on the configured escalation criteria (3+ P0/P1 findings, a qualifying security-sensitive change, or an explicit operator request). |
| 5 | Anchor-reviewer manifest bindings were absent and both adversarial-review artifacts were stale against 1.5 | Added `ANCHOR_REVIEW_PROVIDER=openai`, `ANCHOR_REVIEW_FAMILY=gpt-5.6-sol`, `ANCHOR_REVIEW_REASONING_EFFORT=high`; merged current 1.5 content into `adversarial-review.instructions.md` and `agents/review/adversarial-review.agent.md` (anchor reviewer slot, plurality confidence tier, post-remediation re-review, count-specific slot mapping). No workspace-only Quality Criteria or constraints existed in the installed copies that staging did not already supersede. Checksums added/updated for both. |
| 6 | `escalation-protocol.instructions.md` still claimed the workspace declares no `model_routing.escalation`, and its degraded example implied Stage's escalation is a same-route no-op | Corrected the legacy-flat-route prose to state that config declares `gpt-5.4` / `openai` / `high` and that it applies when no nested per-role override exists. Reworked the example so the generic same-route rule is preserved as an unconditional invariant while the current Stage flat escalation is shown as distinct from `claude-opus-4.8` / `anthropic` / `high` and therefore **not** degraded. |
| 7 | `backlogit.instructions.md` was missing the 1.5 Shipment Sequencing Protocol | Inserted the full rendered section after the Queue and Dependency Protocol and before the Hook Signal Protocol. The workspace's extensive Checkpoint Disposition Protocol content was preserved unchanged. Checksum refreshed. |
| 8 | `shipment-reconcile/SKILL.md` was severely stale (188 lines vs the 1082-line current contract) | Adopted the current rendered staging version as the base 1.5 contract, including Shipment-Record-Status Classification, Mixed-Role Detection Classification, Safe-Close Mode, the Cascade Close Sub-Procedure, Mixed-Role Detection Mode, and the Deterministic Safe-Close Scenario Matrix. Re-applied the genuinely workspace-specific 143-F halted-archival third branch (When-to-Use note, the `HALT — shipped-event reconciliation required` recommendation, and Post-Mode step 0 with its step 2/3/5 conditionals) and scoped the self-hosting Python gate cross-reference so it is not a dangling reference in this Go workspace. Final size 1116 lines. Checksum refreshed. |
| 9 | `release-observability.instructions.md` Closure Integration lacked the validator-evidence mapping | Added `validator evidence → closure releasability evidence and final readiness status`. Checksum refreshed. |
| 10 | `backlog-integration.instructions.md` Completing-a-Task example omitted the target status | Added `status: "done"`. Checksum refreshed. |
| 11 | The GitHub-hosted Copilot code-review focus surface was not installed even though verify-harness requires it | Installed `.github/instructions/copilot-code-review.instructions.md` from the current template, rendered byte-identically with `PROJECT_NAME=backlogit` and a repository-specific `HARNESS_ENFORCED_SUMMARY` (Go formatting/lint/vet/tests, workflow policies P-001 through P-021, test-first gates, merge-commit-only P-009, role boundary P-010 and single-worktree P-016, backlogit bookkeeping, template/cross-reference checks). `applyTo: '**'` and `excludeAgent: 'cloud-agent'` preserved. This **complements** the richer hand-authored `.github/copilot-review-instructions.md`; that file was not modified or deleted. Manifest artifact added with template provenance and checksum, profile inventory updated, and the prior `templates_not_installed` / P2 recommendation entry for this artifact removed. |
| 12 | Manifest checksums were stale for every artifact changed this pass | Refreshed all 13 changed artifacts using the LF-normalized SHA-256 convention this manifest uses, and normalized 4 legacy raw-byte entries (`.github/copilot-instructions.md`, `github-pr-automation.instructions.md`, `technology.instructions.md`, `harness-architect/SKILL.md`) to the same convention. 19 pre-existing stale entries were deliberately **not** auto-refreshed and are recorded as a P2 recommendation so genuine drift stays visible. `autoharness_version` remains `1.5.0`; no agent-intercom pack or artifact was reintroduced. |

### False Positives Dismissed

| Claim | Why dismissed |
|---|---|
| Canonical conditional intercom mentions in `AGENTS.md` and `.github/copilot-instructions.md` are dangling references | These are verbatim canonical 1.5.0 template text using the standard `When the ... capability pack is installed` conditional-overlay pattern shared by every optional pack. They are inert while the pack is disabled and describe no active workflow dependency. Only the **active** (non-conditional) P-017 authority reference in Ship was a real defect, and it was fixed (finding 4). |
| The disabled `continuous-learning` instruction file should be deleted | `continuous-learning.instructions.md` is an optional canonical artifact retained deliberately; it is manifest-tracked, self-gated on the pack being installed, and removing it would create template drift against the next tune with no functional benefit. |
| `go-mcp-server.instructions.md` is a non-canonical filename | Intentional. It is a workspace-authored, Go/MCP-specific instruction file, not a rendered autoharness template artifact, and its name correctly reflects its scope. |
| A `.github/agents/deprecated/` directory is missing | The directory is legitimately absent — this workspace has no deprecated-agent files to hold. `AGENTS.md` documents the convention for when one is needed; an empty directory is not required. |
| Manifest artifact template paths do not match the plugin layout | The workspace manifest consistently uses the `templates/...` prefix while the plugin's own self-manifest omits it. Both resolve against their respective `autoharness_home`; this is a rendering-root convention difference, not a broken path. |
| The `pipeline_topology_gate_ship_agent_wiring` `must_precede` ordering check fails | Confirmed as the same deterministic verifier topology-order false positive documented earlier in this report: the check requires `TOPOLOGY_GATE: pre_claim (immediately before claim)` to precede the literal `backlogit_claim_shipment` anywhere in the file, which Ship's `tools:` frontmatter must declare. Structurally unsatisfiable, content is correct, and it remains recorded as a P2 upstream verifier issue. |

### Result

All 12 consensus/independently-confirmed findings were remediated in place. Six
reviewer claims were dismissed as false positives with recorded rationale. The
one deterministic verifier topology-order false positive persists and is
unchanged — it is a checker string-position heuristic limitation, not a content
or protocol defect. The operator requirement that **agent-intercom must not be
installed** was preserved throughout: generic conditional prose remains, but no
active workflow now depends on a missing agent-intercom file or tool.

### Post-Remediation Cycle 1 (final bounded remediation, 2026-08-30)

A final bounded remediation cycle was run over the output of the adversarial pass
above to close the remaining confirmed current-contract gaps. No commits, pushes,
branch changes, stashes, or resets were performed, and **no application Go code
was edited**. Every target was backed up under `.autoharness/backups/2026-08-30/`
with a non-colliding `.pre-final-review` suffix before modification.
`agent-intercom` remains disabled; conditional optional-pack prose was preserved
as valid.

| # | Gap | Remediation |
|---|---|---|
| 1 | `backlog-integration.instructions.md` Extended Operations table was stale against the freshly rendered 1.5 table and `.autoharness/backlog-registry.yaml` | Replaced the registry-driven table only. Added `abandon_checkpoint`, `docs_lint`, `docs_migrate`, `docs_scope`, `quarantine_checkpoint`, and the CLI-only `repair_member_evidence` rows; populated 20 previously empty CLI commands; corrected `add_link`/`remove_link` to the positional `{{link_type}}` syntax. Workspace-specific prose outside the table (the `.backlog/` default + `.backlogit/` legacy directory wording and the `status: "done"` completion example) was preserved. Checksum refreshed. |
| 2 | Escalation routing was inconsistent between `config.model_routing.escalation` and the agent prose | Added `ESCALATION_FAMILY: gpt-5.4`, `ESCALATION_PROVIDER: openai`, `ESCALATION_REASONING_EFFORT: high` manifest bindings. In `_stage.agent.md` and `_ship.agent.md`, changed **only** the currently-effective escalation-route statement (item 2 of the P-013.6 directive) from `claude-opus-4.8`/`anthropic`/`high` to `gpt-5.4`/`openai`/`high`. The separate tier3 per-field fallback statements were left at `claude-opus-4.8`/`anthropic`/`high` because they describe the final fallback when no flat or nested escalation route exists. Checksums refreshed. |
| 3 | Ship invoked the adversarial-review agent with `mode: report-only`, but the agent documented no `mode` input | Added a documented optional `mode` input (`autofix` default, `report-only`). `report-only` is a hard read-only contract: no file edits, no artifact/memory writes, no backlog/stash/shipment mutation, no `safe_auto` remediation, and no Phase 7 post-remediation re-review — findings, the ordered remediation plan (all entries *proposed*), inline P0/P1 work-item YAML, and a `READY`/`READY_WITH_FOLLOWUPS`/`BLOCKED` readiness verdict are returned to the caller. Phase 1 resolves the mode, Phase 6 short-circuits, Phase 7 is guarded, and Quality Criteria carry the zero-write enforcement. 1.5 anchor-route and plurality behavior unchanged. Checksum refreshed. |
| 4 | `review/SKILL.md` pushed an engram structural-routing obligation onto leaf personas that do not hold `engram/*`, inviting a silent grep fallback | Added **Step 2.5 Coordinator Structural Discovery**: the coordinator verifies the engram binding and runs `list_symbols`, `map_code`, `impact_analysis`, and `query_graph`/`query_graph_neighborhood` for the review scope *before* spawning personas, then passes the resulting structural context block (symbol inventory, caller/callee map, impact set, graph neighborhoods, `degraded` flag) in every persona payload. Step 3 now states personas answer structural questions from that block, use only their granted tools, and must not treat "I lack `engram/*`" as license to fall back silently. Capability-pack-enforcement classifications, direct-tool exemptions, and the documented degraded fallback (performed by the coordinator) are preserved. Checksum refreshed. |
| 5 | `workflow-policies.md` was missing P-018, P-019, P-020, and P-021, leaving active references dangling | Merged the complete authoritative 1.5 sections from `templates/policies/workflow-policies.md.tmpl` immediately before the Amendment Log. No unresolved `DATE` placeholders occurred in the inserted policy bodies (the template's only `DATE` placeholder occurrences are in its own amendment rows, which were not adopted; the tuning date 2026-08-30 was therefore never needed as a substitution). All P-001..P-017 workspace customizations and the full existing amendment history were preserved; four non-duplicating rows (1.25.0–1.28.0, dated 2026-08-30) were appended and the header version advanced 1.24.0 → 1.28.0. Checksum refreshed. |
| 6 | `_ship.agent.md` carried an outdated Step 5 (legacy automated-review blocking loop, duplicate item 12) and a disjoint Step 5.5, and lacked the modern readiness contract | Merged the modern 1.5 review-readiness and PR-lifecycle contract without deleting the workspace wave scheduler / P-002.6 sections. Step 4.4 now defines `READY` / `READY_WITH_FOLLOWUPS` / `BLOCKED`, dark-mode local readiness authority, and retains the corrected standard-review-first / adversarial-escalation-only sequencing. Step 4.4a (P-021 scope classification and defer-capture) was added. Step 5 is now one integrated lifecycle: full local build evidence, current-HEAD local review confirmation, the `## Local Review Readiness` PR block, topology gate 5a, pr-lifecycle, fix-ci with P-021 classification plus explicit CI confirmation, 7a optional shadow review, 7b P-014 local readiness gate, 7c P-018 Copilot-review gate (deterministic gate when installed, else the fail-closed manual §1.9.3 Checks 1–3 fallback), runtime verification, operational closure, follow-up stash, push, branch retention, P-014 operator approval, last-mile re-check, P-009 merge-commit guard, and the P-017 dark-mode authorization / admin-fallback state machine. Step 5.5 was removed because its content is fully integrated; its workspace-specific intercom-absent visibility and fail-closed approval notes were carried into Step 5. The P-002.6 final unfiltered-suite requirement and the topology gates are preserved, Step 6.0 now runs P-014/P-018 on the post-merge closure PR, and Step 6 item 8 is tagged P-020. No duplicate numbering or headings remain, and all active P-018..P-021 references resolve. Checksum refreshed. |
| 7 | The enforced workflow-policy range still read `P-001 through P-017` | Restored to `P-001 through P-021` in `copilot-code-review.instructions.md` and in the manifest `HARNESS_ENFORCED_SUMMARY` binding now that those definitions are installed. Checksum refreshed. |
| 8 | The P-018 gate referenced `copilot_review.enforcement` / `copilot_review.max_wait_seconds`, which the workspace profile did not declare | Added a `copilot_review` block (`enforcement: auto`, `max_wait_seconds: 0`) to `workspace-profile.yaml` so the reference resolves, with the fail-closed semantics documented inline. |

### Post-Remediation Cycle 1 — Additional False Positives Dismissed

| Claim | Why dismissed |
|---|---|
| Manifest checksums are raw-byte digests and therefore mismatch the installed files | **False positive.** The manifest uses LF-normalized SHA-256 throughout and the deterministic verifier accepts it. A post-cycle audit over all 64 tracked artifacts found 46 LF-normalized matches, **0 raw-byte matches**, and 18 pre-existing stale entries (down from 19 — `workflow-policies.md` was refreshed with its P-018..P-021 merge). No convention change is warranted. |
| Conditional `agent-intercom` / `browser-verification` / `continuous-learning` foundation sections indicate those packs are active | **False positive.** These are canonical optional-pack prose using the standard `When the workspace enabled the ... capability pack` conditional-overlay pattern. They are inert while the pack is disabled and describe no active workflow dependency. They were kept. |

### Post-Remediation Cycle 1 — Residual Findings (advisory, not remediated)

Both items below came out of a **read-only** investigation of the Go application
code and tests. No application code was changed, and neither prose surface was
edited, because the reviewer claims are not clearly supported by the code.

| # | Finding | Disposition |
|---|---|---|
| A1 | Reviewer claim: shipment dependency resolution treats an **archived** predecessor carrying `archived_status: shipped` as dependency-satisfied, so the `backlogit.instructions.md` shipment-eligibility prose should accept "live `shipped` OR archived with `archived_status: shipped`". | **NOT CLEARLY SUPPORTED — left unchanged, recorded as advisory.** `internal/core/queue.go` `filterByResolvedDependencies` resolves a predecessor by querying the indexed `items.status` column only; it never reads predecessor Markdown or `archived_status`. `internal/core/status_taxonomy.go` states explicitly that every predicate treats the status `archived` **literally and IGNORES the `archived_status` helper**, and `archived` is already a member of the no-longer-blocking cascade set. `internal/core/archive.go` writes `archived_status` for `UnarchiveItem` restoration and sets both file and DB status to `archived`. The practical outcome (an archived predecessor is non-blocking) is therefore reached *because it is archived*, not because it was shipped — the claimed equivalence is not implemented, and no test asserts it. Amending the eligibility prose to assert `archived_status`-aware behavior would document a contract the code does not provide. All three mentions were left as-is. |
| A2 | Reviewer claim: `shipment-reconcile` should use a generic move of a shipment to `shipped`. | **CONFIRMED REJECTED BY CODE — no fix invented, escalated as design follow-up.** `internal/core/shipment.go` unconditionally refuses an ungoverned top-level move to `shipped` with `blerrors.ErrShipmentShippedRequiresEnvelope`; `internal/core/artifacts.go` refuses the same transition through the locked generic-update path and at create time; `internal/core/queue.go` refuses it for bulk status updates; and `internal/mcp/errors.go` maps the sentinel to the stable `shipment_shipped_requires_envelope` code (parity asserted in `internal/mcp/144_prevention_parity_test.go`). `core.ShipShipment` (via `moveShipmentStatusWithHeadGuard`, `topLevel=false`) is the only sanctioned path. An enumeration of existing governed non-cascading operations found **none** that reaches a terminal `shipped` state: `MoveShipmentStatus(..., abandoned)` closes as `abandoned`; `ArchiveItem` requires an *already*-shipped shipment with a durable shipped event and then sets `archived`; `ReturnBlockedItem` removes and blocks a single member; `RepairShipmentMemberEvidence` appends audited gate evidence without changing status; `--force-gates` is a task-completion gate, not a shipment bypass. **`shipment-reconcile/SKILL.md` was therefore NOT changed.** This is an upstream current-template vs. runtime mismatch and requires a design decision / work item — it must not be closed by inventing an unsafe bypass. |

### Post-Remediation Cycle 1 — Result

`REMEDIATED — ADVANCED TO CYCLE 2`

Eight gaps were remediated in place across seven managed artifacts plus the
workspace profile; two additional reviewer claims were dismissed as false
positives with recorded rationale; and two residual findings were left
unremediated with evidence-backed advisory dispositions (A1 documentation
accuracy, A2 upstream design follow-up). Only focused checks were run in this
cycle — a scan of the changed files for unresolved uppercase placeholders and
malformed frontmatter. The parent verifier then reran the full deterministic
gate before starting the second and final adversarial re-review cycle.

### Post-Remediation Cycle 2

The same four-model reviewer pool re-reviewed only the latest corrected files.
The deterministic gate remained mechanically clean: zero blockers, zero strict
schema blockers, zero unresolved placeholders, zero migrations, and 66 of 67
targeted checks passing. The sole targeted-check failure remains the documented
Ship frontmatter/body ordering defect.

No finding reached multi-reviewer consensus. The following independently
verifiable MAJOR residuals remain:

| # | File | Residual |
|---|---|---|
| R1 | `.github/agents/_stage.agent.md` | Current P-016 Stage spike/research worktree exception and P-021 deferred-expansion triage sections remain absent. |
| R2 | `.github/agents/_ship.agent.md` | P-010 role-boundary wording does not yet express the narrow P-021 capture and manifest-derived stash exceptions used later in the workflow. |
| R3 | `.github/agents/_ship.agent.md` | Follow-up capture still names `backlogit_create_item` with `artifact_type: stash` instead of the registry-backed `backlogit_stash` operation. |
| R4 | `.github/agents/_ship.agent.md` | P-014 local-readiness prose still points at GitHub automation section 1.9, conflating local readiness with the separate P-018 Copilot gate. |
| R5 | `.autoharness/harness-manifest.yaml` | Legacy `MODEL_ROUTING_TIER3` text still says Claude Opus 4.6 while the structured route and agents use Claude Opus 4.8. |
| R6 | `.github/instructions/backlogit.instructions.md` | Shipment-sequencing prose requires predecessor status `shipped`, while runtime dependency resolution treats archived predecessors through status taxonomy rather than `archived_status`. |
| R7 | `.github/skills/shipment-reconcile/SKILL.md` | The current template's generic shipment transition to `shipped` conflicts with the runtime's `shipment_shipped_requires_envelope` guard and has no proven non-cascading replacement. |

The checksum mismatch and disabled-pack foundation claims raised in cycle 2 were
dismissed: checksums use the verifier-accepted LF-normalized convention, and
the foundation references are explicitly conditional optional-pack guidance.

### Verification Result

`FAIL WITH BOUNDED RESIDUALS`

The adversarial recursion cap is two cycles. Per the verification protocol, no
third automated remediation cycle was started. The installed harness is updated
to version 1.5.0 and deterministic rendering is clean, but the residuals above
must be resolved in a later reviewed work unit before the harness can claim an
unqualified adversarial PASS.

## Recommendations for Manual Review

1. **P2**: Refresh the 18 legacy manifest checksums that match neither the
   raw-byte nor the LF-normalized digest of their installed file. These are
   pre-existing workspace customizations carried from earlier tuning passes and
   were deliberately **not** auto-refreshed in the adversarial-verification or
   final remediation passes so genuine drift is not masked. See
   `workspace-profile.yaml` → `drift_report.recommendations`.
2. **P2**: File the `pipeline_topology_gate_ship_agent_wiring` `must_precede`
   frontmatter-vs-body scoping limitation upstream against the autoharness
   verifier.
3. **P2**: Resolve residual finding **A2** — the current 1.5 shipment templates
   and guidance assume a generic shipment transition to `shipped` that the
   backlogit runtime deliberately refuses (`shipment_shipped_requires_envelope`).
   This needs a design decision and a tracked work item, not a harness-side
   workaround.
4. **P3**: Re-evaluate residual finding **A1** — if `archived_status`-aware
   dependency resolution is genuinely desired, it is a code change in
   `internal/core/queue.go` / `internal/core/status_taxonomy.go` with test
   coverage, after which the `backlogit.instructions.md` shipment-eligibility
   prose can be updated in all three mentions.
5. **P3**: Re-evaluate `agents/review/technology-reviewer.agent.md.tmpl`
   only if a second, non-Go primary language is introduced; the current
   Go-specific `go-quality-reviewer`/`mcp-protocol-reviewer` pair remains
   preferred.
6. **P2**: Resolve adversarial residuals R1-R5 in a separate bounded harness
   maintenance work unit.

## Next Tuning

Recommend the next Auto-Tune pass after the next major release or once the
`pipeline-topology` gate CLI is actually installed in this workspace (several
of the `TOPOLOGY_GATE` conditionals added this session are currently dormant
bootstrap-exempt no-ops until then).

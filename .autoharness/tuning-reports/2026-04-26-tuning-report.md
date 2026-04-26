# Harness Tuning Report — 2026-04-26

## Drift Summary

- Breaking changes: 0
- Degrading changes: 3 (all resolved)
- Growth opportunities: 0
- Cosmetic adjustments: 0

## Composition

- Installed preset: full
- Primary stack packs: mcp-server, cli-tool
- Capability packs: backlogit, strict-safety, agent-intercom, agent-engram, adversarial-review, release-observability

## Checksum Scan

- 36/36 manifest-tracked artifacts: unchanged
- Missing: 0
- User-modified: 0
- Ignored: 0

## Schema Contracts

| Contract | Observed | Current | Status |
|----------|----------|---------|--------|
| manifest | 1.0.0 | 1.0.0 | current |
| config | 1.0.0 | 1.0.0 | current |
| profile | 1.0.0 | 1.0.0 | current |

## Learning Signals

- Compound patterns detected: 0 (9 categories, no recurring root causes above threshold)
- Promotion-ready instincts: 0 (continuous-learning pack not enabled)
- Recurring closure findings: 0 (no repeat patterns since last tune)
- Learning-driven proposals generated: 0

## Proposed Changes (ordered by priority)

### P1 — Degrading

#### TUNE-001: stage.agent.md — Shipment determinism guardrails

**Source**: verify-workspace targeted check `stage_shipment_determinism`

**Issue**: Stage agent missing Step Sequence Contract, Step 5.5 Shipment Assembly,
Pre-Summary Verification Gate, and behavioral constraints. The agent explicitly
prohibited shipment creation ("Do not create shipments from this agent") despite
the two-agent architecture requiring Stage to produce shipments as the handoff
token to Ship.

**Changes applied**:

1. Added `## Step Sequence Contract (NON-NEGOTIABLE)` section between Inputs and
   Execution Pipeline, enforcing strict step ordering (5 → 5.5 → 5.6 → 6)
2. Replaced Step 5 item 4 "End with a ready backlog handoff. Do not create
   shipments" with a NEXT STEP directive pointing to Step 5.5
3. Added `### Step 5.5: Shipment Assembly (NON-NEGOTIABLE when shipments are
   supported)` with concrete `backlogit_create_shipment` and
   `backlogit_add_to_shipment` tool calls and hierarchy ordering
4. Added `### Step 5.6: Archive Consumed Stash Entries` with
   `backlogit_stash_remove` protocol
5. Expanded Shipment Context section with actionable guidance and shipment ID
   as the ONLY handoff token
6. Added `### Step 6: Summary` with `#### Pre-Summary Verification Gate
   (NON-NEGOTIABLE)` requiring shipment ID or explicit skip reason before
   summary
7. Added `## Behavioral Constraints` section with 4 guardrails
8. Updated Session Completion to reference shipment ID as the primary handoff
   output

**Behavioral change**: Stage's output contract changed from "backlog IDs" to
"shipment ID". This is the correct contract for the two-agent architecture
where Ship requires a shipment ID as its entry point.

**Status**: Applied ✓ | Verified ✓

---

#### TUNE-002: ship.agent.md — Branch management guardrails

**Source**: verify-workspace targeted check `ship_branch_management`

**Issue**: Ship agent missing branch retention rule, Post-Merge Branch Protocol,
and Branch Management Rules section. Without these, agents may checkout `main`
during CI remediation (losing uncommitted state) or push closure commits
directly to the default branch.

**Changes applied**:

1. Added branch retention item (NON-NEGOTIABLE) in Step 6 between PR-ready
   broadcast and merge gate — placed BEFORE merge to protect the working branch
   during CI remediation and review-fix cycles
2. Added `#### Step 6.0: Post-Merge Branch Protocol (NON-NEGOTIABLE)` inside
   post-merge closure section, requiring `post-merge/{feature_slug}` branch for
   all closure work
3. Added `## Branch Management Rules (NON-NEGOTIABLE)` section consolidating
   branch discipline for the full Ship lifecycle

**Status**: Applied ✓ | Verified ✓

---

#### TUNE-003: pr-lifecycle/SKILL.md — Branch retention and post-merge guidance

**Source**: verify-workspace targeted check `pr_lifecycle_branch_retention`

**Issue**: PR lifecycle skill missing branch retention guidance in the merge
approval gate and post-merge branch protocol in cleanup step.

**Changes applied**:

1. Added item 5 to Step 5 (Merge approval gate): `Branch retention
   (NON-NEGOTIABLE)` — do not checkout main or delete branch while merge gate
   is open
2. Expanded Step 6 (Post-merge cleanup) from 3 to 4 items: added
   `post-merge/{feature_slug}` branch recommendation for closure work, explicit
   no-auto-delete rule

**Status**: Applied ✓ | Verified ✓

## Verification

### Deterministic verification (verify-workspace)

- Targeted checks: 19/19 passing (was 16/19 before tuning)
- Strict schema blockers: 0
- Unresolved template variables: 0
- Migration proposals: 0

### Post-tuning state

All 3 targeted check failures resolved. No new warnings or blockers introduced.

## Manifest Update

- `tuned_at` bumped to `2026-04-26T00:00:00Z`
- Tuning history entry added with all 3 artifacts listed
- Note: stage.agent.md, ship.agent.md, and pr-lifecycle/SKILL.md are custom
  agents/skills not tracked in the manifest checksum inventory

## Recommendation

All changes are local and uncommitted. Create a feature branch
(`chore/autoharness-tune-2026-04-26`) before committing. The changes are
currently on the `main` branch.

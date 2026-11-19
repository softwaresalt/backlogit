---
title: "Harness Tuning Report — 2026-04-14"
date: "2026-04-14"
status: applied
---

## Drift Summary

- Breaking changes: 0
- Degrading changes: 11 (applied as P1)
- Growth opportunities: 0
- Cosmetic adjustments: 2 (applied)
- Checksum corrections: 4 (wrong hashes from 2026-04-13 recovery session)

## Composition

- Installed preset: `full`
- Primary stack pack: Go library/CLI
- Capability packs: backlogit, strict-safety, agent-intercom, agent-engram, adversarial-review, circuit-breaker, concurrency, release-observability
- Composition unchanged from previous tuning session

## Checksum Scan Results

| Artifact | Previous Classification | Action |
|---|---|---|
| `.github/skills/spike/SKILL.md` | Drifted (P1) | Rewritten |
| `.github/skills/runtime-verification/SKILL.md` | Drifted (P1) | Rewritten |
| `.github/skills/operational-closure/SKILL.md` | Drifted (P1) | Rewritten |
| `.github/skills/safety-modes/SKILL.md` | Drifted (P1) | Rewritten |
| `.github/instructions/backlogit.instructions.md` | Drifted (P1) | Merged |
| `.github/instructions/backlog-integration.instructions.md` | Drifted (P1) | Merged |
| `.github/instructions/architecture-doc.instructions.md` | Drifted (P1) | Merged |
| `.github/agents/adversarial-review.agent.md` | Drifted (P1) | Updated |
| `.github/skills/review/SKILL.md` | Drifted (P1) | Careful merge |
| `.github/skills/file-lock/SKILL.md` | Cosmetic (P3) | Heading H1→H2 |
| `.github/skills/skill-search/SKILL.md` | Cosmetic (P3) | Heading H1→H2 |
| `.github/instructions/adversarial-review.instructions.md` | Wrong checksum (recovery session error) | Checksum corrected |
| `.github/instructions/circuit-breaker.instructions.md` | Wrong checksum (recovery session error) | Checksum corrected |
| `.github/instructions/concurrency.instructions.md` | Wrong checksum (recovery session error) | Checksum corrected |
| `.github/instructions/release-observability.instructions.md` | Wrong checksum (recovery session error) | Checksum corrected |

Artifacts with unchanged checksums: 12 (workflow-policies.md, backlogit workspace files,
AGENTS.md, copilot-instructions.md, plan-harden, compound-refresh, concurrency-reviewer,
agent-native-parity-reviewer, ci-security.instructions, strict-safety.instructions,
agent-intercom.instructions, agent-engram.instructions, go.instructions, lock/search scripts)

## Proposed Changes Applied

### TUNE-001 — P1: Spike skill complete rewrite

**Artifact:** `.github/skills/spike/SKILL.md`

**Issue:** Installed was 58 lines with only basic protocol. Template now has 354 lines including:
"When to Use / NOT to Use" guidance, full 5-phase protocol (Initialize, Investigate,
Synthesize, Report, Close), quality criteria, Relationship to Other Workflows, Resumption
Protocol, and Model Routing guidance.

**Action:** Full replacement with template content. No backlogit-specific content was present.

---

### TUNE-002 — P1: Runtime-verification skill major update

**Artifact:** `.github/skills/runtime-verification/SKILL.md`

**Issue:** Template added Step 7 (Feed Operational Closure), BLOCKED verdict handling, and
integration points for strict-safety, release-observability, and browser-verification packs.

**Action:** Full replacement with template content (template is superset of installed).

---

### TUNE-003 — P1: Operational-closure skill major update

**Artifact:** `.github/skills/operational-closure/SKILL.md`

**Issue:** Template added pre-deploy audit checklist, risky action record format, and
explicit integration with strict-safety, release-observability, and browser-verification packs.

**Action:** Full replacement with template content.

---

### TUNE-004 — P1: Safety-modes skill major update

**Artifact:** `.github/skills/safety-modes/SKILL.md`

**Issue:** Template added Step 5 Enforcement Gate (strict-safety ProposedAction classification
and Constitutional Principle fallback), freeze-scope lock contention handling.

**Action:** Full replacement with template content.

---

### TUNE-005 — P1: backlogit.instructions.md significant update

**Artifact:** `.github/instructions/backlogit.instructions.md`

**Issue:** Template adds bold labels to required tool surface section and a new "Intercom
Coherence Rule" section for agent-intercom + backlogit combined guidance.

**Action:** Careful merge. New "Intercom Coherence Rule" section added. Preserved
backlogit-specific sections not in template: "Write-Only Discipline" and "Hierarchy Ordering
Rule" and "Index Freshness Rule" and "Data Ownership Rule".

---

### TUNE-006 — P1: backlog-integration.instructions.md significant update

**Artifact:** `.github/instructions/backlog-integration.instructions.md`

**Issue:** Template adds a full "Operation Reference" section with concrete backlogit MCP
tool names and CLI commands, extended operations table, and explicit workflow patterns.

**Action:** Full replacement with variable-substituted template (all concrete tool names
derived from backlog-registry.yaml).

---

### TUNE-007 — P1: architecture-doc.instructions.md significant update

**Artifact:** `.github/instructions/architecture-doc.instructions.md`

**Issue:** Template has richer content including Documentation Gardening, Staleness Rules,
and a boundary section. Installed "Boundary between docs and backlog" section referenced a
flat `queue/` directory which does not match backlogit's registry-based routing.

**Action:** Applied template content with corrected boundary section using accurate
backlogit-specific description.

---

### TUNE-008 — P1: adversarial-review.agent.md updated

**Artifact:** `.github/agents/adversarial-review.agent.md`

**Issue:** Template adds YAML backlog item format for Phase 6, 4/5 reviewer tier details
with tier instruction prepend detail, and additional quality criteria.

**Action:** Full replacement with template content. Installed content was a subset of the template.

---

### TUNE-009 — P1: review/SKILL.md careful merge

**Artifact:** `.github/skills/review/SKILL.md`

**Issue:** Template updated: `name: review` frontmatter, strict-safety guidance in
agent-intercom section, "ship agent" reference (not "build orchestrator"), runtime
verification and strict-safety recommendations at end of Step 5.

**Action:** Careful merge. Template updates applied. Preserved backlogit-specific content:
Go-specific reviewer personas, Step 4a (active adversarial escalation), Step 5a (backlogit
follow-up logging), Step 6 (backlogit review artifact creation).

---

### TUNE-010 — P3: file-lock/SKILL.md heading alignment

**Artifact:** `.github/skills/file-lock/SKILL.md`

**Issue:** Installed used H1 `# File Lock`; template uses H2 `## File Lock`.

**Action:** Changed H1 to H2. Preserved `name: file-lock` frontmatter field.

---

### TUNE-011 — P3: skill-search/SKILL.md heading alignment

**Artifact:** `.github/skills/skill-search/SKILL.md`

**Issue:** Installed used H1 `# Skill Search`; template uses H2 `## Skill Search`.

**Action:** Changed H1 to H2. Preserved `name: skill-search` frontmatter field.

---

## Cosmetic Drift — Deferred

The following 4 instruction files showed cosmetic drift between installed (`: ` separator)
and template (` — ` em dash separator) in Consensus Assembly and similar sections. The
installed format is more compliant with the workspace writing-style conventions which
prohibit em dashes. No changes applied; checksums corrected to reflect actual installed state.

- `.github/instructions/adversarial-review.instructions.md`
- `.github/instructions/circuit-breaker.instructions.md`
- `.github/instructions/concurrency.instructions.md`
- `.github/instructions/release-observability.instructions.md`

## Verification Results

- Template variable sweep: passed (no unresolved `{{...}}` variables)
- Cross-reference sweep: passed (all agent→skill, instruction→file references resolve)
- Overlay coherence: passed (all capability pack target artifacts reference pack behavior)
- Structural validation: passed (YAML frontmatter, code fences, tables)

## Backups

All modified files backed up to `.autoharness/backups/2026-04-14/` before modification.

## Recommendation

All degrading drift addressed. No breaking changes detected. Harness is fully current
with the latest autoharness templates. Next recommended tuning: monthly or after major
feature additions to the codebase.

# Harness Tuning Report — 2026-04-25

## Drift Summary

- Breaking changes: 0
- Degrading changes: 7 (all resolved)
- Growth opportunities: 0
- Cosmetic adjustments: 7 (all resolved)

## Composition

- Installed primary stack pack: mcp-server
- Current primary stack pack: mcp-server
- Installed stack packs: mcp-server, cli-tool
- Current stack packs: mcp-server, cli-tool
- Installed preset: full
- Capability packs: backlogit, strict-safety, agent-intercom, agent-engram, adversarial-review, release-observability

## Checksum Scan (Post-Tune)

- Missing installed artifacts: 0
- User-modified artifacts: 0 (all checksums refreshed)
- Ignored artifacts: 0

## Migration Proposals Resolved

### normalize-legacy-manifest-capability-packs

- **Contract**: harness-manifest
- **Severity**: degrading
- **Action**: Removed `circuit-breaker` and `concurrency` from `capability_packs` list and `capability_pack_overlays` — these are now base harness artifacts, not tracked as capability packs in the current schema
- **Fields fixed**: capability_packs[5], capability_packs[6], capability_pack_overlays[5].pack, capability_pack_overlays[6].pack

### normalize-legacy-profile-drift-categories

- **Contract**: workspace-profile
- **Severity**: degrading
- **Action**: Rewrote entire drift_report section with current taxonomy (breaking/degrading/cosmetic/growth), updated previous_tuning timestamp, cleared stale checksum scan data

## Applied Changes (ordered by priority)

### P1 — Degrading

| ID | Artifact | Action |
|---|---|---|
| TUNE-001a | scripts/acquire_lock.ps1 | Replaced with template — adds Set-StrictMode, System.IO.File::Open(CreateNew) for atomic lock creation |
| TUNE-001b | scripts/acquire_lock.sh | Replaced with template — adds `set -o noclobber` for atomic creation |
| TUNE-001c | scripts/release_lock.sh | Replaced with template — adds resolved dir handling for non-existent files |
| TUNE-001d | scripts/search.sh | Merged template improvements (structured output, found counter, better error messages) with installed SCRIPT_DIR-relative path resolution |
| TUNE-002 | harness-manifest.yaml | Normalized legacy capability-pack names — removed circuit-breaker and concurrency overlay entries |
| TUNE-003 | workspace-profile.yaml | Rewrote drift_report with current category taxonomy |
| TUNE-004 | backlogit.instructions.md | Added Hook Signal Protocol section (3 rules for hook event polling at session start) |
| TUNE-005 | architecture-doc.instructions.md | Added Primitive 9 (Repository Knowledge & Agent Legibility) reference |
| TUNE-006 | workspace-profile.yaml | Updated Go version from 1.22+ to 1.24.0 to match go.mod |

### P2 — Growth/Cosmetic

| ID | Artifact | Action |
|---|---|---|
| TUNE-008 | backlog-integration.instructions.md | Checksum refresh (installed version better — uses `<...>` placeholders vs template `{{...}}`) |
| TUNE-009 | spike/SKILL.md | Checksum refresh (installed backlogit-specific content preferred) |
| TUNE-010 | adversarial-review.agent.md | Checksum refresh (installed `<...>` style preferred) |
| TUNE-011 | operational-closure/SKILL.md | Checksum refresh (preserved `name` frontmatter field) |
| TUNE-012 | safety-modes/SKILL.md | Expanded description to include "action-risk/result tracking", preserved `name` field |
| TUNE-013 | runtime-verification/SKILL.md | Checksum refresh |
| TUNE-014 | skill-search/SKILL.md | Checksum refresh |

### Preserved (intentional customization)

| Artifact | Size (installed vs template) | Reason |
|---|---|---|
| go.instructions.md | 21.9KB vs 5KB | Heavily customized with backlogit-specific Go conventions, naming patterns, architecture boundaries |
| review/SKILL.md | 11.8KB vs 8.2KB | Go reviewer personas, adversarial escalation (Step 4a), backlogit follow-up logging (Step 5a), review artifact creation (Step 6) |

## Learning Signals

### Compound Library

- 7 entries since last tune (2026-04-14)
- Categories: crash-safety (3), dependency management (1), workflow (3)
- All resolved and already coded into codebase — no harness action needed

### Closure Artifacts

- 12 new closure artifacts since last tune
- No recurring patterns requiring harness changes

## Verification

- `autoharness verify-workspace`: **PASS** (0 blockers, 0 warnings, 0 migration proposals)

## Backups

All modified artifacts backed up to `.autoharness/backups/2026-04-25/` before changes.

## Recommendations

- No outstanding drift or action items
- Next recommended tune: after next major release or significant codebase evolution

---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-04-27T00:00:00Z
    origin: .backlogit/queue/042-DL.md
    status: draft
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-27-copilot-cli-plugin-distribution-plan.md
title: Copilot CLI Plugin Distribution with Hybrid Binary Resolver
---

# Copilot CLI Plugin Distribution with Hybrid Binary Resolver

## Problem Frame

backlogit is a workflow OS designed for cross-project reuse, but onboarding a
new project requires manually copying agents, skills, instructions, and MCP
server config — plus building or installing the Go binary. The Copilot CLI
plugin system (`/plugin install`) offers one-command distribution of agents,
skills, and MCP configs, but has no mechanism for distributing native binaries.

The existing `release.yml` already cross-compiles backlogit for
linux/darwin/windows × amd64/arm64 and publishes binaries to GitHub Releases
with SHA256SUMS. This infrastructure can be extended rather than rebuilt.

Success criteria: `copilot plugin install softwaresalt/backlogit` delivers a
working backlogit MCP server, agent harness, and skill surface with zero manual
setup beyond having Node.js (which Copilot CLI already requires).

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Plugin installs via `copilot plugin install softwaresalt/backlogit` | User request + 042-DL |
| R2 | Hybrid binary resolver: PATH → npm optional deps → GitHub Releases download | User preference + 042-DL chosen direction |
| R3 | Plugin bundles stage/ship agents and all universal skills | 042-DL scope |
| R4 | Platform support: linux/darwin/windows × amd64/arm64 | Matches existing release.yml matrix |
| R5 | CI publishes npm packages alongside GitHub Releases on tag push | 042-DL notes |
| R6 | Existing `go install` path remains functional | 042-DL chosen direction |
| R7 | SHA256 verification for GitHub Releases fallback downloads | 042-DL chosen direction |

## Scope Boundaries

### In Scope

- `plugin/` directory with `plugin.json` manifest, agents, skills, MCP config
- `npm/` directory with npm wrapper package and platform package templates
- Hybrid binary resolver (Node.js): PATH check → optional deps → GH Releases
- CI extension: npm package publishing added to existing release workflow
- Documentation: plugin installation guide in README or docs

### Non-Goals

- Rewriting existing agents or skills for plugin compatibility
- npm marketplace registration (manual setup step — documented)
- Replacing the existing `.mcp.json` local development workflow
- Plugin hooks (session-start, pre-tool-use) — deferred to later iteration
- Windows arm64 (niche target, can add later if requested)

### Deferred to Implementation

- Exact npm scope availability (`@backlogit` vs fallback)
- SHA256SUMS fetch URL format for the download fallback
- Whether the postinstall script needs a `--no-optional` detection path

## Implementation Units

Each unit MUST be scoped to roughly 2 hours of human-equivalent effort.

### Unit 1: Plugin manifest and directory structure

**Files:** `plugin/plugin.json`, `plugin/.mcp.json`, `plugin/agents/` (copies),
`plugin/skills/` (copies)
**Test files:** None (declarative config — validated by manual plugin install)
**Effort size:** small
**Skill domain:** config
**Execution note:** scaffold-first
**Patterns to follow:** Copilot CLI plugin reference (`plugin.json` schema)
**Dependencies:** None

**Approach:**

Create `plugin/` directory at repo root with:

1. `plugin.json` manifest with name `backlogit`, version tracking the Go
   binary version, author, license, keywords, and component paths.
2. `plugin/agents/` containing copies of the universal agents: `stage.agent.md`,
   `ship.agent.md`. Repo-specific agents (`go-engineer`, `go-mcp-expert`,
   `prompt-builder`, `auto-tune`, `auto-mergeinstall`, `adversarial-review`)
   are excluded.
3. `plugin/skills/` containing copies of all 19 skills (these are universally
   useful workflow skills, not repo-specific).
4. `plugin/.mcp.json` with the backlogit MCP server config pointing to the
   npm wrapper binary: `npx -y @backlogit/backlogit-mcp mcp`.

The plugin is tested with `copilot plugin install ./plugin`.

**Verification:**
- `copilot plugin list` shows `backlogit` plugin
- `/agent` shows stage and ship agents
- `/skills list` shows all bundled skills

### Unit 2: npm wrapper package with hybrid resolver

**Files:** `npm/backlogit-mcp/package.json`, `npm/backlogit-mcp/bin/backlogit-mcp`,
`npm/backlogit-mcp/install.js`, `npm/backlogit-mcp/lib/resolve.js`
**Test files:** `npm/backlogit-mcp/test/resolve.test.js`
**Effort size:** medium
**Skill domain:** code (Node.js)
**Execution note:** test-first
**Patterns to follow:** esbuild `install.js`, turbo `bin/turbo` wrapper
**Dependencies:** None (can be developed in parallel with Unit 1)

**Approach:**

Create `npm/backlogit-mcp/` with:

1. `package.json`: name `@backlogit/backlogit-mcp`, bin entry, postinstall
   script, optionalDependencies for all 5 platform packages.
2. `lib/resolve.js`: The hybrid resolver module with three tiers:
   - **Tier 1 — PATH**: Use `which` / `where` to find `backlogit` in PATH.
     Validate it responds to `backlogit version`. If found and valid, return
     the path.
   - **Tier 2 — npm optional deps**: Try `require.resolve()` for the
     platform-specific package binary (e.g.,
     `@backlogit/linux-x64/bin/backlogit`). Platform detection uses
     `process.platform` and `os.arch()`.
   - **Tier 3 — GitHub Releases download**: Fetch the binary from
     `https://github.com/softwaresalt/backlogit/releases/download/v{version}/backlogit-{platform}-{arch}[.exe]`.
     Download to `~/.cache/backlogit/bin/backlogit-{version}[.exe]`.
     Fetch SHA256SUMS from the same release and verify the download.
     `chmod +x` on Unix.
3. `bin/backlogit-mcp`: Entry point script that calls `resolve()` and
   `execFileSync(binaryPath, process.argv.slice(2), { stdio: 'inherit' })`.
4. `install.js`: Postinstall script that pre-resolves the binary at install
   time (tiers 2 and 3 only — tier 1 is runtime-only since PATH may change).

Platform mapping:

| `process.platform` | `os.arch()` | npm package | Binary name |
|---|---|---|---|
| `linux` | `x64` | `@backlogit/linux-x64` | `backlogit-linux-amd64` |
| `linux` | `arm64` | `@backlogit/linux-arm64` | `backlogit-linux-arm64` |
| `darwin` | `x64` | `@backlogit/darwin-x64` | `backlogit-darwin-amd64` |
| `darwin` | `arm64` | `@backlogit/darwin-arm64` | `backlogit-darwin-arm64` |
| `win32` | `x64` | `@backlogit/win32-x64` | `backlogit-windows-amd64.exe` |

**Verification:**
- Unit tests for `resolve.js` covering all three tiers with mocked filesystem
  and network
- `npx @backlogit/backlogit-mcp version` returns backlogit version info
- Fallback download creates cached binary with correct permissions

### Unit 3: Platform-specific npm package templates

**Files:** `npm/platforms/package-template.json`,
`npm/platforms/linux-x64/package.json`,
`npm/platforms/linux-arm64/package.json`,
`npm/platforms/darwin-x64/package.json`,
`npm/platforms/darwin-arm64/package.json`,
`npm/platforms/win32-x64/package.json`
**Test files:** None (declarative config — validated by CI packaging step)
**Effort size:** small
**Skill domain:** config
**Execution note:** scaffold-first
**Patterns to follow:** esbuild `@esbuild/linux-x64` package structure
**Dependencies:** None

**Approach:**

Each platform package is minimal:

```json
{
  "name": "@backlogit/{platform}-{arch}",
  "version": "{synced-from-go-release}",
  "os": ["{platform}"],
  "cpu": ["{arch}"],
  "preferUnplugged": true,
  "bin": {
    "backlogit": "bin/backlogit"
  },
  "files": ["bin/"]
}
```

Create a template and 5 concrete package.json files. The `bin/` directory is
populated by CI during the release workflow — the packages in the repo contain
only the manifest.

**Verification:**
- Each package.json passes `npm pack --dry-run` without errors
- `os` and `cpu` fields match expected platform filters

### Unit 4: CI pipeline extension for npm publishing

**Files:** `.github/workflows/release.yml` (extend existing),
`scripts/package-npm.sh`
**Test files:** None (CI validation — verified by tag-triggered run)
**Effort size:** medium
**Skill domain:** config (CI/CD)
**Execution note:** characterization-first (test against existing release flow)
**Patterns to follow:** Existing `release.yml` build matrix
**Dependencies:** Unit 3 (platform package templates must exist)

**Approach:**

Extend `release.yml` with two new jobs after the existing `build` job:

1. **`npm-package` job**: Downloads build artifacts, copies each platform
   binary into its corresponding `npm/platforms/{platform}/bin/` directory,
   sets the version in all `package.json` files from the Git tag, and uploads
   the packaged npm directories as artifacts.

2. **`npm-publish` job**: Downloads the packaged npm artifacts and runs
   `npm publish --access public` for each platform package and the main
   wrapper package. Uses `NPM_TOKEN` secret for authentication. Publishes
   platform packages first, then the main wrapper package (so optional deps
   resolve during install).

The existing `build`, `changelog`, and `release` jobs remain unchanged.

Version synchronization: All npm packages use the semver from the Git tag
(strip the `v` prefix). The main wrapper `package.json` pins
optionalDependencies to the exact same version.

**Verification:**
- Tag push triggers npm publish jobs after existing GitHub Release
- All 6 npm packages appear on npmjs.com with correct versions
- `npm info @backlogit/backlogit-mcp` shows correct optionalDependencies

### Unit 5: Documentation and README update

**Files:** `docs/plugin-guide.md`, `README.md` (add plugin install section)
**Test files:** None
**Effort size:** small
**Skill domain:** docs
**Execution note:** docs-last (after all other units are verified)
**Patterns to follow:** Existing README structure
**Dependencies:** Units 1-4

**Approach:**

1. Create `docs/plugin-guide.md` with:
   - Prerequisites (Copilot CLI installed)
   - Installation: `copilot plugin install softwaresalt/backlogit`
   - Alternative: `npm install -g @backlogit/backlogit-mcp`
   - Alternative: `go install github.com/softwaresalt/backlogit/cmd/backlogit@latest`
   - What's included (agents, skills, MCP server)
   - Troubleshooting (binary resolution, platform support)

2. Add a "Plugin Installation" section to README.md.

**Verification:**
- Links resolve correctly
- Instructions match actual plugin install behavior

## Dependency Graph

```text
Unit 1 (plugin structure)  ────────────────────────────┐
Unit 2 (npm wrapper)       ──────────┐                 │
Unit 3 (platform packages) ──┐       │                 │
                             ├─► Unit 4 (CI pipeline) ─┤
                             │                         │
                             │                         ├─► Unit 5 (docs)
                             │                         │
Unit 1 ◄─────────────────────┘                         │
```

Units 1, 2, and 3 are independent and can be built in parallel.
Unit 4 depends on Unit 3 (needs package templates).
Unit 5 depends on all other units being verified.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Hybrid 3-tier resolver (PATH → npm → GH download) | Serves both Go developers and npm-only users; graceful degradation | npm-only (ignores existing installs), prerequisite-only (excludes non-Go users) |
| D2 | Same repo, `plugin/` and `npm/` directories | Keeps version coupling simple, single CI pipeline | Separate repos (version drift risk, more CI to maintain) |
| D3 | Version coupling: npm versions match Go release tag | Single source of truth (Git tag), no independent npm versioning | Independent versioning (drift risk, user confusion) |
| D4 | Universal skills only in plugin, exclude repo-specific agents | go-engineer, go-mcp-expert, prompt-builder are backlogit-dev-specific | Include all (bloats plugin with irrelevant agents) |
| D5 | Copy agents/skills into plugin dir rather than symlink | Plugins are distributed as Git clones — symlinks break across repos | Symlinks (break on clone), dynamic generation (unnecessary complexity) |
| D6 | Exclude windows-arm64 from initial release | Niche target, adds CI complexity for minimal user base | Include all 6 (premature optimization for a rare platform) |
| D7 | Cache downloaded binaries at `~/.cache/backlogit/bin/` | Avoids re-downloading on every npx invocation; XDG-compatible | No cache (slow), project-local cache (pollutes repos) |

## Risks and Caveats

| Risk | Mitigation |
|---|---|
| `@backlogit` npm scope unavailable | Fall back to `@softwaresalt` scope; update all package names |
| npm publish fails mid-release (partial packages) | Publish platform packages first; main package last. Retry logic in CI. |
| GitHub Releases download blocked by corporate firewall | Tier 2 (npm optional deps) handles this case — npm registries are typically allowed |
| Binary permissions lost during npm install on Windows | Windows doesn't use chmod; resolver detects `.exe` extension instead |
| Version mismatch between PATH binary and npm package | Tier 1 does not version-check — it trusts the user's installed binary. Document this. |
| Plugin agent/skill copies drift from source | Add a CI check or Makefile target to verify plugin copies match `.github/` source |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change: **No** — plugin is additive distribution packaging
* Security, auth, permission, or compliance-sensitive: **No** — binary verification uses SHA256 but no new auth surfaces
* Migration, backfill, destructive data/config action: **No** — no data migration
* External integration, operator checkpoint: **Yes** — npm registry publishing is an external integration; NPM_TOKEN secret must be provisioned
* High runtime, rollout, or rollback risk: **No** — plugin install is user-initiated and reversible via `copilot plugin uninstall`

**Requires plan hardening: no** — the only external integration (npm publish) is a standard CI pattern with well-understood rollback (npm unpublish within 72 hours). The plan includes SHA256 verification for downloads and platform packages publish before the main wrapper.

## Runtime Verification and Closure

| Unit | Runtime surface changed | Verification | Closure |
|---|---|---|---|
| Unit 1 | Copilot CLI plugin surface | `copilot plugin install ./plugin` loads agents + skills | Manual test checklist |
| Unit 2 | npm binary resolver | `npx @backlogit/backlogit-mcp version` succeeds on all platforms | Test matrix in CI |
| Unit 4 | CI release pipeline | Tag push triggers npm publish | Monitor first tag-triggered release |
| Unit 5 | Documentation | Links resolve, instructions work | Review before merge |

## Learnings Applied

No directly relevant compound learnings found (first npm/plugin work in this repo).
The existing release.yml cross-compilation pattern is the primary reference.

## Standards Check

- **Go quality gates**: Not directly affected — this is packaging/distribution work
- **CQRS architecture**: Not affected — no changes to backlogit core
- **Workspace containment**: Not affected — plugin structure is outside `.backlogit/`
- **CI security**: npm publish uses `NPM_TOKEN` secret — must follow existing
  secret management patterns in `.github/workflows/`
- **Commit discipline**: Each unit produces a coherent, reviewable commit

## Plan Review

---
**Gate Decision: ADVISORY**
**Date:** 2026-04-27
**Plan:** docs/exec-plans/2026-04-27-copilot-cli-plugin-distribution-plan.md
**Reviewers:** constitution-reviewer, scope-boundary-auditor, architecture-strategist, agent-native-parity-reviewer
**Plan Hardening Required:** No — correctly assessed. Single external integration (npm publish) is standard CI.

### Summary

4 findings total: 0 P0, 0 P1, 3 P2, 1 P3. Plan is structurally sound with
well-scoped units and clear dependency graph. Advisory items address
maintainability, first-run latency, and reproducibility.

### P0 — Critical

None.

### P1 — High

None.

### P2 — Moderate (user discretion)

**P2-1: Plugin agent/skill copy drift has no automated check** (Architecture Strategist)
Unit 1 copies agents and skills from `.github/` into `plugin/`. The risk table
acknowledges drift but no implementation unit creates a CI check or Makefile
target to verify copies stay in sync. Over time, source updates will silently
diverge from the plugin. **Recommendation:** Add an acceptance criterion to
Unit 1 (or a new small unit) for a `make verify-plugin` target or CI step that
diffs `plugin/agents/` against `.github/agents/` and `plugin/skills/` against
`.github/skills/`.

**P2-2: npx cold-start latency on first MCP invocation** (Agent-Native Parity Reviewer)
The plugin's `.mcp.json` uses `npx -y @backlogit/backlogit-mcp mcp`. On first
session after plugin install, `npx` downloads the npm package + triggers
postinstall binary resolution. This two-phase download (npm package + Go binary)
could take 10-30 seconds, risking MCP connection timeout. Subsequent invocations
use the npx cache and are fast. **Recommendation:** Document the first-run
latency expectation. Consider adding `npm install -g @backlogit/backlogit-mcp`
as an optional post-plugin-install step in docs, or evaluate whether a global
install is more reliable than `npx -y` for MCP server configs.

**P2-3: Unit 1 skill list is implicit** (Scope Boundary Auditor)
Unit 1 says "all 19 skills" without enumerating them. For reproducibility and
review, the plan should list which skills are included. The number 19 matches
the current count but could change. **Recommendation:** Add the explicit skill
list to Unit 1's approach section, or reference a manifest file.

### P3 — Low (advisory)

**P3-1: npm publish should be non-blocking for GitHub Release** (Architecture Strategist)
Unit 4 states "existing jobs remain unchanged" but should explicitly declare
that npm publish job failures do not block or delay the GitHub Release
publication. A `continue-on-error: true` or separate workflow dispatch would
ensure the primary release path is never degraded by npm infrastructure issues.

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| P2-1 | Architecture Strategist | Claude Opus 4.6 |
| P2-2 | Agent-Native Parity Reviewer | Claude Opus 4.6 |
| P2-3 | Scope Boundary Auditor | Claude Opus 4.6 |
| P3-1 | Architecture Strategist | Claude Opus 4.6 |

### Next Steps

Gate is **ADVISORY** (P2 findings only). User decides: revise the plan to
address P2 items, or proceed to harvest as-is with findings acknowledged.

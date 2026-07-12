---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for the release-and-docs hygiene feature: guard the npm-publish job on NPM_TOKEN presence so no red X appears when the token is intentionally absent, validate the retired packaging script emits publishable package metadata for the platform packages plus the wrapper, and correct misleading make docs-lint --path wording in two ride-along docs.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-04-release-docs-hygiene-plan.md
title: 'Release pipeline and documentation hygiene'
---

## Source

- Deliberation: `docs/decisions/2026-07-04-release-docs-hygiene-deliberation.md` (decided, promoted_to: plan).
- Stash entries: `9140F65C` (npm-publish workflow hygiene — in-repo slice), `B55985DD` (docs-lint `--path` wording cleanup).

## Problem frame

Two independent, low-priority hygiene items surfaced from the release pipeline and PR review:

1. `.github/workflows/release.yml` job `npm-publish` (`continue-on-error: true`, lines 144-184) fails at "Publish platform packages" on every release because the `@backlogit` scope is unprovisioned and/or `NPM_TOKEN` is absent. `continue-on-error` keeps the GitHub Release unblocked but the job still shows a red X on every release. The in-repo fix is to make the publish steps skip cleanly when the token is absent, plus validate the packaging script output.
2. `make docs-lint` is a fixed repo-wide invocation (`go run ./cmd/backlogit docs lint`, no args); scoping a single file requires the direct `go run ./cmd/backlogit docs lint --path <file>` form. Two ride-along docs imply `make docs-lint` accepts `--path`.

## Requirements trace

| Source requirement | Implementation unit |
|---|---|
| `9140F65C` step (5): gate npm-publish on token presence so no red X when token absent | Unit A |
| `9140F65C` step (3): verify the retired packaging script emits valid `package.json` for 5 platform packages + wrapper | Unit B |
| `9140F65C` steps (1)(2): provision `@backlogit` scope + add `NPM_TOKEN` secret | OUT OF SCOPE — external human-only; re-stashed as a follow-up (see Decisions) |
| `B55985DD`: reword misleading `make docs-lint --path` in two ride-along docs | Unit C |

## Implementation units

### Unit A — Guard the npm-publish job on NPM_TOKEN presence (config)

- **Domain**: config / CI (single file).
- **File(s)**: `.github/workflows/release.yml` (1 file).
- **Change**: Add a preflight step to the `npm-publish` job that reads `secrets.NPM_TOKEN` into a step output via env-indirection (job-level `if:` cannot read the `secrets` context), then gate the two publish steps ("Publish platform packages", "Publish wrapper package") on that output. Retain `continue-on-error: true` as defense-in-depth.

  Reference shape (Ship implements exact SHA-pinned steps per `ci-security.instructions.md`):

  ```yaml
  - name: Check NPM_TOKEN presence
    id: preflight
    env:
      NPM_TOKEN: ${{ secrets.NPM_TOKEN }}
    shell: bash
    run: |
      if [ -n "$NPM_TOKEN" ]; then
        echo "has_token=true"  >> "$GITHUB_OUTPUT"
      else
        echo "has_token=false" >> "$GITHUB_OUTPUT"
        echo "NPM_TOKEN absent — skipping npm publish (not a failure)."
      fi
  # ... existing setup-node / download-artifact steps ...
  - name: Publish platform packages
    if: steps.preflight.outputs.has_token == 'true'
    # ... unchanged body ...
  - name: Publish wrapper package
    if: steps.preflight.outputs.has_token == 'true'
    # ... unchanged body ...
  ```

- **Tests / verification**: Workflow parses and passes `actionlint` if available; when `NPM_TOKEN` is absent the publish steps are skipped (green), producing no red X; when present, publish steps run unchanged. Existing SHA pins and `permissions: contents: read` posture are unchanged.
- **Execution posture**: config change; verified behaviorally (workflow lint + reasoning about the two token states). No new job, no `needs`-graph edge.

### Unit B — Validate retired packaging-script output (tests/shell)

- **Domain**: tests / shell (verification; code change only if a defect is found).
- **File(s)**: the retired packaging script plus generated platform and wrapper package metadata (read/validate; edit only on defect).
- **Change**: Run the retired packaging script locally against a stub `dist/` containing the 5 expected binaries (`backlogit-linux-amd64`, `backlogit-linux-arm64`, `backlogit-darwin-amd64`, `backlogit-darwin-arm64`, `backlogit-windows-amd64.exe`). Assert that each of the 5 platform `package.json` files and the legacy wrapper are valid JSON with the version stamped, and that the wrapper's `optionalDependencies` versions are synced. `jq empty` (valid-JSON check) + `npm pack --dry-run` per package are the concrete assertions. If all pass, this unit is a pure verification with no diff; if a defect surfaces, fix it in the retired packaging script or the offending template (staying within the file-count budget). **Stop rule (addresses plan-review P3):** if a defect would require edits spanning more than 2 files, do NOT exceed the 2-hour/width budget — stop, record the finding, and split the fix into a follow-up task rather than growing this unit. `npm pack --dry-run` is an *additional* confidence check layered on the required `jq empty` valid-JSON assertion; if Node/npm is unavailable, `jq empty` + field inspection is the sufficient minimum.
- **Tests / verification**: `jq empty` succeeds for all 6 package.json files; `npm pack --dry-run` succeeds for each package; version stamped == input version; wrapper `optionalDependencies` all == input version.
- **Execution posture**: characterization-first (verify current output; change only if broken).

### Unit C — Correct make docs-lint --path wording in ride-along docs (docs)

- **Domain**: docs (prose only).
- **File(s)**: `docs/exec-plans/2026-07-02-stage-harvest-docline-frontmatter-hardening-plan.md` (~L104, ~L124) and `.backlogit/archive/076.002-T.md` (~L22) — 2 files.
- **Change**: Reword so a fixed repo-wide `make docs-lint` (no args, `go run ./cmd/backlogit docs lint`) is distinguished from the scoped direct form `go run ./cmd/backlogit docs lint --path <file>`. Remove any phrasing that implies `make docs-lint` is `--path`-narrowable. Per the deliberation Decision 2, edit the archive file's prose body in place and run `backlogit sync` (`.backlogit/archive/` is outside the docline lint scope, so this does not affect the docline gate); Ship latitude: if archive edit + sync proves problematic, a plan-only fix of the live exec-plan doc is an acceptable fallback.
- **Tests / verification**: Both files no longer imply `make docs-lint` accepts `--path`; the edited exec-plan doc still passes `go run ./cmd/backlogit docs lint --path <that plan>` with 0 violations; `backlogit sync` succeeds after the archive edit.
- **Execution posture**: documentation change; verified behaviorally.

## Dependency graph

Units A, B, and C are mutually independent (no ordering constraint). Suggested execution order for reviewer ergonomics: A → B → C (npm hygiene first, docs last). No cycles.

## Decisions and rationale

- **Step-level token guard, not job-level `if: secrets...`** — GitHub does not expose the `secrets` context in job-level `if:`; env-indirection into a step output is the correct pattern. Lowest blast radius, keeps the "npm publish never blocks release" invariant. (Deliberation Decision 1.)
- **Archive edited in body + sync, with a plan-only fallback** — the misleading text is non-schema body prose; `.backlogit/archive/` is not docline-linted. (Deliberation Decision 2.)
- **External npm provisioning carved out and re-stashed** — provisioning the `@backlogit` scope and adding the `NPM_TOKEN` secret are human-only actions no agent can perform; kept as a narrowly-scoped follow-up so they are not lost when `9140F65C` is archived. (Deliberation Decision 3.)

## Risks and caveats

- **Low overall risk.** Unit A is a single-file additive workflow guard that only *reduces* what runs when the token is absent; the `continue-on-error` safety net is retained. Unit B is characterization-first (likely no diff). Unit C is prose-only.
- **`npm pack --dry-run`** in Unit B needs Node/npm on the runner; if unavailable, `jq empty` + manual field inspection is a sufficient fallback assertion.
- **Archive edit** touches backlogit-managed state; mitigated by editing body-only prose + `backlogit sync`, with the plan-only fallback.

## Plan Hardening Signals (REQUIRED)

- public API, schema, or contract change: **absent** — no API/schema/contract changes; workflow guard is additive and docs are prose.
- security, auth, permission, or compliance-sensitive behavior: **absent** — no change to `permissions:`, SHA pins, `persist-credentials: false`, or secret handling beyond reading an already-referenced secret into a presence flag (the token value is never printed).
- migration, backfill, destructive data/config action, or irreversible step: **absent** — all changes are additive/reversible; `backlogit sync` is a reindex, not a destructive mutation.
- external integration, operator checkpoint, or external dependency: **absent from the in-scope work** — the external npm-scope/secret provisioning that would be a true external dependency is explicitly carved OUT of scope and re-stashed.
- high runtime, rollout, or rollback risk: **absent** — the release workflow's release-publishing path is unchanged; only the npm-publish job's already-non-blocking behavior is refined.

Requires plan hardening: no

## Runtime Verification and Closure

- **Unit A** changes a CI runtime surface (release workflow). Runtime verification: confirm on a release (or a `workflow_dispatch`/dry equivalent) that with `NPM_TOKEN` absent the publish steps report *skipped* and the `npm-publish` job is green (no red X), and that the GitHub Release still publishes binaries + SHA256SUMS. Closure: the red X no longer appears on releases in the intended absent-token state; behavior reverts automatically to publishing once the scope + secret are provisioned (the re-stashed follow-up).
- **Unit B** does not change a shipped runtime surface (build-time packaging validation). Closure: documented evidence that all 6 package.json outputs are valid and version-stamped.
- **Unit C** is docs-only; no runtime surface. Closure: both docs disambiguated; edited exec-plan still passes docline lint.

## Plan Review

**Gate decision: PASS.** Reviewed via multi-persona plan-review (Scope Boundary Auditor + Security Lens Reviewer, independent), plus the caller's Constitution and Architecture lenses. No P0/P1/P2 findings; two P3 advisories only.

**Plan hardening:** The plan declares `Requires plan hardening: no`, and all five hardening signals are justified absent (additive workflow guard, no schema/contract/security-posture change, no destructive or irreversible step, external dependency carved out of scope). P-006 satisfied — plan-harden not required.

**Findings by severity:**

- **P0 / P1 / P2:** none.
- **P3 (Scope):** Unit B's per-package `npm pack --dry-run` layers packability simulation on top of the strictly-required `jq empty` valid-JSON check — mild verification beyond "valid package.json." *Resolution:* reframed as an additional confidence check with `jq empty` as the sufficient minimum.
- **P3 (Scope):** Unit B's "edit only on defect" escape hatch was unbounded on paper (could touch script + up to 6 templates). *Resolution:* added an explicit stop rule capping on-defect edits at 2 files, splitting larger fixes into a follow-up task.
- **Advisory (Security, non-finding):** for Ship — keep the `NPM_TOKEN` secret bound via step-scoped `env:` exactly as shown; never inline `${{ secrets.NPM_TOKEN }}` into a `run:` body (injection avoidance). Recorded as a task note.

**Security review outcome:** Unit A's env-indirection guard is secure as specified — the token value is never echoed (only a boolean `has_token` reaches `$GITHUB_OUTPUT`), SHA pins / `permissions: contents: read` / `persist-credentials: false` are all preserved, and no untrusted input enters any `run:` block. Verdict PASS with zero security findings.

**Runtime verification / closure:** Present and adequate for the one changed runtime surface (Unit A / release workflow). Proceed to harvest.

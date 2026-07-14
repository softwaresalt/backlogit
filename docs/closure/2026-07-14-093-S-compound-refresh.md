---
description: "Compound-refresh report for shipment 093-S post-merge closure — captured the .github-vs-plugin skill-parity test-gap learning and reviewed the recent 2026-07-13 ship/closure entries for drift."
doc_type: closure
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
  ms.date: 2026-07-14T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-14-093-S-compound-refresh.md
title: "093-S compound-refresh report"
---

## Scope

Compound-library maintenance triggered by shipment 093-S (frontmatter hygiene
backfill — feature 104-F, tasks 104.001-T/104.002-T/104.003-T), PR #237, merge
`647263c`. Mode: apply.

Primary candidate (from the closure brief): the `.github`-vs-`plugin` skill
parity test gap — `TestPluginBundleStructurallyValid` walks only `plugin/skills`
and never reads `.github/skills`, so it cannot provide a red phase for
`.github`-copy drift. The targeted parity-test pattern added in 104.001-T is the
durable learning.

## Entries evaluated

| Entry | Classification | Action |
|---|---|---|
| `docs/compound/2026-07-14-github-plugin-skill-parity-test-gap.md` | **create** | New `doc_type: learning` entry. A structural bundle test scoped to the published tree (`manifest.Skills == "plugin/skills"`) gives false confidence about the in-repo `.github/skills` mirror agents actually load: it passes before and after any `.github` edit and provides no red phase. Rule: guard mirrored assets with a targeted test that reads BOTH paths and asserts FULL frontmatter parity (every key AND value via map equality), not one-key presence; keep the broad structural test as a secondary regression gate for its own scope only. Cites `github_skill_parity_test.go` + `plugin_manifest_test.go:105,129`. |
| `docs/compound/2026-07-13-copilot-review-loop-convergence.md` | keep | Still accurate and directly exercised again in the 093-S feature PR #237 (one Copilot thread raised on push, batched-fix + resolve + verified-converged). No wording drift; frontmatter received the `chunk_strategy`/`schema_version` backfill *as part of 093-S itself* (104.002-T). No further change. |
| `docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md` | keep | Reinforced by this very closure: built `bin/backlogit-closure.exe` from merged HEAD `647263c`, verified embedded `vcs.revision`, and confirmed the archived members carry canonical UTC `Z` `updated_at`. Guidance held exactly; no edit needed. |
| `docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md` | keep | 093-S is the FIRST shipment shipped with the merged UTC-normalized writer. Verified end-to-end: all 5 archived members + all 3 new stash entries carry `…Z` timestamps. Confirms, does not contradict. No edit. |
| `docs/compound/2026-07-13-parallel-test-safe-tz-subprocess-red-phase.md` | keep | Unrelated domain (TZ subprocess red-phase); untouched by 093-S. Still accurate. |

## Stale / low-signal review

No existing compound entries were invalidated or made stale by 093-S. The four
recent `2026-07-13` entries are all reinforced or unaffected. No deletions,
consolidations, or archival needed.

## Files touched

* Created: `docs/compound/2026-07-14-github-plugin-skill-parity-test-gap.md`

Passes `backlogit docs lint` (authoring profile, 0 findings).

## Recommendation

PROCEED — compound library is consistent and current for the skills-parity /
TDD-red-phase, PR-automation, and post-merge-lifecycle domains. The new
parity-test-gap learning fills a genuine coverage-boundary gap; the three
operator-requested stash follow-ups (`8CD8F46A`, `CA877CD1`, `A4BE2FAD`) carry
the remaining governance/regression-guard work forward.

---
type: memory
timestamp: 2026-08-01T02:40:00Z
agent: ship
mode: DARK_MODE_ACTIVE
shipment: 115-S
feature: 133-F
branch: feat/133-shipshipment-cascade-fix
reviewed_head: 1dfa8a492f9ee0277ea2b278d3f8572b9e901d5b
---

# Ship 115-S — Local Review Readiness (Step 4.4)

## Reviewed HEAD

`1dfa8a492f9ee0277ea2b278d3f8572b9e901d5b` (8 commits ahead of `origin/main`
at merge-base `35bcc9dc35b2e4079908ed102d3ca081359101a7`, the staging PR #326
merge commit).

Commits reviewed (oldest to newest):

1. `b64264a2` chore: record dark-mode pipeline-drain activation memory
2. `b6059b96` chore(harness): claim shipment 115-S and its tasks
3. `6773e8dc` test(core): align ship tests with explicit membership contract
4. `5b2bd846` fix(core): neutralize covering-feature archive seams on partial ships
5. `0f3a959f` fix(tests): flip shipment integration test to membership contract
6. `4c188c53` feat(core): add check-only doctor audit for over-archived features
7. `c444d5ae` chore(harness): retire P-015 manual workaround for membership-gated ships
8. `1dfa8a49` fix(core): stop restore from reverting nested feature archived by same ship (review-fix)

## Personas spawned (7/7 complete)

| Persona | Findings | P0 | P1 | P2 | P3 | Outcome |
|---|---|---|---|---|---|---|
| Constitution Reviewer | 11 | 0 | 1 | ~5 | ~5 | P1 fixed (CLI test added, `1dfa8a49`) |
| Go Reviewer | 12 | 0 | 4 | ~5 | ~3 | P1 headline (nested-feature corruption) fixed (`1dfa8a49`); independently verified against source before accepting |
| Learnings Researcher | n/a (informational) | — | — | — | — | Cross-referenced 70 compound docs; recommends updating (not consolidating) the P-015 compound learning doc post-merge |
| Architecture Strategist | 7 | 0 | 1 | 4 | 2 | P1 = independently-derived restatement of the same nested-feature corruption bug; fixed together with Go Reviewer's finding |
| Scope Boundary Auditor | 4 | 0 | 0 | 1 | 3 | P2 (doctor audit membership model narrower than ship-time fix) deferred as follow-up; rest accept-as-is |
| Agent-Native Parity Reviewer | 5 | 0 | 1→2* | 3 | 1 | *Reclassified P1→P2 after verification (see below); all deferred as follow-ups |
| Template Integrity Reviewer | 3 | 0 | 0 | 1 | 2 | All pre-existing/unrelated drift discovered incidentally; deferred |

## The confirmed, fixed P0/P1: nested-feature archival corruption

Go Reviewer and Architecture Strategist independently derived the same bug
from different angles. Independently verified against source
(`shipment_lifecycle.go`): a feature nested under an explicit-member root
feature (reachable via `AdoptItem` re-parenting — confirmed NOT gated by
`AllowedChildren`, unlike `CreateArtifact`; confirmed reachable in this exact
repo via pre-existing `.backlogit/archive/013.001-F.md` / `013.002-F.md`) is
captured as "non-member" by `featureScopeRoots`' upward walk, but is legitimately
swept into the real archive scope and archived by `collectArchiveCandidateIDs`
as a genuine descendant of the explicit-member root. The deferred
`restoreRolledUpNonMemberFeatures` then incorrectly reverted this legitimate
archival, corrupting `archived_from`/`archived_status` without reversing the
stash-archival side effect.

**TDD proof**: wrote
`TestShipShipment_LegitimatelyArchivesNestedFeatureDescendantOfMember`,
confirmed RED against pre-fix code (nested feature ended up `status: active`
back in `queue/` instead of staying `archived`), then implemented the fix
(hoist `archivedIDs` before the `defer`, thread it into
`restoreRolledUpNonMemberFeatures` as an exclusion set, skip restore for any
ID confirmed genuinely archived this call) and confirmed GREEN. Full suite
(`go test ./...`) green with no regressions. `go build`, `go vet`, targeted
`golangci-lint run` on touched packages all clean. `gofmt -l` flags the two
touched core files, but this is the pre-existing repo-wide CRLF artifact
(confirmed: both files are 100% CRLF internally, `git diff --stat` is clean
and surgical — not new drift). Committed as `1dfa8a49`.

## Reclassified finding: Agent-Native Parity Reviewer's MCP doctor-exposure P1 → P2

The reviewer correctly caught that my own earlier "3 of 5 doctor checks are
already CLI-only, so `CheckOverArchivedFeatures` being CLI-only matches
precedent" claim was imprecise: `CheckDuplicates` **is** MCP-wired (confirmed
in `internal/mcp/tools.go`'s `handleDoctor`, params `check_orphans` /
`check_duplicates` / `fix_orphans` / `target`). Only `CheckArchivedFrom` and
`CheckGateEvidence` are the true CLI-only precedents, and both carry their own
doc comments in `doctor.go` explicitly saying "safe to expose on MCP" —
meaning this is pre-existing, acknowledged tool-surface debt, not a settled
architectural boundary.

However, task `133.005-T`'s own text was re-read verbatim and confirmed: it
scopes CLI-only-ness explicitly to the **deferred, out-of-scope destructive
`--fix` remediation** ("MUST be CLI-only... per Constitution VII"), and says
nothing about the read-only check needing to be CLI-only or MCP-exposed. The
task's acceptance criteria only require the check to work correctly via the
CLI with a passing unit test — which it does. Reclassified P1→P2 because:
this is a real but pre-existing, task-silent, tool-surface-completeness gap
(not a defect in what 133.005-T asked for or what was delivered), and fixing
it would require expanding the diff into a different tool's MCP schema/tests
— appropriately a separate, single-skill-domain follow-up task, not a
review-fix-cycle patch to this bug-fix feature.

## Local Review Readiness: READY_WITH_FOLLOWUPS

Zero unresolved P0/P1 findings at reviewed HEAD `1dfa8a49`. Follow-ups
requiring explicit tracking (recorded here per dark-mode's "follow-ups may be
recorded only as Ship's required closure output and reported to Orchestrator"
constraint — none created as new backlog/stash items during this session):

1. **[P2]** `doctor --check-over-archived-features`'s membership-legitimacy
   model (`scanShipmentManifestFeatureIDs`) is narrower than the ship-time fix's
   model — it only recognizes literal manifest membership, not "genuine
   descendant of an explicit-member root." Could produce a false positive for
   a nested feature that was legitimately archived (mirroring the exact class
   of false-positive the ship-time fix just closed). (Scope Boundary Auditor)
2. **[P2]** Wire `CheckOverArchivedFeatures` (and, since already marked "safe
   to expose," `CheckArchivedFrom`/`CheckGateEvidence`) through to the
   `backlogit_doctor` MCP tool schema/handler, so MCP-only agents have parity
   with CLI operators for detecting this bug class. (Agent-Native Parity
   Reviewer, reclassified P1→P2 — see above)
3. **[P2]** Add a `RestoredFeatureIDs` (or similar) field to
   `ShipShipmentResult` so both CLI and MCP callers get a first-class signal
   when the non-member-feature restore mechanism actually fires, instead of
   it being silent on success. (Agent-Native Parity Reviewer)
4. **[P2]** `backlogit_ship_shipment`'s MCP tool description and the CLI
   `shipment ship` help text don't state the explicit-manifest-membership
   invariant this fix makes authoritative. Add one sentence to both.
   (Agent-Native Parity Reviewer)
5. **[P2]** `docs/cli-reference/backlogit_doctor.md` (generated via
   `go run ./cmd/gen-docs`) was not regenerated and is missing the
   `--check-over-archived-features` flag. Run doc regeneration before/at next
   docs pass. (Agent-Native Parity Reviewer)
6. **[P3]** No automated contract test enumerates which `DoctorOptions` are
   intentionally CLI-only vs MCP-exposed; the gap can silently grow with each
   new doctor check. (Agent-Native Parity Reviewer)
7. **[P3]** `plugin/agents/ship.agent.md` (140 lines, static since initial
   plugin commit `9b0cbbf3`) is not a content-parity mirror of
   `.github/agents/.ship.agent.md` (742 lines, actively maintained); no parity
   test exists for agent files (unlike skills, which do have one). Pre-existing
   drift, unrelated to this diff. (self-observed during review prep)
8. **[P3]** Minor unrelated pre-existing doc drift found incidentally while
   verifying cross-references: a mis-resolved template variable one line above
   the actually-edited Step 6.1.b in `.ship.agent.md`; a stale version banner
   in `workflow-policies.md` vs its Amendment Log; a missing-leading-dot path
   typo in `.github/skills/shipment-reconcile/SKILL.md`'s Related Artifacts
   list. All confirmed pre-existing and outside this diff's edit surface.
   (Template Integrity Reviewer)
9. **[Tier 1 compound]** Learnings Researcher recommends updating (not
   consolidating) the existing P-015 compound learning doc to reference this
   fix once merged — feed into `compound-refresh` at post-merge closure.

None of the above block PR creation or merge. All are recorded here as
closure-output follow-ups per the dark-mode contract; no new backlog/stash
items were created during this session.

## Next steps

Proceed to Step 5 PR Lifecycle: `pr-lifecycle` skill, Copilot review (advisory
per dark-mode contract), CI polling, §1.9 pre-merge readiness gate, merge-commit
strategy only.

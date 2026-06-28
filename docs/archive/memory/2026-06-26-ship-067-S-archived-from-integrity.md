# Ship Session — 067-S ArchiveItem archived_from Integrity

- **Date**: 2026-06-26
- **Agent**: Ship
- **Shipment**: 067-S (active) — Feature 067-F
- **Branch**: `feat/067-archived-from-integrity` (off `main` @ f2a183aa)
- **Operator decisions (all confirmed)**: Option B (doctor audit + `--fix-archived-from`); read-time self-heal; malformed `done` records flag-only; migration ships inside 067-S.

## Live census (verified, matches plan)
- 130 self-referential `archived_from`, 259 canonical (258 + legit 036-DL), 211 fieldless, 2 malformed (`done`). Total archive `.md` = 602.

## Progress
- [x] **U1 / 067.001-T** done — `canonicalRestorePath` resolver + `isUnsafeRootDir` (commit 64623bf). In-package test `archive_internal_test.go`.
- [x] **U2 / 067.002-T** done — ArchiveItem stamps canonical queue path for pre-archived items (commit 1e01616). Contract test archive_test.go:70 stays green.
- [x] **U3 / 067.003-T** done — UnarchiveItem read-time self-heal + round-trip/re-archive-stability tests (commit 91606cc). Repurposed prior same-path test.
- [x] **U4 / 067.004-T** done — doctor self-ref + malformed detection (commit c6a3877).
- [x] **U5 / 067.005-T** done — doctor `--fix-archived-from` repair (commit 4816fee). Inline body-preserving codec in core/doctor.go (docline→core import cycle blocks reuse). Symlink-safe, idempotent, atomic writes.
- [x] **U6 / 067.006-T** done — CLI-only `--check-archived-from`/`--fix-archived-from` flags (commit 7ef7c63). MCP handler NOT wired (Principle VII).
- [x] **U7 / 067.007-T** done — closure runbook docs/closure/2026-06-26-archived-from-migration-closure.md (commit d26ff0c). docs lint pass.

## Backlog state + migration
- [x] Backlog: all 7 tasks done/archived (commit be57bf32). 067-S/067-F remain ACTIVE (post-merge closure).
- [x] **Live migration** (commit e30309eb): `doctor --fix-archived-from` rewrote 130 self-ref → canonical. Post-scan 0 self-ref; 2 malformed flagged-only. Byte-clean (260 lines = 130 archived_from pairs, no body/other-field/key-order changes). Idempotent (2nd run 0 repaired, byte-stable).
- [x] CLI ref drift fix (commit 81f79469): regenerated docs/cli-reference/backlogit_doctor.md.

## PR
- **PR #141**: https://github.com/softwaresalt/backlogit/pull/141 (base main, merge-commit per P-009; squash/rebase disabled repo-wide).
- Final HEAD: `9fc0d86d`. CI all 4 green on a643faf8 (CLI Reference Drift, Docline gate, test 1.23, test 1.24); re-running on 9fc0d86d (stash-only, no code).
- Gates all green pre-push: go test ./..., go vet, golangci-lint, gofmt (LF-norm changed files), docs lint.
- Review gate (code-review agent): no P0/P1; all 5 correctness reqs verified.

## Copilot review cycles (all resolved — 0 unresolved threads)
- Cycle 1 (3b1cdb10): atomicWriteArchiveFile Windows rename → remove+retry. 1 thread resolved.
- Cycle 2 (d63c4471): 3 threads — atomicWriteArchiveFile (already fixed), FixArchivedFrom silently skips w/o CheckArchivedFrom (added explicit error + test), ArchiveItem rename (remove+retry). Resolved.
- Cycle 3 (a643faf8): 3 threads (comment/style) — repairArchivedFrom symlink-refusal comment, canonicalRestorePath rejection-set comment, atomicWriteArchiveFile `_ =` vs nolint. Resolved.
- Cycle 4 (f00320ba): 2 threads — (1) REAL data-safety bug: UnarchiveItem read-time self-heal + atomic rename would clobber a distinct pre-existing queue file on POSIX / fail on Windows → added pre-rename refuse-guard + TestUnarchiveItem_RefusesToClobberExistingQueueFile; (2) remaining nolint:errcheck → `_ =` in archive.go. Resolved.
- Total: 9 threads across 4 cycles, 0 unresolved. NOTE: exceeded nominal 3-cycle circuit-breaker by one cycle — justified because cycle 4 surfaced a genuine net-new data-safety defect introduced by the U3 self-heal (not loop thrash). Findings converged.
- **Final fresh Copilot review 08:40:18Z covers current HEAD f00320ba with ZERO new findings.** Merge gate satisfied.

## Final HEAD: f00320ba. CI all 4 green (CLI Reference Drift, Docline gate, test 1.23, test 1.24).
- Gates all green: go test ./... (all pkgs), go vet (clean), golangci-lint --timeout 5m (exit 0), gofmt (LF-norm changed files), docs lint.
- Review gate (code-review agent): no P0/P1; all 5 correctness reqs verified.

## Follow-up stashed (for Stage)
- [x] Stash `8863C6C8` (commit 9fc0d86d): Extract shared body-preserving frontmatter codec + atomic-write helper to a leaf package (remove docline/core duplication from import-cycle workaround).

## STATUS: MERGE-READY — HALTED for operator merge approval. Do NOT merge. Do NOT ship 067-S (separate post-merge closure step: ship 067-S + archive 067-F + tasks after PR #141 merges).

## Key code facts
- archive.go:181 → branch on `currentPath == archivePath`.
- UnarchiveItem self-heal inserted after `resolveWorkspacePath`, before F-006 guard.
- docline codec: `docline.Decode(raw)` / `(*Markdown).Encode()` — body-preserving, sorted-key frontmatter. Use for U5 repair.
- doctor.go: `DoctorFinding`, `FixAction`, `DoctorOptions`, `Doctor()`. fix-orphans continue-on-error pattern at doctor.go:211-221.
- CLI doctor: `internal/cli/doctor.go`. MCP doctor: `internal/mcp/tools.go:1775 handleDoctor` — must NOT wire the repair.
- Windows: working tree CRLF, `core.autocrlf=true`, git stores LF → gofmt CRLF noise is benign; verify via LF-normalized copy.

## Gates run per task: go test ./internal/core/, gofmt (LF-normalized), go vet. Full suite + lint at PR.

## Next: implement U4 detection (test-first), then U5 repair, U6 CLI, U7 docs. Then full gates, run live `doctor` dry-run (expect 130+2), commit `--fix` as isolated commit, post-scan 0 self-ref, PR.

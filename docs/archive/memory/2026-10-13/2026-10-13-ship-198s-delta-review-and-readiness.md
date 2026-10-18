# 198-S delta review round 2, build evidence, and readiness halt (Ship)

Recorded: 2026-10-13T05:55Z (UTC). Shipment 198-S (CT: Restore test-suite health and runtime diagnostics). Feature 198-F. Tasks 198.001-T through 198.005-T. Branch `feat/ct-restore-test-suite-health-and-runtime-diagnostics`. Dark-mode scope: 198-S only. Admin fallback not authorized.

> **Superseded in part (2026-10-13, round 3).** The following corrections in this memo are withdrawn or replaced by `docs/memory/2026-10-13-ship-198s-round3-review-halt.md`, section 8: (a) the section 2 bucket that lists C-4 (D-4) and P-1 (D-6) under "Operator-decided" (D-4 and D-6 are the Orchestrator's dark-mode calls, O1); (b) the C-3 "controls independently" wording; (c) the M-2 timing, which was not disclosed; (d) the D-7 file-list source, which is the pre-PR halt memo item 7, not the wave-1 halt memo; (e) the wave-2 command, which included `-timeout=30m`; (f) the round-2 reviewer summary; (g) the race row label "CI-parity".

## 1. Status

Outcome: HALTED. Pull request not opened. Readiness block not issued. The halt needs operator decisions listed in section 5.

Heads at the time of writing:

* Branch HEAD before this memo's commits: `a5076d48` (Stage errata and stash wording correction).
* Stage docs: `655156c1` (three planning docs). Backlog errata: `a5076d48`.
* Tasks 198.001-T through 198.005-T are `done` and archived. Shipment 198-S remains `active`. Nothing has been merged, and no other shipment has been touched.

## 2. Delta review round 2 (adversarial, multi-persona)

Scope: the diff `3c76a544..HEAD` (Stage's final plan, decision note, and memo edits, the errata, and the operator confirmations), plus the mitigation re-check of the A1.5 dispatch constraints and the T4/T5 bullets, plus the re-confirmation of the earlier pre-PR BLOCKED findings. The reviewers had view-only access and no git. Per the tool-degradation rule, the record declares `TOOL_DEGRADED: git-evidence`. Full per-finding dispositions are in `logs/ship-198s/prior-findings-dispositions.md` (ignored path, kept as working evidence).

Result: the adversarial gate returned BLOCKED (one P0 and several P1 items). The constitution review returned FAIL on P1 items. The scope, correctness, maintainability, security, and Go-style reviewers returned ADVISORY.

Disposition summary:

* Operator-decided (D-1 to D-8, operator reply 2026-10-13T03:26:01Z): C-01 to C-06 constitution P1s; C-1 (Principle II departure, D-2, limited to 198.001-T and 198.003-T); C-2 (D-3); C-4 (D-4, Orchestrator call with Erratum A1.8 E-1); M-1 (D-1 ADVISORY); M-3 (D-8); P-1 (D-6, cite stash 63909DAE, no new capture); P-3 (D-5, `%TEMP%\stage-198s` already absent at 03:39Z, recorded as a violation); U-1 (D-7, disclosed P-005 event).
* Fixed by Ship: M-2 (the A1.5 step 2 CT-scope-file check is now run and recorded on 198.005-T, zero overlap, not CT-scope). The T5 artifact annotation (section 3) supersedes the "isolation fixes held" sentence and the "CT-scope check still to be run" sentence.
* Fixed by Stage (errata A1.8 and archived bodies): P-4 (E-2), P-8 (E-6, stash 253E2505 wording), U-8 (archived 198.002-T Files list), U-9 (E-7/E-8).
* Captured under P-021 C2 (out of scope, Stage-owned text, not edited by Ship under P-010): stash 7802D798 (medium, Stage governance and plan text) and stash 901C79CC (low, record-accuracy corrections). Earlier captures carried forward: FBD0BB95, 253E2505, 8640432F (fourth slog site), 5A6F54BB (T1 relaxed assertion), 63909DAE (flake), D8EF5443 (archived parent for 8640432F).
* Residual risk, disclosed and not fixed: C-3 (mutation-kill executor provenance under R5; the executor model is not exposed; the PR annotation asks for an operator spot-check of the FAIL lines).

Ship's corrections to its own records:

* D-4 and D-6 are the Orchestrator's in-scope calls, not operator decisions. The operator reply omits them. Stage's stash 7802D798 item (a) and stash 901C79CC item (c) cover the attribution fix in Stage-owned text.
* Decision IDs in the Ship memo (D1 to D7) do not match the operator's IDs (D-1 to D-8, MNT-15). This memo uses the operator's IDs.
* Commit scopes: several branch commits use scopes outside the commit-message instruction list (`chore(backlogit)`, `docs(decisions)`, `docs(closure)`, `docs(memory)`). History is not rewritten. The PR body will declare this as a waiver. New commits in this memo's batch use `docs(docs)` and `chore(harness)`.
* D-7 file list: the wave-1 scaffold subagent changed `internal/core/gate_evidence_formal.go` and `internal/release/release.go` in the working tree during scaffolding. The production change was reverted, and HEAD production code is identical to the base. The R6 mutation site was `gate_transition.go`, on ignored copies, through `go test -overlay`. The P-005 telemetry for D-7 is recorded with this file list.

## 3. T5 artifact annotation (Ship-owned annotation)

`docs/closure/198-S-ct-full-suite-green-evidence.md` gained a "Ship annotation" section, committed in this batch. It states that the run is GREEN-AFTER-RERUN, not a clean green, that the "isolation fixes held" statement does not apply, that the CT-scope check was run (zero overlap with the branch diff against base `c5758918`), and that the first-run failure of `TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog` recurs the flake tracked by stash 63909DAE. The gated scalars are unchanged, including the literal `CT_ISOLATION_FIXES_HELD` line the gate requires. The T5 exempt gate was re-run after the annotation and passes.

## 4. Local build and quality evidence (current tree, HEAD `a5076d48`)

| Check | Command | Result | Evidence |
|---|---|---|---|
| Build | `go build ./cmd/backlogit` (output to ignored path) | exit 0 | `logs/ship-198s/prebpr-evidence/go-build.txt` |
| Vet | `go vet ./...` | exit 0 | `logs/ship-198s/prebpr-evidence/go-vet.txt` |
| Lint | `golangci-lint run` (v1.64.8, the CI pin) | exit 0 | `logs/ship-198s/prebpr-evidence/golangci-lint.txt` |
| Format | committed-blob gofmt over `git ls-files '*.go'` (896 files) | 0 unformatted | not written to disk; the check ran against `git show HEAD:<path>` |
| Race (CI-parity) | `go test -race -count=1 -timeout=40m -coverprofile=<ignored path> ./...` at HEAD `3c76a544`; no Go change since | exit 0, 40 of 40 packages ok, 0 races | `logs/ship-198s/race-ci.log`, `race-ci.meta.txt` |
| Unfiltered suite | `go test -count=1 ./...` at `d3aff2c5` (wave 2) | exit 0, 40 ok, 0 FAIL, internal/core 1009.8 s | `logs/ship-198s/w2-full-suite.log` |

Parity caveats for the readiness block:

* CI's literal form uses the default 10-minute per-package timeout. internal/core takes about 1565 s under race on this Windows host, so the run used `-timeout=40m`. The CI timeout is not reproducible here.
* Local toolchain is go1.26.5 (race run). CI pins go 1.24. The vet and lint runs above used the local toolchain with the CI-pinned linter.
* The committed-blob gofmt check replaces the Windows working-copy gofmt, which reports CRLF false positives.
* The first-run failure of TestU172 in the T5 run is the flake tracked by 63909DAE. It is outside CT scope and was not fixed.

## 5. Operator decisions needed before the PR can open

These are the items that only the operator can decide. Each one is minimal and has a stated default.

* **O1 (attribution)**: Confirm that D-4 (exact-name selector for the 198.002-T AC2 disposition, with Erratum A1.8 E-1) and D-6 (T5 stays GREEN-AFTER-RERUN, cite 63909DAE) are the Orchestrator's in-scope calls, recorded as such. Stage then corrects the A1.8 intro and the final Plan Review `approval_basis` (stash 7802D798 (a), 901C79CC (c)). Default: yes.
* **O2 (P-010 event)**: Stage ran linters, `backlogit docs lint`, `go list`, and pwsh fixtures during its own first revision. Accept this as a disclosed P-010 event without re-execution, or require re-execution of those checks. Default: accept as disclosed. The operator's D-7 reply covers the in-tree mutation, not these Stage runs.
* **O3 (A1.8 errata)**: The A1.8 errata were reviewed in round 2 but not by the operator. Accept them as residual risk, or require operator review before the PR. Default: accept as residual risk, with the Stage correction in O1 applied first.
* **O4 (T5 literal)**: The gate requires the scalar `CT_ISOLATION_FIXES_HELD` in the T5 artifact, which encodes the key. Keep it with the E-8 disclosure (the position in the Ship annotation), or direct a reviewed T5 contract amendment. Default: keep with disclosure.
* **O5 (optional)**: The mutation-kill executor (R5) cannot be attested by model. Accept the disclosed gap, or request an operator spot-check of the `--- FAIL:` lines in the T1 and T3 mutation runs. Default: accept with the PR annotation.

## 6. Next steps once the operator answers

1. Apply the Stage corrections for O1 and O3 (Stage-owned; Ship does not edit plan text under P-010).
2. Run a round-3 delta review on the files changed since `a5076d48`, scoped to the changed files.
3. Issue the Local Review Readiness block for the then-current HEAD (`github-pr-automation.instructions.md` §1.9.2), with the evidence in section 4 and this memo's commit list.
4. Open the PR to `main` (merge-commit only; the commit-scope waiver in section 2 goes in the body). Request Copilot review (P-018, `--max-wait`, no `--force`). Run CI on the current HEAD. Run the §1.9 gate and the P-009 check. Merge only when every gate passes, with no `--admin`.
5. Post-merge closure as specified in the Ship agent, with `docs/closure/198-S-198-F-post-merge-closure.md`.

Pending items carried into the PR body and the closure capture list: FU-1 to FU-9 from the spike, 63909DAE, 5A6F54BB, 8640432F, 253E2505, 7802D798, 901C79CC, FBD0BB95, the GO-01/GO-02 durability observations, and the C-3 residual risk.

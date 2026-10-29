A3_RULING: A
A3_RESUME_CHECKPOINT: checkpoint-20261006-021519.json
A3_R02_RATIFIED: yes

# 196-S Amendment 3 Ruling Record

## Ruling

The operator selected A3.4 Option A: a one-time `done -> queued` reopen of
196.001-T and 196.003-T via a temporary `.backlogit/hooks.yaml` transition
override. Spike 002-SP (stash DB071B5D) tracks the long-term reopen policy.

## Delegation

On 2026-10-06 the operator said: "I want YOU to run the script... I will
relaunch start.ps1." Under that explicit delegation the Orchestrator ran
Option A steps 1-10 instead of the operator. No script file was used; each
step ran as a separate, verified command sequence.

## Execution Evidence

| Field | Value |
|---|---|
| CLI binary | `.\bin\backlogit.exe` commit `7c805f9baae7f74edd2b1eede47fcf35fbbc9066` |
| MCP binary | commit `7c805f9baae7f74edd2b1eede47fcf35fbbc9066` (parity confirmed) |
| R14 attestation | MCP storage_root `C:\Source\GitHub\backlogit\.backlogit`; one attached DB `.backlogit\backlogit.db` |
| Snapshot 196.001-T | SHA-256 `75B2A042A9E032B91128D79208289E548CA3345EEA7943AE364AF88006AB93CD` |
| Snapshot 196.003-T | SHA-256 `7E4CF0C8CC31CFAB51324C0A7AC963DEC8A5A95D77D727AC5446E5B4380DC752` |
| hooks.yaml SHA-256 | `121124A2B5874D2904A506F8A0878A3C103E32E8D0C45FE3EF2BBC6AE8E6C7B0` |
| Window start | 2026-10-06T06:54:58.0517337Z |
| Window end | 2026-10-06T06:56:24.2943316Z |
| Moved items | 196.001-T, 196.003-T (`done` to `queued`, archive to queue) |
| Reopen commit | `7bc7432587bc726a8fb46a8d5ad05344427d786a` |

## Verification

* Step 6: `.backlogit/hooks.yaml` absent after the window; git status empty for it.
* Step 7: sync exit 0 (1896 artifacts). CLI, raw file, and MCP all report
  `queued`; no archive copy remains.
* Step 8: staged set held exactly two `R` rows (archive to queue) and one `M`
  row on `.backlogit/hooks_queue.jsonl` (append-only: 2 added, 0 deleted).
  Snapshot diffs changed only `status` and `updated_at`.
* Step 9: `REOPENED` comments appended to both tasks with actor `operator`
  and commit SHA `7bc74325`.

## Deviations

* The pristine `hooks.yaml` was generated with `backlogit init` inside the
  gitignored `logs\optionA\inittmp` directory rather than outside the
  repository, honoring CLI workspace containment. The only edit added
  `- queued` under the `done:` transition row; the diff against pristine
  showed exactly that one line.
* Line 1 must be exactly `A3_RULING: A`, so MD041 is disabled for this file
  by the trailing directive below.

<!-- markdownlint-disable-file MD041 -->

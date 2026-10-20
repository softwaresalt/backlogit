# Ship 196-S Task-Start Verification Halt

## Halt

- Token: `TASK_START_NOT_RECORDED`.
- Shipment: `196-S`; task: `196.001-T`.
- Branch: `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`.
- HEAD at halt: `9791c9de24fcb65c77b0610b0bcfe2463af40602`.
- The shipment remains the only active shipment. No task build-feature dispatch or implementation was started. `196.001-T` was already active and claim-assigned from the shipment claim; Ship did not move its status.

## Evidence and exact stop point

1. Wave-1 harness generation completed and was committed:
   - `413dff42` — `test(harness): scaffold 196-S wave 1 red contracts`
   - `b34ae1c9` — `chore(harness): link 196-S wave-1 harness evidence`
   - `14e987ea` and `9791c9de` — Ship memory checkpoints.
2. The three red-deliverable harnesses compile. Their exact task selectors are assertion-red, with the preserved subtests passing as recorded in their P-004 manifest comments. `196.001-T`, `196.003-T`, and `196.008-T` carry `harness-ready`. The valid wave-1 exemptions `196.004-T` and `196.007-T` remain unscaffolded; neither claim-time pre-work probe has run yet.
3. Before the U1 claim-time log read, the R14 per-task `pragma_database_list` query returned exactly one `main` row at `C:\Source\GitHub\backlogit\.backlogit\backlogit.db`.
4. A scoped P-012 declaration was emitted for read-only no-follow access to `.backlogit/logs/196.001-T.jsonl`. The pre-append open/read passed path containment, reparse-point, no-follow, and opened-path checks. It parsed 9 events, found one shipment-claim event (latest claim event index 2), and found 0 valid current-epoch `WORK_STARTED: 196-S` records.
5. `backlogit_append_comment` returned `ok` for actor `ship`, comment exactly `WORK_STARTED: 196-S`.
6. The mandatory post-append verification then failed **before reading the file**: the local PowerShell P/Invoke helper used an incorrect `GetFileInformationByHandle` entry-point declaration, producing `Unable to find an entry point named 'G' in DLL 'kernel32.dll'.` Consequently, the post-append valid-record count is unknown. Ship did not retry the append, dispatch build-feature, or continue to any later task action. The task-start gate requires a successful no-follow post-append read proving exactly one valid record; the append result alone is not sufficient.
7. The working tree was clean at halt. The operator-local `.github/copilot/` paths and `.git/info/exclude` were not touched. P-005 halt telemetry was recorded.

## Resume requirements

After explicit checkpoint selection and confirmation, remain on this feature branch and shipment only. Before any U1 raw log read, repeat the required R14 `pragma_database_list` check. Use a corrected no-follow reader with verified `GetFileInformationByHandle` entry point and opened-path identity to inspect the latest U1 claim epoch. Do **not** append another start comment automatically. Continue only if the successfully verified log contains exactly one `WORK_STARTED: 196-S` record in the current epoch; otherwise halt for operator recovery without deleting or rewriting any log line.

The pre-halt U1 red baseline `9791c9de24fcb65c77b0610b0bcfe2463af40602` is historical only: the halt memory/checkpoint commits advanced `HEAD` afterward, and build-feature's zero-delta gate compares all changed paths. After explicit resume confirmation and successful verification of exactly one U1 start record, verify the tree is clean and recapture `red_baseline_sha` from the then-current `HEAD` before dispatch. No task implementation has occurred, so this later baseline still contains the committed wave harnesses but excludes the halt-record commits from the task delta. Do not use the historical value merely because it is recorded in the halt checkpoint.

U4/U7 still require their exact P-002.5-screened must-fail pre-work probes at their own claim-time gates. No wave member is done and no PR has been created.

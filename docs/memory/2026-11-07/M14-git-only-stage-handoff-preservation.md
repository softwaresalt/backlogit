---
title: M14 Git-only Stage handoff preservation receipt
date: 2026-11-07
created_at: 2026-11-07T22:02:53Z
---

## Provenance

* Source HEAD: `183ec2d52287a6465e4ef2e2952685f0ba0d7eb0`
* Packet: `.copilot/session-state/4a60e334-a29e-4db4-9f3a-be1e5f382920/files/m14-stage-handoff.json`
* Packet SHA256: `6d46ea337fe886724008e757cbf26183ac0c7063bcb183718f94632d949a6af5`
* Custody recorded in packet: Stage released to Ship; Stage idle
* Packet records actual Stage review as final PASS with findings resolved
* Packet records operator Option 1 authorization at `2026-11-07T21:24:02Z`
* These are provenance facts, not new gate verdicts or policy judgments
* Scope: one Git-only commit on the existing feature branch
* No source, tests, config, plan, or Stage artifact working bytes changed
* No Go, FullVerify, live backlog, disposition, checkpoint, push, PR, or merge operations

## Preservation baseline

The baseline covers the union of tracked and nonignored untracked paths,
including absent tracked files. Each entry is a path and SHA256, or `ABSENT`.
The digest is SHA256 of the sorted inventory serialized as compact UTF-8 JSON.
This new receipt is excluded from the preserved inventory.

* Original inventory entries: `5751`
* Original inventory SHA256: `e8d2c635cd3342fbf37d20301cf5bdf96ec1974bbfa7ab55e8becd9e694d5d78`
* Unrelated porcelain-v1 status entries: `71`
* Unrelated status SHA256: `e972e28741dc530a5b6a991967b5edc4816cdda8750e3acc0219ee53be67dd2e`
* Status digest uses sorted entries, excluding the 13 packet paths and this receipt
* Preserved checkpoint: `.backlogit/checkpoints/checkpoint-20261106-210317.json`
* Checkpoint SHA256: `235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`
* Initial Git index: empty
* All 13 Stage working hashes matched the packet before staging

## Shared stash index selection

Rows were parsed as JSON solely to establish unique exact `id` identities.
Every selected row retains its complete working line bytes, including newline.
No stash working bytes were edited.

* `96F1DFFD`: unchanged complete line already present in HEAD
* `96F1DFFD` line SHA256: `6a9dbd1b475c922f82fcf1e08ed4150ba6e05cea12e4122c791a99119fa96151`
* `F2509AE3`: append complete working line, absent from HEAD
* `F2509AE3` line SHA256: `b3e37546fa739f303a67d09657bf47d64f85b5489fe184d42e2a21386a293529`
* Every other row retains its HEAD baseline bytes and ordering
* Root captures `A47CD72B`, `264ACCED`, `A467B030`, `F0973A97` remain unstaged
* Partial stash blob is written with Git hash-object without filters
* Only that blob is installed in the stash index entry using Git update-index
* HEAD stash SHA256: `7f4df9a11a433918bfacad6cf0da681f50f9dafe928068a906f559306560656d`
* Partial index stash SHA256: `e49a937b3e53ba35cfed43c75c532100220c9a34f18b69efb0d264e50f853186`
* Unchanged working stash SHA256: `99391bab4afd1e5b4b6b76eacf199f7b618cd9b16d9877c258daf322422da592`

## Exact commit path allowlist

```text
.backlogit/archive/089-DL.md
.backlogit/queue/211.013-T.md
.backlogit/queue/211.014-T.md
.backlogit/queue/211.003-T.md
.backlogit/queue/211-F.md
.backlogit/queue/212-S.md
.backlogit/stash.jsonl
docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md
docs/decisions/2026-11-07-212-s-delivery-blockers-rca.md
docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md
docs/compound/test-failures/windows-host-hibernation-during-supervised-test-runs-2026-11-07.md
docs/memory/2026-11-07/212-S-rca-handoff.md
docs/memory/2026-11-07/stage-212-s-089-dl-u1gh-amendment.md
docs/memory/2026-11-07/M14-git-only-stage-handoff-preservation.md
```

## Verification contract

Immediately before staging, reverify source HEAD, packet, all 13 artifact
hashes, checkpoint, original inventory, unrelated status, and empty index.
After staging and after the single commit, require identical artifact and
preservation digests, exact allowlisted changed paths, and the proven partial
stash blob. Terminal verification output supplies the resulting commit SHA.
No policy gates are changed or reinterpreted. No new checkpoint is created.

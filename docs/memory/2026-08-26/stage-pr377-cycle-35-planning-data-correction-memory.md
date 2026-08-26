---
chunk_strategy: h2
description: "PR #377 cycle-35 planning-data correction for two invalid V1 checkpoint agents and an executable current-source checkpoint corpus gate; gate remains FAIL pending independent review"
doc_type: memory
schema_version: "1.0"
source: cycle-35-planning-data-correction-session
title: "PR #377 Cycle 35 Planning Data Correction Memory"
---

**Date**: 2026-08-26
**Agent**: Stage
**Branch**: `chore/stage-130-s`
**PR**: #377
**Cycle**: 35
**Worktree**: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
**HEAD at session start**: `c246eee3189485d77930a45327a1f24d5c1fbb2e`
**Shipment**: `130-S` (queued - not claimed or shipped)
**Scope honored**: planning data, plan gate, feature, checkpoint, and memory artifacts only; no
subagent, push, merge, or Go source

## Baseline

The current-source checkpoint corpus command enumerated 26 files:

* 9 explicitly accepted pre-V1 legacy files
* 15 valid V1-era checkpoints
* 2 invalid V1-era checkpoints

`checkpoint-20260826-064716.json` and `checkpoint-20260826-072421.json` both failed
`ValidateCheckpoint` because they recorded `agent: prompt-builder`, which is outside the accepted
V1 agent vocabulary.

## Correction

Both affected files now record `agent: stage`. Every other key and all context in those files are
byte-for-byte unchanged.

The final cycle-35 Plan Review adds an exact Windows-safe PowerShell gate. It enumerates the live
checkpoint directory, accepts only the nine named pre-V1 files, verifies that those files still do
not declare `schema_version`, and runs the current-source command below for every other file:

```powershell
go run .\cmd\backlogit --cwd . --no-update-check checkpoint get <filename>
```

Any unlisted legacy file, JSON parse error, or V1 `ParseCheckpoint`/`ValidateCheckpoint` error
fails the command.

## Validation

| Gate | Result |
|---|---|
| Current-source checkpoint corpus | 18 V1 valid; 9 explicitly accepted pre-V1; exit 0 |
| Markdown P-008 | 0 issues |
| Docline frontmatter | `valid: true`, 0 violations |
| Index sync | 0 parse failures |
| Topology and live source drift | `WAVE_SIM_OK` 115/115; S=43; M=42; 104 edges; 18 waves; acyclic |
| Go source modified | none |

## Gate state

`cycle: 35`, `decision: FAIL`, `pending: independent-review-required`, `push_allowed: no`;
severity P0=0 / P1=1 (remediated in-pass) / P2=1 (remediated in-pass) / P3=0.

Independent review is required before push. PR-thread reconciliation, shipment claim, and merge
remain blocked. Operator merge approval has not been requested.

---
title: "212-S superseded in-flight convergence narratives"
doc_type: reference
source: docs/memory/compacted/2026-11-11-212-s-superseded-inflight-compacted.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-11"
shipment_id: 212-S
feature_id: 211-F
status: historical
---

## Preserved decisions and limits

Three superseded launch notes record actual unfiltered child starts, not
final PASS. Each consumed its one-run authority. They did not authorize
replay, further failed cycles, timeout extensions or counter resets.
All used cached offline Go1.24, readonly modules and
`go test -count=1 -p=1 -timeout=30m -json ./...`, with 30-second discovery,
2700-second outer and 1800-second test-binary budgets.
They changed no source, task, claim, archive, plan or checkpoint.

| Historical launch | Discovery and authority at capture | Immutable proof location |
| --- | --- | --- |
| Fifth rearm1, d6b23ed3 | Native 0, PID12480, 13.205974s, 50 package lines; four prior failures, M12/six done | `bin/_rr8b31a6d2/fifth-5-rearm1` |
| Sixth, d6b23ed3 | Native 0, PID15212, 28.799291s, 50 package lines; five prior failures, M12/six done | `bin/_rr8b31a6d2/sixth-6-b6d58147` |
| Wave7 rearm1, 183ec2d5 | Native 0, PID5372, 12.311934s, 50 package lines; five full failures plus two prelaunch failures, M12/seven done | `bin/_rr8b31a6d2/wave7-rearm1-183ec2d` |

Discovery stderr was separately retained and empty; no timeout or cleanup
error was observed in those launch receipts. All three final outcomes were
pending at capture. Their later state must not be inferred from launch.
The later [sixth result](../2026-11-07/212-S-sixth-convergence-pass.md)
proves native/wrapper/recorder 0, 46 PASS plus four no-test packages,
3881 top-level passes and five critical RUN/PASS receipts. It retains
five historical failures and is not whole-branch readiness.
Current [handoff](../2026-11-11/212-S-certified-closure-handoff.md)
retains the final historical failure count 15 and the closure approval boundary.

## Hash and cursor traceability

| Input | SHA-256 retained from original note |
| --- | --- |
| Fifth recorder | `8058bd5e1958adc502669dd509811b986fabd4010e3768f429bd2e84b00dc250` |
| Sixth driver | `8b79eda5636e054725e59547ac90a7cb267163f4edca03c65f517370c3a9e4b3` |
| Sixth provenance | `9f4f96691f748436ab3268e02ddb51f873014667a24c8a3f1a6cf4ba5685eb20` |
| Wave7 driver | `8de6d56e44eb18e7b71262385a6e3d702e0b8057fc7200df5f172a6667610a74` |

Protected cursor 210317 remains byte-preserved with SHA-256
`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`.
Parent-owned receipt consumption and original raw backups are not rewritten.
No failed approach is relabeled as success.

## Original narratives

Byte-preserving archive mapping and measured savings are recorded in the
[compaction report](../../closure/2026-11-11-212-s-compaction-report.md).
Original-path redirects preserve citation navigation. Latest result/task
memories, executable/raw proof and active checkpoint pointers remain in place.

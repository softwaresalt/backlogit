---
chunk_strategy: h1-h2-h3
title: "Protect reconstitution's preliminary execution reads with canonical writer locks"
description: "A preliminary plan peek can hold Windows target readers before a globally serialized writer unless execution acquires its canonical locks first."
doc_type: learning
schema_version: "1.0"
source: docs/compound/concurrency-issues/reconstitution-preliminary-reads-under-canonical-execution-locks-2026-11-06.md
problem_type: "concurrency"
category: "concurrency-issues"
component: "reconstitution-execution"
root_cause: "Preliminary planReconstitution artifact reads preceded canonical stash/global/source-membership acquisition"
resolution_type: "code_fix"
severity: "medium"
file_path: "internal/core/shipment_reconstitute_exec.go"
citations:
  - "docs/memory/2026-11-06/stage-212-s-088-dl-u1f-amendment.md"
  - "docs/memory/2026-11-06/212-S-wave1-U1f-harness-manifest.md"
  - "docs/memory/2026-11-06/212-S-fourth-cycle-authorization-and-diagnosis.md"
  - "docs/memory/2026-11-06/212-S-M12-U1f-continuation.md"
tags:
  - "windows"
  - "reader-lifetime"
  - "reconstitution"
  - "canonical-lock-order"
  - "212-S"
---

## Problem and mechanism limits

The execution path performed a preliminary artifact-reading plan before
acquiring the locks that serialize its mutations. That peek could overlap
AssociateCommit's globally locked replacement.

Controlled public-API fixtures proved that both ordinary and explicit
delete-sharing held readers could make the existing durable frontmatter
replacement fail with errno 5 / ErrWriteNotApplied, while a separately
closed-reader condition succeeded. This is compatible with documented
legacy Windows rename semantics; delete sharing alone is not a remedy.
The historical failing reader/handle was not captured.

## Narrow correction

Stage authorized U1f through 088-DL / D12. Pure options, strict YAML, and
in-memory source-ID validation occur before lock acquisition and artifact
I/O. Execution derives sorted validated source IDs and acquires canonical
stash, lifecycle-global, and source-membership protection before the peek.
Preloaded/peek and post-C/B digest comparisons remain enforced.

No global parseFile, preview, atomic API, sharing flag, durability, or
read-bound policy changed. Invalid traversal, reserved global-lock IDs,
empty IDs, and case-folded duplicates fail before locking.

## Verified evidence

Harness commit 644e5ca2 preceded source commit
779f1200990b4dba4f8901492c25865a26c2f192.
The deterministic ordering assertion went RED to GREEN. Three named
subtests passed with zero skips and checked barrier cleanup.
The required broader race/concurrency/zero-write preview command passed
with native exit 0 in 476.713 seconds.
The six-persona scoped review reported no findings.

The operator-approved fourth full run explicitly confirmed all U1f
subtests and the existing concurrent AssociateCommit case PASS.
It nevertheless returned native exit 1 because core failed near its
configured package budget; the recorder did not preserve the panic stack.
The remaining 45 test packages passed. No full-suite or release success,
historical denying-handle attribution, or last-test defect is inferred.

## Prevention and boundaries

* Treat preliminary execution reads as part of the resource lifetime graph
* Keep pure input validation before locks, and preserve canonical lock order
* Use deterministic barriers with release-before-join cleanup, not sleeps
* Preserve lock-free preview's separately selected zero-write contract
* Do not promise protection from external readers or non-global writers
* Do not globally substitute NoFollow or delete-sharing helpers with
  different containment, bounds, and error semantics
* Preserve MoveFileEx WRITE_THROUGH durability; POSIX rename flags belong
  to a different API/flag domain and require their own compatibility proof

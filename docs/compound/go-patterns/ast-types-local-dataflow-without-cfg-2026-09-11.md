---
chunk_strategy: h1-h2-h3
description: "Build bounded AST and go/types analyzers around statement-list ownership, object identity, and conservative lifetime transitions when CFG or SSA is out of scope."
doc_type: learning
docline:
  category: best-practices
  citations:
    - ".backlogit/archive/158.004-T.md"
    - ".backlogit/archive/158.005-T.md"
    - ".backlogit/archive/158.006-T.md"
    - ".backlogit/archive/158.007-T.md"
    - ".backlogit/archive/158.010-T.md"
    - "docs/memory/2026-09-11/fl005-review-remediation-memory.md"
    - "docs/closure/2026-09-11-140-s-pr-436-runtime-verification.md"
    - "docs/closure/2026-09-11-140-s-158-f-pr-436-closure.md"
    - "https://github.com/softwaresalt/backlogit/pull/436"
    - "commit:0b7595d0101415d6446da1239e9f000274b3ad8c"
    - "commit:278a25b671d545846a44833c2996ce3d3bed3c7c"
    - "commit:18deddf9b168ed855636a66bcccc35075bdb0c8a"
    - "commit:674a6b9be382f03dc633607c0f343c34247a8a58"
    - "commit:1cafde3d1477a2812239a967c1a03acc5b53ccfe"
    - "commit:a7a6660a189fa27b414d004e82367157a8b95ae3"
    - "commit:6b7029eadf6418c70045aae552335670be89c7c6"
  component: go-static-analysis
  date: 2026-09-12T02:31:00Z
  file_path: internal/faultline/analyzer
  message: "Lexical block-only or name-based tracking either reports replaced values or hides diagnostics after conditional resets and aliases."
  problem_type: bounded-local-dataflow-precision
  resolution_type: design_change
  root_cause: "AST position alone does not model clause statement lists, lexical object identity, definite replacement, or alias lifetime."
  severity: high
  source_pr: 436
  source_shipment: 140-S
  tags:
    - go
    - ast
    - go-types
    - static-analysis
    - aliases
    - lifetime
    - dataflow
schema_version: "1.0"
source: docs/compound/go-patterns/ast-types-local-dataflow-without-cfg-2026-09-11.md
title: "Bounded AST and go/types dataflow without CFG or SSA"
---

## Problem

The FL001 and FL005 analyzers stayed intentionally intra-function and limited to
AST plus `go/types`. Early implementations produced both error classes:

* False positives after a tracked scanner or context variable was definitely
  replaced
* Hidden diagnostics when a reset occurred only conditionally, an alias was
  rebound on one path, or the source and sink lived directly in a `switch` case
  or `select` communication clause

Block-only ancestry and identifier-name comparisons were not precise enough.

## Root Cause

Three missing abstractions caused the drift:

1. Go statement lists are owned by `BlockStmt`, `CaseClause`, or `CommClause`,
   not only lexical blocks.
2. Names are not variables. Shadowing and rebinding require
   `types.Info.ObjectOf` identity and scope visibility checks.
3. A syntactic assignment is not necessarily a definite lifetime boundary.
   Conditional, loop-local, self-referential, and alias-preserving assignments
   may leave the original value live on a path to the sink.

## Resolution

Use a bounded local analysis with explicit conservative rules.

### Normalize statement-list ownership

Map each relevant statement to its immediate statement-list container:

```go
switch parent := parents[statement].(type) {
case *ast.BlockStmt:
    return parent
case *ast.CaseClause:
    return parent
case *ast.CommClause:
    return parent
}
```

Use that generic owner for ordering, ancestry, suppression attachment, and
definite same-list replacement. Handle control-statement `Init` assignments as
explicit special containers.

### Track symbols, not spellings

Resolve declarations, uses, method selections, and aliases through
`types.Info`. Compare `*types.Var` and `*types.Func` objects directly. At a sink,
confirm that lexical lookup still resolves the tracked name to the same object;
this excludes shadowed variables without fragile text matching.

### End lifetimes only on definite replacement

Treat an assignment as killing a tracked value only when it:

* Targets the same `types.Var`
* Executes unconditionally on the relevant statement-list path
* Occurs after the source and before the sink
* Replaces rather than preserves the tracked value

Keep the value live when the assignment is nested in a conditional or loop,
self-references the variable, or may preserve an alias. For alias sets, add
possible aliases monotonically across ambiguous branches and remove aliases
only for direct definite assignments in the owning list.

## Prevention

* Introduce a single statement-list-owner helper before adding ordering rules
* Build parent maps without descending into nested function literals
* Use object and selection identity for variables, functions, methods, and
  aliases
* Separate possible transitions from definite transitions
* Prefer a conservative diagnostic over suppressing one on an ambiguous path
* Add paired fixtures for block, case, communication clause, init statement,
  shadowing, replacement, self-assignment, and conditional alias rebinding
* Escalate to CFG or SSA only when the required contract explicitly needs
  path-sensitive precision beyond these bounded rules

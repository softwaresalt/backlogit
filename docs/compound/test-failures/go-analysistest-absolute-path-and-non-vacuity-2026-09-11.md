---
chunk_strategy: h1-h2-h3
description: "Use an absolute analysistest testdata root and execute analyzers directly so nested test selection cannot pass without running fixtures."
doc_type: learning
docline:
  category: test-failures
  citations:
    - ".backlogit/archive/158.004-T.md"
    - ".backlogit/archive/158.005-T.md"
    - ".backlogit/archive/158.006-T.md"
    - ".backlogit/archive/158.007-T.md"
    - ".backlogit/archive/158.010-T.md"
    - "docs/closure/2026-09-11-140-s-pr-436-runtime-verification.md"
    - "https://github.com/softwaresalt/backlogit/pull/436"
    - "commit:3d0786949dc115ce4509b56593b94ac584f7da1b"
    - "commit:e0dbf176f368adde0f6a3bdb8c25c6f95a54d1f3"
    - "commit:f0d16f7263d8cb4602d8d8f30f8fcb1a8b94e390"
    - "commit:a3a80b5d848f50e73af4868571f7d9eefddaa80a"
  component: go-analysis-test-harness
  date: 2026-09-12T02:31:00Z
  file_path: internal/faultline/analyzer
  message: "analysistest testdata/GOPATH root must be absolute; nested go test selection can pass without executing the contract"
  problem_type: false-green-test-harness
  resolution_type: design_change
  root_cause: "Relative analysistest roots violate the Go 1.24 GOPATH contract, while subprocess wrappers hide whether the intended nested test and fixture packages actually ran."
  severity: high
  source_pr: 436
  source_shipment: 140-S
  tags:
    - go
    - analysistest
    - testdata
    - gopath
    - non-vacuity
    - subprocess
schema_version: "1.0"
source: docs/compound/test-failures/go-analysistest-absolute-path-and-non-vacuity-2026-09-11.md
title: "Go analysistest needs an absolute root and a non-vacuous harness"
---

## Problem

Analyzer tests failed under the Go 1.24 toolchain when
`analysistest.Run` received a relative testdata path. The first workaround
placed an analyzer contract in `testdata/harness` and launched it through a
nested `go test -run` subprocess. That wrapper could still return success if
the nested selector stopped matching or the contract otherwise did not execute,
creating a false-green test.

## Root Cause

`analysistest.Run` treats its testdata argument as a synthetic GOPATH root.
That root must be absolute in Go 1.24. A nested subprocess also separates the
outer test result from the actual analyzer invocation, so process exit zero
does not prove that every requested fixture package was analyzed.

## Resolution

Run the analyzer contract directly from the package test:

```go
testdata, err := filepath.Abs("testdata")
require.NoError(t, err)

results := analysistest.Run(
    t,
    testdata,
    Analyzer,
    "badfixture",
    "goodfixture",
)
require.Len(t, results, 2)
```

`analysistest.TestData()` is also appropriate for the conventional package-local
layout when it resolves the canonical testdata root. Keep explicit assertions
that the analyzer, name, and run function exist before invoking the fixtures.

## Prevention

* Pass an absolute path whenever supplying a custom analysistest root
* Prefer direct `analysistest.Run` calls over subprocess-owned contract tests
* Assert the number of analyzed package results or another non-vacuity signal
* Keep at least one seeded `// want` diagnostic and one clean fixture
* Avoid relying on outer `go test -run` success as evidence that a nested
  analyzer contract executed

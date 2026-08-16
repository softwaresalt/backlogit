---
chunk_strategy: h1-h2-h3
description: "Post-merge runtime verification for features 139-F through 142-F on the merged main tip."
doc_type: closure
docline:
    date: 2026-08-15T00:00:00Z
    status: accepted
    tags:
        - runtime-verification
        - 139-F
        - 140-F
        - 141-F
        - 142-F
schema_version: "1.0"
source: docs/closure/2026-08-15-139-142-ship-runtime-verification.md
title: "Features 139-F through 142-F — Runtime Verification"
---

# Features 139-F through 142-F — Runtime Verification

## Verdict: PASS

## Verification target

Verification targeted the merged `main` history through the following merge
commits:

* 139-F — PR #361, `1235bcd80879fc59b4634e2b3eadfaf2d746cd9c`
* 140-F — PR #362, `fa54a35ba2c7f2147e8cb0694b9b12be7238070c`
* 141-F — PR #363, `22827b1eedeed7e6cbc31ff870f33950dbefb1ee`
* 142-F — PR #364, `17530fe30f68034bff502362e489eff82fb86fe7`

The merge ancestry gate confirmed the final merge commit is an ancestor of
`origin/main`. No formal release was created or published.

## Automated verification

* Targeted governed registry parity tests passed
* Full `internal/cli` tests passed
* `go test -race '-coverprofile=coverage.out' ./...` passed across all
  packages, including contract and integration tests
* `go vet ./...` passed
* `golangci-lint run --timeout=5m` passed
* Documentation lint and all PR #364 CI checks passed

The temporary race-test coverage file was removed after the successful run.

## Runtime surface

Feature 139-F keeps MCP startup on the canonical artifact-index path rather
than rescanning Markdown link sources. Features 140-F and 141-F preserve
shipment rollback/CAS and dependency-parity behavior. Feature 142-F exercises
the governed registry mappings through registered MCP handlers and verifies
comment and dependency durability in both JSONL/Markdown sources and indexed
projections.

## Protected invariants

* Startup link migration uses one canonical artifact index per operation
* Late shipment failures do not leave unreconciled partial release state
* `persistArtifact` mutation boundaries remain guarded by the repository CAS
  contract
* Governed parity fixtures dispatch through the authoritative registry mapping
* Comment events remain durable in JSONL and visible through the index
* Dependency changes remain durable in Markdown frontmatter and represented in
  the disposable index projection
* The tests do not write outside the backlogit repository

## Residual exposure

The installed backlogit binary used for day-to-day operations may lag the
merged source until it is independently rebuilt and pinned. This is an
operational deployment concern, not a failure of the merged source or its
verification gates. Feature 138-F remains blocked because its approved work
requires changes in the external autoharness repository.

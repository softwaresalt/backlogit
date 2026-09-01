---
breaker_type: universal
doc_type: memory
docline:
  date: 2026-08-24T18:00:00Z
  status: accepted
  tags:
    - circuit-breaker
    - pr-377
    - copilot-review
operation: request-copilot-review
schema_version: "1.0"
source: docs/memory/2026-08-24/circuit-break-pr377-copilot-review-request.md
timestamp: 2026-08-24T18:00:00Z
title: "Circuit breaker: PR 377 Copilot review request"
---

## Failure chain

### Attempt 1

`gh pr edit 377 --add-reviewer copilot` failed with `'' not found`.

### Attempt 2

The GitHub requested-reviewers REST endpoint accepted
`copilot-pull-request-reviewer[bot]`, but returned no requested reviewer.

### Attempt 3

The same endpoint accepted the display login `Copilot`, but returned no
requested reviewer.

After a two-minute review interval, PR 377 still had no pending request and no
completed review.

## Context

* Agent: Orchestrator
* Skill: `pr-lifecycle`
* Pull request: 377
* Head: `540930d6ab975ebe604d595120e38501a6de1185`
* Shipment: `130-S`
* Feature: `147-F`
* CI: all required checks passed
* Files involved: staging artifacts committed at `540930d6`
* Resolution: circuit breaker triggered; the P-014 review gate remains closed
* Resume action: restore a working Copilot review-request path, request review
  for the current head, verify freshness and thread resolution, then merge with
  merge-commit strategy under the existing dark-mode authorization

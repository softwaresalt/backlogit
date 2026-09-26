---
doc_type: memory
schema_version: "1.0"
title: "Compacted memory: autoharness 1.5.0 merge-install"
---

# Compacted Memory: Autoharness 1.5.0 Merge-Install

## Outcome

The workspace installed autoharness `1.5.0`, enabled continuous-learning and
read-only graphtor-docs, and merged PR `#400` with merge commit `7c521bf2`.
The earlier bounded-residual tuning pass was superseded by this completed
merge-install.

## Decisions

* Graphtor-docs remains read-only and must never index this workspace
* Agent allowlists enumerate the eight graphtor read verbs instead of granting
  a wildcard
* Shipment close uses a fail-closed pre-flight gate because a safe
  non-cascading runtime close was unavailable
* Package-version verification must identify the package index that answered
  before pinning
* YAML validation requires a duplicate-key check because ordinary loading can
  silently discard an earlier key

## Review-control learning

The main and closure PR review loops exceeded their cycle limits. The durable
lesson is that a review extension's stated bound has the same force as the
original cap: reaching it requires a halt and explicit operator disposition.
Later ratification does not retroactively authorize earlier work.

The package-index guidance converged on three non-redundant controls:
`PIP_CONFIG_FILE` pointing to the platform null device, `pip --isolated`, and
an explicit PyPI index URL.

## Residuals recorded at closure

* Three workspace agents lacked the then-required tier annotations
* Graphtor MCP registration remained machine-local
* A pre-existing local-only `main` commit required operator disposition

## Archived sources

* `docs/archive/memory/2026-08-30/autoharness-1-5-update-memory.md`
* `docs/archive/memory/2026-08-31/autoharness-1-5-0-merge-install-memory.md`


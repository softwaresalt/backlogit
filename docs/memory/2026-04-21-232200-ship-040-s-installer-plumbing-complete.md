---
title: Ship 040-S — installer plumbing complete
description: Tasks 039.009-T through 039.011-T completed for shipment 040-S
ms.date: 2026-04-21
---

## Completed Tasks

* 039.009-T — Complete Release Workflow ldflags
* 039.010-T — Add SHA256 Checksum Generation
* 039.011-T — Create One-Liner Install Scripts

## Files Added or Modified

* [.github/workflows/release.yml](.github/workflows/release.yml)
* [scripts/install/install.sh](scripts/install/install.sh)
* [scripts/install/install.ps1](scripts/install/install.ps1)

## Outcome

The standalone binary release path now has the delivery plumbing in place:

* release binaries carry version, commit, and build date metadata
* the release job generates and publishes `SHA256SUMS`
* Unix and Windows one-liner installer scripts download the latest release asset, verify checksums, install into a standard user directory, and print PATH guidance

## Blocking State

* 039.012-T remains blocked by telemetry tasks 039.015-T and 039.016-T, in addition to the release-side work that is now complete

## Next Ready Items

* 039.013-T — Telemetry Reporter Behavioral Tests
* 039.014-T — End-to-End Harvest Pipeline Test

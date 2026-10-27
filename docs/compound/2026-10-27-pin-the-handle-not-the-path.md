---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Pin the handle, not the path, and bind it to the expected path"
description: "CY-C (201-S, PR #591): a containment check done on a resolved pathname is a TOCTOU window; verify the pinned handle's final path equals the expected path, and never re-resolve inside the swap window."
doc_type: learning
source: docs/compound/2026-10-27-pin-the-handle-not-the-path.md
docline:
    date: 2026-10-27T00:00:00Z
    severity: medium
    tags:
        - filesystem
        - containment
        - windows
        - 201-S
---

# Pin the handle, not the path, and bind it to the expected path

## Problem

Checking that a path is contained and then reading or creating by that path leaves a swap window. Two traps appeared in CY-C: calling `EvalSymlinks` inside the window made the pin adopt a symlink swapped in after the reparse check, and a "final path is inside storage" check accepted a junction swapped to another in-root directory.

## Solution

* Resolve the destination once, up front, then open the pin on that path and compare the handle-derived final path to it for equality, not containment.
* On POSIX walk each component with `Openat(O_NOFOLLOW)` from a root descriptor and never re-check identity by path.
* Compare JSON member names with the `unicode.SimpleFold` orbit minimum so a scanner matches `encoding/json` key binding.

## Notes

Windows junctions need no privilege (`mklink /J`), so swap regressions can run, not skip, on any developer host. Review-found items are best proven with a failing test before the fix.

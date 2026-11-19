---
description: "Tune an installed harness using the current workspace's local GSD bundle"
agent: Auto-Tune
argument-hint: "workspace=<target-path> [scope=all|instructions|agents|skills|policies|constitution]"
---

# Tune Harness

The tuner reads templates from the current workspace's local `.bin/` bundle and
updates only that workspace's harness artifacts. Global/user-home discovery is
legacy compatibility behavior, not the recommended path.

## Inputs

* ${input:workspace}: (Required in CLI environments, auto-detected in editors) Absolute path to the target workspace.
* ${input:scope}: (Optional) Scope of tuning: `all`, `instructions`, `agents`, `skills`, `policies`, `constitution`. Defaults to `all`.

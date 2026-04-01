---
id: TASK-010.02.02
title: Write installation guide
status: done
assignee: []
created_date: '2026-04-01 22:31'
labels:
  - docs
dependencies:
  - TASK-010.02.01
parent_task_id: TASK-010.02
priority: medium
ordinal: 2000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `docs/installation.md` covering all installation methods for backlogit.

Content to include:
- Prerequisites: Go 1.22+, supported platforms (Linux, macOS, Windows)
- Method 1: `go install github.com/backlogit/backlogit/cmd/backlogit@latest`
- Method 2: Binary download from GitHub releases (future)
- Method 3: Build from source (`git clone`, `go build ./cmd/backlogit`)
- Post-install verification: `backlogit --version`, `backlogit init`
- Shell completion setup (bash, zsh, fish, PowerShell)
- Troubleshooting common installation issues

Files to create:
- `docs/installation.md` (new)

Verification: All installation commands are correct and tested against the current codebase.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Installation guide covers go install, binary download, and build from source
- [ ] #2 Prerequisites section lists Go 1.22+ and platform requirements
- [ ] #3 Verification steps confirm successful installation
<!-- AC:END -->

---
title: Installation
description: Installation methods for backlogit
author: backlogit contributors
ms.date: 2026-04-01
ms.topic: how-to
keywords:
  - backlogit
  - installation
  - go install
  - build from source
---

## Prerequisites

backlogit requires Go 1.24 or later. It runs on Linux, macOS, and Windows. Because the SQLite driver uses a pure-Go implementation with no CGo, no C toolchain or system SQLite library is needed.

Confirm your Go version before installing:

```bash
go version
```

The output should show `go1.24` or higher.

## Method 1: Install with go install

This is the recommended method for most users. The command downloads, compiles, and installs the `backlogit` binary to your `$GOPATH/bin` (or `$GOBIN` if set):

```bash
go install github.com/backlogit/backlogit/cmd/backlogit@latest
```

Ensure `$GOPATH/bin` is in your `PATH`. On most systems this is already the case, but you can verify with:

```bash
echo $PATH | tr ':' '\n' | grep go
```

On Windows with PowerShell:

```powershell
$env:PATH -split ';' | Where-Object { $_ -like '*go*' }
```

## Method 2: Build from Source

Clone the repository and build the binary directly. This gives you access to unreleased changes and lets you inspect the source before running.

```bash
git clone https://github.com/backlogit/backlogit.git
cd backlogit
go build -o backlogit ./cmd/backlogit
```

Move the binary to a directory in your `PATH`:

```bash
# Linux / macOS
mv backlogit /usr/local/bin/

# Windows (run as administrator or adjust the destination)
Move-Item backlogit.exe C:\Windows\System32\
```

## Post-install Verification

Confirm the installation succeeded:

```bash
backlogit help
```

You should see a list of available commands. If the binary is not found, check that the install directory is in your `PATH`.

## Shell Completion

backlogit uses Cobra, which generates completion scripts for Bash, Zsh, Fish, and PowerShell.

**Bash:**

```bash
backlogit completion bash > /etc/bash_completion.d/backlogit
source /etc/bash_completion.d/backlogit
```

**Zsh:**

```zsh
backlogit completion zsh > "${fpath[1]}/_backlogit"
```

**Fish:**

```fish
backlogit completion fish > ~/.config/fish/completions/backlogit.fish
```

**PowerShell:**

```powershell
backlogit completion powershell | Out-String | Invoke-Expression
```

To make PowerShell completion persistent, add the `Invoke-Expression` line to your `$PROFILE` file.

## Common Issues

**`command not found: backlogit` after `go install`**

Your `$GOPATH/bin` is not in `PATH`. Add it to your shell profile:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

**Build fails with a Go version error**

The module requires Go 1.24 or later. Run `go version` and upgrade your toolchain if needed. The official [Go downloads page](https://go.dev/dl/) has installers for all platforms.

**Binary runs but cannot find the workspace**

backlogit looks for a `.backlogit/` directory in the current working directory. Run `backlogit init` to create one, or change into the directory that contains an existing workspace before running commands.

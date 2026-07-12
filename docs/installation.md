---
chunk_strategy: h1-h2-h3
description: Install backlogit from pre-built binaries or from source
doc_type: guide
docline:
    author: backlogit contributors
    keywords:
        - backlogit
        - installation
        - binary download
        - go install
        - PATH
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: how-to
ingested_at: "2026-06-26T02:34:29Z"
schema_version: "1.0"
source: docs/installation.md
title: Installation
---

## Which installation path

Choose the path that matches how you want backlogit to enter the workspace:

| Path | Choose when | Install surface |
|---|---|---|
| Path A - Standalone backlogit | You want to use backlogit directly as a CLI, MCP server, or Copilot CLI plugin | The backlogit binary plus, when using the plugin, the Stage and Ship agents, 19 bundled skills, and the backlogit MCP server |
| Path B - Autoharness-composed harness | Backlogit is being adopted as part of an Autoharness-generated agent harness | The backlogit binary/runtime from this page, plus Autoharness-composed constitution, policies, instructions, agents, and skills in `.github/` |

This page covers the backlogit binary/runtime install methods. They apply to
Path A users who want the CLI directly or the standalone Copilot CLI plugin, and
to Path B users whose generated harness invokes `backlogit` or `backlogit mcp`.
The plugin manifest launches the MCP server with `backlogit mcp`, so the binary
must be on PATH before the plugin starts. If your workspace already has an
Autoharness-generated `.github/` harness, use the Autoharness install or tune
flow for the harness files and install only the runtime with one of the methods
below.
The standalone plugin manifest lives at `.github/plugin/plugin.json`, so the
plain `copilot plugin install softwaresalt/backlogit` owner/repo form is the
canonical cross-platform plugin install command.

> [!IMPORTANT]
> Avoid double-installing. The standalone plugin contributes its own frozen
> agents and skills from `plugin/`, while Autoharness writes repo-local harness
> files into `.github/`. Path B still needs a reachable backlogit runtime; it
> should not use the standalone plugin as the source of its repo-local harness.

## Method 1: One-line install

`backlogit` ships as a standalone executable. You do not need Go for the binary
install paths below.

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/softwaresalt/backlogit/main/scripts/install/install.ps1 | iex
```

### Linux and macOS

```bash
curl -fsSL https://raw.githubusercontent.com/softwaresalt/backlogit/main/scripts/install/install.sh | sh
```

The install scripts:

* detect the current OS and CPU architecture
* download the matching binary from the latest GitHub release
* verify the download against `SHA256SUMS`
* install into a user-writable directory
* print PATH guidance when the install directory is not already discoverable

You can override the install directory with `BACKLOGIT_INSTALL_DIR`.

## Method 2: Download the binary directly

If you prefer to place the executable yourself, download it from
[GitHub Releases](https://github.com/softwaresalt/backlogit/releases).

### Platform matrix

| Platform | Asset |
|---|---|
| Linux x64 | `backlogit-linux-amd64` |
| Linux ARM64 | `backlogit-linux-arm64` |
| macOS x64 | `backlogit-darwin-amd64` |
| macOS ARM64 | `backlogit-darwin-arm64` |
| Windows x64 | `backlogit-windows-amd64.exe` |
| Windows ARM64 | `backlogit-windows-arm64.exe` |

### Install manually

1. Download the binary for your platform from GitHub Releases.
2. Download `SHA256SUMS` from the same release.
3. Verify the checksum.
4. Move the binary into a directory on your PATH.

#### Example checksum verification

```bash
curl -fsSL -O https://github.com/softwaresalt/backlogit/releases/latest/download/backlogit-linux-amd64
curl -fsSL -O https://github.com/softwaresalt/backlogit/releases/latest/download/SHA256SUMS
grep "backlogit-linux-amd64$" SHA256SUMS | sha256sum -c -
chmod +x backlogit-linux-amd64
mv backlogit-linux-amd64 ~/.local/bin/backlogit
```

On macOS, use `shasum -a 256` instead of `sha256sum`:

```bash
curl -fsSL -O https://github.com/softwaresalt/backlogit/releases/latest/download/backlogit-darwin-arm64
curl -fsSL -O https://github.com/softwaresalt/backlogit/releases/latest/download/SHA256SUMS
expected="$(grep 'backlogit-darwin-arm64$' SHA256SUMS | awk '{print $1}')"
actual="$(shasum -a 256 backlogit-darwin-arm64 | awk '{print $1}')"
[ "$expected" = "$actual" ] || { echo "checksum mismatch" >&2; exit 1; }
chmod +x backlogit-darwin-arm64
mv backlogit-darwin-arm64 ~/.local/bin/backlogit
```

```powershell
Invoke-WebRequest https://github.com/softwaresalt/backlogit/releases/latest/download/backlogit-windows-amd64.exe -OutFile backlogit.exe
Invoke-WebRequest https://github.com/softwaresalt/backlogit/releases/latest/download/SHA256SUMS -OutFile SHA256SUMS
$expected = ((Select-String -Path .\SHA256SUMS -Pattern 'backlogit-windows-amd64\.exe$').Line -split '\s+')[0]
$actual = (Get-FileHash .\backlogit.exe -Algorithm SHA256).Hash
if ($expected -ne $actual) { throw 'checksum mismatch' }
Move-Item .\backlogit.exe $HOME\bin\backlogit.exe
```

### PATH guidance

Add the install directory to your PATH if it is not already present.

#### Linux and macOS

```bash
export PATH="$HOME/.local/bin:$PATH"
```

#### Windows PowerShell

```powershell
$target = "$HOME\bin"
[Environment]::SetEnvironmentVariable(
  "Path",
  [Environment]::GetEnvironmentVariable("Path", "User") + ";$target",
  "User"
)
```

Open a new shell after updating PATH.

## Method 3: Install from source

Use this path if you want the Go toolchain workflow for the latest tagged
release. This method requires Go 1.24 or later.

```bash
go install github.com/softwaresalt/backlogit/cmd/backlogit@latest
```

To build locally from a clone instead:

```bash
git clone https://github.com/softwaresalt/backlogit.git
cd backlogit
go build -o backlogit ./cmd/backlogit
go install ./cmd/backlogit
```

## Post-install verification

Confirm the binary is on your PATH:

```bash
backlogit help
```

You should see the available commands. If the shell cannot find `backlogit`,
recheck the directory you installed into and your PATH configuration.

Path A users who also want the bundled Copilot CLI agents and skills can install
the standalone plugin after this check passes:

```bash
copilot plugin install softwaresalt/backlogit
```

For local development from a clone, use `copilot plugin install ./` from the
repository root.

## Shell completion

`backlogit` uses Cobra, which generates completion scripts for Bash, Zsh, Fish,
and PowerShell.

### Bash

```bash
backlogit completion bash > /etc/bash_completion.d/backlogit
source /etc/bash_completion.d/backlogit
```

### Zsh

```zsh
backlogit completion zsh > "${fpath[1]}/_backlogit"
```

### Fish

```fish
backlogit completion fish > ~/.config/fish/completions/backlogit.fish
```

### PowerShell

```powershell
backlogit completion powershell | Out-String | Invoke-Expression
```

To make PowerShell completion persistent, add that line to your `$PROFILE`.

## Common issues

### `backlogit` is not found after installation

Your install directory is not on PATH. Add the directory that contains the
binary, then open a new shell.

### Checksum verification fails

Delete the downloaded binary and `SHA256SUMS`, then download both files again
from the same GitHub release. A mismatch usually means the binary and checksum
file came from different release versions.

### Source install fails with a Go version error

The module requires Go 1.24 or later. Run `go version` and upgrade your
toolchain if needed. The official [Go downloads page](https://go.dev/dl/) has
installers for all supported platforms.

### Binary runs but cannot find the workspace

`backlogit` looks for a `.backlogit/` directory in the current working
directory. Run `backlogit init` to create one, or change into a directory that
already contains an existing workspace.

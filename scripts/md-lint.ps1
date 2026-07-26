<#
.SYNOPSIS
    Enforces the P-008 markdown heading hierarchy (MD001/MD025/MD041) repo-wide.
.DESCRIPTION
    Runs markdownlint-cli2 (pinned) over the version-controlled Markdown corpus.
    The rule set is declared in .markdownlint.json; runner options (gitignore-aware
    globbing) live in .markdownlint-cli2.jsonc. The Node tool invocation is kept in
    this script so operator-facing files stay free of npx/npm tokens (enforced by
    the tests/integration retired-wrapper guard). This is the Windows companion to
    scripts/md-lint.sh.
.EXAMPLE
    scripts/md-lint.ps1
#>

# Stop on any PowerShell (non-native) error so a missing prerequisite cannot let
# the gate pass silently. Without this, a CommandNotFoundException for `npx` can
# leave $LASTEXITCODE stale/0 and `exit $LASTEXITCODE` would falsely report success.
$ErrorActionPreference = 'Stop'

# Fail loudly if the Node tool runner is unavailable (missing prerequisite must
# NOT be treated as a clean gate). Get-Command throws under Stop when npx is absent.
Get-Command npx -CommandType Application -ErrorAction Stop | Out-Null

# Native command: markdownlint exits non-zero on violations; propagate that code.
& npx --yes markdownlint-cli2@0.23.1 "**/*.md"
exit $LASTEXITCODE

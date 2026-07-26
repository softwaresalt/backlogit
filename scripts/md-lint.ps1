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

npx --yes markdownlint-cli2@0.23.1 "**/*.md"
exit $LASTEXITCODE

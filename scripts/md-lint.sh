#!/usr/bin/env bash
# Enforces the P-008 markdown heading hierarchy (MD001/MD025/MD041) repo-wide.
# Rule set is declared in .markdownlint.json; markdownlint-cli2 runner options
# (gitignore-aware globbing) live in .markdownlint-cli2.jsonc so the gate lints the
# non-gitignored Markdown corpus — exactly the tracked set in a clean CI checkout
# (locally it also covers new/untracked non-ignored Markdown; scratch must be gitignored).
#
# markdownlint-cli2 is a Node tool pinned to a fixed version. The invocation
# lives in this script rather than inline in the Makefile / CI workflow so that
# operator-facing files stay free of npx/npm tokens (enforced by the
# tests/integration retired-wrapper guard).
#
# Usage: scripts/md-lint.sh

set -euo pipefail

npx --yes markdownlint-cli2@0.23.1 "**/*.md"

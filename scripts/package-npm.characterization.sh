#!/usr/bin/env bash
# scripts/package-npm.characterization.sh
#
# Characterization test for scripts/package-npm.sh (task 080.002-T).
#
# Pins the CURRENT observable output of package-npm.sh: running it against a stub
# dist/ containing the 5 expected platform binaries must produce valid, version-
# stamped package.json for all 5 platform packages plus the @backlogit/backlogit-mcp
# wrapper, with the wrapper's optionalDependencies versions synced to the input version.
#
# This is characterization-first: it captures existing behavior and is expected to be
# GREEN. It runs against an isolated copy of scripts/ + npm/ so the tracked repo files
# are never mutated.
#
# Requirements: bash + jq (jq is already a runtime dependency of package-npm.sh).
# Usage: bash scripts/package-npm.characterization.sh
# Optional: set RUN_NPM_PACK=1 to additionally run `npm pack --dry-run` per package
#           (optional confidence check per the plan; requires npm on PATH).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

TEST_VERSION="9.9.9-characterization"

# Expected platform packages and their source binary names (mirrors package-npm.sh).
PLATFORM_PKGS=(linux-x64 linux-arm64 darwin-x64 darwin-arm64 win32-x64)
PLATFORM_BINS=(
  backlogit-linux-amd64
  backlogit-linux-arm64
  backlogit-darwin-amd64
  backlogit-darwin-arm64
  backlogit-windows-amd64.exe
)

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

command -v jq >/dev/null 2>&1 || fail "jq is required (it is also a runtime dependency of package-npm.sh)"
[[ -f "${REPO_ROOT}/scripts/package-npm.sh" ]] || fail "missing scripts/package-npm.sh"
[[ -d "${REPO_ROOT}/npm" ]] || fail "missing npm/ package tree"

# Isolated workspace so the tracked npm/**/package.json files are never mutated.
WORKDIR="$(mktemp -d)"
cleanup() { rm -rf "${WORKDIR}"; }
trap cleanup EXIT

mkdir -p "${WORKDIR}/scripts" "${WORKDIR}/dist"
cp "${REPO_ROOT}/scripts/package-npm.sh" "${WORKDIR}/scripts/package-npm.sh"
cp -r "${REPO_ROOT}/npm" "${WORKDIR}/npm"

# Stub the 5 expected platform binaries in the isolated dist/.
for bin in "${PLATFORM_BINS[@]}"; do
  : > "${WORKDIR}/dist/${bin}"
done

echo "Running isolated package-npm.sh ${TEST_VERSION} against stub dist/ ..."
bash "${WORKDIR}/scripts/package-npm.sh" "${TEST_VERSION}" "${WORKDIR}/dist" >/dev/null

# --- Required assertions ---------------------------------------------------------

# 1. Each of the 5 platform package.json is valid JSON with the version stamped.
for pkg in "${PLATFORM_PKGS[@]}"; do
  pj="${WORKDIR}/npm/platforms/${pkg}/package.json"
  [[ -f "${pj}" ]] || fail "missing platform package.json: ${pkg}"
  jq empty "${pj}" 2>/dev/null || fail "invalid JSON: platforms/${pkg}/package.json"
  actual="$(jq -r '.version' "${pj}")"
  [[ "${actual}" == "${TEST_VERSION}" ]] \
    || fail "platforms/${pkg}: version=${actual}, expected ${TEST_VERSION}"
  echo "  [OK] platforms/${pkg}: valid JSON, version=${actual}"
done

# 2. Wrapper package.json is valid JSON, version-stamped, and every
#    optionalDependencies value is synced to the input version.
wrapper="${WORKDIR}/npm/backlogit-mcp/package.json"
[[ -f "${wrapper}" ]] || fail "missing wrapper package.json"
jq empty "${wrapper}" 2>/dev/null || fail "invalid JSON: backlogit-mcp/package.json"

wrapper_version="$(jq -r '.version' "${wrapper}")"
[[ "${wrapper_version}" == "${TEST_VERSION}" ]] \
  || fail "wrapper: version=${wrapper_version}, expected ${TEST_VERSION}"

optdep_count="$(jq -r '.optionalDependencies | length' "${wrapper}")"
[[ "${optdep_count}" -eq 5 ]] \
  || fail "wrapper: optionalDependencies count=${optdep_count}, expected 5"

synced="$(jq -r --arg v "${TEST_VERSION}" \
  '.optionalDependencies | to_entries | all(.value == $v)' "${wrapper}")"
[[ "${synced}" == "true" ]] \
  || fail "wrapper: optionalDependencies not all synced to ${TEST_VERSION}"
echo "  [OK] backlogit-mcp: valid JSON, version=${wrapper_version}, ${optdep_count} optionalDependencies synced"

# --- Optional confidence check ---------------------------------------------------

if [[ "${RUN_NPM_PACK:-0}" == "1" ]] && command -v npm >/dev/null 2>&1; then
  echo "Running optional npm pack --dry-run ..."
  for pkg in "${PLATFORM_PKGS[@]}"; do
    npm pack --dry-run "${WORKDIR}/npm/platforms/${pkg}" >/dev/null 2>&1 \
      && echo "  [OK] npm pack --dry-run: platforms/${pkg}" \
      || echo "  [WARN] npm pack --dry-run failed (non-fatal): platforms/${pkg}"
  done
  npm pack --dry-run "${WORKDIR}/npm/backlogit-mcp" >/dev/null 2>&1 \
    && echo "  [OK] npm pack --dry-run: backlogit-mcp" \
    || echo "  [WARN] npm pack --dry-run failed (non-fatal): backlogit-mcp"
fi

echo "PASS: package-npm.sh emits valid, version-stamped package.json for 5 platform packages + wrapper."

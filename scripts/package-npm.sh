#!/usr/bin/env bash
# scripts/package-npm.sh
# Package backlogit binaries into platform-specific npm packages.
#
# Usage: package-npm.sh <version> <artifacts-dir>
#   version:       semver without leading 'v' (e.g. 1.1.0)
#   artifacts-dir: directory containing backlogit-{goos}-{goarch}[.exe] files

set -euo pipefail

VERSION="${1:?Usage: $0 <version> <artifacts-dir>}"
ARTIFACTS="${2:?Usage: $0 <version> <artifacts-dir>}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

declare -A PLATFORM_BINS=(
  ["linux-x64"]="backlogit-linux-amd64"
  ["linux-arm64"]="backlogit-linux-arm64"
  ["darwin-x64"]="backlogit-darwin-amd64"
  ["darwin-arm64"]="backlogit-darwin-arm64"
  ["win32-x64"]="backlogit-windows-amd64.exe"
)

declare -A WIN_PLATFORMS=(
  ["win32-x64"]=1
)

echo "Packaging backlogit v${VERSION} npm binaries from ${ARTIFACTS}"

for platform in "${!PLATFORM_BINS[@]}"; do
  src_bin="${PLATFORM_BINS[$platform]}"
  src="${ARTIFACTS}/${src_bin}"
  pkg_dir="${REPO_ROOT}/npm/platforms/${platform}"
  bin_dir="${pkg_dir}/bin"

  if [[ ! -f "${src}" ]]; then
    echo "ERROR: missing artifact: ${src}" >&2
    exit 1
  fi

  mkdir -p "${bin_dir}"

  if [[ -v WIN_PLATFORMS["${platform}"] ]]; then
    dest="${bin_dir}/backlogit.exe"
  else
    dest="${bin_dir}/backlogit"
  fi

  cp "${src}" "${dest}"
  chmod +x "${dest}"

  # Stamp version into platform package.json
  jq --arg v "${VERSION}" '.version = $v' "${pkg_dir}/package.json" \
    > "${pkg_dir}/package.json.tmp"
  mv "${pkg_dir}/package.json.tmp" "${pkg_dir}/package.json"

  echo "  [OK] ${platform}: ${src_bin} → ${dest}"
done

# Stamp version into the main wrapper and sync optionalDependencies versions
jq --arg v "${VERSION}" \
  '.version = $v | .optionalDependencies = (.optionalDependencies | with_entries(.value = $v))' \
  "${REPO_ROOT}/npm/backlogit-mcp/package.json" \
  > "${REPO_ROOT}/npm/backlogit-mcp/package.json.tmp"
mv "${REPO_ROOT}/npm/backlogit-mcp/package.json.tmp" \
   "${REPO_ROOT}/npm/backlogit-mcp/package.json"

echo "  [OK] @backlogit/backlogit-mcp version set to ${VERSION}"
echo "Done. All npm packages are ready for publishing."

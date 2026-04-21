#!/bin/sh
set -eu

repo="softwaresalt/backlogit"
install_dir="${BACKLOGIT_INSTALL_DIR:-$HOME/.local/bin}"
tmp_dir=""

cleanup() {
	if [ -n "$tmp_dir" ] && [ -d "$tmp_dir" ]; then
		rm -rf "$tmp_dir"
	fi
}

trap cleanup EXIT INT TERM

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		printf '%s\n' "missing required command: $1" >&2
		exit 1
	fi
}

detect_os() {
	case "$(uname -s)" in
		Linux) printf 'linux' ;;
		Darwin) printf 'darwin' ;;
		*)
			printf '%s\n' "unsupported operating system: $(uname -s)" >&2
			exit 1
			;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
		x86_64 | amd64) printf 'amd64' ;;
		arm64 | aarch64) printf 'arm64' ;;
		*)
			printf '%s\n' "unsupported architecture: $(uname -m)" >&2
			exit 1
			;;
	esac
}

verify_checksum() {
	checksum_line=$(grep "  $asset_name\$" "$checksums_path" || true)
	if [ -z "$checksum_line" ]; then
		printf '%s\n' "checksum entry not found for $asset_name in SHA256SUMS" >&2
		exit 1
	fi

	if command -v sha256sum >/dev/null 2>&1; then
		(
			cd "$tmp_dir"
			printf '%s\n' "$checksum_line" | sha256sum -c -
		)
		return
	fi

	if command -v shasum >/dev/null 2>&1; then
		expected_hash=$(printf '%s\n' "$checksum_line" | awk '{print $1}')
		actual_hash=$(shasum -a 256 "$asset_path" | awk '{print $1}')
		if [ "$expected_hash" != "$actual_hash" ]; then
			printf '%s\n' "checksum mismatch for $asset_name" >&2
			exit 1
		fi
		return
	fi

	printf '%s\n' "missing checksum tool: sha256sum or shasum" >&2
	exit 1
}

os_name=$(detect_os)
arch_name=$(detect_arch)
asset_name="backlogit-${os_name}-${arch_name}"
base_url="https://github.com/${repo}/releases/latest/download"
asset_url="${base_url}/${asset_name}"
checksums_url="${base_url}/SHA256SUMS"

need_cmd curl
tmp_dir=$(mktemp -d)
asset_path="${tmp_dir}/${asset_name}"
checksums_path="${tmp_dir}/SHA256SUMS"

printf '%s\n' "Downloading ${asset_name} from GitHub releases/latest..."
curl -fsSL "$asset_url" -o "$asset_path"
curl -fsSL "$checksums_url" -o "$checksums_path"

verify_checksum

mkdir -p "$install_dir"
cp "$asset_path" "${install_dir}/backlogit"
chmod 0755 "${install_dir}/backlogit"

printf '%s\n' "Installed backlogit to ${install_dir}/backlogit"

case ":$PATH:" in
	*":$install_dir:"*)
		printf '%s\n' "backlogit is ready to use."
		;;
	*)
		printf '%s\n' "Add this directory to your PATH:"
		printf '%s\n' "  export PATH=\"${install_dir}:\$PATH\""
		;;
esac

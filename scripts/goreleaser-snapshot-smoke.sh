#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

source "${script_dir}/release-version.sh"

smoke_version="${OUTLOOK_AGENT_SMOKE_VERSION:-v0.0.0-smoke}"
validate_release_version "$smoke_version"

dist_dir="${repo_root}/dist"
checksum_file="${dist_dir}/SHA256SUMS.txt"

checksum() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  else
    shasum -a 256 "$file" | awk '{print $1}'
  fi
}

goreleaser_command() {
  if [[ -n "${GORELEASER_CMD:-}" ]]; then
    # shellcheck disable=SC2206
    echo "${GORELEASER_CMD}"
    return
  fi
  if command -v goreleaser >/dev/null 2>&1; then
    echo "goreleaser"
    return
  fi
  echo "go run github.com/goreleaser/goreleaser/v2@latest"
}

rm -rf "$dist_dir"
mkdir -p "$dist_dir"

read -r -a goreleaser_cmd <<<"$(goreleaser_command)"

echo "running goreleaser release --snapshot --clean --skip=publish"
GORELEASER_CURRENT_TAG="$smoke_version" "${goreleaser_cmd[@]}" release --snapshot --clean --skip=publish

scripts/release-sbom.sh "$smoke_version" "$dist_dir"

dependency_manifest="$(find "$dist_dir" -maxdepth 1 -type f -name "*_dependency-manifest.json" | sort | head -n 1)"
if [[ -z "$dependency_manifest" ]]; then
  echo "missing dependency manifest in ${dist_dir}" >&2
  exit 1
fi

if [[ ! -f "$checksum_file" ]]; then
  echo "missing SHA256SUMS.txt in ${dist_dir}" >&2
  exit 1
fi

dependency_manifest_name="$(basename "$dependency_manifest")"
if ! grep -Fq "  ${dependency_manifest_name}" "$checksum_file"; then
  printf "%s  %s\n" "$(checksum "$dependency_manifest")" "$dependency_manifest_name" >> "$checksum_file"
fi

scripts/release-verify.sh "$dist_dir"

echo "goreleaser snapshot smoke passed: ${dist_dir}"

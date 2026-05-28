#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

dist_dir="${OUTLOOK_AGENT_DIST_DIR:-$(mktemp -d /private/tmp/outlook-agent-release-smoke.XXXXXX)}"
expected_archives=6

cleanup() {
  if [[ -z "${OUTLOOK_AGENT_KEEP_RELEASE_SMOKE:-}" ]]; then
    rm -rf "$dist_dir"
  fi
}
trap cleanup EXIT

OUTLOOK_AGENT_DIST_DIR="$dist_dir" scripts/release-build.sh smoke

checksum_file="${dist_dir}/SHA256SUMS.txt"
if [[ ! -f "$checksum_file" ]]; then
  echo "missing SHA256SUMS.txt in ${dist_dir}" >&2
  exit 1
fi

archive_count="$(
  find "$dist_dir" -maxdepth 1 -type f \( -name "*.tar.gz" -o -name "*.zip" \) \
    | wc -l \
    | tr -d " "
)"
if [[ "$archive_count" != "$expected_archives" ]]; then
  echo "expected ${expected_archives} release archives, got ${archive_count}" >&2
  find "$dist_dir" -maxdepth 1 -type f | sort >&2
  exit 1
fi

while IFS= read -r archive; do
  archive_name="$(basename "$archive")"
  if ! grep -Fq "  ${archive_name}" "$checksum_file"; then
    echo "archive ${archive_name} is missing from SHA256SUMS.txt" >&2
    exit 1
  fi
done < <(find "$dist_dir" -maxdepth 1 -type f \( -name "*.tar.gz" -o -name "*.zip" \) | sort)

echo "release smoke passed: ${archive_count} archives in ${dist_dir}"

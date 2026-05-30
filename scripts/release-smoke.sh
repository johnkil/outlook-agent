#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

tmp_root="${TMPDIR:-/tmp}"
tmp_root="${tmp_root%/}"
dist_dir="${OUTLOOK_AGENT_DIST_DIR:-$(mktemp -d "${tmp_root}/outlook-agent-release-smoke.XXXXXX")}"
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

scripts/release-verify.sh "$dist_dir"

host_goos="$(go env GOHOSTOS)"
host_goarch="$(go env GOHOSTARCH)"
host_archive="${dist_dir}/outlook-agent_smoke_${host_goos}_${host_goarch}.tar.gz"
if [[ -f "$host_archive" ]]; then
  run_dir="$(mktemp -d "${tmp_root}/outlook-agent-release-run.XXXXXX")"
  project_dir="$(mktemp -d "${tmp_root}/outlook-agent-release-project.XXXXXX")"
  tar -xzf "$host_archive" -C "$run_dir"
  host_binary="${run_dir}/outlook-agent_smoke_${host_goos}_${host_goarch}/outlook-agent"
  version_output="$("$host_binary" version)"
  doctor_output="$("$host_binary" doctor)"
  setup_output="$(cd "$project_dir" && "$host_binary" setup opencode plan --config .local/outlook-agent.json)"
  rm -rf "$run_dir" "$project_dir"
  if ! grep -Fq '"version": "smoke"' <<<"$version_output"; then
    echo "host archive version output did not include embedded smoke version" >&2
    echo "$version_output" >&2
    exit 1
  fi
  if ! grep -Fq '"built_by": "release-build"' <<<"$version_output"; then
    echo "host archive version output did not include release-build metadata" >&2
    echo "$version_output" >&2
    exit 1
  fi
  if ! grep -Fq '"version": "smoke"' <<<"$doctor_output"; then
    echo "host archive doctor output did not include embedded smoke version" >&2
    echo "$doctor_output" >&2
    exit 1
  fi
  if ! grep -Fq ".opencode/skills/outlook-mail/SKILL.md" <<<"$setup_output"; then
    echo "host archive setup opencode plan did not include bundled skills" >&2
    echo "$setup_output" >&2
    exit 1
  fi
fi

echo "release smoke passed: ${archive_count} archives in ${dist_dir}"

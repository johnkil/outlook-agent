#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

dist_dir="${1:-${OUTLOOK_AGENT_DIST_DIR:-${repo_root}/dist}}"
checksum_file="${dist_dir}/SHA256SUMS.txt"
signature_file="${dist_dir}/SHA256SUMS.txt.asc"

if [[ ! -d "$dist_dir" ]]; then
  echo "release dist directory does not exist: ${dist_dir}" >&2
  exit 1
fi
if [[ ! -f "$checksum_file" ]]; then
  echo "missing SHA256SUMS.txt in ${dist_dir}" >&2
  exit 1
fi

checksum() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  else
    shasum -a 256 "$file" | awk '{print $1}'
  fi
}

archive_count=0
while IFS= read -r line || [[ -n "$line" ]]; do
  [[ -z "$line" ]] && continue
  read -r expected archive_name extra <<<"$line"
  if [[ -z "${expected:-}" || -z "${archive_name:-}" || -n "${extra:-}" ]]; then
    echo "invalid checksum line: ${line}" >&2
    exit 1
  fi
  if [[ "$archive_name" == */* || "$archive_name" == .* ]]; then
    echo "unsafe checksum archive name: ${archive_name}" >&2
    exit 1
  fi
  archive_path="${dist_dir}/${archive_name}"
  if [[ ! -f "$archive_path" ]]; then
    echo "checksum references missing archive: ${archive_name}" >&2
    exit 1
  fi
  actual="$(checksum "$archive_path")"
  if [[ "$actual" != "$expected" ]]; then
    echo "checksum mismatch for ${archive_name}" >&2
    echo "expected ${expected}" >&2
    echo "actual   ${actual}" >&2
    exit 1
  fi
  archive_count=$((archive_count + 1))
done < "$checksum_file"

if [[ "$archive_count" -eq 0 ]]; then
  echo "SHA256SUMS.txt did not list any archives" >&2
  exit 1
fi

while IFS= read -r archive; do
  archive_name="$(basename "$archive")"
  if ! grep -Fq "  ${archive_name}" "$checksum_file"; then
    echo "archive ${archive_name} is missing from SHA256SUMS.txt" >&2
    exit 1
  fi
done < <(find "$dist_dir" -maxdepth 1 -type f \( -name "*.tar.gz" -o -name "*.zip" \) | sort)

signature_status="absent"
if [[ -f "$signature_file" ]]; then
  if command -v gpg >/dev/null 2>&1; then
    gpg --verify "$signature_file" "$checksum_file" >/dev/null
    signature_status="verified"
  else
    signature_status="skipped: gpg unavailable"
  fi
fi

echo "release verify passed: ${archive_count} archives in ${dist_dir}; signature=${signature_status}"

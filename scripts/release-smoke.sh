#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

tmp_root="${TMPDIR:-/tmp}"
tmp_root="${tmp_root%/}"
dist_dir="${OUTLOOK_AGENT_DIST_DIR:-$(mktemp -d "${tmp_root}/outlook-agent-release-smoke.XXXXXX")}"
expected_archives=6
smoke_version="v0.0.0-smoke"
run_dir=""
project_dir=""

cleanup() {
  if [[ -n "$run_dir" ]]; then
    rm -rf "$run_dir"
  fi
  if [[ -n "$project_dir" ]]; then
    rm -rf "$project_dir"
  fi
  if [[ -z "${OUTLOOK_AGENT_KEEP_RELEASE_SMOKE:-}" ]]; then
    rm -rf "$dist_dir"
  fi
}
trap cleanup EXIT

OUTLOOK_AGENT_DIST_DIR="$dist_dir" scripts/release-build.sh "$smoke_version"

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

dependency_manifest_count="$(
  find "$dist_dir" -maxdepth 1 -type f -name "*_dependency-manifest.json" \
    | wc -l \
    | tr -d " "
)"
if [[ "$dependency_manifest_count" != "1" ]]; then
  echo "expected one dependency-manifest release artifact, got ${dependency_manifest_count}" >&2
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

dependency_manifest_name="$(basename "$(find "$dist_dir" -maxdepth 1 -type f -name "*_dependency-manifest.json" | sort | head -n 1)")"
if ! grep -Fq "  ${dependency_manifest_name}" "$checksum_file"; then
  echo "dependency-manifest ${dependency_manifest_name} is missing from SHA256SUMS.txt" >&2
  exit 1
fi

scripts/release-verify.sh "$dist_dir"

host_goos="$(go env GOHOSTOS)"
host_goarch="$(go env GOHOSTARCH)"
host_archive="${dist_dir}/outlook-agent_${smoke_version}_${host_goos}_${host_goarch}.tar.gz"
if [[ -f "$host_archive" ]]; then
  run_dir="$(mktemp -d "${tmp_root}/outlook-agent-release-run.XXXXXX")"
  project_dir="$(mktemp -d "${tmp_root}/outlook-agent-release-project.XXXXXX")"
  tar -xzf "$host_archive" -C "$run_dir"
  host_binary="${run_dir}/outlook-agent_${smoke_version}_${host_goos}_${host_goarch}/outlook-agent"
  mkdir -p "${project_dir}/.local"
  config="${project_dir}/.local/outlook-agent.json"
  printf '%s\n' '{"default_profile":"work","profiles":{"work":{"transport":"fake"}}}' > "$config"
  version_output="$("$host_binary" version)"
  doctor_output="$("$host_binary" doctor)"
  setup_output="$(cd "$project_dir" && "$host_binary" setup opencode plan --config .local/outlook-agent.json)"
  people_output="$("$host_binary" people search teammate --config "$config")"
  find_time_output="$("$host_binary" calendar find-time --attendee teammate@example.com --start 2026-05-28T09:00:00Z --end 2026-05-28T12:00:00Z --duration 30 --config "$config")"
  mcp_tools_output="$(
    OUTLOOK_AGENT_BINARY_UNDER_TEST="$host_binary" OUTLOOK_AGENT_SMOKE_CONFIG="$config" python3 - <<'PY'
import json
import os
import select
import subprocess


READ_TIMEOUT_SECONDS = float(os.environ.get("OUTLOOK_AGENT_MCP_SMOKE_TIMEOUT_SECONDS", "10"))


def write_message(proc, payload):
    line = json.dumps(payload, separators=(",", ":")).encode("utf-8") + b"\n"
    proc.stdin.write(line)
    proc.stdin.flush()


def read_message(proc):
    ready, _, _ = select.select([proc.stdout], [], [], READ_TIMEOUT_SECONDS)
    if not ready:
        state = "running" if proc.poll() is None else "exited"
        raise RuntimeError(
            f"MCP server timed out after {READ_TIMEOUT_SECONDS:g}s waiting for stdout line; process={state}"
        )
    line = proc.stdout.readline()
    if not line:
        stderr = ""
        if proc.poll() is not None:
            stderr = proc.stderr.read().decode("utf-8", errors="replace")
        raise RuntimeError("MCP server closed stdout; stderr=" + stderr)
    return json.loads(line.decode("utf-8"))


binary = os.environ["OUTLOOK_AGENT_BINARY_UNDER_TEST"]
config = os.environ["OUTLOOK_AGENT_SMOKE_CONFIG"]
proc = subprocess.Popen(
    [binary, "--config", config, "mcp"],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
)
try:
    write_message(proc, {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2025-06-18",
            "capabilities": {},
            "clientInfo": {"name": "release-smoke", "version": "0.0.1"},
        },
    })
    initialize = read_message(proc)
    if "error" in initialize:
        raise RuntimeError("initialize failed: " + json.dumps(initialize["error"], sort_keys=True))
    write_message(proc, {"jsonrpc": "2.0", "method": "notifications/initialized", "params": {}})
    write_message(proc, {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}})
    tools = read_message(proc)
    if "error" in tools:
        raise RuntimeError("tools/list failed: " + json.dumps(tools["error"], sort_keys=True))
    print(json.dumps(tools, sort_keys=True))
finally:
    proc.terminate()
    try:
        proc.wait(timeout=2)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait(timeout=2)
PY
  )"
  OUTLOOK_AGENT_BINARY_UNDER_TEST="$host_binary" go test ./cmd/outlook-agent -run '^TestBinaryMCPStdioUsesConfiguredDefaultProfile$' -count=1
  if ! grep -Fq "\"version\": \"${smoke_version}\"" <<<"$version_output"; then
    echo "host archive version output did not include embedded smoke version" >&2
    echo "$version_output" >&2
    exit 1
  fi
  if ! grep -Fq '"built_by": "release-build"' <<<"$version_output"; then
    echo "host archive version output did not include release-build metadata" >&2
    echo "$version_output" >&2
    exit 1
  fi
  if ! grep -Fq "\"version\": \"${smoke_version}\"" <<<"$doctor_output"; then
    echo "host archive doctor output did not include embedded smoke version" >&2
    echo "$doctor_output" >&2
    exit 1
  fi
  if ! grep -Fq ".opencode/skills/outlook-mail/SKILL.md" <<<"$setup_output"; then
    echo "host archive setup opencode plan did not include bundled skills" >&2
    echo "$setup_output" >&2
    exit 1
  fi
  if ! grep -Fq '"command": "people search"' <<<"$people_output"; then
    echo "host archive people search output did not include typed command marker" >&2
    echo "$people_output" >&2
    exit 1
  fi
  if ! grep -Fq '"command": "calendar find-time"' <<<"$find_time_output"; then
    echo "host archive calendar find-time output did not include typed command marker" >&2
    echo "$find_time_output" >&2
    exit 1
  fi
  if ! grep -Fq '"name": "outlook.calendar_create_meeting"' <<<"$mcp_tools_output"; then
    echo "host archive MCP tools/list did not include outlook.calendar_create_meeting" >&2
    echo "$mcp_tools_output" >&2
    exit 1
  fi
fi

echo "release smoke passed: ${archive_count} archives in ${dist_dir}"

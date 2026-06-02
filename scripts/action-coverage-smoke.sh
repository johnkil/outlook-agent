#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

tmp_root="${TMPDIR:-/tmp}"
tmp_root="${tmp_root%/}"
work_dir="$(mktemp -d "${tmp_root}/outlook-agent-action-coverage.XXXXXX")"
coverage_json="${work_dir}/coverage.json"
auth_json="${work_dir}/auth.json"
opencode_jsonl="${work_dir}/opencode.jsonl"
opencode_config_dir="${work_dir}/opencode-config"

cleanup() {
  if [[ -z "${OUTLOOK_AGENT_KEEP_ACTION_COVERAGE_SMOKE:-}" ]]; then
    rm -rf "$work_dir"
  fi
}
trap cleanup EXIT

require_command() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "missing required command: ${name}" >&2
    exit 1
  fi
}

run_agent() {
  if [[ -n "${OUTLOOK_AGENT_BIN:-}" ]]; then
    "$OUTLOOK_AGENT_BIN" "$@"
    return
  fi
  go run ./cmd/outlook-agent "$@"
}

require_command jq

run_agent policy coverage > "$coverage_json"

jq -e '
  .ok == true
  and .command == "policy coverage"
  and (.summary.total == (.actions | length))
  and (.summary.total > 0)
  and (.summary.by_transport.owa == 69)
  and ([.actions[] | select((.execution_route // "") == "" or (.live_check_level // "") == "")] | length == 0)
  and ([.actions[] | select(.safety_class == "read_metadata" and .live_check_level != "live_readonly")] | length == 0)
  and ([.actions[] | select((.safety_class == "read_body_explicit" or .safety_class == "read_attachment_explicit") and .live_check_level != "manual_explicit_target")] | length == 0)
  and ([.actions[] | select(.safety_class == "draft_only" and .live_check_level != "live_safe_execute")] | length == 0)
  and ([.actions[] | select((.safety_class == "reversible_single_item" or .safety_class == "reversible_bulk") and .live_check_level != "live_dry_run")] | length == 0)
  and ([.actions[] | select((.safety_class == "destructive" or .safety_class == "send_like" or .safety_class == "settings_or_rules" or .safety_class == "unknown") and .live_check_level != "live_guard_only")] | length == 0)
  and ([.actions[] | select(.safety_class == "destructive" and (.requires_unsafe != true or .execution_route != "unsafe_dry_run_confirm"))] | length == 0)
  and ([.actions[] | select(.transport == "owa" and .action == "DeleteItem" and .requires_unsafe == true and .live_check_level == "live_guard_only")] | length == 1)
  and ([.actions[] | select(.transport == "owa" and .action == "mail.search" and .allowed_direct == true and .live_check_level == "live_readonly")] | length == 1)
' "$coverage_json" >/dev/null

live_auth_ok="skipped"
if [[ -n "${OUTLOOK_AGENT_LIVE_CONFIG:-}" ]]; then
  live_args=(--config "$OUTLOOK_AGENT_LIVE_CONFIG")
  if [[ -n "${OUTLOOK_AGENT_LIVE_PROFILE:-}" ]]; then
    live_args+=(--profile "$OUTLOOK_AGENT_LIVE_PROFILE")
  fi
  run_agent "${live_args[@]}" auth check > "$auth_json"
  jq -e '.ok == true and (.principal // "") != ""' "$auth_json" >/dev/null
  live_auth_ok="true"
fi

opencode_ok="skipped"
if [[ -n "${OUTLOOK_AGENT_OPENCODE_LIVE_DIR:-}" ]]; then
  require_command opencode
  mkdir -p "$opencode_config_dir"
  cat > "${opencode_config_dir}/opencode.json" <<'JSON'
{
  "$schema": "https://opencode.ai/config.json",
  "permission": {
    "outlook-agent_outlook_*": "deny",
    "outlook-agent_outlook_auth_check": "allow",
    "outlook-agent_outlook_capabilities": "allow",
    "outlook-agent_outlook_action_dry_run": "allow"
  }
}
JSON
  if [[ -z "${OUTLOOK_AGENT_OPENCODE_MODEL:-}" ]]; then
    echo "OUTLOOK_AGENT_OPENCODE_MODEL is required when OUTLOOK_AGENT_OPENCODE_LIVE_DIR is set" >&2
    exit 1
  fi
  model="$OUTLOOK_AGENT_OPENCODE_MODEL"
  (
    cd "$OUTLOOK_AGENT_OPENCODE_LIVE_DIR"
    env -u OPENCODE_CONFIG OPENCODE_CONFIG_DIR="$opencode_config_dir" opencode run \
      --model "$model" \
      --format json \
      --title outlook-agent-action-coverage-smoke \
      'Use outlook-agent MCP only. Run a safe action-coverage smoke: call outlook.auth_check, outlook.capabilities, and outlook.action_dry_run for action "DeleteItem" with payload {"Body":{"ItemIds":[{"Id":"dry-run-item"}],"DeleteType":"HardDelete"}} first with unsafe_mode false and then with unsafe_mode true. Do not call outlook.action_confirm. Do not execute any delete, move, send, body-read, or attachment-content action. Final answer must contain only sanitized booleans and counts.' \
      > "$opencode_jsonl"
  )
  jq -s -e '
    def event_part: .part // .;
    def completed($tool):
      [
        .[]
        | select(.type == "tool_use" or .type == "tool")
        | (event_part) as $part
        | select($part.tool == $tool and $part.state.status == "completed")
      ]
      | length;
    completed("outlook-agent_outlook_auth_check") == 1
    and completed("outlook-agent_outlook_capabilities") == 1
    and completed("outlook-agent_outlook_action_dry_run") >= 2
  ' "$opencode_jsonl" >/dev/null
  forbidden_tools="$(
    jq -r -s '
      def event_part: .part // .;
      [
        .[]
        | select(.type == "tool_use" or .type == "tool")
        | (event_part) as $part
        | ($part.tool // empty)
        | select(startswith("outlook-agent_outlook_"))
        | select(
            . != "outlook-agent_outlook_auth_check"
            and . != "outlook-agent_outlook_capabilities"
            and . != "outlook-agent_outlook_action_dry_run"
          )
      ]
      | unique
      | .[]
    ' "$opencode_jsonl"
  )"
  if [[ -n "$forbidden_tools" ]]; then
    echo "forbidden opencode tool calls:" >&2
    while IFS= read -r tool_name; do
      echo "- ${tool_name}" >&2
    done <<< "$forbidden_tools"
    exit 1
  fi
  jq -s -e '
    def part: .part // .;
    def parse_output($value):
      if ($value | type) == "object" then
        if ($value.structuredContent? != null) then
          parse_output($value.structuredContent)
        elif ($value.structured_content? != null) then
          parse_output($value.structured_content)
        elif ($value.content? | type) == "array" then
          parse_output($value.content)
        else
          $value
        end
      elif ($value | type) == "string" then
        ($value | fromjson? // {})
      elif ($value | type) == "array" then
        (([$value[]? | .text? | fromjson? // empty] | first) // {})
      else
        {}
      end;
    def tool_output($part):
      parse_output($part.state.output // $part.state.result // $part.output // $part.result // {});
    def dry_run_call($unsafe):
      [
        .[]
        | select(.type == "tool_use" or .type == "tool")
        | (part) as $part
        | select($part.tool == "outlook-agent_outlook_action_dry_run")
        | select($part.state.status == "completed")
        | select($part.state.input.action == "DeleteItem")
        | select($part.state.input.payload.Body.DeleteType == "HardDelete")
        | select($part.state.input.payload.Body.ItemIds[0].Id == "dry-run-item")
        | select($part.state.input.unsafe_mode == $unsafe)
        | (tool_output($part)) as $output
        | select(
            if $unsafe then
              $output.ok == true
              and (($output.confirmation_token // "") != "")
              and (($output.requires_unsafe // false) == false)
            else
              $output.ok == false
              and $output.requires_unsafe == true
              and (($output.confirmation_token // "") == "")
              and (($output.error // "") != "")
            end
          )
      ]
      | length;
    dry_run_call(false) >= 1
    and dry_run_call(true) >= 1
  ' "$opencode_jsonl" >/dev/null || {
    echo "missing destructive DeleteItem dry-run checks" >&2
    exit 1
  }
  opencode_ok="true"
fi

jq -n \
  --arg live_auth "$live_auth_ok" \
  --arg opencode "$opencode_ok" \
  --slurpfile coverage "$coverage_json" \
  '{
    ok: true,
    command: "action coverage smoke",
    policy_coverage: {
      total: $coverage[0].summary.total,
      by_transport: $coverage[0].summary.by_transport,
      by_live_check_level: $coverage[0].summary.by_live_check_level
    },
    live_auth: $live_auth,
    opencode_mcp_smoke: $opencode
  }'

package app_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseReadinessArtifactsExist(t *testing.T) {
	requiredFiles := map[string][]string{
		filepath.Join("..", "..", "docs", "RELEASE.md"): {
			"# Release Process",
			"scripts/ci-local.sh",
			"scripts/release-smoke.sh",
			"scripts/release-build.sh",
			"SHA256SUMS.txt",
			"OUTLOOK_AGENT_SIGN_RELEASE",
		},
		filepath.Join("..", "..", "scripts", "ci-local.sh"): {
			"-path \"./.cache\"",
			"gofmt -l",
			"go test -count=1 ./...",
			"go test -race ./...",
			"go vet ./...",
			"go build",
			"honnef.co/go/tools/cmd/staticcheck@v0.7.0",
			"scripts/public-safety-check.sh",
			"golang.org/x/vuln/cmd/govulncheck@v1.3.0",
		},
		filepath.Join("..", "..", "scripts", "release-smoke.sh"): {
			"TMPDIR",
			"OUTLOOK_AGENT_DIST_DIR",
			"scripts/release-build.sh",
			"SHA256SUMS.txt",
			"expected_archives=6",
			"\"version\": \"smoke\"",
		},
		filepath.Join("..", "..", "scripts", "release-build.sh"): {
			"GOOS",
			"GOARCH",
			"internal/buildinfo.Version",
			"SHA256SUMS.txt",
			"OUTLOOK_AGENT_SIGN_RELEASE",
		},
		filepath.Join("..", "..", "scripts", "public-safety-check.sh"): {
			"OUTLOOK_AGENT_PUBLIC_SAFETY_PATTERN",
			"rg --hidden",
			"forbidden generated artifact",
		},
		filepath.Join("..", "..", "scripts", "action-coverage-smoke.sh"): {
			"policy coverage",
			"live_check_level",
			"OUTLOOK_AGENT_LIVE_CONFIG",
			"OUTLOOK_AGENT_OPENCODE_LIVE_DIR",
			"OUTLOOK_AGENT_OPENCODE_MODEL is required",
			"outlook.action_dry_run",
		},
		filepath.Join("..", "..", ".github", "workflows", "ci.yml"): {
			"go test -count=1 ./...",
			"go test -race ./...",
			"go vet ./...",
			"honnef.co/go/tools/cmd/staticcheck@v0.7.0",
			"golang.org/x/vuln/cmd/govulncheck@v1.3.0",
			"scripts/public-safety-check.sh",
		},
		filepath.Join("..", "..", ".github", "workflows", "release.yml"): {
			"scripts/release-build.sh",
			"gh release",
			"contents: write",
		},
	}

	for path, markers := range requiredFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read release readiness artifact %s: %v", path, err)
		}
		text := string(data)
		for _, marker := range markers {
			if !strings.Contains(text, marker) {
				t.Fatalf("expected %s to contain %q", path, marker)
			}
		}
	}
}

func TestInstallScriptReadinessMarkers(t *testing.T) {
	path := filepath.Join("..", "..", "install.sh")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read install script %s: %v", path, err)
	}
	text := string(data)
	for _, marker := range []string{
		`REPO="johnkil/outlook-agent"`,
		`BIN_NAME="outlook-agent"`,
		"OUTLOOK_AGENT_VERSION",
		"OUTLOOK_AGENT_INSTALL_DIR",
		"SHA256SUMS.txt",
		"refusing to overwrite symlink",
		"shasum -a 256",
		"validate_tar_members",
		"validate_tar_binary_type",
		"tar -tzf",
		"tar -tvzf",
		"expected binary archive member is not a regular file",
		"unsafe archive member",
		"unexpected archive member",
		`tar -xzf "$archive_name" "$expected_binary_member"`,
		`[ -f "$binary_path" ] && [ ! -L "$binary_path" ]`,
		`install_tmp="$(mktemp "${install_dir}/.${BIN_NAME}.tmp.XXXXXX")"`,
		`mv "$install_tmp" "$target_path"`,
		"outlook-agent help",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("expected %s to contain %q", path, marker)
		}
	}
}

func TestGitHubWorkflowActionsArePinnedByCommitSHA(t *testing.T) {
	workflowFiles, err := githubWorkflowFiles(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("discover GitHub workflows: %v", err)
	}
	if len(workflowFiles) == 0 {
		t.Fatal("expected GitHub workflow files")
	}

	for _, path := range workflowFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read workflow %s: %v", path, err)
		}
		for lineNumber, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "uses:") {
				continue
			}
			reference := strings.TrimSpace(strings.TrimPrefix(trimmed, "uses:"))
			reference = strings.Fields(reference)[0]
			name, ref, ok := strings.Cut(reference, "@")
			if !ok || !isFullCommitSHA(ref) {
				t.Fatalf("workflow %s:%d uses mutable or unpinned action reference %q; pin %s to a full commit SHA", path, lineNumber+1, reference, name)
			}
			if !hasSpecificVersionComment(line) {
				t.Fatalf("workflow %s:%d pins %s to a SHA without a specific semver release comment; use %s@<sha> # vX.Y.Z", path, lineNumber+1, name, name)
			}
		}
	}
}

func githubWorkflowFiles(repoRoot string) ([]string, error) {
	var workflowFiles []string
	for _, extension := range []string{"*.yml", "*.yaml"} {
		matches, err := filepath.Glob(filepath.Join(repoRoot, ".github", "workflows", extension))
		if err != nil {
			return nil, err
		}
		workflowFiles = append(workflowFiles, matches...)
	}
	return workflowFiles, nil
}

func TestGitHubWorkflowFileDiscoveryIncludesYAMLExtension(t *testing.T) {
	repoRoot := t.TempDir()
	workflowDir := filepath.Join(repoRoot, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("create workflow fixture dir: %v", err)
	}
	for _, name := range []string{"ci.yml", "release.yaml", "README.md"} {
		path := filepath.Join(workflowDir, name)
		if err := os.WriteFile(path, []byte("name: fixture\n"), 0o644); err != nil {
			t.Fatalf("write workflow fixture %s: %v", name, err)
		}
	}

	workflowFiles, err := githubWorkflowFiles(repoRoot)
	if err != nil {
		t.Fatalf("discover workflow files: %v", err)
	}

	var basenames []string
	for _, path := range workflowFiles {
		basenames = append(basenames, filepath.Base(path))
	}
	want := []string{"ci.yml", "release.yaml"}
	if strings.Join(basenames, ",") != strings.Join(want, ",") {
		t.Fatalf("expected workflow files %v, got %v", want, basenames)
	}
}

func isFullCommitSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, char := range value {
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') {
			continue
		}
		return false
	}
	return true
}

func hasSpecificVersionComment(line string) bool {
	commentIndex := strings.Index(line, "#")
	if commentIndex == -1 {
		return false
	}
	comment := strings.TrimSpace(line[commentIndex+1:])
	parts := strings.Split(comment, ".")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "v") {
		return false
	}
	return len(parts[0]) > 1 && isDecimal(parts[0][1:]) && isDecimal(parts[1]) && isDecimal(parts[2])
}

func isDecimal(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func TestActionCoverageSmokeRejectsForbiddenOpencodeToolCalls(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for action coverage smoke")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is required for action coverage smoke")
	}

	repoRoot := filepath.Join("..", "..")
	tempDir := t.TempDir()
	coveragePath := filepath.Join(tempDir, "coverage.json")
	fakeAgentPath := filepath.Join(tempDir, "outlook-agent")
	fakeOpencodePath := filepath.Join(tempDir, "opencode")
	liveDir := filepath.Join(tempDir, "opencode-live")
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("create fake opencode live dir: %v", err)
	}

	writeCoverageFixture(t, coveragePath)
	fakeAgent := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$*\" == \"policy coverage\" ]]; then\n  cat " + shellQuote(coveragePath) + "\nelse\n  echo \"unexpected fake outlook-agent args: $*\" >&2\n  exit 2\nfi\n"
	if err := os.WriteFile(fakeAgentPath, []byte(fakeAgent), 0o755); err != nil {
		t.Fatalf("write fake outlook-agent: %v", err)
	}
	fakeOpencode := `#!/usr/bin/env bash
set -euo pipefail
cat <<'JSONL'
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_auth_check","state":{"status":"completed"}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_capabilities","state":{"status":"completed"}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed"}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed"}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_confirm","state":{"status":"completed"}}}
JSONL
`
	if err := os.WriteFile(fakeOpencodePath, []byte(fakeOpencode), 0o755); err != nil {
		t.Fatalf("write fake opencode: %v", err)
	}

	command := exec.Command("bash", filepath.Join("scripts", "action-coverage-smoke.sh"))
	command.Dir = repoRoot
	command.Env = append(os.Environ(),
		"OUTLOOK_AGENT_BIN="+fakeAgentPath,
		"OUTLOOK_AGENT_OPENCODE_LIVE_DIR="+liveDir,
		"OUTLOOK_AGENT_OPENCODE_MODEL=example/test-model",
		"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("expected action coverage smoke to reject forbidden opencode tool call, output=%s", string(output))
	}
	if !strings.Contains(string(output), "forbidden opencode tool calls") {
		t.Fatalf("expected forbidden opencode tool call error, got err=%v output=%s", err, string(output))
	}
}

func TestActionCoverageSmokeRejectsForbiddenTopLevelOpencodeToolCalls(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for action coverage smoke")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is required for action coverage smoke")
	}

	repoRoot := filepath.Join("..", "..")
	tempDir := t.TempDir()
	coveragePath := filepath.Join(tempDir, "coverage.json")
	fakeAgentPath := filepath.Join(tempDir, "outlook-agent")
	fakeOpencodePath := filepath.Join(tempDir, "opencode")
	liveDir := filepath.Join(tempDir, "opencode-live")
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("create fake opencode live dir: %v", err)
	}

	writeCoverageFixture(t, coveragePath)
	fakeAgent := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$*\" == \"policy coverage\" ]]; then\n  cat " + shellQuote(coveragePath) + "\nelse\n  echo \"unexpected fake outlook-agent args: $*\" >&2\n  exit 2\nfi\n"
	if err := os.WriteFile(fakeAgentPath, []byte(fakeAgent), 0o755); err != nil {
		t.Fatalf("write fake outlook-agent: %v", err)
	}
	fakeOpencode := `#!/usr/bin/env bash
set -euo pipefail
cat <<'JSONL'
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_auth_check","state":{"status":"completed","input":{}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_capabilities","state":{"status":"completed","input":{}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed","input":{"action":"DeleteItem","payload":{"Body":{"ItemIds":[{"Id":"dry-run-item"}],"DeleteType":"HardDelete"}},"unsafe_mode":false}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed","input":{"action":"DeleteItem","payload":{"Body":{"ItemIds":[{"Id":"dry-run-item"}],"DeleteType":"HardDelete"}},"unsafe_mode":true}}}}
{"type":"tool","tool":"outlook-agent_outlook_action_confirm","state":{"status":"completed","input":{}}}
JSONL
`
	if err := os.WriteFile(fakeOpencodePath, []byte(fakeOpencode), 0o755); err != nil {
		t.Fatalf("write fake opencode: %v", err)
	}

	command := exec.Command("bash", filepath.Join("scripts", "action-coverage-smoke.sh"))
	command.Dir = repoRoot
	command.Env = append(os.Environ(),
		"OUTLOOK_AGENT_BIN="+fakeAgentPath,
		"OUTLOOK_AGENT_OPENCODE_LIVE_DIR="+liveDir,
		"OUTLOOK_AGENT_OPENCODE_MODEL=example/test-model",
		"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("expected action coverage smoke to reject forbidden top-level opencode tool call, output=%s", string(output))
	}
	if !strings.Contains(string(output), "forbidden opencode tool calls") {
		t.Fatalf("expected forbidden opencode tool call error, got err=%v output=%s", err, string(output))
	}
}

func TestActionCoverageSmokeRequiresDestructiveDryRunInputs(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for action coverage smoke")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is required for action coverage smoke")
	}

	repoRoot := filepath.Join("..", "..")
	tempDir := t.TempDir()
	coveragePath := filepath.Join(tempDir, "coverage.json")
	fakeAgentPath := filepath.Join(tempDir, "outlook-agent")
	fakeOpencodePath := filepath.Join(tempDir, "opencode")
	liveDir := filepath.Join(tempDir, "opencode-live")
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("create fake opencode live dir: %v", err)
	}

	writeCoverageFixture(t, coveragePath)
	fakeAgent := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$*\" == \"policy coverage\" ]]; then\n  cat " + shellQuote(coveragePath) + "\nelse\n  echo \"unexpected fake outlook-agent args: $*\" >&2\n  exit 2\nfi\n"
	if err := os.WriteFile(fakeAgentPath, []byte(fakeAgent), 0o755); err != nil {
		t.Fatalf("write fake outlook-agent: %v", err)
	}
	fakeOpencode := `#!/usr/bin/env bash
set -euo pipefail
cat <<'JSONL'
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_auth_check","state":{"status":"completed","input":{}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_capabilities","state":{"status":"completed","input":{}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed","input":{"action":"mail.search","payload":{"query":"dry-run-item"},"unsafe_mode":false}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed","input":{"action":"mail.search","payload":{"query":"dry-run-item"},"unsafe_mode":true}}}}
JSONL
`
	if err := os.WriteFile(fakeOpencodePath, []byte(fakeOpencode), 0o755); err != nil {
		t.Fatalf("write fake opencode: %v", err)
	}

	command := exec.Command("bash", filepath.Join("scripts", "action-coverage-smoke.sh"))
	command.Dir = repoRoot
	command.Env = append(os.Environ(),
		"OUTLOOK_AGENT_BIN="+fakeAgentPath,
		"OUTLOOK_AGENT_OPENCODE_LIVE_DIR="+liveDir,
		"OUTLOOK_AGENT_OPENCODE_MODEL=example/test-model",
		"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("expected action coverage smoke to reject wrong dry-run inputs, output=%s", string(output))
	}
	if !strings.Contains(string(output), "missing destructive DeleteItem dry-run checks") {
		t.Fatalf("expected missing destructive dry-run error, got err=%v output=%s", err, string(output))
	}
}

func TestActionCoverageSmokeRejectsUnsafeFalseDryRunToken(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for action coverage smoke")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is required for action coverage smoke")
	}

	repoRoot := filepath.Join("..", "..")
	tempDir := t.TempDir()
	coveragePath := filepath.Join(tempDir, "coverage.json")
	fakeAgentPath := filepath.Join(tempDir, "outlook-agent")
	fakeOpencodePath := filepath.Join(tempDir, "opencode")
	liveDir := filepath.Join(tempDir, "opencode-live")
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("create fake opencode live dir: %v", err)
	}

	writeCoverageFixture(t, coveragePath)
	fakeAgent := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$*\" == \"policy coverage\" ]]; then\n  cat " + shellQuote(coveragePath) + "\nelse\n  echo \"unexpected fake outlook-agent args: $*\" >&2\n  exit 2\nfi\n"
	if err := os.WriteFile(fakeAgentPath, []byte(fakeAgent), 0o755); err != nil {
		t.Fatalf("write fake outlook-agent: %v", err)
	}
	fakeOpencode := `#!/usr/bin/env bash
set -euo pipefail
cat <<'JSONL'
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_auth_check","state":{"status":"completed","input":{}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_capabilities","state":{"status":"completed","input":{}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed","input":{"action":"DeleteItem","payload":{"Body":{"ItemIds":[{"Id":"dry-run-item"}],"DeleteType":"HardDelete"}},"unsafe_mode":false},"output":{"action":"DeleteItem","ok":true,"count":1,"reversible":false,"requires_confirmation":true,"confirmation_token":"bad-token"}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed","input":{"action":"DeleteItem","payload":{"Body":{"ItemIds":[{"Id":"dry-run-item"}],"DeleteType":"HardDelete"}},"unsafe_mode":true},"output":{"action":"DeleteItem","ok":true,"count":1,"reversible":false,"requires_confirmation":true,"confirmation_token":"unsafe-token"}}}}
JSONL
`
	if err := os.WriteFile(fakeOpencodePath, []byte(fakeOpencode), 0o755); err != nil {
		t.Fatalf("write fake opencode: %v", err)
	}

	command := exec.Command("bash", filepath.Join("scripts", "action-coverage-smoke.sh"))
	command.Dir = repoRoot
	command.Env = append(os.Environ(),
		"OUTLOOK_AGENT_BIN="+fakeAgentPath,
		"OUTLOOK_AGENT_OPENCODE_LIVE_DIR="+liveDir,
		"OUTLOOK_AGENT_OPENCODE_MODEL=example/test-model",
		"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("expected action coverage smoke to reject unsafe=false dry-run token, output=%s", string(output))
	}
	if !strings.Contains(string(output), "missing destructive DeleteItem dry-run checks") {
		t.Fatalf("expected missing destructive dry-run error, got err=%v output=%s", err, string(output))
	}
}

func TestActionCoverageSmokeAppliesOpencodePermissionOverlay(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for action coverage smoke")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is required for action coverage smoke")
	}

	repoRoot := filepath.Join("..", "..")
	tempDir := t.TempDir()
	coveragePath := filepath.Join(tempDir, "coverage.json")
	fakeAgentPath := filepath.Join(tempDir, "outlook-agent")
	fakeOpencodePath := filepath.Join(tempDir, "opencode")
	liveDir := filepath.Join(tempDir, "opencode-live")
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("create fake opencode live dir: %v", err)
	}

	writeCoverageFixture(t, coveragePath)
	fakeAgent := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$*\" == \"policy coverage\" ]]; then\n  cat " + shellQuote(coveragePath) + "\nelse\n  echo \"unexpected fake outlook-agent args: $*\" >&2\n  exit 2\nfi\n"
	if err := os.WriteFile(fakeAgentPath, []byte(fakeAgent), 0o755); err != nil {
		t.Fatalf("write fake outlook-agent: %v", err)
	}
	fakeOpencode := `#!/usr/bin/env bash
set -euo pipefail
if [[ -z "${OPENCODE_CONFIG_DIR:-}" ]]; then
  echo "missing OPENCODE_CONFIG_DIR" >&2
  exit 9
fi
if [[ -n "${OPENCODE_CONFIG:-}" ]]; then
  echo "OPENCODE_CONFIG must not override smoke permissions" >&2
  exit 9
fi
jq -e '
  .permission["outlook-agent_outlook_*"] == "deny"
  and .permission["outlook-agent_outlook_auth_check"] == "allow"
  and .permission["outlook-agent_outlook_capabilities"] == "allow"
  and .permission["outlook-agent_outlook_action_dry_run"] == "allow"
' "${OPENCODE_CONFIG_DIR}/opencode.json" >/dev/null
cat <<'JSONL'
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_auth_check","state":{"status":"completed","input":{}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_capabilities","state":{"status":"completed","input":{}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed","input":{"action":"DeleteItem","payload":{"Body":{"ItemIds":[{"Id":"dry-run-item"}],"DeleteType":"HardDelete"}},"unsafe_mode":false},"output":{"action":"DeleteItem","ok":false,"count":1,"reversible":false,"requires_confirmation":true,"requires_unsafe":true,"error":"unsafe mode required"}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed","input":{"action":"DeleteItem","payload":{"Body":{"ItemIds":[{"Id":"dry-run-item"}],"DeleteType":"HardDelete"}},"unsafe_mode":true},"output":{"action":"DeleteItem","ok":true,"count":1,"reversible":false,"requires_confirmation":true,"confirmation_token":"unsafe-token"}}}}
JSONL
`
	if err := os.WriteFile(fakeOpencodePath, []byte(fakeOpencode), 0o755); err != nil {
		t.Fatalf("write fake opencode: %v", err)
	}

	command := exec.Command("bash", filepath.Join("scripts", "action-coverage-smoke.sh"))
	command.Dir = repoRoot
	command.Env = append(os.Environ(),
		"OUTLOOK_AGENT_BIN="+fakeAgentPath,
		"OUTLOOK_AGENT_OPENCODE_LIVE_DIR="+liveDir,
		"OUTLOOK_AGENT_OPENCODE_MODEL=example/test-model",
		"OPENCODE_CONFIG="+filepath.Join(tempDir, "unsafe-opencode.json"),
		"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("expected action coverage smoke to apply opencode permission overlay, err=%v output=%s", err, string(output))
	}
	if !strings.Contains(string(output), `"opencode_mcp_smoke": "true"`) {
		t.Fatalf("expected opencode smoke success in output, got %s", string(output))
	}
}

func TestActionCoverageSmokeAcceptsStructuredMCPDryRunOutputs(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for action coverage smoke")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is required for action coverage smoke")
	}

	repoRoot := filepath.Join("..", "..")
	tempDir := t.TempDir()
	coveragePath := filepath.Join(tempDir, "coverage.json")
	fakeAgentPath := filepath.Join(tempDir, "outlook-agent")
	fakeOpencodePath := filepath.Join(tempDir, "opencode")
	liveDir := filepath.Join(tempDir, "opencode-live")
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("create fake opencode live dir: %v", err)
	}

	writeCoverageFixture(t, coveragePath)
	fakeAgent := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$*\" == \"policy coverage\" ]]; then\n  cat " + shellQuote(coveragePath) + "\nelse\n  echo \"unexpected fake outlook-agent args: $*\" >&2\n  exit 2\nfi\n"
	if err := os.WriteFile(fakeAgentPath, []byte(fakeAgent), 0o755); err != nil {
		t.Fatalf("write fake outlook-agent: %v", err)
	}
	fakeOpencode := `#!/usr/bin/env bash
set -euo pipefail
cat <<'JSONL'
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_auth_check","state":{"status":"completed","input":{}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_capabilities","state":{"status":"completed","input":{}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed","input":{"action":"DeleteItem","payload":{"Body":{"ItemIds":[{"Id":"dry-run-item"}],"DeleteType":"HardDelete"}},"unsafe_mode":false},"output":{"structuredContent":{"action":"DeleteItem","ok":false,"count":1,"reversible":false,"requires_confirmation":true,"requires_unsafe":true,"error":"unsafe mode required"}}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed","input":{"action":"DeleteItem","payload":{"Body":{"ItemIds":[{"Id":"dry-run-item"}],"DeleteType":"HardDelete"}},"unsafe_mode":true},"output":{"structuredContent":{"action":"DeleteItem","ok":true,"count":1,"reversible":false,"requires_confirmation":true,"confirmation_token":"unsafe-token"}}}}}
JSONL
`
	if err := os.WriteFile(fakeOpencodePath, []byte(fakeOpencode), 0o755); err != nil {
		t.Fatalf("write fake opencode: %v", err)
	}

	command := exec.Command("bash", filepath.Join("scripts", "action-coverage-smoke.sh"))
	command.Dir = repoRoot
	command.Env = append(os.Environ(),
		"OUTLOOK_AGENT_BIN="+fakeAgentPath,
		"OUTLOOK_AGENT_OPENCODE_LIVE_DIR="+liveDir,
		"OUTLOOK_AGENT_OPENCODE_MODEL=example/test-model",
		"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("expected action coverage smoke to accept structured MCP dry-run outputs, err=%v output=%s", err, string(output))
	}
	if !strings.Contains(string(output), `"opencode_mcp_smoke": "true"`) {
		t.Fatalf("expected opencode smoke success in output, got %s", string(output))
	}
}

func TestActionCoverageSmokeAcceptsRegisteredDeleteItemDryRunInputs(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for action coverage smoke")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is required for action coverage smoke")
	}

	repoRoot := filepath.Join("..", "..")
	tempDir := t.TempDir()
	coveragePath := filepath.Join(tempDir, "coverage.json")
	fakeAgentPath := filepath.Join(tempDir, "outlook-agent")
	fakeOpencodePath := filepath.Join(tempDir, "opencode")
	liveDir := filepath.Join(tempDir, "opencode-live")
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("create fake opencode live dir: %v", err)
	}

	writeCoverageFixture(t, coveragePath)
	fakeAgent := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$*\" == \"policy coverage\" ]]; then\n  cat " + shellQuote(coveragePath) + "\nelse\n  echo \"unexpected fake outlook-agent args: $*\" >&2\n  exit 2\nfi\n"
	if err := os.WriteFile(fakeAgentPath, []byte(fakeAgent), 0o755); err != nil {
		t.Fatalf("write fake outlook-agent: %v", err)
	}
	fakeOpencode := `#!/usr/bin/env bash
set -euo pipefail
cat <<'JSONL'
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_auth_check","state":{"status":"completed","input":{}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_capabilities","state":{"status":"completed","input":{}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed","input":{"action":"DeleteItem","payload":{"Body":{"ItemIds":[{"Id":"dry-run-item"}],"DeleteType":"HardDelete"}},"unsafe_mode":false},"output":{"action":"DeleteItem","ok":false,"count":1,"reversible":false,"requires_confirmation":true,"requires_unsafe":true,"error":"unsafe mode required"}}}}
{"type":"tool_use","part":{"tool":"outlook-agent_outlook_action_dry_run","state":{"status":"completed","input":{"action":"DeleteItem","payload":{"Body":{"ItemIds":[{"Id":"dry-run-item"}],"DeleteType":"HardDelete"}},"unsafe_mode":true},"output":{"action":"DeleteItem","ok":true,"count":1,"reversible":false,"requires_confirmation":true,"confirmation_token":"unsafe-token"}}}}
JSONL
`
	if err := os.WriteFile(fakeOpencodePath, []byte(fakeOpencode), 0o755); err != nil {
		t.Fatalf("write fake opencode: %v", err)
	}

	command := exec.Command("bash", filepath.Join("scripts", "action-coverage-smoke.sh"))
	command.Dir = repoRoot
	command.Env = append(os.Environ(),
		"OUTLOOK_AGENT_BIN="+fakeAgentPath,
		"OUTLOOK_AGENT_OPENCODE_LIVE_DIR="+liveDir,
		"OUTLOOK_AGENT_OPENCODE_MODEL=example/test-model",
		"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("expected action coverage smoke to accept registered DeleteItem dry-run inputs, err=%v output=%s", err, string(output))
	}
	if !strings.Contains(string(output), `"opencode_mcp_smoke": "true"`) {
		t.Fatalf("expected opencode smoke success in output, got %s", string(output))
	}
}

func writeCoverageFixture(t *testing.T, path string) {
	t.Helper()
	type coverageAction struct {
		Action          string `json:"action"`
		Transport       string `json:"transport"`
		SafetyClass     string `json:"safety_class"`
		ExecutionRoute  string `json:"execution_route"`
		LiveCheckLevel  string `json:"live_check_level"`
		RequiresUnsafe  bool   `json:"requires_unsafe,omitempty"`
		AllowedDirect   bool   `json:"allowed_direct"`
		RequiresDryRun  bool   `json:"requires_dry_run"`
		RequiresConfirm bool   `json:"requires_confirmation"`
	}
	actions := []coverageAction{
		{
			Action:         "DeleteItem",
			Transport:      "owa",
			SafetyClass:    "destructive",
			ExecutionRoute: "unsafe_dry_run_confirm",
			LiveCheckLevel: "live_guard_only",
			RequiresUnsafe: true,
			RequiresDryRun: true,
		},
		{
			Action:         "mail.search",
			Transport:      "owa",
			SafetyClass:    "read_metadata",
			ExecutionRoute: "direct",
			LiveCheckLevel: "live_readonly",
			AllowedDirect:  true,
		},
	}
	for len(actions) < 64 {
		actions = append(actions, coverageAction{
			Action:         "fixture.read." + string(rune('a'+len(actions)%26)),
			Transport:      "owa",
			SafetyClass:    "read_metadata",
			ExecutionRoute: "direct",
			LiveCheckLevel: "live_readonly",
			AllowedDirect:  true,
		})
	}
	payload := map[string]any{
		"ok":      true,
		"command": "policy coverage",
		"actions": actions,
		"summary": map[string]any{
			"total":        len(actions),
			"by_transport": map[string]int{"owa": 64},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal coverage fixture: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write coverage fixture: %v", err)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func TestGitHubTemplatesGuideProductionWorkflow(t *testing.T) {
	requiredFiles := map[string][]string{
		filepath.Join("..", "..", ".github", "PULL_REQUEST_TEMPLATE.md"): {
			"## Verification",
			"scripts/ci-local.sh",
			"scripts/release-smoke.sh",
			"Hosted CI",
			"docs/PRODUCTION_BACKLOG.md",
			"public/private boundary",
		},
		filepath.Join("..", "..", ".github", "ISSUE_TEMPLATE", "production-gate.md"): {
			"Production gate",
			"Required evidence",
			"Acceptance criteria",
			"Do not include",
			"tenant endpoints",
			"secrets",
		},
	}

	for path, markers := range requiredFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read GitHub template %s: %v", path, err)
		}
		text := string(data)
		for _, marker := range markers {
			if !strings.Contains(text, marker) {
				t.Fatalf("expected %s to contain %q", path, marker)
			}
		}
	}
}

func TestAgentUXDocumentationNamesHappyPath(t *testing.T) {
	requiredFiles := map[string][]string{
		filepath.Join("..", "..", "README.md"): {
			"outlook-agent help",
			"outlook-agent setup opencode --print",
			".opencode/skills",
			"metadata-first",
		},
		filepath.Join("..", "..", "docs", "OPENCODE.md"): {
			"outlook-agent setup opencode --print",
			".opencode/skills/outlook-mail",
			".opencode/skills/outlook-calendar",
			"capabilities",
			"dry-run",
			"exact confirmation",
		},
		filepath.Join("..", "..", "docs", "SPEC.md"): {
			"outlook-agent help",
			"setup opencode --print",
			"next_steps",
			"metadata-first",
		},
	}

	for path, markers := range requiredFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read UX doc %s: %v", path, err)
		}
		text := string(data)
		for _, marker := range markers {
			if !strings.Contains(text, marker) {
				t.Fatalf("expected %s to contain %q", path, marker)
			}
		}
	}
}

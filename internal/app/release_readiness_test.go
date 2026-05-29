package app_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
			"GOHOSTOS",
			"GOHOSTARCH",
			"\"version\": \"smoke\"",
			"\"built_by\": \"release-build\"",
		},
		filepath.Join("..", "..", "scripts", "release-build.sh"): {
			"GOOS",
			"GOARCH",
			"invalid release version",
			"buildinfo_pkg",
			".Version=",
			".Commit=",
			".Date=",
			".Dirty=",
			".BuiltBy=",
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

func TestDependabotReadiness(t *testing.T) {
	path := filepath.Join("..", "..", ".github", "dependabot.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Dependabot config %s: %v", path, err)
	}
	if err := validateDependabotConfig(data); err != nil {
		t.Fatalf("invalid Dependabot config %s: %v", path, err)
	}
}

func TestDependabotReadinessRejectsBrokenFixtures(t *testing.T) {
	fixtures := map[string]string{
		"commented out config": `
# version: 2
# updates:
#   - package-ecosystem: gomod
#     directory: "/"
#     schedule:
#       interval: weekly
#     open-pull-requests-limit: 5
#     labels:
#       - dependencies
#   - package-ecosystem: github-actions
#     directory: "/"
#     schedule:
#       interval: weekly
#     open-pull-requests-limit: 5
#     labels:
#       - dependencies
`,
		"missing github-actions entry with duplicate gomod markers": `
version: 2
updates:
  - package-ecosystem: gomod
    directory: "/"
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
    labels:
      - dependencies
      - go
    note: package-ecosystem: github-actions
  - package-ecosystem: gomod
    directory: "/"
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
    labels:
      - dependencies
      - github-actions
`,
	}

	for name, fixture := range fixtures {
		t.Run(name, func(t *testing.T) {
			if err := validateDependabotConfig([]byte(fixture)); err == nil {
				t.Fatal("expected broken Dependabot fixture to be rejected")
			}
		})
	}
}

type dependabotUpdate struct {
	ecosystem string
	directory string
	interval  string
	limit     int
	labels    []string
}

type dependabotConfig struct {
	version    int
	hasVersion bool
	hasUpdates bool
	updates    []dependabotUpdate
}

func validateDependabotConfig(data []byte) error {
	config, err := parseDependabotConfig(data)
	if err != nil {
		return err
	}
	if !config.hasVersion || config.version != 2 {
		return fmt.Errorf("expected top-level version: 2")
	}
	if !config.hasUpdates {
		return fmt.Errorf("expected top-level updates")
	}
	if len(config.updates) != 2 {
		return fmt.Errorf("expected exactly two updates, got %d", len(config.updates))
	}

	requiredEcosystems := map[string]bool{
		"gomod":          false,
		"github-actions": false,
	}
	for index, update := range config.updates {
		if _, ok := requiredEcosystems[update.ecosystem]; !ok {
			return fmt.Errorf("update %d has unexpected package-ecosystem %q", index+1, update.ecosystem)
		}
		if requiredEcosystems[update.ecosystem] {
			return fmt.Errorf("duplicate package-ecosystem %q", update.ecosystem)
		}
		requiredEcosystems[update.ecosystem] = true
		if update.directory != "/" {
			return fmt.Errorf("update %s must use directory /", update.ecosystem)
		}
		if update.interval != "weekly" {
			return fmt.Errorf("update %s must use weekly interval", update.ecosystem)
		}
		if update.limit <= 0 {
			return fmt.Errorf("update %s must have positive open-pull-requests-limit", update.ecosystem)
		}
		if len(update.labels) == 0 {
			return fmt.Errorf("update %s must have non-empty labels", update.ecosystem)
		}
	}
	for ecosystem, found := range requiredEcosystems {
		if !found {
			return fmt.Errorf("missing package-ecosystem %q", ecosystem)
		}
	}
	return nil
}

func parseDependabotConfig(data []byte) (dependabotConfig, error) {
	config := dependabotConfig{}
	var current *dependabotUpdate
	inSchedule := false
	inLabels := false

	for lineNumber, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "registries:") {
			return dependabotConfig{}, fmt.Errorf("line %d: registries are not allowed", lineNumber+1)
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		switch {
		case indent == 0:
			current = nil
			inSchedule = false
			inLabels = false
			key, value, ok := yamlKeyValue(trimmed)
			if !ok {
				return dependabotConfig{}, fmt.Errorf("line %d: expected top-level key", lineNumber+1)
			}
			switch key {
			case "version":
				version, err := strconv.Atoi(value)
				if err != nil {
					return dependabotConfig{}, fmt.Errorf("line %d: invalid version %q", lineNumber+1, value)
				}
				config.version = version
				config.hasVersion = true
			case "updates":
				if value != "" {
					return dependabotConfig{}, fmt.Errorf("line %d: updates must be a list", lineNumber+1)
				}
				config.hasUpdates = true
			default:
				return dependabotConfig{}, fmt.Errorf("line %d: unexpected top-level key %q", lineNumber+1, key)
			}
		case indent == 2 && strings.HasPrefix(trimmed, "- "):
			if !config.hasUpdates {
				return dependabotConfig{}, fmt.Errorf("line %d: update entry before updates", lineNumber+1)
			}
			config.updates = append(config.updates, dependabotUpdate{limit: -1})
			current = &config.updates[len(config.updates)-1]
			inSchedule = false
			inLabels = false
			field := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if field != "" {
				if err := applyDependabotUpdateField(current, field, false, false, lineNumber+1); err != nil {
					return dependabotConfig{}, err
				}
			}
		case current == nil:
			return dependabotConfig{}, fmt.Errorf("line %d: nested key outside update entry", lineNumber+1)
		case indent == 4:
			inSchedule = false
			inLabels = false
			key, value, ok := yamlKeyValue(trimmed)
			if !ok {
				return dependabotConfig{}, fmt.Errorf("line %d: expected update key", lineNumber+1)
			}
			if key == "schedule" {
				if value != "" {
					return dependabotConfig{}, fmt.Errorf("line %d: schedule must be a mapping", lineNumber+1)
				}
				inSchedule = true
				continue
			}
			if key == "labels" {
				if value != "" {
					return dependabotConfig{}, fmt.Errorf("line %d: labels must be a list", lineNumber+1)
				}
				inLabels = true
				continue
			}
			if err := applyDependabotUpdateField(current, trimmed, false, false, lineNumber+1); err != nil {
				return dependabotConfig{}, err
			}
		case indent == 6 && inSchedule:
			if err := applyDependabotUpdateField(current, trimmed, true, false, lineNumber+1); err != nil {
				return dependabotConfig{}, err
			}
		case indent == 6 && inLabels && strings.HasPrefix(trimmed, "- "):
			label := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if label == "" {
				return dependabotConfig{}, fmt.Errorf("line %d: empty label", lineNumber+1)
			}
			current.labels = append(current.labels, unquoteYAMLScalar(label))
		default:
			return dependabotConfig{}, fmt.Errorf("line %d: unexpected Dependabot YAML shape", lineNumber+1)
		}
	}

	return config, nil
}

func applyDependabotUpdateField(update *dependabotUpdate, field string, inSchedule bool, inLabels bool, lineNumber int) error {
	key, value, ok := yamlKeyValue(field)
	if !ok {
		return fmt.Errorf("line %d: expected key/value", lineNumber)
	}
	if inLabels {
		return fmt.Errorf("line %d: labels must be list items", lineNumber)
	}
	value = unquoteYAMLScalar(value)
	if inSchedule {
		if key != "interval" {
			return fmt.Errorf("line %d: unexpected schedule key %q", lineNumber, key)
		}
		update.interval = value
		return nil
	}

	switch key {
	case "package-ecosystem":
		update.ecosystem = value
	case "directory":
		update.directory = value
	case "open-pull-requests-limit":
		limit, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("line %d: invalid open-pull-requests-limit %q", lineNumber, value)
		}
		update.limit = limit
	default:
		return fmt.Errorf("line %d: unexpected update key %q", lineNumber, key)
	}
	return nil
}

func yamlKeyValue(line string) (string, string, bool) {
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(key), strings.TrimSpace(value), true
}

func unquoteYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
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
		`"$expected_package_dir/RELEASE.md"`,
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

func TestInstallScriptFallbackDirExplainsPathWhenOutsidePATH(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("install.sh archives are only supported on darwin/linux")
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("install.sh archives are only supported on amd64/arm64")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can still write to chmod 0555 directories; fallback simulation requires a non-root user")
	}

	root := filepath.Join("..", "..")
	home := t.TempDir()
	fakeBin := filepath.Join(t.TempDir(), "bin")
	releaseDir := t.TempDir()
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin: %v", err)
	}
	for name, path := range map[string]string{
		"tar":    "/usr/bin/tar",
		"grep":   "/usr/bin/grep",
		"mktemp": "/usr/bin/mktemp",
		"shasum": "/usr/bin/shasum",
		"uname":  "/usr/bin/uname",
		"gzip":   "/usr/bin/gzip",
		"cut":    "/usr/bin/cut",
		"rm":     "/bin/rm",
		"mkdir":  "/bin/mkdir",
		"cp":     "/bin/cp",
		"chmod":  "/bin/chmod",
		"mv":     "/bin/mv",
		"cat":    "/bin/cat",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("%s is required for installer integration test: %v", path, err)
		}
		if err := os.Symlink(path, filepath.Join(fakeBin, name)); err != nil {
			t.Fatalf("link fake command %s: %v", name, err)
		}
	}

	version := "vfallbacktest"
	archiveName := fmt.Sprintf("outlook-agent_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	packageName := strings.TrimSuffix(archiveName, ".tar.gz")
	packageDir := filepath.Join(releaseDir, packageName)
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatalf("create package dir: %v", err)
	}
	for name, content := range map[string]string{
		"outlook-agent": "#!/bin/sh\nexit 0\n",
		"README.md":     "# README\n",
		"RELEASE.md":    "# Release\n",
	} {
		if err := os.WriteFile(filepath.Join(packageDir, name), []byte(content), 0o755); err != nil {
			t.Fatalf("write package file %s: %v", name, err)
		}
	}
	archivePath := filepath.Join(releaseDir, archiveName)
	tarCmd := exec.Command("/usr/bin/tar", "-czf", archivePath, "-C", releaseDir, packageName)
	if output, err := tarCmd.CombinedOutput(); err != nil {
		t.Fatalf("create archive: %v\n%s", err, string(output))
	}
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	sum := sha256.Sum256(archiveData)
	checksums := fmt.Sprintf("%x  %s\n", sum, archiveName)
	if err := os.WriteFile(filepath.Join(releaseDir, "SHA256SUMS.txt"), []byte(checksums), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}

	fakeCurl := fmt.Sprintf(`#!/bin/sh
set -eu
out=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      shift
      out="$1"
      ;;
    -*)
      ;;
    *)
      url="$1"
      ;;
  esac
  shift
done
case "$url" in
  */SHA256SUMS.txt)
    /bin/cp "$FAKE_RELEASE_DIR/SHA256SUMS.txt" "$out"
    ;;
  */%s)
    /bin/cp "$FAKE_RELEASE_DIR/%s" "$out"
    ;;
  *)
    echo "unexpected url: $url" >&2
    exit 22
    ;;
esac
`, archiveName, archiveName)
	if err := os.WriteFile(filepath.Join(fakeBin, "curl"), []byte(fakeCurl), 0o755); err != nil {
		t.Fatalf("write fake curl: %v", err)
	}
	if err := os.Chmod(fakeBin, 0o555); err != nil {
		t.Fatalf("make fake bin non-writable: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(fakeBin, 0o755)
	})

	cmd := exec.Command("/bin/sh", filepath.Join(root, "install.sh"), "--version", version)
	cmd.Env = []string{
		"HOME=" + home,
		"PATH=" + fakeBin,
		"FAKE_RELEASE_DIR=" + releaseDir,
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install script failed: %v\n%s", err, string(output))
	}
	targetPath := filepath.Join(home, ".local", "bin", "outlook-agent")
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("expected fallback install target: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "not on PATH") {
		t.Fatalf("expected fallback install output to explain PATH issue, got:\n%s", text)
	}
	if !strings.Contains(text, targetPath+" help") {
		t.Fatalf("expected fallback install output to show full binary path, got:\n%s", text)
	}
}

func TestInstallScriptResolvesLatestWithWgetOnly(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("install.sh archives are only supported on darwin/linux")
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("install.sh archives are only supported on amd64/arm64")
	}

	root := filepath.Join("..", "..")
	home := t.TempDir()
	fakeBin := filepath.Join(t.TempDir(), "bin")
	installDir := filepath.Join(t.TempDir(), "install")
	releaseDir := t.TempDir()
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin: %v", err)
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("create install dir: %v", err)
	}
	for name, path := range map[string]string{
		"tar":    "/usr/bin/tar",
		"grep":   "/usr/bin/grep",
		"mktemp": "/usr/bin/mktemp",
		"shasum": "/usr/bin/shasum",
		"uname":  "/usr/bin/uname",
		"gzip":   "/usr/bin/gzip",
		"cut":    "/usr/bin/cut",
		"sed":    "/usr/bin/sed",
		"tail":   "/usr/bin/tail",
		"tr":     "/usr/bin/tr",
		"rm":     "/bin/rm",
		"mkdir":  "/bin/mkdir",
		"cp":     "/bin/cp",
		"chmod":  "/bin/chmod",
		"mv":     "/bin/mv",
		"cat":    "/bin/cat",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("%s is required for installer integration test: %v", path, err)
		}
		if err := os.Symlink(path, filepath.Join(fakeBin, name)); err != nil {
			t.Fatalf("link fake command %s: %v", name, err)
		}
	}

	version := "vwgettest"
	archiveName := fmt.Sprintf("outlook-agent_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	packageName := strings.TrimSuffix(archiveName, ".tar.gz")
	packageDir := filepath.Join(releaseDir, packageName)
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatalf("create package dir: %v", err)
	}
	for name, content := range map[string]string{
		"outlook-agent": "#!/bin/sh\nexit 0\n",
		"README.md":     "# README\n",
		"RELEASE.md":    "# Release\n",
	} {
		if err := os.WriteFile(filepath.Join(packageDir, name), []byte(content), 0o755); err != nil {
			t.Fatalf("write package file %s: %v", name, err)
		}
	}
	archivePath := filepath.Join(releaseDir, archiveName)
	tarCmd := exec.Command("/usr/bin/tar", "-czf", archivePath, "-C", releaseDir, packageName)
	if output, err := tarCmd.CombinedOutput(); err != nil {
		t.Fatalf("create archive: %v\n%s", err, string(output))
	}
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	sum := sha256.Sum256(archiveData)
	checksums := fmt.Sprintf("%x  %s\n", sum, archiveName)
	if err := os.WriteFile(filepath.Join(releaseDir, "SHA256SUMS.txt"), []byte(checksums), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}

	fakeWget := fmt.Sprintf(`#!/bin/sh
set -eu
out=""
url=""
spider=0
quiet=0
server_response=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    -O)
      shift
      out="$1"
      ;;
    --spider)
      spider=1
      ;;
    --server-response)
      server_response=1
      ;;
    -*)
      case "$1" in
        *q*) quiet=1 ;;
      esac
      case "$1" in
        *S*) server_response=1 ;;
      esac
      ;;
    *)
      url="$1"
      ;;
  esac
  shift
done
if [ "$spider" = "1" ]; then
  if [ "$quiet" = "0" ] && [ "$server_response" = "1" ]; then
    echo "  Location: https://github.com/johnkil/outlook-agent/releases/tag/%s" >&2
  fi
  exit 0
fi
case "$url" in
  */SHA256SUMS.txt)
    /bin/cp "$FAKE_RELEASE_DIR/SHA256SUMS.txt" "$out"
    ;;
  */%s)
    /bin/cp "$FAKE_RELEASE_DIR/%s" "$out"
    ;;
  *)
    echo "unexpected url: $url" >&2
    exit 22
    ;;
esac
`, version, archiveName, archiveName)
	if err := os.WriteFile(filepath.Join(fakeBin, "wget"), []byte(fakeWget), 0o755); err != nil {
		t.Fatalf("write fake wget: %v", err)
	}

	cmd := exec.Command("/bin/sh", filepath.Join(root, "install.sh"), "--dir", installDir)
	cmd.Env = []string{
		"HOME=" + home,
		"PATH=" + fakeBin,
		"FAKE_RELEASE_DIR=" + releaseDir,
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install script failed: %v\n%s", err, string(output))
	}
	targetPath := filepath.Join(installDir, "outlook-agent")
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("expected install target: %v", err)
	}
	if !strings.Contains(string(output), "Installed outlook-agent "+version) {
		t.Fatalf("expected install output to include resolved version %s, got:\n%s", version, string(output))
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

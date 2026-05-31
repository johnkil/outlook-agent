# GoReleaser And Update UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a release/update path where GoReleaser owns portable artifact publishing, while Outlook Agent keeps explicit, user-controlled update commands for binary, MCP config, and Codex skills/plugin integration.

**Architecture:** Split the work into three PRs. PR 1 adds GoReleaser in parity mode without removing the existing release scripts or changing user install behavior. PR 2 adds an explicit updater UX that downloads verified GitHub Release artifacts and can re-apply Codex/OpenCode/Claude setup. PR 3 adds a Codex marketplace/package layout so Codex plugin metadata and skills can be upgraded through Codex marketplace commands, while the Go binary remains updated through the installer/updater.

**Tech Stack:** Go, Bash, GitHub Actions, GoReleaser v2, GitHub Releases API, existing `internal/cli`, existing `internal/setup`, existing release scripts.

---

## Scope Decision

This is intentionally **not one PR**. Release publishing, binary self-update, and Codex marketplace packaging are separate subsystems with different failure modes.

Recommended order:

1. **PR 1: GoReleaser parity** - safest, no UX change.
2. **PR 2: Explicit update commands** - user-visible update UX.
3. **PR 3: Codex marketplace package** - plugin distribution/update convenience.

Do not implement silent auto-update. Updates must be explicit because this tool has access to mail and calendar data.

---

## File Structure

### PR 1: GoReleaser parity

- Create: `.goreleaser.yaml`
  - Owns release build matrix, archives, checksums, GitHub Release publishing, and generated dependency manifest artifact.
- Modify: `.github/workflows/release.yml`
  - Uses GoReleaser action for tagged releases, then runs the existing release verification gate.
- Modify: `scripts/release-smoke.sh`
  - Adds a GoReleaser snapshot path or keeps the existing script as the parity oracle.
- Modify: `docs/RELEASE.md`
  - Documents GoReleaser as the release publisher and keeps local CI/release-smoke as required pre-tag gates.
- Modify: `docs/RELEASE_EVIDENCE.md`
  - Adds GoReleaser workflow evidence fields.
- Modify: `internal/app/release_readiness_test.go`
  - Adds tests that enforce `.goreleaser.yaml`, pinned release action, archive naming, checksum coverage, and preservation of existing safety gates.

### PR 2: Explicit update commands

- Create: `internal/update/update.go`
  - Resolves target version, fetches release metadata, downloads archives/checksums, verifies SHA256, unpacks host binary, stages replacement.
- Create: `internal/update/update_test.go`
  - Unit tests for version resolution, checksum matching, host archive selection, refusal cases, and no-secret logging.
- Modify: `internal/cli/cli.go`
  - Adds `outlook-agent update` and `outlook-agent setup upgrade`.
- Modify: `internal/cli/cli_test.go`
  - CLI contract tests for dry-run, explicit version, latest version, and setup composition.
- Modify: `docs/RELEASE.md`
  - Documents update flow.
- Modify: `README.md`
  - Adds short update section.

### PR 3: Codex marketplace package

- Create: `.agents/plugins/marketplace.json`
  - Codex marketplace index pointing to the Outlook Agent plugin package.
- Create: `plugins/outlook-agent/.codex-plugin/plugin.json`
  - Codex plugin manifest generated from the existing export contract.
- Create: `plugins/outlook-agent/.mcp.json`
  - MCP server config pointing to `outlook-agent` on PATH.
- Create: `plugins/outlook-agent/skills/*/SKILL.md`
  - Generated copies from canonical `skills/`.
- Modify: `internal/setup/plugin.go`
  - No change expected in the first marketplace PR; modify only if the existing exporter cannot write the marketplace package shape that tests require.
- Modify: `internal/setup/plugin_test.go`
  - Verifies marketplace package files match generated plugin export and contain no local/private config.
- Modify: `docs/PLUGIN_PACKAGING.md`
  - Documents `codex plugin marketplace add`, `codex plugin marketplace upgrade`, and restart/reload expectations.

---

## PR 1: GoReleaser Parity

### Task 1: Add release-readiness tests for GoReleaser config

**Files:**
- Modify: `internal/app/release_readiness_test.go`
- Create: `.goreleaser.yaml`

- [ ] **Step 1: Write the failing test**

Add a test near the existing release readiness tests:

```go
func TestGoReleaserConfigIsPresentAndPublicSafe(t *testing.T) {
	path := filepath.Join("..", "..", ".goreleaser.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(data)
	for _, required := range []string{
		"project_name: outlook-agent",
		"CGO_ENABLED=0",
		"github.com/johnkil/outlook-agent/internal/buildinfo.Version={{ .Version }}",
		"github.com/johnkil/outlook-agent/internal/buildinfo.Commit={{ .Commit }}",
		"outlook-agent_{{ .Version }}_{{ .Os }}_{{ .Arch }}",
		"SHA256SUMS.txt",
		"dist/",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected .goreleaser.yaml to contain %q", required)
		}
	}
	for _, forbidden := range []string{
		"owa.alfabank.ru",
		"MOSCOW\\",
		"X-OWA-CANARY",
		"cookie",
		"password",
	} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Fatalf(".goreleaser.yaml contains private marker %q", forbidden)
		}
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```bash
go test ./internal/app -run TestGoReleaserConfigIsPresentAndPublicSafe -count=1
```

Expected: FAIL because `.goreleaser.yaml` does not exist.

- [ ] **Step 3: Add minimal `.goreleaser.yaml`**

Create `.goreleaser.yaml` with a parity configuration:

```yaml
version: 2
project_name: outlook-agent

before:
  hooks:
    - go mod tidy

builds:
  - id: outlook-agent
    main: ./cmd/outlook-agent
    binary: outlook-agent
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X github.com/johnkil/outlook-agent/internal/buildinfo.Version={{ .Version }}
      - -X github.com/johnkil/outlook-agent/internal/buildinfo.Commit={{ .Commit }}
      - -X github.com/johnkil/outlook-agent/internal/buildinfo.Date={{ .Date }}
      - -X github.com/johnkil/outlook-agent/internal/buildinfo.Dirty=false
      - -X github.com/johnkil/outlook-agent/internal/buildinfo.BuiltBy=goreleaser

archives:
  - id: outlook-agent
    ids:
      - outlook-agent
    name_template: "outlook-agent_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - README.md
      - docs/RELEASE.md

checksum:
  name_template: SHA256SUMS.txt

changelog:
  use: github

release:
  github:
    owner: johnkil
    name: outlook-agent
```

- [ ] **Step 4: Run the focused test and adjust exact strings**

Run:

```bash
go test ./internal/app -run TestGoReleaserConfigIsPresentAndPublicSafe -count=1
```

Expected: PASS after matching the final GoReleaser syntax.

- [ ] **Step 5: Commit**

```bash
git add .goreleaser.yaml internal/app/release_readiness_test.go
git commit -m "Add GoReleaser release config"
```

### Task 2: Prove GoReleaser snapshot artifacts match release expectations

**Files:**
- Modify: `scripts/release-smoke.sh`
- Modify: `internal/app/release_readiness_test.go`

- [ ] **Step 1: Write the failing test for release smoke coverage**

Extend `TestReleaseReadinessArtifactsExist` so `scripts/release-smoke.sh` must mention GoReleaser snapshot validation:

```go
filepath.Join("..", "..", "scripts", "release-smoke.sh"): {
	"scripts/release-build.sh",
	"scripts/release-verify.sh",
	"goreleaser",
	"release smoke passed",
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```bash
go test ./internal/app -run TestReleaseReadinessArtifactsExist -count=1
```

Expected: FAIL because release smoke does not yet invoke GoReleaser.

- [ ] **Step 3: Add an opt-in GoReleaser smoke path**

Modify `scripts/release-smoke.sh` so the existing smoke remains default, and `OUTLOOK_AGENT_GORELEASER_SMOKE=1` runs:

```bash
if [[ -n "${OUTLOOK_AGENT_GORELEASER_SMOKE:-}" ]]; then
  if ! command -v goreleaser >/dev/null 2>&1; then
    echo "goreleaser is required when OUTLOOK_AGENT_GORELEASER_SMOKE=1" >&2
    exit 1
  fi
  GORELEASER_CURRENT_TAG="${smoke_version}" goreleaser release --snapshot --clean --skip=publish
  scripts/release-verify.sh dist
  echo "goreleaser release smoke passed"
  exit 0
fi
```

Keep the existing script path unchanged for the normal CI/local release smoke.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/app -run 'TestReleaseReadinessArtifactsExist|TestGoReleaserConfigIsPresentAndPublicSafe' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run GoReleaser check**

Run:

```bash
go run github.com/goreleaser/goreleaser/v2@latest check
```

Expected: exit 0.

- [ ] **Step 6: Commit**

```bash
git add scripts/release-smoke.sh internal/app/release_readiness_test.go
git commit -m "Add GoReleaser snapshot smoke gate"
```

### Task 3: Wire release workflow to GoReleaser without deleting fallback scripts

**Files:**
- Modify: `.github/workflows/release.yml`
- Modify: `internal/app/release_readiness_test.go`
- Modify: `docs/RELEASE.md`

- [ ] **Step 1: Write failing test for workflow contract**

Change the release workflow assertions in `TestReleaseReadinessArtifactsExist`:

```go
filepath.Join("..", "..", ".github", "workflows", "release.yml"): {
	"goreleaser/goreleaser-action@",
	"version: '~> v2'",
	"args: release --clean",
	"GITHUB_TOKEN",
	"scripts/release-verify.sh dist",
}
```

- [ ] **Step 2: Run and verify failure**

Run:

```bash
go test ./internal/app -run TestReleaseReadinessArtifactsExist -count=1
```

Expected: FAIL because workflow still calls `scripts/release-build.sh`.

- [ ] **Step 3: Update release workflow**

Resolve the GoReleaser action tag and SHA before editing:

```bash
tag="$(gh release view --repo goreleaser/goreleaser-action --json tagName --jq .tagName)"
git ls-remote https://github.com/goreleaser/goreleaser-action.git "refs/tags/${tag}^{ }"
```

Use the printed SHA in the workflow and keep release verification. The final `uses:` line must contain the resolved commit SHA and the version comment from the commands above, just like the existing pinned workflow actions. The workflow step must set `version: '~> v2'`, `args: release --clean`, and `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`. Keep the next workflow step as `scripts/release-verify.sh dist`.

- [ ] **Step 4: Update docs**

In `docs/RELEASE.md`, replace the tag workflow section with:

```markdown
The tag workflow uses GoReleaser to build and publish release artifacts, then
runs `scripts/release-verify.sh dist` against the generated `dist/` directory.
The local scripts remain the public-safety and fallback release gates.
```

- [ ] **Step 5: Run workflow/tests locally**

Run:

```bash
go test ./internal/app -run 'TestReleaseReadinessArtifactsExist|TestGitHubActionsUsePinnedActions' -count=1
git diff --check
```

Expected: both commands pass.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/release.yml docs/RELEASE.md internal/app/release_readiness_test.go
git commit -m "Use GoReleaser for tag releases"
```

### Task 4: Verify PR 1 end to end

**Files:**
- No new files.

- [ ] **Step 1: Run local gates**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
```

Expected: PASS.

- [ ] **Step 2: Run release smoke paths**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
```

Expected: PASS with `release smoke passed`.

Run:

```bash
OUTLOOK_AGENT_GORELEASER_SMOKE=1 go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean --skip=publish
```

Expected: PASS and a verified `dist/` shape.

- [ ] **Step 3: Open PR**

```bash
git status --short
gh pr create --title "Add GoReleaser release publishing" --body "Summary: add GoReleaser parity config and release workflow wiring; keep existing release scripts as safety/fallback gates. Verification: scripts/ci-local.sh; scripts/release-smoke.sh; GoReleaser snapshot."
```

---

## PR 2: Explicit Update UX

### Task 5: Add update planning API

**Files:**
- Create: `internal/update/update.go`
- Create: `internal/update/update_test.go`

- [ ] **Step 1: Write tests for host archive selection**

Add tests covering:

```go
func TestSelectAssetForHostChoosesDarwinArm64Tarball(t *testing.T)
func TestSelectAssetForHostRejectsUnsupportedOS(t *testing.T)
func TestVerifyChecksumMatchesSHA256SUMS(t *testing.T)
func TestVerifyChecksumRejectsMismatch(t *testing.T)
```

- [ ] **Step 2: Implement minimal update package**

Define:

```go
type Asset struct {
	Name string
	URL  string
}

type Plan struct {
	Version     string
	AssetName   string
	ChecksumURL string
	InstallPath string
}

func SelectHostAsset(version string, goos string, goarch string, assets []Asset) (Asset, error)
func VerifyChecksum(assetName string, data []byte, checksumFile []byte) error
```

- [ ] **Step 3: Run tests**

Run:

```bash
go test ./internal/update -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/update/update.go internal/update/update_test.go
git commit -m "Add release update planning primitives"
```

### Task 6: Add `outlook-agent update --check`

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `README.md`

- [ ] **Step 1: Write CLI tests**

Add tests:

```go
func TestUpdateCheckPrintsMachineReadablePlan(t *testing.T)
func TestUpdateRequiresExplicitApplyForMutation(t *testing.T)
```

Expected JSON shape:

```json
{
  "ok": true,
  "command": "update check",
  "current_version": "v0.3.0",
  "latest_version": "v0.3.1",
  "update_available": true
}
```

- [ ] **Step 2: Add help text**

Add:

```text
outlook-agent update --check [--version v0.3.1]
outlook-agent update apply --version v0.3.1 --yes
```

- [ ] **Step 3: Implement dry-run only**

First implementation must support only `--check`, not binary replacement.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/cli ./internal/update -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go README.md
git commit -m "Add explicit update check command"
```

### Task 7: Add `setup upgrade` composition

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `docs/SETUP_AGENT.md`

- [ ] **Step 1: Write CLI tests**

Add:

```go
func TestSetupUpgradePlansBinaryAndAgentSetup(t *testing.T)
func TestSetupUpgradeApplyRequiresYes(t *testing.T)
```

Expected command:

```bash
outlook-agent setup upgrade --client codex --scope user --config ~/.config/outlook-agent/config.json
```

Expected behavior:

- plan mode prints update status plus setup-agent operations;
- apply mode requires `--yes`;
- apply mode invokes update apply first, then `setup agent apply`.

- [ ] **Step 2: Implement planning path first**

Support:

```text
outlook-agent setup upgrade plan --client codex --scope user --config ~/.config/outlook-agent/config.json
outlook-agent setup upgrade diff --client codex --scope user --config ~/.config/outlook-agent/config.json
```

- [ ] **Step 3: Implement apply path**

Support:

```text
outlook-agent setup upgrade apply --client codex --scope user --config ~/.config/outlook-agent/config.json --yes --backup
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/cli ./internal/setup ./internal/update -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go docs/SETUP_AGENT.md
git commit -m "Add setup upgrade workflow"
```

---

## PR 3: Codex Marketplace Package

### Task 8: Add marketplace package generation tests

**Files:**
- Modify: `internal/setup/plugin_test.go`
- Create: `.agents/plugins/marketplace.json`
- Create: `plugins/outlook-agent/.codex-plugin/plugin.json`
- Create: `plugins/outlook-agent/.mcp.json`

- [ ] **Step 1: Write tests**

Add:

```go
func TestCodexMarketplacePackageMatchesPluginExport(t *testing.T)
func TestCodexMarketplacePackageContainsNoPrivateConfig(t *testing.T)
```

Assertions:

- `plugins/outlook-agent/.codex-plugin/plugin.json` is valid JSON;
- `plugins/outlook-agent/.mcp.json` is valid JSON;
- `plugins/outlook-agent/skills/outlook-mail/SKILL.md` exists;
- marketplace files contain no private domains, usernames, tokens, cookies, canaries, or config contents.

- [ ] **Step 2: Run tests and verify failure**

```bash
go test ./internal/setup -run 'TestCodexMarketplacePackage' -count=1
```

Expected: FAIL because package files do not exist.

- [ ] **Step 3: Generate package from current exporter**

Run:

```bash
outlook-agent setup plugin export --client codex --output plugins/outlook-agent --force
```

Create `.agents/plugins/marketplace.json`:

```json
{
  "name": "outlook-agent",
  "plugins": [
    {
      "name": "outlook-agent",
      "path": "../../plugins/outlook-agent"
    }
  ]
}
```

Run `codex plugin marketplace add "$PWD" --sparse .agents/plugins --sparse plugins` in a disposable checkout during live validation. If Codex rejects the schema, capture the exact error, update this JSON shape, and add the rejected shape to `TestCodexMarketplacePackageMatchesPluginExport` as a regression fixture.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/setup -run 'TestCodexMarketplacePackage' -count=1
scripts/public-safety-check.sh
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add .agents/plugins/marketplace.json plugins/outlook-agent internal/setup/plugin_test.go
git commit -m "Add Codex marketplace package"
```

### Task 9: Document Codex marketplace update flow

**Files:**
- Modify: `docs/PLUGIN_PACKAGING.md`
- Modify: `README.md`

- [ ] **Step 1: Add docs text**

Add:

```markdown
## Codex Marketplace

Install or refresh the marketplace source:

```bash
codex plugin marketplace add johnkil/outlook-agent --sparse .agents/plugins --sparse plugins
codex plugin marketplace upgrade outlook-agent
```

Then open `/plugins` in Codex, enable `outlook-agent`, and restart the Codex
session so MCP and skill metadata are loaded from the refreshed plugin package.

This updates the plugin package. It does not silently update the `outlook-agent`
binary. Update the binary with:

```bash
curl -fsSL https://raw.githubusercontent.com/johnkil/outlook-agent/main/install.sh | sh
```
```

- [ ] **Step 2: Run docs tests**

```bash
go test ./internal/app -run TestSetupDocs -count=1
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add docs/PLUGIN_PACKAGING.md README.md
git commit -m "Document Codex marketplace updates"
```

---

## Full Verification

Run after each PR:

```bash
gofmt -l cmd internal examples skills
git diff --check
go test ./...
```

Run before release-related PR merge:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
```

Run before tagging a release:

```bash
gh run list --branch main --limit 5 --json databaseId,headSha,status,conclusion,workflowName,url
git tag -a v0.3.1 -m "v0.3.1"
git push origin v0.3.1
run_id="$(gh run list --workflow Release --branch v0.3.1 --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch "$run_id" --exit-status
gh release view v0.3.1 --json tagName,isDraft,isPrerelease,url,assets
```

---

## Rollback

- PR 1 rollback: keep existing `scripts/release-build.sh` workflow path, remove GoReleaser workflow step and `.goreleaser.yaml`.
- PR 2 rollback: remove `update` / `setup upgrade` commands. Existing installer remains the supported update path.
- PR 3 rollback: remove marketplace package files. Existing `setup agent` and `setup plugin export` remain supported.

---

## Self-Review Notes

- No silent auto-update is planned.
- Existing release scripts stay as safety/fallback gates until GoReleaser has shipped at least one successful release.
- Codex plugin package update and binary update are deliberately separate.
- Marketplace schema may need one adjustment after live Codex validation; that validation belongs in PR 3, not PR 1.

package setup

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skillassets "github.com/johnkil/outlook-agent/skills"
)

func TestCodexMarketplaceFileUsesCurrentSchema(t *testing.T) {
	marketplace := readJSONMap(t, repoPath("..", "..", ".agents", "plugins", "marketplace.json"))
	if marketplace["name"] != "outlook-agent" {
		t.Fatalf("expected marketplace name outlook-agent, got %#v", marketplace["name"])
	}
	if nestedString(t, marketplace, "interface", "displayName") != "Outlook Agent" {
		t.Fatalf("expected marketplace display name Outlook Agent, got %s", mustJSON(t, marketplace))
	}
	plugins, ok := marketplace["plugins"].([]any)
	if !ok || len(plugins) != 1 {
		t.Fatalf("expected exactly one marketplace plugin, got %s", mustJSON(t, marketplace))
	}
	plugin, ok := plugins[0].(map[string]any)
	if !ok {
		t.Fatalf("expected plugin entry object, got %s", mustJSON(t, marketplace))
	}
	if plugin["name"] != "outlook-agent" {
		t.Fatalf("expected plugin name outlook-agent, got %s", mustJSON(t, plugin))
	}
	source, ok := plugin["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected source object, got %s", mustJSON(t, plugin))
	}
	if source["source"] != "local" {
		t.Fatalf("expected local source, got %s", mustJSON(t, source))
	}
	if source["path"] != "./plugins/outlook-agent" {
		t.Fatalf("expected marketplace-relative plugin path, got %s", mustJSON(t, source))
	}
	if strings.Contains(source["path"].(string), "..") {
		t.Fatalf("marketplace source path must not escape root: %s", mustJSON(t, source))
	}
	if _, hasLegacyPath := plugin["path"]; hasLegacyPath {
		t.Fatalf("expected source.path schema, got legacy path field: %s", mustJSON(t, plugin))
	}
	if nestedString(t, plugin, "policy", "installation") == "" || nestedString(t, plugin, "policy", "authentication") == "" {
		t.Fatalf("expected marketplace policy fields, got %s", mustJSON(t, plugin))
	}
	if plugin["category"] != "Productivity" {
		t.Fatalf("expected Productivity category, got %s", mustJSON(t, plugin))
	}
}

func TestCodexMarketplaceCommittedPackageMatchesExporter(t *testing.T) {
	exportedDir := filepath.Join(t.TempDir(), "outlook-agent")
	plan, err := BuildPluginExportPlan(skillassets.FS, PluginOptions{
		Client: ClientCodex,
		Output: exportedDir,
		Binary: "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildPluginExportPlan returned error: %v", err)
	}
	if err := ApplyPluginExportPlan(plan); err != nil {
		t.Fatalf("ApplyPluginExportPlan returned error: %v", err)
	}

	assertDirectoryFilesEqual(t, exportedDir, repoPath("..", "..", "plugins", "outlook-agent"))
}

func TestCodexMarketplacePackageContainsNoPrivateConfigOrBinary(t *testing.T) {
	root := repoPath("..", "..", "plugins", "outlook-agent")
	assertNoPrivateGeneratedMarkers(t, root)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&0o111 != 0 {
			t.Fatalf("marketplace package must not include executable binary-like file: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk marketplace package: %v", err)
	}
}

func TestCodexMarketplacePackageUsesPluginRootLayout(t *testing.T) {
	root := repoPath("..", "..", "plugins", "outlook-agent")
	manifest := readJSONMap(t, filepath.Join(root, ".codex-plugin", "plugin.json"))
	if manifest["name"] != "outlook-agent" {
		t.Fatalf("expected manifest name outlook-agent, got %s", mustJSON(t, manifest))
	}
	if manifest["version"] != codexPluginVersion {
		t.Fatalf("expected manifest version %s, got %s", codexPluginVersion, mustJSON(t, manifest))
	}
	if manifest["skills"] != "./skills/" || manifest["mcpServers"] != "./.mcp.json" {
		t.Fatalf("expected Codex component pointers, got %s", mustJSON(t, manifest))
	}
	if manifest["license"] != "Apache-2.0" {
		t.Fatalf("expected Apache-2.0 license, got %s", mustJSON(t, manifest))
	}
	if nestedString(t, manifest, "interface", "displayName") != "Outlook Agent" {
		t.Fatalf("expected interface displayName, got %s", mustJSON(t, manifest))
	}
	for _, forbidden := range []string{"mcp", "mcp_servers", "host", "schema_version"} {
		if _, ok := manifest[forbidden]; ok {
			t.Fatalf("expected Codex manifest to omit field %q, got %s", forbidden, mustJSON(t, manifest))
		}
	}

	readJSONMap(t, filepath.Join(root, ".mcp.json"))
	for _, forbiddenPath := range []string{
		filepath.Join(root, ".codex-plugin", ".mcp.json"),
		filepath.Join(root, ".codex-plugin", "skills"),
	} {
		if _, err := os.Stat(forbiddenPath); err == nil {
			t.Fatalf("plugin root layout is wrong; unexpected path exists: %s", forbiddenPath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", forbiddenPath, err)
		}
	}
}

func TestCodexMarketplaceDocsUseAvailableCLICommands(t *testing.T) {
	data, err := os.ReadFile(repoPath("..", "..", "docs", "PLUGIN_PACKAGING.md"))
	if err != nil {
		t.Fatalf("read plugin packaging docs: %v", err)
	}
	text := string(data)
	for _, required := range []string{
		"codex plugin marketplace add johnkil/outlook-agent --sparse .agents/plugins --sparse plugins",
		"codex plugin marketplace upgrade outlook-agent",
		"codex plugin marketplace remove outlook-agent",
		"Marketplace updates refresh plugin metadata, skills, and MCP packaging only.",
		"does not update the `outlook-agent` binary",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected docs to contain %q", required)
		}
	}
	if strings.Contains(text, "codex plugin marketplace list") {
		t.Fatalf("current Codex CLI does not provide marketplace list; docs must not advertise it")
	}
}

func TestPluginExportCreatesCodexTemplatePackage(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "codex-plugin")
	plan, err := BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client: ClientCodex,
		Output: outputDir,
		Binary: "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildPluginExportPlan returned error: %v", err)
	}
	if plan.Client != ClientCodex || len(plan.Operations) == 0 {
		t.Fatalf("unexpected plugin plan: %#v", plan)
	}

	if err := ApplyPluginExportPlan(plan); err != nil {
		t.Fatalf("ApplyPluginExportPlan returned error: %v", err)
	}
	assertJSONFile(t, filepath.Join(outputDir, ".codex-plugin", "plugin.json"))
	assertJSONFile(t, filepath.Join(outputDir, ".mcp.json"))
	assertFileContent(t, filepath.Join(outputDir, "skills", "outlook-mail", "SKILL.md"), testSkillContent("outlook-mail"))
	manifestData, err := os.ReadFile(filepath.Join(outputDir, ".codex-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin manifest: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("plugin manifest is not JSON: %v; content=%s", err, string(manifestData))
	}
	if manifest["skills"] != "./skills/" || manifest["mcpServers"] != "./.mcp.json" {
		t.Fatalf("expected Codex manifest component pointers, got %s", string(manifestData))
	}
	if _, hasCustomMCP := manifest["mcp"]; hasCustomMCP {
		t.Fatalf("expected Codex manifest to omit custom mcp object, got %s", string(manifestData))
	}
	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read MCP package file: %v", err)
	}
	var mcp map[string]any
	if err := json.Unmarshal(mcpData, &mcp); err != nil {
		t.Fatalf("plugin MCP config is not JSON: %v; content=%s", err, string(mcpData))
	}
	if _, ok := mcp["outlook-agent"].(map[string]any); !ok {
		t.Fatalf("expected Codex plugin MCP file to use a direct server map, got %s", string(mcpData))
	}
	if _, hasMCPServers := mcp["mcpServers"]; hasMCPServers {
		t.Fatalf("expected Codex plugin MCP file to omit mcpServers wrapper, got %s", string(mcpData))
	}
	if strings.Contains(string(mcpData), ".local/outlook-agent.json") || strings.Contains(string(mcpData), "/Users/") {
		t.Fatalf("template plugin must not include local config paths: %s", string(mcpData))
	}
}

func TestPluginExportCreatesClaudeTemplatePackage(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "claude-plugin")
	plan, err := BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client: ClientClaudeCode,
		Output: outputDir,
		Binary: "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildPluginExportPlan returned error: %v", err)
	}
	if err := ApplyPluginExportPlan(plan); err != nil {
		t.Fatalf("ApplyPluginExportPlan returned error: %v", err)
	}
	assertJSONFile(t, filepath.Join(outputDir, ".claude-plugin", "plugin.json"))
	assertJSONFile(t, filepath.Join(outputDir, ".mcp.json"))
	assertFileContent(t, filepath.Join(outputDir, "skills", "outlook-calendar", "SKILL.md"), testSkillContent("outlook-calendar"))
	manifestData, err := os.ReadFile(filepath.Join(outputDir, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin manifest: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("plugin manifest is not JSON: %v; content=%s", err, string(manifestData))
	}
	if manifest["skills"] != "./skills/" {
		t.Fatalf("expected Claude manifest skills path pointer, got %s", string(manifestData))
	}
	if manifest["mcpServers"] != "./.mcp.json" {
		t.Fatalf("expected Claude manifest MCP component pointer, got %s", string(manifestData))
	}
	for _, customField := range []string{"mcp", "host", "schema_version"} {
		if _, ok := manifest[customField]; ok {
			t.Fatalf("expected Claude manifest to omit non-component field %q, got %s", customField, string(manifestData))
		}
	}
	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read MCP package file: %v", err)
	}
	var mcp map[string]any
	if err := json.Unmarshal(mcpData, &mcp); err != nil {
		t.Fatalf("plugin MCP config is not JSON: %v; content=%s", err, string(mcpData))
	}
	if _, ok := mcp["mcpServers"].(map[string]any); !ok {
		t.Fatalf("expected Claude plugin MCP file to use standard mcpServers config, got %s", string(mcpData))
	}
}

func TestLocalPluginExportIncludesOnlyConfigPathString(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "codex-plugin-local")
	privateConfig := filepath.Join(t.TempDir(), "outlook-agent.json")
	if err := os.WriteFile(privateConfig, []byte(`{"access_token":"secret","cookie":"secret"}`), 0o600); err != nil {
		t.Fatalf("write private config: %v", err)
	}

	plan, err := BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client:     ClientCodex,
		Output:     outputDir,
		Binary:     "/usr/local/bin/outlook-agent",
		ConfigPath: privateConfig,
		Local:      true,
	})
	if err != nil {
		t.Fatalf("BuildPluginExportPlan returned error: %v", err)
	}
	if err := ApplyPluginExportPlan(plan); err != nil {
		t.Fatalf("ApplyPluginExportPlan returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read MCP package file: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, privateConfig) {
		t.Fatalf("expected local export to include config path string, got %s", text)
	}
	for _, forbidden := range []string{"access_token", "cookie"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Fatalf("local export copied private config contents: %s", text)
		}
	}
}

func TestPluginExportCopiesCanonicalSkillsByteForByte(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "codex-plugin")
	skills, err := LoadCanonicalSkills(skillassets.FS)
	if err != nil {
		t.Fatalf("LoadCanonicalSkills returned error: %v", err)
	}
	plan, err := BuildPluginExportPlan(skillassets.FS, PluginOptions{
		Client: ClientCodex,
		Output: outputDir,
		Binary: "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildPluginExportPlan returned error: %v", err)
	}
	if err := ApplyPluginExportPlan(plan); err != nil {
		t.Fatalf("ApplyPluginExportPlan returned error: %v", err)
	}

	for _, skill := range skills {
		target := filepath.Join(outputDir, "skills", skill.Name, "SKILL.md")
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read exported skill %s: %v", skill.Name, err)
		}
		if !bytes.Equal(data, skill.Content) {
			t.Fatalf("exported skill %s differs from canonical source", skill.Name)
		}
	}
	assertNoPrivateGeneratedMarkers(t, outputDir)
}

func TestPluginExportRefusesNonEmptyOutputWithoutForce(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "codex-plugin")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "README.md"), []byte("operator notes\n"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	_, err := BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client: ClientCodex,
		Output: outputDir,
		Binary: "outlook-agent",
	})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected non-empty output to require --force, got %v", err)
	}
}

func TestPluginExportAllowsIdenticalExistingOutputWithoutForce(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "codex-plugin")
	plan, err := BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client: ClientCodex,
		Output: outputDir,
		Binary: "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildPluginExportPlan returned error: %v", err)
	}
	if err := ApplyPluginExportPlan(plan); err != nil {
		t.Fatalf("ApplyPluginExportPlan returned error: %v", err)
	}

	plan, err = BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client: ClientCodex,
		Output: outputDir,
		Binary: "outlook-agent",
	})
	if err != nil {
		t.Fatalf("expected identical generated output to be reusable without --force: %v", err)
	}
	for _, operation := range plan.Operations {
		if operation.Kind != OperationSkip {
			t.Fatalf("expected identical output to skip all operations, got %#v", plan.Operations)
		}
	}
}

func TestPluginExportAllowsNonEmptyOutputWithForce(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "codex-plugin")
	if err := os.MkdirAll(filepath.Join(outputDir, "skills", "outlook-mail"), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	staleSkill := filepath.Join(outputDir, "skills", "outlook-mail", "SKILL.md")
	if err := os.WriteFile(staleSkill, []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("write stale skill: %v", err)
	}

	plan, err := BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client: ClientCodex,
		Output: outputDir,
		Binary: "outlook-agent",
		Force:  true,
	})
	if err != nil {
		t.Fatalf("BuildPluginExportPlan returned error: %v", err)
	}
	if err := ApplyPluginExportPlan(plan); err != nil {
		t.Fatalf("ApplyPluginExportPlan returned error: %v", err)
	}
	assertFileContent(t, staleSkill, testSkillContent("outlook-mail"))
}

func TestPluginExportRejectsOutputRootSymlink(t *testing.T) {
	root := t.TempDir()
	realOutput := filepath.Join(root, "real-output")
	if err := os.MkdirAll(realOutput, 0o755); err != nil {
		t.Fatalf("create real output dir: %v", err)
	}
	outputLink := filepath.Join(root, "plugin-link")
	if err := os.Symlink(realOutput, outputLink); err != nil {
		t.Fatalf("create output symlink: %v", err)
	}

	_, err := BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client: ClientCodex,
		Output: outputLink,
		Binary: "outlook-agent",
		Force:  true,
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected output symlink rejection even with force, got %v", err)
	}
}

func TestPluginExportRejectsUnsafeRelativeOutputTraversal(t *testing.T) {
	_, err := BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client: ClientCodex,
		Output: filepath.Join("..", "codex-plugin"),
	})
	if err == nil || !strings.Contains(err.Error(), "output") {
		t.Fatalf("expected output traversal rejection, got %v", err)
	}
}

func assertNoPrivateGeneratedMarkers(t *testing.T, root string) {
	t.Helper()
	forbidden := []string{
		"access_token",
		"refresh_token",
		"x-owa-canary",
		"approval_secret",
		"cookie",
		"/users/",
		"/home/",
		"c:\\users\\",
		"alfabank",
		"alfaintra",
		"moscow\\",
	}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, marker := range forbidden {
			if strings.Contains(lower, marker) {
				t.Fatalf("generated plugin file %s contains private marker %q", path, marker)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("walk generated plugin package: %v", err)
	}
}

func repoPath(parts ...string) string {
	return filepath.Join(parts...)
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("%s is not JSON: %v; content=%s", path, err, string(data))
	}
	return payload
}

func nestedString(t *testing.T, payload map[string]any, parent string, child string) string {
	t.Helper()
	nested, ok := payload[parent].(map[string]any)
	if !ok {
		return ""
	}
	value, ok := nested[child].(string)
	if !ok {
		return ""
	}
	return value
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return string(data)
}

func assertDirectoryFilesEqual(t *testing.T, expectedRoot string, actualRoot string) {
	t.Helper()
	expectedFiles := collectRelativeFiles(t, expectedRoot)
	actualFiles := collectRelativeFiles(t, actualRoot)
	if strings.Join(expectedFiles, "\n") != strings.Join(actualFiles, "\n") {
		t.Fatalf("marketplace package files differ from exporter\nexpected:\n%s\nactual:\n%s", strings.Join(expectedFiles, "\n"), strings.Join(actualFiles, "\n"))
	}
	for _, relative := range expectedFiles {
		expected, err := os.ReadFile(filepath.Join(expectedRoot, relative))
		if err != nil {
			t.Fatalf("read expected %s: %v", relative, err)
		}
		actual, err := os.ReadFile(filepath.Join(actualRoot, relative))
		if err != nil {
			t.Fatalf("read actual %s: %v", relative, err)
		}
		if !bytes.Equal(expected, actual) {
			t.Fatalf("marketplace package file %s differs from exporter", relative)
		}
	}
}

func collectRelativeFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

func assertJSONFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("%s is not JSON: %v; content=%s", path, err, string(data))
	}
}

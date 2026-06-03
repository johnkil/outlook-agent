package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/approval"
	"github.com/johnkil/outlook-agent/internal/buildinfo"
	"github.com/johnkil/outlook-agent/internal/capability"
	"github.com/johnkil/outlook-agent/internal/config"
	"github.com/johnkil/outlook-agent/internal/mstimezone"
	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	setupcore "github.com/johnkil/outlook-agent/internal/setup"
	"github.com/johnkil/outlook-agent/internal/setupopencode"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/ews"
	"github.com/johnkil/outlook-agent/internal/transport/fake"
	"github.com/johnkil/outlook-agent/internal/transport/graph"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
	skillassets "github.com/johnkil/outlook-agent/skills"
)

type Options struct {
	ConfigPath string
	Profile    string
}

type Runtime struct {
	BuildTransport        func(context.Context, Options) (transport.Transport, string, error)
	EnrollGraphDeviceCode func(context.Context, Options, func(GraphDeviceCodeChallenge)) (GraphDeviceCodeResult, error)
	RunMCP                func(context.Context, Options) error
}

var now = time.Now

type GraphDeviceCodeChallenge struct {
	VerificationURI string
	UserCode        string
	Message         string
	ExpiresIn       int
	Interval        int
}

type GraphDeviceCodeResult struct {
	Profile   string
	SecretRef string
	TokenType string
	Scope     string
	ExpiresAt string
}

type owaActionDiscoverer interface {
	DiscoverServiceActionsFromURLWithOptions(ctx context.Context, source string, options owa.DiscoveryOptions) ([]string, error)
}

type owaActionDiscoveryDiagnoser interface {
	DiscoverServiceActionsFromURLDiagnostics(ctx context.Context, source string, options owa.DiscoveryOptions) (owa.DiscoveryDiagnostics, error)
}

type owaActionContextDiagnoser interface {
	DiscoverServiceActionContextsFromURLDiagnostics(ctx context.Context, source string, action string, options owa.DiscoveryOptions) (owa.ActionContextDiagnostics, error)
}

// Run executes the CLI command and returns the process exit code.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	return RunWithRuntime(args, stdout, stderr, Runtime{})
}

func RunWithRuntime(args []string, stdout io.Writer, stderr io.Writer, runtime Runtime) int {
	if len(args) == 0 {
		return writeHelp(stdout)
	}
	if len(args) == 1 && isHelpCommand(args[0]) {
		return writeHelp(stdout)
	}
	if setupArgs, ok := setupOpencodeArgsFromRaw(args); ok {
		options, _, err := parseOptionsBeforeCommand(args)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runSetupOpencode(setupArgs, options, stdout, stderr)
	}
	if setupArgs, ok := setupSkillsArgsFromRaw(args); ok {
		return runSetupSkills(setupArgs, stdout, stderr)
	}
	if setupArgs, ok := setupAgentArgsFromRaw(args); ok {
		options, _, err := parseOptionsBeforeCommand(args)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runSetupAgent(setupArgs, options, stdout, stderr)
	}
	if setupArgs, ok := setupApprovalArgsFromRaw(args); ok {
		options, _, err := parseOptionsBeforeCommand(args)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runSetupApproval(setupArgs, options, stdout, stderr)
	}
	if setupArgs, ok := setupPluginArgsFromRaw(args); ok {
		options, _, err := parseOptionsBeforeCommand(args)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runSetupPlugin(setupArgs, options, stdout, stderr)
	}

	options, commandArgs, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(commandArgs) == 0 {
		return writeHelp(stdout)
	}

	switch commandArgs[0] {
	case "", "help", "--help", "-h":
		return writeHelp(stdout)
	case "doctor":
		return runDoctor(stdout, options)
	case "version":
		return runVersion(stdout)
	case "policy":
		if len(commandArgs) == 2 && commandArgs[1] == "explain" {
			return writeJSON(stdout, map[string]any{
				"ok":             true,
				"command":        "policy explain",
				"safety_classes": policy.SafetyClassNames(),
			})
		}
		if len(commandArgs) == 2 && commandArgs[1] == "coverage" {
			return runPolicyCoverage(stdout)
		}
		if len(commandArgs) == 4 && commandArgs[1] == "explain" && commandArgs[2] == "--action" {
			return runPolicyExplainAction(stdout, commandArgs[3])
		}
	case "owa":
		if len(commandArgs) >= 2 && commandArgs[1] == "discover-actions" {
			return runOWADiscoverActions(commandArgs[2:], options, runtime, stdout, stderr)
		}
		if len(commandArgs) >= 2 && commandArgs[1] == "discover-action-context" {
			return runOWADiscoverActionContext(commandArgs[2:], options, runtime, stdout, stderr)
		}
	case "people":
		return runPeopleCommand(commandArgs[1:], options, runtime, stdout, stderr)
	case "calendar":
		return runCalendarCommand(commandArgs[1:], options, runtime, stdout, stderr)
	case "auth":
		if len(commandArgs) == 2 && commandArgs[1] == "check" {
			return runAuthCheck(stdout, options, runtime)
		}
		if len(commandArgs) == 2 && commandArgs[1] == "graph-device-code" {
			return runAuthGraphDeviceCode(stdout, stderr, options, runtime)
		}
	case "mcp":
		if runtime.RunMCP == nil {
			fmt.Fprintln(stderr, "mcp runner is not configured")
			return 4
		}
		if err := runtime.RunMCP(context.Background(), options); err != nil {
			fmt.Fprintf(stderr, "mcp server failed: %v\n", err)
			return 4
		}
		return 0
	}

	fmt.Fprintf(stderr, "unknown command: %s\n", commandArgs[0])
	return 1
}

type graphDeviceCodeOutput struct {
	OK        bool   `json:"ok"`
	Command   string `json:"command"`
	Profile   string `json:"profile,omitempty"`
	SecretRef string `json:"secret_ref,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	Scope     string `json:"scope,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Error     string `json:"error,omitempty"`
}

type doctorConfigOutput struct {
	Found bool   `json:"found"`
	Kind  string `json:"kind"`
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
}

type doctorSecretStoreOutput struct {
	Kind                string `json:"kind"`
	Available           bool   `json:"available"`
	RefConfigured       bool   `json:"ref_configured"`
	Readable            bool   `json:"readable,omitempty"`
	Writable            bool   `json:"writable,omitempty"`
	ProviderConfigured  bool   `json:"provider_configured,omitempty"`
	RequiresCGOForWrite bool   `json:"requires_cgo_for_write,omitempty"`
	Warning             string `json:"warning,omitempty"`
	Recommendation      string `json:"recommendation,omitempty"`
}

type doctorApprovalOutput struct {
	Mode                    string `json:"mode"`
	RequiredByDefault       bool   `json:"required_by_default"`
	SecretConfigured        bool   `json:"secret_configured"`
	LegacyTokenConfigured   bool   `json:"legacy_token_configured,omitempty"`
	HostIntegrationRequired bool   `json:"host_integration_required"`
	Warning                 string `json:"warning,omitempty"`
}

type doctorOutput struct {
	OK          bool                    `json:"ok"`
	Command     string                  `json:"command"`
	Version     string                  `json:"version"`
	Profile     string                  `json:"profile,omitempty"`
	Config      doctorConfigOutput      `json:"config"`
	SecretStore doctorSecretStoreOutput `json:"secret_store"`
	Approval    doctorApprovalOutput    `json:"approval"`
	MCPStdio    bool                    `json:"mcp_stdio"`
	Transports  []string                `json:"transports"`
	NextSteps   []string                `json:"next_steps,omitempty"`
	Error       string                  `json:"error,omitempty"`
}

type versionOutput struct {
	OK      bool   `json:"ok"`
	Command string `json:"command"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Dirty   string `json:"dirty"`
	BuiltBy string `json:"built_by"`
}

type setupOpencodeOutput struct {
	Schema string                            `json:"$schema,omitempty"`
	MCP    map[string]setupOpencodeMCPServer `json:"mcp"`
}

type setupOpencodeMCPServer struct {
	Type    string   `json:"type"`
	Command []string `json:"command"`
	Enabled bool     `json:"enabled"`
}

func runDoctor(stdout io.Writer, options Options) int {
	loaded, source, err := config.Load(config.Options{ExplicitPath: options.ConfigPath})
	profile := options.Profile
	if profile == "" {
		profile = loaded.DefaultProfile
	}
	currentBuild := buildinfo.Current()
	secretStore := doctorSecretStore(profile, loaded)
	approvalReadiness := doctorApproval(profile, loaded)
	output := doctorOutput{
		OK:      err == nil,
		Command: "doctor",
		Version: currentBuild.Version,
		Profile: profile,
		Config: doctorConfigOutput{
			Found: source.Found,
			Kind:  source.Kind,
			Path:  source.Path,
		},
		SecretStore: secretStore,
		Approval:    approvalReadiness,
		MCPStdio:    true,
		Transports:  []string{"fake", "graph", "ews", "owa"},
	}
	if err != nil {
		output.Error = err.Error()
		output.Config.Error = err.Error()
		output.NextSteps = doctorNextSteps(output)
		writeJSON(stdout, output)
		return 1
	}
	output.NextSteps = doctorNextSteps(output)
	return writeJSON(stdout, output)
}

func runVersion(stdout io.Writer) int {
	currentBuild := buildinfo.Current()
	return writeJSON(stdout, versionOutput{
		OK:      true,
		Command: "version",
		Version: currentBuild.Version,
		Commit:  currentBuild.Commit,
		Date:    currentBuild.Date,
		Dirty:   currentBuild.Dirty,
		BuiltBy: currentBuild.BuiltBy,
	})
}

const doctorMaxFileSecretBytes = 1024 * 1024

func doctorSecretStore(profile string, loaded config.Config) doctorSecretStoreOutput {
	configuredProfile, ok := loaded.Profiles[profile]
	transportName := "fake"
	if ok && strings.TrimSpace(configuredProfile.Transport) != "" {
		transportName = strings.TrimSpace(configuredProfile.Transport)
	}
	rawRef := ""
	if ok {
		rawRef = strings.TrimSpace(configuredProfile.SecretRef)
	}
	if rawRef == "" {
		if transportName == "fake" {
			return doctorSecretStoreOutput{Kind: "none", Available: true}
		}
		return doctorSecretStoreOutput{
			Kind:           "none",
			Available:      false,
			RefConfigured:  false,
			Warning:        "live profile transport " + transportName + " requires a secret_ref",
			Recommendation: "Configure a secret_ref for the selected live profile using keychain:, file:, or external:.",
		}
	}

	ref := secret.Ref(rawRef)
	switch {
	case strings.HasPrefix(rawRef, "file:"):
		return doctorFileSecretStore(ref)
	case strings.HasPrefix(rawRef, "external:"):
		return doctorExternalSecretStore(ref, loaded.Secrets.External)
	case strings.HasPrefix(rawRef, "keychain:"):
		return doctorKeychainSecretStore(ref)
	default:
		return doctorSecretStoreOutput{
			Kind:           "unknown",
			Available:      false,
			RefConfigured:  true,
			Warning:        "unsupported secret_ref prefix",
			Recommendation: "Use keychain:service/account, file:/absolute/path, or external:name.",
		}
	}
}

func doctorFileSecretStore(ref secret.Ref) doctorSecretStoreOutput {
	output := doctorSecretStoreOutput{
		Kind:          "file",
		RefConfigured: true,
	}
	path, err := secret.ParseFileRef(ref)
	if err != nil {
		output.Warning = err.Error()
		output.Recommendation = "Use file:/absolute/path with user-only permissions for file-backed secrets."
		return output
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			output.Writable = directoryLooksUserWritable(filepath.Dir(path))
			output.Warning = "file secret not found"
			output.Recommendation = "Create the file secret with user-only permissions before using this live profile."
			return output
		}
		output.Warning = "file secret is not accessible"
		output.Recommendation = "Check the file secret path and permissions."
		return output
	}
	if info.IsDir() {
		output.Warning = "file secret path points to a directory"
		output.Recommendation = "Use file:/absolute/path pointing to a regular user-only secret file."
		return output
	}
	output.Writable = info.Mode().Perm()&0o200 != 0
	if info.Mode().Perm()&0o077 != 0 {
		output.Warning = "file secret must have user-only permissions"
		output.Recommendation = "Restrict the file secret permissions to 0600 before using this profile."
		return output
	}
	if info.Size() > doctorMaxFileSecretBytes {
		output.Warning = "file secret is too large"
		output.Recommendation = "Replace the file secret with a bounded token file."
		return output
	}
	handle, err := os.Open(path)
	if err != nil {
		output.Warning = "file secret is not readable"
		output.Recommendation = "Check the file secret path and owner permissions."
		return output
	}
	_ = handle.Close()
	output.Readable = true
	output.Available = true
	return output
}

func directoryLooksUserWritable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o200 != 0
}

func doctorExternalSecretStore(ref secret.Ref, configured map[string]config.ExternalSecretCommand) doctorSecretStoreOutput {
	output := doctorSecretStoreOutput{
		Kind:          "external",
		RefConfigured: true,
	}
	name, err := secret.ParseExternalRef(ref)
	if err != nil {
		output.Warning = err.Error()
		output.Recommendation = "Use external:name without path separators and define secrets.external[name]."
		return output
	}
	command, ok := configured[name]
	output.ProviderConfigured = ok
	if !ok {
		output.Warning = "external secret provider is not configured"
		output.Recommendation = "Define the external secret provider under secrets.external before using this profile."
		return output
	}
	commandPath := strings.TrimSpace(command.Command)
	if commandPath == "" {
		output.Warning = "external secret command is empty"
		output.Recommendation = "Configure an absolute command path for the external secret provider."
		return output
	}
	if !filepath.IsAbs(commandPath) {
		output.Warning = "external secret command must be absolute"
		output.Recommendation = "Use an absolute command path for the external secret provider."
		return output
	}
	info, err := os.Stat(commandPath)
	if err != nil {
		output.Warning = "external secret command is not accessible"
		output.Recommendation = "Check that the external secret command exists and is executable."
		return output
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		output.Warning = "external secret command is not executable"
		output.Recommendation = "Point the external secret provider at an executable command."
		return output
	}
	output.Readable = true
	output.Available = true
	return output
}

func doctorKeychainSecretStore(ref secret.Ref) doctorSecretStoreOutput {
	output := doctorSecretStoreOutput{
		Kind:          "keychain",
		RefConfigured: true,
		Readable:      secret.KeychainReadSupported(),
		Writable:      secret.KeychainWriteSupported(),
	}
	if _, err := secret.ParseKeychainRef(ref); err != nil {
		output.Warning = err.Error()
		output.Recommendation = "Use keychain:service/account for macOS Keychain-backed secrets."
		return output
	}
	output.Available = output.Readable
	if limitation := secret.KeychainWriteLimitation(); limitation != "" {
		output.Warning = limitation
		output.Recommendation = keychainRecommendation(limitation)
		output.RequiresCGOForWrite = strings.Contains(strings.ToLower(limitation), "cgo")
	}
	return output
}

func keychainRecommendation(limitation string) string {
	if strings.Contains(strings.ToLower(limitation), "cgo") {
		return "Use file: or external: for portable enrollment writes, or run a local darwin+cgo build when Keychain writes are required."
	}
	return "Use file: or external: secret stores on platforms without macOS Keychain support."
}

func doctorApproval(profile string, loaded config.Config) doctorApprovalOutput {
	transportName := "fake"
	if selected, ok := loaded.Profiles[profile]; ok && strings.TrimSpace(selected.Transport) != "" {
		transportName = selected.Transport
	}
	policy := approval.PolicyFromEnv(transportName, os.Getenv)
	defaultPolicy := approval.PolicyFromEnv(transportName, func(string) string { return "" })
	output := doctorApprovalOutput{
		Mode:                    string(policy.Mode),
		RequiredByDefault:       defaultPolicy.Mode == approval.ModeRequired,
		SecretConfigured:        strings.TrimSpace(policy.Secret) != "",
		LegacyTokenConfigured:   strings.TrimSpace(policy.LegacyToken) != "",
		HostIntegrationRequired: policy.Mode == approval.ModeRequired,
	}
	switch {
	case output.HostIntegrationRequired && !output.SecretConfigured:
		output.Warning = "OUTLOOK_AGENT_APPROVAL_SECRET is required for high-risk actions in required approval mode"
	case policy.Mode == approval.ModeOptional && output.LegacyTokenConfigured && defaultPolicy.Mode == approval.ModeRequired:
		output.Warning = "legacy OUTLOOK_AGENT_APPROVAL_TOKEN is compatibility-only and not production-grade approval"
	}
	return output
}

type policyExplainActionOutput struct {
	OK      bool                `json:"ok"`
	Command string              `json:"command"`
	Action  string              `json:"action"`
	Matches []capability.Detail `json:"matches"`
	Unknown *capability.Detail  `json:"unknown,omitempty"`
}

type policyCoverageOutput struct {
	OK      bool                  `json:"ok"`
	Command string                `json:"command"`
	Actions []policyCoverageRow   `json:"actions"`
	Summary policyCoverageSummary `json:"summary"`
}

type policyCoverageRow struct {
	Action                 string `json:"action"`
	Transport              string `json:"transport"`
	SafetyClass            string `json:"safety_class"`
	Level                  int    `json:"level"`
	AllowedDirect          bool   `json:"allowed_direct"`
	RequiresDryRun         bool   `json:"requires_dry_run"`
	RequiresConfirmation   bool   `json:"requires_confirmation"`
	RequiresUnsafe         bool   `json:"requires_unsafe,omitempty"`
	RequiresExplicitTarget bool   `json:"requires_explicit_target,omitempty"`
	RequiresExplicitIntent bool   `json:"requires_explicit_intent,omitempty"`
	ExecutionRoute         string `json:"execution_route"`
	LiveCheckLevel         string `json:"live_check_level"`
}

type policyCoverageSummary struct {
	Total            int            `json:"total"`
	ByTransport      map[string]int `json:"by_transport"`
	BySafetyClass    map[string]int `json:"by_safety_class"`
	ByLiveCheckLevel map[string]int `json:"by_live_check_level"`
}

func runPolicyExplainAction(stdout io.Writer, actionName string) int {
	matches := make([]capability.Detail, 0)
	for _, definition := range builtinActionDefinitions() {
		if strings.EqualFold(definition.Name, actionName) {
			matches = append(matches, capability.FromDefinition(definition))
		}
	}
	output := policyExplainActionOutput{
		OK:      true,
		Command: "policy explain",
		Action:  actionName,
		Matches: matches,
	}
	if len(matches) == 0 {
		unknown := capability.FromDefinition(action.Definition{
			Name:      actionName,
			Transport: "",
			Class:     policy.Unknown,
			Level:     action.LevelDiscovered,
		})
		output.Unknown = &unknown
	}
	return writeJSON(stdout, output)
}

func runPolicyCoverage(stdout io.Writer) int {
	definitions := builtinActionDefinitions()
	sort.Slice(definitions, func(left int, right int) bool {
		if definitions[left].Transport == definitions[right].Transport {
			return strings.ToLower(definitions[left].Name) < strings.ToLower(definitions[right].Name)
		}
		return definitions[left].Transport < definitions[right].Transport
	})
	rows := make([]policyCoverageRow, 0, len(definitions))
	summary := policyCoverageSummary{
		ByTransport:      map[string]int{},
		BySafetyClass:    map[string]int{},
		ByLiveCheckLevel: map[string]int{},
	}
	for _, definition := range definitions {
		detail := capability.FromDefinition(definition)
		liveCheckLevel := liveCheckLevelFor(definition.Class)
		rows = append(rows, policyCoverageRow{
			Action:                 definition.Name,
			Transport:              definition.Transport,
			SafetyClass:            detail.SafetyClass,
			Level:                  detail.Level,
			AllowedDirect:          detail.AllowedDirect,
			RequiresDryRun:         detail.RequiresDryRun,
			RequiresConfirmation:   detail.RequiresConfirmation,
			RequiresUnsafe:         detail.RequiresUnsafe,
			RequiresExplicitTarget: detail.RequiresExplicitTarget,
			RequiresExplicitIntent: detail.RequiresExplicitIntent,
			ExecutionRoute:         detail.ExecutionRoute,
			LiveCheckLevel:         liveCheckLevel,
		})
		summary.Total++
		summary.ByTransport[definition.Transport]++
		summary.BySafetyClass[string(definition.Class)]++
		summary.ByLiveCheckLevel[liveCheckLevel]++
	}
	return writeJSON(stdout, policyCoverageOutput{
		OK:      true,
		Command: "policy coverage",
		Actions: rows,
		Summary: summary,
	})
}

func liveCheckLevelFor(class policy.SafetyClass) string {
	switch class {
	case policy.ReadMetadata:
		return "live_readonly"
	case policy.ReadBodyExplicit, policy.ReadAttachmentExplicit:
		return "manual_explicit_target"
	case policy.DraftOnly:
		return "live_safe_execute"
	case policy.ReversibleSingleItem, policy.ReversibleBulk:
		return "live_dry_run"
	case policy.Destructive, policy.SendLike, policy.SettingsOrRules, policy.Unknown:
		return "live_guard_only"
	default:
		return "live_guard_only"
	}
}

func builtinActionDefinitions() []action.Definition {
	clients := []transport.Transport{
		fake.New(),
		graph.NewTransport(graph.Config{BaseURL: "https://graph.example.test/v1.0", SecretRef: secret.Ref("keychain:graph.example.test/access-token")}, nil, nil),
		ews.NewTransport(ews.Config{EndpointURL: "https://mail.example.test/EWS/Exchange.asmx", Username: "DOMAIN\\user", SecretRef: secret.Ref("keychain:mail.example.test/DOMAIN\\user")}, nil, nil),
		owa.NewTransport(owa.Config{BaseURL: "https://mail.example.test", Username: "DOMAIN\\user", SecretRef: secret.Ref("keychain:mail.example.test/DOMAIN\\user")}, nil, nil),
	}
	definitions := make([]action.Definition, 0)
	for _, client := range clients {
		definitions = append(definitions, client.Capabilities(context.Background()).Actions...)
	}
	return definitions
}

func runPeopleCommand(args []string, options Options, runtime Runtime, stdout io.Writer, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "people command requires search or resolve")
		return 1
	}
	switch args[0] {
	case "search":
		query, mailbox, err := parsePeopleArgs(args[1:])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runTypedReadAction(stdout, options, runtime, "people search", "people.search", map[string]any{"query": query, "mailbox": mailbox}, "people")
	case "resolve":
		query, mailbox, err := parsePeopleArgs(args[1:])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runTypedReadAction(stdout, options, runtime, "people resolve", "people.resolve", map[string]any{"query": query, "mailbox": mailbox}, "person")
	default:
		fmt.Fprintf(stderr, "unknown people command: %s\n", args[0])
		return 1
	}
}

func runCalendarCommand(args []string, options Options, runtime Runtime, stdout io.Writer, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "calendar command requires list, availability, find-time, mutual-free, create-meeting, delete-event, or cancel-meeting")
		return 1
	}
	switch args[0] {
	case "list":
		payload, err := parseCalendarWindowArgs(args[1:], false)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runTypedReadAction(stdout, options, runtime, "calendar list", "calendar.list", payload, "events")
	case "availability":
		payload, err := parseCalendarWindowArgs(args[1:], true)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runCalendarAvailability(stdout, options, runtime, payload)
	case "find-time", "mutual-free":
		payload, err := parseCalendarFindTimeArgs(args[1:])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runCalendarFindTime(stdout, options, runtime, payload)
	case "create-meeting":
		payload, dryRunOnly, confirmToken, err := parseCalendarCreateMeetingArgs(args[1:])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runCalendarCreateMeeting(stdout, options, runtime, payload, dryRunOnly, confirmToken)
	case "delete-event":
		payload, err := parseCalendarDeleteEventArgs(args[1:])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runCalendarMutationDryRun(stdout, options, runtime, "calendar delete-event dry-run", "calendar.delete_event", payload)
	case "cancel-meeting":
		payload, err := parseCalendarCancelMeetingArgs(args[1:])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runCalendarMutationDryRun(stdout, options, runtime, "calendar cancel-meeting dry-run", "calendar.cancel_meeting", payload)
	default:
		fmt.Fprintf(stderr, "unknown calendar command: %s\n", args[0])
		return 1
	}
}

func runCalendarAvailability(stdout io.Writer, options Options, runtime Runtime, payload map[string]any) int {
	client, errCode, err := buildCLITransport(stdout, options, runtime, "calendar availability")
	if err != nil {
		return errCode
	}
	if query := stringAny(payload["with"]); query != "" {
		email, resolveData, err := resolvePersonEmail(client, query, stringAny(payload["mailbox"]))
		if err != nil {
			_ = writeJSON(stdout, resolveErrorOutput("calendar availability", err, resolveData))
			return 3
		}
		payload["email"] = email
		delete(payload, "with")
	}
	return runTypedReadActionWithClient(stdout, client, "calendar availability", "calendar.availability", payload, "windows")
}

func runCalendarFindTime(stdout io.Writer, options Options, runtime Runtime, payload map[string]any) int {
	client, errCode, err := buildCLITransport(stdout, options, runtime, "calendar find-time")
	if err != nil {
		return errCode
	}
	attendees := stringSliceAny(payload["attendees"])
	for _, query := range stringSliceAny(payload["with"]) {
		email, resolveData, err := resolvePersonEmail(client, query, stringAny(payload["mailbox"]))
		if err != nil {
			_ = writeJSON(stdout, resolveErrorOutput("calendar find-time", err, resolveData))
			return 3
		}
		attendees = append(attendees, email)
	}
	payload["attendees"] = attendees
	delete(payload, "with")
	return runTypedReadActionWithClient(stdout, client, "calendar find-time", "calendar.find_time", payload, "suggestions")
}

func runCalendarCreateMeeting(stdout io.Writer, options Options, runtime Runtime, payload map[string]any, dryRunOnly bool, confirmToken string) int {
	client, errCode, err := buildCLITransport(stdout, options, runtime, "calendar create-meeting")
	if err != nil {
		return errCode
	}
	attendees := nonEmptyStrings(stringSliceAny(payload["attendees"]))
	for _, query := range stringSliceAny(payload["with"]) {
		email, resolveData, err := resolvePersonEmail(client, query, stringAny(payload["mailbox"]))
		if err != nil {
			_ = writeJSON(stdout, resolveErrorOutput("calendar create-meeting", err, resolveData))
			return 3
		}
		attendees = append(attendees, email)
	}
	payload["attendees"] = nonEmptyStrings(attendees)
	delete(payload, "with")
	if dryRunOnly {
		summary := client.DryRun(context.Background(), transport.ActionRequest{Name: "calendar.create_meeting", Payload: payload})
		return writeJSON(stdout, map[string]any{
			"ok":      summary.Error == "",
			"command": "calendar create-meeting dry-run",
			"dry_run": summary,
		})
	}
	if strings.TrimSpace(confirmToken) == "" {
		_ = writeJSON(stdout, map[string]any{
			"ok":      false,
			"command": "calendar create-meeting",
			"error":   "calendar create-meeting CLI supports review-only dry-run and does not issue confirmation tokens; confirmed execution must go through MCP outlook.calendar_create_meeting or outlook.action_confirm",
		})
		return 1
	}
	_ = writeJSON(stdout, map[string]any{
		"ok":      false,
		"command": "calendar create-meeting",
		"error":   "confirmed calendar create-meeting execution is available through MCP outlook.calendar_create_meeting or outlook.action_confirm",
	})
	return 1
}

func runCalendarMutationDryRun(stdout io.Writer, options Options, runtime Runtime, command string, actionName string, payload map[string]any) int {
	client, errCode, err := buildCLITransport(stdout, options, runtime, command)
	if err != nil {
		return errCode
	}
	summary := client.DryRun(context.Background(), transport.ActionRequest{Name: actionName, Payload: payload})
	return writeJSON(stdout, map[string]any{
		"ok":      summary.Error == "",
		"command": command,
		"dry_run": summary,
	})
}

func resolveErrorOutput(command string, err error, data map[string]any) map[string]any {
	output := map[string]any{"ok": false, "command": command, "error": err.Error()}
	if data == nil {
		return output
	}
	if candidates, ok := data["candidates"]; ok {
		output["candidates"] = candidates
	}
	output["data"] = data
	return output
}

func runTypedReadAction(stdout io.Writer, options Options, runtime Runtime, command string, actionName string, payload map[string]any, resultKey string) int {
	client, errCode, err := buildCLITransport(stdout, options, runtime, command)
	if err != nil {
		return errCode
	}
	return runTypedReadActionWithClient(stdout, client, command, actionName, payload, resultKey)
}

func buildCLITransport(stdout io.Writer, options Options, runtime Runtime, command string) (transport.Transport, int, error) {
	if runtime.BuildTransport == nil {
		_ = writeJSON(stdout, map[string]any{"ok": false, "command": command, "error": "transport profile is not configured"})
		return nil, 4, errors.New("transport profile is not configured")
	}
	client, _, err := runtime.BuildTransport(context.Background(), options)
	if err != nil {
		_ = writeJSON(stdout, map[string]any{"ok": false, "command": command, "error": err.Error()})
		return nil, 3, err
	}
	return client, 0, nil
}

func runTypedReadActionWithClient(stdout io.Writer, client transport.Transport, command string, actionName string, payload map[string]any, resultKey string) int {
	cleanPayload := map[string]any{}
	for key, value := range payload {
		if value == nil {
			continue
		}
		if text, ok := value.(string); ok && text == "" {
			continue
		}
		cleanPayload[key] = value
	}
	response := client.Execute(context.Background(), transport.ActionRequest{Name: actionName, Payload: cleanPayload})
	output := map[string]any{"ok": response.OK, "command": command}
	if response.OK {
		output[resultKey] = response.Data[resultKey]
		return writeJSON(stdout, output)
	}
	output["error"] = response.Error
	if response.Data != nil {
		output["data"] = response.Data
	}
	_ = writeJSON(stdout, output)
	return 3
}

func resolvePersonEmail(client transport.Transport, query string, mailbox string) (string, map[string]any, error) {
	payload := map[string]any{"query": query}
	if strings.TrimSpace(mailbox) != "" {
		payload["mailbox"] = mailbox
	}
	response := client.Execute(context.Background(), transport.ActionRequest{Name: "people.resolve", Payload: payload})
	if !response.OK {
		return "", response.Data, errors.New(response.Error)
	}
	person, _ := response.Data["person"].(map[string]any)
	email := stringAny(person["email"])
	if email == "" {
		return "", response.Data, fmt.Errorf("people.resolve did not return an email")
	}
	return email, nil, nil
}

func parsePeopleArgs(args []string) (string, string, error) {
	var query string
	var mailbox string
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--json":
			continue
		case "--query":
			index++
			if index >= len(args) {
				return "", "", fmt.Errorf("--query requires a value")
			}
			query = args[index]
		case "--mailbox":
			index++
			if index >= len(args) {
				return "", "", fmt.Errorf("--mailbox requires a value")
			}
			mailbox = args[index]
		default:
			if query == "" {
				query = args[index]
			} else {
				query += " " + args[index]
			}
		}
	}
	if strings.TrimSpace(query) == "" {
		return "", "", fmt.Errorf("people query is required")
	}
	return query, mailbox, nil
}

func parseCalendarWindowArgs(args []string, allowEmail bool) (map[string]any, error) {
	payload := map[string]any{}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--json":
			continue
		case "--start":
			value, next, err := valueArg(args, index, "--start")
			if err != nil {
				return nil, err
			}
			payload["start"] = value
			index = next
		case "--end":
			value, next, err := valueArg(args, index, "--end")
			if err != nil {
				return nil, err
			}
			payload["end"] = value
			index = next
		case "--date":
			value, next, err := valueArg(args, index, "--date")
			if err != nil {
				return nil, err
			}
			payload["date"] = value
			index = next
		case "--timezone":
			value, next, err := valueArg(args, index, "--timezone")
			if err != nil {
				return nil, err
			}
			payload["time_zone"] = value
			index = next
		case "--email":
			if !allowEmail {
				return nil, fmt.Errorf("--email is not supported for calendar list")
			}
			value, next, err := valueArg(args, index, "--email")
			if err != nil {
				return nil, err
			}
			payload["email"] = value
			index = next
		case "--with":
			if !allowEmail {
				return nil, fmt.Errorf("--with is not supported for calendar list")
			}
			value, next, err := valueArg(args, index, "--with")
			if err != nil {
				return nil, err
			}
			payload["with"] = value
			index = next
		case "--mailbox":
			value, next, err := valueArg(args, index, "--mailbox")
			if err != nil {
				return nil, err
			}
			payload["mailbox"] = value
			index = next
		default:
			return nil, fmt.Errorf("unknown calendar option: %s", args[index])
		}
	}
	if err := applyDateWindow(payload, false); err != nil {
		return nil, err
	}
	if payload["start"] == nil || payload["end"] == nil {
		return nil, fmt.Errorf("calendar command requires --start and --end")
	}
	return payload, nil
}

func parseCalendarFindTimeArgs(args []string) (map[string]any, error) {
	payload := map[string]any{"tentative": "busy"}
	attendees := make([]string, 0)
	withPeople := make([]string, 0)
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--json":
			continue
		case "--attendee":
			index++
			if index >= len(args) {
				return nil, fmt.Errorf("--attendee requires a value")
			}
			attendees = append(attendees, args[index])
		case "--email":
			index++
			if index >= len(args) {
				return nil, fmt.Errorf("--email requires a value")
			}
			attendees = append(attendees, args[index])
		case "--with":
			index++
			if index >= len(args) {
				return nil, fmt.Errorf("--with requires a value")
			}
			withPeople = append(withPeople, args[index])
		case "--start":
			value, next, err := valueArg(args, index, "--start")
			if err != nil {
				return nil, err
			}
			payload["start"] = value
			index = next
		case "--end":
			value, next, err := valueArg(args, index, "--end")
			if err != nil {
				return nil, err
			}
			payload["end"] = value
			index = next
		case "--date":
			value, next, err := valueArg(args, index, "--date")
			if err != nil {
				return nil, err
			}
			payload["date"] = value
			index = next
		case "--duration":
			value, next, err := valueArg(args, index, "--duration")
			if err != nil {
				return nil, err
			}
			duration, err := parseDurationMinutes(value)
			if err != nil {
				return nil, err
			}
			payload["duration_minutes"] = duration
			index = next
		case "--min":
			value, next, err := valueArg(args, index, "--min")
			if err != nil {
				return nil, err
			}
			duration, err := parseDurationMinutes(value)
			if err != nil {
				return nil, err
			}
			payload["duration_minutes"] = duration
			index = next
		case "--timezone":
			value, next, err := valueArg(args, index, "--timezone")
			if err != nil {
				return nil, err
			}
			payload["time_zone"] = value
			index = next
		case "--tentative":
			value, next, err := valueArg(args, index, "--tentative")
			if err != nil {
				return nil, err
			}
			if value != "busy" && value != "free" {
				return nil, fmt.Errorf("--tentative must be busy or free")
			}
			payload["tentative"] = value
			index = next
		case "--mailbox":
			value, next, err := valueArg(args, index, "--mailbox")
			if err != nil {
				return nil, err
			}
			payload["mailbox"] = value
			index = next
		default:
			return nil, fmt.Errorf("unknown calendar find-time option: %s", args[index])
		}
	}
	if err := applyDateWindow(payload, true); err != nil {
		return nil, err
	}
	if len(attendees) == 0 && len(withPeople) == 0 {
		return nil, fmt.Errorf("calendar find-time requires at least one --attendee or --with")
	}
	if payload["start"] == nil || payload["end"] == nil {
		return nil, fmt.Errorf("calendar find-time requires --start and --end")
	}
	payload["attendees"] = attendees
	payload["with"] = withPeople
	return payload, nil
}

func parseCalendarCreateMeetingArgs(args []string) (map[string]any, bool, string, error) {
	payload := map[string]any{}
	attendees := make([]string, 0)
	withPeople := make([]string, 0)
	var dryRunOnly bool
	var confirmToken string
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--json":
			continue
		case "--subject":
			value, next, err := valueArg(args, index, "--subject")
			if err != nil {
				return nil, false, "", err
			}
			payload["subject"] = value
			index = next
		case "--attendee":
			value, next, err := valueArg(args, index, "--attendee")
			if err != nil {
				return nil, false, "", err
			}
			attendees = append(attendees, value)
			index = next
		case "--with":
			value, next, err := valueArg(args, index, "--with")
			if err != nil {
				return nil, false, "", err
			}
			withPeople = append(withPeople, value)
			index = next
		case "--start":
			value, next, err := valueArg(args, index, "--start")
			if err != nil {
				return nil, false, "", err
			}
			payload["start"] = value
			index = next
		case "--end":
			value, next, err := valueArg(args, index, "--end")
			if err != nil {
				return nil, false, "", err
			}
			payload["end"] = value
			index = next
		case "--timezone":
			value, next, err := valueArg(args, index, "--timezone")
			if err != nil {
				return nil, false, "", err
			}
			payload["time_zone"] = value
			index = next
		case "--location":
			value, next, err := valueArg(args, index, "--location")
			if err != nil {
				return nil, false, "", err
			}
			payload["location"] = value
			index = next
		case "--body":
			value, next, err := valueArg(args, index, "--body")
			if err != nil {
				return nil, false, "", err
			}
			payload["body"] = value
			index = next
		case "--mailbox":
			value, next, err := valueArg(args, index, "--mailbox")
			if err != nil {
				return nil, false, "", err
			}
			payload["mailbox"] = value
			index = next
		case "--dry-run":
			dryRunOnly = true
		case "--confirm-token":
			value, next, err := valueArg(args, index, "--confirm-token")
			if err != nil {
				return nil, false, "", err
			}
			confirmToken = value
			index = next
		default:
			return nil, false, "", fmt.Errorf("unknown calendar create-meeting option: %s", args[index])
		}
	}
	if strings.TrimSpace(stringAny(payload["subject"])) == "" {
		return nil, false, "", fmt.Errorf("calendar create-meeting requires --subject")
	}
	if payload["start"] == nil || payload["end"] == nil {
		return nil, false, "", fmt.Errorf("calendar create-meeting requires --start and --end")
	}
	attendees = nonEmptyStrings(attendees)
	if len(attendees) == 0 && len(withPeople) == 0 {
		return nil, false, "", fmt.Errorf("calendar create-meeting requires at least one nonblank --attendee or --with")
	}
	payload["attendees"] = attendees
	payload["with"] = withPeople
	return payload, dryRunOnly, confirmToken, nil
}

func parseCalendarDeleteEventArgs(args []string) (map[string]any, error) {
	return parseCalendarEventMutationArgs(args, "delete-event", false, "outlook.calendar_delete_event")
}

func parseCalendarCancelMeetingArgs(args []string) (map[string]any, error) {
	return parseCalendarEventMutationArgs(args, "cancel-meeting", true, "outlook.calendar_cancel_meeting")
}

func parseCalendarEventMutationArgs(args []string, command string, allowComment bool, mcpCommand string) (map[string]any, error) {
	payload := map[string]any{}
	var dryRunOnly bool
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--event-id":
			value, next, err := valueArg(args, index, "--event-id")
			if err != nil {
				return nil, err
			}
			payload["event_id"] = strings.TrimSpace(value)
			index = next
		case "--change-key":
			value, next, err := valueArg(args, index, "--change-key")
			if err != nil {
				return nil, err
			}
			payload["change_key"] = strings.TrimSpace(value)
			index = next
		case "--mailbox":
			value, next, err := valueArg(args, index, "--mailbox")
			if err != nil {
				return nil, err
			}
			payload["mailbox"] = strings.TrimSpace(value)
			index = next
		case "--comment":
			if !allowComment {
				return nil, fmt.Errorf("unknown calendar %s option: %s", command, args[index])
			}
			value, next, err := valueArg(args, index, "--comment")
			if err != nil {
				return nil, err
			}
			payload["comment"] = strings.TrimSpace(value)
			index = next
		case "--dry-run":
			dryRunOnly = true
		default:
			return nil, fmt.Errorf("unknown calendar %s option: %s", command, args[index])
		}
	}
	if strings.TrimSpace(stringAny(payload["event_id"])) == "" {
		return nil, fmt.Errorf("calendar %s requires --event-id", command)
	}
	if !dryRunOnly {
		return nil, fmt.Errorf("calendar %s requires --dry-run; confirmed execution must go through MCP %s or outlook.action_confirm", command, mcpCommand)
	}
	return payload, nil
}

func valueArg(args []string, index int, flag string) (string, int, error) {
	index++
	if index >= len(args) {
		return "", index, fmt.Errorf("%s requires a value", flag)
	}
	return args[index], index, nil
}

func applyDateWindow(payload map[string]any, workingHours bool) error {
	dateText := strings.TrimSpace(stringAny(payload["date"]))
	if dateText == "" {
		return nil
	}
	location := time.Local
	if zone := strings.TrimSpace(stringAny(payload["time_zone"])); zone != "" {
		loaded, err := cliTimeLocation(zone)
		if err != nil {
			return fmt.Errorf("unknown timezone %q", zone)
		}
		location = loaded
	}
	base := now().In(location)
	var day time.Time
	switch strings.ToLower(dateText) {
	case "today":
		day = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, location)
	case "tomorrow":
		next := base.AddDate(0, 0, 1)
		day = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, location)
	default:
		parsed, err := time.ParseInLocation("2006-01-02", dateText, location)
		if err != nil {
			return fmt.Errorf("--date must be today, tomorrow, or YYYY-MM-DD")
		}
		day = parsed
	}
	start := day
	end := day.AddDate(0, 0, 1)
	if workingHours {
		start = time.Date(day.Year(), day.Month(), day.Day(), 9, 0, 0, 0, location)
		end = time.Date(day.Year(), day.Month(), day.Day(), 18, 0, 0, 0, location)
	}
	payload["start"] = start.Format(time.RFC3339)
	payload["end"] = end.Format(time.RFC3339)
	delete(payload, "date")
	return nil
}

func cliTimeLocation(timeZone string) (*time.Location, error) {
	timeZone = strings.TrimSpace(timeZone)
	if mapped := mstimezone.IANALocationName(timeZone); mapped != "" {
		timeZone = mapped
	}
	return time.LoadLocation(timeZone)
}

func parseDurationMinutes(value string) (float64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("duration requires a value")
	}
	if strings.HasSuffix(trimmed, "m") || strings.HasSuffix(trimmed, "h") || strings.HasSuffix(trimmed, "s") {
		duration, err := time.ParseDuration(trimmed)
		if err != nil {
			return 0, fmt.Errorf("duration must be minutes or a Go duration like 30m")
		}
		return duration.Minutes(), nil
	}
	duration, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("duration must be a number of minutes")
	}
	return duration, nil
}

func stringAny(value any) string {
	text, _ := value.(string)
	return text
}

func stringSliceAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		output := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				output = append(output, text)
			}
		}
		return output
	default:
		return nil
	}
}

func nonEmptyStrings(values []string) []string {
	output := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			output = append(output, value)
		}
	}
	return output
}

func runAuthCheck(stdout io.Writer, options Options, runtime Runtime) int {
	if runtime.BuildTransport == nil {
		return writeJSON(stdout, map[string]any{
			"ok":      false,
			"command": "auth check",
			"error":   "transport profile is not configured",
		})
	}
	client, profile, err := runtime.BuildTransport(context.Background(), options)
	if err != nil {
		writeJSON(stdout, map[string]any{
			"ok":      false,
			"command": "auth check",
			"error":   err.Error(),
		})
		return 3
	}
	if profile == "" {
		profile = "default"
	}
	result := client.Authenticate(context.Background(), profile)
	code := 0
	if !result.OK {
		code = 3
	}
	writeJSON(stdout, map[string]any{
		"ok":        result.OK,
		"command":   "auth check",
		"principal": result.Principal,
		"error":     result.Error,
	})
	return code
}

func runAuthGraphDeviceCode(stdout io.Writer, stderr io.Writer, options Options, runtime Runtime) int {
	if runtime.EnrollGraphDeviceCode == nil {
		return writeJSON(stdout, graphDeviceCodeOutput{
			OK:      false,
			Command: "auth graph-device-code",
			Error:   "graph device-code enrollment is not configured",
		})
	}
	result, err := runtime.EnrollGraphDeviceCode(context.Background(), options, func(challenge GraphDeviceCodeChallenge) {
		if challenge.Message != "" {
			fmt.Fprintln(stderr, challenge.Message)
			return
		}
		if challenge.VerificationURI != "" || challenge.UserCode != "" {
			fmt.Fprintf(stderr, "Open %s and enter %s.\n", challenge.VerificationURI, challenge.UserCode)
		}
	})
	if err != nil {
		writeJSON(stdout, graphDeviceCodeOutput{
			OK:      false,
			Command: "auth graph-device-code",
			Profile: result.Profile,
			Error:   err.Error(),
		})
		return 3
	}
	return writeJSON(stdout, graphDeviceCodeOutput{
		OK:        true,
		Command:   "auth graph-device-code",
		Profile:   result.Profile,
		SecretRef: result.SecretRef,
		TokenType: result.TokenType,
		Scope:     result.Scope,
		ExpiresAt: result.ExpiresAt,
	})
}

func runOWADiscoverActions(args []string, options Options, runtime Runtime, stdout io.Writer, stderr io.Writer) int {
	sources, err := parseDiscoverActionsArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	discovered := make([]string, 0)
	diagnosticSources := make([]owa.DiscoverySourceDiagnostics, 0)
	for _, path := range sources.Files {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "read discovery file: %v\n", err)
			return 1
		}
		discovered = append(discovered, owa.DiscoverServiceActions(string(data))...)
	}
	if len(sources.URLs) > 0 {
		if runtime.BuildTransport == nil {
			fmt.Fprintln(stderr, "transport profile is not configured")
			return 4
		}
		client, _, err := runtime.BuildTransport(context.Background(), options)
		if err != nil {
			fmt.Fprintf(stderr, "build transport: %v\n", err)
			return 4
		}
		for _, source := range sources.URLs {
			options := owa.DiscoveryOptions{
				IncludeLinkedScripts:  sources.IncludeLinkedScripts,
				FollowNavigationHints: sources.FollowNavigationHints,
				ContinueOnHTTPError:   sources.Diagnostics,
				MaxSources:            sources.MaxSources,
			}
			var actions []string
			var err error
			if sources.Diagnostics {
				diagnoser, ok := client.(owaActionDiscoveryDiagnoser)
				if !ok {
					fmt.Fprintln(stderr, "configured transport does not support OWA discovery diagnostics")
					return 4
				}
				diagnostics, diagnosticErr := diagnoser.DiscoverServiceActionsFromURLDiagnostics(context.Background(), source, options)
				actions = diagnostics.Actions
				diagnosticSources = append(diagnosticSources, diagnostics.Sources...)
				err = diagnosticErr
			} else {
				discoverer, ok := client.(owaActionDiscoverer)
				if !ok {
					fmt.Fprintln(stderr, "configured transport does not support OWA action discovery")
					return 4
				}
				actions, err = discoverer.DiscoverServiceActionsFromURLWithOptions(context.Background(), source, options)
			}
			if err != nil {
				fmt.Fprintf(stderr, "discover OWA actions: %v\n", err)
				return 4
			}
			discovered = append(discovered, actions...)
		}
	}
	report := owa.CompareDiscoveredServiceActions(discovered)
	if sources.Diagnostics {
		return writeJSON(stdout, struct {
			owa.DiscoveryReport
			Sources []owa.DiscoverySourceDiagnostics `json:"sources"`
		}{
			DiscoveryReport: report,
			Sources:         diagnosticSources,
		})
	}
	return writeJSON(stdout, report)
}

func runOWADiscoverActionContext(args []string, options Options, runtime Runtime, stdout io.Writer, stderr io.Writer) int {
	sources, err := parseDiscoverActionContextArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if runtime.BuildTransport == nil {
		fmt.Fprintln(stderr, "transport profile is not configured")
		return 4
	}
	client, _, err := runtime.BuildTransport(context.Background(), options)
	if err != nil {
		fmt.Fprintf(stderr, "build transport: %v\n", err)
		return 4
	}
	diagnoser, ok := client.(owaActionContextDiagnoser)
	if !ok {
		fmt.Fprintln(stderr, "configured transport does not support OWA action context discovery")
		return 4
	}
	output := owa.ActionContextDiagnostics{
		Action:  sources.Action,
		Sources: []owa.ActionContextSourceDiagnostics{},
	}
	for _, source := range sources.URLs {
		diagnostics, err := diagnoser.DiscoverServiceActionContextsFromURLDiagnostics(context.Background(), source, sources.Action, owa.DiscoveryOptions{
			IncludeLinkedScripts:  sources.IncludeLinkedScripts,
			FollowNavigationHints: sources.FollowNavigationHints,
			ContinueOnHTTPError:   true,
			MaxSources:            sources.MaxSources,
		})
		if err != nil {
			fmt.Fprintf(stderr, "discover OWA action context: %v\n", err)
			return 4
		}
		output.Sources = append(output.Sources, diagnostics.Sources...)
	}
	return writeJSON(stdout, output)
}

type discoverActionSources struct {
	Files                 []string
	URLs                  []string
	IncludeLinkedScripts  bool
	FollowNavigationHints bool
	Diagnostics           bool
	MaxSources            int
}

type discoverActionContextSources struct {
	Action                string
	URLs                  []string
	IncludeLinkedScripts  bool
	FollowNavigationHints bool
	MaxSources            int
}

func parseDiscoverActionsArgs(args []string) (discoverActionSources, error) {
	var sources discoverActionSources
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--file":
			index++
			if index >= len(args) {
				return discoverActionSources{}, fmt.Errorf("--file requires a value")
			}
			sources.Files = append(sources.Files, args[index])
		case "--url":
			index++
			if index >= len(args) {
				return discoverActionSources{}, fmt.Errorf("--url requires a value")
			}
			sources.URLs = append(sources.URLs, args[index])
		case "--include-linked-scripts":
			sources.IncludeLinkedScripts = true
		case "--follow-navigation-hints":
			sources.FollowNavigationHints = true
		case "--diagnostics":
			sources.Diagnostics = true
		case "--max-sources":
			index++
			if index >= len(args) {
				return discoverActionSources{}, fmt.Errorf("--max-sources requires a value")
			}
			value, err := strconv.Atoi(args[index])
			if err != nil || value <= 0 {
				return discoverActionSources{}, fmt.Errorf("--max-sources requires a positive integer")
			}
			sources.MaxSources = value
		default:
			return discoverActionSources{}, fmt.Errorf("unknown discover-actions argument: %s", args[index])
		}
	}
	if len(sources.Files) == 0 && len(sources.URLs) == 0 {
		return discoverActionSources{}, fmt.Errorf("owa discover-actions requires --file or --url")
	}
	return sources, nil
}

func parseDiscoverActionContextArgs(args []string) (discoverActionContextSources, error) {
	var sources discoverActionContextSources
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--action":
			index++
			if index >= len(args) {
				return discoverActionContextSources{}, fmt.Errorf("--action requires a value")
			}
			sources.Action = args[index]
		case "--url":
			index++
			if index >= len(args) {
				return discoverActionContextSources{}, fmt.Errorf("--url requires a value")
			}
			sources.URLs = append(sources.URLs, args[index])
		case "--include-linked-scripts":
			sources.IncludeLinkedScripts = true
		case "--follow-navigation-hints":
			sources.FollowNavigationHints = true
		case "--max-sources":
			index++
			if index >= len(args) {
				return discoverActionContextSources{}, fmt.Errorf("--max-sources requires a value")
			}
			value, err := strconv.Atoi(args[index])
			if err != nil || value <= 0 {
				return discoverActionContextSources{}, fmt.Errorf("--max-sources requires a positive integer")
			}
			sources.MaxSources = value
		default:
			return discoverActionContextSources{}, fmt.Errorf("unknown discover-action-context argument: %s", args[index])
		}
	}
	if sources.Action == "" {
		return discoverActionContextSources{}, fmt.Errorf("owa discover-action-context requires --action")
	}
	if len(sources.URLs) == 0 {
		return discoverActionContextSources{}, fmt.Errorf("owa discover-action-context requires --url")
	}
	return sources, nil
}

func parseOptions(args []string) (Options, []string, error) {
	var options Options
	commandArgs := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--config":
			index++
			if index >= len(args) {
				return Options{}, nil, fmt.Errorf("--config requires a value")
			}
			options.ConfigPath = args[index]
		case "--profile":
			index++
			if index >= len(args) {
				return Options{}, nil, fmt.Errorf("--profile requires a value")
			}
			options.Profile = args[index]
		default:
			commandArgs = append(commandArgs, args[index])
		}
	}
	return options, commandArgs, nil
}

const helpText = `Outlook Agent

Safe local CLI and MCP server for Outlook-like mail and calendar access.

Usage:
  outlook-agent help
  outlook-agent --help
  outlook-agent doctor
  outlook-agent version
  outlook-agent auth check --config <path> [--profile <name>]
  outlook-agent auth graph-device-code --config <path> [--profile <name>]
  outlook-agent people search <query> [--config <path>] [--profile <name>]
  outlook-agent people resolve <query> [--config <path>] [--profile <name>]
  outlook-agent calendar list (--date <today|tomorrow|YYYY-MM-DD>|--start <ts> --end <ts>) [--timezone <tz>]
  outlook-agent calendar availability (--email <addr>|--with <query>) (--date <date>|--start <ts> --end <ts>) [--timezone <tz>]
  outlook-agent calendar find-time (--attendee <addr>|--with <query>) (--date <date>|--start <ts> --end <ts>) [--duration <minutes|30m>] [--timezone <tz>] [--tentative busy|free]
  outlook-agent calendar create-meeting --subject <text> (--attendee <addr>|--with <query>) --start <ts> --end <ts> [--timezone <tz>] [--location <text>] [--body <text>] --dry-run
  outlook-agent calendar delete-event --event-id <id> [--change-key <ck>] [--mailbox <mb>] --dry-run
  outlook-agent calendar cancel-meeting --event-id <id> [--change-key <ck>] [--comment <text>] [--mailbox <mb>] --dry-run
  outlook-agent policy explain [--action <name>]
  outlook-agent setup opencode --print [--binary <path>] [--config <path>]
  outlook-agent setup opencode print [--binary <path>] [--config <path>]
  outlook-agent setup opencode plan [--binary <path>] [--config <path>]
  outlook-agent setup opencode diff [--binary <path>] [--config <path>]
  outlook-agent setup opencode apply [--binary <path>] [--config <path>] --yes [--force|--backup]
  outlook-agent setup skills plan --client <opencode|codex|claude-code|all> --scope <project|user>
  outlook-agent setup skills diff --client <opencode|codex|claude-code|all> --scope <project|user>
  outlook-agent setup skills apply --client <opencode|codex|claude-code|all> --scope <project|user> --yes [--backup] [--allow-duplicates]
  outlook-agent setup agent plan --client <opencode|codex|claude-code> --scope <project|user> --config <path> [--use-approval-wrapper]
  outlook-agent setup agent diff --client <opencode|codex|claude-code> --scope <project|user> --config <path>
  outlook-agent setup agent apply --client <opencode|codex|claude-code> --scope <project|user> --config <path> --yes [--backup] [--allow-duplicates]
  outlook-agent setup approval plan --client <opencode|codex|claude-code> --scope <project|user> --config <path> [--secret-file <path>]
  outlook-agent setup approval diff --client <opencode|codex|claude-code> --scope <project|user> --config <path> [--secret-file <path>]
  outlook-agent setup approval apply --client <opencode|codex|claude-code> --scope <project|user> --config <path> --yes [--secret-file <path>]
  outlook-agent setup plugin export --client <codex|claude-code> --output <path> [--local --config <path>] [--binary <path>] [--force]
  outlook-agent mcp --config <path>

Agent workflow:
  Use metadata-first reads. Fetch message bodies and attachments only for
  explicit targets. Use dry-run and exact confirmation for broad, mutating,
  send-like, destructive, or unknown actions.
`

func isHelpCommand(command string) bool {
	return command == "" || command == "help" || command == "--help" || command == "-h"
}

func setupOpencodeArgsFromRaw(args []string) ([]string, bool) {
	commandIndex := firstCommandIndex(args)
	if commandIndex+1 < len(args) && args[commandIndex] == "setup" && args[commandIndex+1] == "opencode" {
		return args[commandIndex+2:], true
	}
	return nil, false
}

func setupSkillsArgsFromRaw(args []string) ([]string, bool) {
	commandIndex := firstCommandIndex(args)
	if commandIndex+1 < len(args) && args[commandIndex] == "setup" && args[commandIndex+1] == "skills" {
		return args[commandIndex+2:], true
	}
	return nil, false
}

func setupAgentArgsFromRaw(args []string) ([]string, bool) {
	commandIndex := firstCommandIndex(args)
	if commandIndex+1 < len(args) && args[commandIndex] == "setup" && args[commandIndex+1] == "agent" {
		return args[commandIndex+2:], true
	}
	return nil, false
}

func setupApprovalArgsFromRaw(args []string) ([]string, bool) {
	commandIndex := firstCommandIndex(args)
	if commandIndex+1 < len(args) && args[commandIndex] == "setup" && args[commandIndex+1] == "approval" {
		return args[commandIndex+2:], true
	}
	return nil, false
}

func setupPluginArgsFromRaw(args []string) ([]string, bool) {
	commandIndex := firstCommandIndex(args)
	if commandIndex+1 < len(args) && args[commandIndex] == "setup" && args[commandIndex+1] == "plugin" {
		return args[commandIndex+2:], true
	}
	return nil, false
}

func firstCommandIndex(args []string) int {
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--config", "--profile":
			index++
			if index >= len(args) {
				return len(args)
			}
		default:
			return index
		}
	}
	return len(args)
}

func writeHelp(stdout io.Writer) int {
	if _, err := fmt.Fprint(stdout, helpText); err != nil {
		return 1
	}
	return 0
}

func doctorNextSteps(output doctorOutput) []string {
	steps := make([]string, 0)
	if output.Config.Kind == "none" {
		steps = append(steps, "No config file was found; Outlook Agent will use the safe fake transport until you pass --config <path> or OUTLOOK_AGENT_CONFIG.")
		steps = append(steps, "Run outlook-agent setup opencode --print after choosing the binary and config path for your agent client.")
	}
	if output.Config.Kind != "none" && output.Config.Error != "" {
		steps = append(steps, "Create the missing config file or update the configured config path: "+output.Config.Path)
	}
	if !output.SecretStore.Available {
		if output.SecretStore.Recommendation != "" {
			steps = append(steps, output.SecretStore.Recommendation)
		} else if output.SecretStore.Warning != "" {
			steps = append(steps, output.SecretStore.Warning)
		} else {
			steps = append(steps, "Configure an approved secret-store backend before live profiles.")
		}
	} else if output.SecretStore.Recommendation != "" {
		steps = append(steps, output.SecretStore.Recommendation)
	}
	if output.Approval.HostIntegrationRequired && !output.Approval.SecretConfigured {
		steps = append(steps, "Configure OUTLOOK_AGENT_APPROVAL_SECRET in the trusted host/operator environment before high-risk live actions.")
		steps = append(steps, "Run outlook-agent setup approval plan --client <opencode|codex|claude-code> --scope <project|user> after choosing the trusted host secret location.")
	}
	if output.MCPStdio {
		steps = append(steps, "OpenCode can run Outlook Agent through a local MCP entry that executes outlook-agent --config <path> mcp.")
	}
	return steps
}

type setupOpencodeArgs struct {
	Command    string
	Binary     string
	ConfigPath string
	Yes        bool
	Force      bool
	Backup     bool
}

type setupSkillsArgs struct {
	Command         string
	Client          setupcore.Client
	Scope           setupcore.Scope
	ProjectDir      string
	HomeDir         string
	Yes             bool
	Backup          bool
	AllowDuplicates bool
	JSON            bool
}

type setupAgentArgs struct {
	Command            string
	Client             setupcore.Client
	Scope              setupcore.Scope
	ProjectDir         string
	HomeDir            string
	ConfigPath         string
	Binary             string
	UseApprovalWrapper bool
	Yes                bool
	Backup             bool
	AllowDuplicates    bool
	JSON               bool
}

type setupApprovalArgs struct {
	Command    string
	Client     setupcore.Client
	Scope      setupcore.Scope
	ProjectDir string
	HomeDir    string
	ConfigPath string
	SecretFile string
	Binary     string
	Yes        bool
	JSON       bool
}

type setupPluginArgs struct {
	Command    string
	Client     setupcore.Client
	Output     string
	ConfigPath string
	Binary     string
	Local      bool
	Force      bool
}

func runSetupSkills(args []string, stdout io.Writer, stderr io.Writer) int {
	settings, err := parseSetupSkillsArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	plan, err := setupcore.BuildSkillsPlan(skillassets.FS, setupcore.SkillsOptions{
		Client:     settings.Client,
		Scope:      settings.Scope,
		ProjectDir: settings.ProjectDir,
		HomeDir:    settings.HomeDir,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch settings.Command {
	case "plan":
		return writeJSON(stdout, plan)
	case "diff":
		if _, err := fmt.Fprint(stdout, setupcore.DiffSkillsPlan(plan)); err != nil {
			return 1
		}
		return 0
	case "apply":
		if err := setupcore.ApplySkillsPlan(plan, setupcore.ApplyOptions{
			Yes:             settings.Yes,
			Backup:          settings.Backup,
			AllowDuplicates: settings.AllowDuplicates,
		}); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		response := map[string]any{
			"ok":         true,
			"command":    "setup skills apply",
			"operations": plan.Operations,
			"duplicates": plan.Duplicates,
		}
		if len(plan.Warnings) > 0 {
			response["warnings"] = plan.Warnings
		}
		return writeJSON(stdout, response)
	default:
		fmt.Fprintf(stderr, "unknown setup skills command: %s\n", settings.Command)
		return 1
	}
}

func runSetupAgent(args []string, options Options, stdout io.Writer, stderr io.Writer) int {
	settings, err := parseSetupAgentArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if settings.ConfigPath == "" {
		settings.ConfigPath = options.ConfigPath
	}
	plan, err := setupcore.BuildAgentPlan(skillassets.FS, setupcore.AgentOptions{
		Client:             settings.Client,
		Scope:              settings.Scope,
		ProjectDir:         settings.ProjectDir,
		HomeDir:            settings.HomeDir,
		ConfigPath:         settings.ConfigPath,
		Binary:             settings.Binary,
		UseApprovalWrapper: settings.UseApprovalWrapper,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch settings.Command {
	case "plan":
		return writeJSON(stdout, plan)
	case "diff":
		if _, err := fmt.Fprint(stdout, setupcore.DiffAgentPlan(plan)); err != nil {
			return 1
		}
		return 0
	case "apply":
		if err := setupcore.ApplyAgentPlan(plan, setupcore.ApplyOptions{
			Yes:             settings.Yes,
			Backup:          settings.Backup,
			AllowDuplicates: settings.AllowDuplicates,
		}); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return writeJSON(stdout, map[string]any{
			"ok":      true,
			"command": "setup agent apply",
			"mcp":     plan.MCP,
			"skills":  plan.Skills,
		})
	default:
		fmt.Fprintf(stderr, "unknown setup agent command: %s\n", settings.Command)
		return 1
	}
}

func runSetupApproval(args []string, options Options, stdout io.Writer, stderr io.Writer) int {
	settings, err := parseSetupApprovalArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if settings.ConfigPath == "" {
		settings.ConfigPath = options.ConfigPath
	}
	plan, err := setupcore.BuildApprovalPlan(setupcore.ApprovalOptions{
		Client:     settings.Client,
		Scope:      settings.Scope,
		ProjectDir: settings.ProjectDir,
		HomeDir:    settings.HomeDir,
		ConfigPath: settings.ConfigPath,
		Binary:     settings.Binary,
		SecretFile: settings.SecretFile,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch settings.Command {
	case "plan":
		return writeJSON(stdout, plan)
	case "diff":
		if _, err := fmt.Fprint(stdout, setupcore.DiffApprovalPlan(plan)); err != nil {
			return 1
		}
		return 0
	case "apply":
		if err := setupcore.ApplyApprovalPlan(plan, setupcore.ApplyOptions{Yes: settings.Yes}); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return writeJSON(stdout, map[string]any{
			"ok":          true,
			"command":     "setup approval apply",
			"secret_file": plan.SecretFile,
			"wrapper":     plan.Wrapper,
		})
	default:
		fmt.Fprintf(stderr, "unknown setup approval command: %s\n", settings.Command)
		return 1
	}
}

func runSetupPlugin(args []string, options Options, stdout io.Writer, stderr io.Writer) int {
	settings, err := parseSetupPluginArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if settings.ConfigPath == "" {
		settings.ConfigPath = options.ConfigPath
	}
	switch settings.Command {
	case "export":
		plan, err := setupcore.BuildPluginExportPlan(skillassets.FS, setupcore.PluginOptions{
			Client:     settings.Client,
			Output:     settings.Output,
			Binary:     settings.Binary,
			ConfigPath: settings.ConfigPath,
			Local:      settings.Local,
			Force:      settings.Force,
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := setupcore.ApplyPluginExportPlan(plan); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return writeJSON(stdout, map[string]any{
			"ok":         true,
			"command":    "setup plugin export",
			"client":     plan.Client,
			"output":     plan.Output,
			"operations": plan.Operations,
		})
	default:
		fmt.Fprintf(stderr, "unknown setup plugin command: %s\n", settings.Command)
		return 1
	}
}

func runSetupOpencode(args []string, options Options, stdout io.Writer, stderr io.Writer) int {
	settings, err := parseSetupOpencodeArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if settings.ConfigPath == "" {
		settings.ConfigPath = options.ConfigPath
	}
	if settings.Command == "print" {
		return writeSetupOpencodePrint(stdout, settings)
	}
	plan, err := setupopencode.BuildPlan(setupopencode.Options{
		RepoRoot:    ".",
		Binary:      settings.Binary,
		ConfigPath:  settings.ConfigPath,
		TargetScope: setupopencode.ScopeProject,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch settings.Command {
	case "plan":
		return writeJSON(stdout, plan)
	case "diff":
		if _, err := fmt.Fprint(stdout, setupopencode.Diff(plan)); err != nil {
			return 1
		}
		return 0
	case "apply":
		if err := setupopencode.Apply(plan, setupopencode.ApplyOptions{
			Yes:    settings.Yes,
			Force:  settings.Force,
			Backup: settings.Backup,
		}); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return writeJSON(stdout, map[string]any{
			"ok":      true,
			"command": "setup opencode apply",
			"targets": plan.Targets,
		})
	default:
		fmt.Fprintf(stderr, "unknown setup opencode command: %s\n", settings.Command)
		return 1
	}
}

func writeSetupOpencodePrint(stdout io.Writer, settings setupOpencodeArgs) int {
	command := []string{settings.Binary}
	if settings.ConfigPath != "" {
		command = append(command, "--config", settings.ConfigPath)
	}
	command = append(command, "mcp")
	return writeJSON(stdout, setupOpencodeOutput{
		Schema: "https://opencode.ai/config.json",
		MCP: map[string]setupOpencodeMCPServer{
			"outlook-agent": {
				Type:    "local",
				Command: command,
				Enabled: true,
			},
		},
	})
}

func parseSetupOpencodeArgs(args []string) (setupOpencodeArgs, error) {
	settings := setupOpencodeArgs{Command: "print", Binary: "outlook-agent"}
	if len(args) > 0 {
		switch args[0] {
		case "print", "plan", "diff", "apply":
			settings.Command = args[0]
			args = args[1:]
		case "--print":
			settings.Command = "print"
			args = args[1:]
		}
	}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--print":
			settings.Command = "print"
		case "--binary":
			index++
			if index >= len(args) {
				return setupOpencodeArgs{}, fmt.Errorf("--binary requires a value")
			}
			settings.Binary = args[index]
		case "--config":
			index++
			if index >= len(args) {
				return setupOpencodeArgs{}, fmt.Errorf("--config requires a value")
			}
			settings.ConfigPath = args[index]
		case "--yes":
			settings.Yes = true
		case "--force":
			settings.Force = true
		case "--backup":
			settings.Backup = true
		default:
			return setupOpencodeArgs{}, fmt.Errorf("unknown setup opencode argument: %s", args[index])
		}
	}
	if settings.Command != "apply" && (settings.Yes || settings.Force || settings.Backup) {
		return setupOpencodeArgs{}, fmt.Errorf("--yes, --force, and --backup are only valid for setup opencode apply")
	}
	if settings.Force && settings.Backup {
		return setupOpencodeArgs{}, fmt.Errorf("--force and --backup are mutually exclusive")
	}
	return settings, nil
}

func parseSetupSkillsArgs(args []string) (setupSkillsArgs, error) {
	settings := setupSkillsArgs{
		Command: "plan",
		Client:  setupcore.ClientOpenCode,
		Scope:   setupcore.ScopeProject,
	}
	if len(args) > 0 {
		switch args[0] {
		case "plan", "diff", "apply":
			settings.Command = args[0]
			args = args[1:]
		}
	}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--client":
			index++
			if index >= len(args) {
				return setupSkillsArgs{}, fmt.Errorf("--client requires a value")
			}
			settings.Client = setupcore.Client(args[index])
		case "--scope":
			index++
			if index >= len(args) {
				return setupSkillsArgs{}, fmt.Errorf("--scope requires a value")
			}
			settings.Scope = setupcore.Scope(args[index])
		case "--project-dir":
			index++
			if index >= len(args) {
				return setupSkillsArgs{}, fmt.Errorf("--project-dir requires a value")
			}
			settings.ProjectDir = args[index]
		case "--home-dir":
			index++
			if index >= len(args) {
				return setupSkillsArgs{}, fmt.Errorf("--home-dir requires a value")
			}
			settings.HomeDir = args[index]
		case "--yes":
			settings.Yes = true
		case "--backup":
			settings.Backup = true
		case "--allow-duplicates":
			settings.AllowDuplicates = true
		case "--json":
			settings.JSON = true
		default:
			return setupSkillsArgs{}, fmt.Errorf("unknown setup skills argument: %s", args[index])
		}
	}
	if settings.Command != "apply" && (settings.Yes || settings.Backup || settings.AllowDuplicates) {
		return setupSkillsArgs{}, fmt.Errorf("--yes, --backup, and --allow-duplicates are only valid for setup skills apply")
	}
	return settings, nil
}

func parseSetupAgentArgs(args []string) (setupAgentArgs, error) {
	settings := setupAgentArgs{
		Command: "plan",
		Client:  setupcore.ClientOpenCode,
		Scope:   setupcore.ScopeProject,
		Binary:  "outlook-agent",
	}
	if len(args) > 0 {
		switch args[0] {
		case "plan", "diff", "apply":
			settings.Command = args[0]
			args = args[1:]
		}
	}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--client":
			index++
			if index >= len(args) {
				return setupAgentArgs{}, fmt.Errorf("--client requires a value")
			}
			settings.Client = setupcore.Client(args[index])
		case "--scope":
			index++
			if index >= len(args) {
				return setupAgentArgs{}, fmt.Errorf("--scope requires a value")
			}
			settings.Scope = setupcore.Scope(args[index])
		case "--project-dir":
			index++
			if index >= len(args) {
				return setupAgentArgs{}, fmt.Errorf("--project-dir requires a value")
			}
			settings.ProjectDir = args[index]
		case "--home-dir":
			index++
			if index >= len(args) {
				return setupAgentArgs{}, fmt.Errorf("--home-dir requires a value")
			}
			settings.HomeDir = args[index]
		case "--config":
			index++
			if index >= len(args) {
				return setupAgentArgs{}, fmt.Errorf("--config requires a value")
			}
			settings.ConfigPath = args[index]
		case "--binary":
			index++
			if index >= len(args) {
				return setupAgentArgs{}, fmt.Errorf("--binary requires a value")
			}
			settings.Binary = args[index]
		case "--use-approval-wrapper":
			settings.UseApprovalWrapper = true
		case "--yes":
			settings.Yes = true
		case "--backup":
			settings.Backup = true
		case "--allow-duplicates":
			settings.AllowDuplicates = true
		case "--json":
			settings.JSON = true
		default:
			return setupAgentArgs{}, fmt.Errorf("unknown setup agent argument: %s", args[index])
		}
	}
	if settings.Command != "apply" && (settings.Yes || settings.Backup || settings.AllowDuplicates) {
		return setupAgentArgs{}, fmt.Errorf("--yes, --backup, and --allow-duplicates are only valid for setup agent apply")
	}
	return settings, nil
}

func parseSetupApprovalArgs(args []string) (setupApprovalArgs, error) {
	settings := setupApprovalArgs{
		Command: "plan",
		Client:  setupcore.ClientOpenCode,
		Scope:   setupcore.ScopeProject,
		Binary:  "outlook-agent",
	}
	if len(args) > 0 {
		switch args[0] {
		case "plan", "diff", "apply":
			settings.Command = args[0]
			args = args[1:]
		}
	}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--client":
			index++
			if index >= len(args) {
				return setupApprovalArgs{}, fmt.Errorf("--client requires a value")
			}
			settings.Client = setupcore.Client(args[index])
		case "--scope":
			index++
			if index >= len(args) {
				return setupApprovalArgs{}, fmt.Errorf("--scope requires a value")
			}
			settings.Scope = setupcore.Scope(args[index])
		case "--project-dir":
			index++
			if index >= len(args) {
				return setupApprovalArgs{}, fmt.Errorf("--project-dir requires a value")
			}
			settings.ProjectDir = args[index]
		case "--home-dir":
			index++
			if index >= len(args) {
				return setupApprovalArgs{}, fmt.Errorf("--home-dir requires a value")
			}
			settings.HomeDir = args[index]
		case "--config":
			index++
			if index >= len(args) {
				return setupApprovalArgs{}, fmt.Errorf("--config requires a value")
			}
			settings.ConfigPath = args[index]
		case "--secret-file":
			index++
			if index >= len(args) {
				return setupApprovalArgs{}, fmt.Errorf("--secret-file requires a value")
			}
			settings.SecretFile = args[index]
		case "--binary":
			index++
			if index >= len(args) {
				return setupApprovalArgs{}, fmt.Errorf("--binary requires a value")
			}
			settings.Binary = args[index]
		case "--yes":
			settings.Yes = true
		case "--json":
			settings.JSON = true
		default:
			return setupApprovalArgs{}, fmt.Errorf("unknown setup approval argument: %s", args[index])
		}
	}
	if settings.Command != "apply" && settings.Yes {
		return setupApprovalArgs{}, fmt.Errorf("--yes is only valid for setup approval apply")
	}
	return settings, nil
}

func parseSetupPluginArgs(args []string) (setupPluginArgs, error) {
	settings := setupPluginArgs{
		Command: "export",
		Client:  setupcore.ClientCodex,
		Binary:  "outlook-agent",
	}
	if len(args) > 0 {
		switch args[0] {
		case "export":
			settings.Command = args[0]
			args = args[1:]
		}
	}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--client":
			index++
			if index >= len(args) {
				return setupPluginArgs{}, fmt.Errorf("--client requires a value")
			}
			settings.Client = setupcore.Client(args[index])
		case "--output":
			index++
			if index >= len(args) {
				return setupPluginArgs{}, fmt.Errorf("--output requires a value")
			}
			settings.Output = args[index]
		case "--config":
			index++
			if index >= len(args) {
				return setupPluginArgs{}, fmt.Errorf("--config requires a value")
			}
			settings.ConfigPath = args[index]
		case "--binary":
			index++
			if index >= len(args) {
				return setupPluginArgs{}, fmt.Errorf("--binary requires a value")
			}
			settings.Binary = args[index]
		case "--local":
			settings.Local = true
		case "--force":
			settings.Force = true
		default:
			return setupPluginArgs{}, fmt.Errorf("unknown setup plugin argument: %s", args[index])
		}
	}
	return settings, nil
}

func parseOptionsBeforeCommand(args []string) (Options, []string, error) {
	var options Options
	commandIndex := firstCommandIndex(args)
	for index := 0; index < commandIndex; index++ {
		switch args[index] {
		case "--config":
			index++
			if index >= commandIndex {
				return Options{}, nil, fmt.Errorf("--config requires a value")
			}
			options.ConfigPath = args[index]
		case "--profile":
			index++
			if index >= commandIndex {
				return Options{}, nil, fmt.Errorf("--profile requires a value")
			}
			options.Profile = args[index]
		default:
			return Options{}, nil, fmt.Errorf("unknown command: %s", args[index])
		}
	}
	return options, args[commandIndex:], nil
}

func writeJSON(stdout io.Writer, payload any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return 1
	}
	return 0
}

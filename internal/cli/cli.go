package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/buildinfo"
	"github.com/johnkil/outlook-agent/internal/capability"
	"github.com/johnkil/outlook-agent/internal/config"
	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/ews"
	"github.com/johnkil/outlook-agent/internal/transport/fake"
	"github.com/johnkil/outlook-agent/internal/transport/graph"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
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
	Kind      string `json:"kind"`
	Available bool   `json:"available"`
}

type doctorOutput struct {
	OK          bool                    `json:"ok"`
	Command     string                  `json:"command"`
	Version     string                  `json:"version"`
	Profile     string                  `json:"profile,omitempty"`
	Config      doctorConfigOutput      `json:"config"`
	SecretStore doctorSecretStoreOutput `json:"secret_store"`
	MCPStdio    bool                    `json:"mcp_stdio"`
	Transports  []string                `json:"transports"`
	NextSteps   []string                `json:"next_steps,omitempty"`
	Error       string                  `json:"error,omitempty"`
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
	secretStore := doctorSecretStore(profile, loaded)
	output := doctorOutput{
		OK:      err == nil,
		Command: "doctor",
		Version: buildinfo.Version,
		Profile: profile,
		Config: doctorConfigOutput{
			Found: source.Found,
			Kind:  source.Kind,
			Path:  source.Path,
		},
		SecretStore: secretStore,
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

func doctorSecretStore(profile string, loaded config.Config) doctorSecretStoreOutput {
	configuredProfile, ok := loaded.Profiles[profile]
	if ok && strings.HasPrefix(configuredProfile.SecretRef, "file:") {
		return doctorSecretStoreOutput{Kind: "file", Available: true}
	}
	return doctorSecretStoreOutput{Kind: "keychain", Available: runtime.GOOS == "darwin"}
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
  outlook-agent auth check --config <path> [--profile <name>]
  outlook-agent auth graph-device-code --config <path> [--profile <name>]
  outlook-agent policy explain [--action <name>]
  outlook-agent setup opencode --print [--binary <path>] [--config <path>]
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
		steps = append(steps, "The macOS Keychain secret store is unavailable on this platform; configure an approved secret-store backend before live profiles.")
	}
	if output.MCPStdio {
		steps = append(steps, "OpenCode can run Outlook Agent through a local MCP entry that executes outlook-agent --config <path> mcp.")
	}
	return steps
}

type setupOpencodeArgs struct {
	Binary     string
	ConfigPath string
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
	settings := setupOpencodeArgs{Binary: "outlook-agent"}
	seenPrint := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--print":
			seenPrint = true
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
		default:
			return setupOpencodeArgs{}, fmt.Errorf("unknown setup opencode argument: %s", args[index])
		}
	}
	if !seenPrint {
		return setupOpencodeArgs{}, fmt.Errorf("setup opencode requires --print")
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

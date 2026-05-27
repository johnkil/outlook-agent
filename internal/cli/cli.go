package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
)

type Options struct {
	ConfigPath string
	Profile    string
}

type Runtime struct {
	BuildTransport func(context.Context, Options) (transport.Transport, string, error)
	RunMCP         func(context.Context, Options) error
}

type owaActionDiscoverer interface {
	DiscoverServiceActionsFromURLWithOptions(ctx context.Context, source string, options owa.DiscoveryOptions) ([]string, error)
}

type owaActionDiscoveryDiagnoser interface {
	DiscoverServiceActionsFromURLDiagnostics(ctx context.Context, source string, options owa.DiscoveryOptions) (owa.DiscoveryDiagnostics, error)
}

// Run executes the CLI command and returns the process exit code.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	return RunWithRuntime(args, stdout, stderr, Runtime{})
}

func RunWithRuntime(args []string, stdout io.Writer, stderr io.Writer, runtime Runtime) int {
	options, commandArgs, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(commandArgs) == 0 {
		fmt.Fprintln(stderr, "missing command")
		return 1
	}

	switch commandArgs[0] {
	case "doctor":
		return writeJSON(stdout, map[string]any{
			"ok":         true,
			"command":    "doctor",
			"mcp_stdio":  true,
			"transports": []string{"fake", "graph", "ews", "owa"},
		})
	case "policy":
		if len(commandArgs) == 2 && commandArgs[1] == "explain" {
			return writeJSON(stdout, map[string]any{
				"ok":             true,
				"command":        "policy explain",
				"safety_classes": policy.SafetyClassNames(),
			})
		}
	case "owa":
		if len(commandArgs) >= 2 && commandArgs[1] == "discover-actions" {
			return runOWADiscoverActions(commandArgs[2:], options, runtime, stdout, stderr)
		}
	case "auth":
		if len(commandArgs) == 2 && commandArgs[1] == "check" {
			return runAuthCheck(stdout, options, runtime)
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

type discoverActionSources struct {
	Files                 []string
	URLs                  []string
	IncludeLinkedScripts  bool
	FollowNavigationHints bool
	Diagnostics           bool
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

func writeJSON(stdout io.Writer, payload any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return 1
	}
	return 0
}

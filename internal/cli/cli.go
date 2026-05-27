package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

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
			return runOWADiscoverActions(commandArgs[2:], stdout, stderr)
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

func runOWADiscoverActions(args []string, stdout io.Writer, stderr io.Writer) int {
	path, err := parseDiscoverActionsArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "read discovery file: %v\n", err)
		return 1
	}
	report := owa.CompareDiscoveredServiceActions(owa.DiscoverServiceActions(string(data)))
	return writeJSON(stdout, report)
}

func parseDiscoverActionsArgs(args []string) (string, error) {
	var path string
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--file":
			index++
			if index >= len(args) {
				return "", fmt.Errorf("--file requires a value")
			}
			path = args[index]
		default:
			return "", fmt.Errorf("unknown discover-actions argument: %s", args[index])
		}
	}
	if path == "" {
		return "", fmt.Errorf("owa discover-actions requires --file")
	}
	return path, nil
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

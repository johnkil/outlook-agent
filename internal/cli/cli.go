package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/johnkil/outlook-agent/internal/policy"
)

type Runtime struct {
	RunMCP func(context.Context) error
}

// Run executes the CLI command and returns the process exit code.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	return RunWithRuntime(args, stdout, stderr, Runtime{})
}

func RunWithRuntime(args []string, stdout io.Writer, stderr io.Writer, runtime Runtime) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "missing command")
		return 1
	}

	switch args[0] {
	case "doctor":
		return writeJSON(stdout, map[string]any{
			"ok":         true,
			"command":    "doctor",
			"mcp_stdio":  true,
			"transports": []string{"fake", "graph", "ews", "owa"},
		})
	case "policy":
		if len(args) == 2 && args[1] == "explain" {
			return writeJSON(stdout, map[string]any{
				"ok":             true,
				"command":        "policy explain",
				"safety_classes": policy.SafetyClassNames(),
			})
		}
	case "auth":
		if len(args) == 2 && args[1] == "check" {
			return writeJSON(stdout, map[string]any{
				"ok":      false,
				"command": "auth check",
				"error":   "transport profile is not configured",
			})
		}
	case "mcp":
		if runtime.RunMCP == nil {
			fmt.Fprintln(stderr, "mcp runner is not configured")
			return 4
		}
		if err := runtime.RunMCP(context.Background()); err != nil {
			fmt.Fprintf(stderr, "mcp server failed: %v\n", err)
			return 4
		}
		return 0
	}

	fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
	return 1
}

func writeJSON(stdout io.Writer, payload any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return 1
	}
	return 0
}

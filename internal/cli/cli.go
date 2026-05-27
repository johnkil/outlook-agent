package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/johnkil/outlook-agent/internal/policy"
)

// Run executes the CLI command and returns the process exit code.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
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
		fmt.Fprintln(stderr, "mcp server is not implemented yet")
		return 4
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

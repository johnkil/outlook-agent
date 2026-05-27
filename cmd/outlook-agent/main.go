package main

import (
	"context"
	"os"

	"github.com/johnkil/outlook-agent/internal/app"
	"github.com/johnkil/outlook-agent/internal/cli"
	"github.com/johnkil/outlook-agent/internal/mcpserver"
	"github.com/johnkil/outlook-agent/internal/transport"
)

func main() {
	os.Exit(cli.RunWithRuntime(os.Args[1:], os.Stdout, os.Stderr, cli.Runtime{
		BuildTransport: func(_ context.Context, options cli.Options) (transport.Transport, string, error) {
			result, err := app.BuildTransportResult(app.Options{ConfigPath: options.ConfigPath, Profile: options.Profile})
			return result.Client, result.Profile, err
		},
		RunMCP: func(ctx context.Context, options cli.Options) error {
			result, err := app.BuildTransportResult(app.Options{ConfigPath: options.ConfigPath, Profile: options.Profile})
			if err != nil {
				return err
			}
			return mcpserver.RunStdioWithTransportProfile(ctx, result.Client, result.Profile)
		},
	}))
}

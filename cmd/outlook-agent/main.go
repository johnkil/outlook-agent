package main

import (
	"context"
	"os"

	"github.com/johnkil/outlook-agent/internal/app"
	"github.com/johnkil/outlook-agent/internal/cli"
	"github.com/johnkil/outlook-agent/internal/mcpserver"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/graph"
)

func main() {
	os.Exit(cli.RunWithRuntime(os.Args[1:], os.Stdout, os.Stderr, cli.Runtime{
		BuildTransport: func(_ context.Context, options cli.Options) (transport.Transport, string, error) {
			result, err := app.BuildTransportResult(app.Options{ConfigPath: options.ConfigPath, Profile: options.Profile})
			return result.Client, result.Profile, err
		},
		EnrollGraphDeviceCode: func(ctx context.Context, options cli.Options, onChallenge func(cli.GraphDeviceCodeChallenge)) (cli.GraphDeviceCodeResult, error) {
			result, err := app.EnrollGraphDeviceCode(ctx, app.Options{ConfigPath: options.ConfigPath, Profile: options.Profile}, func(challenge graph.DeviceCodeChallenge) {
				onChallenge(cli.GraphDeviceCodeChallenge{
					VerificationURI: challenge.VerificationURI,
					UserCode:        challenge.UserCode,
					Message:         challenge.Message,
					ExpiresIn:       challenge.ExpiresIn,
					Interval:        challenge.Interval,
				})
			})
			return cli.GraphDeviceCodeResult{
				Profile:   result.Profile,
				SecretRef: result.SecretRef,
				TokenType: result.TokenType,
				Scope:     result.Scope,
				ExpiresAt: result.ExpiresAt,
			}, err
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

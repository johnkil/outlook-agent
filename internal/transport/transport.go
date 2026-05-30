package transport

import (
	"context"

	"github.com/johnkil/outlook-agent/internal/action"
)

type AuthResult struct {
	OK        bool   `json:"ok"`
	Principal string `json:"principal,omitempty"`
	Error     string `json:"error,omitempty"`
}

type CapabilitySet struct {
	Actions []action.Definition `json:"actions"`
}

type ActionRequest struct {
	Name       string         `json:"name"`
	Payload    map[string]any `json:"payload,omitempty"`
	UnsafeMode bool           `json:"unsafe_mode,omitempty"`
}

type ActionResponse struct {
	OK    bool           `json:"ok"`
	Data  map[string]any `json:"data,omitempty"`
	Error string         `json:"error,omitempty"`
}

type DryRunSummary struct {
	Action               string        `json:"action"`
	Count                int           `json:"count"`
	Reversible           bool          `json:"reversible"`
	RequiresConfirmation bool          `json:"requires_confirmation"`
	SafetyClass          string        `json:"safety_class,omitempty"`
	Review               *ReviewPacket `json:"review,omitempty"`
	Warnings             []string      `json:"warnings,omitempty"`
	Error                string        `json:"error,omitempty"`
}

type Transport interface {
	Name() string
	Authenticate(ctx context.Context, profile string) AuthResult
	Capabilities(ctx context.Context) CapabilitySet
	Execute(ctx context.Context, request ActionRequest) ActionResponse
	DryRun(ctx context.Context, request ActionRequest) DryRunSummary
}

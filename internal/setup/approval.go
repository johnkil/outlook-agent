package setup

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ApprovalOptions struct {
	Client     Client
	Scope      Scope
	ProjectDir string
	HomeDir    string
	Binary     string
	ConfigPath string
	SecretFile string
}

type ApprovalPlan struct {
	Command    string          `json:"command"`
	Client     Client          `json:"client"`
	Scope      Scope           `json:"scope"`
	Binary     string          `json:"binary"`
	ConfigPath string          `json:"config_path,omitempty"`
	SecretFile string          `json:"secret_file"`
	Wrapper    ConfigOperation `json:"wrapper"`
	Warnings   []string        `json:"warnings,omitempty"`
}

func BuildApprovalPlan(options ApprovalOptions) (ApprovalPlan, error) {
	if options.Client == "" {
		options.Client = ClientOpenCode
	}
	if options.Client == ClientAll {
		return ApprovalPlan{}, errors.New("setup approval requires one client")
	}
	if options.Scope == "" {
		options.Scope = ScopeProject
	}
	if options.Binary == "" {
		options.Binary = "outlook-agent"
	}
	projectDir, err := resolveDir(options.ProjectDir, ".")
	if err != nil {
		return ApprovalPlan{}, fmt.Errorf("resolve project dir: %w", err)
	}
	homeDir := options.HomeDir
	if homeDir == "" {
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return ApprovalPlan{}, fmt.Errorf("resolve home dir: %w", err)
		}
	}
	homeDir, err = filepath.Abs(homeDir)
	if err != nil {
		return ApprovalPlan{}, fmt.Errorf("resolve home dir: %w", err)
	}

	secretPath, wrapperPath, rootPath, err := approvalPaths(options.Scope, projectDir, homeDir, options.SecretFile)
	if err != nil {
		return ApprovalPlan{}, err
	}
	if err := rejectChildPathSymlinks(rootPath, wrapperPath); err != nil {
		return ApprovalPlan{}, err
	}
	if err := rejectChildPathSymlinks(rootPath, secretPath); err != nil {
		return ApprovalPlan{}, err
	}

	content := approvalWrapperContent(options.Binary, options.ConfigPath, secretPath)
	currentContent, err := os.ReadFile(wrapperPath)
	if err != nil && !os.IsNotExist(err) {
		return ApprovalPlan{}, fmt.Errorf("read approval wrapper %s: %w", wrapperPath, err)
	}
	operation := ConfigOperation{
		Client:         options.Client,
		Kind:           OperationCreate,
		TargetPath:     wrapperPath,
		Reason:         "approval host wrapper does not exist",
		content:        content,
		currentContent: append([]byte(nil), currentContent...),
		rootPath:       rootPath,
	}
	if len(currentContent) > 0 {
		if bytes.Equal(currentContent, content) {
			operation.Kind = OperationSkip
			operation.Reason = "approval host wrapper already matches planned content"
		} else {
			operation.Kind = OperationUpdate
			operation.Reason = "approval host wrapper differs from planned content"
		}
	}

	return ApprovalPlan{
		Command:    "setup approval plan",
		Client:     options.Client,
		Scope:      options.Scope,
		Binary:     options.Binary,
		ConfigPath: options.ConfigPath,
		SecretFile: secretPath,
		Wrapper:    operation,
		Warnings:   approvalWarnings(options.Scope, options.ConfigPath),
	}, nil
}

func DiffApprovalPlan(plan ApprovalPlan) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("target %s (%s/approval-wrapper): %s\n", filepath.ToSlash(plan.Wrapper.TargetPath), plan.Client, plan.Wrapper.Kind))
	if plan.Wrapper.Kind != OperationSkip {
		builder.WriteString("--- current\n")
		writePlanContent(&builder, plan.Wrapper.currentContent)
		builder.WriteString("+++ planned\n")
		writePlanContent(&builder, plan.Wrapper.content)
		builder.WriteByte('\n')
	}
	builder.WriteString(fmt.Sprintf("secret %s: create if missing, preserve if present, require mode 0600\n", filepath.ToSlash(plan.SecretFile)))
	return builder.String()
}

func ApplyApprovalPlan(plan ApprovalPlan, options ApplyOptions) error {
	if !options.Yes {
		return errors.New("apply requires yes")
	}
	if plan.SecretFile == "" {
		return errors.New("approval plan is missing secret file")
	}
	if err := os.MkdirAll(filepath.Dir(plan.SecretFile), 0o700); err != nil {
		return fmt.Errorf("create approval secret dir %s: %w", filepath.Dir(plan.SecretFile), err)
	}
	if err := writeApprovalSecretIfMissing(plan.SecretFile); err != nil {
		return err
	}
	if plan.Wrapper.Kind != OperationSkip {
		if err := rejectChildPathSymlinks(plan.Wrapper.rootPath, plan.Wrapper.TargetPath); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(plan.Wrapper.TargetPath), 0o700); err != nil {
			return fmt.Errorf("create approval wrapper dir %s: %w", filepath.Dir(plan.Wrapper.TargetPath), err)
		}
		if err := atomicWriteFile(plan.Wrapper.TargetPath, plan.Wrapper.content, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func approvalPaths(scope Scope, projectDir string, homeDir string, secretFile string) (string, string, string, error) {
	var rootPath string
	var defaultSecret string
	var defaultWrapper string
	switch scope {
	case ScopeProject:
		rootPath = projectDir
		defaultSecret = filepath.Join(projectDir, ".local", "outlook-agent-approval-secret")
		defaultWrapper = filepath.Join(projectDir, ".local", "bin", "outlook-agent-host-mcp")
	case ScopeUser:
		rootPath = homeDir
		defaultSecret = filepath.Join(homeDir, ".config", "outlook-agent", "approval-secret")
		defaultWrapper = filepath.Join(homeDir, ".local", "bin", "outlook-agent-host-mcp")
	default:
		return "", "", "", fmt.Errorf("unsupported scope: %s", scope)
	}
	if secretFile == "" {
		secretFile = defaultSecret
	}
	secretPath, err := filepath.Abs(secretFile)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve approval secret file: %w", err)
	}
	if scope == ScopeProject {
		localRoot := filepath.Join(projectDir, ".local")
		rel, err := filepath.Rel(localRoot, secretPath)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
			return "", "", "", errors.New("project approval secret must live under .local/")
		}
	}
	return secretPath, defaultWrapper, rootPath, nil
}

func approvalWarnings(scope Scope, configPath string) []string {
	warnings := projectConfigWarnings(scope, configPath)
	if scope == ScopeProject {
		warnings = append(warnings, "project-scope approval material should live under .local/ and .local/ should be gitignored")
	}
	return warnings
}

func writeApprovalSecretIfMissing(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return fmt.Errorf("approval secret path points to a directory: %s", path)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("restrict approval secret permissions %s: %w", path, err)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("stat approval secret %s: %w", path, err)
	}
	secret, err := generateApprovalSecret()
	if err != nil {
		return err
	}
	return atomicWriteFile(path, []byte(secret+"\n"), 0o600)
}

func generateApprovalSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate approval secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func approvalWrapperContent(binary string, configPath string, secretPath string) []byte {
	args := []string{}
	if strings.TrimSpace(configPath) != "" {
		args = append(args, "--config", configPath)
	}
	args = append(args, "mcp")
	quotedArgs := make([]string, 0, len(args)+1)
	quotedArgs = append(quotedArgs, shellSingleQuote(binary))
	for _, arg := range args {
		quotedArgs = append(quotedArgs, shellSingleQuote(arg))
	}
	content := "#!/bin/sh\n" +
		"set -eu\n" +
		"secret_file=" + shellSingleQuote(secretPath) + "\n" +
		"if [ ! -r \"$secret_file\" ]; then\n" +
		"  echo \"approval secret file is not readable: $secret_file\" >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"OUTLOOK_AGENT_APPROVAL_MODE=\"${OUTLOOK_AGENT_APPROVAL_MODE:-required}\"\n" +
		"OUTLOOK_AGENT_APPROVAL_SECRET=\"$(LC_ALL=C tr -d '\\r\\n' < \"$secret_file\")\"\n" +
		"export OUTLOOK_AGENT_APPROVAL_MODE OUTLOOK_AGENT_APPROVAL_SECRET\n" +
		"exec " + strings.Join(quotedArgs, " ") + "\n"
	return []byte(content)
}

func shellSingleQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

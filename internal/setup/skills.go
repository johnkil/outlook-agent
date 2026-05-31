package setup

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

type Client string

const (
	ClientOpenCode   Client = "opencode"
	ClientCodex      Client = "codex"
	ClientClaudeCode Client = "claude-code"
)

type Scope string

const (
	ScopeProject Scope = "project"
	ScopeUser    Scope = "user"
)

type Skill struct {
	Name     string
	Content  []byte
	Metadata SkillMetadata
}

type SkillMetadata struct {
	Name          string
	Description   string
	License       string
	Compatibility string
	Clients       []Client
	MCPServer     string
	ToolPrefix    string
}

func LoadCanonicalSkills(fsys fs.FS) ([]Skill, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read skills: %w", err)
	}
	skills := make([]Skill, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		content, err := fs.ReadFile(fsys, path.Join(entry.Name(), "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("read skill %s: %w", entry.Name(), err)
		}
		metadata, err := parseSkillMetadata(content)
		if err != nil {
			return nil, fmt.Errorf("parse skill %s metadata: %w", entry.Name(), err)
		}
		if metadata.Name != entry.Name() {
			return nil, fmt.Errorf("skill %s metadata name mismatch: %s", entry.Name(), metadata.Name)
		}
		skills = append(skills, Skill{
			Name:     entry.Name(),
			Content:  append([]byte(nil), content...),
			Metadata: metadata,
		})
	}
	sort.Slice(skills, func(left int, right int) bool {
		return skills[left].Name < skills[right].Name
	})
	return skills, nil
}

func parseSkillMetadata(content []byte) (SkillMetadata, error) {
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return SkillMetadata{}, fmt.Errorf("missing YAML frontmatter")
	}
	rest := content[len("---\n"):]
	end := bytes.Index(rest, []byte("\n---\n"))
	if end < 0 {
		return SkillMetadata{}, fmt.Errorf("unterminated YAML frontmatter")
	}

	var metadata SkillMetadata
	var section string
	var subsection string
	lines := strings.Split(string(rest[:end]), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "name:"):
			metadata.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			section, subsection = "", ""
		case strings.HasPrefix(trimmed, "description:"):
			metadata.Description = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			section, subsection = "", ""
		case strings.HasPrefix(trimmed, "license:"):
			metadata.License = strings.TrimSpace(strings.TrimPrefix(trimmed, "license:"))
			section, subsection = "", ""
		case trimmed == "compatibility:":
			section, subsection = "compatibility", ""
		case strings.HasPrefix(trimmed, "compatibility:"):
			metadata.Compatibility = strings.TrimSpace(strings.TrimPrefix(trimmed, "compatibility:"))
			section, subsection = "", ""
		case section == "compatibility" && trimmed == "clients:":
			subsection = "clients"
		case section == "compatibility" && subsection == "clients" && strings.HasPrefix(trimmed, "- "):
			metadata.Clients = append(metadata.Clients, Client(strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))))
		case trimmed == "metadata:":
			section, subsection = "metadata", ""
		case section == "metadata" && strings.HasPrefix(trimmed, "outlook_agent_mcp_server:"):
			metadata.MCPServer = strings.TrimSpace(strings.TrimPrefix(trimmed, "outlook_agent_mcp_server:"))
		case section == "metadata" && strings.HasPrefix(trimmed, "outlook_agent_tool_prefix:"):
			metadata.ToolPrefix = strings.TrimSpace(strings.TrimPrefix(trimmed, "outlook_agent_tool_prefix:"))
		case section == "metadata" && strings.HasPrefix(trimmed, "outlook_agent_clients:"):
			metadata.Clients = parseSkillClients(strings.TrimSpace(strings.TrimPrefix(trimmed, "outlook_agent_clients:")))
		case section == "metadata" && strings.HasPrefix(trimmed, "mcp_server:"):
			metadata.MCPServer = strings.TrimSpace(strings.TrimPrefix(trimmed, "mcp_server:"))
		case section == "metadata" && strings.HasPrefix(trimmed, "tool_prefix:"):
			metadata.ToolPrefix = strings.TrimSpace(strings.TrimPrefix(trimmed, "tool_prefix:"))
		default:
			return SkillMetadata{}, fmt.Errorf("unsupported metadata line: %s", trimmed)
		}
	}
	if metadata.Name == "" {
		return SkillMetadata{}, fmt.Errorf("missing name")
	}
	if metadata.Description == "" {
		return SkillMetadata{}, fmt.Errorf("missing description")
	}
	if metadata.License == "" {
		return SkillMetadata{}, fmt.Errorf("missing license")
	}
	if len(metadata.Clients) == 0 {
		return SkillMetadata{}, fmt.Errorf("missing compatible clients")
	}
	if metadata.MCPServer == "" {
		return SkillMetadata{}, fmt.Errorf("missing mcp_server")
	}
	if metadata.ToolPrefix == "" {
		return SkillMetadata{}, fmt.Errorf("missing tool_prefix")
	}
	return metadata, nil
}

func parseSkillClients(value string) []Client {
	clients := make([]Client, 0)
	for _, part := range strings.Split(value, ",") {
		client := strings.TrimSpace(part)
		if client != "" {
			clients = append(clients, Client(client))
		}
	}
	return clients
}

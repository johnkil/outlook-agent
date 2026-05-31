package setup

import (
	"strings"
	"testing"

	skillassets "github.com/johnkil/outlook-agent/skills"
)

func TestLoadCanonicalSkillsRequiresPortableMetadata(t *testing.T) {
	skills, err := LoadCanonicalSkills(skillassets.FS)
	if err != nil {
		t.Fatalf("LoadCanonicalSkills returned error: %v", err)
	}

	expectedNames := []string{
		"outlook-calendar",
		"outlook-calendar-daily-brief",
		"outlook-calendar-free-up-time",
		"outlook-calendar-meeting-prep",
		"outlook-mail",
		"outlook-mail-inbox-triage",
		"outlook-mail-reply-drafting",
		"outlook-mail-subscription-cleanup",
		"outlook-mail-task-extraction",
	}
	if len(skills) != len(expectedNames) {
		t.Fatalf("expected %d canonical skills, got %d: %#v", len(expectedNames), len(skills), skills)
	}
	for index, expected := range expectedNames {
		if skills[index].Name != expected {
			t.Fatalf("expected skill %d to be %q, got %q", index, expected, skills[index].Name)
		}
	}

	for _, skill := range skills {
		if skill.Metadata.Name != skill.Name {
			t.Fatalf("expected metadata name for %s to match directory name, got %q", skill.Name, skill.Metadata.Name)
		}
		if skill.Metadata.Description == "" {
			t.Fatalf("expected %s to have a description", skill.Name)
		}
		if skill.Metadata.License != "Apache-2.0" {
			t.Fatalf("expected %s to use Apache-2.0 license metadata, got %q", skill.Name, skill.Metadata.License)
		}
		assertClientCompatible(t, skill, ClientOpenCode)
		assertClientCompatible(t, skill, ClientCodex)
		assertClientCompatible(t, skill, ClientClaudeCode)
		if skill.Metadata.MCPServer != "outlook-agent" {
			t.Fatalf("expected %s mcp server metadata to be outlook-agent, got %q", skill.Name, skill.Metadata.MCPServer)
		}
		if skill.Metadata.ToolPrefix != "outlook." {
			t.Fatalf("expected %s tool prefix metadata to be outlook., got %q", skill.Name, skill.Metadata.ToolPrefix)
		}
	}
}

func TestCanonicalSkillsDoNotContainPrivateMarkers(t *testing.T) {
	skills, err := LoadCanonicalSkills(skillassets.FS)
	if err != nil {
		t.Fatalf("LoadCanonicalSkills returned error: %v", err)
	}

	for _, skill := range skills {
		lower := strings.ToLower(string(skill.Content))
		for _, forbidden := range []string{
			"access_token",
			"refresh_token",
			"x-owa-canary",
			"cookie",
			"password",
			"owa.alfabank",
			"alfabank",
			"alfaintra",
			"moscow\\",
		} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("skill %s contains forbidden marker %q", skill.Name, forbidden)
			}
		}
	}
}

func assertClientCompatible(t *testing.T, skill Skill, client Client) {
	t.Helper()
	for _, candidate := range skill.Metadata.Clients {
		if candidate == client {
			return
		}
	}
	t.Fatalf("expected %s to be compatible with %s, got %#v", skill.Name, client, skill.Metadata.Clients)
}

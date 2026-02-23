package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverModels(t *testing.T) {
	raw := []byte(`{
		"provider": {
			"anthropic": {
				"models": {
					"claude-sonnet-4-20250514": {
						"name": "Claude Sonnet 4"
					}
				}
			},
			"google": {
				"models": {
					"gemini-2.5-flash": {
						"name": "Gemini 2.5 Flash"
					},
					"gemini-2.5-pro": {}
				}
			}
		}
	}`)

	models := discoverModels(raw)
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}

	// Should be sorted by ID
	if models[0].ID != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("unexpected first model: %s", models[0].ID)
	}
	if models[0].Name != "Claude Sonnet 4" {
		t.Errorf("unexpected name: %s", models[0].Name)
	}

	// Model without name should fall back to ID
	found := false
	for _, m := range models {
		if m.ID == "google/gemini-2.5-pro" && m.Name == "gemini-2.5-pro" {
			found = true
		}
	}
	if !found {
		t.Error("expected gemini-2.5-pro to have name fallback")
	}
}

func TestDiscoverTargets_BuiltIn(t *testing.T) {
	raw := []byte(`{}`)
	targets := discoverTargets("/nonexistent", raw)

	// Should have 4 built-in agents
	if len(targets) != 4 {
		t.Fatalf("expected 4 built-in targets, got %d", len(targets))
	}

	names := make(map[string]bool)
	for _, tgt := range targets {
		names[tgt.Name] = true
	}

	for _, expected := range []string{"build", "plan", "general", "explore"} {
		if !names[expected] {
			t.Errorf("missing built-in agent: %s", expected)
		}
	}
}

func TestDiscoverTargets_WithConfiguredAgent(t *testing.T) {
	raw := []byte(`{
		"agent": {
			"build": {
				"model": "anthropic/claude-sonnet-4-20250514"
			},
			"reviewer": {
				"mode": "subagent",
				"model": "google/gemini-2.5-flash"
			}
		}
	}`)

	targets := discoverTargets("/nonexistent", raw)

	// Should have 4 built-in + 1 custom = 5
	if len(targets) != 5 {
		t.Fatalf("expected 5 targets, got %d", len(targets))
	}

	// Check build has model set
	for _, tgt := range targets {
		if tgt.Name == "build" {
			if tgt.Model != "anthropic/claude-sonnet-4-20250514" {
				t.Errorf("build model = %q, want anthropic/claude-sonnet-4-20250514", tgt.Model)
			}
		}
		if tgt.Name == "reviewer" {
			if tgt.Mode != "subagent" {
				t.Errorf("reviewer mode = %q, want subagent", tgt.Mode)
			}
		}
	}
}

func TestDiscoverTargets_SystemAgentsExcluded(t *testing.T) {
	raw := []byte(`{
		"agent": {
			"compaction": { "model": "something" },
			"title": { "model": "something" },
			"summary": { "model": "something" }
		}
	}`)

	targets := discoverTargets("/nonexistent", raw)

	for _, tgt := range targets {
		if systemAgents[tgt.Name] {
			t.Errorf("system agent %q should be excluded", tgt.Name)
		}
	}
}

func TestDiscoverMarkdownAgents(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "agents")
	os.MkdirAll(agentDir, 0755)

	content := `---
description: Security auditor
mode: subagent
model: anthropic/claude-sonnet-4-20250514
---

You are a security auditor.
`
	os.WriteFile(filepath.Join(agentDir, "security.md"), []byte(content), 0644)

	raw := []byte(`{}`)
	seen := make(map[string]bool)
	targets := discoverMarkdownAgents(agentDir, raw, seen)

	if len(targets) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(targets))
	}
	if targets[0].Name != "security" {
		t.Errorf("name = %q, want security", targets[0].Name)
	}
	if targets[0].Mode != "subagent" {
		t.Errorf("mode = %q, want subagent", targets[0].Mode)
	}
	if targets[0].Model != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want anthropic/claude-sonnet-4-20250514", targets[0].Model)
	}
}

// TestMarkdownModeWinsOverJSONDefault verifies that when an agent has a markdown
// definition with an explicit mode and a JSON entry with no mode, the markdown
// mode is used (not the JSON default of "all"). This is the librarian scenario.
func TestMarkdownModeWinsOverJSONDefault(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "agents")
	os.MkdirAll(agentDir, 0755)

	// markdown defines mode: subagent
	content := "---\ndescription: Docs researcher\nmode: subagent\n---\nYou research docs.\n"
	os.WriteFile(filepath.Join(agentDir, "librarian.md"), []byte(content), 0644)

	// JSON entry has a model override but no mode (would default to "all")
	raw := []byte(`{
		"agent": {
			"librarian": {"model": "openrouter/anthropic/claude-haiku-4.5:nitro"}
		}
	}`)

	targets := discoverTargets(dir, raw)

	var librarian *Target
	for i := range targets {
		if targets[i].Name == "librarian" {
			librarian = &targets[i]
			break
		}
	}
	if librarian == nil {
		t.Fatal("librarian not found in targets")
	}
	if librarian.Mode != "subagent" {
		t.Errorf("mode = %q, want subagent (markdown should win over JSON default)", librarian.Mode)
	}
	if librarian.Model != "openrouter/anthropic/claude-haiku-4.5:nitro" {
		t.Errorf("model = %q, want JSON model override to be preserved", librarian.Model)
	}
}

// TestJSONOnlyAgentDefaultsToAll verifies that a JSON-only agent with no mode
// still defaults to "all" (no regression).
func TestJSONOnlyAgentDefaultsToAll(t *testing.T) {
	raw := []byte(`{
		"agent": {
			"scout": {"model": "openai/gpt-5"}
		}
	}`)

	targets := discoverTargets("/nonexistent", raw)

	var scout *Target
	for i := range targets {
		if targets[i].Name == "scout" {
			scout = &targets[i]
			break
		}
	}
	if scout == nil {
		t.Fatal("scout not found in targets")
	}
	if scout.Mode != "all" {
		t.Errorf("mode = %q, want all for JSON-only agent with no mode", scout.Mode)
	}
}

func TestParseFrontmatterField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	content := `---
description: My agent
mode: primary
model: google/gemini-2.5-flash
---

Content here
`
	os.WriteFile(path, []byte(content), 0644)

	if got := parseFrontmatterField(path, "mode"); got != "primary" {
		t.Errorf("mode = %q, want primary", got)
	}
	if got := parseFrontmatterField(path, "model"); got != "google/gemini-2.5-flash" {
		t.Errorf("model = %q, want google/gemini-2.5-flash", got)
	}
	if got := parseFrontmatterField(path, "nonexistent"); got != "" {
		t.Errorf("nonexistent = %q, want empty", got)
	}
}

func TestSetModel(t *testing.T) {
	// Create a temp config
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	initial := `{
  "agent": {
    "build": {}
  }
}`
	os.WriteFile(configPath, []byte(initial), 0644)

	// Override config dir for this test
	os.Setenv("OPENCODE_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCODE_CONFIG_DIR")

	// Set a model
	err := SetModel(KindAgent, "build", "anthropic/claude-sonnet-4")
	if err != nil {
		t.Fatalf("SetModel: %v", err)
	}

	// Read back and verify
	data, _ := os.ReadFile(configPath)
	raw := string(data)
	if !strings.Contains(raw, `"anthropic/claude-sonnet-4"`) {
		t.Errorf("config should contain model, got: %s", raw)
	}

	// Clear the model
	err = SetModel(KindAgent, "build", "")
	if err != nil {
		t.Fatalf("SetModel clear: %v", err)
	}

	data, _ = os.ReadFile(configPath)
	raw = string(data)
	if strings.Contains(raw, "claude-sonnet-4") {
		t.Errorf("config should not contain model after clear, got: %s", raw)
	}
}

func TestSetAgentOrder_ReordersKeys(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	initial := `{
  "agent": {
    "build": {"model": "anthropic/claude-sonnet-4"},
    "plan": {},
    "scout": {"mode": "primary"},
    "refine": {"mode": "primary"}
  }
}`
	os.WriteFile(configPath, []byte(initial), 0644)
	os.Setenv("OPENCODE_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCODE_CONFIG_DIR")

	// Reorder: refine before scout
	err := SetAgentOrder([]string{"refine", "scout"})
	if err != nil {
		t.Fatalf("SetAgentOrder: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	raw := string(data)

	// Verify both agents still present and in correct order
	refineIdx := strings.Index(raw, `"refine"`)
	scoutIdx := strings.Index(raw, `"scout"`)
	if refineIdx < 0 || scoutIdx < 0 {
		t.Fatalf("expected both agents in config, got: %s", raw)
	}
	if refineIdx > scoutIdx {
		t.Errorf("expected refine before scout, got order: refine=%d scout=%d\nconfig: %s", refineIdx, scoutIdx, raw)
	}
}

func TestSetAgentOrder_PreservesValues(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	initial := `{
  "agent": {
    "scout": {"model": "openai/gpt-5", "mode": "primary"},
    "refine": {"model": "anthropic/claude-opus-4"}
  }
}`
	os.WriteFile(configPath, []byte(initial), 0644)
	os.Setenv("OPENCODE_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCODE_CONFIG_DIR")

	err := SetAgentOrder([]string{"refine", "scout"})
	if err != nil {
		t.Fatalf("SetAgentOrder: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	raw := string(data)

	if !strings.Contains(raw, `"openai/gpt-5"`) {
		t.Errorf("scout model value should be preserved, got: %s", raw)
	}
	if !strings.Contains(raw, `"anthropic/claude-opus-4"`) {
		t.Errorf("refine model value should be preserved, got: %s", raw)
	}
}

func TestSetAgentOrder_SkipsUnknownNames(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	initial := `{
  "agent": {
    "scout": {},
    "refine": {}
  }
}`
	os.WriteFile(configPath, []byte(initial), 0644)
	os.Setenv("OPENCODE_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCODE_CONFIG_DIR")

	// includes a nonexistent name — should not error and should preserve both real agents
	err := SetAgentOrder([]string{"refine", "nonexistent", "scout"})
	if err != nil {
		t.Fatalf("SetAgentOrder with unknown name: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	raw := string(data)

	if !strings.Contains(raw, `"scout"`) || !strings.Contains(raw, `"refine"`) {
		t.Errorf("both real agents should be present, got: %s", raw)
	}
	if strings.Contains(raw, `"nonexistent"`) {
		t.Errorf("nonexistent agent should not be added, got: %s", raw)
	}
}

func TestSetAgentOrder_NoOpWithEmptyAgentSection(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	initial := `{"theme": "dark"}`
	os.WriteFile(configPath, []byte(initial), 0644)
	os.Setenv("OPENCODE_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCODE_CONFIG_DIR")

	err := SetAgentOrder([]string{"build", "plan"})
	if err != nil {
		t.Fatalf("SetAgentOrder with no agent section: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	raw := string(data)
	// Config should be unchanged
	if raw != initial {
		t.Errorf("config should be unchanged when no agent section, got: %s", raw)
	}
}

func TestBuiltInAgentsLocked(t *testing.T) {
	raw := []byte(`{}`)
	targets := discoverTargets("/nonexistent", raw)

	for _, tgt := range targets {
		switch tgt.Name {
		case "build", "plan":
			if !tgt.Locked {
				t.Errorf("agent %q should be Locked=true", tgt.Name)
			}
		case "general", "explore":
			if tgt.Locked {
				t.Errorf("agent %q should be Locked=false", tgt.Name)
			}
		}
	}
}

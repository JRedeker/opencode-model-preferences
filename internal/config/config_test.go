package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

// -- CLI-first model source tests --------------------------------------------
// These tests verify that Load uses CLI-discovered models as primary source
// and falls back to provider.*.models config parsing when CLI fails.

func TestLoad_UsesCLIModelsAsPrimarySource(t *testing.T) {
	dir := t.TempDir()
	// Config has one model in provider.*.models
	configJSON := `{
		"provider": {
			"anthropic": {
				"models": {
					"claude-config-only": {"name": "Config Only Model"}
				}
			}
		}
	}`
	os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(configJSON), 0644)
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

	// CLI returns a different (larger) set of models
	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "opencode" && len(args) == 1 && args[0] == "models" {
			return []byte("anthropic/claude-cli-model-1\nanthropic/claude-cli-model-2\ngoogle/gemini-cli\n"), nil
		}
		return nil, nil // refresh call succeeds silently
	})

	state, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Should have CLI models, not config-only model
	ids := make(map[string]bool)
	for _, m := range state.Models {
		ids[m.ID] = true
	}

	if ids["anthropic/claude-config-only"] {
		t.Error("config-only model should not appear when CLI returns models")
	}
	if !ids["anthropic/claude-cli-model-1"] {
		t.Error("CLI model anthropic/claude-cli-model-1 should be present")
	}
	if !ids["anthropic/claude-cli-model-2"] {
		t.Error("CLI model anthropic/claude-cli-model-2 should be present")
	}
	if !ids["google/gemini-cli"] {
		t.Error("CLI model google/gemini-cli should be present")
	}
}

func TestLoad_FallsBackToConfigWhenCLIFails(t *testing.T) {
	dir := t.TempDir()
	configJSON := `{
		"provider": {
			"anthropic": {
				"models": {
					"claude-config-fallback": {"name": "Config Fallback Model"}
				}
			}
		}
	}`
	os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(configJSON), 0644)
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

	// CLI fetch fails (non-zero exit)
	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "opencode" && len(args) == 1 && args[0] == "models" {
			return []byte("error: provider unavailable"), &execExitError{}
		}
		return nil, nil // refresh call succeeds
	})

	state, err := Load()
	if err != nil {
		t.Fatalf("Load() should not error on CLI fetch failure (fallback), got: %v", err)
	}

	ids := make(map[string]bool)
	for _, m := range state.Models {
		ids[m.ID] = true
	}

	if !ids["anthropic/claude-config-fallback"] {
		t.Error("config fallback model should be present when CLI fails")
	}
}

func TestLoad_FallsBackToConfigWhenCLIReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	configJSON := `{
		"provider": {
			"anthropic": {
				"models": {
					"claude-config-only": {"name": "Config Only"}
				}
			}
		}
	}`
	os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(configJSON), 0644)
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

	// CLI returns empty output (no parseable models)
	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "opencode" && len(args) == 1 && args[0] == "models" {
			return []byte(""), nil
		}
		return nil, nil
	})

	state, err := Load()
	if err != nil {
		t.Fatalf("Load() should not error on empty CLI output (fallback), got: %v", err)
	}

	ids := make(map[string]bool)
	for _, m := range state.Models {
		ids[m.ID] = true
	}

	if !ids["anthropic/claude-config-only"] {
		t.Error("config model should be present when CLI returns empty output")
	}
}

func TestLoad_CLIModelsAreSortedDeterministically(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(`{}`), 0644)
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "opencode" && len(args) == 1 && args[0] == "models" {
			// Return in reverse order
			return []byte("openai/gpt-5\ngoogle/gemini-2.5-pro\nanthropic/claude-sonnet-4-6\n"), nil
		}
		return nil, nil
	})

	state, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(state.Models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(state.Models))
	}
	if state.Models[0].ID != "anthropic/claude-sonnet-4-6" {
		t.Errorf("models[0].ID = %q, want anthropic/claude-sonnet-4-6 (sorted)", state.Models[0].ID)
	}
}

// execExitError is a minimal os/exec.ExitError stand-in for tests.
type execExitError struct{}

func (e *execExitError) Error() string { return "exit status 1" }

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

func TestDiscoverTargets_FindsProjectMarkdownAgentsFromNestedDir(t *testing.T) {
	root := t.TempDir()
	agentDir := filepath.Join(root, ".opencode", "agents")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}

	content := `---
description: Docs researcher
mode: subagent
---

You research docs.
`
	if err := os.WriteFile(filepath.Join(agentDir, "librarian.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write librarian.md: %v", err)
	}

	nested := filepath.Join(root, "nested", "workspace")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir nested: %v", err)
	}

	targets := discoverTargets("/nonexistent", []byte(`{}`))

	var librarian *Target
	for i := range targets {
		if targets[i].Name == "librarian" {
			librarian = &targets[i]
			break
		}
	}
	if librarian == nil {
		t.Fatal("librarian not found; expected project .opencode agent to be discovered")
	}
	if librarian.Mode != "subagent" {
		t.Errorf("mode = %q, want subagent", librarian.Mode)
	}
}

func TestDiscoverTargets_UsesOPENCODEPROJECTDIRForMarkdownAgents(t *testing.T) {
	root := t.TempDir()
	agentDir := filepath.Join(root, ".opencode", "agents")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}

	content := "---\nmode: subagent\n---\n"
	if err := os.WriteFile(filepath.Join(agentDir, "reviewer.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write reviewer.md: %v", err)
	}

	t.Setenv("OPENCODE_PROJECT_DIR", root)
	targets := discoverTargets("/nonexistent", []byte(`{}`))

	found := false
	for _, t := range targets {
		if t.Name == "reviewer" && t.Mode == "subagent" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected reviewer subagent from OPENCODE_PROJECT_DIR/.opencode/agents")
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
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

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
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

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
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

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
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

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

func TestDiscoverMarkdownAgents_HiddenField(t *testing.T) {
	dir := t.TempDir()

	// hidden: true agent
	hiddenContent := `---
description: Internal researcher
mode: subagent
hidden: true
---

You are hidden.
`
	os.WriteFile(filepath.Join(dir, "adv-researcher.md"), []byte(hiddenContent), 0644)

	// hidden: false agent (explicit)
	visibleContent := `---
description: Librarian
mode: subagent
hidden: false
---

You are visible.
`
	os.WriteFile(filepath.Join(dir, "librarian.md"), []byte(visibleContent), 0644)

	// no hidden field (defaults to false)
	defaultContent := `---
description: Explorer
mode: subagent
---

You explore.
`
	os.WriteFile(filepath.Join(dir, "explore-custom.md"), []byte(defaultContent), 0644)

	raw := []byte(`{}`)
	seen := make(map[string]bool)
	targets := discoverMarkdownAgents(dir, raw, seen)

	if len(targets) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(targets))
	}

	byName := make(map[string]Target)
	for _, tgt := range targets {
		byName[tgt.Name] = tgt
	}

	if !byName["adv-researcher"].Hidden {
		t.Error("adv-researcher should have Hidden=true")
	}
	if byName["librarian"].Hidden {
		t.Error("librarian should have Hidden=false (explicit)")
	}
	if byName["explore-custom"].Hidden {
		t.Error("explore-custom should have Hidden=false (default)")
	}
}

func TestDiscoverTargets_DescriptionFromJSON(t *testing.T) {
	raw := []byte(`{
		"agent": {
			"adv-reviewer": {
				"mode": "subagent",
				"hidden": true,
				"description": "Lead review synthesizer"
			}
		}
	}`)

	targets := discoverTargets("/nonexistent", raw)

	var reviewer *Target
	for i := range targets {
		if targets[i].Name == "adv-reviewer" {
			reviewer = &targets[i]
			break
		}
	}
	if reviewer == nil {
		t.Fatal("adv-reviewer not found in targets")
	}
	if reviewer.Description != "Lead review synthesizer" {
		t.Errorf("description = %q, want %q", reviewer.Description, "Lead review synthesizer")
	}
	if !reviewer.Hidden {
		t.Error("adv-reviewer should have Hidden=true")
	}
}

func TestDiscoverMarkdownAgents_DescriptionFromFrontmatter(t *testing.T) {
	dir := t.TempDir()

	content := `---
description: Security auditor for OWASP checks
mode: subagent
hidden: true
---

You are a security auditor.
`
	os.WriteFile(filepath.Join(dir, "adv-security-reviewer.md"), []byte(content), 0644)

	raw := []byte(`{}`)
	seen := make(map[string]bool)
	targets := discoverMarkdownAgents(dir, raw, seen)

	if len(targets) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(targets))
	}
	if targets[0].Description != "Security auditor for OWASP checks" {
		t.Errorf("description = %q, want %q", targets[0].Description, "Security auditor for OWASP checks")
	}
}

// -- Routing config tests ----------------------------------------------------

func TestRoutingPath_RespectsOPENCODE_CONFIG_DIR(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", dir)
	got := RoutingPath()
	want := filepath.Join(dir, "omp-routing.json")
	if got != want {
		t.Errorf("RoutingPath() = %q, want %q", got, want)
	}
}

func TestLoadRouting_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", dir)
	rc, err := LoadRouting()
	if err != nil {
		t.Fatalf("LoadRouting() error on missing file: %v", err)
	}
	if len(rc.RoleModels) != 0 {
		t.Errorf("expected empty RoleModels, got %d", len(rc.RoleModels))
	}
	if len(rc.TargetRoles) != 0 {
		t.Errorf("expected empty TargetRoles, got %d", len(rc.TargetRoles))
	}
}

func TestLoadRouting_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", dir)
	content := `{"role_models":{"brain":"anthropic/claude-opus-4"},"target_roles":{"build":"brain"}}`
	os.WriteFile(filepath.Join(dir, "omp-routing.json"), []byte(content), 0644)

	rc, err := LoadRouting()
	if err != nil {
		t.Fatalf("LoadRouting() error: %v", err)
	}
	if rc.RoleModels[RoleBrain] != "anthropic/claude-opus-4" {
		t.Errorf("RoleModels[brain] = %q, want anthropic/claude-opus-4", rc.RoleModels[RoleBrain])
	}
	if rc.TargetRoles["build"] != RoleBrain {
		t.Errorf("TargetRoles[build] = %q, want brain", rc.TargetRoles["build"])
	}
}

func TestLoadRouting_NilMapsInitialized(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", dir)
	// JSON with no fields — maps should still be initialized
	os.WriteFile(filepath.Join(dir, "omp-routing.json"), []byte(`{}`), 0644)

	rc, err := LoadRouting()
	if err != nil {
		t.Fatalf("LoadRouting() error: %v", err)
	}
	if rc.RoleModels == nil {
		t.Error("RoleModels should be initialized, got nil")
	}
	if rc.TargetRoles == nil {
		t.Error("TargetRoles should be initialized, got nil")
	}
}

func TestSaveRouting_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", dir)
	rc := RoutingConfig{
		RoleModels:  map[UserRole]string{RoleBrain: "anthropic/claude-opus-4"},
		TargetRoles: map[string]UserRole{"build": RoleBrain},
	}
	if err := SaveRouting(rc); err != nil {
		t.Fatalf("SaveRouting() error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "omp-routing.json"))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if !strings.Contains(string(data), "brain") {
		t.Errorf("saved file should contain role name, got: %s", data)
	}
}

func TestSaveRouting_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

	rc := RoutingConfig{
		RoleModels: map[UserRole]string{
			RoleBrain:   "anthropic/claude-opus-4",
			RoleBuilder: "anthropic/claude-sonnet-4",
		},
		TargetRoles: map[string]UserRole{
			"build":   RoleBrain,
			"plan":    RoleBrain,
			"general": RoleBuilder,
		},
	}
	if err := SaveRouting(rc); err != nil {
		t.Fatalf("SaveRouting() error: %v", err)
	}
	loaded, err := LoadRouting()
	if err != nil {
		t.Fatalf("LoadRouting() error: %v", err)
	}
	if loaded.RoleModels[RoleBrain] != "anthropic/claude-opus-4" {
		t.Errorf("round-trip RoleModels[brain] = %q", loaded.RoleModels[RoleBrain])
	}
	if loaded.RoleModels[RoleBuilder] != "anthropic/claude-sonnet-4" {
		t.Errorf("round-trip RoleModels[builder] = %q", loaded.RoleModels[RoleBuilder])
	}
	if loaded.TargetRoles["build"] != RoleBrain {
		t.Errorf("round-trip TargetRoles[build] = %q", loaded.TargetRoles["build"])
	}
	if loaded.TargetRoles["general"] != RoleBuilder {
		t.Errorf("round-trip TargetRoles[general] = %q", loaded.TargetRoles["general"])
	}
}

// TestSaveRouting_AtomicWrite verifies that SaveRouting leaves no temp files
// behind after a successful write (atomic temp+rename pattern).
func TestSaveRouting_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

	rc := RoutingConfig{
		RoleModels:  map[UserRole]string{RoleBrain: "openai/gpt-5"},
		TargetRoles: map[string]UserRole{},
	}
	if err := SaveRouting(rc); err != nil {
		t.Fatalf("SaveRouting() error: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".omp-routing-") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind after SaveRouting: %s", e.Name())
		}
	}
}

// -- ApplyRouting tests ------------------------------------------------------

func TestApplyRouting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

	initial := `{
  "agent": {
    "build": {"mode": "primary"},
    "plan": {"mode": "primary"},
    "general": {"mode": "subagent"},
    "explore": {"mode": "subagent"}
  },
  "command": {
    "deploy": {}
  }
}`
	os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(initial), 0644)

	targets := []Target{
		{Name: "build", Kind: KindAgent, Mode: "primary"},
		{Name: "plan", Kind: KindAgent, Mode: "primary"},
		{Name: "general", Kind: KindAgent, Mode: "subagent"},
		{Name: "explore", Kind: KindAgent, Mode: "subagent"},
		{Name: "deploy", Kind: KindCommand},
	}
	rc := RoutingConfig{
		RoleModels: map[UserRole]string{
			RoleBrain:   "anthropic/claude-opus-4",
			RoleBuilder: "anthropic/claude-sonnet-4",
		},
		TargetRoles: map[string]UserRole{
			"build":   RoleBrain,
			"plan":    RoleBrain,
			"general": RoleBuilder,
			// explore has no role — should be left unchanged
			"deploy": RoleBrain,
		},
	}

	if err := ApplyRouting(rc, targets); err != nil {
		t.Fatalf("ApplyRouting() error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "opencode.json"))
	raw := string(data)

	// Agents with brain role get opus
	for _, name := range []string{"build", "plan"} {
		got := gjson.Get(raw, "agent."+name+".model").String()
		if got != "anthropic/claude-opus-4" {
			t.Errorf("agent %q model = %q, want anthropic/claude-opus-4", name, got)
		}
	}
	// general has builder role → sonnet
	got := gjson.Get(raw, "agent.general.model").String()
	if got != "anthropic/claude-sonnet-4" {
		t.Errorf("agent general model = %q, want anthropic/claude-sonnet-4", got)
	}
	// explore has no role → no model field written
	if gjson.Get(raw, "agent.explore.model").Exists() {
		t.Error("explore should not have model set (no role assigned)")
	}
	// command deploy has brain role → opus
	got = gjson.Get(raw, "command.deploy.model").String()
	if got != "anthropic/claude-opus-4" {
		t.Errorf("command deploy model = %q, want anthropic/claude-opus-4", got)
	}
}

func TestApplyRouting_UnmappedRoleSkipped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

	initial := `{
  "agent": {
    "build": {"mode": "primary", "model": "existing/model"}
  }
}`
	os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(initial), 0644)

	targets := []Target{
		{Name: "build", Kind: KindAgent, Mode: "primary"},
	}
	// build has a role, but that role has no model mapped
	rc := RoutingConfig{
		RoleModels:  map[UserRole]string{},
		TargetRoles: map[string]UserRole{"build": RoleFixer},
	}

	if err := ApplyRouting(rc, targets); err != nil {
		t.Fatalf("ApplyRouting() error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "opencode.json"))
	got := gjson.Get(string(data), "agent.build.model").String()
	if got != "existing/model" {
		t.Errorf("build model = %q, want existing/model (unmapped role should not change it)", got)
	}
}

func TestApplyRouting_ClearAll(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", dir)

	initial := `{
  "agent": {
    "build": {"model": "anthropic/claude-opus-4"},
    "general": {"model": "anthropic/claude-haiku-4"}
  }
}`
	os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(initial), 0644)

	targets := []Target{
		{Name: "build", Kind: KindAgent, Mode: "primary"},
		{Name: "general", Kind: KindAgent, Mode: "subagent"},
	}

	if err := ClearAllModelAssignments(targets); err != nil {
		t.Fatalf("ClearAllModelAssignments() error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "opencode.json"))
	raw := string(data)

	if gjson.Get(raw, "agent.build.model").Exists() {
		t.Error("build model should be cleared")
	}
	if gjson.Get(raw, "agent.general.model").Exists() {
		t.Error("general model should be cleared")
	}
}

func TestIsValidUserRole(t *testing.T) {
	for _, r := range AllUserRoles() {
		if !IsValidUserRole(r) {
			t.Errorf("IsValidUserRole(%q) = false, want true", r)
		}
	}
	if IsValidUserRole("bogus") {
		t.Error("IsValidUserRole(bogus) = true, want false")
	}
}

func TestRoleForTarget(t *testing.T) {
	cases := []struct {
		name   string
		target Target
		want   Role
	}{
		{"primary agent is orchestrator", Target{Kind: KindAgent, Mode: "primary"}, RoleOrchestrator},
		{"subagent is worker", Target{Kind: KindAgent, Mode: "subagent"}, RoleWorker},
		{"all mode is orchestrator", Target{Kind: KindAgent, Mode: "all"}, RoleOrchestrator},
		{"empty mode is orchestrator", Target{Kind: KindAgent, Mode: ""}, RoleOrchestrator},
		{"command is orchestrator", Target{Kind: KindCommand}, RoleOrchestrator},
		{"command with arbitrary mode is still orchestrator", Target{Kind: KindCommand, Mode: "subagent"}, RoleOrchestrator},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RoleForTarget(tc.target)
			if got != tc.want {
				t.Errorf("RoleForTarget(%+v) = %q, want %q", tc.target, got, tc.want)
			}
		})
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

// -- Role taxonomy tests -----------------------------------------------------

func TestUserRoles_AllFiveDeclared(t *testing.T) {
	want := []UserRole{RoleBrain, RoleTasker, RoleBuilder, RoleLibrarian, RoleFixer}
	got := AllUserRoles()
	if len(got) != len(want) {
		t.Fatalf("AllUserRoles() returned %d roles, want %d: %v", len(got), len(want), got)
	}
	byRole := make(map[UserRole]bool)
	for _, r := range got {
		byRole[r] = true
	}
	for _, r := range want {
		if !byRole[r] {
			t.Errorf("missing role %q in AllUserRoles()", r)
		}
	}
}

func TestUserRoles_DescriptionsNonEmpty(t *testing.T) {
	for _, r := range AllUserRoles() {
		if UserRoleDescription(r) == "" {
			t.Errorf("UserRoleDescription(%q) is empty", r)
		}
	}
}

func TestUserRoles_DoNotAffectRoleForTarget(t *testing.T) {
	// Declaring user roles must not change how RoleForTarget classifies targets.
	cases := []struct {
		target Target
		want   Role
	}{
		{Target{Kind: KindAgent, Mode: "primary"}, RoleOrchestrator},
		{Target{Kind: KindAgent, Mode: "subagent"}, RoleWorker},
		{Target{Kind: KindCommand}, RoleOrchestrator},
	}
	for _, tc := range cases {
		if got := RoleForTarget(tc.target); got != tc.want {
			t.Errorf("RoleForTarget(%+v) = %q, want %q (user roles must not affect routing)", tc.target, got, tc.want)
		}
	}
}

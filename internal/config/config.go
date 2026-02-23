// Package config reads and writes OpenCode's global configuration.
//
// It discovers agents (built-in + markdown), commands, and available
// models from the provider registry. Config writes use tidwall/sjson
// for surgical JSON path updates that preserve formatting.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// TargetKind distinguishes agents from commands.
type TargetKind string

const (
	KindAgent   TargetKind = "agent"
	KindCommand TargetKind = "command"
)

// Target represents an agent or command that can have a model preference.
type Target struct {
	Name    string
	Kind    TargetKind
	Mode    string // "primary", "subagent", "system" (agents only)
	Model   string // current model preference, empty = none
	BuiltIn bool
	Locked  bool // true for built-in primary agents whose cycle order is fixed by OpenCode
}

// Model represents an available model from a provider.
type Model struct {
	Provider string
	ID       string // full ID: provider/model-id
	Name     string // display name
}

// State holds the full resolved state for the TUI.
type State struct {
	Targets []Target
	Models  []Model
}

// Built-in agents from OpenCode core.
var builtinAgents = []Target{
	{Name: "build", Kind: KindAgent, Mode: "primary", BuiltIn: true, Locked: true},
	{Name: "plan", Kind: KindAgent, Mode: "primary", BuiltIn: true, Locked: true},
	{Name: "general", Kind: KindAgent, Mode: "subagent", BuiltIn: true},
	{Name: "explore", Kind: KindAgent, Mode: "subagent", BuiltIn: true},
}

// System agents that should not be shown.
var systemAgents = map[string]bool{
	"compaction": true,
	"title":      true,
	"summary":    true,
}

// ConfigDir returns the OpenCode global config directory.
func ConfigDir() string {
	if dir := os.Getenv("OPENCODE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "opencode")
}

// ConfigPath returns the path to opencode.json.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "opencode.json")
}

// Load reads the global config and resolves all targets and models.
func Load() (*State, error) {
	configDir := ConfigDir()
	if configDir == "" {
		return nil, fmt.Errorf("could not determine config directory")
	}

	raw, err := os.ReadFile(ConfigPath())
	if err != nil {
		return nil, fmt.Errorf("reading opencode.json: %w", err)
	}

	state := &State{}

	// 1. Discover models from provider registry
	state.Models = discoverModels(raw)

	// 2. Discover agents: built-in + config + markdown
	state.Targets = discoverTargets(configDir, raw)

	return state, nil
}

// discoverModels extracts all models from provider.*.models in the config.
func discoverModels(raw []byte) []Model {
	var models []Model

	providers := gjson.GetBytes(raw, "provider")
	if !providers.Exists() {
		return models
	}

	providers.ForEach(func(providerID, providerVal gjson.Result) bool {
		providerVal.Get("models").ForEach(func(modelID, modelVal gjson.Result) bool {
			name := modelVal.Get("name").String()
			if name == "" {
				name = modelID.String()
			}
			models = append(models, Model{
				Provider: providerID.String(),
				ID:       providerID.String() + "/" + modelID.String(),
				Name:     name,
			})
			return true
		})
		return true
	})

	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	return models
}

// discoverTargets finds all agents and commands from config + markdown files.
func discoverTargets(configDir string, raw []byte) []Target {
	seen := make(map[string]bool)
	var targets []Target

	// Built-in agents
	for _, a := range builtinAgents {
		a.Model = gjson.GetBytes(raw, "agent."+a.Name+".model").String()
		targets = append(targets, a)
		seen[a.Name] = true
	}

	// JSON-configured agents
	gjson.GetBytes(raw, "agent").ForEach(func(name, val gjson.Result) bool {
		n := name.String()
		if seen[n] || systemAgents[n] {
			return true
		}
		mode := val.Get("mode").String()
		if mode == "" {
			mode = "all"
		}
		targets = append(targets, Target{
			Name:  n,
			Kind:  KindAgent,
			Mode:  mode,
			Model: val.Get("model").String(),
		})
		seen[n] = true
		return true
	})

	// Markdown agents: global + project
	for _, dir := range []string{
		filepath.Join(configDir, "agents"),
		filepath.Join(".opencode", "agents"),
	} {
		targets = append(targets, discoverMarkdownAgents(dir, raw, seen)...)
	}

	// JSON-configured commands
	gjson.GetBytes(raw, "command").ForEach(func(name, val gjson.Result) bool {
		n := name.String()
		targets = append(targets, Target{
			Name:  n,
			Kind:  KindCommand,
			Model: val.Get("model").String(),
		})
		seen["cmd:"+n] = true
		return true
	})

	// Markdown commands: global + project
	for _, dir := range []string{
		filepath.Join(configDir, "commands"),
		filepath.Join(".opencode", "commands"),
	} {
		targets = append(targets, discoverMarkdownCommands(dir, raw, seen)...)
	}

	return targets
}

// discoverMarkdownAgents scans a directory for *.md agent definitions.
func discoverMarkdownAgents(dir string, raw []byte, seen map[string]bool) []Target {
	var targets []Target

	entries, err := os.ReadDir(dir)
	if err != nil {
		return targets
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		if seen[name] || systemAgents[name] {
			continue
		}

		mode := parseFrontmatterField(filepath.Join(dir, e.Name()), "mode")
		if mode == "" {
			mode = "all"
		}

		// Check if there's a model override in the JSON config
		model := gjson.GetBytes(raw, "agent."+name+".model").String()

		// Also check frontmatter for model
		if model == "" {
			model = parseFrontmatterField(filepath.Join(dir, e.Name()), "model")
		}

		targets = append(targets, Target{
			Name:  name,
			Kind:  KindAgent,
			Mode:  mode,
			Model: model,
		})
		seen[name] = true
	}

	return targets
}

// discoverMarkdownCommands scans a directory for *.md command definitions.
func discoverMarkdownCommands(dir string, raw []byte, seen map[string]bool) []Target {
	var targets []Target

	entries, err := os.ReadDir(dir)
	if err != nil {
		return targets
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		key := "cmd:" + name
		if seen[key] {
			continue
		}

		model := gjson.GetBytes(raw, "command."+name+".model").String()
		if model == "" {
			model = parseFrontmatterField(filepath.Join(dir, e.Name()), "model")
		}

		targets = append(targets, Target{
			Name:  name,
			Kind:  KindCommand,
			Model: model,
		})
		seen[key] = true
	}

	return targets
}

// parseFrontmatterField does a minimal parse of YAML frontmatter for a single field.
func parseFrontmatterField(path, field string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return ""
	}

	end := strings.Index(content[3:], "---")
	if end < 0 {
		return ""
	}

	frontmatter := content[3 : 3+end]
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, field+":") {
			val := strings.TrimPrefix(line, field+":")
			return strings.TrimSpace(val)
		}
	}
	return ""
}

// SetModel writes a model preference to the global config.
// Pass empty string to clear the preference.
func SetModel(kind TargetKind, name, model string) error {
	configPath := ConfigPath()

	raw, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	jsonPath := string(kind) + "." + name + ".model"

	var updated []byte
	if model == "" {
		// Remove the model key
		updated, err = sjson.DeleteBytes(raw, jsonPath)
		if err != nil {
			return fmt.Errorf("deleting key: %w", err)
		}
	} else {
		updated, err = sjson.SetBytes(raw, jsonPath, model)
		if err != nil {
			return fmt.Errorf("setting key: %w", err)
		}
	}

	// Verify it's still valid JSON
	if !json.Valid(updated) {
		return fmt.Errorf("resulting config is invalid JSON")
	}

	return os.WriteFile(configPath, updated, 0644)
}

// SetAgentOrder rewrites the agent section of opencode.json so that keys appear
// in the given order. This controls the Tab-cycle order for custom primary agents
// in OpenCode, since it uses JS object insertion order.
//
// Built-in agents (build, plan) are always first in OpenCode's cycle regardless
// of JSON order, so only custom/non-locked agents benefit from reordering.
//
// names must contain all agent names currently in the config agent section.
// Any names not present in the current config are ignored.
func SetAgentOrder(names []string) error {
	configPath := ConfigPath()

	raw, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	// Collect existing agent entries in a map: name -> raw JSON value
	agentEntries := make(map[string]string)
	gjson.GetBytes(raw, "agent").ForEach(func(key, val gjson.Result) bool {
		agentEntries[key.String()] = val.Raw
		return true
	})

	if len(agentEntries) == 0 {
		// Nothing to reorder
		return nil
	}

	// Delete the entire agent section, then rebuild in order
	updated, err := sjson.DeleteBytes(raw, "agent")
	if err != nil {
		return fmt.Errorf("deleting agent section: %w", err)
	}

	// Re-insert entries in the requested order (skip any names not in config)
	for _, name := range names {
		raw, ok := agentEntries[name]
		if !ok {
			continue
		}
		var val interface{}
		if err := json.Unmarshal([]byte(raw), &val); err != nil {
			return fmt.Errorf("parsing agent %q: %w", name, err)
		}
		updated, err = sjson.SetBytes(updated, "agent."+name, val)
		if err != nil {
			return fmt.Errorf("writing agent %q: %w", name, err)
		}
		delete(agentEntries, name)
	}

	// Append any remaining agents not in the names list (preserve them at end)
	for name, raw := range agentEntries {
		var val interface{}
		if err := json.Unmarshal([]byte(raw), &val); err != nil {
			return fmt.Errorf("parsing agent %q: %w", name, err)
		}
		updated, err = sjson.SetBytes(updated, "agent."+name, val)
		if err != nil {
			return fmt.Errorf("writing remaining agent %q: %w", name, err)
		}
	}

	if !json.Valid(updated) {
		return fmt.Errorf("resulting config is invalid JSON")
	}

	return os.WriteFile(configPath, updated, 0644)
}

// Package config — preferences.go
//
// Direct per-target model preferences. Each agent or sub-agent maps directly to
// a model ID with no intermediate abstraction. Config is stored in
// ~/.config/opencode/omp-preferences.json (separate from opencode.json which
// uses additionalProperties:false).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// PreferencesConfig holds per-target model assignments.
// TargetModels maps each target name (agent or sub-agent) to a model ID.
// ClearedModels tracks targets whose model was explicitly cleared by the user,
// so ApplyPreferences can remove the model key from opencode.json.
type PreferencesConfig struct {
	TargetModels  map[string]string `json:"target_models"`
	ClearedModels map[string]bool   `json:"cleared_models,omitempty"`
}

// PreferencesPath returns the path to omp-preferences.json, respecting
// OPENCODE_CONFIG_DIR.
func PreferencesPath() string {
	return filepath.Join(ConfigDir(), "omp-preferences.json")
}

// LoadPreferences reads the preferences config from disk.
// Returns an empty PreferencesConfig (no error) if the file does not exist.
func LoadPreferences() (PreferencesConfig, error) {
	data, err := os.ReadFile(PreferencesPath())
	if os.IsNotExist(err) {
		return PreferencesConfig{
			TargetModels:  make(map[string]string),
			ClearedModels: make(map[string]bool),
		}, nil
	}
	if err != nil {
		return PreferencesConfig{}, err
	}
	var pc PreferencesConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return PreferencesConfig{}, err
	}
	if pc.TargetModels == nil {
		pc.TargetModels = make(map[string]string)
	}
	if pc.ClearedModels == nil {
		pc.ClearedModels = make(map[string]bool)
	}
	return pc, nil
}

// SavePreferences writes the preferences config to disk atomically
// (temp file + rename).
func SavePreferences(pc PreferencesConfig) error {
	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(PreferencesPath(), data, 0644)
}

// ApplyPreferences writes model preferences to opencode.json for all targets
// that have a model assignment in the preferences config. Targets without an
// assignment are left unchanged unless explicitly cleared. Creates new entries
// in opencode.json when a target has a model to set but no existing entry.
func ApplyPreferences(pc PreferencesConfig, targets []Target) error {
	configPath := ConfigPath()
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	updated := raw
	for _, t := range targets {
		existsInConfig := gjson.GetBytes(raw, "agent."+t.Name).Exists()
		jsonPath := "agent." + t.Name + ".model"

		// Explicitly cleared: remove the model key from opencode.json.
		// Skip if the target doesn't exist in config — nothing to clear.
		if pc.ClearedModels[t.Name] {
			if !existsInConfig {
				continue
			}
			updated, err = sjson.DeleteBytes(updated, jsonPath)
			if err != nil {
				return fmt.Errorf("deleting %s: %w", jsonPath, err)
			}
			continue
		}

		// Set model if assigned. sjson.SetBytes creates intermediate
		// objects automatically, so targets not yet in opencode.json
		// get a new entry with just the model field.
		model, ok := pc.TargetModels[t.Name]
		if !ok || model == "" {
			continue
		}

		updated, err = sjson.SetBytes(updated, jsonPath, model)
		if err != nil {
			return fmt.Errorf("setting %s: %w", jsonPath, err)
		}
	}

	return os.WriteFile(configPath, updated, 0644)
}

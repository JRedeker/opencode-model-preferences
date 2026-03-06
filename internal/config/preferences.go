// Package config — preferences.go
//
// Direct per-target model preferences. Each agent or command maps directly to
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
// TargetModels maps each target name (agent or command) to a model ID.
type PreferencesConfig struct {
	TargetModels map[string]string `json:"target_models"`
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
			TargetModels: make(map[string]string),
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
// assignment are left unchanged. Only writes to targets that already exist in
// opencode.json.
func ApplyPreferences(pc PreferencesConfig, targets []Target) error {
	configPath := ConfigPath()
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	updated := raw
	for _, t := range targets {
		model, ok := pc.TargetModels[t.Name]
		if !ok || model == "" {
			continue
		}

		var jsonPath string
		if t.Kind == KindCommand {
			jsonPath = "command." + t.Name + ".model"
		} else {
			jsonPath = "agent." + t.Name + ".model"
		}

		// Only write if the target already exists in config.
		var keyExists bool
		if t.Kind == KindCommand {
			keyExists = gjson.GetBytes(raw, "command."+t.Name).Exists()
		} else {
			keyExists = gjson.GetBytes(raw, "agent."+t.Name).Exists()
		}
		if !keyExists {
			continue
		}

		updated, err = sjson.SetBytes(updated, jsonPath, model)
		if err != nil {
			return fmt.Errorf("setting %s: %w", jsonPath, err)
		}
	}

	return os.WriteFile(configPath, updated, 0644)
}

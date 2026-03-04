// Package config — routing.go
//
// Routing config is stored in a separate file (~/.config/opencode/omp-routing.json)
// because opencode.json uses additionalProperties:false and rejects unknown keys.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// writeFileAtomic writes data to path atomically using a temp file + rename,
// so a crash mid-write cannot corrupt the destination file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".omp-routing-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// -- User-facing role taxonomy -----------------------------------------------
//
// These five roles define the vocabulary for model assignment. They are
// declarative only — not yet wired to any routing or target-mapping logic.
// Future work will let users assign a model to each role and map roles to
// agents/commands.
//
// NOTE: This taxonomy is intentionally forward-declared. The constants and
// helpers are exported so callers can reference them in UI and documentation
// without depending on internal routing logic. Do not remove until the
// role-to-agent mapping feature is implemented.

// UserRole is a named slot in the user-facing role taxonomy.
type UserRole string

const (
	// RoleBrain is the model for orchestration, planning, and reviewing.
	RoleBrain UserRole = "brain"
	// RoleTasker is the model for purely agentic operations and tool calling.
	RoleTasker UserRole = "tasker"
	// RoleBuilder is the model for coding.
	RoleBuilder UserRole = "builder"
	// RoleLibrarian is the model for fact-checking and research.
	RoleLibrarian UserRole = "librarian"
	// RoleFixer is the model for expertise and problem solving.
	RoleFixer UserRole = "fixer"
)

// userRoleDescriptions maps each UserRole to its human-readable description.
var userRoleDescriptions = map[UserRole]string{
	RoleBrain:     "Orchestration, planning, and reviewing",
	RoleTasker:    "Purely agentic operations and tool calling",
	RoleBuilder:   "Coding",
	RoleLibrarian: "Fact-checking and research",
	RoleFixer:     "Expertise and problem solving",
}

// AllUserRoles returns the canonical ordered list of all five user roles.
func AllUserRoles() []UserRole {
	return []UserRole{RoleBrain, RoleTasker, RoleBuilder, RoleLibrarian, RoleFixer}
}

// UserRoleDescription returns the human-readable description for a role.
func UserRoleDescription(r UserRole) string {
	return userRoleDescriptions[r]
}

// -- Mapping -----------------------------------------------------------------

// Mapping defines a named pair of orchestrator and worker models.
// Orchestrator model is applied to primary/all agents and commands.
// Worker model is applied to subagents.
type Mapping struct {
	Name         string `json:"name"`
	Orchestrator string `json:"orchestrator"`
	Worker       string `json:"worker"`
}

// RoutingConfig holds all named mappings.
type RoutingConfig struct {
	Mappings []Mapping `json:"mappings"`
}

// RoutingPath returns the path to omp-routing.json, respecting OPENCODE_CONFIG_DIR.
func RoutingPath() string {
	return filepath.Join(ConfigDir(), "omp-routing.json")
}

// LoadRouting reads the routing config from disk.
// Returns an empty RoutingConfig (no error) if the file does not exist.
func LoadRouting() (RoutingConfig, error) {
	data, err := os.ReadFile(RoutingPath())
	if os.IsNotExist(err) {
		return RoutingConfig{}, nil
	}
	if err != nil {
		return RoutingConfig{}, err
	}
	var rc RoutingConfig
	if err := json.Unmarshal(data, &rc); err != nil {
		return RoutingConfig{}, err
	}
	return rc, nil
}

// ValidateMapping checks whether the orchestrator and worker model IDs in a
// mapping exist in the known model list. Returns advisory warning strings
// (not errors) — models may be available at runtime even if not in the list.
func ValidateMapping(m Mapping, known []Model) []string {
	knownSet := make(map[string]bool, len(known))
	for _, mdl := range known {
		knownSet[mdl.ID] = true
	}
	var warnings []string
	if m.Orchestrator != "" && !knownSet[m.Orchestrator] {
		warnings = append(warnings, "orchestrator model not found in registry: "+m.Orchestrator)
	}
	if m.Worker != "" && !knownSet[m.Worker] {
		warnings = append(warnings, "worker model not found in registry: "+m.Worker)
	}
	return warnings
}

// SaveRouting writes the routing config to disk atomically (temp file + rename),
// so a crash mid-write cannot corrupt the routing config.
func SaveRouting(rc RoutingConfig) error {
	data, err := json.MarshalIndent(rc, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(RoutingPath(), data, 0644)
}

// ApplyActiveMapping writes the orchestrator model to all primary/all agents
// and commands, and the worker model to all subagents, using sjson chaining
// so the entire write is a single atomic file operation.
func ApplyActiveMapping(m Mapping, targets []Target) error {
	configPath := ConfigPath()
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	updated := raw
	for _, t := range targets {
		role := RoleForTarget(t)
		var model string
		if role == RoleOrchestrator {
			model = m.Orchestrator
		} else {
			model = m.Worker
		}

		var jsonPath string
		if t.Kind == KindCommand {
			jsonPath = "command." + t.Name + ".model"
		} else {
			jsonPath = "agent." + t.Name + ".model"
		}

		// Only write if the key already exists in config (don't create new agent entries)
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

// ClearAllModelAssignments removes the model field from all agents and commands
// that have one set in opencode.json.
func ClearAllModelAssignments(targets []Target) error {
	configPath := ConfigPath()
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	updated := raw
	for _, t := range targets {
		var jsonPath string
		if t.Kind == KindCommand {
			jsonPath = "command." + t.Name + ".model"
		} else {
			jsonPath = "agent." + t.Name + ".model"
		}
		updated, err = sjson.DeleteBytes(updated, jsonPath)
		if err != nil {
			return fmt.Errorf("deleting %s: %w", jsonPath, err)
		}
	}

	return os.WriteFile(configPath, updated, 0644)
}

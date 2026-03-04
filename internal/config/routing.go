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

// UserRole is a named slot in the user-facing role taxonomy.
// Roles are purely for model mapping — they carry no context or system prompt.
type UserRole string

const (
	RoleBrain     UserRole = "brain"
	RoleTasker    UserRole = "tasker"
	RoleBuilder   UserRole = "builder"
	RoleLibrarian UserRole = "librarian"
	RoleFixer     UserRole = "fixer"
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

// IsValidUserRole returns true if r is one of the five defined roles.
func IsValidUserRole(r UserRole) bool {
	_, ok := userRoleDescriptions[r]
	return ok
}

// -- Routing config ----------------------------------------------------------

// RoutingConfig holds role-to-model mappings and target-to-role assignments.
//
// RoleModels maps each role to a model ID. If a role has no entry (or empty
// string), targets assigned to that role keep their existing model unchanged.
//
// TargetRoles maps each target name (agent or command) to a role. Targets
// without an entry have no role assignment and are not affected by ApplyRouting.
type RoutingConfig struct {
	RoleModels  map[UserRole]string `json:"role_models"`
	TargetRoles map[string]UserRole `json:"target_roles"`
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
		return RoutingConfig{
			RoleModels:  make(map[UserRole]string),
			TargetRoles: make(map[string]UserRole),
		}, nil
	}
	if err != nil {
		return RoutingConfig{}, err
	}
	var rc RoutingConfig
	if err := json.Unmarshal(data, &rc); err != nil {
		return RoutingConfig{}, err
	}
	if rc.RoleModels == nil {
		rc.RoleModels = make(map[UserRole]string)
	}
	if rc.TargetRoles == nil {
		rc.TargetRoles = make(map[string]UserRole)
	}
	return rc, nil
}

// SaveRouting writes the routing config to disk atomically (temp file + rename).
func SaveRouting(rc RoutingConfig) error {
	data, err := json.MarshalIndent(rc, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(RoutingPath(), data, 0644)
}

// ApplyRouting writes model preferences to opencode.json for all targets that
// have a role assigned AND whose role has a model mapped. Targets without a
// role assignment or whose role has no model are left unchanged.
func ApplyRouting(rc RoutingConfig, targets []Target) error {
	configPath := ConfigPath()
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	updated := raw
	for _, t := range targets {
		role, hasRole := rc.TargetRoles[t.Name]
		if !hasRole {
			continue
		}
		model, hasModel := rc.RoleModels[role]
		if !hasModel || model == "" {
			continue
		}

		var jsonPath string
		if t.Kind == KindCommand {
			jsonPath = "command." + t.Name + ".model"
		} else {
			jsonPath = "agent." + t.Name + ".model"
		}

		// Only write if the target already exists in config
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

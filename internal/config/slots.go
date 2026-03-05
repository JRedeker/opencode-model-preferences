// Package config — slots.go
//
// Slot-based model mapping replaces the fixed 5-role taxonomy with user-defined
// named slots. Each slot has an ID (e.g. "slot-1"), a user-editable display name,
// and an optional model assignment. Targets are assigned to slots by ID.
//
// Config is stored in ~/.config/opencode/omp-slots.json (separate from
// opencode.json which uses additionalProperties:false).
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Slot is a named model assignment slot.
type Slot struct {
	ID    string `json:"id"`    // stable identifier, e.g. "slot-1"
	Name  string `json:"name"`  // user-editable display name, e.g. "Brain"
	Model string `json:"model"` // model ID, empty = no assignment
}

// SlotsConfig holds the slot definitions and per-target model routing.
//
// Slots is the ordered list of user-defined slots (default: 4).
// TargetSlots maps each target name to a slot ID. Targets without an entry
// have no slot assignment and are not affected by ApplySlots.
// TargetModels maps a target directly to a model ID and overrides slot mapping.
type SlotsConfig struct {
	Slots        []Slot            `json:"slots"`
	TargetSlots  map[string]string `json:"target_slots"`
	TargetModels map[string]string `json:"target_models,omitempty"`
}

// DefaultSlotsConfig returns a fresh SlotsConfig with 4 default slots and no
// target assignments. Used when no omp-slots.json exists yet.
func DefaultSlotsConfig() SlotsConfig {
	return SlotsConfig{
		Slots: []Slot{
			{ID: "slot-1", Name: "Slot 1"},
			{ID: "slot-2", Name: "Slot 2"},
			{ID: "slot-3", Name: "Slot 3"},
			{ID: "slot-4", Name: "Slot 4"},
		},
		TargetSlots:  make(map[string]string),
		TargetModels: make(map[string]string),
	}
}

// SlotsPath returns the path to omp-slots.json, respecting OPENCODE_CONFIG_DIR.
func SlotsPath() string {
	return filepath.Join(ConfigDir(), "omp-slots.json")
}

// LoadSlots reads the slots config from disk.
// Returns an empty SlotsConfig (no error) if the file does not exist.
func LoadSlots() (SlotsConfig, error) {
	data, err := os.ReadFile(SlotsPath())
	if os.IsNotExist(err) {
		return SlotsConfig{
			Slots:        []Slot{},
			TargetSlots:  make(map[string]string),
			TargetModels: make(map[string]string),
		}, nil
	}
	if err != nil {
		return SlotsConfig{}, err
	}
	var sc SlotsConfig
	if err := json.Unmarshal(data, &sc); err != nil {
		return SlotsConfig{}, err
	}
	if sc.Slots == nil {
		sc.Slots = []Slot{}
	}
	if sc.TargetSlots == nil {
		sc.TargetSlots = make(map[string]string)
	}
	if sc.TargetModels == nil {
		sc.TargetModels = make(map[string]string)
	}
	return sc, nil
}

// SaveSlots writes the slots config to disk atomically (temp file + rename).
func SaveSlots(sc SlotsConfig) error {
	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(SlotsPath(), data, 0644)
}

// -- Legacy routing support (migration only) ---------------------------------

// routingPath returns the path to the legacy omp-routing.json file.
func routingPath() string {
	return filepath.Join(ConfigDir(), "omp-routing.json")
}

// legacyRoutingConfig is the old on-disk format, used only for migration.
type legacyRoutingConfig struct {
	RoleModels  map[string]string `json:"role_models"`
	TargetRoles map[string]string `json:"target_roles"`
}

// loadLegacyRouting reads the old omp-routing.json for migration purposes.
func loadLegacyRouting() (legacyRoutingConfig, error) {
	data, err := os.ReadFile(routingPath())
	if err != nil {
		return legacyRoutingConfig{}, err
	}
	var rc legacyRoutingConfig
	if err := json.Unmarshal(data, &rc); err != nil {
		return legacyRoutingConfig{}, err
	}
	if rc.RoleModels == nil {
		rc.RoleModels = make(map[string]string)
	}
	if rc.TargetRoles == nil {
		rc.TargetRoles = make(map[string]string)
	}
	return rc, nil
}

// roleToSlotID maps the old 5-role taxonomy to slot IDs.
// brain→slot-1, tasker→slot-2, builder→slot-3, librarian→slot-4, fixer→slot-4.
var roleToSlotID = map[string]string{
	"brain":     "slot-1",
	"tasker":    "slot-2",
	"builder":   "slot-3",
	"librarian": "slot-4",
	"fixer":     "slot-4",
}

// migrationRoleOrder defines the deterministic order for processing roles
// during migration. When multiple roles map to the same slot (librarian and
// fixer both → slot-4), the last role in this order wins. This ensures
// consistent migration results regardless of Go map iteration order.
var migrationRoleOrder = []string{"brain", "tasker", "builder", "librarian", "fixer"}

// MigrateRoutingToSlots migrates omp-routing.json to omp-slots.json if:
//   - omp-routing.json exists, AND
//   - omp-slots.json does NOT exist
//
// Returns (true, nil) if migration was performed, (false, nil) if skipped,
// or (false, err) on error.
func MigrateRoutingToSlots() (bool, error) {
	rPath := routingPath()
	slotsPath := SlotsPath()

	// Skip if slots file already exists.
	if _, err := os.Stat(slotsPath); err == nil {
		return false, nil
	}

	// Skip if routing file doesn't exist.
	if _, err := os.Stat(rPath); os.IsNotExist(err) {
		return false, nil
	}

	// Load old routing config.
	rc, err := loadLegacyRouting()
	if err != nil {
		return false, fmt.Errorf("reading omp-routing.json for migration: %w", err)
	}

	// Build new slots config from defaults.
	sc := DefaultSlotsConfig()

	// Map role models → slot models in deterministic order.
	// When multiple roles map to the same slot (librarian/fixer → slot-4),
	// the last role in migrationRoleOrder wins consistently.
	slotByID := make(map[string]*Slot, len(sc.Slots))
	for i := range sc.Slots {
		slotByID[sc.Slots[i].ID] = &sc.Slots[i]
	}
	for _, role := range migrationRoleOrder {
		model, ok := rc.RoleModels[role]
		if !ok {
			continue
		}
		if slotID, ok := roleToSlotID[role]; ok {
			if s, ok := slotByID[slotID]; ok {
				s.Model = model
			}
		}
	}

	// Map target roles → target slots.
	for target, role := range rc.TargetRoles {
		if slotID, ok := roleToSlotID[role]; ok {
			sc.TargetSlots[target] = slotID
		}
	}

	// Save new slots config.
	if err := SaveSlots(sc); err != nil {
		return false, fmt.Errorf("saving omp-slots.json during migration: %w", err)
	}

	// Rename omp-routing.json to omp-routing.json.migrated as a backup breadcrumb.
	// Non-fatal: if rename fails, log and continue — the migration already succeeded.
	migratedPath := rPath + ".migrated"
	if err := os.Rename(rPath, migratedPath); err != nil {
		log.Printf("omp: could not rename %s to %s (migration succeeded, old file remains): %v",
			rPath, migratedPath, err)
	}

	return true, nil
}

// ApplySlots writes model preferences to opencode.json for all targets that
// have either a direct model assignment (TargetModels) or a slot assignment
// with a mapped slot model. Direct target models override slot mapping.
// Targets without any effective mapping are left unchanged.
// Only writes to targets that already exist in opencode.json.
func ApplySlots(sc SlotsConfig, targets []Target) error {
	configPath := ConfigPath()
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	// Build slot ID → model lookup for O(1) access.
	slotModels := make(map[string]string, len(sc.Slots))
	for _, s := range sc.Slots {
		slotModels[s.ID] = s.Model
	}

	updated := raw
	for _, t := range targets {
		model := ""

		if direct, ok := sc.TargetModels[t.Name]; ok && direct != "" {
			model = direct
		} else {
			slotID, hasSlot := sc.TargetSlots[t.Name]
			if !hasSlot {
				continue
			}
			slotModel, hasModel := slotModels[slotID]
			if !hasModel || slotModel == "" {
				continue
			}
			model = slotModel
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

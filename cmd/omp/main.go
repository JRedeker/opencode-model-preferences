package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/anomalyco/opencode-model-preferences/internal/config"
	"github.com/anomalyco/opencode-model-preferences/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

var runProgram = func(m tea.Model) (tea.Model, error) {
	p := tea.NewProgram(m, tea.WithAltScreen())
	return p.Run()
}

func main() {
	if err := run(os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run is the testable entry point. It writes errors to w and returns them so
// callers can inspect without os.Exit coupling.
func run(w io.Writer) error {
	// Refresh the provider model registry before loading config so the picker
	// always shows the latest available models.
	if err := config.RefreshModels(); err != nil {
		return fmt.Errorf("refreshing models: %w", err)
	}

	state, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(state.Models) == 0 {
		return fmt.Errorf(
			"no models found via opencode CLI or provider config\n" +
				"  Ensure 'opencode models' works, or configure providers in ~/.config/opencode/opencode.json",
		)
	}

	// Migrate from legacy omp-routing.json to omp-slots.json if needed.
	if migrated, err := config.MigrateRoutingToSlots(); err != nil {
		// Migration failure is non-fatal: log and continue with fresh slots.
		log.Printf("omp: migration from omp-routing.json failed (continuing with defaults): %v", err)
	} else if migrated {
		log.Printf("omp: migrated omp-routing.json → omp-slots.json")
	}

	slots, err := config.LoadSlots()
	if err != nil {
		return fmt.Errorf("loading slots config: %w", err)
	}

	// If no slots defined yet, initialize with defaults.
	if len(slots.Slots) == 0 {
		slots = config.DefaultSlotsConfig()
	}

	m := tui.New(state, slots)
	if _, err := runProgram(m); err != nil {
		if errors.Is(err, tea.ErrInterrupted) || errors.Is(err, tea.ErrProgramKilled) {
			return nil
		}
		return fmt.Errorf("running TUI: %w", err)
	}
	return nil
}

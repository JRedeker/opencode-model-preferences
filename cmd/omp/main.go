package main

import (
	"fmt"
	"io"
	"os"

	"github.com/anomalyco/opencode-model-preferences/internal/config"
	"github.com/anomalyco/opencode-model-preferences/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

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
			"no models found in provider registry\n" +
				"  Configure providers in ~/.config/opencode/opencode.json",
		)
	}

	m := tui.New(state)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}
	return nil
}

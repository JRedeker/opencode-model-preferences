package tui

import (
	"testing"

	"github.com/anomalyco/opencode-model-preferences/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdate_WindowSizeBeforeModelSelectionDoesNotPanic(t *testing.T) {
	m := New(&config.State{
		Targets: []config.Target{
			{Name: "build", Kind: config.KindAgent, Mode: "primary"},
		},
	})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Update panicked on initial window size message: %v", r)
		}
	}()

	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
}

func TestUpdate_WindowSizeWithEmptyTargetsDoesNotPanic(t *testing.T) {
	m := New(&config.State{})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Update panicked with empty targets on window size: %v", r)
		}
	}()

	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
}

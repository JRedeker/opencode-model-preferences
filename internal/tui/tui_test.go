package tui

import (
	"strings"
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

func TestView_80x80_TargetListRendersKeyContent(t *testing.T) {
	m := New(&config.State{
		Targets: []config.Target{
			{Name: "build", Kind: config.KindAgent, Mode: "primary"},
		},
	})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 80})
	rendered := updated.(Model).View()

	if !strings.Contains(rendered, "Model Preferences") {
		t.Fatalf("expected target list title in 80x80 render")
	}
	if !strings.Contains(rendered, "build") {
		t.Fatalf("expected target entry in 80x80 render")
	}
}

func TestView_80x80_ModelPickerRendersModelEntry(t *testing.T) {
	m := New(&config.State{
		Targets: []config.Target{
			{Name: "build", Kind: config.KindAgent, Mode: "primary"},
		},
		Models: []config.Model{
			{ID: "openai/gpt-5.3-codex", Name: "GPT 5.3 Codex"},
		},
	})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 80})
	m2 := updated.(Model)
	m2.targetList.Select(1)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rendered := updated.(Model).View()

	if !strings.Contains(rendered, "Select model for: build") {
		t.Fatalf("expected model picker title in 80x80 render")
	}
	if !strings.Contains(rendered, "openai/gpt-5.3-codex") {
		t.Fatalf("expected model entry in 80x80 render")
	}
}

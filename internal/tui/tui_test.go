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

func TestBuildTargetItems_HiddenAgentsSeparateSection(t *testing.T) {
	targets := []config.Target{
		{Name: "build", Kind: config.KindAgent, Mode: "primary"},
		{Name: "general", Kind: config.KindAgent, Mode: "subagent"},
		{Name: "adv-researcher", Kind: config.KindAgent, Mode: "subagent", Hidden: true},
	}

	items := buildTargetItems(targets)

	// Collect section headers and agent names in order
	var sections []string
	var names []string
	for _, item := range items {
		switch v := item.(type) {
		case sectionItem:
			sections = append(sections, v.label)
		case targetItem:
			names = append(names, v.target.Name)
		}
	}

	// Expect three sections: Agents, Sub-Agents, Hidden Agents
	if len(sections) != 3 {
		t.Fatalf("expected 3 section headers, got %d: %v", len(sections), sections)
	}
	if sections[0] != "Agents" {
		t.Errorf("sections[0] = %q, want Agents", sections[0])
	}
	if sections[1] != "Sub-Agents" {
		t.Errorf("sections[1] = %q, want Sub-Agents", sections[1])
	}
	if sections[2] != "Hidden Agents" {
		t.Errorf("sections[2] = %q, want Hidden Agents", sections[2])
	}

	// adv-researcher should appear after general
	generalIdx := -1
	advIdx := -1
	for i, n := range names {
		if n == "general" {
			generalIdx = i
		}
		if n == "adv-researcher" {
			advIdx = i
		}
	}
	if generalIdx < 0 || advIdx < 0 {
		t.Fatalf("expected both general and adv-researcher in names, got %v", names)
	}
	if advIdx <= generalIdx {
		t.Errorf("adv-researcher (%d) should appear after general (%d)", advIdx, generalIdx)
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

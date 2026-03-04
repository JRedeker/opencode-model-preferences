package tui

import (
	"strings"
	"testing"

	"github.com/anomalyco/opencode-model-preferences/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdate_WindowSizeDoesNotPanic(t *testing.T) {
	m := New(&config.State{}, config.RoutingConfig{})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Update panicked on window size message: %v", r)
		}
	}()

	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
}

func TestUpdate_WindowSizeWithEmptyStateDoesNotPanic(t *testing.T) {
	m := New(&config.State{}, config.RoutingConfig{})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Update panicked with empty state: %v", r)
		}
	}()

	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
}

func TestView_80x80_MappingListRendersTitle(t *testing.T) {
	m := New(&config.State{}, config.RoutingConfig{
		Mappings: []config.Mapping{
			{Name: "quality", Orchestrator: "anthropic/claude-opus-4", Worker: "anthropic/claude-haiku-4"},
		},
	})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 80})
	rendered := updated.(Model).View()

	if !strings.Contains(rendered, "Model Routing") {
		t.Fatalf("expected 'Model Routing' title in render, got:\n%s", rendered)
	}
}

func TestView_80x80_MappingListShowsMappingName(t *testing.T) {
	m := New(&config.State{}, config.RoutingConfig{
		Mappings: []config.Mapping{
			{Name: "quality", Orchestrator: "anthropic/claude-opus-4", Worker: "anthropic/claude-haiku-4"},
		},
	})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 80})
	rendered := updated.(Model).View()

	if !strings.Contains(rendered, "quality") {
		t.Fatalf("expected mapping name 'quality' in render, got:\n%s", rendered)
	}
}

func TestBuildMappingItems_EmptyRoutingHasNewMappingEntry(t *testing.T) {
	items := buildMappingItems(config.RoutingConfig{})

	var hasNew bool
	for _, item := range items {
		if _, ok := item.(newMappingItem); ok {
			hasNew = true
		}
	}
	if !hasNew {
		t.Error("expected newMappingItem in empty routing list")
	}
}

func TestBuildMappingItems_MappingsAppearBeforeNewEntry(t *testing.T) {
	rc := config.RoutingConfig{
		Mappings: []config.Mapping{
			{Name: "fast", Orchestrator: "openai/gpt-4o-mini", Worker: "openai/gpt-4o-mini"},
		},
	}
	items := buildMappingItems(rc)

	var mappingIdx, newIdx int
	for i, item := range items {
		switch item.(type) {
		case mappingItem:
			mappingIdx = i
		case newMappingItem:
			newIdx = i
		}
	}
	if mappingIdx >= newIdx {
		t.Errorf("mapping item (%d) should appear before new mapping item (%d)", mappingIdx, newIdx)
	}
}

// TestRefreshTargetModels verifies that after activation, in-memory target
// model values are updated to reflect the applied mapping. This prevents
// subsequent activation attempts from showing false "will overwrite" warnings.
func TestRefreshTargetModels_UpdatesInMemoryState(t *testing.T) {
	state := &config.State{
		Targets: []config.Target{
			{Name: "build", Kind: config.KindAgent, Mode: "primary", Model: "old/orchestrator"},
			{Name: "general", Kind: config.KindAgent, Mode: "subagent", Model: "old/worker"},
		},
	}
	m := New(state, config.RoutingConfig{})

	applied := config.Mapping{
		Name:         "quality",
		Orchestrator: "anthropic/claude-opus-4",
		Worker:       "anthropic/claude-haiku-4",
	}
	m.refreshTargetModels(applied)

	if m.state.Targets[0].Model != "anthropic/claude-opus-4" {
		t.Errorf("orchestrator target model = %q, want anthropic/claude-opus-4", m.state.Targets[0].Model)
	}
	if m.state.Targets[1].Model != "anthropic/claude-haiku-4" {
		t.Errorf("worker target model = %q, want anthropic/claude-haiku-4", m.state.Targets[1].Model)
	}
}

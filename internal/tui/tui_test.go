package tui

import (
	"strings"
	"testing"

	"github.com/anomalyco/opencode-model-preferences/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdate_WindowSizeDoesNotPanic(t *testing.T) {
	m := New(&config.State{}, config.PreferencesConfig{
		TargetModels: map[string]string{},
	})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Update panicked on window size message: %v", r)
		}
	}()

	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
}

func TestView_AssignmentsView_ShowsTitle(t *testing.T) {
	state := &config.State{
		Targets: []config.Target{
			{Name: "build", Kind: config.KindAgent, Mode: "primary", Model: "anthropic/claude-opus-4"},
		},
	}
	m := New(state, config.PreferencesConfig{
		TargetModels: map[string]string{},
	})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 80})
	rendered := updated.(Model).View()

	if !strings.Contains(rendered, "Agents") {
		t.Fatalf("expected 'Agents' in render, got:\n%s", rendered)
	}
}

func TestView_AssignmentsView_ShowsAgentName(t *testing.T) {
	state := &config.State{
		Targets: []config.Target{
			{Name: "build", Kind: config.KindAgent, Mode: "primary", Model: "anthropic/claude-opus-4"},
		},
	}
	m := New(state, config.PreferencesConfig{
		TargetModels: map[string]string{},
	})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 80})
	rendered := updated.(Model).View()

	if !strings.Contains(rendered, "build") {
		t.Fatalf("expected agent name 'build' in render, got:\n%s", rendered)
	}
}

func TestView_AssignmentsView_ShowsAssignedModel(t *testing.T) {
	state := &config.State{
		Targets: []config.Target{
			{Name: "build", Kind: config.KindAgent, Mode: "primary"},
		},
	}
	prefs := config.PreferencesConfig{
		TargetModels: map[string]string{"build": "anthropic/claude-opus-4"},
	}
	m := New(state, prefs)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 80})
	rendered := updated.(Model).View()

	if !strings.Contains(rendered, "anthropic/claude-opus-4") {
		t.Fatalf("expected model 'anthropic/claude-opus-4' in render, got:\n%s", rendered)
	}
}

func TestBuildTargetItems_HiddenAgentsExcluded(t *testing.T) {
	targets := []config.Target{
		{Name: "build", Kind: config.KindAgent},
		{Name: "hidden-agent", Kind: config.KindAgent, Hidden: true},
	}
	prefs := config.PreferencesConfig{
		TargetModels: map[string]string{},
	}
	items := buildTargetItems(targets, prefs)

	for _, item := range items {
		if ti, ok := item.(targetItem); ok && ti.target.Name == "hidden-agent" {
			t.Error("hidden agent should not appear in target items")
		}
	}
}

func TestBuildTargetItems_SeparatesAgentsAndCommands(t *testing.T) {
	targets := []config.Target{
		{Name: "build", Kind: config.KindAgent},
		{Name: "deploy", Kind: config.KindCommand},
	}
	prefs := config.PreferencesConfig{
		TargetModels: map[string]string{},
	}
	items := buildTargetItems(targets, prefs)

	var sections []string
	for _, item := range items {
		if s, ok := item.(sectionItem); ok {
			sections = append(sections, s.label)
		}
	}
	if len(sections) != 2 || sections[0] != "Agents" || sections[1] != "Commands" {
		t.Errorf("expected [Agents, Commands] sections, got %v", sections)
	}
}

func TestBuildTargetItems_ShowsModelInDescription(t *testing.T) {
	targets := []config.Target{
		{Name: "build", Kind: config.KindAgent, Model: "anthropic/claude-opus-4"},
	}
	prefs := config.PreferencesConfig{
		TargetModels: map[string]string{"build": "openai/gpt-5"},
	}

	items := buildTargetItems(targets, prefs)
	for _, item := range items {
		ti, ok := item.(targetItem)
		if !ok {
			continue
		}
		desc := ti.Description()
		if !strings.Contains(desc, "openai/gpt-5") {
			t.Fatalf("expected model in description, got: %s", desc)
		}
		return
	}
	t.Fatal("target item not found")
}

func TestBuildTargetItems_PendingChangeShown(t *testing.T) {
	targets := []config.Target{
		{Name: "build", Kind: config.KindAgent, Model: "anthropic/claude-opus-4"},
	}
	prefs := config.PreferencesConfig{
		TargetModels: map[string]string{"build": "openai/gpt-5"},
	}

	items := buildTargetItems(targets, prefs)
	for _, item := range items {
		ti, ok := item.(targetItem)
		if !ok {
			continue
		}
		desc := ti.Description()
		// Should show pending change since pref differs from current
		if !strings.Contains(desc, "pending") {
			t.Fatalf("expected 'pending' in description when model differs, got: %s", desc)
		}
		return
	}
	t.Fatal("target item not found")
}

func TestBuildModelPickItems_IncludesClearOption(t *testing.T) {
	models := []config.Model{
		{ID: "anthropic/claude-opus-4", Provider: "anthropic", Name: "Claude Opus 4"},
	}
	items := buildModelPickItems(models)

	if len(items) != 2 {
		t.Fatalf("expected 2 items (clear + 1 model), got %d", len(items))
	}
	first, ok := items[0].(pickItem)
	if !ok {
		t.Fatal("first item should be a pickItem")
	}
	if first.value != "" {
		t.Errorf("first item value should be empty (clear option), got %q", first.value)
	}
}

func TestView_AssignmentsView_ShowsKeyHints(t *testing.T) {
	state := &config.State{
		Targets: []config.Target{{Name: "build", Kind: config.KindAgent}},
	}
	prefs := config.PreferencesConfig{TargetModels: map[string]string{}}
	m := New(state, prefs)

	rendered := m.View()
	if !strings.Contains(rendered, "set model") {
		t.Fatalf("expected 'set model' hint in view, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "d: clear") {
		t.Fatalf("expected 'd: clear' hint in view, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "a: apply") {
		t.Fatalf("expected 'a: apply' hint in view, got:\n%s", rendered)
	}
}

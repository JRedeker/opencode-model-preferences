package tui

import (
	"strings"
	"testing"

	"github.com/anomalyco/opencode-model-preferences/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdate_WindowSizeDoesNotPanic(t *testing.T) {
	m := New(&config.State{}, config.RoutingConfig{
		RoleModels:  map[config.UserRole]string{},
		TargetRoles: map[string]config.UserRole{},
	})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Update panicked on window size message: %v", r)
		}
	}()

	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
}

func TestView_AgentsView_ShowsTitle(t *testing.T) {
	state := &config.State{
		Targets: []config.Target{
			{Name: "build", Kind: config.KindAgent, Mode: "primary", Model: "anthropic/claude-opus-4"},
		},
	}
	m := New(state, config.RoutingConfig{
		RoleModels:  map[config.UserRole]string{},
		TargetRoles: map[string]config.UserRole{},
	})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 80})
	rendered := updated.(Model).View()

	if !strings.Contains(rendered, "Agents") {
		t.Fatalf("expected 'Agents' in render, got:\n%s", rendered)
	}
}

func TestView_AgentsView_ShowsAgentName(t *testing.T) {
	state := &config.State{
		Targets: []config.Target{
			{Name: "build", Kind: config.KindAgent, Mode: "primary", Model: "anthropic/claude-opus-4"},
		},
	}
	m := New(state, config.RoutingConfig{
		RoleModels:  map[config.UserRole]string{},
		TargetRoles: map[string]config.UserRole{},
	})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 80})
	rendered := updated.(Model).View()

	if !strings.Contains(rendered, "build") {
		t.Fatalf("expected agent name 'build' in render, got:\n%s", rendered)
	}
}

func TestView_AgentsView_ShowsAssignedRole(t *testing.T) {
	state := &config.State{
		Targets: []config.Target{
			{Name: "build", Kind: config.KindAgent, Mode: "primary"},
		},
	}
	routing := config.RoutingConfig{
		RoleModels:  map[config.UserRole]string{config.RoleBrain: "anthropic/claude-opus-4"},
		TargetRoles: map[string]config.UserRole{"build": config.RoleBrain},
	}
	m := New(state, routing)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 80})
	rendered := updated.(Model).View()

	if !strings.Contains(rendered, "brain") {
		t.Fatalf("expected role 'brain' in render, got:\n%s", rendered)
	}
}

func TestBuildTargetItems_HiddenAgentsExcluded(t *testing.T) {
	targets := []config.Target{
		{Name: "build", Kind: config.KindAgent},
		{Name: "hidden-agent", Kind: config.KindAgent, Hidden: true},
	}
	routing := config.RoutingConfig{
		RoleModels:  map[config.UserRole]string{},
		TargetRoles: map[string]config.UserRole{},
	}
	items := buildTargetItems(targets, routing)

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
	routing := config.RoutingConfig{
		RoleModels:  map[config.UserRole]string{},
		TargetRoles: map[string]config.UserRole{},
	}
	items := buildTargetItems(targets, routing)

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

func TestBuildRoleItems_AllFiveRoles(t *testing.T) {
	routing := config.RoutingConfig{
		RoleModels:  map[config.UserRole]string{},
		TargetRoles: map[string]config.UserRole{},
	}
	items := buildRoleItems(routing)

	if len(items) != 5 {
		t.Fatalf("expected 5 role items, got %d", len(items))
	}
	names := make(map[string]bool)
	for _, item := range items {
		if ri, ok := item.(roleItem); ok {
			names[string(ri.role)] = true
		}
	}
	for _, r := range config.AllUserRoles() {
		if !names[string(r)] {
			t.Errorf("missing role %q in role items", r)
		}
	}
}

func TestBuildRoleItems_ShowsMappedModel(t *testing.T) {
	routing := config.RoutingConfig{
		RoleModels:  map[config.UserRole]string{config.RoleBrain: "anthropic/claude-opus-4"},
		TargetRoles: map[string]config.UserRole{},
	}
	items := buildRoleItems(routing)

	for _, item := range items {
		if ri, ok := item.(roleItem); ok && ri.role == config.RoleBrain {
			if ri.model != "anthropic/claude-opus-4" {
				t.Errorf("brain role model = %q, want anthropic/claude-opus-4", ri.model)
			}
			return
		}
	}
	t.Error("brain role not found in items")
}

func TestBuildRoleItems_UnmappedShowsPlaceholder(t *testing.T) {
	routing := config.RoutingConfig{
		RoleModels:  map[config.UserRole]string{},
		TargetRoles: map[string]config.UserRole{},
	}
	items := buildRoleItems(routing)

	for _, item := range items {
		if ri, ok := item.(roleItem); ok {
			if ri.model != "(unmapped)" {
				t.Errorf("role %q model = %q, want (unmapped)", ri.role, ri.model)
			}
		}
	}
}

package tui

import (
	"strings"
	"testing"

	"github.com/anomalyco/opencode-model-preferences/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdate_WindowSizeDoesNotPanic(t *testing.T) {
	m := New(&config.State{}, config.SlotsConfig{
		Slots:       []config.Slot{},
		TargetSlots: map[string]string{},
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
	m := New(state, config.SlotsConfig{
		Slots:       []config.Slot{},
		TargetSlots: map[string]string{},
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
	m := New(state, config.SlotsConfig{
		Slots:       []config.Slot{},
		TargetSlots: map[string]string{},
	})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 80})
	rendered := updated.(Model).View()

	if !strings.Contains(rendered, "build") {
		t.Fatalf("expected agent name 'build' in render, got:\n%s", rendered)
	}
}

func TestView_AssignmentsView_ShowsAssignedSlot(t *testing.T) {
	state := &config.State{
		Targets: []config.Target{
			{Name: "build", Kind: config.KindAgent, Mode: "primary"},
		},
	}
	slots := config.SlotsConfig{
		Slots:       []config.Slot{{ID: "slot-1", Name: "Brain", Model: "anthropic/claude-opus-4"}},
		TargetSlots: map[string]string{"build": "slot-1"},
	}
	m := New(state, slots)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 80})
	rendered := updated.(Model).View()

	if !strings.Contains(rendered, "Brain") {
		t.Fatalf("expected slot name 'Brain' in render, got:\n%s", rendered)
	}
}

func TestBuildTargetItems_HiddenAgentsExcluded(t *testing.T) {
	targets := []config.Target{
		{Name: "build", Kind: config.KindAgent},
		{Name: "hidden-agent", Kind: config.KindAgent, Hidden: true},
	}
	slots := config.SlotsConfig{
		Slots:       []config.Slot{},
		TargetSlots: map[string]string{},
	}
	items := buildTargetItems(targets, slots)

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
	slots := config.SlotsConfig{
		Slots:       []config.Slot{},
		TargetSlots: map[string]string{},
	}
	items := buildTargetItems(targets, slots)

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

func TestBuildSlotItems_AllSlots(t *testing.T) {
	slots := config.SlotsConfig{
		Slots: []config.Slot{
			{ID: "slot-1", Name: "Brain", Model: "anthropic/claude-opus-4"},
			{ID: "slot-2", Name: "Worker", Model: ""},
			{ID: "slot-3", Name: "Fast", Model: "anthropic/claude-haiku-4"},
			{ID: "slot-4", Name: "Cheap", Model: ""},
		},
		TargetSlots: map[string]string{},
	}
	items := buildSlotItems(slots)

	if len(items) != 4 {
		t.Fatalf("expected 4 slot items, got %d", len(items))
	}
	names := make(map[string]bool)
	for _, item := range items {
		if si, ok := item.(slotItem); ok {
			names[si.slot.Name] = true
		}
	}
	for _, s := range slots.Slots {
		if !names[s.Name] {
			t.Errorf("missing slot %q in slot items", s.Name)
		}
	}
}

func TestBuildSlotItems_ShowsMappedModel(t *testing.T) {
	slots := config.SlotsConfig{
		Slots:       []config.Slot{{ID: "slot-1", Name: "Brain", Model: "anthropic/claude-opus-4"}},
		TargetSlots: map[string]string{},
	}
	items := buildSlotItems(slots)

	for _, item := range items {
		if si, ok := item.(slotItem); ok && si.slot.ID == "slot-1" {
			if si.slot.Model != "anthropic/claude-opus-4" {
				t.Errorf("slot-1 model = %q, want anthropic/claude-opus-4", si.slot.Model)
			}
			return
		}
	}
	t.Error("slot-1 not found in items")
}

func TestBuildSlotItems_UnmappedShowsPlaceholder(t *testing.T) {
	slots := config.SlotsConfig{
		Slots:       []config.Slot{{ID: "slot-1", Name: "Brain", Model: ""}},
		TargetSlots: map[string]string{},
	}
	items := buildSlotItems(slots)

	for _, item := range items {
		if si, ok := item.(slotItem); ok {
			desc := si.Description()
			if !strings.Contains(desc, "(unmapped)") {
				t.Errorf("slot %q description = %q, want to contain (unmapped)", si.slot.Name, desc)
			}
		}
	}
}

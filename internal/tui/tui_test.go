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

func TestNextSlotID_UsesMaxSuffixPlusOne(t *testing.T) {
	slots := []config.Slot{
		{ID: "slot-1", Name: "Slot 1"},
		{ID: "slot-4", Name: "Slot 4"},
		{ID: "custom", Name: "Custom"},
	}

	got := nextSlotID(slots)
	if got != "slot-5" {
		t.Fatalf("nextSlotID() = %q, want slot-5", got)
	}
}

func TestAddSlot_AppendsNewSlotWithDefaultName(t *testing.T) {
	sc := config.SlotsConfig{
		Slots: []config.Slot{
			{ID: "slot-1", Name: "Slot 1"},
			{ID: "slot-2", Name: "Slot 2"},
		},
		TargetSlots: map[string]string{},
	}

	added := addSlot(&sc)

	if added.ID != "slot-3" {
		t.Fatalf("added.ID = %q, want slot-3", added.ID)
	}
	if added.Name != "Slot 3" {
		t.Fatalf("added.Name = %q, want Slot 3", added.Name)
	}
	if len(sc.Slots) != 3 {
		t.Fatalf("len(sc.Slots) = %d, want 3", len(sc.Slots))
	}
}

func TestRemoveSlot_RemovesAssignmentsToDeletedSlot(t *testing.T) {
	sc := config.SlotsConfig{
		Slots: []config.Slot{
			{ID: "slot-1", Name: "Slot 1"},
			{ID: "slot-2", Name: "Slot 2"},
		},
		TargetSlots: map[string]string{
			"build":   "slot-1",
			"general": "slot-1",
			"deploy":  "slot-2",
		},
	}

	removed, cleared := removeSlot(&sc, "slot-1")
	if !removed {
		t.Fatal("expected removed=true")
	}
	if cleared != 2 {
		t.Fatalf("cleared = %d, want 2", cleared)
	}
	if len(sc.Slots) != 1 || sc.Slots[0].ID != "slot-2" {
		t.Fatalf("remaining slots = %#v, want only slot-2", sc.Slots)
	}
	if _, ok := sc.TargetSlots["build"]; ok {
		t.Fatal("build assignment should be removed")
	}
	if _, ok := sc.TargetSlots["general"]; ok {
		t.Fatal("general assignment should be removed")
	}
	if got := sc.TargetSlots["deploy"]; got != "slot-2" {
		t.Fatalf("deploy assignment = %q, want slot-2", got)
	}
}

func TestRenameSlot_UpdatesMatchingSlotName(t *testing.T) {
	sc := config.SlotsConfig{
		Slots: []config.Slot{{ID: "slot-1", Name: "Slot 1"}},
	}

	ok := renameSlot(&sc, "slot-1", "Fast Lane")
	if !ok {
		t.Fatal("expected rename to succeed")
	}
	if sc.Slots[0].Name != "Fast Lane" {
		t.Fatalf("slot name = %q, want Fast Lane", sc.Slots[0].Name)
	}
}

func TestView_SlotsView_ShowsRenameAddRemoveHints(t *testing.T) {
	state := &config.State{}
	slots := config.SlotsConfig{
		Slots: []config.Slot{{ID: "slot-1", Name: "Slot 1"}},
	}
	m := New(state, slots)
	m.view = viewSlots

	rendered := m.View()
	if !strings.Contains(rendered, "r: rename slot") {
		t.Fatalf("expected rename hint in slots view, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "n: add slot") {
		t.Fatalf("expected add hint in slots view, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "x: remove slot") {
		t.Fatalf("expected remove hint in slots view, got:\n%s", rendered)
	}
}

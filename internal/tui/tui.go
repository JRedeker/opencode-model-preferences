// Package tui implements the Bubbletea TUI for model preference selection.
//
// Flow: main list (agents/commands) → model picker → write to config → back to list.
package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/anomalyco/opencode-model-preferences/internal/config"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// -- Styles ------------------------------------------------------------------

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6E3A1")).
			Italic(true)

	sectionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true).
			PaddingTop(1)
)

// -- List items --------------------------------------------------------------

// sectionItem is a non-selectable section header in the target list.
type sectionItem struct{ label string }

func (s sectionItem) Title() string       { return s.label }
func (s sectionItem) Description() string { return "" }
func (s sectionItem) FilterValue() string { return "" }

// targetItem wraps a config.Target for the list.
type targetItem struct {
	target config.Target
}

func (t targetItem) Title() string { return t.target.Name }
func (t targetItem) Description() string {
	var parts []string
	if t.target.Kind == config.KindAgent {
		parts = append(parts, t.target.Mode)
		if t.target.Locked {
			parts = append(parts, "[locked]")
		}
	} else {
		parts = append(parts, "command")
	}
	if t.target.Model != "" {
		parts = append(parts, "model: "+t.target.Model)
	} else {
		parts = append(parts, "model: (default)")
	}
	return strings.Join(parts, " | ")
}
func (t targetItem) FilterValue() string { return t.target.Name }

// modelItem wraps a config.Model for the picker.
type modelItem struct {
	model    config.Model
	isClear  bool // special "clear" entry
	selected bool // currently assigned
}

func (m modelItem) Title() string {
	if m.isClear {
		return "(clear preference)"
	}
	return m.model.ID
}
func (m modelItem) Description() string {
	if m.isClear {
		return "Revert to session default"
	}
	desc := m.model.Name
	if m.selected {
		desc += " (current)"
	}
	return desc
}
func (m modelItem) FilterValue() string {
	if m.isClear {
		return "clear none default"
	}
	return m.model.ID + " " + m.model.Name
}

// buildTargetItems constructs the ordered, section-headed list items:
// Agents → Sub-Agents → Hidden Agents → Other (commands).
func buildTargetItems(targets []config.Target) []list.Item {
	var primary, subagent, hidden, commands []config.Target
	for _, t := range targets {
		switch {
		case t.Kind == config.KindAgent && t.Hidden:
			hidden = append(hidden, t)
		case t.Kind == config.KindAgent && (t.Mode == "primary" || t.Mode == "all"):
			primary = append(primary, t)
		case t.Kind == config.KindAgent && t.Mode == "subagent":
			subagent = append(subagent, t)
		default:
			commands = append(commands, t)
		}
	}

	var items []list.Item
	if len(primary) > 0 {
		items = append(items, sectionItem{"Agents"})
		for _, t := range primary {
			items = append(items, targetItem{target: t})
		}
	}
	if len(subagent) > 0 {
		items = append(items, sectionItem{"Sub-Agents"})
		for _, t := range subagent {
			items = append(items, targetItem{target: t})
		}
	}
	if len(hidden) > 0 {
		items = append(items, sectionItem{"Hidden Agents"})
		for _, t := range hidden {
			items = append(items, targetItem{target: t})
		}
	}
	if len(commands) > 0 {
		items = append(items, sectionItem{"Other"})
		for _, t := range commands {
			items = append(items, targetItem{target: t})
		}
	}
	return items
}

// -- Delegate ----------------------------------------------------------------

// itemDelegate wraps the default delegate but renders sectionItems as
// styled, non-selectable section headers instead of normal list rows.
type itemDelegate struct {
	inner list.DefaultDelegate
}

func newDelegate() itemDelegate {
	return itemDelegate{inner: list.NewDefaultDelegate()}
}

func (d itemDelegate) Height() int                               { return d.inner.Height() }
func (d itemDelegate) Spacing() int                              { return d.inner.Spacing() }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return d.inner.Update(msg, m) }
func (d itemDelegate) ShortHelp() []key.Binding                  { return d.inner.ShortHelp() }
func (d itemDelegate) FullHelp() [][]key.Binding                 { return d.inner.FullHelp() }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	if s, ok := item.(sectionItem); ok {
		if m.Width() <= 0 {
			return
		}
		label := ansi.Truncate(s.label, m.Width()-4, "…")
		rendered := sectionStyle.Render("── " + label + " ──")
		fmt.Fprint(w, rendered) //nolint: errcheck
		return
	}
	d.inner.Render(w, m, index, item)
}

// -- View state --------------------------------------------------------------

type viewState int

const (
	viewTargets viewState = iota
	viewModels
)

// -- Model -------------------------------------------------------------------

type Model struct {
	state      *config.State
	view       viewState
	targetList list.Model
	modelList  list.Model
	selected   *config.Target // the target we're configuring
	status     string         // status message after writes
	width      int
	height     int
}

// New creates the initial TUI model.
func New(state *config.State) Model {
	items := buildTargetItems(state.Targets)

	delegate := newDelegate()
	targetList := list.New(items, delegate, 0, 0)
	targetList.Title = "Model Preferences"
	targetList.Styles.Title = titleStyle
	targetList.SetShowStatusBar(true)
	targetList.SetFilteringEnabled(true)

	modelList := list.New([]list.Item{}, delegate, 0, 0)
	modelList.SetShowStatusBar(true)
	modelList.SetFilteringEnabled(true)

	return Model{
		state:      state,
		view:       viewTargets,
		targetList: targetList,
		modelList:  modelList,
	}
}

// -- Messages ----------------------------------------------------------------

type writeResultMsg struct {
	err    error
	target string
	model  string
}

type reorderResultMsg struct {
	err error
}

// -- Update ------------------------------------------------------------------

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := appStyle.GetFrameSize()
		m.targetList.SetSize(msg.Width-h, msg.Height-v)
		m.modelList.SetSize(msg.Width-h, msg.Height-v)
		return m, nil

	case writeResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error: %v", msg.err)
		} else if msg.model == "" {
			m.status = fmt.Sprintf("Cleared model for %s", msg.target)
		} else {
			m.status = fmt.Sprintf("Set %s -> %s", msg.target, msg.model)
		}
		// Reload state and rebuild target list
		newState, err := config.Load()
		if err == nil {
			m.state = newState
			m = m.rebuildTargetList()
		}
		m.view = viewTargets
		return m, nil

	case reorderResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error reordering: %v", msg.err)
		} else {
			m.status = "Agent order saved"
		}
		newState, err := config.Load()
		if err == nil {
			m.state = newState
			m = m.rebuildTargetList()
		}
		return m, nil

	case tea.KeyMsg:
		// Don't handle keys when filtering
		if m.view == viewTargets && m.targetList.FilterState() == list.Filtering {
			break
		}
		if m.view == viewModels && m.modelList.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
			if m.view == viewModels {
				m.view = viewTargets
				m.status = ""
				return m, nil
			}
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if m.view == viewModels {
				m.view = viewTargets
				m.status = ""
				return m, nil
			}
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			return m.handleSelect()

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+up"))):
			if m.view == viewTargets {
				return m.handleReorder(-1)
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+down"))):
			if m.view == viewTargets {
				return m.handleReorder(1)
			}
		}
	}

	// Delegate to active list
	var cmd tea.Cmd
	if m.view == viewTargets {
		m.targetList, cmd = m.targetList.Update(msg)
	} else {
		m.modelList, cmd = m.modelList.Update(msg)
	}
	return m, cmd
}

func (m Model) handleSelect() (tea.Model, tea.Cmd) {
	if m.view == viewTargets {
		item, ok := m.targetList.SelectedItem().(targetItem)
		if !ok {
			// sectionItem or nothing selected — do nothing
			return m, nil
		}
		t := item.target
		m.selected = &t
		m.modelList = m.buildModelList(t)
		m.view = viewModels
		m.status = ""
		return m, nil
	}

	// Model selection
	item, ok := m.modelList.SelectedItem().(modelItem)
	if !ok {
		return m, nil
	}

	target := m.selected
	var modelID string
	if !item.isClear {
		modelID = item.model.ID
	}

	// Write async
	return m, func() tea.Msg {
		err := config.SetModel(target.Kind, target.Name, modelID)
		return writeResultMsg{
			err:    err,
			target: target.Name,
			model:  modelID,
		}
	}
}

func (m Model) buildModelList(t config.Target) list.Model {
	var items []list.Item

	// Clear option first
	items = append(items, modelItem{isClear: true})

	// All available models
	for _, mdl := range m.state.Models {
		items = append(items, modelItem{
			model:    mdl,
			selected: mdl.ID == t.Model,
		})
	}

	delegate := newDelegate()
	ml := list.New(items, delegate, 0, 0)
	ml.Title = fmt.Sprintf("Select model for: %s", t.Name)
	ml.Styles.Title = titleStyle
	ml.SetFilteringEnabled(true)
	ml.SetShowStatusBar(true)

	h, v := appStyle.GetFrameSize()
	ml.SetSize(m.width-h, m.height-v)

	return ml
}

func (m Model) handleReorder(delta int) (tea.Model, tea.Cmd) {
	item, ok := m.targetList.SelectedItem().(targetItem)
	if !ok {
		return m, nil
	}
	t := item.target

	// Only moveable primary agents can be reordered
	if t.Kind != config.KindAgent || t.Locked {
		return m, nil
	}
	if t.Mode != "primary" && t.Mode != "all" {
		return m, nil
	}

	// Extract ordered list of moveable agents from current state
	var moveable []config.Target
	for _, tgt := range m.state.Targets {
		if tgt.Kind == config.KindAgent && !tgt.Locked && (tgt.Mode == "primary" || tgt.Mode == "all") {
			moveable = append(moveable, tgt)
		}
	}

	// Find current index
	idx := -1
	for i, tgt := range moveable {
		if tgt.Name == t.Name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return m, nil
	}

	newIdx := idx + delta
	if newIdx < 0 || newIdx >= len(moveable) {
		return m, nil
	}

	// Swap
	moveable[idx], moveable[newIdx] = moveable[newIdx], moveable[idx]

	// Build ordered names for SetAgentOrder (all agents in config, with moveable in new order)
	// Collect all config-keyed agent names preserving non-moveable positions
	names := make([]string, 0, len(moveable))
	for _, tgt := range moveable {
		names = append(names, tgt.Name)
	}

	// Optimistically update state order so UI moves immediately
	newTargets := make([]config.Target, 0, len(m.state.Targets))
	moveableIdx := 0
	for _, tgt := range m.state.Targets {
		if tgt.Kind == config.KindAgent && !tgt.Locked && (tgt.Mode == "primary" || tgt.Mode == "all") {
			newTargets = append(newTargets, moveable[moveableIdx])
			moveableIdx++
		} else {
			newTargets = append(newTargets, tgt)
		}
	}
	m.state.Targets = newTargets
	m = m.rebuildTargetList()

	// Move cursor to follow the item
	items := m.targetList.Items()
	for i, it := range items {
		if ti, ok := it.(targetItem); ok && ti.target.Name == t.Name {
			m.targetList.Select(i)
			break
		}
	}

	return m, func() tea.Msg {
		err := config.SetAgentOrder(names)
		return reorderResultMsg{err: err}
	}
}

func (m Model) rebuildTargetList() Model {
	items := buildTargetItems(m.state.Targets)
	m.targetList.SetItems(items)
	return m
}

// -- View --------------------------------------------------------------------

func (m Model) View() string {
	var content string
	if m.view == viewTargets {
		content = m.targetList.View()
	} else {
		content = m.modelList.View()
	}

	if m.status != "" {
		content += "\n" + statusStyle.Render(m.status)
	}

	return appStyle.Render(content)
}

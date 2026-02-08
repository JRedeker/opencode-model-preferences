// Package tui implements the Bubbletea TUI for model preference selection.
//
// Flow: main list (agents/commands) → model picker → write to config → back to list.
package tui

import (
	"fmt"
	"strings"

	"github.com/anomalyco/opencode-model-preferences/internal/config"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
)

// -- List items --------------------------------------------------------------

// targetItem wraps a config.Target for the list.
type targetItem struct {
	target config.Target
}

func (t targetItem) Title() string { return t.target.Name }
func (t targetItem) Description() string {
	var parts []string
	if t.target.Kind == config.KindAgent {
		parts = append(parts, t.target.Mode)
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
	// Build target list items grouped by section
	var items []list.Item

	// Primary agents
	for _, t := range state.Targets {
		if t.Kind == config.KindAgent && (t.Mode == "primary" || t.Mode == "all") {
			items = append(items, targetItem{target: t})
		}
	}

	// Subagents
	for _, t := range state.Targets {
		if t.Kind == config.KindAgent && t.Mode == "subagent" {
			items = append(items, targetItem{target: t})
		}
	}

	// Commands
	for _, t := range state.Targets {
		if t.Kind == config.KindCommand {
			items = append(items, targetItem{target: t})
		}
	}

	delegate := list.NewDefaultDelegate()
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

	delegate := list.NewDefaultDelegate()
	ml := list.New(items, delegate, 0, 0)
	ml.Title = fmt.Sprintf("Select model for: %s", t.Name)
	ml.Styles.Title = titleStyle
	ml.SetFilteringEnabled(true)
	ml.SetShowStatusBar(true)

	h, v := appStyle.GetFrameSize()
	ml.SetSize(m.width-h, m.height-v)

	return ml
}

func (m Model) rebuildTargetList() Model {
	var items []list.Item

	for _, t := range m.state.Targets {
		if t.Kind == config.KindAgent && (t.Mode == "primary" || t.Mode == "all") {
			items = append(items, targetItem{target: t})
		}
	}
	for _, t := range m.state.Targets {
		if t.Kind == config.KindAgent && t.Mode == "subagent" {
			items = append(items, targetItem{target: t})
		}
	}
	for _, t := range m.state.Targets {
		if t.Kind == config.KindCommand {
			items = append(items, targetItem{target: t})
		}
	}

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

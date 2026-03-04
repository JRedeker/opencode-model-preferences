// Package tui implements the Bubbletea TUI for model routing management.
//
// Three views, all using bubbles/list for consistent UX:
//   - Agents view: list of agents/commands with current model and assigned role.
//     Press 'r' to assign a role, 'a' to apply routing to opencode.json.
//   - Roles view: list of 5 roles with their mapped model.
//     Press 'enter' to pick a model for a role.
//   - Picker view: full list.Model for selecting a role or model, with filtering.
//
// Roles are purely for model mapping — they carry no context or system prompt.
// If a role has no model mapped, targets assigned to it keep their existing model.
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

	roleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CBA6F7")).
			Bold(true)

	faintStyle = lipgloss.NewStyle().Faint(true)
)

// -- List items --------------------------------------------------------------

// sectionItem is a non-selectable section header.
type sectionItem struct{ label string }

func (s sectionItem) Title() string       { return s.label }
func (s sectionItem) Description() string { return "" }
func (s sectionItem) FilterValue() string { return "" }

// targetItem wraps a config.Target for the agents list.
type targetItem struct {
	target config.Target
	role   string // assigned role label, or "(none)"
}

func (t targetItem) Title() string { return t.target.Name }
func (t targetItem) Description() string {
	model := t.target.Model
	if model == "" {
		model = "(no model)"
	}
	return fmt.Sprintf("model: %s  role: %s", model, t.role)
}
func (t targetItem) FilterValue() string {
	return t.target.Name + " " + t.target.Model + " " + t.role
}

// roleItem wraps a UserRole for the roles list.
type roleItem struct {
	role  config.UserRole
	model string // mapped model, or "(unmapped)"
	desc  string
}

func (r roleItem) Title() string       { return string(r.role) }
func (r roleItem) Description() string { return fmt.Sprintf("%s  →  %s", r.desc, r.model) }
func (r roleItem) FilterValue() string { return string(r.role) + " " + r.model }

// pickItem is a selectable option in a picker list (role or model).
type pickItem struct {
	label string
	value string // empty string = "clear" option
}

func (p pickItem) Title() string       { return p.label }
func (p pickItem) Description() string { return "" }
func (p pickItem) FilterValue() string { return p.label }

// -- Item builders -----------------------------------------------------------

func buildTargetItems(targets []config.Target, routing config.RoutingConfig) []list.Item {
	var agents, commands []list.Item
	for _, t := range targets {
		if t.Hidden {
			continue
		}
		roleName := "(none)"
		if r, ok := routing.TargetRoles[t.Name]; ok {
			roleName = string(r)
		}
		item := targetItem{target: t, role: roleName}
		if t.Kind == config.KindCommand {
			commands = append(commands, item)
		} else {
			agents = append(agents, item)
		}
	}
	var items []list.Item
	if len(agents) > 0 {
		items = append(items, sectionItem{"Agents"})
		items = append(items, agents...)
	}
	if len(commands) > 0 {
		items = append(items, sectionItem{"Commands"})
		items = append(items, commands...)
	}
	return items
}

func buildRoleItems(routing config.RoutingConfig) []list.Item {
	var items []list.Item
	for _, r := range config.AllUserRoles() {
		model := "(unmapped)"
		if m, ok := routing.RoleModels[r]; ok && m != "" {
			model = m
		}
		items = append(items, roleItem{
			role:  r,
			model: model,
			desc:  config.UserRoleDescription(r),
		})
	}
	return items
}

func buildRolePickItems() []list.Item {
	items := []list.Item{
		pickItem{label: "(none — clear role)", value: ""},
	}
	for _, r := range config.AllUserRoles() {
		items = append(items, pickItem{
			label: fmt.Sprintf("%s — %s", r, config.UserRoleDescription(r)),
			value: string(r),
		})
	}
	return items
}

func buildModelPickItems(models []config.Model) []list.Item {
	items := []list.Item{
		pickItem{label: "(none — clear model)", value: ""},
	}
	for _, mdl := range models {
		items = append(items, pickItem{label: mdl.ID, value: mdl.ID})
	}
	return items
}

// -- Delegate ----------------------------------------------------------------

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

// -- Pick delegate (single-line items, no description) -----------------------

type pickDelegate struct{}

func (d pickDelegate) Height() int                             { return 1 }
func (d pickDelegate) Spacing() int                            { return 0 }
func (d pickDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d pickDelegate) ShortHelp() []key.Binding                { return nil }
func (d pickDelegate) FullHelp() [][]key.Binding               { return nil }

func (d pickDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	pi, ok := item.(pickItem)
	if !ok {
		return
	}
	cursor := "  "
	style := lipgloss.NewStyle()
	if index == m.Index() {
		cursor = "> "
		style = style.Foreground(lipgloss.Color("#7D56F4")).Bold(true)
	}
	label := pi.label
	if m.Width() > 4 {
		label = ansi.Truncate(label, m.Width()-4, "…")
	}
	fmt.Fprint(w, cursor+style.Render(label)) //nolint: errcheck
}

// -- View state --------------------------------------------------------------

type viewState int

const (
	viewAgents viewState = iota // agent/command list
	viewRoles                   // role→model list
	viewPicker                  // picker list (role or model selection)
)

// pickerKind tracks what the picker is selecting.
type pickerKind int

const (
	pickRole  pickerKind = iota // picking a role for a target
	pickModel                   // picking a model for a role
)

// -- Messages ----------------------------------------------------------------

type applyResultMsg struct{ err error }
type saveRoutingMsg struct{ err error }

type rolePickDoneMsg struct {
	targetName string
	role       config.UserRole
	cleared    bool
}

type modelPickDoneMsg struct {
	role    config.UserRole
	model   string
	cleared bool
}

// -- Model -------------------------------------------------------------------

// Model is the top-level Bubbletea model.
type Model struct {
	state   *config.State
	routing config.RoutingConfig
	view    viewState

	agentList  list.Model
	roleList   list.Model
	pickerList list.Model

	// picker context
	pickerKind       pickerKind
	pickerTargetName string          // which target (for role picker)
	pickerRole       config.UserRole // which role (for model picker)

	status string
	width  int
	height int
}

// New creates the initial TUI model.
func New(state *config.State, routing config.RoutingConfig) Model {
	delegate := newDelegate()

	agentItems := buildTargetItems(state.Targets, routing)
	al := list.New(agentItems, delegate, 0, 0)
	al.Title = "Agents & Commands"
	al.Styles.Title = titleStyle
	al.SetShowStatusBar(true)
	al.SetFilteringEnabled(true)

	roleItems := buildRoleItems(routing)
	rl := list.New(roleItems, delegate, 0, 0)
	rl.Title = "Role → Model Mapping"
	rl.Styles.Title = titleStyle
	rl.SetShowStatusBar(false)
	rl.SetFilteringEnabled(false)

	// Picker starts empty; populated when opened
	pl := list.New(nil, pickDelegate{}, 0, 0)
	pl.Styles.Title = titleStyle
	pl.SetShowStatusBar(true)
	pl.SetFilteringEnabled(true)

	return Model{
		state:      state,
		routing:    routing,
		view:       viewAgents,
		agentList:  al,
		roleList:   rl,
		pickerList: pl,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// -- Update ------------------------------------------------------------------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := appStyle.GetFrameSize()
		m.agentList.SetSize(msg.Width-h, msg.Height-v)
		// Role list needs less height: 5 summary lines + help + status + padding
		roleExtra := 9
		m.roleList.SetSize(msg.Width-h, msg.Height-v-roleExtra)
		m.pickerList.SetSize(msg.Width-h, msg.Height-v)
		return m, nil

	case applyResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error applying: %v", msg.err)
		} else {
			m.status = "Routing applied to opencode.json"
		}
		return m, nil

	case saveRoutingMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error saving: %v", msg.err)
		}
		return m, nil

	case rolePickDoneMsg:
		if msg.cleared {
			delete(m.routing.TargetRoles, msg.targetName)
			m.status = fmt.Sprintf("Cleared role for %s", msg.targetName)
		} else {
			m.routing.TargetRoles[msg.targetName] = msg.role
			m.status = fmt.Sprintf("Set %s → %s", msg.targetName, msg.role)
		}
		m.view = viewAgents
		m.rebuildAgentList()
		return m, m.saveRoutingCmd()

	case modelPickDoneMsg:
		if msg.cleared {
			delete(m.routing.RoleModels, msg.role)
			m.status = fmt.Sprintf("Cleared model for %s", msg.role)
		} else {
			m.routing.RoleModels[msg.role] = msg.model
			m.status = fmt.Sprintf("Set %s → %s", msg.role, msg.model)
		}
		m.view = viewRoles
		m.rebuildRoleList()
		return m, m.saveRoutingCmd()

	case tea.KeyMsg:
		// Picker view: enter selects, esc goes back
		if m.view == viewPicker {
			if m.pickerList.FilterState() == list.Filtering {
				break // let filter handle keys
			}
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				return m.handlePickerSelect()
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				if m.pickerKind == pickRole {
					m.view = viewAgents
				} else {
					m.view = viewRoles
				}
				m.status = ""
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
				return m, tea.Quit
			}
			break
		}

		if m.activeList().FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if m.view == viewRoles {
				m.view = viewAgents
				return m, nil
			}
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			if m.view == viewAgents {
				m.view = viewRoles
			} else if m.view == viewRoles {
				m.view = viewAgents
			}
			m.status = ""
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
			if m.view == viewAgents {
				return m.openRolePicker()
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if m.view == viewRoles {
				return m.openModelPicker()
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("a"))):
			if m.view == viewAgents {
				return m.applyRouting()
			}
		}
	}

	// Delegate to active view
	var cmd tea.Cmd
	switch m.view {
	case viewAgents:
		m.agentList, cmd = m.agentList.Update(msg)
	case viewRoles:
		m.roleList, cmd = m.roleList.Update(msg)
	case viewPicker:
		m.pickerList, cmd = m.pickerList.Update(msg)
	}
	return m, cmd
}

func (m *Model) activeList() *list.Model {
	if m.view == viewRoles {
		return &m.roleList
	}
	return &m.agentList
}

// openRolePicker opens a list picker to assign a role to the selected agent.
func (m Model) openRolePicker() (tea.Model, tea.Cmd) {
	item, ok := m.agentList.SelectedItem().(targetItem)
	if !ok {
		return m, nil
	}

	items := buildRolePickItems()
	m.pickerList.SetItems(items)
	m.pickerList.Title = fmt.Sprintf("Role for %s", item.target.Name)
	m.pickerList.ResetFilter()
	m.pickerList.Select(0)

	m.pickerKind = pickRole
	m.pickerTargetName = item.target.Name
	m.view = viewPicker
	m.status = ""
	return m, nil
}

// openModelPicker opens a list picker to assign a model to the selected role.
func (m Model) openModelPicker() (tea.Model, tea.Cmd) {
	item, ok := m.roleList.SelectedItem().(roleItem)
	if !ok {
		return m, nil
	}

	items := buildModelPickItems(m.state.Models)
	m.pickerList.SetItems(items)
	m.pickerList.Title = fmt.Sprintf("Model for %s", item.role)
	m.pickerList.ResetFilter()
	m.pickerList.Select(0)

	m.pickerKind = pickModel
	m.pickerRole = item.role
	m.view = viewPicker
	m.status = ""
	return m, nil
}

func (m Model) handlePickerSelect() (tea.Model, tea.Cmd) {
	item, ok := m.pickerList.SelectedItem().(pickItem)
	if !ok {
		return m, nil
	}

	if m.pickerKind == pickRole {
		targetName := m.pickerTargetName
		value := item.value
		return m, func() tea.Msg {
			if value == "" {
				return rolePickDoneMsg{targetName: targetName, cleared: true}
			}
			return rolePickDoneMsg{targetName: targetName, role: config.UserRole(value)}
		}
	}

	// Model picker
	role := m.pickerRole
	value := item.value
	return m, func() tea.Msg {
		if value == "" {
			return modelPickDoneMsg{role: role, cleared: true}
		}
		return modelPickDoneMsg{role: role, model: value}
	}
}

func (m Model) applyRouting() (tea.Model, tea.Cmd) {
	routing := m.routing
	targets := m.state.Targets
	return m, func() tea.Msg {
		err := config.ApplyRouting(routing, targets)
		return applyResultMsg{err: err}
	}
}

func (m Model) saveRoutingCmd() tea.Cmd {
	routing := m.routing
	return func() tea.Msg {
		err := config.SaveRouting(routing)
		return saveRoutingMsg{err: err}
	}
}

func (m *Model) rebuildAgentList() {
	items := buildTargetItems(m.state.Targets, m.routing)
	m.agentList.SetItems(items)
}

func (m *Model) rebuildRoleList() {
	items := buildRoleItems(m.routing)
	m.roleList.SetItems(items)
}

// -- View --------------------------------------------------------------------

func (m Model) View() string {
	var content string

	switch m.view {
	case viewAgents:
		content = m.agentList.View()
		if m.status != "" {
			content += "\n" + statusStyle.Render(m.status)
		}
		content += "\n" + faintStyle.Render("r: assign role  a: apply routing  tab: roles view  q: quit")

	case viewRoles:
		content = m.roleList.View()
		if m.status != "" {
			content += "\n" + statusStyle.Render(m.status)
		}
		var summary []string
		for _, r := range config.AllUserRoles() {
			model := faintStyle.Render("—")
			if m, ok := m.routing.RoleModels[r]; ok && m != "" {
				model = roleStyle.Render(m)
			}
			summary = append(summary, fmt.Sprintf("  %s → %s", roleStyle.Render(string(r)), model))
		}
		content += "\n" + strings.Join(summary, "\n")
		content += "\n" + faintStyle.Render("enter: set model  tab: agents view  esc: back  q: quit")

	case viewPicker:
		content = m.pickerList.View()
		content += "\n" + faintStyle.Render("enter: select  /: filter  esc: cancel")
	}

	return appStyle.Render(content)
}

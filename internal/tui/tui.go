// Package tui implements the Bubbletea TUI for model routing management.
//
// Two views:
//   - Agents view: list of agents/commands with current model and assigned role.
//     Press 'r' to assign a role, 'a' to apply routing to opencode.json.
//   - Roles view: list of 5 roles with their mapped model.
//     Press 'enter' to pick a model for a role.
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
	"github.com/charmbracelet/huh"
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

// -- View state --------------------------------------------------------------

type viewState int

const (
	viewAgents viewState = iota // agent/command list
	viewRoles                   // role→model list
	viewForm                    // huh form (role picker or model picker)
)

// -- Messages ----------------------------------------------------------------

type applyResultMsg struct{ err error }
type saveRoutingMsg struct{ err error }

type rolePickDoneMsg struct {
	targetName string
	role       config.UserRole
	cleared    bool // true = user chose to clear the role
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

	agentList list.Model
	roleList  list.Model
	form      *huh.Form

	// form context
	formRoleValue  string          // bound to huh select for role picker
	formModelValue string          // bound to huh select for model picker
	formTargetName string          // which target we're assigning a role to
	formRole       config.UserRole // which role we're assigning a model to

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

	return Model{
		state:     state,
		routing:   routing,
		view:      viewAgents,
		agentList: al,
		roleList:  rl,
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
		m.roleList.SetSize(msg.Width-h, msg.Height-v)
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
		} else {
			m.routing.TargetRoles[msg.targetName] = msg.role
		}
		m.view = viewAgents
		m.rebuildAgentList()
		m.status = fmt.Sprintf("Set %s → %s", msg.targetName, msg.role)
		return m, m.saveRoutingCmd()

	case modelPickDoneMsg:
		if msg.cleared {
			delete(m.routing.RoleModels, msg.role)
		} else {
			m.routing.RoleModels[msg.role] = msg.model
		}
		m.view = viewRoles
		m.rebuildRoleList()
		m.status = fmt.Sprintf("Set %s → %s", msg.role, msg.model)
		return m, m.saveRoutingCmd()

	case tea.KeyMsg:
		if m.view == viewForm {
			break // form handles its own keys
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
	case viewForm:
		if m.form != nil {
			form, fCmd := m.form.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				m.form = f
			}
			if m.form.State == huh.StateCompleted {
				return m, m.handleFormComplete()
			}
			if m.form.State == huh.StateAborted {
				// Return to previous view
				if m.formTargetName != "" {
					m.view = viewAgents
				} else {
					m.view = viewRoles
				}
				m.status = ""
				return m, nil
			}
			cmd = fCmd
		}
	}
	return m, cmd
}

func (m *Model) activeList() *list.Model {
	if m.view == viewRoles {
		return &m.roleList
	}
	return &m.agentList
}

// openRolePicker opens a huh form to assign a role to the selected agent.
func (m Model) openRolePicker() (tea.Model, tea.Cmd) {
	item, ok := m.agentList.SelectedItem().(targetItem)
	if !ok {
		return m, nil
	}

	opts := []huh.Option[string]{
		huh.NewOption("(none — clear role)", ""),
	}
	for _, r := range config.AllUserRoles() {
		label := fmt.Sprintf("%s — %s", r, config.UserRoleDescription(r))
		opts = append(opts, huh.NewOption(label, string(r)))
	}

	// Pre-select current role
	m.formRoleValue = ""
	if r, ok := m.routing.TargetRoles[item.target.Name]; ok {
		m.formRoleValue = string(r)
	}

	m.formTargetName = item.target.Name
	m.formRole = "" // not used for role picker
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Role for %s", item.target.Name)).
				Description("Assign a role to this agent/command").
				Options(opts...).
				Value(&m.formRoleValue),
		),
	)

	m.view = viewForm
	return m, m.form.Init()
}

// openModelPicker opens a huh form to assign a model to the selected role.
func (m Model) openModelPicker() (tea.Model, tea.Cmd) {
	item, ok := m.roleList.SelectedItem().(roleItem)
	if !ok {
		return m, nil
	}

	opts := []huh.Option[string]{
		huh.NewOption("(none — clear model)", ""),
	}
	for _, mdl := range m.state.Models {
		opts = append(opts, huh.NewOption(mdl.ID, mdl.ID))
	}

	// Pre-select current model
	m.formModelValue = ""
	if mdl, ok := m.routing.RoleModels[item.role]; ok {
		m.formModelValue = mdl
	}

	m.formTargetName = "" // not used for model picker
	m.formRole = item.role
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Model for %s", item.role)).
				Description(config.UserRoleDescription(item.role)).
				Options(opts...).
				Value(&m.formModelValue),
		),
	)

	m.view = viewForm
	return m, m.form.Init()
}

func (m Model) handleFormComplete() tea.Cmd {
	if m.formTargetName != "" {
		// Role picker completed
		targetName := m.formTargetName
		roleVal := m.formRoleValue
		return func() tea.Msg {
			if roleVal == "" {
				return rolePickDoneMsg{targetName: targetName, cleared: true}
			}
			return rolePickDoneMsg{targetName: targetName, role: config.UserRole(roleVal)}
		}
	}
	// Model picker completed
	role := m.formRole
	modelVal := m.formModelValue
	return func() tea.Msg {
		if modelVal == "" {
			return modelPickDoneMsg{role: role, cleared: true}
		}
		return modelPickDoneMsg{role: role, model: modelVal}
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

	case viewForm:
		if m.form != nil {
			content = m.form.View()
		}
	}

	return appStyle.Render(content)
}

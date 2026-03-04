// Package tui implements the Bubbletea TUI for model routing management.
//
// Flow: mapping list → (new/edit mapping via huh form) → activate mapping → back to list.
//
// A "mapping" pairs a name with an orchestrator model (applied to primary/all
// agents and commands) and a worker model (applied to subagents). Activating a
// mapping writes those models to opencode.json via ApplyActiveMapping.
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

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAB387")).
			Italic(true)
)

// -- List items --------------------------------------------------------------

// sectionItem is a non-selectable section header.
type sectionItem struct{ label string }

func (s sectionItem) Title() string       { return s.label }
func (s sectionItem) Description() string { return "" }
func (s sectionItem) FilterValue() string { return "" }

// mappingItem wraps a config.Mapping for the list.
type mappingItem struct {
	mapping config.Mapping
}

func (m mappingItem) Title() string { return m.mapping.Name }
func (m mappingItem) Description() string {
	return fmt.Sprintf("orchestrator: %s | worker: %s", m.mapping.Orchestrator, m.mapping.Worker)
}
func (m mappingItem) FilterValue() string {
	return m.mapping.Name + " " + m.mapping.Orchestrator + " " + m.mapping.Worker
}

// newMappingItem is the special "create new mapping" entry.
type newMappingItem struct{}

func (n newMappingItem) Title() string       { return "(+ new mapping)" }
func (n newMappingItem) Description() string { return "Create a new orchestrator/worker model pair" }
func (n newMappingItem) FilterValue() string { return "new create" }

// buildMappingItems constructs the list items for the mapping list view.
func buildMappingItems(rc config.RoutingConfig) []list.Item {
	var items []list.Item
	if len(rc.Mappings) > 0 {
		items = append(items, sectionItem{"Saved Mappings"})
		for _, m := range rc.Mappings {
			items = append(items, mappingItem{mapping: m})
		}
	}
	items = append(items, sectionItem{"Actions"})
	items = append(items, newMappingItem{})
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
	viewMappings viewState = iota // mapping list
	viewForm                      // huh form for create/edit
)

// -- Messages ----------------------------------------------------------------

type activateResultMsg struct {
	err     error
	mapping config.Mapping
}

type saveRoutingResultMsg struct {
	err error
}

type formDoneMsg struct {
	mapping config.Mapping
	saved   bool // false = cancelled
}

// -- Model -------------------------------------------------------------------

// Model is the top-level Bubbletea model.
type Model struct {
	state   *config.State
	routing config.RoutingConfig
	view    viewState

	mappingList list.Model
	form        *huh.Form

	// form field values (bound to huh)
	formName         string
	formOrchestrator string
	formWorker       string
	editIdx          int // -1 = new, >=0 = editing existing

	// pendingActivate holds a mapping awaiting confirmation when existing
	// per-target model prefs would be overwritten.
	pendingActivate *config.Mapping

	status   string
	warnings []string
	width    int
	height   int
}

// New creates the initial TUI model.
func New(state *config.State, routing config.RoutingConfig) Model {
	items := buildMappingItems(routing)
	delegate := newDelegate()
	ml := list.New(items, delegate, 0, 0)
	ml.Title = "Model Routing"
	ml.Styles.Title = titleStyle
	ml.SetShowStatusBar(true)
	ml.SetFilteringEnabled(true)

	return Model{
		state:       state,
		routing:     routing,
		view:        viewMappings,
		mappingList: ml,
		editIdx:     -1,
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
		m.mappingList.SetSize(msg.Width-h, msg.Height-v)
		return m, nil

	case activateResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error activating: %v", msg.err)
		} else {
			m.status = fmt.Sprintf("Activated mapping '%s'", msg.mapping.Name)
			// Refresh in-memory target models so subsequent activation checks
			// reflect the newly applied values (avoids false "will overwrite" warnings).
			m.refreshTargetModels(msg.mapping)
		}
		m.view = viewMappings
		return m, nil

	case saveRoutingResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error saving: %v", msg.err)
		} else {
			m.status = "Mapping saved"
		}
		m.view = viewMappings
		m.rebuildMappingList()
		return m, nil

	case formDoneMsg:
		if !msg.saved {
			m.view = viewMappings
			m.status = ""
			return m, nil
		}
		// Save the mapping
		mapping := msg.mapping
		routing := m.routing
		if m.editIdx >= 0 && m.editIdx < len(routing.Mappings) {
			routing.Mappings[m.editIdx] = mapping
		} else {
			routing.Mappings = append(routing.Mappings, mapping)
		}
		m.routing = routing
		return m, func() tea.Msg {
			err := config.SaveRouting(routing)
			return saveRoutingResultMsg{err: err}
		}

	case tea.KeyMsg:
		if m.view == viewForm {
			// Form handles its own keys
			break
		}
		if m.view == viewMappings && m.mappingList.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c", "esc"))):
			if m.pendingActivate != nil {
				// Cancel pending activation
				m.pendingActivate = nil
				m.status = "Activation cancelled"
				return m, nil
			}
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			m.pendingActivate = nil // cancel any pending activation on navigation
			return m.handleSelect()

		case key.Matches(msg, key.NewBinding(key.WithKeys("a"))):
			// 'a' activates the selected mapping (or confirms pending activation)
			if m.view == viewMappings {
				if m.pendingActivate != nil {
					// Second press: confirmed — apply
					mapping := *m.pendingActivate
					m.pendingActivate = nil
					targets := m.state.Targets
					return m, func() tea.Msg {
						err := config.ApplyActiveMapping(mapping, targets)
						return activateResultMsg{err: err, mapping: mapping}
					}
				}
				return m.handleActivate()
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
			// 'd' deletes the selected mapping
			if m.view == viewMappings {
				return m.handleDelete()
			}
		}
	}

	// Delegate to active view
	var cmd tea.Cmd
	if m.view == viewMappings {
		m.mappingList, cmd = m.mappingList.Update(msg)
	} else if m.view == viewForm && m.form != nil {
		form, fCmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}
		if m.form.State == huh.StateCompleted {
			return m, func() tea.Msg {
				return formDoneMsg{
					mapping: config.Mapping{
						Name:         m.formName,
						Orchestrator: m.formOrchestrator,
						Worker:       m.formWorker,
					},
					saved: true,
				}
			}
		}
		if m.form.State == huh.StateAborted {
			return m, func() tea.Msg {
				return formDoneMsg{saved: false}
			}
		}
		cmd = fCmd
	}
	return m, cmd
}

func (m Model) handleSelect() (tea.Model, tea.Cmd) {
	switch item := m.mappingList.SelectedItem().(type) {
	case newMappingItem:
		return m.openForm(-1, config.Mapping{})
	case mappingItem:
		// Enter on a mapping: open edit form
		idx := m.findMappingIdx(item.mapping.Name)
		return m.openForm(idx, item.mapping)
	}
	return m, nil
}

func (m Model) handleActivate() (tea.Model, tea.Cmd) {
	item, ok := m.mappingList.SelectedItem().(mappingItem)
	if !ok {
		return m, nil
	}
	mapping := item.mapping

	// Check if any targets already have a model set — warn before overwriting.
	var existing []string
	for _, t := range m.state.Targets {
		if t.Model != "" {
			existing = append(existing, t.Name)
		}
	}
	if len(existing) > 0 {
		m.pendingActivate = &mapping
		m.status = fmt.Sprintf(
			"⚠ Will overwrite existing model prefs for: %s — press 'a' again to confirm, any other key to cancel",
			strings.Join(existing, ", "),
		)
		return m, nil
	}

	targets := m.state.Targets
	return m, func() tea.Msg {
		err := config.ApplyActiveMapping(mapping, targets)
		return activateResultMsg{err: err, mapping: mapping}
	}
}

func (m Model) handleDelete() (tea.Model, tea.Cmd) {
	item, ok := m.mappingList.SelectedItem().(mappingItem)
	if !ok {
		return m, nil
	}
	idx := m.findMappingIdx(item.mapping.Name)
	if idx < 0 {
		return m, nil
	}
	routing := m.routing
	routing.Mappings = append(routing.Mappings[:idx], routing.Mappings[idx+1:]...)
	m.routing = routing
	m.rebuildMappingList()
	return m, func() tea.Msg {
		err := config.SaveRouting(routing)
		return saveRoutingResultMsg{err: err}
	}
}

func (m Model) findMappingIdx(name string) int {
	for i, mp := range m.routing.Mappings {
		if mp.Name == name {
			return i
		}
	}
	return -1
}

func (m Model) openForm(editIdx int, existing config.Mapping) (tea.Model, tea.Cmd) {
	m.editIdx = editIdx
	m.formName = existing.Name
	m.formOrchestrator = existing.Orchestrator
	m.formWorker = existing.Worker

	// Build model ID options for the selects
	modelOpts := make([]huh.Option[string], 0, len(m.state.Models))
	for _, mdl := range m.state.Models {
		modelOpts = append(modelOpts, huh.NewOption(mdl.ID, mdl.ID))
	}

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Mapping name").
				Description("A short label for this mapping (e.g. 'fast', 'quality')").
				Value(&m.formName),
			huh.NewSelect[string]().
				Title("Orchestrator model").
				Description("Applied to primary/all agents and commands").
				Options(modelOpts...).
				Value(&m.formOrchestrator),
			huh.NewSelect[string]().
				Title("Worker model").
				Description("Applied to subagents").
				Options(modelOpts...).
				Value(&m.formWorker),
		),
	)

	m.view = viewForm
	return m, m.form.Init()
}

func (m *Model) rebuildMappingList() {
	items := buildMappingItems(m.routing)
	m.mappingList.SetItems(items)
}

// refreshTargetModels updates the in-memory Target.Model values to reflect
// the models that were just applied by ApplyActiveMapping. This prevents
// subsequent activation attempts from incorrectly showing "will overwrite"
// warnings for models that were set by the current activation.
func (m *Model) refreshTargetModels(applied config.Mapping) {
	for i := range m.state.Targets {
		role := config.RoleForTarget(m.state.Targets[i])
		if role == config.RoleOrchestrator {
			m.state.Targets[i].Model = applied.Orchestrator
		} else {
			m.state.Targets[i].Model = applied.Worker
		}
	}
}

// -- View --------------------------------------------------------------------

func (m Model) View() string {
	var content string

	switch m.view {
	case viewMappings:
		content = m.mappingList.View()
		if len(m.warnings) > 0 {
			content += "\n" + warningStyle.Render("⚠ "+strings.Join(m.warnings, "; "))
		}
		if m.status != "" {
			content += "\n" + statusStyle.Render(m.status)
		}
		content += "\n" + lipgloss.NewStyle().Faint(true).Render("enter: edit  a: activate  d: delete  q: quit")

	case viewForm:
		if m.form != nil {
			content = m.form.View()
		}
	}

	return appStyle.Render(content)
}

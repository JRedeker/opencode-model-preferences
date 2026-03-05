// Package tui implements the Bubbletea TUI for slot-based model management.
//
// Three views, all using bubbles/list for consistent UX:
//   - Assignments view: list of agents/commands with current model and assigned slot.
//     Press 's' to assign a slot, 'a' to apply slots to opencode.json.
//   - Slots view: list of user-defined slots with their mapped model.
//     Press 'enter' to pick a model for a slot, 'r' to rename a slot.
//   - Picker view: full list.Model for selecting a slot or model, with filtering.
//
// Slots are purely for model mapping — they carry no context or system prompt.
// If a slot has no model mapped, targets assigned to it keep their existing model.
package tui

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/anomalyco/opencode-model-preferences/internal/config"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
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

	slotStyle = lipgloss.NewStyle().
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

// targetItem wraps a config.Target for the assignments list.
type targetItem struct {
	target   config.Target
	slotName string // assigned slot display name, or "(none)"
}

func (t targetItem) Title() string { return t.target.Name }
func (t targetItem) Description() string {
	model := t.target.Model
	if model == "" {
		model = "(no model)"
	}
	return fmt.Sprintf("model: %s  slot: %s", model, t.slotName)
}
func (t targetItem) FilterValue() string {
	return t.target.Name + " " + t.target.Model + " " + t.slotName
}

// slotItem wraps a config.Slot for the slots list.
type slotItem struct {
	slot config.Slot
}

func (s slotItem) Title() string { return s.slot.Name }
func (s slotItem) Description() string {
	model := s.slot.Model
	if model == "" {
		model = "(unmapped)"
	}
	return fmt.Sprintf("%s  →  %s", s.slot.ID, model)
}
func (s slotItem) FilterValue() string { return s.slot.Name + " " + s.slot.Model + " " + s.slot.ID }

// pickItem is a selectable option in a picker list (slot or model).
type pickItem struct {
	label string
	value string // empty string = "clear" option
}

func (p pickItem) Title() string       { return p.label }
func (p pickItem) Description() string { return "" }
func (p pickItem) FilterValue() string { return p.label }

// -- Item builders -----------------------------------------------------------

func buildTargetItems(targets []config.Target, slots config.SlotsConfig) []list.Item {
	// Build slot ID → name lookup.
	slotNames := make(map[string]string, len(slots.Slots))
	for _, s := range slots.Slots {
		slotNames[s.ID] = s.Name
	}

	var agents, commands []list.Item
	for _, t := range targets {
		if t.Hidden {
			continue
		}
		slotName := "(none)"
		if slotID, ok := slots.TargetSlots[t.Name]; ok {
			if name, ok := slotNames[slotID]; ok {
				slotName = name
			} else {
				slotName = slotID // fallback to ID if name not found
			}
		}
		item := targetItem{target: t, slotName: slotName}
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

func buildSlotItems(slots config.SlotsConfig) []list.Item {
	var items []list.Item
	for _, s := range slots.Slots {
		items = append(items, slotItem{slot: s})
	}
	return items
}

func buildSlotPickItems(slots config.SlotsConfig) []list.Item {
	items := []list.Item{
		pickItem{label: "(none — clear slot)", value: ""},
	}
	for _, s := range slots.Slots {
		label := s.Name
		if s.Model != "" {
			label = fmt.Sprintf("%s — %s", s.Name, s.Model)
		}
		items = append(items, pickItem{label: label, value: s.ID})
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
	viewAssignments viewState = iota // agent/command list with slot assignments
	viewSlots                        // slot→model list
	viewPicker                       // picker list (slot or model selection)
	viewRename                       // text input to rename slot
)

// pickerKind tracks what the picker is selecting.
type pickerKind int

const (
	pickSlot  pickerKind = iota // picking a slot for a target
	pickModel                   // picking a model for a slot
)

// -- Messages ----------------------------------------------------------------

type applyResultMsg struct{ err error }
type saveSlotsMsg struct{ err error }

type slotPickDoneMsg struct {
	targetName string
	slotID     string
	cleared    bool
}

type modelPickDoneMsg struct {
	slotID  string
	model   string
	cleared bool
}

// -- Model -------------------------------------------------------------------

// Model is the top-level Bubbletea model.
type Model struct {
	state *config.State
	slots config.SlotsConfig
	view  viewState

	assignmentList list.Model
	slotList       list.Model
	pickerList     list.Model

	// picker context
	pickerKind       pickerKind
	pickerTargetName string // which target (for slot picker)
	pickerSlotID     string // which slot ID (for model picker)

	status string
	width  int
	height int

	renameInput  textinput.Model
	renameSlotID string
}

// New creates the initial TUI model.
func New(state *config.State, slots config.SlotsConfig) Model {
	delegate := newDelegate()

	assignmentItems := buildTargetItems(state.Targets, slots)
	al := list.New(assignmentItems, delegate, 0, 0)
	al.Title = "Agents & Commands"
	al.Styles.Title = titleStyle
	al.SetShowStatusBar(true)
	al.SetFilteringEnabled(true)

	slotItems := buildSlotItems(slots)
	sl := list.New(slotItems, delegate, 0, 0)
	sl.Title = "Slots → Models"
	sl.Styles.Title = titleStyle
	sl.SetShowStatusBar(false)
	sl.SetFilteringEnabled(false)

	// Picker starts empty; populated when opened
	pl := list.New(nil, pickDelegate{}, 0, 0)
	pl.Styles.Title = titleStyle
	pl.SetShowStatusBar(true)
	pl.SetFilteringEnabled(true)

	ri := textinput.New()
	ri.Placeholder = "Slot name"
	ri.CharLimit = 80
	ri.Width = 50

	return Model{
		state:          state,
		slots:          slots,
		view:           viewAssignments,
		assignmentList: al,
		slotList:       sl,
		pickerList:     pl,
		renameInput:    ri,
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
		m.assignmentList.SetSize(msg.Width-h, msg.Height-v)
		// Slot list needs less height: slot summary lines + help + status + padding
		slotExtra := 9
		m.slotList.SetSize(msg.Width-h, msg.Height-v-slotExtra)
		m.pickerList.SetSize(msg.Width-h, msg.Height-v)
		m.renameInput.Width = msg.Width - h - 6
		return m, nil

	case applyResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error applying: %v", msg.err)
		} else {
			m.status = "Slots applied to opencode.json"
		}
		return m, nil

	case saveSlotsMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error saving: %v", msg.err)
		}
		return m, nil

	case slotPickDoneMsg:
		if msg.cleared {
			delete(m.slots.TargetSlots, msg.targetName)
			m.status = fmt.Sprintf("Cleared slot for %s", msg.targetName)
		} else {
			m.slots.TargetSlots[msg.targetName] = msg.slotID
			slotName := msg.slotID
			for _, s := range m.slots.Slots {
				if s.ID == msg.slotID {
					slotName = s.Name
					break
				}
			}
			m.status = fmt.Sprintf("Set %s → %s", msg.targetName, slotName)
		}
		m.view = viewAssignments
		m.rebuildAssignmentList()
		return m, m.saveSlotsCmd()

	case modelPickDoneMsg:
		for i, s := range m.slots.Slots {
			if s.ID == msg.slotID {
				if msg.cleared {
					m.slots.Slots[i].Model = ""
					m.status = fmt.Sprintf("Cleared model for %s", s.Name)
				} else {
					m.slots.Slots[i].Model = msg.model
					m.status = fmt.Sprintf("Set %s → %s", s.Name, msg.model)
				}
				break
			}
		}
		m.view = viewSlots
		m.rebuildSlotList()
		return m, m.saveSlotsCmd()

	case tea.KeyMsg:
		if m.view == viewRename {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				name := strings.TrimSpace(m.renameInput.Value())
				if name == "" {
					m.status = "Slot name cannot be empty"
					return m, nil
				}
				if renameSlot(&m.slots, m.renameSlotID, name) {
					m.status = fmt.Sprintf("Renamed %s", name)
					m.view = viewSlots
					m.rebuildSlotList()
					m.rebuildAssignmentList()
					m.renameInput.Reset()
					return m, m.saveSlotsCmd()
				}
				m.status = "Could not rename slot"
				m.view = viewSlots
				m.renameInput.Reset()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				m.view = viewSlots
				m.renameInput.Reset()
				m.status = ""
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.renameInput, cmd = m.renameInput.Update(msg)
			return m, cmd
		}

		// Picker view: enter selects, esc goes back
		if m.view == viewPicker {
			if m.pickerList.FilterState() == list.Filtering {
				break // let filter handle keys
			}
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				return m.handlePickerSelect()
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				if m.pickerKind == pickSlot {
					m.view = viewAssignments
				} else {
					m.view = viewSlots
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
			if m.view == viewSlots {
				m.view = viewAssignments
				return m, nil
			}
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			if m.view == viewAssignments {
				m.view = viewSlots
			} else if m.view == viewSlots {
				m.view = viewAssignments
			}
			m.status = ""
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
			if m.view == viewAssignments {
				return m.openSlotPicker()
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if m.view == viewSlots {
				return m.openModelPicker()
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
			if m.view == viewSlots {
				return m.openRenameSlot()
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("n"))):
			if m.view == viewSlots {
				slot := addSlot(&m.slots)
				m.status = fmt.Sprintf("Added %s", slot.Name)
				m.rebuildSlotList()
				m.rebuildAssignmentList()
				m.slotList.Select(len(m.slots.Slots) - 1)
				return m, m.saveSlotsCmd()
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("x", "backspace", "delete"))):
			if m.view == viewSlots {
				if len(m.slots.Slots) <= 1 {
					m.status = "Cannot remove the last slot"
					return m, nil
				}
				item, ok := m.slotList.SelectedItem().(slotItem)
				if !ok {
					return m, nil
				}
				removed, cleared := removeSlot(&m.slots, item.slot.ID)
				if !removed {
					m.status = "Could not remove slot"
					return m, nil
				}
				m.status = fmt.Sprintf("Removed %s (cleared %d assignment(s))", item.slot.Name, cleared)
				oldIndex := m.slotList.Index()
				m.rebuildSlotList()
				m.rebuildAssignmentList()
				if len(m.slots.Slots) > 0 {
					if oldIndex >= len(m.slots.Slots) {
						oldIndex = len(m.slots.Slots) - 1
					}
					m.slotList.Select(oldIndex)
				}
				return m, m.saveSlotsCmd()
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("a"))):
			if m.view == viewAssignments {
				return m.applySlots()
			}
		}
	}

	// Delegate to active view
	var cmd tea.Cmd
	switch m.view {
	case viewAssignments:
		m.assignmentList, cmd = m.assignmentList.Update(msg)
	case viewSlots:
		m.slotList, cmd = m.slotList.Update(msg)
	case viewPicker:
		m.pickerList, cmd = m.pickerList.Update(msg)
	case viewRename:
		m.renameInput, cmd = m.renameInput.Update(msg)
	}
	return m, cmd
}

func (m *Model) activeList() *list.Model {
	if m.view == viewSlots {
		return &m.slotList
	}
	return &m.assignmentList
}

// openSlotPicker opens a list picker to assign a slot to the selected target.
func (m Model) openSlotPicker() (tea.Model, tea.Cmd) {
	item, ok := m.assignmentList.SelectedItem().(targetItem)
	if !ok {
		return m, nil
	}

	items := buildSlotPickItems(m.slots)
	m.pickerList.SetItems(items)
	m.pickerList.Title = fmt.Sprintf("Slot for %s", item.target.Name)
	m.pickerList.ResetFilter()
	m.pickerList.Select(0)

	m.pickerKind = pickSlot
	m.pickerTargetName = item.target.Name
	m.view = viewPicker
	m.status = ""
	return m, nil
}

// openModelPicker opens a list picker to assign a model to the selected slot.
func (m Model) openModelPicker() (tea.Model, tea.Cmd) {
	item, ok := m.slotList.SelectedItem().(slotItem)
	if !ok {
		return m, nil
	}

	items := buildModelPickItems(m.state.Models)
	m.pickerList.SetItems(items)
	m.pickerList.Title = fmt.Sprintf("Model for %s", item.slot.Name)
	m.pickerList.ResetFilter()
	m.pickerList.Select(0)

	m.pickerKind = pickModel
	m.pickerSlotID = item.slot.ID
	m.view = viewPicker
	m.status = ""
	return m, nil
}

func (m Model) openRenameSlot() (tea.Model, tea.Cmd) {
	item, ok := m.slotList.SelectedItem().(slotItem)
	if !ok {
		return m, nil
	}

	m.renameSlotID = item.slot.ID
	m.renameInput.SetValue(item.slot.Name)
	m.renameInput.Focus()
	m.renameInput.CursorEnd()
	m.view = viewRename
	m.status = ""
	return m, textinput.Blink
}

func (m Model) handlePickerSelect() (tea.Model, tea.Cmd) {
	item, ok := m.pickerList.SelectedItem().(pickItem)
	if !ok {
		return m, nil
	}

	if m.pickerKind == pickSlot {
		targetName := m.pickerTargetName
		value := item.value
		return m, func() tea.Msg {
			if value == "" {
				return slotPickDoneMsg{targetName: targetName, cleared: true}
			}
			return slotPickDoneMsg{targetName: targetName, slotID: value}
		}
	}

	// Model picker
	slotID := m.pickerSlotID
	value := item.value
	return m, func() tea.Msg {
		if value == "" {
			return modelPickDoneMsg{slotID: slotID, cleared: true}
		}
		return modelPickDoneMsg{slotID: slotID, model: value}
	}
}

func (m Model) applySlots() (tea.Model, tea.Cmd) {
	slots := m.slots
	targets := m.state.Targets
	return m, func() tea.Msg {
		err := config.ApplySlots(slots, targets)
		return applyResultMsg{err: err}
	}
}

func (m Model) saveSlotsCmd() tea.Cmd {
	slots := m.slots
	return func() tea.Msg {
		err := config.SaveSlots(slots)
		return saveSlotsMsg{err: err}
	}
}

func (m *Model) rebuildAssignmentList() {
	items := buildTargetItems(m.state.Targets, m.slots)
	m.assignmentList.SetItems(items)
}

func (m *Model) rebuildSlotList() {
	items := buildSlotItems(m.slots)
	m.slotList.SetItems(items)
}

func nextSlotID(slots []config.Slot) string {
	maxN := 0
	for _, s := range slots {
		if !strings.HasPrefix(s.ID, "slot-") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(s.ID, "slot-"))
		if err != nil {
			continue
		}
		if n > maxN {
			maxN = n
		}
	}
	next := maxN + 1
	return fmt.Sprintf("slot-%d", next)
}

func addSlot(sc *config.SlotsConfig) config.Slot {
	id := nextSlotID(sc.Slots)
	n := strings.TrimPrefix(id, "slot-")
	s := config.Slot{ID: id, Name: fmt.Sprintf("Slot %s", n)}
	sc.Slots = append(sc.Slots, s)
	if sc.TargetSlots == nil {
		sc.TargetSlots = make(map[string]string)
	}
	return s
}

func removeSlot(sc *config.SlotsConfig, slotID string) (bool, int) {
	idx := -1
	for i, s := range sc.Slots {
		if s.ID == slotID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false, 0
	}

	sc.Slots = append(sc.Slots[:idx], sc.Slots[idx+1:]...)
	cleared := 0
	for target, assigned := range sc.TargetSlots {
		if assigned == slotID {
			delete(sc.TargetSlots, target)
			cleared++
		}
	}
	return true, cleared
}

func renameSlot(sc *config.SlotsConfig, slotID, name string) bool {
	for i, s := range sc.Slots {
		if s.ID == slotID {
			sc.Slots[i].Name = name
			return true
		}
	}
	return false
}

// -- View --------------------------------------------------------------------

func (m Model) View() string {
	var content string

	switch m.view {
	case viewAssignments:
		content = m.assignmentList.View()
		if m.status != "" {
			content += "\n" + statusStyle.Render(m.status)
		}
		content += "\n" + faintStyle.Render("s: assign slot  a: apply slots  tab: slots view  q: quit")

	case viewSlots:
		content = m.slotList.View()
		if m.status != "" {
			content += "\n" + statusStyle.Render(m.status)
		}
		var summary []string
		for _, s := range m.slots.Slots {
			model := faintStyle.Render("—")
			if s.Model != "" {
				model = slotStyle.Render(s.Model)
			}
			summary = append(summary, fmt.Sprintf("  %s → %s", slotStyle.Render(s.Name), model))
		}
		content += "\n" + strings.Join(summary, "\n")
		content += "\n" + faintStyle.Render("enter: set model  r: rename slot  n: add slot  x: remove slot  tab: assignments view  esc: back  q: quit")

	case viewPicker:
		content = m.pickerList.View()
		content += "\n" + faintStyle.Render("enter: select  /: filter  esc: cancel")

	case viewRename:
		content = titleStyle.Render("Rename Slot") + "\n\n"
		content += m.renameInput.View()
		content += "\n\n" + faintStyle.Render("enter: save  esc: cancel")
	}

	return appStyle.Render(content)
}

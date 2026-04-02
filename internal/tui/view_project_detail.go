package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/thesimonho/warden/client"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/internal/tui/components"
)

// worktreeItem wraps a Worktree to satisfy list.DefaultItem.
type worktreeItem struct {
	wt engine.Worktree
}

// FilterValue returns the searchable text for filtering.
func (i worktreeItem) FilterValue() string { return i.wt.ID }

// Title returns the worktree name (plain text for filter highlighting).
func (i worktreeItem) Title() string { return i.wt.ID }

// Description returns branch info (single line — status is rendered by the delegate).
func (i worktreeItem) Description() string {
	if i.wt.Branch != "" {
		return "branch: " + i.wt.Branch
	}
	return "branch: (none)"
}

// statusText returns the status line with optional exit code.
func (i worktreeItem) statusText() string {
	s := string(i.wt.State)
	if i.wt.ExitCode != nil && *i.wt.ExitCode > 0 {
		s += fmt.Sprintf(" (exit %d)", *i.wt.ExitCode)
	}
	return s
}

// normalStatusPad matches the normal description padding for status lines.
var normalStatusPad = lipgloss.NewStyle().Padding(0, 0, 0, 2)

// worktreeDelegate renders worktree items as 3 lines:
// title (name), description (branch), and a color-coded status line.
type worktreeDelegate struct {
	base list.DefaultDelegate
}

// newWorktreeDelegate creates a delegate with styled 3-line items.
func newWorktreeDelegate() worktreeDelegate {
	d := list.NewDefaultDelegate()
	d.SetHeight(3)
	d.SetSpacing(1)

	d.Styles.NormalTitle = lipgloss.NewStyle().Padding(0, 0, 0, 2)
	d.Styles.NormalDesc = lipgloss.NewStyle().Padding(0, 0, 0, 2).Foreground(colorGray)

	d.Styles.SelectedTitle = lipgloss.NewStyle().
		Padding(0, 0, 0, 1).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorAccent).
		Foreground(colorAccent).
		Bold(true)
	d.Styles.SelectedDesc = lipgloss.NewStyle().
		Padding(0, 0, 0, 1).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorAccent).
		Foreground(colorSubtle)

	d.Styles.DimmedTitle = lipgloss.NewStyle().Padding(0, 0, 0, 2).Foreground(colorGray)
	d.Styles.DimmedDesc = lipgloss.NewStyle().Padding(0, 0, 0, 2).Foreground(colorGray)

	d.Styles.FilterMatch = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)

	return worktreeDelegate{base: d}
}

// Height returns 3 lines: title + branch + status.
func (d worktreeDelegate) Height() int { return d.base.Height() }

// Spacing returns the gap between items.
func (d worktreeDelegate) Spacing() int { return d.base.Spacing() }

// Update delegates to the base.
func (d worktreeDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return d.base.Update(msg, m)
}

// Render draws 2 lines via the base delegate (title + branch), then
// appends a third line with the status colored by worktree state.
func (d worktreeDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	// Let the base render title + description (2 lines).
	var buf strings.Builder
	d.base.Render(&buf, m, index, item)

	wt, ok := item.(worktreeItem)
	if !ok {
		_, _ = fmt.Fprint(w, buf.String())
		return
	}

	// Determine padding to match the base's style alignment.
	isSelected := index == m.Index() && m.FilterState() != list.Filtering
	statusStyle := components.WorktreeStateStyle(wt.wt.State)

	var statusLine string
	if isSelected {
		// Match selected desc padding (border + 1 pad).
		statusLine = d.base.Styles.SelectedDesc.Render(statusStyle.Render(wt.statusText()))
	} else {
		// Match normal desc padding (2 pad).
		statusLine = normalStatusPad.Render(statusStyle.Render(wt.statusText()))
	}

	_, _ = fmt.Fprint(w, buf.String()+"\n"+statusLine)
}

// ProjectDetailView displays worktrees for a single project.
type ProjectDetailView struct {
	client        Client
	projectID     string
	agentType     string
	projectName   string
	disconnectKey string
	list          list.Model
	worktrees     []engine.Worktree
	loading       bool
	err           error
	keys          WorktreeKeyMap
	width         int
	height        int
	// New worktree input overlay.
	showNewInput     bool
	newWorktreeInput textinput.Model
}

// NewProjectDetailView creates a project detail view.
func NewProjectDetailView(client Client, projectID, agentType, projectName, disconnectKey string) *ProjectDetailView {
	delegate := newWorktreeDelegate()

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = projectName
	l.SetShowTitle(true)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return &ProjectDetailView{
		client:        client,
		projectID:     projectID,
		agentType:     agentType,
		projectName:   projectName,
		disconnectKey: disconnectKey,
		list:          l,
		loading:       true,
		keys:          DefaultWorktreeKeyMap(),
	}
}

// Init fetches the worktree list.
func (v *ProjectDetailView) Init() tea.Cmd {
	v.loading = true
	return loadWorktrees(v.client, v.projectID, v.agentType)
}

// Update handles messages for the project detail view.
func (v *ProjectDetailView) Update(msg tea.Msg) (View, tea.Cmd) {
	// If the new worktree input is active, delegate to it.
	if v.showNewInput {
		return v.updateNewWorktreeInput(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		contentW := msg.Width - 4
		v.list.SetSize(contentW, msg.Height-7)
		return v, nil

	case WorktreesLoadedMsg:
		v.loading = false
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		v.worktrees = msg.Worktrees
		items := make([]list.Item, len(v.worktrees))
		for i, wt := range v.worktrees {
			items[i] = worktreeItem{wt: wt}
		}
		cmd := v.list.SetItems(items)
		return v, cmd

	case OperationResultMsg:
		if msg.Err != nil {
			v.err = msg.Err
		}
		return v, loadWorktrees(v.client, v.projectID, v.agentType)

	case SSEEventMsg:
		evt := eventbus.SSEEvent(msg)
		switch evt.Event {
		case eventbus.SSEWorktreeState, eventbus.SSEWorktreeListChanged:
			return v, loadWorktrees(v.client, v.projectID, v.agentType)
		}
		return v, nil

	case TerminalExitedMsg:
		var cmds []tea.Cmd
		if selected, ok := v.list.SelectedItem().(worktreeItem); ok {
			cmds = append(cmds, disconnectTerminal(v.client, v.projectID, v.agentType, selected.wt.ID))
		}
		cmds = append(cmds, loadWorktrees(v.client, v.projectID, v.agentType))
		return v, tea.Batch(cmds...)

	case tea.KeyPressMsg:
		if v.err != nil {
			v.err = nil
			return v, loadWorktrees(v.client, v.projectID, v.agentType)
		}
		// Don't handle keys while filtering.
		if v.list.FilterState() == list.Filtering {
			break
		}
		return v.handleKey(msg)
	}

	// Delegate to the list for cursor movement, filtering, etc.
	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return v, cmd
}

func (v *ProjectDetailView) updateNewWorktreeInput(msg tea.Msg) (View, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			v.showNewInput = false
			return v, nil
		case "enter":
			name := v.newWorktreeInput.Value()
			if name != "" {
				v.showNewInput = false
				return v, createWorktree(v.client, v.projectID, v.agentType, name)
			}
			return v, nil
		}
	}

	var cmd tea.Cmd
	v.newWorktreeInput, cmd = v.newWorktreeInput.Update(msg)
	return v, cmd
}

func (v *ProjectDetailView) openNewWorktreeInput() tea.Cmd {
	ti := textinput.New()
	ti.Placeholder = "feature-name"
	ti.Prompt = "> "
	v.newWorktreeInput = ti
	v.showNewInput = true
	return ti.Focus()
}

// Render renders the project detail view.
func (v *ProjectDetailView) Render(width, height int) string {
	// Show new worktree form overlay.
	if v.showNewInput {
		s := Styles.Muted.Render("← ") + Styles.Bold.Render(v.projectName) + "\n\n"
		s += Styles.Bold.Render("New Worktree") + "\n"
		s += Styles.Muted.Render("Letters, numbers, hyphens, and underscores.") + "\n\n"
		s += v.newWorktreeInput.View() + "\n\n"
		s += Styles.Muted.Render("enter to create · esc to cancel")
		return s
	}

	if v.loading {
		return Styles.Muted.Render("← ") + Styles.Bold.Render(v.projectName) + "\n\nLoading worktrees..."
	}
	if v.err != nil {
		return Styles.Muted.Render("← ") + Styles.Bold.Render(v.projectName) + "\n\n" +
			Styles.Error.Render("Error: "+v.err.Error())
	}

	return v.list.View()
}

// HelpKeyMap returns the project detail view's key bindings for the help bar.
func (v *ProjectDetailView) HelpKeyMap() help.KeyMap {
	disconnectHint := key.NewBinding(
		key.WithKeys(""),
		key.WithHelp(v.disconnectKey, "disconnect"),
	)
	return worktreeHelpKeyMap{keys: v.keys, disconnectHint: disconnectHint}
}

// worktreeHelpKeyMap wraps WorktreeKeyMap to include the disconnect key hint.
type worktreeHelpKeyMap struct {
	keys           WorktreeKeyMap
	disconnectHint key.Binding
}

// filterBinding is shown in the help bar to indicate filtering.
var filterBinding = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter"))

func (k worktreeHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.keys.Connect, k.keys.New, filterBinding, k.keys.Back, moreHelp}
}

func (k worktreeHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.keys.Connect, k.disconnectHint, k.keys.Disconnect, k.keys.Kill},
		{k.keys.Remove, k.keys.New, k.keys.Cleanup, filterBinding, k.keys.Back},
	}
}

func (v *ProjectDetailView) handleKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	selected, hasSelected := v.list.SelectedItem().(worktreeItem)

	switch {
	case key.Matches(msg, v.keys.Back):
		return v, func() tea.Msg { return NavigateBackMsg{} }

	case key.Matches(msg, v.keys.Connect):
		if hasSelected {
			return v, attachTerminal(v.client, v.projectID, v.agentType, selected.wt.ID)
		}

	case key.Matches(msg, v.keys.Disconnect):
		if hasSelected {
			return v, disconnectTerminal(v.client, v.projectID, v.agentType, selected.wt.ID)
		}

	case key.Matches(msg, v.keys.Kill):
		if hasSelected {
			return v, killWorktree(v.client, v.projectID, v.agentType, selected.wt.ID)
		}

	case key.Matches(msg, v.keys.Remove):
		if hasSelected {
			return v, removeWorktree(v.client, v.projectID, v.agentType, selected.wt.ID)
		}

	case key.Matches(msg, v.keys.New):
		cmd := v.openNewWorktreeInput()
		return v, cmd

	case key.Matches(msg, v.keys.Cleanup):
		return v, cleanupWorktrees(v.client, v.projectID, v.agentType)
	}

	// Delegate to list for cursor movement.
	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return v, cmd
}

// --- Commands ---

func loadWorktrees(client Client, projectID, agentType string) tea.Cmd {
	return func() tea.Msg {
		worktrees, err := client.ListWorktrees(context.Background(), projectID, agentType)
		return WorktreesLoadedMsg{Worktrees: worktrees, Err: err}
	}
}

// attachTerminal starts the terminal process and opens a viewer connection.
func attachTerminal(c Client, projectID, agentType, worktreeID string) tea.Cmd {
	return func() tea.Msg {
		_, err := c.ConnectTerminal(context.Background(), projectID, agentType, worktreeID)
		if err != nil {
			return TerminalExitedMsg{Err: err}
		}
		conn, err := c.AttachTerminal(context.Background(), projectID, worktreeID)
		if err != nil {
			return TerminalExitedMsg{Err: err}
		}
		return execTerminalMsg{conn: conn}
	}
}

// execTerminalMsg carries the terminal connection to be exec'd.
type execTerminalMsg struct {
	conn client.TerminalConnection
}

func createWorktree(client Client, projectID, agentType, name string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.CreateWorktree(context.Background(), projectID, agentType, name)
		return OperationResultMsg{Operation: "create", Err: err}
	}
}

func disconnectTerminal(client Client, projectID, agentType, worktreeID string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.DisconnectTerminal(context.Background(), projectID, agentType, worktreeID)
		return OperationResultMsg{Operation: "disconnect", Err: err}
	}
}

func killWorktree(client Client, projectID, agentType, worktreeID string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.KillWorktreeProcess(context.Background(), projectID, agentType, worktreeID)
		return OperationResultMsg{Operation: "kill", Err: err}
	}
}

func removeWorktree(client Client, projectID, agentType, worktreeID string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.RemoveWorktree(context.Background(), projectID, agentType, worktreeID)
		return OperationResultMsg{Operation: "remove", Err: err}
	}
}

func cleanupWorktrees(client Client, projectID, agentType string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.CleanupWorktrees(context.Background(), projectID, agentType)
		return OperationResultMsg{Operation: "cleanup", Err: err}
	}
}

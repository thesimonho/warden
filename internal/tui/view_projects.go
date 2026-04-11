package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/event"
	"github.com/thesimonho/warden/internal/tui/components"
)

// ProjectsView displays the project list with status, worktree counts, and cost.
type ProjectsView struct {
	client      Client
	projects    []api.ProjectResponse
	table       table.Model
	tableStyles table.Styles
	loading     bool
	err         error
	keys        ProjectKeyMap
	width       int
}

// NewProjectsView creates a new project list view.
func NewProjectsView(client Client) *ProjectsView {
	ts := tableStyles()
	columns := projectColumns(0)
	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(15),
		table.WithStyles(ts),
	)

	return &ProjectsView{
		client:      client,
		table:       t,
		tableStyles: ts,
		loading:     true,
		keys:        DefaultProjectKeyMap(),
	}
}

// Init fetches the project list.
func (v *ProjectsView) Init() tea.Cmd {
	v.loading = true
	return loadProjects(v.client)
}

// Update handles messages for the project list.
func (v *ProjectsView) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		contentW := msg.Width - 4 // subtract app padding
		v.tableStyles.Selected = v.tableStyles.Selected.Width(contentW)
		v.table.SetStyles(v.tableStyles)
		v.table.SetColumns(projectColumns(contentW))
		v.table.SetWidth(contentW)
		return v, nil

	case ProjectsLoadedMsg:
		v.loading = false
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		v.projects = msg.Projects
		v.table.SetRows(projectRows(v.projects))
		return v, nil

	case OperationResultMsg:
		if msg.Err != nil {
			v.err = msg.Err
		}
		return v, loadProjects(v.client)

	case SSEEventMsg:
		// Only reload on state/cost changes, not heartbeats.
		evt := event.SSEEvent(msg)
		switch evt.Event {
		case event.SSEWorktreeState, event.SSEProjectState, event.SSEWorktreeListChanged:
			return v, loadProjects(v.client)
		}
		return v, nil

	case tea.KeyPressMsg:
		// Any keypress dismisses the error overlay.
		if v.err != nil {
			v.err = nil
			return v, loadProjects(v.client)
		}
		return v.handleKey(msg)
	}

	// Delegate to table for scroll handling.
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return v, cmd
}

// Render renders the project list.
func (v *ProjectsView) Render(width, height int) string {
	if v.loading {
		return "Loading projects..."
	}
	if v.err != nil {
		return Styles.Error.Render("Error: " + v.err.Error())
	}
	if len(v.projects) == 0 {
		return Styles.Muted.Render("No projects configured. Press 'n' to create one.")
	}

	var s strings.Builder

	// Summary bar.
	running, activeWorktrees, totalCost := summarizeProjects(v.projects)
	s.WriteString(Styles.Bold.Render("Running: ") + fmt.Sprintf("%d", running) + "  ")
	s.WriteString(Styles.Bold.Render("Active: ") + fmt.Sprintf("%d", activeWorktrees) + "  ")
	s.WriteString(Styles.Bold.Render("Cost: ") + components.FormatCost(totalCost))
	s.WriteString("\n\n")

	// Adjust table height to fill available space.
	// Subtract 2 for summary bar + blank line above the table.
	tableHeight := height - 2
	if tableHeight < 5 {
		tableHeight = 5
	}
	v.table.SetHeight(tableHeight)

	s.WriteString(v.table.View())

	return s.String()
}

// HelpKeyMap returns the project view's key bindings for the help bar.
// Bindings are enabled/disabled based on the selected project's state.
func (v *ProjectsView) HelpKeyMap() help.KeyMap {
	selected := v.selectedProject()
	isRunning := selected != nil && selected.State == components.ContainerStateRunning
	v.keys.Open.SetEnabled(isRunning)
	return v.keys
}

func (v *ProjectsView) handleKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	selected := v.selectedProject()

	switch {
	case key.Matches(msg, v.keys.Open):
		// Only open running containers.
		if selected != nil && selected.State == components.ContainerStateRunning {
			return v, func() tea.Msg {
				return NavigateMsg{
					Tab:            TabProjects,
					ProjectID:      selected.ProjectID,
					AgentType:      string(selected.AgentType),
					ProjectName:    selected.Name,
					ForwardedPorts: selected.ForwardedPorts,
				}
			}
		}

	case key.Matches(msg, v.keys.Edit):
		if selected == nil || selected.ProjectID == "" {
			break
		}
		if selected.HasContainer {
			formView := NewContainerEditView(v.client, selected.ProjectID, string(selected.AgentType))
			return formView, formView.Init()
		}
		// No container — open create form pre-filled with project info.
		formView := NewContainerFormView(v.client)
		return formView, formView.Init()

	case key.Matches(msg, v.keys.Toggle):
		if selected != nil && selected.HasContainer {
			if selected.State == components.ContainerStateRunning {
				return v, stopProject(v.client, selected.ProjectID, string(selected.AgentType))
			}
			return v, restartProject(v.client, selected.ProjectID, string(selected.AgentType))
		}

	case key.Matches(msg, v.keys.New):
		formView := NewContainerFormView(v.client)
		return formView, formView.Init()

	case key.Matches(msg, v.keys.Remove):
		if selected != nil {
			manageView := NewManageProjectView(v.client, selected.ProjectID, string(selected.AgentType), selected.Name, selected.HasContainer)
			return manageView, manageView.Init()
		}

	case key.Matches(msg, v.keys.Refresh):
		return v, loadProjects(v.client)
	}

	// Delegate to table for cursor movement.
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return v, cmd
}

func (v *ProjectsView) selectedProject() *api.ProjectResponse {
	cursor := v.table.Cursor()
	if cursor >= 0 && cursor < len(v.projects) {
		return &v.projects[cursor]
	}
	return nil
}

// --- Table helpers ---

func projectColumns(width int) []table.Column {
	// Fixed-width columns; Name gets all remaining space.
	// Each column has 2 chars of cell padding (Padding(0,1) on each side).
	const agentW, statusW, worktreeW, costW, networkW = 16, 12, 12, 10, 14
	const numCols = 6
	const cellPadding = 2 * numCols
	fixed := agentW + statusW + worktreeW + costW + networkW
	nameW := width - fixed - cellPadding
	if nameW < 12 {
		nameW = 12
	}
	return []table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Agent", Width: agentW},
		{Title: "Status", Width: statusW},
		{Title: "Worktrees", Width: worktreeW},
		{Title: "Cost", Width: costW},
		{Title: "Network", Width: networkW},
	}
}

func projectRows(projects []api.ProjectResponse) []table.Row {
	rows := make([]table.Row, len(projects))
	for i, p := range projects {
		state := p.State
		if !p.HasContainer {
			state = "no container"
		}

		worktreeInfo := fmt.Sprintf("%d active", p.ActiveWorktreeCount)
		cost := components.FormatBudgetProgress(p.TotalCost, p.CostBudget)

		agentLabel := agent.ShortLabel(p.AgentType)
		if p.AgentVersion != "" {
			agentLabel += " " + p.AgentVersion
		}

		rows[i] = table.Row{
			p.Name,
			agentLabel,
			state,
			worktreeInfo,
			cost,
			string(p.NetworkMode),
		}
	}
	return rows
}

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		Bold(true).
		Foreground(colorSubtle)
	s.Selected = s.Selected.
		Foreground(colorWhite).
		Background(colorAccent).
		Bold(true)
	return s
}

// --- Commands ---

func loadProjects(client Client) tea.Cmd {
	return func() tea.Msg {
		projects, err := client.ListProjects(context.Background())
		return ProjectsLoadedMsg{Projects: projects, Err: err}
	}
}

func stopProject(client Client, id, agentType string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.StopProject(context.Background(), id, agentType)
		return OperationResultMsg{Operation: "stop", Err: err}
	}
}

func restartProject(client Client, id, agentType string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.RestartProject(context.Background(), id, agentType)
		return OperationResultMsg{Operation: "restart", Err: err}
	}
}

// --- Helpers ---

func summarizeProjects(projects []api.ProjectResponse) (running, activeWorktrees int, totalCost float64) {
	for _, p := range projects {
		if p.State == components.ContainerStateRunning {
			running++
		}
		activeWorktrees += p.ActiveWorktreeCount
		totalCost += p.TotalCost
	}
	return
}

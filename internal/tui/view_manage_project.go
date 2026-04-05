package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// manageAction identifies a toggle in the manage project dialog.
type manageAction int

const (
	actionRemove manageAction = iota
	actionDeleteContainer
	actionResetCosts
	actionCount // sentinel — number of actions
)

// ManageProjectView is an inline overlay for managing a project.
// It presents three independent destructive actions as toggleable
// checkboxes, mirroring the web delete-project-dialog.
type ManageProjectView struct {
	client    Client
	projectID string
	agentType string
	name      string
	// Whether the project has a container.
	hasContainer bool

	// Checkbox states — all unchecked by default.
	checked [actionCount]bool
	cursor  manageAction

	// Execution state.
	executing bool
	err       error

	keys ManageKeyMap
}

// NewManageProjectView creates a manage dialog for the given project.
func NewManageProjectView(client Client, projectID, agentType, name string, hasContainer bool) *ManageProjectView {
	return &ManageProjectView{
		client:       client,
		projectID:    projectID,
		agentType:    agentType,
		name:         name,
		hasContainer: hasContainer,
		keys:         DefaultManageKeyMap(),
	}
}

// Init returns nil — all state is passed in at construction time.
func (v *ManageProjectView) Init() tea.Cmd { return nil }

// Update handles messages for the manage dialog.
func (v *ManageProjectView) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case OperationResultMsg:
		v.executing = false
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		return v, func() tea.Msg { return NavigateBackMsg{} }

	case tea.KeyPressMsg:
		if v.err != nil {
			v.err = nil
			return v, nil
		}

		switch {
		case key.Matches(msg, v.keys.Back):
			return v, func() tea.Msg { return NavigateBackMsg{} }

		case key.Matches(msg, v.keys.Up):
			v.moveCursor(-1)

		case key.Matches(msg, v.keys.Down):
			v.moveCursor(1)

		case key.Matches(msg, v.keys.Toggle):
			v.toggleCurrent()

		case key.Matches(msg, v.keys.Confirm):
			return v.startExecution()
		}
	}

	return v, nil
}

// Render draws the manage project overlay.
func (v *ManageProjectView) Render(width, height int) string {
	if v.err != nil {
		return Styles.Error.Render("Error: " + v.err.Error())
	}
	if v.executing {
		return Styles.Muted.Render("Processing…")
	}

	var s strings.Builder
	s.WriteString(Styles.Bold.Render("Manage Project") + "\n")
	s.WriteString(Styles.Muted.Render(v.name) + "\n\n")

	labels := [actionCount]string{
		"Remove from Warden",
		"Delete container",
		"Reset cost history",
	}
	descs := [actionCount]string{
		"Untrack this project from Warden",
		"Stop and permanently remove the container",
		"Clear all tracked cost data",
	}

	for i := manageAction(0); i < actionCount; i++ {
		cursor := "  "
		if i == v.cursor {
			cursor = "> "
		}

		checkbox := "[ ]"
		if v.checked[i] {
			checkbox = "[x]"
		}

		disabled := i == actionDeleteContainer && !v.hasContainer
		label := labels[i]
		desc := descs[i]

		if disabled {
			label = Styles.Muted.Render(label)
			desc = "No container exists"
			checkbox = Styles.Muted.Render("[ ]")
		}

		s.WriteString(cursor + checkbox + " " + label + "\n")
		s.WriteString("      " + Styles.Muted.Render(desc) + "\n")
	}

	if v.checked[actionRemove] && !v.checked[actionDeleteContainer] && v.hasContainer {
		s.WriteString("\n")
		s.WriteString(Styles.Warning.Render("⚠ The container will remain on disk.") + "\n")
	}

	s.WriteString("\n")
	if v.hasAnyChecked() {
		s.WriteString(Styles.Muted.Render("enter to confirm · space to toggle · esc to cancel"))
	} else {
		s.WriteString(Styles.Muted.Render("space to toggle · esc to cancel"))
	}

	return s.String()
}

// HelpKeyMap returns the manage dialog's key bindings for the help bar.
func (v *ManageProjectView) HelpKeyMap() help.KeyMap {
	return v.keys
}

// --- Key handling helpers ---

func (v *ManageProjectView) moveCursor(delta int) {
	next := int(v.cursor) + delta
	if next < 0 {
		next = int(actionCount) - 1
	} else if next >= int(actionCount) {
		next = 0
	}
	v.cursor = manageAction(next)
}

func (v *ManageProjectView) toggleCurrent() {
	if v.cursor == actionDeleteContainer && !v.hasContainer {
		return
	}
	v.checked[v.cursor] = !v.checked[v.cursor]
}

func (v *ManageProjectView) hasAnyChecked() bool {
	for _, c := range v.checked {
		if c {
			return true
		}
	}
	return false
}

// startExecution validates and begins executing the selected actions.
func (v *ManageProjectView) startExecution() (View, tea.Cmd) {
	if !v.hasAnyChecked() {
		return v, nil
	}

	v.executing = true
	return v, v.executeActions()
}

// executeActions runs the selected actions in the correct order and
// returns an OperationResultMsg when done. Container deletion runs
// first, cost reset runs next, and project removal runs last so
// earlier steps can still resolve the project row.
func (v *ManageProjectView) executeActions() tea.Cmd {
	projectID := v.projectID
	agentType := v.agentType
	client := v.client
	doDelete := v.checked[actionDeleteContainer] && v.hasContainer
	doReset := v.checked[actionResetCosts]
	doRemove := v.checked[actionRemove]

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		var errors []string

		if doDelete {
			if _, err := client.DeleteContainer(ctx, projectID, agentType); err != nil {
				errors = append(errors, fmt.Sprintf("delete container: %v", err))
			}
		}

		if doReset {
			if err := client.ResetProjectCosts(ctx, projectID, agentType); err != nil {
				errors = append(errors, fmt.Sprintf("reset costs: %v", err))
			}
		}

		if doRemove {
			if _, err := client.RemoveProject(ctx, projectID, agentType); err != nil {
				errors = append(errors, fmt.Sprintf("remove project: %v", err))
			}
		}

		if len(errors) > 0 {
			return OperationResultMsg{
				Operation: "manage",
				Err:       fmt.Errorf("failed: %s", strings.Join(errors, ", ")),
			}
		}
		return OperationResultMsg{Operation: "manage", Err: nil}
	}
}

// --- Key bindings ---

// ManageKeyMap defines key bindings for the manage project dialog.
type ManageKeyMap struct {
	Toggle  key.Binding
	Confirm key.Binding
	Back    key.Binding
	Up      key.Binding
	Down    key.Binding
}

// DefaultManageKeyMap returns the default manage dialog key bindings.
func DefaultManageKeyMap() ManageKeyMap {
	return ManageKeyMap{
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
	}
}

// Compile-time check.
var _ help.KeyMap = ManageKeyMap{}

// ShortHelp returns bindings shown in the compact help bar.
func (k ManageKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Toggle, k.Confirm, k.Back}
}

// FullHelp returns bindings shown in expanded help.
func (k ManageKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Toggle},
		{k.Confirm, k.Back},
	}
}

package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// manageAction identifies a toggle in the manage project dialog.
type manageAction int

const (
	actionRemove manageAction = iota
	actionDeleteContainer
	actionResetCosts
	actionPurgeAudit
	actionCount // sentinel — number of actions
)

// purgeConfirmWord returns the text the user must type to confirm audit purge.
// Uses the project name so the confirmation is project-specific.
func purgeConfirmWord(name string) string {
	return name
}

// ManageProjectView is an inline overlay for managing a project.
// It presents four independent destructive actions as toggleable
// checkboxes, mirroring the web manage-project-dialog.
type ManageProjectView struct {
	client    Client
	projectID string
	name      string
	// Whether the project has a container.
	hasContainer bool

	// Checkbox states — all unchecked by default.
	checked [actionCount]bool
	cursor  manageAction

	// Purge audit confirmation input.
	confirmInput textinput.Model
	confirming   bool

	// Execution state.
	executing bool
	err       error

	keys ManageKeyMap
}

// NewManageProjectView creates a manage dialog for the given project.
func NewManageProjectView(client Client, projectID, name string, hasContainer bool) *ManageProjectView {
	ti := textinput.New()
	ti.Placeholder = purgeConfirmWord(name)
	ti.Prompt = "> "
	ti.CharLimit = len(name) + 5

	return &ManageProjectView{
		client:       client,
		projectID:    projectID,
		name:         name,
		hasContainer: hasContainer,
		confirmInput: ti,
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
		if v.confirming {
			return v.updateConfirm(msg)
		}

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
		"Purge audit history",
	}
	descs := [actionCount]string{
		"Untrack this project from Warden",
		"Stop and permanently remove the container",
		"Clear all tracked cost data",
		"Permanently delete all audit events",
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

	if v.confirming {
		s.WriteString("\n")
		s.WriteString(Styles.Error.Render("Type '"+purgeConfirmWord(v.name)+"' to confirm audit deletion:") + "\n")
		s.WriteString(v.confirmInput.View() + "\n")
		s.WriteString(Styles.Muted.Render("enter to confirm · esc to cancel") + "\n")
		return s.String()
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
// If purge-audit is checked, shows the text confirmation first.
func (v *ManageProjectView) startExecution() (View, tea.Cmd) {
	if !v.hasAnyChecked() {
		return v, nil
	}

	if v.checked[actionPurgeAudit] && !v.confirming {
		v.confirming = true
		return v, v.confirmInput.Focus()
	}

	v.executing = true
	return v, v.executeActions()
}

// updateConfirm handles key presses during the purge confirmation input.
func (v *ManageProjectView) updateConfirm(msg tea.KeyPressMsg) (View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.confirming = false
		v.confirmInput.SetValue("")
		return v, nil
	case "enter":
		if v.confirmInput.Value() == purgeConfirmWord(v.name) {
			v.confirming = false
			v.executing = true
			return v, v.executeActions()
		}
		return v, nil
	}

	var cmd tea.Cmd
	v.confirmInput, cmd = v.confirmInput.Update(msg)
	return v, cmd
}

// executeActions runs the selected actions in the correct order and
// returns an OperationResultMsg when done. Container deletion runs
// first, cost reset and audit purge run concurrently, and project
// removal runs last so earlier steps can still resolve the project row.
func (v *ManageProjectView) executeActions() tea.Cmd {
	projectID := v.projectID
	client := v.client
	doDelete := v.checked[actionDeleteContainer] && v.hasContainer
	doReset := v.checked[actionResetCosts]
	doPurge := v.checked[actionPurgeAudit]
	doRemove := v.checked[actionRemove]

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		var errors []string

		if doDelete {
			if _, err := client.DeleteContainer(ctx, projectID); err != nil {
				errors = append(errors, fmt.Sprintf("delete container: %v", err))
			}
		}

		// Cost reset and audit purge are independent — run concurrently.
		type result struct {
			label string
			err   error
		}
		ch := make(chan result, 2)
		concurrent := 0

		if doReset {
			concurrent++
			go func() {
				ch <- result{"reset costs", client.ResetProjectCosts(ctx, projectID)}
			}()
		}
		if doPurge {
			concurrent++
			go func() {
				ch <- result{"purge audit", client.PurgeProjectAudit(ctx, projectID)}
			}()
		}
		for range concurrent {
			r := <-ch
			if r.err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", r.label, r.err))
			}
		}

		if doRemove {
			if _, err := client.RemoveProject(ctx, projectID); err != nil {
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
		{k.Up, k.Down, k.Toggle, k.Confirm, k.Back},
	}
}

package tui

import (
	"context"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/internal/tui/components"
)

func (v *ContainerFormView) handleKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	if v.browsing && v.dirBrowser != nil {
		return v.handleBrowsingKey(msg)
	}
	if v.editingMount {
		// Required mounts only allow editing the host path — skip tab to container path.
		if msg.String() == "tab" && v.isRequiredMount(v.mountCursor) {
			return v, nil
		}
		return v.handleInlineEditKey(msg, &v.mountInputs, v.cancelMountEdit, v.saveMountInputs)
	}
	if v.editingEnv {
		return v.handleInlineEditKey(msg, &v.envInputs, v.cancelEnvEdit, v.saveEnvInputs)
	}
	if v.editing {
		if msg.String() == "esc" {
			v.blurActiveField()
			v.editing = false
			return v, nil
		}
		return v.updateActiveField(msg)
	}

	// Navigation mode.
	switch {
	case msg.String() == "esc":
		return v, func() tea.Msg { return NavigateBackMsg{} }
	case msg.String() == "up" || msg.String() == "k":
		v.moveCursor(-1)
	case msg.String() == "down" || msg.String() == "j":
		v.moveCursor(1)
	case msg.String() == "enter" || msg.String() == " ":
		return v.activateField()
	case msg.String() == "tab":
		return v.cycleSelection()
	case msg.String() == "x":
		return v.removeCurrentItem()
	case msg.String() == "r":
		if v.cursor == fieldMounts && v.mountCursor >= 0 && v.mountCursor < len(v.mounts) {
			v.mounts[v.mountCursor].ReadOnly = !v.mounts[v.mountCursor].ReadOnly
		}
	}

	return v, nil
}

// handleInlineEditKey handles keys for mount/env inline editing.
// The two-input tab/enter/esc pattern is identical for both.
func (v *ContainerFormView) handleInlineEditKey(
	msg tea.KeyPressMsg,
	inputs *[2]textinput.Model,
	cancelFn func(),
	saveFn func(),
) (View, tea.Cmd) {
	switch msg.String() {
	case "enter":
		saveFn()
		return v, nil
	case "esc":
		cancelFn()
		return v, nil
	case "tab":
		if inputs[0].Focused() {
			inputs[0].Blur()
			return v, inputs[1].Focus()
		}
		inputs[1].Blur()
		return v, inputs[0].Focus()
	}
	return v.updateActiveField(msg)
}

func (v *ContainerFormView) saveMountInputs() {
	if v.mountCursor >= 0 && v.mountCursor < len(v.mounts) {
		v.mounts[v.mountCursor].HostPath = v.mountInputs[0].Value()
		v.mounts[v.mountCursor].ContainerPath = v.mountInputs[1].Value()
	}
	v.editingMount = false
	v.mountIsNew = false
	v.mountInputs[0].Blur()
	v.mountInputs[1].Blur()
}

func (v *ContainerFormView) cancelMountEdit() {
	if v.mountIsNew {
		v.mounts = append(v.mounts[:v.mountCursor], v.mounts[v.mountCursor+1:]...)
		if v.mountCursor >= len(v.mounts) {
			v.mountCursor = max(len(v.mounts)-1, -1)
		}
	}
	v.editingMount = false
	v.mountIsNew = false
	v.mountInputs[0].Blur()
	v.mountInputs[1].Blur()
}

func (v *ContainerFormView) saveEnvInputs() {
	if v.envCursor >= 0 && v.envCursor < len(v.envVars) {
		v.envVars[v.envCursor].key = v.envInputs[0].Value()
		v.envVars[v.envCursor].value = v.envInputs[1].Value()
	}
	v.editingEnv = false
	v.envIsNew = false
	v.envInputs[0].Blur()
	v.envInputs[1].Blur()
}

func (v *ContainerFormView) cancelEnvEdit() {
	if v.envIsNew {
		v.envVars = append(v.envVars[:v.envCursor], v.envVars[v.envCursor+1:]...)
		if v.envCursor >= len(v.envVars) {
			v.envCursor = max(len(v.envVars)-1, -1)
		}
	}
	v.editingEnv = false
	v.envIsNew = false
	v.envInputs[0].Blur()
	v.envInputs[1].Blur()
}

func (v *ContainerFormView) handleBrowsingKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.browsing = false
		v.dirBrowser = nil
		return v, nil
	case "space", " ":
		v.inputs[1].SetValue(v.dirBrowser.Path())
		v.browsing = false
		v.dirBrowser = nil
		return v, nil
	}
	var cmd tea.Cmd
	v.dirBrowser, cmd = v.dirBrowser.Update(msg)
	return v, cmd
}

// cycleSelection cycles the value of selection fields (tab key).
func (v *ContainerFormView) cycleSelection() (View, tea.Cmd) {
	switch v.cursor {
	case fieldAgentType:
		if v.editID == "" { // read-only in edit mode
			v.agentType = (v.agentType + 1) % len(agentTypes)
			v.refilterDefaultMounts()
			v.domains.SetValue(defaultDomainsForAgent(v.restrictedDomains, agentTypes[v.agentType]))
		}
	case fieldNetwork:
		v.network = (v.network + 1) % len(networkModes)
	case fieldSkipPerms:
		v.skipPerm = !v.skipPerm
	case fieldRuntimes:
		if v.runtimeCursor >= 0 && v.runtimeCursor < len(v.runtimeDefaults) {
			v.toggleRuntime(v.runtimeDefaults[v.runtimeCursor].ID)
		}
	case fieldAccessItems:
		if v.accessCursor >= 0 && v.accessCursor < len(v.accessItems) {
			v.toggleAccessItem(v.accessItems[v.accessCursor].ID)
		}
	}
	return v, nil
}

// isFieldVisible returns whether a field should be shown.
func (v *ContainerFormView) isFieldVisible(field int) bool {
	switch field {
	case fieldDomains:
		return networkModes[v.network] == "restricted"
	case fieldImage, fieldAccessItems, fieldMounts, fieldEnvVars:
		return v.advancedOpen
	}
	return true
}

// moveCursor moves the cursor by delta, skipping hidden fields.
// For access/mount/env sections, navigates sub-items.
func (v *ContainerFormView) moveCursor(delta int) {
	if v.cursor == fieldRuntimes {
		next := v.runtimeCursor + delta
		if next < 0 {
			v.runtimeCursor = 0
			v.moveCursorField(delta)
			return
		}
		if next >= len(v.runtimeDefaults) {
			v.moveCursorField(delta)
			return
		}
		v.runtimeCursor = next
		return
	}

	if v.cursor == fieldAccessItems {
		next := v.accessCursor + delta
		if next < 0 {
			v.accessCursor = 0
			v.moveCursorField(delta)
			return
		}
		if next >= len(v.accessItems) {
			v.moveCursorField(delta)
			return
		}
		v.accessCursor = next
		return
	}

	if v.cursor == fieldMounts {
		next := v.mountCursor + delta
		if next < -1 {
			v.mountCursor = -1
			v.moveCursorField(delta)
			return
		}
		if next >= len(v.mounts) {
			v.moveCursorField(delta)
			return
		}
		v.mountCursor = next
		return
	}

	if v.cursor == fieldEnvVars {
		next := v.envCursor + delta
		if next < -1 {
			v.envCursor = -1
			v.moveCursorField(delta)
			return
		}
		if next >= len(v.envVars) {
			v.moveCursorField(delta)
			return
		}
		v.envCursor = next
		return
	}

	v.moveCursorField(delta)
}

// moveCursorField moves the main field cursor, skipping hidden fields.
func (v *ContainerFormView) moveCursorField(delta int) {
	next := v.cursor + delta
	for next >= 0 && next < fieldCount {
		if v.isFieldVisible(next) {
			v.cursor = next
			if next == fieldRuntimes {
				if delta > 0 {
					v.runtimeCursor = 0
				} else {
					v.runtimeCursor = max(len(v.runtimeDefaults)-1, 0)
				}
			}
			if next == fieldAccessItems {
				if delta > 0 {
					v.accessCursor = 0
				} else {
					v.accessCursor = max(len(v.accessItems)-1, 0)
				}
			}
			if next == fieldMounts {
				if delta > 0 {
					v.mountCursor = -1
				} else {
					v.mountCursor = max(len(v.mounts)-1, -1)
				}
			}
			if next == fieldEnvVars {
				if delta > 0 {
					v.envCursor = -1
				} else {
					v.envCursor = max(len(v.envVars)-1, -1)
				}
			}
			return
		}
		next += delta
	}
}

func (v *ContainerFormView) activateField() (View, tea.Cmd) {
	switch v.cursor {
	case fieldName, fieldImage:
		idx := 0
		if v.cursor == fieldImage {
			idx = 2
		}
		v.editing = true
		return v, v.inputs[idx].Focus()

	case fieldBudget:
		v.editing = true
		return v, v.budgetInput.Focus()

	case fieldPath:
		return v.openDirectoryBrowser()

	case fieldDomains:
		v.editing = true
		return v, v.domains.Focus()

	case fieldAgentType:
		if v.editID == "" {
			v.agentType = (v.agentType + 1) % len(agentTypes)
			v.refilterDefaultMounts()
		}
	case fieldNetwork:
		v.network = (v.network + 1) % len(networkModes)
	case fieldSkipPerms:
		v.skipPerm = !v.skipPerm
	case fieldAdvanced:
		v.advancedOpen = !v.advancedOpen
	case fieldRuntimes:
		if v.runtimeCursor >= 0 && v.runtimeCursor < len(v.runtimeDefaults) {
			v.toggleRuntime(v.runtimeDefaults[v.runtimeCursor].ID)
		}
	case fieldAccessItems:
		if v.accessCursor >= 0 && v.accessCursor < len(v.accessItems) {
			v.toggleAccessItem(v.accessItems[v.accessCursor].ID)
		}

	case fieldMounts:
		return v.activateMountField()
	case fieldEnvVars:
		return v.activateEnvField()
	case fieldSubmit:
		return v, v.submit()
	}
	return v, nil
}

func (v *ContainerFormView) activateMountField() (View, tea.Cmd) {
	if v.mountCursor == -1 {
		v.mounts = append(v.mounts, api.Mount{ReadOnly: true})
		v.mountCursor = len(v.mounts) - 1
		v.mountIsNew = true
		return v.startMountEdit()
	}
	if v.mountCursor >= 0 && v.mountCursor < len(v.mounts) {
		v.mountIsNew = false
		return v.startMountEdit()
	}
	return v, nil
}

func (v *ContainerFormView) startMountEdit() (View, tea.Cmd) {
	m := v.mounts[v.mountCursor]
	v.mountInputs[0].SetValue(m.HostPath)
	v.mountInputs[1].SetValue(m.ContainerPath)
	v.editingMount = true
	// Container path must stay at the agent's expected location;
	// only let the user remap which host directory backs it.
	if v.isRequiredMount(v.mountCursor) {
		v.mountInputs[1].Blur()
	}
	return v, v.mountInputs[0].Focus()
}

func (v *ContainerFormView) activateEnvField() (View, tea.Cmd) {
	if v.envCursor == -1 {
		v.envVars = append(v.envVars, envVarEntry{})
		v.envCursor = len(v.envVars) - 1
		v.envIsNew = true
		return v.startEnvEdit()
	}
	if v.envCursor >= 0 && v.envCursor < len(v.envVars) {
		v.envIsNew = false
		return v.startEnvEdit()
	}
	return v, nil
}

func (v *ContainerFormView) startEnvEdit() (View, tea.Cmd) {
	e := v.envVars[v.envCursor]
	v.envInputs[0].SetValue(e.key)
	v.envInputs[1].SetValue(e.value)
	v.editingEnv = true
	v.envInputFocus = 0
	return v, v.envInputs[0].Focus()
}

// removeCurrentItem removes the selected mount or env var.
func (v *ContainerFormView) removeCurrentItem() (View, tea.Cmd) {
	if v.cursor == fieldMounts && v.mountCursor >= 0 && v.mountCursor < len(v.mounts) {
		if v.isRequiredMount(v.mountCursor) {
			return v, nil // agent won't function without its config directory
		}
		v.mounts = append(v.mounts[:v.mountCursor], v.mounts[v.mountCursor+1:]...)
		if v.mountCursor >= len(v.mounts) {
			v.mountCursor = len(v.mounts) - 1
		}
		if len(v.mounts) == 0 {
			v.mountCursor = -1
		}
		return v, nil
	}
	if v.cursor == fieldEnvVars && v.envCursor >= 0 && v.envCursor < len(v.envVars) {
		v.envVars = append(v.envVars[:v.envCursor], v.envVars[v.envCursor+1:]...)
		if v.envCursor >= len(v.envVars) {
			v.envCursor = len(v.envVars) - 1
		}
		if len(v.envVars) == 0 {
			v.envCursor = -1
		}
		return v, nil
	}
	return v, nil
}

func (v *ContainerFormView) openDirectoryBrowser() (View, tea.Cmd) {
	startPath := v.inputs[1].Value()
	if startPath == "" && v.defaults != nil && v.defaults.HomeDir != "" {
		startPath = v.defaults.HomeDir
	}
	if startPath == "" {
		startPath = "/"
	}
	v.dirBrowser = components.NewDirectoryBrowser(startPath, func(path string) tea.Cmd {
		return func() tea.Msg {
			entries, err := v.client.ListDirectories(context.Background(), path, false)
			return components.DirectoryBrowserMsg{Path: path, Entries: entries, Err: err}
		}
	})
	v.browsing = true
	return v, v.dirBrowser.Init()
}

func (v *ContainerFormView) blurActiveField() {
	switch v.cursor {
	case fieldName:
		v.inputs[0].Blur()
	case fieldPath:
		v.inputs[1].Blur()
	case fieldImage:
		v.inputs[2].Blur()
	case fieldBudget:
		v.budgetInput.Blur()
	case fieldDomains:
		v.domains.Blur()
	}
}

func (v *ContainerFormView) updateActiveField(msg tea.Msg) (View, tea.Cmd) {
	var cmd tea.Cmd
	switch {
	case v.editingMount:
		if v.mountInputs[0].Focused() {
			v.mountInputs[0], cmd = v.mountInputs[0].Update(msg)
		} else {
			v.mountInputs[1], cmd = v.mountInputs[1].Update(msg)
		}
	case v.editingEnv:
		if v.envInputs[0].Focused() {
			v.envInputs[0], cmd = v.envInputs[0].Update(msg)
		} else {
			v.envInputs[1], cmd = v.envInputs[1].Update(msg)
		}
	case v.cursor == fieldName:
		v.inputs[0], cmd = v.inputs[0].Update(msg)
	case v.cursor == fieldImage:
		v.inputs[2], cmd = v.inputs[2].Update(msg)
	case v.cursor == fieldBudget:
		v.budgetInput, cmd = v.budgetInput.Update(msg)
	case v.cursor == fieldDomains:
		v.domains, cmd = v.domains.Update(msg)
	}
	return v, cmd
}

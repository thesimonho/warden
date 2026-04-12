package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/internal/tui/components"
)

// handleOrphanKey handles y/n key input during orphan container confirmation.
func (v *ContainerFormView) handleOrphanKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		req := v.pendingOrphan.Request
		req.ForceReplace = true
		v.pendingOrphan = nil
		v.loading = true
		return v, func() tea.Msg {
			_, err := v.client.CreateContainer(context.Background(), "", string(req.AgentType), req)
			return OperationResultMsg{Operation: "create", Err: err}
		}
	case "n", "N", "esc":
		v.pendingOrphan = nil
	}
	return v, nil
}

func (v *ContainerFormView) handleKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	if v.browsing && v.dirBrowser != nil {
		return v.handleBrowsingKey(msg)
	}
	if v.editingPort {
		switch msg.String() {
		case "enter":
			v.savePortInput()
			return v, nil
		case "esc":
			v.cancelPortEdit()
			return v, nil
		}
		return v.updateActiveField(msg)
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
	case msg.String() == "]":
		v.switchStep(1)
	case msg.String() == "[":
		v.switchStep(-1)
	case msg.String() == "enter" || msg.String() == " ":
		return v.activateField()
	case msg.String() == "tab":
		return v.cycleSelection()
	case msg.String() == "x":
		return v.removeCurrentItem()
	case msg.String() == "r":
		if v.step == stepAdvanced && v.fieldCursor == advMounts && v.mountCursor >= 0 && v.mountCursor < len(v.mounts) {
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
		v.inputs[inputPath].SetValue(v.dirBrowser.Path())
		v.browsing = false
		v.dirBrowser = nil
		return v, nil
	}
	var cmd tea.Cmd
	v.dirBrowser, cmd = v.dirBrowser.Update(msg)
	return v, cmd
}

// switchStep moves to a different step, resetting the field cursor.
func (v *ContainerFormView) switchStep(delta int) {
	next := int(v.step) + delta
	if next < 0 || next >= int(stepCount) {
		return
	}
	v.step = formStep(next)
	v.fieldCursor = 0
	v.resetSubCursors()
}

// resetSubCursors resets all sub-cursors to their default positions.
func (v *ContainerFormView) resetSubCursors() {
	v.runtimeCursor = 0
	v.accessCursor = 0
	v.mountCursor = -1
	v.envCursor = -1
}

// cycleSelection cycles the value of selection fields (tab key).
func (v *ContainerFormView) cycleSelection() (View, tea.Cmd) {
	switch v.step {
	case stepGeneral:
		switch v.fieldCursor {
		case genAgentType:
			if v.editID == "" {
				v.agentType = (v.agentType + 1) % len(agentTypes)
				v.refilterDefaultMounts()
				v.domains.SetValue(defaultDomainsForAgent(v.restrictedDomains, agentTypes[v.agentType]))
			}
		case genSkipPerms:
			v.skipPerm = !v.skipPerm
		}
	case stepEnvironment:
		switch v.fieldCursor {
		case envRuntimes:
			if v.runtimeCursor >= 0 && v.runtimeCursor < len(v.runtimeDefaults) {
				v.toggleRuntime(v.runtimeDefaults[v.runtimeCursor].ID)
			}
		case envAccessItems:
			if v.accessCursor >= 0 && v.accessCursor < len(v.accessItems) {
				v.toggleAccessItem(v.accessItems[v.accessCursor].ID)
			}
		}
	case stepNetwork:
		if v.fieldCursor == netNetwork {
			v.network = (v.network + 1) % len(networkModes)
		}
	}
	return v, nil
}

// isFieldVisible returns whether a field should be shown in the current step.
func (v *ContainerFormView) isFieldVisible(field int) bool {
	if v.step == stepGeneral {
		isRemote := v.source == sourceRemote
		switch field {
		case genPath:
			return !isRemote
		case genCloneURL, genTemporary:
			return isRemote
		}
	}
	if v.step == stepNetwork && field == netDomains {
		return networkModes[v.network] == "restricted"
	}
	return true
}

// moveCursor moves the cursor by delta, navigating sub-items within sections.
func (v *ContainerFormView) moveCursor(delta int) {
	if v.step == stepEnvironment && v.fieldCursor == envRuntimes {
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

	if v.step == stepEnvironment && v.fieldCursor == envAccessItems {
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

	if v.step == stepAdvanced && v.fieldCursor == advMounts {
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

	if v.step == stepNetwork && v.fieldCursor == netPorts {
		next := v.portCursor + delta
		if next < -1 {
			v.portCursor = -1
			v.moveCursorField(delta)
			return
		}
		if next >= len(v.forwardedPorts) {
			v.moveCursorField(delta)
			return
		}
		v.portCursor = next
		return
	}

	if v.step == stepAdvanced && v.fieldCursor == advEnvVars {
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

// moveCursorField moves the main field cursor within the current step,
// skipping hidden fields. Initializes sub-cursors when entering sections.
func (v *ContainerFormView) moveCursorField(delta int) {
	count := fieldCountForStep(v.step)
	next := v.fieldCursor + delta
	for next >= 0 && next < count {
		if v.isFieldVisible(next) {
			v.fieldCursor = next
			v.initSubCursorForField(delta)
			return
		}
		next += delta
	}
}

// initSubCursorForField sets the correct sub-cursor position when
// the field cursor enters a section with sub-items.
func (v *ContainerFormView) initSubCursorForField(delta int) {
	switch v.step {
	case stepEnvironment:
		switch v.fieldCursor {
		case envRuntimes:
			if delta > 0 {
				v.runtimeCursor = 0
			} else {
				v.runtimeCursor = max(len(v.runtimeDefaults)-1, 0)
			}
		case envAccessItems:
			if delta > 0 {
				v.accessCursor = 0
			} else {
				v.accessCursor = max(len(v.accessItems)-1, 0)
			}
		}
	case stepNetwork:
		if v.fieldCursor == netPorts {
			if delta > 0 {
				v.portCursor = -1
			} else {
				v.portCursor = max(len(v.forwardedPorts)-1, -1)
			}
		}
	case stepAdvanced:
		switch v.fieldCursor {
		case advMounts:
			if delta > 0 {
				v.mountCursor = -1
			} else {
				v.mountCursor = max(len(v.mounts)-1, -1)
			}
		case advEnvVars:
			if delta > 0 {
				v.envCursor = -1
			} else {
				v.envCursor = max(len(v.envVars)-1, -1)
			}
		}
	}
}

func (v *ContainerFormView) activateField() (View, tea.Cmd) {
	switch v.step {
	case stepGeneral:
		return v.activateGeneralField()
	case stepEnvironment:
		return v.activateEnvironmentField()
	case stepNetwork:
		return v.activateNetworkField()
	case stepAdvanced:
		return v.activateAdvancedField()
	}
	return v, nil
}

func (v *ContainerFormView) activateGeneralField() (View, tea.Cmd) {
	switch v.fieldCursor {
	case genAgentType:
		if v.editID == "" {
			v.agentType = (v.agentType + 1) % len(agentTypes)
			v.refilterDefaultMounts()
		}
	case genName:
		v.editing = true
		return v, v.inputs[inputName].Focus()
	case genSource:
		if v.editID == "" {
			v.source = (v.source + 1) % len(projectSources)
		}
	case genPath:
		return v.openDirectoryBrowser()
	case genCloneURL:
		v.editing = true
		return v, v.inputs[inputCloneURL].Focus()
	case genTemporary:
		v.temporary = !v.temporary
	case genSkipPerms:
		v.skipPerm = !v.skipPerm
	case genBudget:
		v.editing = true
		return v, v.budgetInput.Focus()
	case genSubmit:
		return v, v.submit()
	}
	return v, nil
}

func (v *ContainerFormView) activateEnvironmentField() (View, tea.Cmd) {
	switch v.fieldCursor {
	case envRuntimes:
		if v.runtimeCursor >= 0 && v.runtimeCursor < len(v.runtimeDefaults) {
			v.toggleRuntime(v.runtimeDefaults[v.runtimeCursor].ID)
		}
	case envAccessItems:
		if v.accessCursor >= 0 && v.accessCursor < len(v.accessItems) {
			v.toggleAccessItem(v.accessItems[v.accessCursor].ID)
		}
	case envSubmit:
		return v, v.submit()
	}
	return v, nil
}

func (v *ContainerFormView) activateNetworkField() (View, tea.Cmd) {
	switch v.fieldCursor {
	case netNetwork:
		v.network = (v.network + 1) % len(networkModes)
		// When switching to restricted with no domains, populate defaults
		// from the template or server-provided list for the current agent.
		if networkModes[v.network] == "restricted" && strings.TrimSpace(v.domains.Value()) == "" {
			v.populateDefaultDomains()
		}
	case netDomains:
		v.editing = true
		return v, v.domains.Focus()
	case netPorts:
		if v.portCursor == -1 {
			v.forwardedPorts = append(v.forwardedPorts, 0)
			v.portCursor = len(v.forwardedPorts) - 1
			v.portIsNew = true
			v.portInput.SetValue("")
			v.editingPort = true
			return v, v.portInput.Focus()
		}
		if v.portCursor >= 0 && v.portCursor < len(v.forwardedPorts) {
			v.portIsNew = false
			v.portInput.SetValue(fmt.Sprintf("%d", v.forwardedPorts[v.portCursor]))
			v.editingPort = true
			return v, v.portInput.Focus()
		}
	case netSubmit:
		return v, v.submit()
	}
	return v, nil
}

func (v *ContainerFormView) activateAdvancedField() (View, tea.Cmd) {
	switch v.fieldCursor {
	case advImage:
		v.editing = true
		return v, v.inputs[inputImage].Focus()
	case advMounts:
		return v.activateMountField()
	case advEnvVars:
		return v.activateEnvField()
	case advSubmit:
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
	if v.step == stepAdvanced && v.fieldCursor == advMounts && v.mountCursor >= 0 && v.mountCursor < len(v.mounts) {
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
	if v.step == stepAdvanced && v.fieldCursor == advEnvVars && v.envCursor >= 0 && v.envCursor < len(v.envVars) {
		v.envVars = append(v.envVars[:v.envCursor], v.envVars[v.envCursor+1:]...)
		if v.envCursor >= len(v.envVars) {
			v.envCursor = len(v.envVars) - 1
		}
		if len(v.envVars) == 0 {
			v.envCursor = -1
		}
		return v, nil
	}
	if v.step == stepNetwork && v.fieldCursor == netPorts && v.portCursor >= 0 && v.portCursor < len(v.forwardedPorts) {
		v.forwardedPorts = append(v.forwardedPorts[:v.portCursor], v.forwardedPorts[v.portCursor+1:]...)
		if v.portCursor >= len(v.forwardedPorts) {
			v.portCursor = len(v.forwardedPorts) - 1
		}
		if len(v.forwardedPorts) == 0 {
			v.portCursor = -1
		}
		return v, nil
	}
	return v, nil
}

func (v *ContainerFormView) openDirectoryBrowser() (View, tea.Cmd) {
	startPath := v.inputs[inputPath].Value()
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
	switch v.step {
	case stepGeneral:
		switch v.fieldCursor {
		case genName:
			v.inputs[inputName].Blur()
		case genPath:
			v.inputs[inputPath].Blur()
		case genBudget:
			v.budgetInput.Blur()
		}
	case stepNetwork:
		if v.fieldCursor == netDomains {
			v.domains.Blur()
		}
		if v.fieldCursor == netPorts {
			v.portInput.Blur()
		}
	case stepAdvanced:
		if v.fieldCursor == advImage {
			v.inputs[inputImage].Blur()
		}
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
	case v.editingPort:
		v.portInput, cmd = v.portInput.Update(msg)
	case v.step == stepGeneral && v.fieldCursor == genName:
		v.inputs[inputName], cmd = v.inputs[inputName].Update(msg)
	case v.step == stepGeneral && v.fieldCursor == genCloneURL:
		v.inputs[inputCloneURL], cmd = v.inputs[inputCloneURL].Update(msg)
	case v.step == stepGeneral && v.fieldCursor == genBudget:
		v.budgetInput, cmd = v.budgetInput.Update(msg)
	case v.step == stepNetwork && v.fieldCursor == netDomains:
		v.domains, cmd = v.domains.Update(msg)
	case v.step == stepAdvanced && v.fieldCursor == advImage:
		v.inputs[inputImage], cmd = v.inputs[inputImage].Update(msg)
	}
	return v, cmd
}

package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"

	"github.com/thesimonho/warden/agent"
)

// Form field styles.
var (
	formLabel       = lipgloss.NewStyle().Bold(true).Foreground(colorError)
	formValue       = lipgloss.NewStyle().Padding(0, 0, 0, 2)
	formCursor      = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	formDescription = lipgloss.NewStyle().Foreground(colorGray).Padding(0, 0, 0, 2)
)

// Pre-rendered cursor prefix (avoids re-rendering every frame).
var cursorMarker = formCursor.Render("> ")

// cursorPrefix returns the cursor arrow if active, or two spaces.
func cursorPrefix(isActive bool) string {
	if isActive {
		return cursorMarker
	}
	return "  "
}

// subItemPrefix returns the indented cursor for sub-items.
func subItemPrefix(isSelected bool) string {
	if isSelected {
		return "  " + cursorMarker
	}
	return "    "
}

// textInputView returns the rendered value of a text input.
func textInputView(input textinput.Model, isFocused bool) string {
	if isFocused {
		return input.View()
	}
	if v := input.Value(); v != "" {
		return v
	}
	return Styles.Muted.Render(input.Placeholder)
}

// boolSelector renders a [yes] / [no] toggle display.
func boolSelector(active bool) string {
	if active {
		return formCursor.Render("[yes]") + " " + Styles.Muted.Render(" no ")
	}
	return Styles.Muted.Render(" yes ") + " " + formCursor.Render("[no]")
}

// orEmpty returns the value or a muted "(empty)" placeholder.
func orEmpty(val string) string {
	if val == "" {
		return Styles.Muted.Render("(empty)")
	}
	return val
}

// roLabel returns a styled read-only/read-write indicator.
func roLabel(readOnly bool) string {
	if readOnly {
		return Styles.Muted.Render(" [RO]")
	}
	return Styles.Warning.Render(" [RW]")
}

// isSensitiveKey returns true for keys that likely contain secrets.
func isSensitiveKey(k string) bool {
	upper := strings.ToUpper(k)
	return strings.Contains(upper, "KEY") ||
		strings.Contains(upper, "SECRET") ||
		strings.Contains(upper, "TOKEN") ||
		strings.Contains(upper, "PASSWORD")
}

// Render renders the form within the app's standard layout.
func (v *ContainerFormView) Render(width, height int) string {
	title := "Create Project"
	if v.editID != "" {
		title = "Edit Project"
	}

	// Directory browser takes over the full content area.
	if v.browsing && v.dirBrowser != nil {
		v.dirBrowser.SetHeight(height - 4)
		return v.dirBrowser.View()
	}

	var s strings.Builder
	s.WriteString(Styles.Muted.Render("← ") + Styles.Bold.Render(title) + "\n\n")

	if v.loading {
		s.WriteString("Loading...")
		return s.String()
	}

	if v.err != nil {
		s.WriteString(Styles.Error.Render("Error: "+v.err.Error()) + "\n\n")
	}

	// Step bar.
	s.WriteString(v.renderStepBar())
	s.WriteString("\n\n")

	rawLines, rawCursorLine := v.buildFieldLines()

	// Flatten multi-line entries (e.g. textarea values) so every entry
	// in the slice is exactly one visual row. This ensures the scroll
	// logic counts rows correctly regardless of dynamic content height.
	var lines []string
	cursorLine := 0
	for i, line := range rawLines {
		if i == rawCursorLine {
			cursorLine = len(lines)
		}
		parts := strings.Split(line, "\n")
		lines = append(lines, parts...)
	}

	// Reserve space for title (2 lines), step bar (1 line), blank (1 line).
	maxVisible := height - 5
	if maxVisible < 5 {
		maxVisible = 5
	}
	if len(lines) <= maxVisible {
		for _, line := range lines {
			s.WriteString(line + "\n")
		}
	} else {
		offset := cursorLine - maxVisible/2
		if offset < 0 {
			offset = 0
		}
		if offset+maxVisible > len(lines) {
			offset = len(lines) - maxVisible
		}
		if offset > 0 {
			s.WriteString(Styles.Muted.Render("  ↑ scroll up") + "\n")
			maxVisible--
		}
		hasMore := offset+maxVisible < len(lines)
		if hasMore {
			maxVisible--
		}
		for i := offset; i < offset+maxVisible && i < len(lines); i++ {
			s.WriteString(lines[i] + "\n")
		}
		if hasMore {
			s.WriteString(Styles.Muted.Render("  ↓ scroll down") + "\n")
		}
	}

	return s.String()
}

// renderStepBar renders the horizontal step navigation bar.
func (v *ContainerFormView) renderStepBar() string {
	var parts []string
	for i := formStep(0); i < stepCount; i++ {
		label := stepLabels[i]
		badge := v.stepBadge(i)

		if i == v.step {
			parts = append(parts, formCursor.Render("["+label+badge+"]"))
		} else {
			parts = append(parts, Styles.Muted.Render(" "+label+badge+" "))
		}
	}
	return strings.Join(parts, "  ")
}

// stepBadge returns a badge character for the step tab.
func (v *ContainerFormView) stepBadge(s formStep) string {
	switch s {
	case stepGeneral:
		if v.inputs[inputName].Value() == "" || v.inputs[inputPath].Value() == "" {
			return "*"
		}
		return "✓"
	case stepEnvironment:
		if len(v.runtimeDefaults) > 0 || len(v.accessItems) > 0 {
			return "✓"
		}
	case stepNetwork:
		return "✓"
	case stepAdvanced:
		if v.inputs[inputImage].Value() != defaultContainerImage ||
			len(v.mounts) > 0 || len(v.envVars) > 0 {
			return "✓"
		}
	}
	return ""
}

// stepSummary returns a short description of the step's current state.
func (v *ContainerFormView) stepSummary(s formStep) string {
	switch s {
	case stepGeneral:
		name := v.inputs[inputName].Value()
		if name == "" || v.inputs[inputPath].Value() == "" {
			return "Setup required"
		}
		selected := agentTypes[v.agentType]
		return agentTypeLabels[selected] + ", " + name

	case stepEnvironment:
		if len(v.runtimeDefaults) == 0 {
			return "Detecting..."
		}
		count := 0
		for _, r := range v.runtimeDefaults {
			if v.runtimeToggles[r.ID] {
				count++
			}
		}
		accessCount := 0
		for _, item := range v.accessItems {
			if v.accessToggles[item.ID] {
				accessCount++
			}
		}
		s := fmt.Sprintf("%d runtime", count)
		if count != 1 {
			s += "s"
		}
		if accessCount > 0 {
			s += fmt.Sprintf(", %d access", accessCount)
		}
		return s

	case stepNetwork:
		mode := networkModes[v.network]
		switch mode {
		case "full":
			return "Full access"
		case "restricted":
			return "Restricted"
		default:
			return "No network"
		}

	case stepAdvanced:
		var parts []string
		if v.inputs[inputImage].Value() != defaultContainerImage {
			parts = append(parts, "Custom image")
		}
		if len(v.mounts) > 0 {
			parts = append(parts, "Mounts")
		}
		if len(v.envVars) > 0 {
			parts = append(parts, "Env vars")
		}
		if len(parts) > 0 {
			return strings.Join(parts, ", ")
		}
		return "Defaults applied"
	}
	return ""
}

// buildFieldLines returns the content lines for the current step,
// plus the line index of the current cursor for scroll-to-cursor.
func (v *ContainerFormView) buildFieldLines() ([]string, int) {
	switch v.step {
	case stepGeneral:
		return v.buildGeneralFields()
	case stepEnvironment:
		return v.buildEnvironmentFields()
	case stepNetwork:
		return v.buildNetworkFields()
	case stepAdvanced:
		return v.buildAdvancedFields()
	}
	return nil, 0
}

func (v *ContainerFormView) buildGeneralFields() ([]string, int) {
	var lines []string
	cursorLine := 0

	v.appendField(&lines, &cursorLine, genAgentType, "Agent", v.fieldViewGeneral(genAgentType), "")
	v.appendField(&lines, &cursorLine, genName, "Name", v.fieldViewGeneral(genName), "")
	v.appendField(&lines, &cursorLine, genPath, "Project Path", v.fieldViewGeneral(genPath), "Host directory to mount")

	var skipPermsDesc string
	if agentTypes[v.agentType] == agent.Codex {
		skipPermsDesc = "Auto-approve all Codex actions (--dangerously-bypass-approvals-and-sandbox)"
	} else {
		skipPermsDesc = "Auto-approve all Claude Code actions (--dangerously-skip-permissions)"
	}
	v.appendField(&lines, &cursorLine, genSkipPerms, "Skip Permissions", v.fieldViewGeneral(genSkipPerms), skipPermsDesc)
	v.appendField(&lines, &cursorLine, genBudget, "Project Budget (USD)", v.fieldViewGeneral(genBudget), "Auto-pauses agents when exceeded")

	// Submit button.
	v.appendSubmitButton(&lines, &cursorLine, genSubmit)

	return lines, cursorLine
}

func (v *ContainerFormView) buildEnvironmentFields() ([]string, int) {
	var lines []string
	cursorLine := 0

	// Runtime toggles.
	if len(v.runtimeDefaults) > 0 {
		isActive := v.fieldCursor == envRuntimes
		if isActive && v.runtimeCursor < 0 {
			cursorLine = len(lines)
		}
		lines = append(lines, cursorPrefix(isActive && v.runtimeCursor < 0)+formLabel.Render("Runtimes"))
		lines = append(lines, formDescription.Render("Language runtimes to install in the container"))
		for i, r := range v.runtimeDefaults {
			isSelected := isActive && v.runtimeCursor == i
			if isSelected {
				cursorLine = len(lines)
			}
			prefix := subItemPrefix(isSelected)
			toggle := boolSelector(v.runtimeToggles[r.ID])
			if r.AlwaysEnabled {
				toggle = Styles.Muted.Render("(required)")
			}
			suffix := ""
			if !r.AlwaysEnabled && r.Detected {
				suffix = " " + Styles.Muted.Render("(detected)")
			}
			lines = append(lines, prefix+r.Label+" "+toggle+suffix)
			if r.Description != "" {
				lines = append(lines, formDescription.Render("  "+r.Description))
			}
		}
		lines = append(lines, "")
	}

	// Access item toggles.
	if len(v.accessItems) > 0 {
		isActive := v.fieldCursor == envAccessItems
		if isActive && v.accessCursor < 0 {
			cursorLine = len(lines)
		}
		lines = append(lines, cursorPrefix(isActive && v.accessCursor < 0)+formLabel.Render("Access"))
		lines = append(lines, formDescription.Render("Passthrough access items to containers"))
		for i, item := range v.accessItems {
			isSelected := isActive && v.accessCursor == i
			if isSelected {
				cursorLine = len(lines)
			}
			prefix := subItemPrefix(isSelected)
			toggle := boolSelector(v.accessToggles[item.ID])
			if !item.Detection.Available {
				toggle = Styles.Muted.Render("(unavailable)")
			}
			lines = append(lines, prefix+item.Label+" "+toggle)
			if item.Description != "" {
				lines = append(lines, formDescription.Render("  "+item.Description))
			}
		}
		lines = append(lines, "")
	}

	// Submit button.
	v.appendSubmitButton(&lines, &cursorLine, envSubmit)

	return lines, cursorLine
}

func (v *ContainerFormView) buildNetworkFields() ([]string, int) {
	var lines []string
	cursorLine := 0

	v.appendField(&lines, &cursorLine, netNetwork, "Network", v.fieldViewNetwork(netNetwork), networkDescriptions[networkModes[v.network]])

	if v.isFieldVisible(netDomains) {
		v.appendField(&lines, &cursorLine, netDomains, "Allowed Domains", v.fieldViewNetwork(netDomains), "One per line")
	}

	// Forwarded ports.
	v.appendListSection(&lines, &cursorLine,
		netPorts, "Forwarded Ports", "Container ports exposed via reverse proxy",
		"Add Port", v.portCursor, v.renderPortItems)

	// Submit button.
	v.appendSubmitButton(&lines, &cursorLine, netSubmit)

	return lines, cursorLine
}

func (v *ContainerFormView) buildAdvancedFields() ([]string, int) {
	var lines []string
	cursorLine := 0

	v.appendField(&lines, &cursorLine, advImage, "Image", v.fieldViewAdvanced(advImage), "")

	// Bind mounts.
	v.appendListSection(&lines, &cursorLine,
		advMounts, "Bind Mounts", "Additional host directories",
		"Add Mount", v.mountCursor, v.renderMountItems)

	// Environment variables.
	v.appendListSection(&lines, &cursorLine,
		advEnvVars, "Environment Variables", "",
		"Add Variable", v.envCursor, v.renderEnvItems)

	// Submit button.
	v.appendSubmitButton(&lines, &cursorLine, advSubmit)

	return lines, cursorLine
}

// appendField renders a standard label + value + description field.
func (v *ContainerFormView) appendField(lines *[]string, cursorLine *int, id int, label, value, desc string) {
	isActive := id == v.fieldCursor
	if isActive {
		*cursorLine = len(*lines)
	}
	*lines = append(*lines, cursorPrefix(isActive)+formLabel.Render(label+":"))
	*lines = append(*lines, formValue.Render(value))
	if desc != "" {
		*lines = append(*lines, formDescription.Render(desc))
	}
	*lines = append(*lines, "")
}

// appendSubmitButton appends the Create/Save button to the lines.
func (v *ContainerFormView) appendSubmitButton(lines *[]string, cursorLine *int, fieldID int) {
	*lines = append(*lines, "")
	isActive := fieldID == v.fieldCursor
	if isActive {
		*cursorLine = len(*lines)
	}
	submitLabel := "Save"
	if v.editID == "" {
		submitLabel = "Create"
	}
	if isActive {
		*lines = append(*lines, cursorPrefix(true)+formCursor.Render("["+submitLabel+"]"))
	} else {
		*lines = append(*lines, cursorPrefix(false)+Styles.Muted.Render("["+submitLabel+"]"))
	}
}

// appendListSection appends a section header and items to lines.
// Shared between mounts and env vars to avoid duplication.
func (v *ContainerFormView) appendListSection(
	lines *[]string, cursorLine *int,
	fieldID int,
	label, desc, addLabel string,
	subCursor int,
	renderItems func(isActive bool) []string,
) {
	isActive := v.fieldCursor == fieldID
	isOnHeader := isActive && subCursor == -1

	if isOnHeader {
		*cursorLine = len(*lines)
	}

	headerLine := cursorPrefix(isOnHeader) + formLabel.Render(label+":")
	if isOnHeader {
		headerLine += formCursor.Render(" [" + addLabel + "]")
	} else {
		headerLine += Styles.Muted.Render(" (enter: add)")
	}
	*lines = append(*lines, headerLine)

	if desc != "" {
		*lines = append(*lines, formDescription.Render(desc))
	}

	items := renderItems(isActive)
	if len(items) == 0 {
		*lines = append(*lines, formValue.Render(Styles.Muted.Render("None configured.")))
	} else {
		if isActive && subCursor >= 0 {
			*cursorLine = len(*lines) + subCursor
		}
		*lines = append(*lines, items...)
	}
	*lines = append(*lines, "")
}

// renderMountItems renders mount sub-items.
func (v *ContainerFormView) renderMountItems(isActive bool) []string {
	rcp := v.requiredContainerPath()
	var lines []string
	for i, m := range v.mounts {
		isSelected := isActive && v.mountCursor == i
		prefix := subItemPrefix(isSelected)
		reqLabel := ""
		if rcp != "" && m.ContainerPath == rcp {
			reqLabel = " (required)"
		}
		if v.editingMount && isSelected {
			lines = append(lines, prefix+v.mountInputs[0].View()+" → "+v.mountInputs[1].View()+roLabel(m.ReadOnly)+reqLabel)
		} else {
			lines = append(lines, prefix+orEmpty(m.HostPath)+" → "+orEmpty(m.ContainerPath)+roLabel(m.ReadOnly)+reqLabel)
		}
	}
	return lines
}

// renderEnvItems renders env var sub-items.
func (v *ContainerFormView) renderEnvItems(isActive bool) []string {
	var lines []string
	for i, e := range v.envVars {
		isSelected := isActive && v.envCursor == i
		prefix := subItemPrefix(isSelected)
		if v.editingEnv && isSelected {
			lines = append(lines, prefix+v.envInputs[0].View()+" = "+v.envInputs[1].View())
		} else {
			val := orEmpty(e.value)
			if isSensitiveKey(e.key) && e.value != "" {
				val = "********"
			}
			lines = append(lines, prefix+orEmpty(e.key)+" = "+val)
		}
	}
	return lines
}

// renderPortItems renders forwarded port sub-items.
func (v *ContainerFormView) renderPortItems(isActive bool) []string {
	var lines []string
	for i, port := range v.forwardedPorts {
		isSelected := isActive && v.portCursor == i
		prefix := subItemPrefix(isSelected)
		if v.editingPort && isSelected {
			lines = append(lines, prefix+"Port: "+v.portInput.View())
		} else {
			lines = append(lines, prefix+fmt.Sprintf(":%d", port))
		}
	}
	return lines
}

// fieldViewGeneral renders a General step field value.
func (v *ContainerFormView) fieldViewGeneral(field int) string {
	switch field {
	case genAgentType:
		selected := agentTypes[v.agentType]
		var parts []string
		for _, at := range agentTypes {
			label := agentTypeLabels[at]
			if at == selected {
				parts = append(parts, formCursor.Render("["+label+"]"))
			} else {
				parts = append(parts, Styles.Muted.Render(" "+label+" "))
			}
		}
		return strings.Join(parts, " ")

	case genName:
		return textInputView(v.inputs[inputName], v.editing && v.fieldCursor == genName)
	case genPath:
		return textInputView(v.inputs[inputPath], false)
	case genSkipPerms:
		return boolSelector(v.skipPerm)
	case genBudget:
		if v.editing && v.fieldCursor == genBudget {
			return "$ " + v.budgetInput.View()
		}
		if val := v.budgetInput.Value(); val != "" {
			return "$ " + val
		}
		return Styles.Muted.Render("unlimited")
	}
	return ""
}

// fieldViewNetwork renders a Network step field value.
func (v *ContainerFormView) fieldViewNetwork(field int) string {
	switch field {
	case netNetwork:
		mode := networkModes[v.network]
		var parts []string
		for _, m := range networkModes {
			if m == mode {
				parts = append(parts, formCursor.Render("["+m+"]"))
			} else {
				parts = append(parts, Styles.Muted.Render(" "+m+" "))
			}
		}
		return strings.Join(parts, " ")

	case netDomains:
		if v.editing && v.fieldCursor == netDomains {
			return v.domains.View()
		}
		val := v.domains.Value()
		if val == "" {
			return Styles.Muted.Render("(none)")
		}
		domainLines := strings.Split(val, "\n")
		if len(domainLines) > 10 {
			return strings.Join(domainLines[:10], "\n") + "\n" + Styles.Muted.Render(fmt.Sprintf("... +%d more", len(domainLines)-10))
		}
		return val
	}
	return ""
}

// fieldViewAdvanced renders an Advanced step field value.
func (v *ContainerFormView) fieldViewAdvanced(field int) string {
	if field == advImage {
		return textInputView(v.inputs[inputImage], v.editing && v.fieldCursor == advImage)
	}
	return ""
}

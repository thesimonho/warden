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

	maxVisible := height - 2
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

// buildFieldLines returns all form content as lines for scrolling,
// plus the line index of the current cursor for scroll-to-cursor.
func (v *ContainerFormView) buildFieldLines() ([]string, int) {
	var lines []string
	cursorLine := 0

	// markCursor records the cursor position when the field is active.
	markCursor := func(fieldID int) {
		if fieldID == v.cursor {
			cursorLine = len(lines)
		}
	}

	// appendField renders a standard label + value + description field.
	appendField := func(id int, label, value, desc string) {
		markCursor(id)
		lines = append(lines, cursorPrefix(id == v.cursor)+formLabel.Render(label+":"))
		lines = append(lines, formValue.Render(value))
		if desc != "" {
			lines = append(lines, formDescription.Render(desc))
		}
		lines = append(lines, "")
	}

	// Core fields.
	appendField(fieldAgentType, "Agent", v.fieldView(fieldAgentType), "")
	appendField(fieldName, "Name", v.fieldView(fieldName), "")
	appendField(fieldPath, "Project Path", v.fieldView(fieldPath), "Host directory to mount")

	var skipPermsDesc string
	if agentTypes[v.agentType] == agent.Codex {
		skipPermsDesc = "Auto-approve all Codex actions (--dangerously-bypass-approvals-and-sandbox)"
	} else {
		skipPermsDesc = "Auto-approve all Claude Code actions (--dangerously-skip-permissions)"
	}
	appendField(fieldSkipPerms, "Skip Permissions", v.fieldView(fieldSkipPerms), skipPermsDesc)
	appendField(fieldBudget, "Project Budget (USD)", v.fieldView(fieldBudget), "Auto-pauses agents when exceeded")
	appendField(fieldNetwork, "Network", v.fieldView(fieldNetwork), networkDescriptions[networkModes[v.network]])

	if v.isFieldVisible(fieldDomains) {
		appendField(fieldDomains, "Allowed Domains", v.fieldView(fieldDomains), "One per line")
	}

	// Runtime toggles.
	if len(v.runtimeDefaults) > 0 {
		isActive := v.cursor == fieldRuntimes
		markCursor(fieldRuntimes)
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

	// Advanced toggle.
	markCursor(fieldAdvanced)
	arrow := "▶"
	if v.advancedOpen {
		arrow = "▼"
	}
	advLabel := arrow + " Advanced"
	if v.cursor == fieldAdvanced {
		lines = append(lines, cursorPrefix(true)+formCursor.Render(advLabel))
	} else {
		lines = append(lines, cursorPrefix(false)+Styles.Muted.Render(advLabel))
	}
	lines = append(lines, "")

	if v.advancedOpen {
		appendField(fieldImage, "Image", v.fieldView(fieldImage), "")

		// Access item toggles.
		if len(v.accessItems) > 0 {
			isActive := v.cursor == fieldAccessItems
			markCursor(fieldAccessItems)
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

		v.appendListSection(&lines, &cursorLine,
			fieldMounts, "Bind Mounts", "Additional host directories",
			"Add Mount", v.mountCursor, v.renderMountItems)

		v.appendListSection(&lines, &cursorLine,
			fieldEnvVars, "Environment Variables", "",
			"Add Variable", v.envCursor, v.renderEnvItems)
	}

	// Submit button.
	lines = append(lines, "")
	markCursor(fieldSubmit)
	submitLabel := "Save"
	if v.editID == "" {
		submitLabel = "Create"
	}
	if v.cursor == fieldSubmit {
		lines = append(lines, cursorPrefix(true)+formCursor.Render("["+submitLabel+"]"))
	} else {
		lines = append(lines, cursorPrefix(false)+Styles.Muted.Render("["+submitLabel+"]"))
	}

	return lines, cursorLine
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
	isActive := v.cursor == fieldID
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
		// Track cursor line for sub-items.
		if isActive && subCursor >= 0 {
			headerSize := 1
			if desc != "" {
				headerSize = 2
			}
			*cursorLine = len(*lines) + subCursor - headerSize + headerSize
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

func (v *ContainerFormView) fieldView(field int) string {
	switch field {
	case fieldAgentType:
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

	case fieldName:
		return textInputView(v.inputs[0], v.editing && v.cursor == fieldName)
	case fieldPath:
		return textInputView(v.inputs[1], false)
	case fieldImage:
		return textInputView(v.inputs[2], v.editing && v.cursor == fieldImage)

	case fieldBudget:
		if v.editing && v.cursor == fieldBudget {
			return "$ " + v.budgetInput.View()
		}
		if val := v.budgetInput.Value(); val != "" {
			return "$ " + val
		}
		return Styles.Muted.Render("unlimited")

	case fieldNetwork:
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

	case fieldDomains:
		if v.editing && v.cursor == fieldDomains {
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

	case fieldSkipPerms:
		return boolSelector(v.skipPerm)

	}
	return ""
}

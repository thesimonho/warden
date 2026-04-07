package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

// FormKeyMap defines key bindings for the form view.
type FormKeyMap struct {
	Activate key.Binding
	Back     key.Binding
	Remove   key.Binding
	PrevStep key.Binding
	NextStep key.Binding
}

// DefaultFormKeyMap returns the default form key bindings.
func DefaultFormKeyMap() FormKeyMap {
	return FormKeyMap{
		Activate: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "edit/toggle"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Remove: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "remove"),
		),
		PrevStep: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "prev step"),
		),
		NextStep: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "next step"),
		),
	}
}

// ShortHelp returns bindings for the short help bar.
func (k FormKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Activate, k.PrevStep, k.NextStep, k.Back, moreHelp}
}

// FullHelp returns bindings for the expanded help.
func (k FormKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Activate, k.Back},
		{k.PrevStep, k.NextStep},
	}
}

// HelpKeyMap returns the form view's key bindings, which vary by mode.
func (v *ContainerFormView) HelpKeyMap() help.KeyMap {
	if v.browsing {
		return browsingHelpKeyMap
	}
	if v.editing || v.editingPort {
		return editingHelpKeyMap
	}
	if v.editingMount || v.editingEnv {
		return inlineEditHelpKeyMap
	}
	if v.step == stepAdvanced && v.fieldCursor == advMounts && v.mountCursor >= 0 {
		return formWithRemoveKeyMap{keys: v.keys, isMounts: true}
	}
	if v.step == stepAdvanced && v.fieldCursor == advEnvVars && v.envCursor >= 0 {
		return formWithRemoveKeyMap{keys: v.keys, isMounts: false}
	}
	if v.step == stepNetwork && v.fieldCursor == netPorts && v.portCursor >= 0 {
		return formWithRemoveKeyMap{keys: v.keys, isMounts: false}
	}
	if v.step == stepGeneral && v.fieldCursor == genSkipPerms {
		return formSelectionKeyMap{keys: v.keys}
	}
	if v.step == stepNetwork && v.fieldCursor == netNetwork {
		return formSelectionKeyMap{keys: v.keys}
	}
	return v.keys
}

// --- Static help keymaps (hoisted to avoid per-render allocations) ---

// Pre-built key bindings for help bar display.
var (
	bindingSpace = key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "select"))
	bindingOpen  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open"))
	bindingEsc   = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel"))
	bindingDone  = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "done editing"))
	bindingTab   = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	bindingSave  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save"))
	bindingCycle = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab/enter", "cycle"))
	bindingRO    = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "read-only/read-write"))
)

// browsingHelpKeyMap is shown when the directory browser is active.
var browsingHelpKeyMap = staticKeyMap{
	bindings: []key.Binding{bindingSpace, bindingOpen, bindingEsc},
}

// editingHelpKeyMap is shown when a text field is active.
var editingHelpKeyMap = staticKeyMap{
	bindings: []key.Binding{bindingDone},
}

// inlineEditHelpKeyMap is shown when editing mount or env var inline fields.
var inlineEditHelpKeyMap = staticKeyMap{
	bindings: []key.Binding{bindingTab, bindingSave, bindingEsc},
}

// credEditHelpKeyMap is shown when editing a credential inline.
// Includes cycle hint for source/injection type fields.
var credEditHelpKeyMap = staticKeyMap{
	bindings: []key.Binding{bindingTab, bindingCycle, bindingSave, bindingEsc},
}

// staticKeyMap is a help.KeyMap backed by a fixed slice of bindings.
type staticKeyMap struct {
	bindings []key.Binding
}

func (k staticKeyMap) ShortHelp() []key.Binding  { return k.bindings }
func (k staticKeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.bindings} }

// formSelectionKeyMap shows tab/cycle hint for selection fields.
type formSelectionKeyMap struct {
	keys FormKeyMap
}

func (k formSelectionKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{bindingCycle, k.keys.PrevStep, k.keys.NextStep, k.keys.Back, moreHelp}
}

func (k formSelectionKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{bindingCycle, k.keys.Back},
		{k.keys.PrevStep, k.keys.NextStep},
	}
}

// formWithRemoveKeyMap shows remove alongside edit/back.
type formWithRemoveKeyMap struct {
	keys     FormKeyMap
	isMounts bool
}

func (k formWithRemoveKeyMap) ShortHelp() []key.Binding {
	bindings := []key.Binding{k.keys.Activate, k.keys.Remove}
	if k.isMounts {
		bindings = append(bindings, bindingRO)
	}
	return append(bindings, k.keys.PrevStep, k.keys.NextStep, k.keys.Back, moreHelp)
}

func (k formWithRemoveKeyMap) FullHelp() [][]key.Binding {
	col1 := []key.Binding{k.keys.Activate, k.keys.Remove}
	if k.isMounts {
		col1 = append(col1, bindingRO)
	}
	return [][]key.Binding{
		col1,
		{k.keys.PrevStep, k.keys.NextStep},
		{k.keys.Back},
	}
}

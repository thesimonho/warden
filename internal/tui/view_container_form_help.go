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
	}
}

// ShortHelp returns bindings for the short help bar.
func (k FormKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Activate, k.Back, moreHelp}
}

// FullHelp returns bindings for the expanded help.
func (k FormKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Activate, k.Back},
	}
}

// HelpKeyMap returns the form view's key bindings, which vary by mode.
func (v *ContainerFormView) HelpKeyMap() help.KeyMap {
	if v.browsing {
		return browsingHelpKeyMap
	}
	if v.editing {
		return editingHelpKeyMap
	}
	if v.editingMount || v.editingEnv {
		return inlineEditHelpKeyMap
	}
	if v.cursor == fieldMounts && v.mountCursor >= 0 {
		return formWithRemoveKeyMap{keys: v.keys, isMounts: true}
	}
	if v.cursor == fieldEnvVars && v.envCursor >= 0 {
		return formWithRemoveKeyMap{keys: v.keys, isMounts: false}
	}
	if v.cursor == fieldNetwork || v.cursor == fieldSkipPerms {
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
	return []key.Binding{bindingCycle, k.keys.Back, moreHelp}
}

func (k formSelectionKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{bindingCycle, k.keys.Back}}
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
	return append(bindings, k.keys.Back, moreHelp)
}

func (k formWithRemoveKeyMap) FullHelp() [][]key.Binding {
	bindings := []key.Binding{k.keys.Activate, k.keys.Remove}
	if k.isMounts {
		bindings = append(bindings, bindingRO)
	}
	return [][]key.Binding{append(bindings, k.keys.Back)}
}

package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

// Compile-time checks: all keymaps must satisfy help.KeyMap.
var (
	_ help.KeyMap = GlobalKeyMap{}
	_ help.KeyMap = ProjectKeyMap{}
	_ help.KeyMap = WorktreeKeyMap{}
	_ help.KeyMap = SettingsKeyMap{}
	_ help.KeyMap = AuditLogKeyMap{}
	_ help.KeyMap = AccessKeyMap{}
)

// moreHelp is appended to every ShortHelp to show the toggle hint.
var moreHelp = key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "more"))

// navBinding is a display-only binding shown in full help for j/k navigation.
// Actual navigation is handled by the bubbles table/list components.
var navBinding = key.NewBinding(
	key.WithKeys("j", "k"),
	key.WithHelp("j/k", "navigate"),
)

// GlobalKeyMap defines key bindings available across all views.
type GlobalKeyMap struct {
	Quit key.Binding
	Help key.Binding
	Tab1 key.Binding
	Tab2 key.Binding
	Tab3 key.Binding
	Tab4 key.Binding
}

// DefaultGlobalKeyMap returns the default global key bindings.
func DefaultGlobalKeyMap() GlobalKeyMap {
	return GlobalKeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Tab1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "projects"),
		),
		Tab2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "settings"),
		),
		Tab3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "access"),
		),
		Tab4: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "audit log"),
		),
	}
}

// ShortHelp returns bindings shown in the compact help bar.
func (k GlobalKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, moreHelp}
}

// FullHelp returns bindings shown in expanded help.
func (k GlobalKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{navBinding, k.Help, k.Quit},
	}
}

// --- Standardized action keys ---
// n = new, x = remove, X = kill
// j/k = navigate (handled by bubbles components)

// ProjectKeyMap defines key bindings for the project list view.
type ProjectKeyMap struct {
	Open    key.Binding
	Edit    key.Binding
	Toggle  key.Binding // start/stop toggle based on container state
	Remove  key.Binding
	New     key.Binding
	Refresh key.Binding
}

// DefaultProjectKeyMap returns the default project list key bindings.
func DefaultProjectKeyMap() ProjectKeyMap {
	return ProjectKeyMap{
		Open: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "start/stop"),
		),
		Remove: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "manage"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "refresh"),
		),
	}
}

// ShortHelp returns bindings shown in the compact help bar.
func (k ProjectKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.New, k.Toggle, moreHelp}
}

// FullHelp returns bindings shown in expanded help.
func (k ProjectKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Open, k.New, k.Toggle, k.Edit},
		{k.Remove, k.Refresh},
	}
}

// WorktreeKeyMap defines key bindings for the project detail view.
type WorktreeKeyMap struct {
	Connect    key.Binding
	Disconnect key.Binding
	Kill       key.Binding
	Remove     key.Binding
	New        key.Binding
	Cleanup    key.Binding
	Back       key.Binding
}

// DefaultWorktreeKeyMap returns the default worktree key bindings.
func DefaultWorktreeKeyMap() WorktreeKeyMap {
	return WorktreeKeyMap{
		Connect: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "connect"),
		),
		Disconnect: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "disconnect"),
		),
		Kill: key.NewBinding(
			key.WithKeys("X"),
			key.WithHelp("X", "kill"),
		),
		Remove: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "remove"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new worktree"),
		),
		Cleanup: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "cleanup"),
		),
		Back: key.NewBinding(
			key.WithKeys("backspace", "esc"),
			key.WithHelp("esc", "back"),
		),
	}
}

// ShortHelp returns bindings shown in the compact help bar.
func (k WorktreeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Connect, k.New, k.Back, moreHelp}
}

// FullHelp returns bindings shown in expanded help.
func (k WorktreeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Connect, k.Disconnect, k.Kill, k.Remove},
		{k.New, k.Cleanup, k.Back},
	}
}

// SettingsKeyMap defines key bindings for the settings view.
type SettingsKeyMap struct {
	Toggle key.Binding
}

// DefaultSettingsKeyMap returns the default settings key bindings.
func DefaultSettingsKeyMap() SettingsKeyMap {
	return SettingsKeyMap{
		Toggle: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter", "toggle"),
		),
	}
}

// ShortHelp returns bindings shown in the compact help bar.
func (k SettingsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Toggle, moreHelp}
}

// FullHelp returns bindings shown in expanded help.
func (k SettingsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Toggle}}
}

// AuditLogKeyMap defines key bindings for the audit log view.
type AuditLogKeyMap struct {
	CategoryFilter key.Binding
	LevelFilter    key.Binding
	SourceFilter   key.Binding
	ProjectFilter  key.Binding
	TimeRange      key.Binding
	AutoRefresh    key.Binding
	Refresh        key.Binding
	Clear          key.Binding
}

// DefaultAuditLogKeyMap returns the default audit log key bindings.
func DefaultAuditLogKeyMap() AuditLogKeyMap {
	return AuditLogKeyMap{
		CategoryFilter: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "category"),
		),
		LevelFilter: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "level"),
		),
		SourceFilter: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "source"),
		),
		ProjectFilter: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "project"),
		),
		TimeRange: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "time range"),
		),
		AutoRefresh: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "auto-refresh"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "refresh"),
		),
		Clear: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "clear filtered"),
		),
	}
}

// ShortHelp returns bindings shown in the compact help bar.
func (k AuditLogKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.CategoryFilter, k.ProjectFilter, k.TimeRange, moreHelp}
}

// FullHelp returns bindings shown in expanded help.
func (k AuditLogKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.CategoryFilter, k.LevelFilter, k.SourceFilter, k.ProjectFilter},
		{k.TimeRange, k.AutoRefresh, k.Refresh, k.Clear},
	}
}

// AccessKeyMap defines key bindings for the access management view.
type AccessKeyMap struct {
	Edit    key.Binding
	New     key.Binding
	Delete  key.Binding
	Reset   key.Binding
	Test    key.Binding
	Refresh key.Binding
}

// DefaultAccessKeyMap returns the default access view key bindings.
func DefaultAccessKeyMap() AccessKeyMap {
	return AccessKeyMap{
		Edit: key.NewBinding(
			key.WithKeys("e", "enter"),
			key.WithHelp("e/enter", "edit"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Reset: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "reset"),
		),
		Test: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "test"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "refresh"),
		),
	}
}

// ShortHelp returns bindings shown in the compact help bar.
func (k AccessKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Edit, k.New, k.Delete, moreHelp}
}

// FullHelp returns bindings shown in expanded help.
func (k AccessKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Edit, k.New, k.Delete},
		{k.Reset, k.Test, k.Refresh},
	}
}

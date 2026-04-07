package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"context"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/internal/tui/components"
	"github.com/thesimonho/warden/runtime"
	"github.com/thesimonho/warden/version"
)

// View is the interface that all TUI views implement.
// Each view is a tea.Model that can also report its help bindings.
// inputCapture is an optional interface for views that capture text input.
// When active, the app skips tab-switching keys so they reach the input.
type inputCapture interface {
	IsCapturingInput() bool
}

type View interface {
	// Init returns the initial command for the view.
	Init() tea.Cmd
	// Update handles messages and returns commands.
	Update(msg tea.Msg) (View, tea.Cmd)
	// Render returns the view's content as a string.
	// Named Render (not View) to avoid conflict with tea.Model.View().
	Render(width, height int) string
	// HelpKeyMap returns the view's key bindings for the help bar.
	HelpKeyMap() help.KeyMap
}

// App is the root tea.Model that manages tab navigation, SSE events,
// and delegates to the active view.
type App struct {
	client          Client
	activeTab       Tab
	activeView      View
	detailView      View // non-nil when inside project detail
	tabs            []Tab
	tabLabels       []string
	keys            GlobalKeyMap
	help            help.Model
	width           int
	height          int
	eventCh         <-chan eventbus.SSEEvent
	unsubscribe     func()
	err             error
	auditLogMode    api.AuditLogMode
	disconnectKey   string // e.g. "ctrl+\\"
	dockerAvailable bool
}

// NewApp creates the root TUI model backed by the given Client.
//
// SSE subscription starts eagerly here (not in Init) because the Bubble
// Tea v2 Model interface requires Init() to return only a Cmd, not the
// modified model. The SSE channel lives for the app's entire lifetime
// and is shared across all views — each view receives events via the
// SSEEventMsg message type.
//
// If SSE subscription fails (e.g. server not running), the TUI still
// works but without real-time updates. The error is shown in the UI.
func NewApp(client Client) App {
	h := help.New()
	h.Styles = helpStyles()

	// Check initial settings state.
	auditLogMode := api.AuditLogOff
	disconnectKey := engine.DefaultDisconnectKey
	if settings, err := client.GetSettings(context.Background()); err == nil {
		auditLogMode = settings.AuditLogMode
		disconnectKey = settings.DisconnectKey
	}

	// Check Docker availability.
	dockerAvailable := false
	if runtimes, err := client.ListRuntimes(context.Background()); err == nil {
		for _, rt := range runtimes {
			if rt.Name == runtime.RuntimeDocker && rt.Available {
				dockerAvailable = true
				break
			}
		}
	}

	app := App{
		client:          client,
		activeTab:       TabProjects,
		activeView:      NewProjectsView(client),
		keys:            DefaultGlobalKeyMap(),
		help:            h,
		auditLogMode:    auditLogMode,
		disconnectKey:   disconnectKey,
		dockerAvailable: dockerAvailable,
	}
	app.rebuildTabs()

	// Subscribe to SSE events eagerly so the channel is ready for Init().
	ch, unsub, err := client.SubscribeEvents(context.Background())
	if err != nil {
		app.err = err
	} else {
		app.eventCh = ch
		app.unsubscribe = unsub
	}

	return app
}

// Init starts the SSE listener and initializes the default view.
func (a App) Init() tea.Cmd {
	cmds := []tea.Cmd{a.activeView.Init()}
	if a.eventCh != nil {
		cmds = append(cmds, waitForEvent(a.eventCh))
	}
	return tea.Batch(cmds...)
}

// Update handles global keys and delegates to the active view.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Fall through to delegate to active view (tables need width).

	case tea.KeyPressMsg:
		if a.help.ShowAll {
			a.help.ShowAll = false
			return a, nil
		}

		// In detail view, intercept global keys before delegating.
		// Skip tab switching when the view is capturing text input
		// (e.g. form fields, search filters) so number keys reach the input.
		if a.detailView != nil {
			inputActive := false
			if ic, ok := a.detailView.(inputCapture); ok {
				inputActive = ic.IsCapturingInput()
			}

			switch {
			case key.Matches(msg, a.keys.Quit):
				if a.unsubscribe != nil {
					a.unsubscribe()
				}
				return a, tea.Quit
			case key.Matches(msg, a.keys.Help):
				a.help.ShowAll = !a.help.ShowAll
				return a, nil
			case !inputActive && key.Matches(msg, a.keys.Tab1):
				a.detailView = nil
				return a.switchTab(TabProjects)
			case !inputActive && key.Matches(msg, a.keys.Tab2):
				a.detailView = nil
				return a.switchTab(TabSettings)
			case !inputActive && key.Matches(msg, a.keys.Tab3):
				a.detailView = nil
				return a.switchTab(TabAccess)
			case !inputActive && key.Matches(msg, a.keys.Tab4):
				if a.auditLogMode != api.AuditLogOff {
					a.detailView = nil
					return a.switchTab(TabAudit)
				}
			}
			return a.updateDetailView(msg)
		}

		switch {
		case key.Matches(msg, a.keys.Quit):
			if a.unsubscribe != nil {
				a.unsubscribe()
			}
			return a, tea.Quit

		case key.Matches(msg, a.keys.Help):
			a.help.ShowAll = !a.help.ShowAll
			return a, nil

		case key.Matches(msg, a.keys.Tab1):
			return a.switchTab(TabProjects)
		case key.Matches(msg, a.keys.Tab2):
			return a.switchTab(TabSettings)
		case key.Matches(msg, a.keys.Tab3):
			return a.switchTab(TabAccess)
		case key.Matches(msg, a.keys.Tab4):
			if a.auditLogMode != api.AuditLogOff {
				return a.switchTab(TabAudit)
			}
		}

	case OperationResultMsg:
		// Refresh tabs when audit log mode changes.
		if msg.Operation == "change_auditlogmode" && msg.Err == nil {
			if settings, err := a.client.GetSettings(context.Background()); err == nil {
				a.auditLogMode = settings.AuditLogMode
			}
			a.rebuildTabs()
			if a.activeTab == TabAudit && a.auditLogMode == api.AuditLogOff {
				return a.switchTab(TabSettings)
			}
		}
		// Update detach key when it changes.
		if msg.Operation == "change_disconnectkey" && msg.Err == nil {
			if settings, err := a.client.GetSettings(context.Background()); err == nil {
				a.disconnectKey = settings.DisconnectKey
			}
		}

	case NavigateMsg:
		return a.handleNavigate(msg)

	case NavigateBackMsg:
		if a.detailView != nil {
			a.detailView = nil
			cmd := a.activeView.Init()
			return a, cmd
		}
		// Also handle returning from a form view back to the tab's list.
		a.activeView = a.viewForTab(a.activeTab)
		a.activeView, _ = a.activeView.Update(tea.WindowSizeMsg{
			Width: a.width, Height: a.height,
		})
		cmd := a.activeView.Init()
		return a, cmd

	case SSEEventMsg:
		var cmd tea.Cmd
		if a.detailView != nil {
			a.detailView, cmd = a.detailView.Update(msg)
		} else if a.activeView != nil {
			a.activeView, cmd = a.activeView.Update(msg)
		}
		return a, tea.Batch(cmd, waitForEvent(a.eventCh))

	case EventStreamClosedMsg:
		return a, nil

	case execTerminalMsg:
		cmd := tea.Exec(&TerminalExecCmd{
			conn:          msg.conn,
			disconnectKey: engine.DisconnectKeyToByte(a.disconnectKey),
		}, func(err error) tea.Msg {
			return TerminalExitedMsg{Err: err}
		})
		return a, cmd

	case TerminalExitedMsg:
		if msg.Err != nil {
			a.err = msg.Err
		}
		if a.detailView != nil {
			return a.updateDetailView(msg)
		}
		return a, nil
	}

	// Delegate to active view.
	if a.detailView != nil {
		return a.updateDetailView(msg)
	}
	if a.activeView != nil {
		var cmd tea.Cmd
		a.activeView, cmd = a.activeView.Update(msg)
		return a, cmd
	}
	return a, nil
}

// View renders the TUI.
//
// Layout (top to bottom):
//
//	┌─ app padding (1 row) ──────────────────────┐
//	│  tab bar                                    │
//	│  ──────────── separator                     │
//	│  content (fills remaining vertical space)   │
//	│  help bar                                   │
//	└─ app padding (1 row) ──────────────────────┘
func (a App) View() tea.View {
	activeView := a.detailView
	if activeView == nil {
		activeView = a.activeView
	}

	cw := a.contentWidth()

	// Header.
	header := components.RenderTabBar(a.tabLabels, int(a.activeTab), max(cw, 20), version.Version)

	// Help bar — merges view bindings with global bindings.
	a.help.SetWidth(cw)
	var helpBar string
	if activeView != nil {
		helpBar = a.help.View(mergedKeyMap{
			view:   activeView.HelpKeyMap(),
			global: a.keys,
		})
	}

	// Docker warning banner (shown below header when Docker is unavailable).
	var dockerWarning string
	if !a.dockerAvailable {
		dockerWarning = Styles.Warning.Render(
			"Docker is not running — container operations are disabled. " +
				"Install Docker or start the daemon.",
		)
	}

	// Content area — fills remaining vertical space.
	// Vertical budget: header (3) + help bar (1) + app padding (2) = 6.
	// Add 1 for docker warning line when present.
	// The "\n" joiners in body don't add extra rows — they transition
	// from the last line of one section to the first of the next.
	contentH := a.height - 6
	if dockerWarning != "" {
		contentH--
	}
	if contentH < 5 {
		contentH = 5
	}
	var content string
	if activeView != nil {
		content = activeView.Render(cw, contentH)
	}
	if a.err != nil {
		content += "\n" + Styles.Error.Render("Error: "+a.err.Error())
	}
	// Pad content to fill the middle so the help bar sits at the bottom.
	// MaxHeight clips overflow so views can't push the help bar down.
	contentBox := lipgloss.NewStyle().
		Width(cw).
		Height(contentH).
		MaxHeight(contentH).
		Render(content)

	body := header
	if dockerWarning != "" {
		body += "\n" + dockerWarning
	}
	body += "\n" + contentBox + "\n" + helpBar

	appStyle := Styles.App.Width(a.width)
	view := tea.NewView(appStyle.Render(body))
	view.AltScreen = true
	return view
}

// --- Private helpers ---

func (a App) updateDetailView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.detailView, cmd = a.detailView.Update(msg)
	return a, cmd
}

func (a App) handleNavigate(msg NavigateMsg) (tea.Model, tea.Cmd) {
	if msg.ProjectID != "" {
		a.detailView = NewProjectDetailView(a.client, msg.ProjectID, msg.AgentType, msg.ProjectName, a.disconnectKey, msg.ForwardedPorts)
		a.detailView, _ = a.detailView.Update(tea.WindowSizeMsg{
			Width: a.width, Height: a.height,
		})
		cmd := a.detailView.Init()
		return a, cmd
	}
	return a.switchTab(msg.Tab)
}

func (a App) switchTab(tab Tab) (tea.Model, tea.Cmd) {
	if tab == a.activeTab && a.detailView == nil {
		return a, nil
	}
	a.activeTab = tab
	a.detailView = nil
	a.activeView = a.viewForTab(tab)
	// Send the current dimensions so tables get a width.
	a.activeView, _ = a.activeView.Update(tea.WindowSizeMsg{
		Width: a.width, Height: a.height,
	})
	cmd := a.activeView.Init()
	return a, cmd
}

func (a App) viewForTab(tab Tab) View {
	switch tab {
	case TabProjects:
		return NewProjectsView(a.client)
	case TabSettings:
		return NewSettingsView(a.client)
	case TabAudit:
		return NewAuditLogView(a.client)
	case TabAccess:
		return NewAccessView(a.client)
	default:
		return NewProjectsView(a.client)
	}
}

// rebuildTabs sets the tab list based on event log mode.
func (a *App) rebuildTabs() {
	a.tabs = []Tab{TabProjects, TabSettings, TabAccess}
	if a.auditLogMode != api.AuditLogOff {
		a.tabs = append(a.tabs, TabAudit)
	}
	a.tabLabels = make([]string, len(a.tabs))
	for i, tab := range a.tabs {
		a.tabLabels[i] = TabLabels[tab]
	}
}

func (a App) contentWidth() int {
	w := a.width - 4
	if w < 20 {
		return 20
	}
	return w
}

// mergedKeyMap combines a view's keybindings with global keybindings.
// ShortHelp shows only the view's bindings; FullHelp appends global
// bindings (tab navigation, quit) as an additional column.
type mergedKeyMap struct {
	view   help.KeyMap
	global GlobalKeyMap
}

// ShortHelp delegates to the view's short help.
func (m mergedKeyMap) ShortHelp() []key.Binding {
	return m.view.ShortHelp()
}

// FullHelp appends global bindings (including j/k navigation) to the view's full help.
func (m mergedKeyMap) FullHelp() [][]key.Binding {
	groups := m.view.FullHelp()
	groups = append(groups, []key.Binding{
		navBinding, m.global.Quit,
	})
	return groups
}

// waitForEvent bridges a Go channel to Bubble Tea's command system.
// It blocks in a goroutine until an event arrives, then returns it as
// a message. After handling each SSEEventMsg, the app re-issues this
// command to keep listening — creating a continuous event loop.
//
// This is the canonical Go pattern for SSE consumption in Bubble Tea:
//
//	// In Init():
//	cmds = append(cmds, waitForEvent(ch))
//
//	// In Update(), after handling SSEEventMsg:
//	return a, tea.Batch(viewCmd, waitForEvent(a.eventCh))
func waitForEvent(ch <-chan eventbus.SSEEvent) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return EventStreamClosedMsg{}
		}
		return SSEEventMsg(event)
	}
}

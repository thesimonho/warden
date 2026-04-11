package tui

import (
	"context"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/thesimonho/warden/api"
)

// focusHeartbeatInterval must be less than the server's 45s focusEntryTTL.
const focusHeartbeatInterval = 30 * time.Second

// focusReporter tracks the TUI's focus state and reports it to the server
// so the system tray can suppress desktop notifications when the user is
// actively viewing a project.
type focusReporter struct {
	client   Client
	clientID string
	focused  bool
	// Current project context (empty when on the project list or other tabs).
	projectID   string
	agentType   string
	worktreeIDs []string
}

// newFocusReporter creates a focus reporter with a random client ID.
func newFocusReporter(client Client) *focusReporter {
	return &focusReporter{
		client:   client,
		clientID: uuid.NewString(),
	}
}

// setProjectContext updates the project being viewed and sends an update.
// No-ops if the context hasn't changed.
func (fr *focusReporter) setProjectContext(projectID, agentType string, worktreeIDs []string) tea.Cmd {
	if fr.projectID == projectID && fr.agentType == agentType &&
		slices.Equal(fr.worktreeIDs, worktreeIDs) {
		return nil
	}
	fr.projectID = projectID
	fr.agentType = agentType
	fr.worktreeIDs = worktreeIDs
	return fr.sendUpdate()
}

// clearProjectContext clears the project context and sends an update.
// No-ops if already cleared.
func (fr *focusReporter) clearProjectContext() tea.Cmd {
	if fr.projectID == "" {
		return nil
	}
	fr.projectID = ""
	fr.agentType = ""
	fr.worktreeIDs = nil
	return fr.sendUpdate()
}

// handleFocus processes a terminal focus event.
func (fr *focusReporter) handleFocus() tea.Cmd {
	if fr.focused {
		return nil
	}
	fr.focused = true
	return fr.sendUpdate()
}

// handleBlur processes a terminal blur event.
func (fr *focusReporter) handleBlur() tea.Cmd {
	if !fr.focused {
		return nil
	}
	fr.focused = false
	return fr.sendUpdate()
}

// sendUpdate sends the current focus state to the server.
func (fr *focusReporter) sendUpdate() tea.Cmd {
	req := api.FocusRequest{
		ClientID: fr.clientID,
		Focused:  fr.focused && fr.projectID != "",
	}
	if req.Focused {
		req.ProjectID = fr.projectID
		req.AgentType = fr.agentType
		req.WorktreeIDs = fr.worktreeIDs
	}

	return func() tea.Msg {
		// Best-effort: focus tracking should not interrupt TUI operation.
		_ = fr.client.ReportFocus(context.Background(), req)
		return nil
	}
}

// heartbeatCmd returns a command that ticks the focus heartbeat.
func (fr *focusReporter) heartbeatCmd() tea.Cmd {
	return tea.Tick(focusHeartbeatInterval, func(time.Time) tea.Msg {
		return focusHeartbeatMsg{}
	})
}

// handleHeartbeat refreshes the server-side TTL if focused.
func (fr *focusReporter) handleHeartbeat() tea.Cmd {
	if fr.focused && fr.projectID != "" {
		return tea.Batch(fr.sendUpdate(), fr.heartbeatCmd())
	}
	return fr.heartbeatCmd()
}

// focusHeartbeatMsg is sent periodically to refresh the focus TTL.
type focusHeartbeatMsg struct{}

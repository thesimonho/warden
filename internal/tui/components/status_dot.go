// Package components provides reusable TUI widgets for the Warden terminal UI.
package components

import (
	"charm.land/lipgloss/v2"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/event"
)

// Status dot colors.
var (
	dotGreen  = lipgloss.NewStyle().Foreground(ColorSuccess)
	dotAmber  = lipgloss.NewStyle().Foreground(ColorWarning)
	dotPurple = lipgloss.NewStyle().Foreground(ColorPurple)
	dotGray   = lipgloss.NewStyle().Foreground(ColorGray)
	dotBlue   = lipgloss.NewStyle().Foreground(ColorBlue)
	dotRed    = lipgloss.NewStyle().Foreground(ColorError)
	dotOrange = lipgloss.NewStyle().Foreground(ColorOrange)
)

// WorktreeStateStyle returns the lipgloss style for a worktree state.
// Use this to color both the dot and status text consistently.
func WorktreeStateStyle(state engine.WorktreeState) lipgloss.Style {
	switch state {
	case engine.WorktreeStateConnected:
		return dotGreen
	case engine.WorktreeStateShell:
		return dotAmber
	case engine.WorktreeStateBackground:
		return dotPurple
	case engine.WorktreeStateStopped:
		return dotGray
	default:
		return dotGray
	}
}

// WorktreeStateDot returns a styled dot character for the given worktree state.
// States follow docs/developer/terminology.md:
//   - connected → green ●
//   - shell     → amber ●
//   - background → purple ●
//   - stopped → gray ○
func WorktreeStateDot(state engine.WorktreeState) string {
	switch state {
	case engine.WorktreeStateConnected:
		return dotGreen.Render("●")
	case engine.WorktreeStateShell:
		return dotAmber.Render("●")
	case engine.WorktreeStateBackground:
		return dotPurple.Render("●")
	case engine.WorktreeStateStopped:
		return dotGray.Render("○")
	default:
		return dotGray.Render("○")
	}
}

// AttentionDot returns a styled dot for attention/notification states.
// These overlay the worktree state dot when Claude needs input.
//   - permission_prompt    → orange ◉
//   - elicitation_dialog  → red ◉
//   - idle_prompt         → blue ◉
func AttentionDot(notificationType event.NotificationType) string {
	switch notificationType {
	case event.NotificationPermissionPrompt:
		return dotOrange.Render("◉")
	case event.NotificationElicitationDialog:
		return dotRed.Render("◉")
	case event.NotificationIdlePrompt:
		return dotBlue.Render("◉")
	default:
		return ""
	}
}

// Container state strings as reported by Docker.
const (
	ContainerStateRunning = "running"
	ContainerStateExited  = "exited"
	ContainerStateStopped = "stopped"
)

// ContainerStateDot returns a styled dot for container state.
func ContainerStateDot(state string) string {
	switch state {
	case ContainerStateRunning:
		return dotGreen.Render("●")
	case ContainerStateExited, ContainerStateStopped:
		return dotAmber.Render("●")
	default:
		return dotGray.Render("○")
	}
}

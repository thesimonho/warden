package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/lipgloss/v2"

	"github.com/thesimonho/warden/internal/tui/components"
)

// Color aliases — single source of truth is components.Color*.
var (
	colorGray    = components.ColorGray
	colorSubtle  = components.ColorSubtle
	colorAccent  = components.ColorAccent
	colorSuccess = components.ColorSuccess
	colorError   = components.ColorError
	colorWarning = components.ColorWarning
	colorWhite   = lipgloss.BrightWhite
)

// Styles defines reusable lipgloss styles for the TUI.
var Styles = struct {
	// Layout
	App         lipgloss.Style
	Header      lipgloss.Style
	Footer      lipgloss.Style
	StatusBar   lipgloss.Style
	ContentArea lipgloss.Style

	// Text
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Muted    lipgloss.Style
	Error    lipgloss.Style
	Success  lipgloss.Style
	Warning  lipgloss.Style
	Bold     lipgloss.Style
}{
	App: lipgloss.NewStyle().Padding(1, 2),
	Header: lipgloss.NewStyle().
		Bold(true).
		Foreground(colorAccent),
	Footer: lipgloss.NewStyle().
		Foreground(colorSubtle),
	StatusBar: lipgloss.NewStyle().
		Foreground(colorSubtle).
		Padding(0, 1),
	ContentArea: lipgloss.NewStyle().
		Padding(1, 0),

	Title: lipgloss.NewStyle().
		Bold(true),
	Subtitle: lipgloss.NewStyle().
		Foreground(colorSubtle),
	Muted: lipgloss.NewStyle().
		Foreground(colorGray),
	Error: lipgloss.NewStyle().
		Foreground(colorError),
	Success: lipgloss.NewStyle().
		Foreground(colorSuccess),
	Warning: lipgloss.NewStyle().
		Foreground(colorWarning),
	Bold: lipgloss.NewStyle().
		Bold(true),
}

// helpStyles returns help.Styles with light grey text matching inactive tabs.
func helpStyles() help.Styles {
	keyStyle := lipgloss.NewStyle().Foreground(colorSubtle)
	descStyle := lipgloss.NewStyle().Foreground(colorGray)
	sepStyle := lipgloss.NewStyle().Foreground(colorGray)
	return help.Styles{
		ShortKey:       keyStyle,
		ShortDesc:      descStyle,
		ShortSeparator: sepStyle,
		FullKey:        keyStyle,
		FullDesc:       descStyle,
		FullSeparator:  sepStyle,
		Ellipsis:       sepStyle,
	}
}

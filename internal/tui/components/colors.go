package components

import "charm.land/lipgloss/v2"

// ANSI basic 16 color palette. Using the terminal's own colors so the
// TUI inherits the user's color scheme automatically. Exported so the
// parent tui package can reference them without duplication.
var (
	ColorGray    = lipgloss.Black
	ColorSubtle  = lipgloss.BrightWhite
	ColorAccent  = lipgloss.Blue
	ColorSuccess = lipgloss.Green
	ColorError   = lipgloss.Red
	ColorWarning = lipgloss.Yellow
	ColorPurple  = lipgloss.Magenta
	ColorBlue    = lipgloss.Blue
	ColorOrange  = lipgloss.BrightYellow
)

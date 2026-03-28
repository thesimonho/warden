package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Custom borders for the tab bar, following the Bubble Tea tabs example.
// Active tabs have an open bottom edge to visually connect to the content
// area below, while inactive tabs have a closed bottom.
var (
	activeTabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      " ",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┘",
		BottomRight: "└",
	}
	inactiveTabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "─",
		BottomRight: "─",
	}

	tabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent).
			Border(activeTabBorder, true).
			BorderForeground(ColorAccent).
			Padding(0, 1)
	tabInactive = lipgloss.NewStyle().
			Foreground(ColorSubtle).
			Border(inactiveTabBorder, true).
			BorderForeground(ColorGray).
			BorderBottomForeground(ColorAccent).
			Padding(0, 1)
	tabGapStyle = lipgloss.NewStyle().
			Foreground(ColorAccent)
)

// RenderTabBar renders a horizontal tab bar with numbered labels
// and the active tab highlighted. Labels are prefixed with [1], [2], etc.
// The active tab has an open bottom border to connect to the content below.
func RenderTabBar(labels []string, activeIndex int, width int) string {
	var tabs []string
	for i, label := range labels {
		numbered := fmt.Sprintf("[%d] %s", i+1, label)
		if i == activeIndex {
			tabs = append(tabs, tabActive.Render(numbered))
		} else {
			tabs = append(tabs, tabInactive.Render(numbered))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)

	// Fill remaining width with a bottom border to complete the tab bar line.
	rowWidth := lipgloss.Width(row)
	if gap := width - rowWidth; gap > 0 {
		fill := tabGapStyle.Render(strings.Repeat("─", gap))
		// Append the fill to the last line of the row (the bottom border line).
		lines := strings.Split(row, "\n")
		lines[len(lines)-1] += fill
		row = strings.Join(lines, "\n")
	}

	return row
}

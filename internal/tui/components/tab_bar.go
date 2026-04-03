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

// versionStyle renders the version string in a subtle color on the tab bar.
var versionStyle = lipgloss.NewStyle().
	Foreground(ColorSubtle)

// RenderTabBar renders a horizontal tab bar with numbered labels
// and the active tab highlighted. Labels are prefixed with [1], [2], etc.
// The active tab has an open bottom border to connect to the content below.
// The version string is rendered right-aligned in the gap fill area.
func RenderTabBar(labels []string, activeIndex int, width int, versionStr string) string {
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

	// Fill remaining width with a bottom border, placing the version
	// string right-aligned before the trailing edge.
	rowWidth := lipgloss.Width(row)
	if gap := width - rowWidth; gap > 0 {
		versionRendered := ""
		versionWidth := 0
		if versionStr != "" {
			versionRendered = versionStyle.Render(versionStr)
			versionWidth = lipgloss.Width(versionRendered)
		}

		fillWidth := gap - versionWidth
		if fillWidth < 0 {
			fillWidth = 0
			versionRendered = ""
		}

		fill := tabGapStyle.Render(strings.Repeat("─", fillWidth)) + versionRendered
		lines := strings.Split(row, "\n")
		lines[len(lines)-1] += fill
		row = strings.Join(lines, "\n")
	}

	return row
}

# TUI Components

Located at `internal/tui/components/`.

## Files

| File | Purpose |
| --- | --- |
| `status_dot.go` | Maps `WorktreeState`, `NotificationType`, and container state to styled unicode dot characters. Exports `FormatStatusDot()` function. Defines state→color mapping (connected=green, shell=amber, background=purple, disconnected=gray, working=pulsing, etc.). |
| `cost.go` | `FormatCost(cents int64) string` and `FormatDuration(d time.Duration) string` — ported from `web/src/lib/cost.ts`. |
| `tab_bar.go` | Renders horizontal tab bar with active tab highlighted, inactive tabs muted. Used in the main UI header. |
| `colors.go` | Single source of truth for ANSI basic 16 color palette. Exported vars (`ColorAccent`, `ColorGray`, etc.) used by both components and parent `tui` package. |
| `directory_browser.go` | `DirectoryBrowser` — navigable filesystem tree with scrolling support. `SetHeight()` constrains visible rows, auto-adjusts scroll offset to keep cursor visible. Loads directories via `Client.ListDirectories()`. |

## Tests

| File | Purpose |
| --- | --- |
| `status_dot_test.go` | All worktree states × notification types → correct styled output. |
| `cost_test.go` | Cost and duration formatting tests ported from frontend. |
| `directory_browser_test.go` | Height clamping, entry loading, error handling, keyboard navigation, scroll offset tracking, scroll indicators. |
| `tab_bar_test.go` | Tab bar plain text output verification with ANSI stripping. |

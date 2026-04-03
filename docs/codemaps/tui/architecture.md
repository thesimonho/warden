# TUI Architecture

Terminal user interface using Bubble Tea v2. Located at `internal/tui/`. Written against the `Client` interface — serves as both a product and a reference implementation for Go developers.

## Design Principles

- **Tab-based navigation** — Three top-level tabs: Projects (default), Settings, Audit. Tab switching via `[1]`, `[2]`, `[3]` keys.
- **Project view** → worktree detail flow — Click a project to see its worktrees (bubbles/list with custom delegate for colored state dots).
- **Forms** — Container creation/edit uses native bubbles components (textinput, textarea, directory browser). Full field set: name, path, skip permissions, network mode, allowed domains, and an Advanced collapsible section with image, bind mounts, and environment variables.
- **Terminal passthrough** — Pressing `enter` on a connected worktree runs `tea.Exec()` to yield the terminal to the remote PTY. User presses configured disconnect key (default `ctrl+\`) to return to Warden.
- **ANSI basic 16 colors** — All colors use terminal palette constants (`lipgloss.BrightBlue`, `lipgloss.Red`, etc.) so the TUI inherits the user's terminal theme. Single source of truth in `components/colors.go`.
- **Real-time updates** — SSE subscription in `NewApp()` (not `Init()`) broadcasts worktree state and cost changes to all views via `SSEEventMsg`.

## Key Files

| File | Purpose |
| --- | --- |
| `client.go` | `Client` interface — the key architectural boundary. Each method maps 1:1 to an API endpoint with GoDoc comment. Methods: list/add/remove projects, reset costs, purge audit, list/create/connect worktrees, terminal operations, settings, event log, defaults, event subscription. |
| `adapter.go` | `ServiceAdapter` wraps `*Warden` to satisfy `Client` for embedded mode (TUI binary). Most methods delegate to `w.Service`; `AttachTerminal` uses docker exec; `SubscribeEvents` uses `w.Broker.Subscribe()`. Enables TUI and external Go clients to use the same interface. |
| `app.go` | Root `tea.Model` — manages four tabs, active view routing, SSE event subscription (started in `NewApp()`), global key handling (quit, help, tab switching). Delegates `Update()` and `Render()` to active view. Tracks `auditLogMode` setting and `disconnectKey`. |
| `common.go` | Message types: `ProjectsLoadedMsg`, `WorktreesLoadedMsg`, `SettingsLoadedMsg`, `RuntimesLoadedMsg`, `EventLogLoadedMsg`, `AuditLogLoadedMsg`, `AuditProjectsLoadedMsg`, `DefaultsLoadedMsg`, `OperationResultMsg`, `NavigateMsg`, `NavigateBackMsg`, `SSEEventMsg`, `EventStreamClosedMsg`. `Tab` enum: `TabProjects`, `TabSettings`, `TabEventLog`, `TabAuditLog`. |
| `keymap.go` | Key binding definitions using `bubbles/key`. Keymaps: `GlobalKeyMap`, `ProjectKeyMap`, `WorktreeKeyMap`, `SettingsKeyMap`, `AuditLogKeyMap`, `ManageKeyMap`. |
| `render.go` | Shared rendering helpers: `padRight`, `truncate`. |
| `terminal.go` | `TerminalExecCmd` struct — bridges stdin/stdout to `client.TerminalConnection` for terminal passthrough mode. Handles raw terminal mode, SIGWINCH resize, and graceful close on disconnect key press. |
| `theme.go` | Lip Gloss v2 styles — imports colors from `components/colors.go`. Defines `Styles` struct with layout, text, and component styles. |
| `validate.go` | `ValidateWorktreeName` — git branch name validation rules ported from webapp. |

## Key Constants and Enums

- `Tab` enum: `TabProjects` (0), `TabSettings` (1), `TabEventLog` (2), `TabAuditLog` (3).
- `TabLabels` map: maps `Tab` to display string ("Projects", "Settings", "Event Log", "Audit Log").
- **Keybindings**: `[1]` = Projects, `[2]` = Settings, `[3]` = Event Log, `[4]` = Audit Log.
- **Standardized action keys**: `n` = new, `x` = remove, `X` = kill, `j`/`k` = navigate.
- **Disconnect key** — configurable in settings, defaults to `config.DefaultDisconnectKey` (typically `"ctrl+\\"`).

## Tests

| File | Purpose |
| --- | --- |
| `validate_test.go` | Worktree name validation tests — all git branch naming rules. |
| `adapter_test.go` | Interface compliance tests for `ServiceAdapter`. |
| `client_compliance_test.go` | Compile-time check: `client.Client` satisfies `tui.Client`. |
| `view_container_form_test.go` | Form logic: field visibility, cursor navigation, mount/env sub-cursor, add/remove items, submit validation, sensitive key detection. |

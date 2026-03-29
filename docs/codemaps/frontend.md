# Frontend Codemap

## App Structure

| File            | Purpose                                                                                                           |
| --------------- | ----------------------------------------------------------------------------------------------------------------- |
| `src/main.tsx`  | React root mount                                                                                                  |
| `src/App.tsx`   | Router setup: `/` (home), `/projects/:id` (project), `/workspace`, `/audit` |
| `src/index.css` | Tailwind v4 base + theme imports; custom font-size scale (shifted down one level: `text-base` = 14px), h1-h3 base styles |

## Pages

| File                           | Route           | Purpose                                    |
| ------------------------------ | --------------- | ------------------------------------------ |
| `src/pages/home-page.tsx`      | `/`             | Project grid with cost dashboard and cards  |
| `src/pages/project-page.tsx`   | `/projects/:id` | Route wrapper — reads project ID from URL, renders `ProjectView` in fixed viewport layout. Subscribes to `budget_container_stopped` SSE events and auto-redirects to home page when the current project's container is stopped by budget enforcement. |
| `src/pages/workspace-page.tsx` | `/workspace`    | Grid of embedded `ProjectView` instances. Each cell is a full project view with its own sidebar and grid/canvas toggle. Project IDs from `?ids=` query param. |
| `src/pages/audit-page.tsx`    | `/audit`        | Audit log viewer with summary dashboard (sessions/tools/prompts/cost), category filters, level filters, project filter, activity timeline brush, CSV/JSON export, scoped delete dialog (by project/category/age) with type-to-confirm |

## Components

### Shared (flat)

| File                                      | Purpose                                                                             |
| ----------------------------------------- | ----------------------------------------------------------------------------------- |
| `components/layout.tsx`                   | App shell — header with nav title ("Warden"), theme toggle, audit nav button |
| `components/settings-dialog.tsx`          | Settings panel (runtime selection, notifications toggle, auditLogMode radio group [off/standard/detailed], default project budget input, budget enforcement action toggles: show warning, stop worktrees, stop container, prevent restart) |
| `components/theme-toggle.tsx`             | Light/dark theme switcher                                                           |
| `components/project-filter.tsx`           | Reusable project name filter dropdown (autocomplete from event log projects)        |
| `components/activity-timeline.tsx`        | Stacked bar chart with brush/scrubber for time range selection. Uses recharts via shadcn chart wrapper. Bars stacked by audit category (session/agent/prompt/config/budget/system/debug), colored with `--category-*` CSS variables. Emits `since`/`until` ISO strings on brush drag. |
| `components/audit-log-table.tsx`          | Virtualized audit log table (TanStack Table + TanStack Virtual). Columns: timestamp, project ID, project (containerName), worktree, level, category, event, message. Fuzzy search, resizable columns, expandable rows with wrapped JSON detail (`<pre>` with `whitespace-pre-wrap`), persistent sorting/sizing. |

### Home (`components/home/`)

Components for the home page and project management.

| File                                      | Purpose                                                                             |
| ----------------------------------------- | ----------------------------------------------------------------------------------- |
| `components/home/project-grid.tsx`        | Grid of project cards                                                               |
| `components/home/project-card.tsx`        | Card showing project status, worktree counts, cost, Claude status, network mode indicator (shield icon for non-full modes), actions |
| `components/home/cost-dashboard.tsx`      | Aggregate stats bar: running projects, active worktrees, total cost                 |
| `components/home/claude-status-indicator.tsx` | Visual indicator for Claude's status (idle/working/needs permission/needs input/waiting for prompt) |
| `components/home/status-badge.tsx`        | Container state badge (running/stopped/etc.)                                        |
| `components/home/add-project-dialog.tsx`  | Dialog: create new project, edit existing, or create container for no-container project via ProjectConfigForm. Exports `CreateForProject` type. |
| `components/home/manage-project-dialog.tsx` | Management dialog with four independent destructive actions: remove from Warden, delete container, reset costs, purge audit. Type-to-confirm for audit purge. Keeps dialog open on partial failure. |
| `components/home/recent-workspaces.tsx`   | Recently visited workspace tabs                                                     |
| `components/home/project-config-form.tsx` | Create/edit form with fields: name, workspace path, image, permissions, network mode, allowed domains, cost budget, and Advanced collapsible with bind mounts and environment variables. Mount presets (Git, SSH) are fetched from `/api/v1/defaults`, toggled via checkboxes, and their mounts/env vars are merged into form data on submit. User-defined mounts are kept separate from preset mounts. `ProjectConfigFormData` includes `enabledPresets: string[]`. Used inside `add-project-dialog.tsx`. |
| `components/home/directory-browser.tsx`   | Fuzzy-finder style filesystem picker. Supports `mode="directory"` (default, directory-only) and `mode="file"` (directories + files, clicking a file commits its path). Split input with browsable prefix and filter. |


### Project (`components/project/`)

Components for the project view, used by both `project-page.tsx` and `workspace-page.tsx`.

| File                                      | Purpose                                                                             |
| ----------------------------------------- | ----------------------------------------------------------------------------------- |
| `components/project/project-view.tsx`     | Core project UI — sidebar with view mode toggle + grid or canvas terminal display. Accepts `projectId` as a prop. Used standalone by `project-page.tsx` and embedded in `workspace-page.tsx`. Each instance has independent canvas/panel state. Callbacks use refs for volatile values (`canvasSize`, `transform.scale`, `panels`) to keep function identity stable across SSE-driven re-renders. |
| `components/project/project-sidebar.tsx`  | Project sidebar: view mode toggle (Grid/Canvas), controlled project dropdown (URL-driven), worktree list with right-click context menu (Reveal in File Manager + Disconnect + Remove actions) |
| `components/project/canvas-view.tsx`      | Draggable/resizable Rnd panel wrapping a `TerminalCard` on the canvas: position/size from store, selection ring, CSS transition animations, shift+click selection, `data-canvas-panel` attr for marquee hit detection. Custom memo comparison on data props only (callbacks are stable). |
| `components/project/grid-view.tsx`        | Grid display mode: renders `TerminalCard` instances in a responsive CSS grid layout (no pan/zoom/drag). Auto-sizes grid columns based on panel count. `GridCell` uses custom memo on data props only. |
| `components/project/terminal-card.tsx`    | Terminal with title bar chrome (status dot, project name, branch, attention indicator, action buttons) + xterm.js rendering via `useTerminal` hook. Tab toggle (Terminal / Changes) in title bar. Terminal div stays mounted but hidden when Changes tab is active to preserve xterm.js instance. Layout-agnostic — used by both canvas-view (in Rnd) and grid-view (in CSS grid). |
| `components/project/changes-view.tsx`    | GitHub-style "Files changed" view: file list with +/- stats, status badges (A/M/D/R), click-to-expand per-file inline diffs via `@git-diff-view/react`. All files collapsed by default. Refresh button for on-demand refetch. |
| `components/project/worktree-list.tsx`    | Worktree list with header (new worktree button, refresh), grouped item rendering (On Canvas / Available), and `WorktreeRow` — individual worktree entry with status dot, branch, connect icon, right-click context menu (Reveal in File Manager, Disconnect, Remove). |
| `components/project/new-worktree-dialog.tsx` | Dialog to create a new git worktree with name validation (git branch rules) |
| `components/project/remove-worktree-dialog.tsx` | Confirmation dialog before removing a worktree, warns about uncommitted changes being lost |

### UI Primitives (`components/ui/`)

| File              | Source    | Notes                                                                                                                      |
| ----------------- | --------- | -------------------------------------------------------------------------------------------------------------------------- |
| `alert-dialog.tsx` | shadcn/ui | Confirmation dialog for destructive actions                                                                                 |
| `badge.tsx`       | shadcn/ui |                                                                                                                            |
| `chart.tsx`       | shadcn/ui | Recharts wrapper: `ChartContainer` (themed ResponsiveContainer), `ChartTooltip`, `ChartTooltipContent`, `ChartLegend`, `ChartLegendContent`. Generates CSS variables per chart config key. |
| `button.tsx`      | shadcn/ui | Props: `icon` (React.ElementType) for auto-sized icon placement, `loading` (boolean) for spin animation. Icons auto-sized by variant, not overridable. |
| `card.tsx`        | shadcn/ui |
| `context-menu.tsx` | shadcn/ui | Right-click context menu for actions (e.g., worktree Disconnect/Remove)                                                    |
| `popover.tsx`     | shadcn/ui | Floating popover                                                                                |
| `select.tsx`      | shadcn/ui | Select dropdown                                                                                                            |
| `checkbox.tsx`    | shadcn/ui |                                                                                                                            |
| `collapsible.tsx` | shadcn/ui |                                                                                                                            |
| `dialog.tsx`      | shadcn/ui |                                                                                                                            |
| `input.tsx`       | shadcn/ui |                                                                                                                            |
| `radio-group.tsx` | shadcn/ui |                                                                                                                            |
| `scroll-area.tsx` | shadcn/ui |                                                                                                                            |
| `separator.tsx`   | shadcn/ui |                                                                                                                            |
| `skeleton.tsx`    | shadcn/ui |                                                                                                                            |
| `switch.tsx`      | shadcn/ui |                                                                                                                            |
| `tabs.tsx`        | shadcn/ui |                                                                                                                            |
| `textarea.tsx`    | shadcn/ui |                                                                                                                            |
| `tooltip.tsx`     | shadcn/ui |                                                                                                                            |

#### Text Sizing

The font-size scale in `src/index.css` is shifted down one level from Tailwind defaults, making body text default to 14px.

| Class       | Size  | Use for                        |
| ----------- | ----- | ------------------------------ |
| `text-xs`   | 10px  | Fine print, zoom indicators    |
| `text-sm`   | 12px  | Secondary labels, metadata     |
| `text-base` | 14px  | Body text (inherited default)  |
| `text-lg`   | 16px  | Emphasis, subheadings          |
| `text-xl`   | 18px  | h2 headings                    |
| `text-2xl`  | 20px  | h1 headings                    |

Heading elements (`h1`--`h3`) have base styles defined in `src/index.css` -- don't repeat sizing/weight on heading tags.

## Libraries

| File              | Purpose                                                                                                                                                                                                                                                                                                       |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `lib/api.ts`      | API client: centralized `apiFetch` helper and `ApiError` class with `code` and `status` fields for programmatic error handling; all mutation endpoints return typed result objects (`ProjectResult`, `WorktreeResult`, `ContainerResult`) instead of status strings; endpoints: `fetchProjects`, `stopProject` → `ProjectResult`, `restartProject` → `ProjectResult`, `fetchWorktrees`, `createWorktree` → `WorktreeResult`, `connectTerminal` → `WorktreeResult`, `disconnectTerminal` → `WorktreeResult`, `removeWorktree` → `WorktreeResult`, `createContainer` → `ContainerResult`, `listDirectories`, `addProject` → `ProjectResult`, `removeProject` → `ProjectResult`, `deleteContainer` → `ContainerResult`, `resetProjectCosts`, `purgeProjectAudit`, `validateContainer`, `fetchRuntimes`, `fetchSettings`, `updateSettings`, `fetchDefaults` → `DefaultsResponse` with `presets`, `revealInFileManager`, `worktreeHostPath`, `fetchAuditLog`, `fetchAuditSummary`, `fetchAuditProjects`, `postAuditEvent`, `deleteAuditEvents`, `auditExportUrl` |
| `lib/cost.ts`     | Cost and duration formatting: `formatCost`, `formatDuration`                                                                                                                                                                                                                                                   |
| `lib/settings.ts` | localStorage-backed settings (`loadSettings`, `saveSettings`, `DEFAULT_SETTINGS`) — notifications toggle only (event log setting is server-side) |
| `lib/types.ts`    | Shared types: `Project` (incl. `projectId`, `costBudget`, `isEstimatedCost`, `mountedDir`, `workspaceDir`), `WorkspaceMount` (host↔container path pair, with `workspaceMount()` extractor), `ContainerConfig` (incl. `enabledPresets`, `costBudget`), `Worktree`, `ProjectResult`, `WorktreeResult`, `ContainerResult`, `CreateContainerRequest` (incl. `enabledPresets`, `costBudget`), `MountPreset` (preset ID, label, description, availability, mounts [], env vars []), `NetworkMode` enum, `ClaudeStatus`, `WorktreeState`, `NotificationType`, `RuntimeInfo`, `ServerSettings` (incl. `defaultProjectBudget`, `auditLogMode`, `budgetAction{Warn,StopWorktrees,StopContainer,PreventStart}`), `AuditLogMode` (off/standard/detailed), `DirEntry`, `WorktreeStateEvent`, `ProjectStateEvent`, `BudgetEventPayload` (shared base: projectId, containerName, totalCost, budget), `BudgetExceededEvent` (= `BudgetEventPayload`), `BudgetContainerStoppedEvent` (extends with containerId), `AuditSource` enum, `AuditLogEntry`, `AuditCategory` (session/agent/prompt/config/budget/system/debug), `AuditFilters` (incl. `projectID` instead of `container`, `source` and `level`), `AuditSummary`, `ToolCount`, `TimeRange`, `PostAuditEventRequest`, `ProjectConfigFormData` (incl. `enabledPresets`, `costBudget`); helpers `hasActiveTerminal()`, `isConnectedWorktree()`, `isSessionAlive()` |
| `lib/canvas-store.ts` | Canvas panel state hook (`useCanvasStore`): panel CRUD, selection (`selectPanels`, `toggleSelection`, `clearSelection`), layout actions (`applyLayout` — grid/horizontal/vertical), smart new-panel positioning (right of focused panel or viewport center), layout animation flag (`isLayoutAnimating`, `withLayoutAnimation`, `clearLayoutAnimation`) |
| `lib/canvas-layout.ts` | Pure layout computation functions: `layoutGrid`, `layoutHorizontal`, `layoutVertical` — arrange panels by bounding box anchor, preserving individual dimensions |
| `lib/domain-groups.ts` | Domain group presets for restricted network mode: `DomainGroup` type, `domainGroups` (Minimal, AI Dev, Web Dev, Full Stack), `findMatchingGroup()` |
| `lib/notification-config.ts` | Single source of truth for notification type display properties (label, dot color, text color, message). `getAttentionConfig()` returns config for a given `NotificationType`. Used by status indicators, canvas sidebar, worktree cards, and notification hooks. |
| `lib/activity-buckets.ts` | Pure bucketing logic for the activity timeline: groups `AuditLogEntry[]` into adaptive time buckets (1h/6h/1d/1w/30d based on data span) with counts per audit category (session, agent, prompt, config, budget, system, debug). Exports `chooseBucketWidth()`, `CATEGORY_KEYS`, `ActivityBucket`/`BucketResult` types, and `bucketEventsByCategory()`. |
| `lib/terminal-themes.ts` | Builds xterm.js `ITheme` from active CSS `--terminal-*` custom properties. `getTerminalTheme()` reads CSS variables so terminal colors follow light/dark mode. |
| `lib/utils.ts`    | `cn()` (tailwind class merge), `relativeTime()` (human-readable timestamps), `abbreviateHomePath()` (~/... display for home directories), `parentDir()` (parent directory of a POSIX path) |

## External Libraries

| Library | Purpose |
| ------- | ------- |
| recharts | Charting library (MIT). Used via shadcn `chart.tsx` wrapper for the activity timeline brush/scrubber. |
| sonner  | Toast notifications for user feedback (success, error, warning messages) |
| Prettier | Code formatter with eslint-config-prettier integration; `npm run format` formats src files; prettier-plugin-tailwindcss sorts Tailwind classes |

## Hooks

| File                             | Purpose                                                                         |
| -------------------------------- | ------------------------------------------------------------------------------- |
| `hooks/use-projects.ts`          | Polls `/api/projects` at configurable interval, provides loading/refreshing/error/refetch. Handles `budget_exceeded` SSE events with toast notification and auto-refetch. Budget container-stopped redirect is handled in `project-page.tsx` via `use-event-source.ts`. |
| `hooks/use-diff.ts`              | On-demand diff fetch for the Changes tab. Fetches when `enabled` is true, returns `{ diff, isLoading, error, refetch }`. No polling/SSE. |
| `hooks/use-worktrees.ts`         | Polls `/api/projects/{id}/worktrees` for worktree state; applies push-based terminal state from SSE `WorktreeStateEvent` |
| `hooks/use-terminal.ts`          | xterm.js lifecycle: create/attach xterm instance, WebSocket connection to `/api/projects/{id}/ws/{wid}`, resize events, reconnect logic, cleanup on unmount. Buffers incoming WebSocket messages and flushes to xterm.js once per `requestAnimationFrame` to prevent rapid output bursts from blocking the main thread. |
| `hooks/use-notifications.ts`     | Browser notifications for worktree events (needs input, all worktrees complete) |
| `hooks/use-theme.ts`             | Theme persistence to localStorage (`warden-theme`)                              |
| `hooks/use-recent-workspaces.ts` | Tracks recently visited workspace projects                                      |
| `hooks/use-canvas-pan-zoom.ts`  | Pan-zoom state for infinite canvas: Ctrl+left-drag pan, Ctrl+wheel zoom, trackpad two-finger scroll pan, trackpad pinch zoom, plain wheel pass-through to xterm.js on panels, `fitAll` (animated), `panTo`, `viewportToCanvas` coordinate conversion. Middle-click is freed for native behavior. |
| `hooks/use-reveal-in-file-manager.ts` | Returns a stable callback (or undefined) to open a worktree's host directory in the system file manager. Takes a `WorkspaceMount` to map container paths to host paths. |
| `hooks/use-canvas-worktree-state.ts` | Derives worktree visual state (dots, labels, attention) for canvas panels; applies push-based terminal state from SSE `WorktreeStateEvent` |

## Themes

| File                      | Purpose     |
| ------------------------- | ----------- |
| `themes/permafrost.css`   | Light theme — defines semantic colors, `--category-*` audit category colors |
| `themes/frostpunk.css`    | Dark theme — defines semantic colors, `--category-*` audit category colors |

## Tests

### Unit Tests (Vitest)

| File                  | Tests                                                                                 |
| --------------------- | ------------------------------------------------------------------------------------- |
| `lib/api.test.ts`     | `worktreeHostPath` (modern/legacy/fallback path mapping), `createWorktree` (POST, body, content-type), `connectTerminal`, `disconnectTerminal`, `fetchWorktreeDiff` (GET, URL construction, response parsing)  |
| `lib/cost.test.ts`    | `formatCost`, `formatDuration`                                                        |
| `lib/settings.test.ts`| `loadSettings`/`saveSettings` (localStorage, including notifications toggle)          |
| `lib/types.test.ts`   | `hasActiveTerminal`, `isConnectedWorktree`, `isSessionAlive` (all worktree states incl. background) |
| `lib/domain-groups.test.ts` | Domain group data integrity, `findMatchingGroup` matching logic                |
| `lib/canvas-layout.test.ts` | `layoutGrid`, `layoutHorizontal`, `layoutVertical` (anchor, gaps, dimension preservation) |
| `lib/activity-buckets.test.ts` | `chooseBucketWidth`: all tier boundaries. `bucketEventsByCategory`: empty input, single event, multi-category counting, adaptive bucket widths, gap filling, sorting, category-less event skipping, event count preservation |
| `lib/utils.test.ts`   | `relativeTime` (with mocked time)                                                     |

### E2E Tests (Playwright)

Tests run against the live dev stack (`just dev`). Each test creates its own container via API.

| File                                | Coverage                                                                                |
| ----------------------------------- | --------------------------------------------------------------------------------------- |
| `e2e/home-page.spec.ts`            | Project card visibility, status badge, navigation via View, stop/start cycle            |
| `e2e/project-page.spec.ts`         | Sidebar worktree list, grid/canvas empty state, panel add/disconnect in both view modes  |
| `e2e/terminal-connection.spec.ts`   | Grid-mode terminal connect/show, dimensions, disconnect/reconnect (UI + API)            |
| `e2e/terminal-resilience.spec.ts`   | Grid-mode multi-terminal, disconnect independence, worktree creation, navigate-away-and-back; canvas-mode panel resize |
| `e2e/panel-maximize.spec.ts`        | Maximize/restore, maximize with terminal iframe                                         |
| `e2e/panel-layout.spec.ts`          | Multi-panel shift+click selection, grid/horizontal/vertical layout (button + keyboard), Escape deselect, fit-all |
| `e2e/project-lifecycle.spec.ts`     | Full project create/edit/stop/restart/delete lifecycle via UI                            |
| `e2e/container-integration.spec.ts` | Container infrastructure validation, worktree API operations, state transitions          |
| `e2e/navigation.spec.ts`           | Page load, browser back/forward, add project button visibility                          |

#### E2E Infrastructure

| File                        | Purpose                                                                       |
| --------------------------- | ----------------------------------------------------------------------------- |
| `playwright.config.ts`      | Chromium, headed by default, 120s timeout, artifact capture on failure         |
| `e2e/global-setup.ts`       | Creates `/tmp/warden-e2e-workspace` git repo, health-checks dev server        |
| `e2e/global-teardown.ts`    | Cleans workspace, warns about leaked `warden-e2e-*` containers                |
| `e2e/helpers/fixtures.ts`   | `test` extended with `testProject` (auto-creates/destroys container), `runtime` |
| `e2e/helpers/api.ts`        | Direct API wrappers for setup/teardown (bypass UI)                            |
| `e2e/helpers/selectors.ts`  | Centralized `data-testid` selector constants                                  |

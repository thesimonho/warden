# Frontend Testing

## Unit Tests (Vitest)

| File | Tests |
| --- | --- |
| `lib/api.test.ts` | `worktreeHostPath` (modern/legacy/fallback path mapping), `createWorktree` (POST, body, content-type), `connectTerminal`, `disconnectTerminal`, `fetchWorktreeDiff` (GET, URL construction, response parsing) |
| `lib/cost.test.ts` | `formatCost`, `formatDuration` |
| `lib/settings.test.ts` | `loadSettings`/`saveSettings` (localStorage, including notifications toggle) |
| `lib/types.test.ts` | `hasActiveTerminal`, `isConnectedWorktree`, `isSessionAlive` (all worktree states incl. background) |
| `lib/domain-groups.test.ts` | Domain group data integrity, `findMatchingGroup` matching logic |
| `lib/canvas-layout.test.ts` | `layoutGrid`, `layoutHorizontal`, `layoutVertical` (anchor, gaps, dimension preservation) |
| `lib/activity-buckets.test.ts` | `chooseBucketWidth`: all tier boundaries. `bucketEventsByCategory`: empty input, single event, multi-category counting, adaptive bucket widths, gap filling, sorting, category-less event skipping, event count preservation |
| `lib/utils.test.ts` | `relativeTime` (with mocked time) |

## E2E Tests (Playwright)

Tests run against the live dev stack (`just dev`). Each test creates its own container via API.

| File | Coverage |
| --- | --- |
| `e2e/home-page.spec.ts` | Project card visibility, status badge, navigation via View, stop/start cycle |
| `e2e/project-page.spec.ts` | Sidebar worktree list, grid/canvas empty state, panel add/disconnect in both view modes |
| `e2e/terminal-connection.spec.ts` | Grid-mode terminal connect/show, dimensions, disconnect/reconnect (UI + API) |
| `e2e/terminal-resilience.spec.ts` | Grid-mode multi-terminal, disconnect independence, worktree creation, navigate-away-and-back; canvas-mode panel resize |
| `e2e/panel-maximize.spec.ts` | Maximize/restore, maximize with terminal iframe |
| `e2e/panel-layout.spec.ts` | Multi-panel shift+click selection, grid/horizontal/vertical layout (button + keyboard), Escape deselect, fit-all |
| `e2e/project-lifecycle.spec.ts` | Full project create/edit/stop/restart/delete lifecycle via UI |
| `e2e/container-integration.spec.ts` | Container infrastructure validation, worktree API operations, state transitions |
| `e2e/navigation.spec.ts` | Page load, browser back/forward, add project button visibility |

### E2E Infrastructure

| File | Purpose |
| --- | --- |
| `playwright.config.ts` | Chromium, headed by default, 120s timeout, artifact capture on failure |
| `e2e/global-setup.ts` | Creates `/tmp/warden-e2e-workspace` git repo, health-checks dev server |
| `e2e/global-teardown.ts` | Cleans workspace, warns about leaked `warden-e2e-*` containers |
| `e2e/helpers/fixtures.ts` | `test` extended with `testProject` (auto-creates/destroys container), `runtime` |
| `e2e/helpers/api.ts` | Direct API wrappers for setup/teardown (bypass UI) |
| `e2e/helpers/selectors.ts` | Centralized `data-testid` selector constants |

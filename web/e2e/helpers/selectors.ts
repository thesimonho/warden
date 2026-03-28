/**
 * Centralized data-testid selectors for E2E tests.
 *
 * Single source of truth — if a testid changes in the component,
 * update it here and all tests pick up the change.
 */

/** Builds a `[data-testid="..."]` selector. */
function tid(id: string): string {
  return `[data-testid="${id}"]`
}

export const selectors = {
  /* ── Home page ─────────────────────────────────────────────────── */
  addProjectButton: tid('add-project-button'),
  selectModeButton: tid('select-mode-button'),
  settingsButton: tid('settings-button'),
  projectGrid: tid('project-grid'),

  /* ── Project card ──────────────────────────────────────────────── */
  projectCard: (name: string) => tid(`project-card-${name}`),
  projectViewButton: (name: string) => `${tid(`project-card-${name}`)} ${tid('view-button')}`,
  projectStopButton: (name: string) => `${tid(`project-card-${name}`)} ${tid('stop-button')}`,
  projectRestartButton: (name: string) => `${tid(`project-card-${name}`)} ${tid('restart-button')}`,
  projectStartButton: (name: string) => `${tid(`project-card-${name}`)} ${tid('start-button')}`,
  projectEditButton: (name: string) => `${tid(`project-card-${name}`)} ${tid('edit-button')}`,
  projectRemoveButton: (name: string) => `${tid(`project-card-${name}`)} ${tid('remove-button')}`,
  statusBadge: tid('status-badge'),

  /* ── Project sidebar ──────────────────────────────────────────── */
  projectSidebar: tid('project-sidebar'),
  projectSelect: tid('project-select'),
  worktreeRow: (id: string) => tid(`worktree-row-${id}`),
  worktreeStateDot: (id: string) => tid(`worktree-state-dot-${id}`),

  /* ── View mode toggle ─────────────────────────────────────────── */
  gridTab: '[role="tab"]:has-text("Grid")',
  canvasTab: '[role="tab"]:has-text("Canvas")',

  /* ── Grid view ───────────────────────────────────────────────── */
  gridView: tid('grid-view'),
  gridEmptyState: tid('grid-empty-state'),
  gridCell: (id: string) => tid(`grid-cell-${id}`),
  gridCellDisconnect: (id: string) => `${tid(`grid-cell-${id}`)} ${tid('disconnect-button')}`,

  /* ── Canvas panel ──────────────────────────────────────────────── */
  canvasPanel: (id: string) => tid(`canvas-panel-${id}`),
  canvasPanelDisconnect: (id: string) => `${tid(`canvas-panel-${id}`)} ${tid('disconnect-button')}`,
  canvasPanelMaximize: (id: string) => `${tid(`canvas-panel-${id}`)} ${tid('maximize-button')}`,
  canvasEmptyState: tid('canvas-empty-state'),

  /* ── Canvas toolbar ────────────────────────────────────────────── */
  layoutGridButton: tid('layout-grid-button'),
  layoutHorizontalButton: tid('layout-horizontal-button'),
  layoutVerticalButton: tid('layout-vertical-button'),
  fitSelectionButton: tid('fit-selection-button'),
  fitAllButton: tid('fit-all-button'),

  /* ── Worktree list ────────────────────────────────────────────── */
  newWorktreeButton: tid('new-worktree-button'),
  worktreeList: tid('worktree-list'),

  /* ── New worktree dialog ───────────────────────────────────────── */
  worktreeNameInput: tid('worktree-name-input'),
  createWorktreeButton: tid('create-worktree-button'),

  /* ── Terminal panel ────────────────────────────────────────────── */
  terminalContainer: tid('terminal-container'),
  terminalStatus: tid('terminal-status'),
  terminalStatusDot: tid('terminal-status-dot'),
  terminalDisconnectedMessage: tid('terminal-disconnected'),
} as const

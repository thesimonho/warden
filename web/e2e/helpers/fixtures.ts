import { randomUUID } from 'crypto'
import { execSync } from 'child_process'
import { existsSync, mkdirSync, writeFileSync, rmSync } from 'fs'
import path from 'path'
import { test as base, expect as baseExpect, type Page } from '@playwright/test'
import { TEST_WORKSPACE } from '../global-setup'
import {
  createTestProject,
  createWorktree,
  removeTestProject,
  waitForProjectState,
  fetchRuntimes,
  fetchWorktrees,
  killWorktreeProcess,
  waitForWorktreeState,
  type ApiRuntime,
} from './api'

/** Info about a test project created by the fixture. */
export interface TestProjectInfo {
  /** Container ID (also used as project ID in API calls). */
  id: string
  /** Container name. */
  name: string
  /** Active container runtime. */
  runtime: ApiRuntime
  /** Agent type (defaults to claude-code, overridable via WARDEN_AGENT_TYPE). */
  agentType: string
}

/**
 * Generates a unique test project name.
 *
 * Format: `warden-e2e-{random}` to make cleanup easy
 * (global teardown warns about any remaining `warden-e2e-*` containers).
 */
export function generateProjectName(): string {
  const suffix = randomUUID().slice(0, 8)
  return `warden-e2e-${suffix}`
}

/** Detects the active runtime from the Warden API. */
async function detectRuntime(): Promise<ApiRuntime> {
  const runtimes = await fetchRuntimes()
  const active = runtimes.find((r) => r.available)
  if (!active) {
    throw new Error('No container runtime available. Install Docker or Podman.')
  }
  return active
}

/**
 * Extended Playwright test with custom fixtures.
 *
 * `testProject` — worker-scoped: creates one container per parallel worker,
 * shared across all tests assigned to that worker. This avoids the ~30s
 * container startup per test.
 *
 * `runtime` — the detected container runtime (docker or podman).
 */
export const test = base.extend<
  { cleanupTerminals: void },
  { testProject: TestProjectInfo; runtime: ApiRuntime }
>({
  runtime: [async ({}, use) => {
    const runtime = await detectRuntime()
    await use(runtime)
  }, { scope: 'worker' }],

  testProject: [async ({ runtime }, use) => {
    const name = generateProjectName()
    const agentType = process.env.WARDEN_AGENT_TYPE ?? 'claude-code'
    let projectId: string | undefined

    /* Each worker needs a unique workspace directory so project IDs
       (sha256 of host path) don't collide across parallel workers. */
    const workerWorkspace = path.join(TEST_WORKSPACE, name)
    mkdirSync(workerWorkspace, { recursive: true })
    if (!existsSync(path.join(workerWorkspace, '.git'))) {
      execSync('git init', { cwd: workerWorkspace, stdio: 'pipe' })
      writeFileSync(path.join(workerWorkspace, 'README.md'), '# E2E Test Workspace\n')
      execSync('git add .', { cwd: workerWorkspace, stdio: 'pipe' })
      execSync(
        'git -c user.email="e2e@warden.test" -c user.name="Warden E2E" commit -m "initial commit"',
        { cwd: workerWorkspace, stdio: 'pipe' },
      )
    }

    /* Retry container creation — when multiple workers start simultaneously,
       port allocation or resource contention can cause transient failures. */
    const maxAttempts = 3
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
      try {
        const result = await createTestProject(name, workerWorkspace, {
          skipPermissions: true,
          agentType,
        })
        projectId = result.projectId
        break
      } catch (err) {
        if (attempt === maxAttempts) throw err
        /* Stagger retries to reduce contention. */
        await new Promise((r) => setTimeout(r, attempt * 3000))
      }
    }

    try {
      /* Wait for the container to reach running state. */
      await waitForProjectState(name, 'running', 60_000)

      await use({
        id: projectId!,
        name,
        runtime,
        agentType,
      })
    } finally {
      /* Always clean up, even if the test fails. */
      if (projectId) {
        await removeTestProject(projectId, agentType)
      }
      /* Clean up worker-specific workspace. */
      if (existsSync(workerWorkspace)) {
        rmSync(workerWorkspace, { recursive: true, force: true })
      }
    }
  }, { scope: 'worker' }],

  /**
   * Auto-fixture: disconnects all terminals after each test.
   *
   * Without this, connected terminals accumulate across tests in the
   * shared container.
   */
  cleanupTerminals: [async ({ testProject }, use) => {
    await use()

    /* After the test: kill every active worktree process so the next test
       starts from a clean "stopped" state. Using kill instead of
       disconnect ensures the tmux session is fully stopped — a background
       session would cause connectTerminal to skip create-terminal.sh,
       which means no terminal_connected event fires. */
    try {
      const worktrees = await fetchWorktrees(testProject.id, testProject.agentType)
      const active = worktrees.filter((wt) =>
        wt.state === 'connected' || wt.state === 'shell' || wt.state === 'background',
      )
      if (active.length > 0) {
        await Promise.all(
          active.map((wt) => killWorktreeProcess(testProject.id, wt.id, testProject.agentType).catch(() => {})),
        )
        await Promise.all(
          active.map((wt) =>
            waitForWorktreeState(testProject.id, wt.id, 'stopped', 10_000, testProject.agentType).catch(() => {}),
          ),
        )
      }
    } catch {
      /* Best-effort cleanup — don't fail the test. */
    }
  }, { auto: true }],
})

export { expect } from '@playwright/test'

/**
 * Returns a locator for the first terminal container on the page.
 *
 * xterm.js is rendered directly in the DOM (no iframe). The terminal
 * panel renders a div with `data-testid="terminal-container"` that
 * holds the xterm.js instance.
 */
export function terminalContainer(page: Page) {
  return page.locator('[data-testid="terminal-container"]').first()
}

/**
 * Waits for a terminal to be fully rendered and interactive.
 *
 * Verifies: terminal container attached → xterm.js initialized →
 * canvas rendered → input textarea ready. This is the minimum bar
 * for "the terminal is usable."
 *
 * @param page - Playwright page.
 * @param timeoutMs - Max time to wait for terminal readiness.
 */
export async function waitForTerminalReady(
  page: Page,
  timeoutMs = 45_000,
): Promise<void> {
  const container = terminalContainer(page)

  /* 1. Terminal container exists in the DOM. */
  await container.waitFor({ state: 'attached', timeout: timeoutMs })

  /* 2. xterm.js container rendered. */
  await container.locator('.xterm').waitFor({ state: 'visible', timeout: timeoutMs })

  /* 3. Terminal rows rendered (xterm.js v6 uses DOM renderer by default). */
  await container.locator('.xterm-rows').waitFor({ state: 'attached', timeout: 10_000 })

  /* 4. Hidden textarea exists (keyboard input capture ready). */
  await container.locator('textarea.xterm-helper-textarea').waitFor({ state: 'attached', timeout: 5_000 })
}

/**
 * Asserts the terminal is fully usable — rendered, interactive, and responsive.
 *
 * Goes beyond `waitForTerminalReady` by verifying the canvas has painted
 * with non-zero dimensions and the input textarea exists. Use this for tests
 * that need to confirm the terminal isn't just "present" but actually works.
 *
 * @param page - Playwright page.
 */
export async function assertTerminalUsable(page: Page): Promise<void> {
  const container = terminalContainer(page)

  /* xterm.js container must be visible. */
  await baseExpect(container.locator('.xterm')).toBeVisible()

  /* Terminal rows must have non-zero rendered dimensions (DOM renderer). */
  const rows = container.locator('.xterm-rows')
  await baseExpect(rows).toBeAttached()
  const box = await rows.boundingBox()
  baseExpect(box).toBeTruthy()
  baseExpect(box!.width).toBeGreaterThan(10)
  baseExpect(box!.height).toBeGreaterThan(10)

  /* Textarea for input must exist (proves xterm.js initialized its input layer). */
  await baseExpect(container.locator('textarea.xterm-helper-textarea')).toBeAttached()
}

/**
 * Navigates to the project page and waits for the sidebar.
 *
 * @param page - Playwright page.
 * @param projectId - Container ID to navigate to.
 */
export async function navigateToProject(
  page: Page,
  projectId: string,
  agentType = 'claude-code',
): Promise<void> {
  await page.goto(`/projects/${projectId}/${agentType}`)
  await page.locator('[data-testid="project-sidebar"]').waitFor({ state: 'visible' })
}

/**
 * Switches the project view to canvas mode.
 *
 * The default view mode is Grid. Canvas-specific tests must call this
 * after navigating to the project page.
 *
 * @param page - Playwright page (must already be on the project page).
 */
export async function switchToCanvasMode(page: Page): Promise<void> {
  await page.locator('[role="tab"]:has-text("Canvas")').click()
  await baseExpect(page.locator('[role="tab"]:has-text("Canvas")')).toHaveAttribute('data-state', 'active')
}

/**
 * Creates a worktree via API and waits for it to appear in the sidebar.
 *
 * Ignores "already exists" errors since the container is shared across
 * tests in the same worker. The sidebar polls worktrees every 15s, so
 * the timeout must survive at least one full poll cycle.
 *
 * @param page - Playwright page (must already be on the canvas page).
 * @param projectId - Container ID.
 * @param name - Worktree name to create.
 */
export async function ensureWorktreeVisible(
  page: Page,
  projectId: string,
  name: string,
): Promise<void> {
  try {
    await createWorktree(projectId, name)
  } catch {
    /* Worktree may already exist from a prior test (shared container). */
  }
  /* Sidebar polls worktrees every 15s — wait long enough to survive a full cycle. */
  await baseExpect(page.locator(`[data-testid="worktree-row-${name}"]`)).toBeVisible({ timeout: 30_000 })
}

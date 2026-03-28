import { execSync } from 'child_process'
import { existsSync, mkdirSync, writeFileSync, rmSync } from 'fs'
import path from 'path'
import { test as base, expect } from '@playwright/test'
import { generateProjectName } from './helpers/fixtures'
import { TEST_WORKSPACE } from './global-setup'
import {
  createTestProject,
  removeTestProject,
  waitForProjectState,
} from './helpers/api'
import { selectors } from './helpers/selectors'

/**
 * Creates a unique workspace directory with a git repo inside TEST_WORKSPACE.
 * Returns the path. Each workspace produces a unique project ID.
 */
function createUniqueWorkspace(name: string): string {
  const workspace = path.join(TEST_WORKSPACE, name)
  mkdirSync(workspace, { recursive: true })
  if (!existsSync(path.join(workspace, '.git'))) {
    execSync('git init', { cwd: workspace, stdio: 'pipe' })
    writeFileSync(path.join(workspace, 'README.md'), '# E2E Test Workspace\n')
    execSync('git add .', { cwd: workspace, stdio: 'pipe' })
    execSync(
      'git -c user.email="e2e@warden.test" -c user.name="Warden E2E" commit -m "initial commit"',
      { cwd: workspace, stdio: 'pipe' },
    )
  }
  return workspace
}

/**
 * Destructive project lifecycle tests get their own container.
 *
 * These tests stop/restart/remove the project, which would break other tests
 * if sharing the worker-scoped container.
 */
const test = base.extend<{ isolatedProject: { id: string; name: string } }>({
  isolatedProject: async ({}, use) => {
    const name = generateProjectName()
    const workspace = createUniqueWorkspace(name)
    const result = await createTestProject(name, workspace, { skipPermissions: true })
    await waitForProjectState(name, 'running', 60_000)

    await use({ id: result.projectId, name })

    await removeTestProject(result.projectId)
    if (existsSync(workspace)) {
      rmSync(workspace, { recursive: true, force: true })
    }
  },
})

test.describe('Project lifecycle', () => {
  test('should stop and restart a project', async ({ page, isolatedProject }) => {
    await page.goto('/')
    const card = page.locator(selectors.projectCard(isolatedProject.name))
    await expect(card).toBeVisible()

    /* Stop the project. */
    await card.locator('[data-testid="stop-button"]').click()
    await expect(card.locator(selectors.statusBadge)).toHaveText('exited', {
      timeout: 30_000,
    })

    /* Start it again. */
    await card.locator('[data-testid="start-button"]').click()
    await expect(card.locator(selectors.statusBadge)).toHaveText('running', {
      timeout: 60_000,
    })
  })
})

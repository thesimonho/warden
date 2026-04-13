import { existsSync, rmSync } from 'node:fs'
import { test as base, expect } from '@playwright/test'
import { createTestProject, removeTestProject, waitForProjectState } from './helpers/api'
import { createUniqueWorkspace, generateProjectName } from './helpers/fixtures'
import { selectors } from './helpers/selectors'

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
    await waitForProjectState(result.name, 'running', 60_000)

    await use({ id: result.projectId, name: result.name })

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

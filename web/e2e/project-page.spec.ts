import { expect, navigateToProject, switchToCanvasMode, test } from './helpers/fixtures'
import { selectors } from './helpers/selectors'

test.describe('Project page', () => {
  test('should show sidebar with worktree list', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)

    /* Sidebar should be visible with at least the main worktree. */
    await expect(page.locator(selectors.projectSidebar)).toBeVisible()
    await expect(page.locator(selectors.worktreeRow('main'))).toBeVisible()
  })

  test('should show grid empty state when no panels are open', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)

    /* Grid is the default view mode. */
    await expect(page.locator(selectors.gridEmptyState)).toBeVisible()
    await expect(page.locator(selectors.gridEmptyState)).toContainText(
      'Select a worktree from the sidebar',
    )
  })

  test('should show canvas empty state when no panels are open', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)
    await switchToCanvasMode(page)

    await expect(page.locator(selectors.canvasEmptyState)).toBeVisible()
    await expect(page.locator(selectors.canvasEmptyState)).toContainText(
      'Select a worktree from the sidebar',
    )
  })

  test('should add a terminal panel when clicking a worktree in grid mode', async ({
    page,
    testProject,
  }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)

    /* Click the main worktree to add it as a grid cell. */
    await page.locator(selectors.worktreeRow('main')).click()

    /* Empty state should disappear and a grid cell should appear. */
    await expect(page.locator(selectors.gridEmptyState)).not.toBeVisible({ timeout: 30_000 })
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })
  })

  test('should add a terminal panel when clicking a worktree in canvas mode', async ({
    page,
    testProject,
  }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)
    await switchToCanvasMode(page)

    /* Click the main worktree to add it as a panel. */
    await page.locator(selectors.worktreeRow('main')).click()

    /* Empty state should disappear and a panel should appear. */
    await expect(page.locator(selectors.canvasEmptyState)).not.toBeVisible({ timeout: 30_000 })
    await expect(page.locator(selectors.canvasPanel('main'))).toBeVisible({ timeout: 30_000 })
  })

  test('should disconnect panel and return to empty state', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)

    /* Connect a terminal in grid mode. */
    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })

    /* Focus the cell first — the unfocused overlay blocks the title bar. */
    await page.locator(selectors.gridCell('main')).click()
    await page.locator(selectors.gridCellDisconnect('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).not.toBeVisible({ timeout: 15_000 })
    await expect(page.locator(selectors.gridEmptyState)).toBeVisible()
  })
})

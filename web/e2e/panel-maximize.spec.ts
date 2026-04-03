import { test, expect, navigateToProject, switchToCanvasMode, waitForTerminalReady, assertTerminalUsable } from './helpers/fixtures'
import { selectors } from './helpers/selectors'

test.describe('Panel maximize', () => {
  test('should maximize a panel and restore it', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)
    await switchToCanvasMode(page)

    await page.locator(selectors.worktreeRow('main')).click()
    const panel = page.locator(selectors.canvasPanel('main'))
    await expect(panel).toBeVisible({ timeout: 30_000 })

    const initialBox = await panel.boundingBox()
    expect(initialBox).toBeTruthy()

    await page.locator(selectors.canvasPanelMaximize('main')).click()
    await page.waitForTimeout(500)
    const maximizedBox = await panel.boundingBox()
    expect(maximizedBox).toBeTruthy()
    expect(maximizedBox!.width).toBeGreaterThan(initialBox!.width)
    expect(maximizedBox!.height).toBeGreaterThan(initialBox!.height)

    await page.locator(selectors.canvasPanelMaximize('main')).click()
    await page.waitForTimeout(500)
    const restoredBox = await panel.boundingBox()
    expect(restoredBox).toBeTruthy()
    expect(restoredBox!.width).toBeCloseTo(initialBox!.width, -1)
    expect(restoredBox!.height).toBeCloseTo(initialBox!.height, -1)
  })

  test('should maximize panel and terminal remains usable', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)
    await switchToCanvasMode(page)

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.canvasPanel('main'))).toBeVisible({ timeout: 30_000 })
    await waitForTerminalReady(page)
    await assertTerminalUsable(page)

    await page.locator(selectors.canvasPanelMaximize('main')).click()
    await page.waitForTimeout(500)

    await assertTerminalUsable(page)
  })

  test('should fit all panels to viewport', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)
    await switchToCanvasMode(page)

    await page.locator(selectors.worktreeRow('main')).click()
    const panel = page.locator(selectors.canvasPanel('main'))
    await expect(panel).toBeVisible({ timeout: 30_000 })

    /* Record position before fit. */
    const before = await panel.boundingBox()

    await page.locator(selectors.fitAllButton).click()
    await page.waitForTimeout(500)

    /* Panel should still be visible and position should have changed. */
    await expect(panel).toBeVisible()
    const after = await panel.boundingBox()
    expect(after).toBeTruthy()
    const moved = before!.x !== after!.x || before!.y !== after!.y
    const resized = before!.width !== after!.width || before!.height !== after!.height
    expect(moved || resized).toBeTruthy()
  })
})

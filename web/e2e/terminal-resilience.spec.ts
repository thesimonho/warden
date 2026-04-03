import {
  test,
  expect,
  navigateToProject,
  switchToCanvasMode,
  waitForTerminalReady,
  assertTerminalUsable,
  terminalContainer,
  ensureWorktreeVisible,
} from './helpers/fixtures'
import { selectors } from './helpers/selectors'

test.describe('Terminal advanced', () => {
  test('should survive navigate-away-and-back with working terminal', async ({
    page,
    testProject,
  }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })
    await waitForTerminalReady(page)
    await assertTerminalUsable(page)

    await page.goto('/')
    await expect(page.locator(selectors.projectCard(testProject.name))).toBeVisible()

    await navigateToProject(page, testProject.id, testProject.agentType)

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })
    await waitForTerminalReady(page)
    await assertTerminalUsable(page)
  })

  test('should handle rapid navigate-away-and-back', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })

    for (let i = 0; i < 3; i++) {
      await page.goto('/')
      await navigateToProject(page, testProject.id, testProject.agentType)
    }

    await expect(page.locator(selectors.worktreeRow('main'))).toBeVisible()

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })
    await waitForTerminalReady(page)
    await assertTerminalUsable(page)
  })

  test('should connect two usable terminals simultaneously', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)
    await ensureWorktreeVisible(page, testProject.id, 'e2e-multi')

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })

    await page.locator(selectors.worktreeRow('e2e-multi')).click()
    await expect(page.locator(selectors.gridCell('e2e-multi'))).toBeVisible({ timeout: 30_000 })

    const containers = page.locator(selectors.terminalContainer)
    await expect(containers).toHaveCount(2, { timeout: 30_000 })

    for (const container of await containers.all()) {
      await expect(container.locator('.xterm')).toBeVisible({ timeout: 30_000 })
      await expect(container.locator('.xterm-rows')).toBeAttached()
    }
  })

  test('should disconnect one terminal without affecting the other', async ({
    page,
    testProject,
  }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)
    await ensureWorktreeVisible(page, testProject.id, 'e2e-indep')

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })

    await page.locator(selectors.worktreeRow('e2e-indep')).click()
    await expect(page.locator(selectors.gridCell('e2e-indep'))).toBeVisible({ timeout: 30_000 })

    await expect(page.locator(selectors.terminalContainer)).toHaveCount(2, { timeout: 30_000 })

    /* Focus the cell first — the unfocused overlay blocks the title bar. */
    await page.locator(selectors.gridCell('e2e-indep')).click()
    await page.locator(selectors.gridCellDisconnect('e2e-indep')).click()
    await expect(page.locator(selectors.gridCell('e2e-indep'))).not.toBeVisible({ timeout: 15_000 })

    await expect(page.locator(selectors.gridCell('main'))).toBeVisible()
    await assertTerminalUsable(page)
  })

  test('should resize panel and xterm.js canvas stays rendered', async ({ page, testProject }) => {
    /* Panel resize via drag handles is a canvas-specific feature. */
    await navigateToProject(page, testProject.id, testProject.agentType)
    await switchToCanvasMode(page)

    await page.locator(selectors.worktreeRow('main')).click()
    const panel = page.locator(selectors.canvasPanel('main'))
    await expect(panel).toBeVisible({ timeout: 30_000 })
    await waitForTerminalReady(page)
    await assertTerminalUsable(page)

    const panelBox = await panel.boundingBox()
    expect(panelBox).toBeTruthy()

    const handleX = panelBox!.x + panelBox!.width - 3
    const handleY = panelBox!.y + panelBox!.height - 3
    await page.mouse.move(handleX, handleY)
    await page.mouse.down()
    await page.mouse.move(handleX + 100, handleY + 100, { steps: 5 })
    await page.mouse.up()

    const container = terminalContainer(page)
    const rows = container.locator('.xterm-rows')
    const rowsBox = await rows.boundingBox()
    expect(rowsBox).toBeTruthy()
    expect(rowsBox!.width).toBeGreaterThan(10)
    expect(rowsBox!.height).toBeGreaterThan(10)
  })

  test('should create a worktree and connect a usable terminal', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)

    await page.locator(selectors.newWorktreeButton).click()

    const wtName = `e2e-ui-${Date.now().toString(36)}`
    await page.locator(selectors.worktreeNameInput).fill(wtName)

    await page.locator(selectors.createWorktreeButton).click()

    await expect(page.locator(selectors.worktreeRow(wtName))).toBeVisible({ timeout: 30_000 })

    await page.locator(selectors.worktreeRow(wtName)).click()
    await expect(page.locator(selectors.gridCell(wtName))).toBeVisible({ timeout: 30_000 })
    await waitForTerminalReady(page)
    await assertTerminalUsable(page)
  })
})

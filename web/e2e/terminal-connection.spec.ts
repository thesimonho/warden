import { disconnectTerminal, waitForWorktreeState } from './helpers/api'
import {
  assertTerminalUsable,
  expect,
  navigateToProject,
  terminalContainer,
  test,
  waitForTerminalReady,
} from './helpers/fixtures'
import { selectors } from './helpers/selectors'

test.describe('Terminal connection', () => {
  test('should connect terminal with xterm.js rendering', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })

    await waitForTerminalReady(page)
    await assertTerminalUsable(page)
  })

  test('should render terminal canvas with non-zero dimensions', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })
    await waitForTerminalReady(page)

    const container = terminalContainer(page)
    const rows = container.locator('.xterm-rows')
    const box = await rows.boundingBox()
    expect(box).toBeTruthy()
    expect(box!.width).toBeGreaterThan(100)
    expect(box!.height).toBeGreaterThan(50)
  })

  test('should disconnect via panel button and reconnect with working terminal', async ({
    page,
    testProject,
  }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })
    await waitForTerminalReady(page)
    await assertTerminalUsable(page)

    /* Click the grid cell first to focus it — the unfocused overlay
       (absolute inset-0 z-10) blocks pointer events on the title bar. */
    await page.locator(selectors.gridCell('main')).click()

    /* Disconnect via the UI button — grid cell should disappear. */
    await page.locator(selectors.gridCellDisconnect('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).not.toBeVisible({ timeout: 15_000 })

    /* Wait for the sidebar to reflect a reconnectable state. */
    await expect(async () => {
      const row = page.locator(selectors.worktreeRow('main'))
      const text = await row.textContent()
      /* Worktree should be in background or shell state — anything except "Connected". */
      expect(text?.toLowerCase()).not.toContain('connected')
    }).toPass({ timeout: 30_000 })

    /* Reconnect by clicking the worktree again. */
    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })

    await waitForTerminalReady(page)
    await assertTerminalUsable(page)
  })

  test('should disconnect via API and reflect background state', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.gridCell('main'))).toBeVisible({ timeout: 30_000 })
    await waitForTerminalReady(page)

    /* Disconnect via API — the backend should transition to background.
       The grid cell stays visible (background is still "alive") but the
       sidebar should reflect the state change. */
    await disconnectTerminal(testProject.id, 'main', testProject.agentType)

    await waitForWorktreeState(
      testProject.id,
      'main',
      ['background', 'shell'],
      30_000,
      testProject.agentType,
    )
  })
})

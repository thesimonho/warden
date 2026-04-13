import type { Page } from '@playwright/test'
import {
  ensureWorktreeVisible,
  expect,
  navigateToProject,
  switchToCanvasMode,
  test,
} from './helpers/fixtures'
import { selectors } from './helpers/selectors'

/**
 * Shift+clicks on a canvas panel by coordinates.
 *
 * Uses keyboard Shift hold + mouse click at the panel's center rather than
 * locator.click({ modifiers: ['Shift'] }) because the Rnd component handles
 * mousedown events natively and locator clicks with force:true may not
 * propagate the shift modifier to the native event handler.
 */
async function shiftClickPanel(page: Page, worktreeId: string) {
  /* Click the title bar — not the terminal content. xterm.js captures
     mousedown events in the terminal area, preventing them from reaching
     the Rnd component's onMouseDown handler which checks e.shiftKey. */
  const titleBar = page.locator(selectors.canvasPanel(worktreeId)).locator('.canvas-panel-handle')
  await expect(titleBar).toBeVisible({ timeout: 15_000 })
  const box = await titleBar.boundingBox()
  if (!box) throw new Error(`Canvas panel ${worktreeId} title bar not found`)

  await page.keyboard.down('Shift')
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2)
  await page.keyboard.up('Shift')
}

/**
 * Selects two canvas panels and focuses the canvas container for keyboard shortcuts.
 */
async function selectTwoPanels(page: Page, panelA: string, panelB: string) {
  await shiftClickPanel(page, panelA)
  await shiftClickPanel(page, panelB)

  /* Move focus from the terminal textarea to the canvas container so
     keyboard shortcuts reach the onKeyDown handler. Use focus() instead of
     click() to avoid triggering the marquee selection handler which clears
     the selection. The canvas container is the parent of [data-canvas-toolbar]. */
  await page.locator('[data-canvas-toolbar]').locator('..').focus()
}

test.describe('Panel layout', () => {
  test('should open two panels and select both via shift+click', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)
    await switchToCanvasMode(page)
    await ensureWorktreeVisible(page, testProject.id, 'e2e-shift')

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.canvasPanel('main'))).toBeVisible({ timeout: 30_000 })

    await page.locator(selectors.worktreeRow('e2e-shift')).click()
    await expect(page.locator(selectors.canvasPanel('e2e-shift'))).toBeVisible({ timeout: 30_000 })

    /* Fit all panels to viewport so they're not stuck under the sidebar. */
    await page.locator(selectors.fitAllButton).click()
    await page.waitForTimeout(1000)

    await shiftClickPanel(page, 'main')
    await shiftClickPanel(page, 'e2e-shift')

    await expect(page.locator(selectors.layoutGridButton)).toBeVisible()
    await expect(page.locator(selectors.layoutHorizontalButton)).toBeVisible()
    await expect(page.locator(selectors.layoutVerticalButton)).toBeVisible()
    await expect(page.locator(selectors.fitSelectionButton)).toBeVisible()
  })

  test('should apply grid layout to selected panels', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)
    await switchToCanvasMode(page)
    await ensureWorktreeVisible(page, testProject.id, 'e2e-grid')

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.canvasPanel('main'))).toBeVisible({ timeout: 30_000 })
    await page.locator(selectors.worktreeRow('e2e-grid')).click()
    await expect(page.locator(selectors.canvasPanel('e2e-grid'))).toBeVisible({ timeout: 30_000 })

    await page.locator(selectors.fitAllButton).click()
    await page.waitForTimeout(1000)
    await selectTwoPanels(page, 'main', 'e2e-grid')

    await page.locator(selectors.layoutGridButton).click()
    await page.waitForTimeout(1000)

    /* Both panels should be visible with non-zero dimensions after grid layout. */
    const mainBox = await page.locator(selectors.canvasPanel('main')).boundingBox()
    const gridBox = await page.locator(selectors.canvasPanel('e2e-grid')).boundingBox()
    expect(mainBox).toBeTruthy()
    expect(gridBox).toBeTruthy()
    expect(mainBox!.width).toBeGreaterThan(100)
    expect(gridBox!.width).toBeGreaterThan(100)
  })

  test('should apply horizontal layout via keyboard shortcut', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)
    await switchToCanvasMode(page)
    await ensureWorktreeVisible(page, testProject.id, 'e2e-horiz')

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.canvasPanel('main'))).toBeVisible({ timeout: 30_000 })
    await page.locator(selectors.worktreeRow('e2e-horiz')).click()
    await expect(page.locator(selectors.canvasPanel('e2e-horiz'))).toBeVisible({ timeout: 30_000 })

    await page.locator(selectors.fitAllButton).click()
    await page.waitForTimeout(1000)
    await selectTwoPanels(page, 'main', 'e2e-horiz')

    await page.keyboard.press('h')
    await page.waitForTimeout(1000)

    const mainBox = await page.locator(selectors.canvasPanel('main')).boundingBox()
    const horizBox = await page.locator(selectors.canvasPanel('e2e-horiz')).boundingBox()
    expect(mainBox).toBeTruthy()
    expect(horizBox).toBeTruthy()

    expect(Math.abs(mainBox!.y - horizBox!.y)).toBeLessThan(5)
    expect(Math.abs(mainBox!.x - horizBox!.x)).toBeGreaterThan(100)
  })

  test('should apply vertical layout via keyboard shortcut', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)
    await switchToCanvasMode(page)
    await ensureWorktreeVisible(page, testProject.id, 'e2e-vert')

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.canvasPanel('main'))).toBeVisible({ timeout: 30_000 })
    await page.locator(selectors.worktreeRow('e2e-vert')).click()
    await expect(page.locator(selectors.canvasPanel('e2e-vert'))).toBeVisible({ timeout: 30_000 })

    await page.locator(selectors.fitAllButton).click()
    await page.waitForTimeout(1000)
    await selectTwoPanels(page, 'main', 'e2e-vert')

    await page.keyboard.press('v')
    await page.waitForTimeout(1000)

    const mainBox = await page.locator(selectors.canvasPanel('main')).boundingBox()
    const vertBox = await page.locator(selectors.canvasPanel('e2e-vert')).boundingBox()
    expect(mainBox).toBeTruthy()
    expect(vertBox).toBeTruthy()

    expect(Math.abs(mainBox!.x - vertBox!.x)).toBeLessThan(5)
    expect(Math.abs(mainBox!.y - vertBox!.y)).toBeGreaterThan(100)
  })

  test('should deselect panels with Escape key', async ({ page, testProject }) => {
    await navigateToProject(page, testProject.id, testProject.agentType)
    await switchToCanvasMode(page)
    await ensureWorktreeVisible(page, testProject.id, 'e2e-esc')

    await page.locator(selectors.worktreeRow('main')).click()
    await expect(page.locator(selectors.canvasPanel('main'))).toBeVisible({ timeout: 30_000 })
    await page.locator(selectors.worktreeRow('e2e-esc')).click()
    await expect(page.locator(selectors.canvasPanel('e2e-esc'))).toBeVisible({ timeout: 30_000 })

    await page.locator(selectors.fitAllButton).click()
    await page.waitForTimeout(1000)
    await selectTwoPanels(page, 'main', 'e2e-esc')
    await expect(page.locator(selectors.layoutGridButton)).toBeVisible()

    await page.keyboard.press('Escape')

    await expect(page.locator(selectors.layoutGridButton)).not.toBeVisible()
  })
})

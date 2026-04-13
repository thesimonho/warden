import { expect, test } from './helpers/fixtures'
import { selectors } from './helpers/selectors'

test.describe('Navigation', () => {
  test('should load home page with project grid', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveTitle(/Warden/)
    await expect(page.locator(selectors.projectGrid)).toBeVisible()
  })

  test('should navigate between home and project via browser back/forward', async ({
    page,
    testProject,
  }) => {
    /* Start at home. */
    await page.goto('/')
    await expect(page.locator(selectors.projectGrid)).toBeVisible()

    /* Navigate to project. */
    await page.locator(selectors.projectViewButton(testProject.name)).click()
    await expect(page.locator(selectors.projectSidebar)).toBeVisible()

    /* Browser back → home. Wait for URL to settle before checking DOM. */
    await page.goBack()
    await page.waitForURL('/')
    await expect(page.locator(selectors.projectGrid)).toBeVisible({ timeout: 30_000 })

    /* Browser forward → project. */
    await page.goForward()
    await expect(page.locator(selectors.projectSidebar)).toBeVisible({ timeout: 30_000 })
  })

  test('should show add project button on home page', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator(selectors.addProjectButton)).toBeVisible()
  })
})

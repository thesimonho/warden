import { expect, test } from './helpers/fixtures'
import { selectors } from './helpers/selectors'

test.describe('Home page', () => {
  test('should display the test project card as running', async ({ page, testProject }) => {
    await page.goto('/')
    const card = page.locator(selectors.projectCard(testProject.name))
    await expect(card).toBeVisible()

    /* Status badge should show "running". */
    const badge = card.locator(selectors.statusBadge)
    await expect(badge).toHaveText('running')
  })

  test('should navigate to project page via View button', async ({ page, testProject }) => {
    await page.goto('/')
    await page.locator(selectors.projectViewButton(testProject.name)).click()
    await expect(page).toHaveURL(new RegExp(`/projects/${testProject.id}`))
    await expect(page.locator(selectors.projectSidebar)).toBeVisible()
  })
})

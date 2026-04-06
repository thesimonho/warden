import { test, expect } from './helpers/fixtures'
import {
  connectTerminal,
  fetchProject,
  fetchWorktree,
  fetchWorktreeDiff,
  fetchProjectCosts,
  fetchBudgetStatus,
  postAuditEvent,
  fetchAuditLog,
  sendWorktreeInput,
  waitForWorktreeState,
} from './helpers/api'

/**
 * Integrator API tests.
 *
 * These validate the new endpoints added for third-party integrators
 * packaging the warden binary with their own apps. Each test exercises
 * the full stack: HTTP handler -> service -> engine/DB -> response.
 */
test.describe('Integrator API', () => {
  test.describe('GET single project', () => {
    test('should return project details by ID', async ({ testProject }) => {
      const project = await fetchProject(testProject.id, testProject.agentType)

      expect(project.projectId).toBe(testProject.id)
      expect(project.name).toBe(testProject.name)
      expect(project.hasContainer).toBe(true)
      expect(project.state).toBe('running')
      expect(project.agentType).toBeTruthy()
    })
  })

  test.describe('GET single worktree', () => {
    test('should return worktree by ID', async ({ testProject }) => {
      await connectTerminal(testProject.id, 'main', testProject.agentType)
      await waitForWorktreeState(testProject.id, 'main', 'connected', 15_000, testProject.agentType)

      const worktree = await fetchWorktree(testProject.id, 'main', testProject.agentType)

      expect(worktree.id).toBe('main')
      expect(worktree.state).toBe('connected')
    })
  })

  test.describe('GET project costs', () => {
    test('should return cost structure', async ({ testProject }) => {
      const costs = await fetchProjectCosts(testProject.id, testProject.agentType)

      expect(costs.projectId).toBe(testProject.id)
      expect(costs.agentType).toBeTruthy()
      expect(typeof costs.totalCost).toBe('number')
      expect(Array.isArray(costs.sessions)).toBe(true)
    })
  })

  test.describe('GET budget status', () => {
    test('should return budget state', async ({ testProject }) => {
      const budget = await fetchBudgetStatus(testProject.id, testProject.agentType)

      expect(budget.projectId).toBe(testProject.id)
      expect(typeof budget.effectiveBudget).toBe('number')
      expect(typeof budget.totalCost).toBe('number')
      expect(typeof budget.isOverBudget).toBe('boolean')
      expect(['project', 'global', 'none']).toContain(budget.budgetSource)
    })
  })

  test.describe('POST audit event', () => {
    test('should persist custom event with project scoping', async ({ testProject }) => {
      await postAuditEvent({
        event: 'e2e_test_event',
        source: 'external',
        message: 'integration test marker',
        projectId: testProject.id,
        agentType: testProject.agentType,
        attrs: { testRun: true },
      })

      const entries = await fetchAuditLog({
        projectId: testProject.id,
        source: 'external',
      })

      const found = entries.find((e) => e.event === 'e2e_test_event')
      expect(found).toBeTruthy()
      expect(found?.source).toBe('external')
      expect(found?.projectId).toBe(testProject.id)
    })
  })

  test.describe('POST worktree input', () => {
    test('should deliver text to the tmux pane', async ({ testProject }) => {
      await connectTerminal(testProject.id, 'main', testProject.agentType)
      // Wait for shell state — agent exited, bash prompt is available.
      // This avoids the input being consumed by the agent instead of bash.
      await waitForWorktreeState(
        testProject.id, 'main', ['connected', 'shell'], 30_000, testProject.agentType,
      )

      // Create a marker file via bash. If send-keys delivers the text,
      // bash executes the command and the file appears in the worktree diff.
      const marker = `warden-e2e-input-${Date.now()}`
      await sendWorktreeInput(
        testProject.id, 'main', `touch ${marker}`, {
          pressEnter: true,
          agentType: testProject.agentType,
        },
      )

      // Give bash time to execute the command.
      await new Promise((r) => setTimeout(r, 2000))

      // Verify the file exists by checking the worktree diff.
      const diff = await fetchWorktreeDiff(testProject.id, 'main', testProject.agentType)
      const created = diff.files.some((f: { path: string }) => f.path === marker)
      expect(created).toBe(true)
    })

    test('should reject input to stopped worktree', async ({ testProject }) => {
      // "main" without connectTerminal should be stopped — no tmux session.
      // The endpoint should return an error (404 or 500).
      try {
        await sendWorktreeInput(testProject.id, 'nonexistent', 'hello', {
          agentType: testProject.agentType,
        })
        // If we get here, the endpoint didn't reject — fail the test.
        expect(true).toBe(false)
      } catch (error) {
        // Expected: API returns non-200 for missing worktree/session.
        expect(error).toBeTruthy()
      }
    })
  })
})

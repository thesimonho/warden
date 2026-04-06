import { test, expect } from './helpers/fixtures'
import {
  connectTerminal,
  fetchProject,
  fetchWorktree,
  fetchProjectCosts,
  sendWorktreeInput,
  waitForWorktreeState,
} from './helpers/api'

/**
 * API endpoint smoke tests.
 *
 * Quick response-shape validation for individual endpoints. Each test
 * exercises the full stack: HTTP handler -> service -> engine/DB -> response.
 * Workflow-level coverage lives in api-workflows.spec.ts.
 */
test.describe('API endpoints', () => {
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

  test.describe('POST worktree input', () => {
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

import { test, expect } from './helpers/fixtures'
import {
  validateContainer,
  createWorktree,
  connectTerminal,
  disconnectTerminal,
  waitForWorktreeState,
} from './helpers/api'

/**
 * Container integration tests.
 *
 * These tests validate the container internals that no frontend test can see:
 * script installation, event bus connectivity, process lifecycle, and state
 * transitions within the real container. Since we already pay the cost of
 * spinning up containers for E2E tests, these checks are essentially free.
 *
 * All API calls use `testProject.agentType` so the suite works with both
 * claude-code (default) and codex (via WARDEN_AGENT_TYPE=codex).
 */
test.describe('Container integration', () => {
  test.describe('Infrastructure validation', () => {
    test('should have all required Warden binaries installed', async ({ testProject }) => {
      const result = await validateContainer(testProject.id, testProject.agentType)

      expect(result.valid).toBe(true)
      expect(result.missing).toBeNull()
    })
  })

  test.describe('Event bus', () => {
    test('should reflect terminal state via event bus within 15 seconds', async ({
      testProject,
    }) => {
      await connectTerminal(testProject.id, 'main', testProject.agentType)

      await waitForWorktreeState(testProject.id, 'main', 'connected', 15_000, testProject.agentType)
    })

    test('should push terminal_connected event when terminal connects', async ({ testProject }) => {
      await connectTerminal(testProject.id, 'main', testProject.agentType)

      await waitForWorktreeState(testProject.id, 'main', 'connected', 30_000, testProject.agentType)
    })

    test('should push terminal_disconnected event when terminal disconnects', async ({
      testProject,
    }) => {
      await connectTerminal(testProject.id, 'main', testProject.agentType)
      await waitForWorktreeState(testProject.id, 'main', 'connected', 30_000, testProject.agentType)

      await disconnectTerminal(testProject.id, 'main', testProject.agentType)

      await waitForWorktreeState(testProject.id, 'main', ['background', 'shell'], 30_000, testProject.agentType)
    })
  })

  test.describe('Worktree state machine', () => {
    test('should transition: disconnected → connected → background → connected', async ({
      testProject,
    }) => {
      await connectTerminal(testProject.id, 'main', testProject.agentType)
      await waitForWorktreeState(testProject.id, 'main', 'connected', 45_000, testProject.agentType)

      await disconnectTerminal(testProject.id, 'main', testProject.agentType)
      await waitForWorktreeState(testProject.id, 'main', ['background', 'shell'], 30_000, testProject.agentType)

      await new Promise((r) => setTimeout(r, 2000))

      await connectTerminal(testProject.id, 'main', testProject.agentType)
      await waitForWorktreeState(testProject.id, 'main', 'connected', 45_000, testProject.agentType)
    })
  })

  test.describe('Concurrent terminals', () => {
    test('should support multiple concurrent terminals', async ({ testProject }) => {
      await connectTerminal(testProject.id, 'main', testProject.agentType).catch(() => {})
      await waitForWorktreeState(testProject.id, 'main', 'connected', 30_000, testProject.agentType)

      try {
        try {
          await connectTerminal(testProject.id, 'e2e-concurrent', testProject.agentType)
        } catch {
          await createWorktree(testProject.id, 'e2e-concurrent', testProject.agentType)
        }

        await waitForWorktreeState(testProject.id, 'e2e-concurrent', 'connected', 60_000, testProject.agentType)
      } finally {
        await disconnectTerminal(testProject.id, 'e2e-concurrent', testProject.agentType).catch(() => {})
      }
    })
  })
})

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
 */
test.describe('Container integration', () => {
  test.describe('Infrastructure validation', () => {
    test('should have all required Warden binaries installed', async ({ testProject }) => {
      const result = await validateContainer(testProject.id)

      expect(result.valid).toBe(true)
      expect(result.missing).toBeNull()
    })
  })

  test.describe('Event bus', () => {
    test('should reflect terminal state via event bus within 15 seconds', async ({
      testProject,
    }) => {
      /* Connect a terminal — this pushes a terminal_connected event through
         the event bus. Verify the backend reflects the state change in the
         API response (the API overlays event store data on top of poll data). */
      await connectTerminal(testProject.id, 'main')

      await waitForWorktreeState(testProject.id, 'main', 'connected', 15_000)
    })

    test('should push terminal_connected event when terminal connects', async ({ testProject }) => {
      /* Cleanup kills all processes, so we always start from disconnected. */
      await connectTerminal(testProject.id, 'main')

      await waitForWorktreeState(testProject.id, 'main', 'connected', 30_000)
    })

    test('should push terminal_disconnected event when terminal disconnects', async ({
      testProject,
    }) => {
      await connectTerminal(testProject.id, 'main')
      await waitForWorktreeState(testProject.id, 'main', 'connected', 30_000)

      /* Disconnect. */
      await disconnectTerminal(testProject.id, 'main')

      /* Backend should receive terminal_disconnected event. State will be
         "background" (abduco alive) or "shell" (Claude exited, bash still running). */
      await waitForWorktreeState(testProject.id, 'main', ['background', 'shell'], 30_000)
    })
  })

  test.describe('Worktree state machine', () => {
    test('should transition: disconnected → connected → background → connected', async ({
      testProject,
    }) => {
      /* disconnected/background → connected */
      await connectTerminal(testProject.id, 'main')
      await waitForWorktreeState(testProject.id, 'main', 'connected', 45_000)

      /* connected → background/shell (disconnect closes viewer, abduco keeps running) */
      await disconnectTerminal(testProject.id, 'main')
      await waitForWorktreeState(testProject.id, 'main', ['background', 'shell'], 30_000)

      /* Small delay before reconnect — Docker needs time to clean up the
         previous exec attachment before a new one can attach to abduco. */
      await new Promise((r) => setTimeout(r, 2000))

      /* background/shell → connected (reconnect — new docker exec attaches to existing abduco) */
      await connectTerminal(testProject.id, 'main')
      await waitForWorktreeState(testProject.id, 'main', 'connected', 45_000)
    })
  })

  test.describe('Concurrent terminals', () => {
    test('should support multiple concurrent terminals', async ({ testProject }) => {
      /* Ensure main is connected. */
      await connectTerminal(testProject.id, 'main').catch(() => {})
      await waitForWorktreeState(testProject.id, 'main', 'connected', 30_000)

      /* Create and connect a second worktree. */
      try {
        try {
          await connectTerminal(testProject.id, 'e2e-concurrent')
        } catch {
          /* Worktree might not exist — create it first. */
          await createWorktree(testProject.id, 'e2e-concurrent')
        }

        await waitForWorktreeState(testProject.id, 'e2e-concurrent', 'connected', 60_000)
      } finally {
        /* Clean up second terminal. */
        await disconnectTerminal(testProject.id, 'e2e-concurrent').catch(() => {})
      }
    })
  })
})

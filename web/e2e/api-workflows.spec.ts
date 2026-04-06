import { isolatedTest, expect } from './helpers/fixtures'
import {
  collectSSEEvents,
  connectTerminal,
  createWorktree,
  deleteAccessItem,
  deleteContainer,
  deleteProject,
  disconnectTerminal,
  fetchAuditExport,
  fetchAuditLog,
  fetchBudgetStatus,
  fetchContainerConfig,
  fetchProject,
  fetchProjectCosts,
  fetchSettings,
  fetchWorktree,
  fetchWorktreeDiff,
  getAccessItem,
  createAccessItem,
  killWorktreeProcess,
  listAccessItems,
  postAuditEvent,
  removeWorktree,
  resetCosts,
  sendWorktreeInput,
  sleep,
  stopProject,
  restartProject,
  updateAccessItem,
  updateContainer,
  updateSettings,
  waitForProjectState,
  waitForWorktreeState,
} from './helpers/api'

/**
 * Integrator API flow tests.
 *
 * Each test exercises a complete multi-step workflow that a third-party
 * integrator would perform using only the HTTP API. No Playwright page
 * interactions — pure API calls.
 */
const test = isolatedTest

test.describe('Integrator API flows', () => {
  // --- Full project lifecycle (headless) ---

  test.describe('Full project lifecycle', () => {
    test('should manage a project from creation to deletion via API', async ({
      isolatedProject: { id, name, agentType },
    }) => {
      // Verify project is running.
      const project = await fetchProject(id, agentType)
      expect(project.state).toBe('running')
      expect(project.hasContainer).toBe(true)

      // Connect terminal and wait for shell (agent exited, bash prompt available).
      await connectTerminal(id, 'main', agentType)
      await waitForWorktreeState(id, 'main', ['connected', 'shell'], 30_000, agentType)

      // Send a command and verify execution via diff.
      const marker = `lifecycle-marker-${Date.now()}`
      await sendWorktreeInput(id, 'main', `touch ${marker}`, {
        pressEnter: true,
        agentType,
      })
      await sleep(2000)

      const diff = await fetchWorktreeDiff(id, 'main', agentType)
      expect(diff.files.some((f) => f.path === marker)).toBe(true)

      // Kill worktree and verify stopped.
      await killWorktreeProcess(id, 'main', agentType)
      await waitForWorktreeState(id, 'main', 'stopped', 15_000, agentType)

      // Stop container.
      await stopProject(id, agentType)
      await waitForProjectState(name, 'exited', 30_000)

      // Restart container.
      await restartProject(id, agentType)
      await waitForProjectState(name, 'running', 60_000)

      // Delete container, then project registration.
      await deleteContainer(id, agentType)
      await deleteProject(id, agentType)

      // Verify project is gone (404).
      try {
        await fetchProject(id, agentType)
        throw new Error('fetchProject should have thrown for deleted project')
      } catch (error) {
        expect((error as Error).message).toMatch(/404|not found/i)
      }
    })
  })

  // --- Multi-worktree isolation ---

  test.describe('Multi-worktree isolation', () => {
    test('should manage independent worktree lifecycles', async ({
      testProject: { id, agentType },
    }) => {
      const wtA = `e2e-flow2-a-${Date.now().toString(36)}`
      const wtB = `e2e-flow2-b-${Date.now().toString(36)}`

      // Create two worktrees (each starts its own agent session).
      await createWorktree(id, wtA, agentType)
      await createWorktree(id, wtB, agentType)

      // Wait for both to be running.
      await Promise.all([
        waitForWorktreeState(id, wtA, ['connected', 'shell'], 60_000, agentType),
        waitForWorktreeState(id, wtB, ['connected', 'shell'], 60_000, agentType),
      ])

      // Verify both worktrees are visible and have independent state.
      const [stateA, stateB] = await Promise.all([
        fetchWorktree(id, wtA, agentType),
        fetchWorktree(id, wtB, agentType),
      ])
      expect(['connected', 'shell']).toContain(stateA.state)
      expect(['connected', 'shell']).toContain(stateB.state)

      // Kill worktree A — worktree B should be completely unaffected.
      await killWorktreeProcess(id, wtA, agentType)
      await waitForWorktreeState(id, wtA, 'stopped', 15_000, agentType)

      const wtBAfterKill = await fetchWorktree(id, wtB, agentType)
      expect(['connected', 'shell', 'background']).toContain(wtBAfterKill.state)

      // Disconnect B, verify it goes to background (not killed with A).
      await disconnectTerminal(id, wtB, agentType)
      await waitForWorktreeState(id, wtB, ['background', 'shell'], 15_000, agentType)

      // Reconnect B — should work independently of A's stopped state.
      await connectTerminal(id, wtB, agentType)
      await waitForWorktreeState(id, wtB, ['connected', 'shell'], 30_000, agentType)

      // Verify A is still stopped throughout.
      const wtAFinal = await fetchWorktree(id, wtA, agentType)
      expect(wtAFinal.state).toBe('stopped')

      // Cleanup: kill and remove both worktrees.
      await killWorktreeProcess(id, wtB, agentType).catch(() => {})
      await Promise.all([
        removeWorktree(id, wtA, agentType).catch(() => {}),
        removeWorktree(id, wtB, agentType).catch(() => {}),
      ])
    })
  })

  // --- SSE-driven state machine ---

  test.describe('SSE-driven state machine', () => {
    test('should deliver state transitions in order via SSE', async ({
      testProject: { id, agentType },
    }) => {
      // Start collecting SSE events filtered to our project.
      const eventsPromise = collectSSEEvents({
        projectId: id,
        agentType,
        timeoutMs: 45_000,
      })
      await sleep(500)

      // Drive state transitions: connect → disconnect → kill → reconnect.
      await connectTerminal(id, 'main', agentType)
      await waitForWorktreeState(id, 'main', 'connected', 15_000, agentType)

      await disconnectTerminal(id, 'main', agentType)
      await waitForWorktreeState(id, 'main', ['background', 'shell'], 15_000, agentType)

      await killWorktreeProcess(id, 'main', agentType)
      await waitForWorktreeState(id, 'main', 'stopped', 15_000, agentType)

      // Brief pause before reconnecting to ensure stopped event is flushed.
      await sleep(1000)

      await connectTerminal(id, 'main', agentType)
      await waitForWorktreeState(id, 'main', 'connected', 15_000, agentType)

      // Let final events flush before collecting.
      await sleep(2000)

      const events = await eventsPromise

      // Extract worktree state values from SSE events.
      const worktreeStates = events
        .filter((e) => e.event === 'worktree_state')
        .map((e) => e.data.state as string)

      // Verify the expected state subsequence appears in order.
      // Full sequence is connected → background → stopped → connected, but we
      // only check the key transitions (background is an intermediate state).
      const expectedSubsequence = ['connected', 'stopped', 'connected']
      let cursor = 0
      for (const state of worktreeStates) {
        if (cursor < expectedSubsequence.length && state === expectedSubsequence[cursor]) {
          cursor++
        }
      }
      expect(cursor).toBe(expectedSubsequence.length)
    })
  })

  // --- Audit trail for compliance ---

  test.describe('Audit trail for compliance', () => {
    test('should persist, query, and export custom audit events', async ({
      testProject: { id, agentType },
    }) => {
      // Capture start time with 1s buffer to avoid timestamp races.
      const since = new Date(Date.now() - 1000).toISOString()

      // Post three distinct audit events sequentially to guarantee ordering.
      const eventNames = ['flow4_event_1', 'flow4_event_2', 'flow4_event_3']
      for (let i = 0; i < eventNames.length; i++) {
        await postAuditEvent({
          event: eventNames[i],
          source: 'external',
          projectId: id,
          agentType,
          message: `compliance marker ${i + 1}`,
          attrs: { seq: i + 1, testRun: true },
        })
      }

      const until = new Date(Date.now() + 1000).toISOString()

      // Query with project filter — all 3 should be present.
      const byProject = await fetchAuditLog({ projectId: id, source: 'external' })
      for (const name of eventNames) {
        expect(byProject.some((e) => e.event === name)).toBe(true)
      }

      // Export as JSONL and verify our events appear.
      const exportBody = await fetchAuditExport({
        format: 'json',
        projectId: id,
        since,
        until,
      })

      const exportLines = exportBody
        .trim()
        .split('\n')
        .filter(Boolean)
        .map((line) => JSON.parse(line) as { event: string; ts: string })

      const exportedNames = exportLines.map((e) => e.event)
      for (const name of eventNames) {
        expect(exportedNames).toContain(name)
      }

      // Verify all three of our events are present in the export.
      const ourExported = exportLines.filter((e) =>
        eventNames.includes(e.event),
      )
      expect(ourExported.length).toBe(eventNames.length)
    })
  })

  // --- Container config update cycle ---

  test.describe('Container config update', () => {
    test('should update container config and verify persistence', async ({
      isolatedProject: { id, agentType },
    }) => {
      // Get current config.
      const config = await fetchContainerConfig(id, agentType)
      expect(config.name).toBeTruthy()
      expect(config.image).toBeTruthy()
      expect(config.projectPath).toBeTruthy()

      // Update with a new budget (lightweight, no container recreation).
      const newBudget = 25.0
      await updateContainer(id, agentType, {
        ...config,
        costBudget: newBudget,
      })

      // Verify the change persisted.
      const updated = await fetchContainerConfig(id, agentType)
      expect(updated.costBudget).toBe(newBudget)

      // Verify the container is still running.
      const project = await fetchProject(id, agentType)
      expect(project.state).toBe('running')
    })
  })

  // --- Budget guardrails ---

  test.describe('Budget guardrails', () => {
    test('should enforce per-project and global budget sources', async ({
      isolatedProject: { id, agentType },
    }) => {
      // Save original settings for cleanup.
      const originalSettings = await fetchSettings()
      const originalBudget = originalSettings.defaultProjectBudget

      try {
        // Set a per-project budget via container config.
        const config = await fetchContainerConfig(id, agentType)
        await updateContainer(id, agentType, {
          ...config,
          costBudget: 50.0,
        })

        // Verify budget status shows per-project source.
        const budgetWithProject = await fetchBudgetStatus(id, agentType)
        expect(budgetWithProject.effectiveBudget).toBe(50.0)
        expect(budgetWithProject.budgetSource).toBe('project')
        expect(budgetWithProject.isOverBudget).toBe(false)

        // Set a global budget via settings.
        await updateSettings({ defaultProjectBudget: 100.0 })

        // Remove per-project budget (0 = use global default).
        const configNow = await fetchContainerConfig(id, agentType)
        await updateContainer(id, agentType, {
          ...configNow,
          costBudget: 0,
        })

        // Verify budget falls back to global.
        const budgetWithGlobal = await fetchBudgetStatus(id, agentType)
        expect(budgetWithGlobal.effectiveBudget).toBe(100.0)
        expect(budgetWithGlobal.budgetSource).toBe('global')

        // Reset costs and verify.
        await resetCosts(id, agentType)
        const costs = await fetchProjectCosts(id, agentType)
        expect(costs.totalCost).toBe(0)
      } finally {
        // Restore original global budget.
        await updateSettings({ defaultProjectBudget: originalBudget })
      }
    })
  })

  // --- Access item CRUD ---

  test.describe('Access item CRUD', () => {
    test('should create, read, update, and delete a custom access item', async () => {
      // List built-in items — git and ssh should exist.
      const { items: builtInItems } = await listAccessItems()
      expect(builtInItems.some((item) => item.id === 'git')).toBe(true)
      expect(builtInItems.some((item) => item.id === 'ssh')).toBe(true)

      // Create a custom access item.
      const created = await createAccessItem({
        label: 'E2E Test Credential',
        description: 'Test access item for flow tests',
        credentials: [
          {
            label: 'Test Token',
            sources: [{ type: 'env', value: 'E2E_TEST_TOKEN' }],
            injections: [{ type: 'env', key: 'TEST_TOKEN_INJECTED' }],
          },
        ],
      })

      expect(created.id).toBeTruthy()
      expect(created.label).toBe('E2E Test Credential')
      expect(created.builtIn).toBe(false)

      try {
        // Read it back.
        const fetched = await getAccessItem(created.id)
        expect(fetched.id).toBe(created.id)
        expect(fetched.label).toBe('E2E Test Credential')
        expect(fetched.description).toBe('Test access item for flow tests')

        // Update the label.
        const updated = await updateAccessItem(created.id, {
          label: 'E2E Updated Credential',
        })
        expect(updated.label).toBe('E2E Updated Credential')

        // Verify update persisted.
        const refetched = await getAccessItem(created.id)
        expect(refetched.label).toBe('E2E Updated Credential')
      } finally {
        // Delete the item.
        await deleteAccessItem(created.id)
      }

      // Verify it's gone (404).
      try {
        await getAccessItem(created.id)
        throw new Error('getAccessItem should have thrown for deleted item')
      } catch (error) {
        expect((error as Error).message).toMatch(/404|not found/i)
      }
    })
  })
})

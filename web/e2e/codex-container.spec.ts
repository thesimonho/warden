import { existsSync, rmSync } from 'node:fs'
import {
  type ApiRuntime,
  connectTerminal,
  createTestProjectWithRetry,
  disconnectTerminal,
  fetchDockerStatus,
  fetchProjects,
  fetchWorktrees,
  killWorktreeProcess,
  removeTestProject,
  validateContainer,
  waitForProjectState,
  waitForWorktreeState,
} from './helpers/api'
import {
  test as base,
  createUniqueWorkspace,
  expect,
  generateProjectName,
  type TestProjectInfo,
} from './helpers/fixtures'

/**
 * Codex container integration tests.
 *
 * Validates that Warden correctly creates and manages containers with
 * agentType: 'codex'. These tests do NOT require an OpenAI API key —
 * they verify container infrastructure, not Codex CLI execution.
 */

/** Extended test with a worker-scoped Codex project fixture. */
const codexTest = base.extend<
  { cleanupCodexTerminals: void },
  { codexProject: TestProjectInfo; codexRuntime: ApiRuntime }
>({
  codexRuntime: [
    async ({}, use) => {
      const info = await fetchDockerStatus()
      if (!info.available) throw new Error('No container runtime available.')
      await use(info)
    },
    { scope: 'worker' },
  ],

  codexProject: [
    async ({ codexRuntime }, use) => {
      const name = generateProjectName()
      const workerWorkspace = createUniqueWorkspace(`${name}-codex`)

      const { projectId, serverName } = await createTestProjectWithRetry(name, workerWorkspace, {
        agentType: 'codex',
        skipPermissions: true,
      })

      try {
        await waitForProjectState(serverName, 'running', 60_000)
        await use({ id: projectId, name: serverName, runtime: codexRuntime, agentType: 'codex' })
      } finally {
        await removeTestProject(projectId, 'codex')
        if (existsSync(workerWorkspace)) rmSync(workerWorkspace, { recursive: true, force: true })
      }
    },
    { scope: 'worker' },
  ],

  cleanupCodexTerminals: [
    async ({ codexProject }, use) => {
      await use()

      try {
        const worktrees = await fetchWorktrees(codexProject.id, 'codex')
        const active = worktrees.filter(
          (wt) => wt.state === 'connected' || wt.state === 'shell' || wt.state === 'background',
        )
        if (active.length > 0) {
          await Promise.all(
            active.map((wt) =>
              killWorktreeProcess(codexProject.id, wt.id, 'codex').catch(() => {}),
            ),
          )
          await Promise.all(
            active.map((wt) =>
              waitForWorktreeState(codexProject.id, wt.id, 'stopped', 10_000, 'codex').catch(
                () => {},
              ),
            ),
          )
        }
      } catch {
        /* Best-effort cleanup. */
      }
    },
    { auto: true },
  ],
})

codexTest.describe('Codex container integration', () => {
  codexTest('should create a container with codex agent type', async ({ codexProject }) => {
    const projects = await fetchProjects()
    const project = projects.find(
      (p) => p.projectId === codexProject.id && p.agentType === 'codex' && p.hasContainer,
    )

    expect(project).toBeTruthy()
    expect(project!.state).toBe('running')
  })

  codexTest('should have all required Warden binaries installed', async ({ codexProject }) => {
    const result = await validateContainer(codexProject.id, 'codex')

    expect(result.valid).toBe(true)
    expect(result.missing).toBeNull()
  })

  codexTest('should support terminal connection', async ({ codexProject }) => {
    await connectTerminal(codexProject.id, 'main', 'codex')
    await waitForWorktreeState(codexProject.id, 'main', 'connected', 30_000, 'codex')
  })

  codexTest('should reflect terminal state transitions via event bus', async ({ codexProject }) => {
    await connectTerminal(codexProject.id, 'main', 'codex')
    await waitForWorktreeState(codexProject.id, 'main', 'connected', 30_000, 'codex')

    await disconnectTerminal(codexProject.id, 'main', 'codex')
    await waitForWorktreeState(codexProject.id, 'main', ['background', 'shell'], 30_000, 'codex')
  })
})

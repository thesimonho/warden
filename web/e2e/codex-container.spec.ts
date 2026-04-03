import { test as base, expect } from './helpers/fixtures'
import {
  type TestProjectInfo,
  generateProjectName,
} from './helpers/fixtures'
import {
  validateContainer,
  connectTerminal,
  disconnectTerminal,
  killWorktreeProcess,
  waitForWorktreeState,
  waitForProjectState,
  createTestProject,
  removeTestProject,
  fetchProjects,
  fetchWorktrees,
  fetchRuntimes,
  type ApiRuntime,
} from './helpers/api'
import { existsSync, mkdirSync, writeFileSync, rmSync } from 'fs'
import { execSync } from 'child_process'
import path from 'path'
import { TEST_WORKSPACE } from './global-setup'

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
  codexRuntime: [async ({}, use) => {
    const runtimes = await fetchRuntimes()
    const active = runtimes.find((r) => r.available)
    if (!active) throw new Error('No container runtime available.')
    await use(active)
  }, { scope: 'worker' }],

  codexProject: [async ({ codexRuntime }, use) => {
    const name = generateProjectName()
    let projectId: string | undefined

    const workerWorkspace = path.join(TEST_WORKSPACE, `${name}-codex`)
    mkdirSync(workerWorkspace, { recursive: true })
    if (!existsSync(path.join(workerWorkspace, '.git'))) {
      execSync('git init', { cwd: workerWorkspace, stdio: 'pipe' })
      writeFileSync(path.join(workerWorkspace, 'README.md'), '# Codex E2E Test\n')
      execSync('git add .', { cwd: workerWorkspace, stdio: 'pipe' })
      execSync(
        'git -c user.email="e2e@warden.test" -c user.name="Warden E2E" commit -m "initial commit"',
        { cwd: workerWorkspace, stdio: 'pipe' },
      )
    }

    const maxAttempts = 3
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
      try {
        const result = await createTestProject(name, workerWorkspace, {
          agentType: 'codex',
          skipPermissions: true,
        })
        projectId = result.projectId
        break
      } catch (err) {
        if (attempt === maxAttempts) throw err
        await new Promise((r) => setTimeout(r, attempt * 3000))
      }
    }

    try {
      await waitForProjectState(name, 'running', 60_000)
      await use({ id: projectId!, name, runtime: codexRuntime })
    } finally {
      if (projectId) await removeTestProject(projectId, 'codex')
      if (existsSync(workerWorkspace)) rmSync(workerWorkspace, { recursive: true, force: true })
    }
  }, { scope: 'worker' }],

  cleanupCodexTerminals: [async ({ codexProject }, use) => {
    await use()

    try {
      const worktrees = await fetchWorktrees(codexProject.id, 'codex')
      const active = worktrees.filter((wt) =>
        wt.state === 'connected' || wt.state === 'shell' || wt.state === 'background',
      )
      if (active.length > 0) {
        await Promise.all(
          active.map((wt) => killWorktreeProcess(codexProject.id, wt.id, 'codex').catch(() => {})),
        )
        await Promise.all(
          active.map((wt) =>
            waitForWorktreeState(codexProject.id, wt.id, 'disconnected', 10_000, 'codex').catch(() => {}),
          ),
        )
      }
    } catch {
      /* Best-effort cleanup. */
    }
  }, { auto: true }],
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
    await waitForWorktreeState(
      codexProject.id,
      'main',
      ['background', 'shell'],
      30_000,
      'codex',
    )
  })
})

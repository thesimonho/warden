import { execSync } from 'node:child_process'
import { existsSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { test as base, expect } from '@playwright/test'
import { TEST_WORKSPACE } from './global-setup'
import { createTestProject, removeTestProject, waitForProjectState } from './helpers/api'
import { generateProjectName } from './helpers/fixtures'

/**
 * Access item integration tests.
 *
 * These tests verify that containers with SSH and Git access items enabled
 * start successfully on Docker. The entrypoint must handle
 * bind-mount directory ownership correctly.
 */
base.describe('Access items', () => {
  let projectId: string | undefined
  let workerWorkspace: string | undefined

  base.afterEach(async () => {
    if (projectId) {
      await removeTestProject(projectId).catch(() => {})
      projectId = undefined
    }
    if (workerWorkspace && existsSync(workerWorkspace)) {
      rmSync(workerWorkspace, { recursive: true, force: true })
      workerWorkspace = undefined
    }
  })

  base('should start container with SSH and Git access items enabled', async () => {
    const name = generateProjectName()
    workerWorkspace = path.join(TEST_WORKSPACE, name)
    mkdirSync(workerWorkspace, { recursive: true })
    if (!existsSync(path.join(workerWorkspace, '.git'))) {
      execSync('git init', { cwd: workerWorkspace, stdio: 'pipe' })
      writeFileSync(path.join(workerWorkspace, 'README.md'), '# Access item test\n')
      execSync('git add .', { cwd: workerWorkspace, stdio: 'pipe' })
      execSync(
        'git -c user.email="e2e@warden.test" -c user.name="Warden E2E" commit -m "initial commit"',
        { cwd: workerWorkspace, stdio: 'pipe' },
      )
    }

    const result = await createTestProject(name, workerWorkspace, {
      skipPermissions: true,
      enabledAccessItems: ['git', 'ssh'],
    })
    projectId = result.projectId

    /* The container must reach running state, which means the entrypoint
       completed without crashing. A permission error in .ssh directory
       handling would cause a restart loop and the state would never
       reach "running" within the timeout.
       Use result.name which includes the mode suffix (e.g. "-dev"). */
    const project = await waitForProjectState(result.name, 'running', 60_000)
    expect(project.state).toBe('running')
  })
})

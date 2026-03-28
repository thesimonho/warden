import { rmSync, existsSync } from 'fs'
import { TEST_WORKSPACE } from './global-setup'
import { fetchProjects, removeTestProject } from './helpers/api'

/**
 * Runs once after all test files.
 *
 * Cleans up the test workspace and removes any leaked test containers.
 */
/** E2E database directory — matches the path in playwright.config.ts webServer. */
const E2E_DB_DIR = '/tmp/warden-e2e-db'

export default async function globalTeardown() {
  /* Clean up test workspace. */
  if (existsSync(TEST_WORKSPACE)) {
    rmSync(TEST_WORKSPACE, { recursive: true, force: true })
  }

  /* Clean up isolated E2E database. */
  if (existsSync(E2E_DB_DIR)) {
    rmSync(E2E_DB_DIR, { recursive: true, force: true })
  }

  /* Auto-clean leaked test containers. */
  try {
    const projects = await fetchProjects()
    const leaked = projects.filter((p) => p.name.startsWith('warden-e2e-'))

    await Promise.all(leaked.map(async (project) => {
      console.warn(`[E2E] Cleaning up leaked container: ${project.name}`)
      await removeTestProject(project.projectId)
    }))

    if (leaked.length > 0) {
      console.warn(`[E2E] Cleaned up ${leaked.length} leaked container(s)`)
    }
  } catch {
    /* Server may already be stopped — ignore. */
  }
}

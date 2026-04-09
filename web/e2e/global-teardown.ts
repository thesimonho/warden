import { execSync } from 'child_process'
import { rmSync, existsSync } from 'fs'
import { E2E_CACHE_DIR, TEST_WORKSPACE } from './global-setup'
import { fetchProjects, removeTestProject } from './helpers/api'

/**
 * Runs once after all test files.
 *
 * Cleans up the test workspace and removes any leaked test containers.
 * Uses a two-layer approach: API cleanup first, then CLI fallback for
 * containers the API missed (e.g. server crashed during the run).
 */

/** E2E database directory — matches the path in playwright.config.ts webServer. */
const E2E_DB_DIR = `${E2E_CACHE_DIR}/warden-e2e-db`

export default async function globalTeardown() {
  /* Clean up test workspace. */
  if (existsSync(TEST_WORKSPACE)) {
    rmSync(TEST_WORKSPACE, { recursive: true, force: true })
  }

  /* Clean up isolated E2E database. */
  if (existsSync(E2E_DB_DIR)) {
    rmSync(E2E_DB_DIR, { recursive: true, force: true })
  }

  /* Layer 1: API cleanup — removes DB entries + containers. */
  try {
    const projects = await fetchProjects()
    const leaked = projects.filter((p) => p.name.startsWith('warden-e2e-'))

    await Promise.all(leaked.map(async (project) => {
      console.warn(`[E2E] Cleaning up leaked container: ${project.name}`)
      await removeTestProject(project.projectId, project.agentType)
    }))

    if (leaked.length > 0) {
      console.warn(`[E2E] Cleaned up ${leaked.length} leaked container(s)`)
    }
  } catch {
    /* Server may already be stopped — fall through to CLI cleanup. */
  }

  /* Layer 2: CLI fallback — force-remove orphaned containers directly. */
  try {
    const containers = execSync(
      'docker ps -a --filter "name=warden-e2e-" --format "{{.Names}}"',
      { stdio: 'pipe', timeout: 10_000 },
    ).toString().trim()
    if (containers) {
      const names = containers.split('\n').filter(Boolean)
      for (const name of names) {
        execSync(`docker rm -f ${name}`, { stdio: 'pipe', timeout: 10_000 })
      }
      console.warn(`[E2E] Force-removed ${names.length} orphaned container(s) via docker`)
    }
  } catch {
    /* Docker not available or no containers — skip. */
  }
}

import { execSync } from 'node:child_process'
import { existsSync, mkdirSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { fetchProjects, getBaseURL, removeTestProject } from './helpers/api'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

/** Base directory for E2E temp files. Falls back to /tmp if HOME is not set (CI). */
export const E2E_CACHE_DIR = process.env.HOME ? `${process.env.HOME}/.cache` : '/tmp'

/** Workspace directory used by all E2E test containers. */
export const TEST_WORKSPACE = `${E2E_CACHE_DIR}/warden-e2e-workspace`

/** Image tag for E2E test containers, built from local source. */
export const E2E_IMAGE = 'warden-e2e:local'

/**
 * Builds the container image from local source so E2E tests always
 * run against the current code, not a published release.
 *
 * Build output is suppressed unless the build fails.
 */
function buildTestImage(runtime: string): void {
  const containerDir = path.resolve(__dirname, '../../container')

  // Agent CLIs are installed at container startup, not baked into the image.
  // No build args needed — pinned versions are passed as env vars by the engine.
  console.log(`[E2E] Building test image ${E2E_IMAGE} with ${runtime}...`)
  try {
    execSync(`${runtime} build -t ${E2E_IMAGE} ${containerDir}`, {
      stdio: 'pipe',
    })
  } catch (err) {
    const { stdout, stderr } = err as { stdout: Buffer; stderr: Buffer }
    process.stderr.write(stdout)
    process.stderr.write(stderr)
    throw new Error(`Failed to build test image with ${runtime}`)
  }
  console.log(`[E2E] Image built successfully`)
}

/**
 * Runs once before all test files.
 *
 * Builds a local container image from source, creates a minimal git
 * repo at TEST_WORKSPACE for worktree tests, verifies the dev server
 * is reachable, and cleans up stale containers.
 */
export default async function globalSetup() {
  /* Ensure the test workspace exists with a valid git repo. */
  if (!existsSync(TEST_WORKSPACE)) {
    mkdirSync(TEST_WORKSPACE, { recursive: true })
  }

  const gitDir = path.join(TEST_WORKSPACE, '.git')
  if (!existsSync(gitDir)) {
    execSync('git init', { cwd: TEST_WORKSPACE, stdio: 'pipe' })
    writeFileSync(path.join(TEST_WORKSPACE, 'README.md'), '# E2E Test Workspace\n')
    execSync('git add .', { cwd: TEST_WORKSPACE, stdio: 'pipe' })
    execSync(
      'git -c user.email="e2e@warden.test" -c user.name="Warden E2E" commit -m "initial commit"',
      { cwd: TEST_WORKSPACE, stdio: 'pipe' },
    )
  }

  /* Verify a server is reachable. getBaseURL probes :5173 then :8090. */
  try {
    await getBaseURL()
  } catch {
    throw new Error(
      'No server reachable at localhost:5173 or :8092. Run `just dev` or let Playwright start the server.',
    )
  }

  /* Build the test container image from local source. */
  buildTestImage('docker')

  /* Clean up leftover E2E containers from previous interrupted runs.
     Two-layer cleanup: API first (removes DB entries + containers), then
     CLI fallback (catches orphaned containers the API missed). */
  try {
    const projects = await fetchProjects()
    const stale = projects.filter((p) => p.name.startsWith('warden-e2e-'))

    await Promise.all(
      stale.map(async (project) => removeTestProject(project.projectId, project.agentType)),
    )

    if (stale.length > 0) {
      console.log(`[E2E] Cleaned up ${stale.length} stale container(s)`)
    }
  } catch {
    /* non-fatal */
  }

  /* CLI fallback: force-remove any orphaned warden-e2e-* containers that
     the API cleanup missed (e.g. server was down during previous teardown). */
  try {
    const containers = execSync(
      `${activeRuntime} ps -a --filter "name=warden-e2e-" --format "{{.Names}}"`,
      { stdio: 'pipe', timeout: 10_000 },
    )
      .toString()
      .trim()
    if (containers) {
      const names = containers.split('\n').filter(Boolean)
      for (const name of names) {
        execSync(`${activeRuntime} rm -f ${name}`, { stdio: 'pipe', timeout: 10_000 })
      }
      console.log(`[E2E] Force-removed ${names.length} orphaned container(s) via ${activeRuntime}`)
    }
  } catch {
    /* non-fatal */
  }
}

import { execSync } from 'child_process'
import { cpSync, existsSync, mkdirSync, writeFileSync, rmSync } from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'
import { test, expect } from '@playwright/test'

/**
 * Devcontainer feature integration test.
 *
 * Verifies that adding the Warden devcontainer feature to a
 * devcontainer.json produces a working container with all required
 * infrastructure (tmux, terminal scripts, Claude Code CLI).
 *
 * This test exercises the user workflow directly:
 * 1. Write a devcontainer.json with the Warden feature
 * 2. Run `devcontainer up` to build and start the container
 * 3. Exec into the container to verify Warden infrastructure
 * 4. Clean up with `devcontainer down`
 *
 * Prerequisites:
 * - `devcontainer` CLI installed on the host
 * - Docker running
 */

const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..')

// Import would create a circular dep with global-setup, so inline the same logic.
const E2E_CACHE_DIR = process.env.HOME ? `${process.env.HOME}/.cache` : '/tmp'
const BUILD_DIR = `${E2E_CACHE_DIR}/warden-e2e-devcontainer-feature`

/** Warden binaries that must be present in a working container. */
const REQUIRED_BINARIES = [
  '/usr/local/bin/create-terminal.sh',
  '/usr/local/bin/disconnect-terminal.sh',
  '/usr/local/bin/kill-worktree.sh',
  '/usr/local/bin/entrypoint.sh',
  '/usr/local/bin/warden-event-claude.sh',
  '/usr/local/bin/warden-event-codex.sh',
  '/usr/local/bin/warden-heartbeat.sh',
]

/** Check if the devcontainer CLI is available. */
function hasDevcontainerCLI(): boolean {
  try {
    execSync('devcontainer --version', { stdio: 'pipe' })
    return true
  } catch {
    return false
  }
}

/**
 * Container ID extracted from `devcontainer up` output during beforeAll.
 * Used by execInContainer to bypass the devcontainer CLI for exec
 * (avoiding OCI runtime CWD namespace errors).
 */
let devcontainerContainerId: string | undefined

/**
 * Runs a command inside the devcontainer and returns stdout.
 *
 * Uses `docker exec` directly instead of `devcontainer exec` to avoid
 * OCI runtime errors where the host CWD falls outside the container's
 * mount namespace root.
 */
function execInContainer(cmd: string): string {
  if (!devcontainerContainerId) {
    throw new Error('devcontainerContainerId not set — did beforeAll run?')
  }
  return execSync(
    `docker exec ${devcontainerContainerId} sh -c '${cmd}'`,
    { stdio: 'pipe', timeout: 30_000 },
  ).toString().trim()
}

test.describe('Devcontainer feature', () => {
  test.skip(!hasDevcontainerCLI(), 'devcontainer CLI not installed')

  test.beforeAll(() => {
    /* Create build context with a devcontainer.json referencing
       the local Warden feature (staged the same way CI publishes it). */
    if (existsSync(BUILD_DIR)) {
      rmSync(BUILD_DIR, { recursive: true })
    }

    const devcontainerDir = path.join(BUILD_DIR, '.devcontainer')
    const featureDest = path.join(devcontainerDir, 'warden-feature')
    mkdirSync(featureDest, { recursive: true })

    /* Stage local feature: copy feature metadata + all scripts. */
    const featureSource = path.join(REPO_ROOT, 'container', 'devcontainer-feature')
    const scriptsSource = path.join(REPO_ROOT, 'container', 'scripts')
    cpSync(featureSource, featureDest, { recursive: true })
    cpSync(scriptsSource, featureDest, { recursive: true })

    const devcontainerConfig = {
      image: 'mcr.microsoft.com/devcontainers/base:ubuntu-24.04',
      features: {
        './warden-feature': {},
      },
    }
    writeFileSync(
      path.join(devcontainerDir, 'devcontainer.json'),
      JSON.stringify(devcontainerConfig, null, 2),
    )

    /* Build and start the container. Capture the container ID from
       the JSON output so exec can use `docker exec` directly (avoiding
       OCI runtime CWD namespace errors with `devcontainer exec`). */
    console.log('[E2E] Running devcontainer up with local Warden feature...')
    const upOutput = execSync(
      `devcontainer up --workspace-folder ${BUILD_DIR}`,
      { stdio: 'pipe', timeout: 180_000 },
    ).toString()
    try {
      const parsed = JSON.parse(upOutput) as { outcome: string; containerId: string }
      if (parsed.outcome !== 'success') {
        throw new Error(`devcontainer up failed: ${upOutput}`)
      }
      devcontainerContainerId = parsed.containerId
    } catch (err) {
      process.stderr.write(upOutput)
      throw err
    }
    console.log(`[E2E] Devcontainer is running (${devcontainerContainerId?.slice(0, 12)})`)
  })

  test.afterAll(() => {
    /* Tear down the devcontainer. */
    try {
      execSync(`devcontainer down --workspace-folder ${BUILD_DIR}`, {
        stdio: 'pipe',
        timeout: 30_000,
      })
    } catch { /* best effort */ }

    if (existsSync(BUILD_DIR)) {
      rmSync(BUILD_DIR, { recursive: true })
    }
  })

  test('should have all required Warden binaries installed', () => {
    for (const binary of REQUIRED_BINARIES) {
      const result = execInContainer(`test -x ${binary} && echo ok || echo missing`)
      expect(result, `${binary} should be executable`).toBe('ok')
    }
  })

  test('should have tmux installed', () => {
    const result = execInContainer('which tmux')
    expect(result).toContain('tmux')
  })

  test('should have warden user created', () => {
    const result = execInContainer('id warden')
    expect(result).toContain('warden')
  })

  test('should have WARDEN_MANAGED env var set', () => {
    const result = execInContainer('printenv WARDEN_MANAGED')
    expect(result).toBe('true')
  })

  test('should have Claude Code managed settings installed', () => {
    const result = execInContainer('test -f /etc/claude-code/managed-settings.json && echo ok || echo missing')
    expect(result).toBe('ok')
  })
})

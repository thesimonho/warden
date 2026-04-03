import { defineConfig, devices, type PlaywrightTestConfig } from '@playwright/test'

/**
 * Playwright E2E test configuration.
 *
 * Tests are split into two projects:
 * - **ui**: Lightweight tests that use at most one worktree. Run in parallel.
 * - **container**: Tests that spin up multiple worktrees or exercise container
 *   internals. Serialized to avoid Docker/Podman resource contention.
 *
 * Set `WARDEN_RUNTIME` to override the container runtime (docker | podman).
 * When set, project names are prefixed with the runtime (e.g. `docker-ui`,
 * `podman-container`) for clearer reporting in multi-runtime test runs.
 * See `just test-e2e-matrix` for running the full suite against both runtimes.
 *
 * Prefers the Vite dev server at :5173 (fastest iteration, always up-to-date).
 * Falls back to building the frontend and starting the Go backend at :8090
 * if Vite isn't running.
 */

/** Check if a server is already running at a URL. */
async function isServerUp(url: string): Promise<boolean> {
  try {
    const response = await fetch(url, { signal: AbortSignal.timeout(1000) })
    return response.ok
  } catch {
    return false
  }
}

/** Detect which server to use: Vite dev server or Go backend. */
async function resolveServer(): Promise<{
  baseURL: string
  webServer?: PlaywrightTestConfig['webServer']
}> {
  /* Prefer Vite if it's running — always has latest code. */
  if (await isServerUp('http://localhost:5173/api/v1/health')) {
    return { baseURL: 'http://localhost:5173' }
  }

  /* Prefer Go backend if it's running. */
  if (await isServerUp('http://localhost:8090/api/v1/health')) {
    return { baseURL: 'http://localhost:8090' }
  }

  /* Neither running — start Go backend with fresh build and isolated DB. */
  return {
    baseURL: 'http://localhost:8090',
    webServer: {
      command:
        'just build-web && WARDEN_DB_DIR=/tmp/warden-e2e-db WARDEN_LOG_LEVEL=warn WARDEN_NO_OPEN=1 go run ./cmd/warden-desktop',
      cwd: '..',
      url: 'http://localhost:8090/api/v1/health',
      reuseExistingServer: false,
      timeout: 120_000,
      stdout: 'ignore',
      stderr: 'ignore',
    },
  }
}

const serverConfig = await resolveServer()

/**
 * When WARDEN_RUNTIME is set, prefix project names so multi-runtime
 * reports clearly distinguish Docker vs Podman results.
 */
const runtimePrefix = process.env.WARDEN_RUNTIME ? `${process.env.WARDEN_RUNTIME}-` : ''

/** Test-file patterns for the two project tiers. */
const uiTestMatch = /home-page|navigation|project-page|project-lifecycle|panel-maximize|terminal-connection/
const containerTestMatch = /container-integration|codex-container|devcontainer-feature|panel-layout|terminal-resilience/

export default defineConfig({
  testDir: './e2e',
  outputDir: './e2e-results',

  /* Timeouts — container operations are slow. */
  timeout: 120_000,
  expect: { timeout: 15_000 },

  /* Tests within a file are serial (each file is one test suite).
     Retries are disabled — each retry spawns a new worker (= new container),
     which compounds the problem rather than fixing flaky tests. */
  fullyParallel: false,
  workers: 4,
  retries: 0,

  reporter: process.env.CI ? [['html', { outputFolder: 'playwright-report' }]] : [['list']],

  globalSetup: './e2e/global-setup.ts',
  globalTeardown: './e2e/global-teardown.ts',

  use: {
    baseURL: serverConfig.baseURL,

    headless: true,

    /* Capture artifacts on failure for debugging. */
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
  },

  ...(serverConfig.webServer ? { webServer: serverConfig.webServer } : {}),

  projects: [
    {
      name: `${runtimePrefix}ui`,
      testMatch: uiTestMatch,
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: `${runtimePrefix}container`,
      testMatch: containerTestMatch,
      use: { ...devices['Desktop Chrome'] },
      workers: 1,
    },
  ],
})

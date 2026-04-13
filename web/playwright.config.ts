import { defineConfig, devices, type PlaywrightTestConfig } from '@playwright/test'

/**
 * Playwright E2E test configuration.
 *
 * Tests are split into two projects:
 * - **ui**: Lightweight tests that use at most one worktree. Run in parallel.
 * - **container**: Tests that spin up multiple worktrees or exercise container
 *   internals. Serialized to avoid Docker resource contention.
 *
 * Always uses a standalone Go backend on :8092 with an isolated database
 * (~/.cache/warden-e2e-db). Never falls back to the Vite dev server (:5173)
 * or production (:8090) — using the dev server would write E2E test data
 * into the dev database and contaminate the dev UI.
 */

/** Check if a server is already running at a URL and returning valid JSON. */
async function isServerUp(url: string): Promise<boolean> {
  try {
    const response = await fetch(url, { signal: AbortSignal.timeout(2000) })
    if (!response.ok) return false
    // Validate JSON — Vite returns index.html (200) when the Go backend is down.
    const body = (await response.json()) as { status?: string }
    return body.status === 'ok'
  } catch {
    return false
  }
}

/** Resolve the E2E server configuration. Always targets :8092. */
async function resolveServer(): Promise<{
  baseURL: string
  webServer?: PlaywrightTestConfig['webServer']
}> {
  /* Reuse standalone E2E server if already running from a previous invocation. */
  if (await isServerUp('http://localhost:8092/api/v1/health')) {
    return { baseURL: 'http://localhost:8092' }
  }

  /* Start a standalone Go backend on :8092 with isolated DB. */
  return {
    baseURL: 'http://localhost:8092',
    webServer: {
      command:
        'just build-web && WARDEN_DB_DIR="${HOME}/.cache/warden-e2e-db" ADDR=127.0.0.1:8092 WARDEN_LOG_LEVEL=warn WARDEN_NO_OPEN=1 go run ./cmd/warden-desktop',
      cwd: '..',
      url: 'http://localhost:8092/api/v1/health',
      reuseExistingServer: false,
      timeout: 120_000,
      stdout: 'ignore',
      stderr: 'ignore',
    },
  }
}

const serverConfig = await resolveServer()

const runtimePrefix = ''

/** Test-file patterns for the two project tiers. */
const uiTestMatch =
  /home-page|navigation|project-page|project-lifecycle|panel-maximize|terminal-connection/
const containerTestMatch =
  /container-integration|codex-container|devcontainer-feature|panel-layout|terminal-resilience|api-endpoints|api-workflows/

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
  workers: 2,
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

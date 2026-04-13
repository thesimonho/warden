import { execSync } from 'node:child_process'
import {
  connectTerminal,
  createWorktree,
  disconnectTerminal,
  validateContainer,
  waitForWorktreeState,
} from './helpers/api'
import { expect, test } from './helpers/fixtures'

/**
 * Runs a shell command inside the test container.
 * Uses `docker exec` directly (same pattern as devcontainer-feature.spec.ts).
 */
function execInContainer(containerName: string, cmd: string): string {
  return execSync(`docker exec ${containerName} sh -c ${JSON.stringify(cmd)}`, {
    stdio: 'pipe',
    timeout: 30_000,
  })
    .toString()
    .trim()
}

/**
 * Container integration tests.
 *
 * These tests validate the container internals that no frontend test can see:
 * script installation, event bus connectivity, process lifecycle, and state
 * transitions within the real container. Since we already pay the cost of
 * spinning up containers for E2E tests, these checks are essentially free.
 *
 * All API calls use `testProject.agentType` so the suite works with both
 * claude-code (default) and codex (via WARDEN_AGENT_TYPE=codex).
 */
test.describe('Container integration', () => {
  test.describe('Infrastructure validation', () => {
    test('should have all required Warden binaries installed', async ({ testProject }) => {
      const result = await validateContainer(testProject.id, testProject.agentType)

      expect(result.valid).toBe(true)
      expect(result.missing).toBeNull()
    })
  })

  test.describe('Event bus', () => {
    test('should reflect terminal state via event bus within 15 seconds', async ({
      testProject,
    }) => {
      await connectTerminal(testProject.id, 'main', testProject.agentType)

      await waitForWorktreeState(testProject.id, 'main', 'connected', 15_000, testProject.agentType)
    })

    test('should push terminal_connected event when terminal connects', async ({ testProject }) => {
      await connectTerminal(testProject.id, 'main', testProject.agentType)

      await waitForWorktreeState(testProject.id, 'main', 'connected', 30_000, testProject.agentType)
    })

    test('should push terminal_disconnected event when terminal disconnects', async ({
      testProject,
    }) => {
      await connectTerminal(testProject.id, 'main', testProject.agentType)
      await waitForWorktreeState(testProject.id, 'main', 'connected', 30_000, testProject.agentType)

      await disconnectTerminal(testProject.id, 'main', testProject.agentType)

      await waitForWorktreeState(
        testProject.id,
        'main',
        ['background', 'shell'],
        30_000,
        testProject.agentType,
      )
    })
  })

  test.describe('Worktree state machine', () => {
    test('should transition: stopped → connected → background → connected', async ({
      testProject,
    }) => {
      await connectTerminal(testProject.id, 'main', testProject.agentType)
      await waitForWorktreeState(testProject.id, 'main', 'connected', 45_000, testProject.agentType)

      await disconnectTerminal(testProject.id, 'main', testProject.agentType)
      await waitForWorktreeState(
        testProject.id,
        'main',
        ['background', 'shell'],
        30_000,
        testProject.agentType,
      )

      await new Promise((r) => setTimeout(r, 2000))

      await connectTerminal(testProject.id, 'main', testProject.agentType)
      await waitForWorktreeState(testProject.id, 'main', 'connected', 45_000, testProject.agentType)
    })
  })

  test.describe('Concurrent terminals', () => {
    test('should support multiple concurrent terminals', async ({ testProject }) => {
      await connectTerminal(testProject.id, 'main', testProject.agentType).catch(() => {})
      await waitForWorktreeState(testProject.id, 'main', 'connected', 30_000, testProject.agentType)

      try {
        try {
          await connectTerminal(testProject.id, 'e2e-concurrent', testProject.agentType)
        } catch {
          await createWorktree(testProject.id, 'e2e-concurrent', testProject.agentType)
        }

        await waitForWorktreeState(
          testProject.id,
          'e2e-concurrent',
          'connected',
          60_000,
          testProject.agentType,
        )
      } finally {
        await disconnectTerminal(testProject.id, 'e2e-concurrent', testProject.agentType).catch(
          () => {},
        )
      }
    })
  })

  test.describe('Symlink protection', () => {
    /**
     * Writes the entrypoint's symlink dereference logic to a script inside
     * the container and executes it. This avoids shell quoting issues with
     * docker exec while testing the exact guard logic from user-entrypoint.sh.
     */
    function runDereferenceScript(containerName: string): void {
      // Write script to a file to avoid quoting issues with docker exec.
      execSync(
        `docker exec ${containerName} sh -c 'cat > /tmp/deref-test.sh << "SCRIPT"\n` +
          '#!/bin/sh\n' +
          'for config_dir in /home/warden/.claude; do\n' +
          '  [ -d "$config_dir" ] || continue\n' +
          '  find "$config_dir" -maxdepth 1 -type l 2>/dev/null | while IFS= read -r link; do\n' +
          '    target=$(readlink -f "$link" 2>/dev/null) || continue\n' +
          '    [ -f "$target" ] || continue\n' +
          '    target_dir=$(dirname "$target")\n' +
          '    if [ ! -w "$target_dir" ]; then\n' +
          '      continue\n' +
          '    fi\n' +
          '    cp --remove-destination "$target" "$link" 2>/dev/null || true\n' +
          '  done\n' +
          'done\n' +
          "SCRIPT'",
        { stdio: 'pipe', timeout: 10_000 },
      )
      execInContainer(containerName, 'chmod +x /tmp/deref-test.sh')
      // Run as warden user — matches the real entrypoint (gosu drops to warden).
      // Root bypasses -w checks, so running as root would mask the guard.
      execSync(`docker exec --user warden ${containerName} /tmp/deref-test.sh`, {
        stdio: 'pipe',
        timeout: 10_000,
      })
    }

    test('should not dereference symlinks whose targets are in read-only directories', async ({
      testProject,
    }) => {
      const name = testProject.name

      // Create a read-only directory with a config file, then symlink to it
      // from ~/.claude/. This simulates a Nix Home Manager symlink without
      // the overlay mount that would normally hide it.
      // Use root to create the read-only store (simulates /nix/store owned by root).
      execInContainer(name, 'mkdir -p /tmp/readonly-store')
      execInContainer(name, 'echo test-content > /tmp/readonly-store/test-config.json')
      execInContainer(name, 'chmod 555 /tmp/readonly-store')
      // Symlink created by warden user (owns ~/.claude/)
      execSync(
        `docker exec --user warden ${name} ln -sf /tmp/readonly-store/test-config.json /home/warden/.claude/test-config.json`,
        {
          stdio: 'pipe',
          timeout: 10_000,
        },
      )

      const beforeType = execInContainer(name, 'stat -c %F /home/warden/.claude/test-config.json')
      expect(beforeType).toBe('symbolic link')

      runDereferenceScript(name)

      // The symlink should still be a symlink (not replaced with a regular file)
      const afterType = execInContainer(name, 'stat -c %F /home/warden/.claude/test-config.json')
      expect(afterType).toBe('symbolic link')

      // Clean up
      execInContainer(name, 'rm -f /home/warden/.claude/test-config.json')
      execInContainer(name, 'chmod 755 /tmp/readonly-store')
      execInContainer(name, 'rm -rf /tmp/readonly-store')
    })

    test('should dereference symlinks whose targets are in writable directories', async ({
      testProject,
    }) => {
      const name = testProject.name

      // Create a writable directory with a config file, then symlink to it.
      // This simulates a normal dotfile symlink (e.g. GNU Stow) that SHOULD
      // be dereferenced so the agent can write to it.
      // All files owned by warden so cp --remove-destination can replace the symlink.
      execSync(`docker exec --user warden ${name} mkdir -p /tmp/writable-store`, {
        stdio: 'pipe',
        timeout: 10_000,
      })
      execSync(
        `docker exec --user warden ${name} sh -c 'echo test-content > /tmp/writable-store/test-config.json'`,
        {
          stdio: 'pipe',
          timeout: 10_000,
        },
      )
      execSync(
        `docker exec --user warden ${name} ln -sf /tmp/writable-store/test-config.json /home/warden/.claude/test-config.json`,
        {
          stdio: 'pipe',
          timeout: 10_000,
        },
      )

      const beforeType = execInContainer(name, 'stat -c %F /home/warden/.claude/test-config.json')
      expect(beforeType).toBe('symbolic link')

      runDereferenceScript(name)

      // The symlink should now be a regular file (dereferenced)
      const afterType = execInContainer(name, 'stat -c %F /home/warden/.claude/test-config.json')
      expect(afterType).toBe('regular file')

      // Content should match the original
      const content = execInContainer(name, 'cat /home/warden/.claude/test-config.json')
      expect(content).toBe('test-content')

      // Clean up
      execInContainer(name, 'rm -f /home/warden/.claude/test-config.json')
      execInContainer(name, 'rm -rf /tmp/writable-store')
    })
  })
})

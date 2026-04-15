/**
 * Worktree and terminal lifecycle API functions.
 *
 * @module
 */
import type { DiffResponse, Worktree, WorktreeResult } from '@/lib/types'

import { apiFetch, projectUrl } from './api-core'

/** TTL for the fetch deduplication cache (milliseconds). */
const FETCH_CACHE_TTL_MS = 5_000

/** Cached fetch entry: stores the in-flight or recently resolved promise. */
interface CacheEntry {
  promise: Promise<Worktree[]>
  timestamp: number
}

/**
 * Module-level deduplication cache for worktree fetches.
 *
 * When multiple hooks (useWorktrees, useCanvasWorktreeState) poll the same
 * project within the TTL window, they share a single in-flight request
 * instead of issuing duplicate HTTP calls.
 */
const fetchCache = new Map<string, CacheEntry>()

/**
 * Fetches all worktrees for a given project with their terminal state.
 *
 * @param projectId - The project ID to fetch worktrees for.
 * @param agentType - The CLI agent type for this project.
 * @returns An array of worktrees belonging to the project.
 */
export async function fetchWorktrees(projectId: string, agentType: string): Promise<Worktree[]> {
  const key = `${projectId}/${agentType}`
  const now = Date.now()

  const cached = fetchCache.get(key)
  if (cached && now - cached.timestamp < FETCH_CACHE_TTL_MS) {
    return cached.promise
  }

  const promise = fetchWorktreesUncached(projectId, agentType)
  fetchCache.set(key, { promise, timestamp: now })

  // Clean up cache entry after TTL to bound memory. On error, delete
  // immediately so the next caller retries. On success, delete after
  // the TTL so concurrent callers within the window still share the result.
  promise.then(
    () => setTimeout(() => fetchCache.delete(key), FETCH_CACHE_TTL_MS),
    () => fetchCache.delete(key),
  )

  return promise
}

/** Performs the actual HTTP fetch without caching. */
async function fetchWorktreesUncached(projectId: string, agentType: string): Promise<Worktree[]> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/worktrees`)
  return response.json() as Promise<Worktree[]>
}

/**
 * Creates a new git worktree and connects a terminal to it.
 *
 * @param projectId - The project to create the worktree in.
 * @param agentType - The CLI agent type for this project.
 * @param name - The name for the new worktree.
 * @returns The worktree result with worktree and project IDs.
 */
export async function createWorktree(
  projectId: string,
  agentType: string,
  name: string,
): Promise<WorktreeResult> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/worktrees`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  return response.json() as Promise<WorktreeResult>
}

/**
 * Connects a terminal to a worktree, starting Claude Code.
 *
 * @param projectId - The project the worktree belongs to.
 * @param agentType - The CLI agent type for this project.
 * @param worktreeId - The worktree ID to connect.
 * @returns The worktree result with worktree and project IDs.
 */
export async function connectTerminal(
  projectId: string,
  agentType: string,
  worktreeId: string,
): Promise<WorktreeResult> {
  const response = await apiFetch(
    `${projectUrl(projectId, agentType)}/worktrees/${worktreeId}/connect`,
    { method: 'POST' },
  )
  return response.json() as Promise<WorktreeResult>
}

/**
 * Disconnects the terminal viewer from a worktree.
 * The tmux session (and Claude/bash) continues running in the background.
 *
 * @param projectId - The project the worktree belongs to.
 * @param agentType - The CLI agent type for this project.
 * @param worktreeId - The worktree ID to disconnect.
 * @returns The worktree result with worktree and project IDs.
 */
export async function disconnectTerminal(
  projectId: string,
  agentType: string,
  worktreeId: string,
): Promise<WorktreeResult> {
  const response = await apiFetch(
    `${projectUrl(projectId, agentType)}/worktrees/${worktreeId}/disconnect`,
    { method: 'POST' },
  )
  return response.json() as Promise<WorktreeResult>
}

/**
 * Kills the tmux session and all child processes for a worktree.
 * This is destructive — the terminal session is destroyed and cannot be reconnected.
 *
 * @param projectId - The project the worktree belongs to.
 * @param agentType - The CLI agent type for this project.
 * @param worktreeId - The worktree ID to kill.
 * @returns The worktree result with worktree and project IDs.
 */
export async function killWorktreeProcess(
  projectId: string,
  agentType: string,
  worktreeId: string,
): Promise<WorktreeResult> {
  const response = await apiFetch(
    `${projectUrl(projectId, agentType)}/worktrees/${worktreeId}/kill`,
    { method: 'POST' },
  )
  return response.json() as Promise<WorktreeResult>
}

/**
 * Resets a worktree: kills the process, clears agent session files, and removes
 * terminal tracking state. Audit history is preserved. The worktree itself is
 * preserved.
 *
 * @param projectId - The project the worktree belongs to.
 * @param agentType - The CLI agent type for this project.
 * @param worktreeId - The worktree ID to reset.
 * @returns The worktree result with worktree and project IDs.
 */
export async function resetWorktree(
  projectId: string,
  agentType: string,
  worktreeId: string,
): Promise<WorktreeResult> {
  const response = await apiFetch(
    `${projectUrl(projectId, agentType)}/worktrees/${worktreeId}/reset`,
    { method: 'POST' },
  )
  return response.json() as Promise<WorktreeResult>
}

/**
 * Removes a worktree entirely: kills processes, runs `git worktree remove`,
 * and cleans up tracking state. Cannot remove the main worktree.
 *
 * @param projectId - The project the worktree belongs to.
 * @param agentType - The CLI agent type for this project.
 * @param worktreeId - The worktree ID to remove.
 * @returns The worktree result with worktree and project IDs.
 */
export async function removeWorktree(
  projectId: string,
  agentType: string,
  worktreeId: string,
): Promise<WorktreeResult> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/worktrees/${worktreeId}`, {
    method: 'DELETE',
  })
  return response.json() as Promise<WorktreeResult>
}

/** Result of cleaning up orphaned worktree directories. */
interface CleanupWorktreesResult {
  removed: string[] | null
}

/**
 * Removes orphaned worktree directories that exist on disk but are no longer tracked by git.
 *
 * @param projectId - The project whose container to clean up.
 * @param agentType - The CLI agent type for this project.
 * @returns The list of removed worktree IDs.
 */
export async function cleanupWorktrees(
  projectId: string,
  agentType: string,
): Promise<CleanupWorktreesResult> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/worktrees/cleanup`, {
    method: 'POST',
  })
  return response.json() as Promise<CleanupWorktreesResult>
}

/**
 * Fetches uncommitted changes (tracked + untracked) for a worktree.
 *
 * @param projectId - Container ID.
 * @param agentType - The CLI agent type for this project.
 * @param worktreeId - Worktree ID.
 * @returns Diff response with per-file stats and unified diff.
 */
export async function fetchWorktreeDiff(
  projectId: string,
  agentType: string,
  worktreeId: string,
): Promise<DiffResponse> {
  const response = await apiFetch(
    `${projectUrl(projectId, agentType)}/worktrees/${worktreeId}/diff`,
  )
  return response.json() as Promise<DiffResponse>
}

/**
 * Computes the host filesystem path for a worktree by stripping the
 * container-side workspace prefix and prepending the host mount dir.
 *
 * Falls back to `mountedDir + containerPath` when `containerPath` does not
 * start with `workspaceDir` (should not happen in practice — all worktree
 * paths live under the workspace mount).
 */
export function worktreeHostPath(
  mountedDir: string,
  containerPath: string,
  workspaceDir: string,
): string {
  const relative = containerPath.startsWith(workspaceDir)
    ? containerPath.slice(workspaceDir.length)
    : containerPath
  return mountedDir + relative
}

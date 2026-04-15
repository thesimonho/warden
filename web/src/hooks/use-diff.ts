import { useCallback, useEffect, useState } from 'react'

import { fetchWorktreeDiff } from '@/lib/api'
import type { DiffResponse } from '@/lib/types'

/** Return type for the useDiff hook. */
interface UseDiffResult {
  diff: DiffResponse | null
  isLoading: boolean
  error: string | null
  refetch: () => void
}

/**
 * Fetches the worktree diff on demand — only when `enabled` is true.
 *
 * Designed for the "Changes" tab in the terminal card: the diff is
 * fetched once when the user clicks the tab, and can be refreshed
 * manually via `refetch`. No polling or SSE — diffs are not real-time.
 *
 * @param projectId - Container ID.
 * @param agentType - The CLI agent type for this project.
 * @param worktreeId - Worktree ID.
 * @param enabled - When true, fetch the diff. When false, skip.
 * @returns Diff data, loading state, error, and a refetch function.
 */
export function useDiff(
  projectId: string,
  agentType: string,
  worktreeId: string,
  enabled: boolean,
): UseDiffResult {
  const [diff, setDiff] = useState<DiffResponse | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const refetch = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    try {
      const data = await fetchWorktreeDiff(projectId, agentType, worktreeId)
      setDiff(data)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch diff'
      setError(message)
    } finally {
      setIsLoading(false)
    }
  }, [projectId, agentType, worktreeId])

  // Fetch when enabled transitions to true.
  useEffect(() => {
    if (enabled) {
      refetch()
    }
  }, [enabled, refetch])

  return { diff, isLoading, error, refetch }
}

import { startTransition, useCallback, useEffect, useRef, useState } from 'react'
import { deriveWorktreeStateFromEvent } from '@/lib/types'
import type { Worktree, WorktreeStateEvent, WorktreeListChangedEvent } from '@/lib/types'
import { fetchWorktrees } from '@/lib/api'
import { useEventSource, SSE_POLL_INTERVAL_MS } from '@/hooks/use-event-source'

/** Return type for the useWorktrees hook. */
interface UseWorktreesResult {
  worktrees: Worktree[]
  isLoading: boolean
  error: string | null
  refetch: () => void
}

/**
 * Polls the API for worktrees belonging to a project, with real-time
 * attention updates via SSE.
 *
 * Fetches immediately on mount, then every 15 seconds as a safety net.
 * SSE worktree_state events update needsInput/notificationType immediately.
 *
 * @param projectId - The project to fetch worktrees for.
 * @param agentType - The CLI agent type for this project.
 * @returns Worktrees, loading state, error, and a manual refetch function.
 */
export function useWorktrees(projectId: string, agentType: string): UseWorktreesResult {
  const [worktrees, setWorktrees] = useState<Worktree[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const prevJSON = useRef('')

  const refetch = useCallback(async () => {
    try {
      const data = await fetchWorktrees(projectId, agentType)
      const json = JSON.stringify(data)
      if (json !== prevJSON.current) {
        prevJSON.current = json
        startTransition(() => setWorktrees(data ?? []))
      }
      startTransition(() => setError(null))
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch worktrees'
      setError(message)
    } finally {
      setIsLoading(false)
    }
  }, [projectId, agentType])

  useEffect(() => {
    refetch()
    const interval = setInterval(refetch, SSE_POLL_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [refetch])

  /** Applies a worktree_state SSE event to local state. */
  const handleWorktreeState = useCallback(
    (event: WorktreeStateEvent) => {
      if (event.projectId !== projectId || event.agentType !== agentType) return

      startTransition(() => {
        setWorktrees((prev) => {
          const index = prev.findIndex((wt) => wt.id === event.worktreeId)
          if (index === -1) return prev

          const wt = prev[index]

          const nextState = deriveWorktreeStateFromEvent(event, wt.state)

          const nextExitCode =
            event.exitCode != null && event.exitCode >= 0 ? event.exitCode : wt.exitCode

          const isSame =
            wt.needsInput === event.needsInput &&
            wt.notificationType === event.notificationType &&
            wt.state === nextState &&
            wt.exitCode === nextExitCode
          if (isSame) return prev

          const updated = [...prev]
          updated[index] = {
            ...wt,
            state: nextState,
            needsInput: event.needsInput,
            notificationType: event.notificationType,
            exitCode: nextExitCode,
          }
          return updated
        })
      })
    },
    [projectId, agentType],
  )

  /** Refetches the worktree list when a structural change occurs. */
  const handleWorktreeListChanged = useCallback(
    (event: WorktreeListChangedEvent) => {
      if (event.projectId === projectId && event.agentType === agentType) refetch()
    },
    [projectId, agentType, refetch],
  )

  useEventSource({
    onWorktreeState: handleWorktreeState,
    onWorktreeListChanged: handleWorktreeListChanged,
  })

  return { worktrees, isLoading, error, refetch }
}

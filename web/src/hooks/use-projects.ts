import { startTransition, useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import type { BudgetExceededEvent, Project, ProjectStateEvent } from '@/lib/types'
import { fetchProjects } from '@/lib/api'
import { useEventSource, type RuntimeStatusEvent } from '@/hooks/use-event-source'

/** Default polling interval — reduced from 60s since SSE handles real-time updates. */
const DEFAULT_POLL_INTERVAL_MS = 30_000

/** Per-project runtime installation status from SSE events. */
export interface RuntimeStatus {
  message: string
  phase: 'installing' | 'installed'
}

/** Return type for the useProjects hook. */
interface UseProjectsResult {
  projects: Project[]
  isLoading: boolean
  /** True briefly during each background poll cycle. */
  isRefreshing: boolean
  error: string | null
  refetch: () => void
  /** Runtime install status keyed by "projectId/agentType". */
  runtimeStatuses: Map<string, RuntimeStatus>
}

/**
 * Polls the API for the list of projects at a regular interval,
 * with real-time cost updates via SSE.
 *
 * Fetches immediately on mount, then every 30 seconds as a safety net.
 * SSE project_state events update totalCost immediately between polls.
 *
 * @returns The current projects, loading state, error, and refetch function.
 */
export function useProjects(pollIntervalMs = DEFAULT_POLL_INTERVAL_MS): UseProjectsResult {
  const [projects, setProjects] = useState<Project[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const hasLoadedOnce = useRef(false)

  const refetch = useCallback(async () => {
    if (hasLoadedOnce.current) {
      setIsRefreshing(true)
    }
    try {
      const data = await fetchProjects()
      startTransition(() => {
        setProjects((prev) => {
          const hasChanged = JSON.stringify(prev) !== JSON.stringify(data)
          return hasChanged ? data : prev
        })
        setError(null)
      })
      // Clear runtime statuses for stopped containers — stale events from
      // recreation (where the container is briefly started then stopped)
      // should not persist on the card.
      setRuntimeStatuses((prev) => {
        if (prev.size === 0) return prev
        const running = new Set(
          data.filter((p) => p.state === 'running').map((p) => `${p.projectId}/${p.agentType}`),
        )
        const next = new Map<string, RuntimeStatus>()
        for (const [key, status] of prev) {
          if (running.has(key)) next.set(key, status)
        }
        return next.size === prev.size ? prev : next
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch projects'
      setError(message)
    } finally {
      hasLoadedOnce.current = true
      setIsLoading(false)
      setIsRefreshing(false)
    }
  }, [])

  useEffect(() => {
    refetch()
    const interval = setInterval(refetch, pollIntervalMs)
    return () => clearInterval(interval)
  }, [refetch, pollIntervalMs])

  /** Applies a project_state SSE event to local state (cost + attention). */
  const handleProjectState = useCallback((event: ProjectStateEvent) => {
    startTransition(() => {
      setProjects((prev) => {
        const index = prev.findIndex(
          (p) => p.projectId === event.projectId && p.agentType === event.agentType,
        )
        if (index === -1) return prev
        const project = prev[index]

        const costChanged = event.totalCost > 0 && project.totalCost !== event.totalCost
        const attentionChanged =
          project.needsInput !== event.needsInput ||
          project.notificationType !== event.notificationType

        if (!costChanged && !attentionChanged) return prev

        const updated = [...prev]
        updated[index] = {
          ...project,
          ...(costChanged && { totalCost: event.totalCost }),
          needsInput: event.needsInput,
          notificationType: event.notificationType,
        }
        return updated
      })
    })
  }, [])

  /** Shows a toast when a project exceeds its cost budget. */
  const handleBudgetExceeded = useCallback(
    (event: BudgetExceededEvent) => {
      toast.error(
        `Budget exceeded for ${event.containerName}: $${event.totalCost.toFixed(2)} / $${event.budget.toFixed(2)}`,
      )
      refetch()
    },
    [refetch],
  )

  const [runtimeStatuses, setRuntimeStatuses] = useState<Map<string, RuntimeStatus>>(new Map())
  const runtimeTimersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())

  // Clean up all pending timers on unmount.
  useEffect(() => {
    const timers = runtimeTimersRef.current
    return () => {
      for (const id of timers.values()) clearTimeout(id)
      timers.clear()
    }
  }, [])

  /** Tracks runtime install progress per project on the card. */
  const handleRuntimeStatus = useCallback((event: RuntimeStatusEvent) => {
    const key = `${event.projectId}/${event.agentType ?? ''}`

    // Cancel any pending timer for this key.
    const existing = runtimeTimersRef.current.get(key)
    if (existing) clearTimeout(existing)

    if (event.phase === 'installing') {
      setRuntimeStatuses((prev) => {
        const next = new Map(prev)
        next.set(key, { message: `Installing ${event.runtimeLabel}...`, phase: 'installing' })
        return next
      })
      // Safety timeout: clear if no "installed" event arrives within 60s.
      runtimeTimersRef.current.set(
        key,
        setTimeout(() => {
          runtimeTimersRef.current.delete(key)
          setRuntimeStatuses((prev) => {
            const next = new Map(prev)
            if (next.get(key)?.phase === 'installing') next.delete(key)
            return next
          })
        }, 60_000),
      )
    } else {
      setRuntimeStatuses((prev) => {
        const next = new Map(prev)
        next.set(key, { message: `${event.runtimeLabel} installed`, phase: 'installed' })
        return next
      })
      // Clear the "installed" message after a brief delay.
      runtimeTimersRef.current.set(
        key,
        setTimeout(() => {
          runtimeTimersRef.current.delete(key)
          setRuntimeStatuses((prev) => {
            const next = new Map(prev)
            if (next.get(key)?.phase === 'installed') next.delete(key)
            return next
          })
        }, 3000),
      )
    }
  }, [])

  useEventSource({
    onProjectState: handleProjectState,
    onBudgetExceeded: handleBudgetExceeded,
    onRuntimeStatus: handleRuntimeStatus,
  })

  return { projects, isLoading, isRefreshing, error, refetch, runtimeStatuses }
}

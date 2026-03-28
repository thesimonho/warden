import { startTransition, useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import type { BudgetExceededEvent, Project, ProjectStateEvent } from '@/lib/types'
import { fetchProjects } from '@/lib/api'
import { useEventSource } from '@/hooks/use-event-source'

/** Default polling interval — reduced from 60s since SSE handles real-time updates. */
const DEFAULT_POLL_INTERVAL_MS = 30_000

/** Return type for the useProjects hook. */
interface UseProjectsResult {
  projects: Project[]
  isLoading: boolean
  /** True briefly during each background poll cycle. */
  isRefreshing: boolean
  error: string | null
  refetch: () => void
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

  /** Applies a project_state SSE event to local state. */
  const handleProjectState = useCallback((event: ProjectStateEvent) => {
    startTransition(() => {
      setProjects((prev) => {
        const index = prev.findIndex((p) => p.projectId === event.projectId)
        if (index === -1) return prev
        const project = prev[index]
        if (project.totalCost === event.totalCost) return prev
        const updated = [...prev]
        updated[index] = { ...project, totalCost: event.totalCost }
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

  useEventSource({ onProjectState: handleProjectState, onBudgetExceeded: handleBudgetExceeded })

  return { projects, isLoading, isRefreshing, error, refetch }
}

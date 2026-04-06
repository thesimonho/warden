import { startTransition, useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import type { BudgetExceededEvent, Project, ProjectStateEvent } from '@/lib/types'
import { fetchProjects } from '@/lib/api'
import {
  useEventSource,
  type AgentStatusEvent,
  type RuntimeStatusEvent,
} from '@/hooks/use-event-source'

/** Default polling interval — reduced from 60s since SSE handles real-time updates. */
const DEFAULT_POLL_INTERVAL_MS = 30_000

/** Per-project installation status from SSE events (agent CLI or runtime). */
export interface InstallStatus {
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
  /** Install status keyed by "projectId/agentType". */
  installStatuses: Map<string, InstallStatus>
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
          const hasChanged = projectsChanged(prev, data)
          return hasChanged ? data : prev
        })
        setError(null)
      })
      // Clear install statuses for stopped containers — stale events from
      // recreation (where the container is briefly started then stopped)
      // should not persist on the card.
      setInstallStatuses((prev) => {
        if (prev.size === 0) return prev
        const running = new Set(
          data.filter((p) => p.state === 'running').map((p) => `${p.projectId}/${p.agentType}`),
        )
        const next = new Map<string, InstallStatus>()
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

  const [installStatuses, setInstallStatuses] = useState<Map<string, InstallStatus>>(new Map())
  const installTimersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())

  // Clean up all pending timers on unmount.
  useEffect(() => {
    const timers = installTimersRef.current
    return () => {
      for (const id of timers.values()) clearTimeout(id)
      timers.clear()
    }
  }, [])

  /** Applies an install status event (agent CLI or runtime) to the shared status map. */
  const applyInstallStatus = useCallback(
    (
      timerKey: string,
      projectKey: string,
      phase: 'installing' | 'installed',
      installingMessage: string,
      installedMessage: string,
      installingTimeoutMs: number,
    ) => {
      const existing = installTimersRef.current.get(timerKey)
      if (existing) clearTimeout(existing)

      if (phase === 'installing') {
        setInstallStatuses((prev) => {
          const next = new Map(prev)
          next.set(projectKey, { message: installingMessage, phase: 'installing' })
          return next
        })
        // Safety timeout: clear if no "installed" event arrives.
        installTimersRef.current.set(
          timerKey,
          setTimeout(() => {
            installTimersRef.current.delete(timerKey)
            setInstallStatuses((prev) => {
              const next = new Map(prev)
              if (next.get(projectKey)?.phase === 'installing') next.delete(projectKey)
              return next
            })
          }, installingTimeoutMs),
        )
      } else {
        setInstallStatuses((prev) => {
          const next = new Map(prev)
          next.set(projectKey, { message: installedMessage, phase: 'installed' })
          return next
        })
        installTimersRef.current.set(
          timerKey,
          setTimeout(() => {
            installTimersRef.current.delete(timerKey)
            setInstallStatuses((prev) => {
              const next = new Map(prev)
              if (next.get(projectKey)?.phase === 'installed') next.delete(projectKey)
              return next
            })
          }, 3000),
        )
      }
    },
    [],
  )

  const handleRuntimeStatus = useCallback(
    (event: RuntimeStatusEvent) => {
      const projectKey = `${event.projectId}/${event.agentType ?? ''}`
      const timerKey = `runtime:${projectKey}`
      applyInstallStatus(
        timerKey,
        projectKey,
        event.phase,
        `Installing ${event.runtimeLabel}...`,
        `${event.runtimeLabel} installed`,
        60_000,
      )
    },
    [applyInstallStatus],
  )

  const handleAgentStatus = useCallback(
    (event: AgentStatusEvent) => {
      const projectKey = `${event.projectId}/${event.agentType ?? ''}`
      const timerKey = `agent:${projectKey}`
      const label = event.agentType === 'codex' ? 'Codex' : 'Claude Code'
      applyInstallStatus(
        timerKey,
        projectKey,
        event.phase,
        `Installing ${label} ${event.version}...`,
        `${label} ${event.version} installed`,
        120_000,
      )
    },
    [applyInstallStatus],
  )

  useEventSource({
    onProjectState: handleProjectState,
    onBudgetExceeded: handleBudgetExceeded,
    onRuntimeStatus: handleRuntimeStatus,
    onAgentStatus: handleAgentStatus,
  })

  return { projects, isLoading, isRefreshing, error, refetch, installStatuses }
}

/**
 * Compares two project lists by the fields that actually change between polls.
 * Avoids the overhead of two full JSON.stringify serializations per poll cycle.
 */
function projectsChanged(prev: Project[], next: Project[]): boolean {
  if (prev.length !== next.length) return true
  for (let i = 0; i < prev.length; i++) {
    const a = prev[i]
    const b = next[i]
    if (
      a.projectId !== b.projectId ||
      a.state !== b.state ||
      a.totalCost !== b.totalCost ||
      a.needsInput !== b.needsInput ||
      a.notificationType !== b.notificationType ||
      a.activeWorktreeCount !== b.activeWorktreeCount ||
      a.hasContainer !== b.hasContainer ||
      a.agentStatus !== b.agentStatus
    ) {
      return true
    }
  }
  return false
}

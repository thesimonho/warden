import { useEffect, useRef } from 'react'
import type { NotificationType, Project } from '@/lib/types'
import { getAttentionConfig } from '@/lib/notification-config'

/**
 * Sends browser notifications when project worktrees need attention or change state.
 *
 * Tracks previous project state and fires notifications on transitions:
 * - A project's `needsInput` becomes true (with type-specific messages)
 * - A running project's `activeWorktreeCount` drops to zero (all worktrees done)
 *
 * Each notification fires at most once per attention cycle. The "already
 * notified" flag is only cleared when the agent goes back to working
 * (needsInput becomes false for a sustained period), so SSE event races
 * (e.g. CostUpdate briefly clearing needsInput before TurnComplete
 * restores it) don't trigger duplicate notifications.
 *
 * @param projects - Current list of projects to monitor.
 * @param enabled - Whether notifications are enabled in settings.
 */
export function useNotifications(projects: readonly Project[], enabled: boolean): void {
  const previousRef = useRef<Map<string, ProjectSnapshot>>(new Map())
  /** Tracks whether we already notified for the current attention cycle. */
  const notifiedRef = useRef<Set<string>>(new Set())

  useEffect(() => {
    if (!enabled) return
    if (!('Notification' in window) || Notification.permission !== 'granted') return

    const previous = previousRef.current
    const next = new Map<string, ProjectSnapshot>()

    for (const project of projects) {
      const snapshot: ProjectSnapshot = {
        needsInput: project.needsInput ?? false,
        notificationType: project.notificationType,
        activeWorktreeCount: project.activeWorktreeCount,
        state: project.state,
      }
      next.set(project.projectId, snapshot)

      const prev = previous.get(project.projectId)
      if (!prev) continue

      // Clear the notified flag when the agent genuinely goes back to working.
      if (!snapshot.needsInput && prev.needsInput) {
        notifiedRef.current.delete(project.projectId)
      }

      if (snapshot.needsInput && !prev.needsInput && !notifiedRef.current.has(project.projectId)) {
        const config = getAttentionConfig(snapshot.notificationType)
        notify(`${project.name} needs attention`, config.message)
        notifiedRef.current.add(project.projectId)
      }

      const wasRunning = prev.state === 'running' && prev.activeWorktreeCount > 0
      const allDone = snapshot.state === 'running' && snapshot.activeWorktreeCount === 0
      if (wasRunning && allDone && !notifiedRef.current.has(project.projectId)) {
        notify(`${project.name} worktrees complete`, 'All worktrees have finished.')
        notifiedRef.current.add(project.projectId)
      }
    }

    previousRef.current = next
  }, [projects, enabled])
}

/** Minimal snapshot of project state for diffing. */
interface ProjectSnapshot {
  needsInput: boolean
  notificationType?: NotificationType
  activeWorktreeCount: number
  state: string
}

/** Sends a browser notification. */
function notify(title: string, body: string): void {
  try {
    new Notification(title, { body, icon: '/favicon.ico' })
  } catch {
    // Notification constructor can throw in some environments
  }
}

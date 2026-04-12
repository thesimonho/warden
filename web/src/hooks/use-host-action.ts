import { useCallback } from 'react'
import { toast } from 'sonner'
import { worktreeHostPath } from '@/lib/api'
import type { Worktree, WorkspaceMount } from '@/lib/types'

/**
 * Factory hook for host filesystem actions on worktrees (e.g. reveal
 * in file manager, open in editor). Returns a stable callback that
 * maps the worktree's container path to the host path and calls the
 * given action, or undefined when no mount mapping is available.
 *
 * @param action - API function that accepts an absolute host path.
 * @param errorLabel - Human-readable label for the toast on failure.
 * @param mount - Host<->container path mapping, or undefined if unavailable.
 */
export function useHostAction(
  action: (path: string) => Promise<void>,
  errorLabel: string,
  mount: WorkspaceMount | undefined,
) {
  const callback = useCallback(
    (worktree: Worktree) => {
      if (!mount) return
      const hostPath = worktreeHostPath(mount.mountedDir, worktree.path, mount.workspaceDir)
      action(hostPath).catch((err) => {
        const message = err instanceof Error ? err.message : 'Unknown error'
        toast.error(errorLabel, { description: message })
      })
    },
    [action, errorLabel, mount],
  )

  return mount ? callback : undefined
}

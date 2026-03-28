import { useCallback } from 'react'
import { toast } from 'sonner'
import { revealInFileManager, worktreeHostPath } from '@/lib/api'
import type { Worktree, WorkspaceMount } from '@/lib/types'

/**
 * Returns a stable callback that opens a worktree's host directory
 * in the system file manager, or undefined when no mount mapping is available.
 *
 * @param mount - Host↔container path mapping, or undefined if unavailable.
 */
export function useRevealInFileManager(mount: WorkspaceMount | undefined) {
  const reveal = useCallback(
    (worktree: Worktree) => {
      if (!mount) return
      const hostPath = worktreeHostPath(mount.mountedDir, worktree.path, mount.workspaceDir)
      revealInFileManager(hostPath).catch((err) => {
        const message = err instanceof Error ? err.message : 'Unknown error'
        toast.error('Failed to open file manager', { description: message })
      })
    },
    [mount],
  )

  return mount ? reveal : undefined
}

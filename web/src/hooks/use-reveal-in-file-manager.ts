import { revealInFileManager } from '@/lib/api'
import type { WorkspaceMount } from '@/lib/types'
import { useHostAction } from './use-host-action'

/**
 * Returns a stable callback that opens a worktree's host directory
 * in the system file manager, or undefined when no mount mapping is available.
 *
 * @param mount - Host<->container path mapping, or undefined if unavailable.
 */
export function useRevealInFileManager(mount: WorkspaceMount | undefined) {
  return useHostAction(revealInFileManager, 'Failed to open file manager', mount)
}

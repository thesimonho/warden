import { openInEditor } from '@/lib/api'
import type { WorkspaceMount } from '@/lib/types'
import { useHostAction } from './use-host-action'

/**
 * Returns a stable callback that opens a worktree's host directory
 * in the user's preferred code editor, or undefined when no mount
 * mapping is available.
 *
 * @param mount - Host<->container path mapping, or undefined if unavailable.
 */
export function useOpenInEditor(mount: WorkspaceMount | undefined) {
  return useHostAction(openInEditor, 'Failed to open editor', mount)
}

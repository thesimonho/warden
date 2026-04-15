import { type RefObject, useCallback } from 'react'
import { toast } from 'sonner'

import type { TerminalCardHandle } from '@/components/project/terminal-card'
import { disconnectTerminal } from '@/lib/api'

/**
 * Returns a stable callback that disconnects a terminal panel.
 *
 * Detaches the terminal ref, calls the disconnect API, shows a toast
 * on error, and invokes `onRemove` to remove the panel from the view.
 *
 * @param projectId - Container ID.
 * @param agentType - The CLI agent type for this project.
 * @param worktreeId - Worktree ID.
 * @param panelId - Panel ID to remove.
 * @param terminalRef - Ref to the TerminalCard handle for cleanup.
 * @param onRemove - Callback to remove the panel from the parent.
 */
export function useTerminalDisconnect(
  projectId: string,
  agentType: string,
  worktreeId: string,
  panelId: string,
  terminalRef: RefObject<TerminalCardHandle | null>,
  onRemove: (panelId: string) => void,
): () => Promise<void> {
  return useCallback(async () => {
    terminalRef.current?.detach()

    try {
      await disconnectTerminal(projectId, agentType, worktreeId)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error'
      toast.error('Failed to disconnect', { description: message })
    }
    onRemove(panelId)
  }, [projectId, agentType, worktreeId, panelId, terminalRef, onRemove])
}

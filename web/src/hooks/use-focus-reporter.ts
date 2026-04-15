/**
 * Reports viewer focus state to the server for desktop notification suppression.
 *
 * A worktree is "focused" when it has a panel open in the grid/canvas while the
 * browser tab has focus. No viewport intersection is checked — if the panel
 * exists in the view, it counts as focused. Multiple worktrees can be focused
 * simultaneously.
 *
 * Unfocus triggers: tab switch, window minimize, click to another app, inactive
 * 2nd-monitor window, navigation away from the project page (hook unmounts).
 *
 * The hook sends a heartbeat every 30 seconds to keep the server-side TTL
 * alive (server expires entries after 45s of silence). On page unload,
 * `navigator.sendBeacon` ensures the unfocus message is delivered reliably.
 *
 * @module
 */
import { useCallback, useEffect, useRef } from 'react'

import type { FocusRequest } from '@/lib/api-focus'
import { reportFocus, reportFocusBeacon } from '@/lib/api-focus'
import type { CanvasPanel } from '@/lib/canvas-store'

/** Heartbeat interval — must be less than the server's 45s focusEntryTTL. */
const HEARTBEAT_INTERVAL_MS = 30_000

/**
 * Reports which worktrees are focused to the Warden server.
 *
 * Mount this hook in the project view where the panel list is available.
 * When the component unmounts (user navigates away), it sends `focused: false`.
 *
 * @param projectId - The current project ID.
 * @param agentType - The current agent type.
 * @param panels - All open panels in the grid/canvas.
 */
export function useFocusReporter(
  projectId: string,
  agentType: string,
  panels: CanvasPanel[],
): void {
  const clientIdRef = useRef(crypto.randomUUID())

  // Keep a ref to the latest values so callbacks/intervals always read current state.
  const stateRef = useRef({ projectId, agentType, panels })
  useEffect(() => {
    stateRef.current = { projectId, agentType, panels }
  }, [projectId, agentType, panels])

  /** Builds and sends a focus request based on current state. */
  const sendFocusState = useCallback(() => {
    const { projectId: pid, agentType: at, panels: p } = stateRef.current
    const hasFocus = document.hasFocus()

    const req: FocusRequest = {
      clientId: clientIdRef.current,
      focused: hasFocus && p.length > 0,
      ...(hasFocus && p.length > 0
        ? {
            projectId: pid,
            agentType: at,
            worktreeIds: p.map((panel) => panel.worktreeId),
          }
        : {}),
    }

    reportFocus(req).catch(() => {
      // Best-effort — focus tracking should never interrupt the user.
    })
  }, [])

  // Subscribe to visibility/focus changes.
  useEffect(() => {
    document.addEventListener('visibilitychange', sendFocusState)
    window.addEventListener('focus', sendFocusState)
    window.addEventListener('blur', sendFocusState)

    return () => {
      document.removeEventListener('visibilitychange', sendFocusState)
      window.removeEventListener('focus', sendFocusState)
      window.removeEventListener('blur', sendFocusState)
    }
  }, [sendFocusState])

  // Re-send when panels change (worktree added/removed/swapped).
  useEffect(() => {
    sendFocusState()
  }, [sendFocusState])

  // Heartbeat: refresh the server-side TTL every 30 seconds.
  // Sends unconditionally — sendFocusState reads document.hasFocus()
  // and the server skips broadcasts for unchanged heartbeats.
  useEffect(() => {
    const interval = setInterval(sendFocusState, HEARTBEAT_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [sendFocusState])

  // On page unload, send focused=false via sendBeacon (survives page teardown).
  // SPA navigation triggers cleanup via the panels effect above.
  useEffect(() => {
    const clientId = clientIdRef.current
    const handleUnload = () => reportFocusBeacon({ clientId, focused: false })

    window.addEventListener('beforeunload', handleUnload)
    return () => window.removeEventListener('beforeunload', handleUnload)
  }, [])
}

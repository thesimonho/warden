/**
 * Viewer focus state API functions.
 *
 * Reports which project and worktrees the client is actively viewing so the
 * system tray can suppress desktop notifications for focused projects.
 *
 * @module
 */
import { apiFetch, API_BASE } from './api-core'

/** Request body for POST /api/v1/focus. */
export interface FocusRequest {
  clientId: string
  focused: boolean
  projectId?: string
  agentType?: string
  worktreeIds?: string[]
}

/**
 * Reports the client's focus state to the server.
 *
 * Fire-and-forget — errors are silently ignored since focus tracking
 * is best-effort and should never interrupt the user experience.
 */
export async function reportFocus(req: FocusRequest): Promise<void> {
  await apiFetch('/api/v1/focus', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
}

/**
 * Reports focus state via sendBeacon for reliable delivery during page unload.
 *
 * `navigator.sendBeacon` is guaranteed to be sent even during `beforeunload`,
 * unlike `fetch` which may be cancelled by the browser.
 */
export function reportFocusBeacon(req: FocusRequest): void {
  const url = `${API_BASE}/api/v1/focus`
  const blob = new Blob([JSON.stringify(req)], { type: 'application/json' })
  navigator.sendBeacon(url, blob)
}

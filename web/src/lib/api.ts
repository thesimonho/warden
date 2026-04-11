/**
 * Warden HTTP API client — reference implementation for web frontends.
 *
 * If you're building a web application against the Warden API, copy the
 * patterns in this file. Every function demonstrates how to call a specific
 * endpoint and type the response. Key patterns:
 *
 * - **Centralized error handling**: `apiFetch` wraps `fetch`, checks the
 *   response status, and extracts the `{ error }` body the Go server returns.
 *   All callers get consistent error messages without per-call try/catch.
 *
 * - **Response typing**: Each function casts the parsed JSON to a known type
 *   via `response.json() as Promise<T>`. The types in `types.ts` mirror the
 *   Go structs — keep them in sync when the API changes.
 *
 * - **Request bodies**: POST/PUT calls set `Content-Type: application/json`
 *   and pass `JSON.stringify(body)`. The Go server expects JSON for all
 *   mutation endpoints.
 *
 * - **Query parameters**: GET endpoints with filters (e.g. `fetchAuditLog`)
 *   build a `URLSearchParams` and append it to the URL. Only non-empty
 *   values are included.
 *
 * For the Go HTTP client equivalent, see the `client/` package.
 * For SSE events, see `use-event-source.ts`.
 * For terminal WebSocket, see `use-terminal.ts`.
 *
 * @module
 */

// Re-export everything from domain modules so existing imports keep working.
// Each module corresponds to a resource domain in the API.
export { ApiError } from './api-core'
export {
  fetchProjects,
  addProject,
  removeProject,
  stopProject,
  restartProject,
  resetProjectCosts,
  purgeProjectAudit,
  batchProjectOperation,
} from './api-projects'
export {
  fetchWorktrees,
  createWorktree,
  connectTerminal,
  disconnectTerminal,
  killWorktreeProcess,
  resetWorktree,
  removeWorktree,
  cleanupWorktrees,
  fetchWorktreeDiff,
  worktreeHostPath,
} from './api-worktrees'
export {
  validateContainer,
  createContainer,
  deleteContainer,
  fetchContainerConfig,
  updateContainer,
  revealInFileManager,
  listDirectories,
} from './api-containers'
export {
  fetchDockerStatus,
  fetchSettings,
  updateSettings,
  fetchDefaults,
  readProjectTemplate,
  validateProjectTemplate,
  shutdownServer,
} from './api-settings'
export type { DefaultMount, DefaultEnvVar, Defaults } from './api-settings'
export {
  fetchAuditLog,
  fetchAuditSummary,
  auditExportUrl,
  fetchAuditProjects,
  postAuditEvent,
  deleteAuditEvents,
} from './api-audit'
export {
  fetchAccessItems,
  fetchAccessItem,
  createAccessItem,
  updateAccessItem,
  deleteAccessItem,
  resetAccessItem,
  resolveAccessItems,
} from './api-access'
export { uploadClipboardImage } from './api-clipboard'
export { reportFocus, reportFocusBeacon } from './api-focus'
export type { FocusRequest } from './api-focus'

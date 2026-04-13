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

export {
  createAccessItem,
  deleteAccessItem,
  fetchAccessItem,
  fetchAccessItems,
  resetAccessItem,
  resolveAccessItems,
  updateAccessItem,
} from './api-access'
export {
  auditExportUrl,
  deleteAuditEvents,
  fetchAuditLog,
  fetchAuditProjects,
  fetchAuditSummary,
  postAuditEvent,
} from './api-audit'
export { uploadClipboardImage } from './api-clipboard'
export {
  checkContainerName,
  createContainer,
  deleteContainer,
  fetchContainerConfig,
  listDirectories,
  revealInFileManager,
  updateContainer,
  validateContainer,
} from './api-containers'
// Re-export everything from domain modules so existing imports keep working.
// Each module corresponds to a resource domain in the API.
export { ApiError } from './api-core'
export type { FocusRequest } from './api-focus'
export { reportFocus, reportFocusBeacon } from './api-focus'
export {
  addProject,
  batchProjectOperation,
  fetchProjects,
  purgeProjectAudit,
  removeProject,
  resetProjectCosts,
  restartProject,
  stopProject,
} from './api-projects'
export type { DefaultEnvVar, DefaultMount, Defaults } from './api-settings'
export {
  fetchDefaults,
  fetchDockerStatus,
  fetchSettings,
  readProjectTemplate,
  shutdownServer,
  updateSettings,
  validateProjectTemplate,
} from './api-settings'
export {
  cleanupWorktrees,
  connectTerminal,
  createWorktree,
  disconnectTerminal,
  fetchWorktreeDiff,
  fetchWorktrees,
  killWorktreeProcess,
  removeWorktree,
  resetWorktree,
  worktreeHostPath,
} from './api-worktrees'

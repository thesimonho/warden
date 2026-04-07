/**
 * Core API utilities — error handling and base fetch wrapper.
 *
 * These are the building blocks for all API client functions. The `apiFetch`
 * helper centralizes error extraction so individual endpoint functions
 * don't need per-call error handling.
 *
 * @module
 */

/** Base URL for API requests. Empty string since Vite proxies /api to the backend. */
export const API_BASE = ''

/**
 * Returns the port the Go backend is listening on. In production the SPA
 * is served from the same origin, so `window.location.port` is correct.
 * In development the Vite dev server runs on a different port and proxies
 * /api to the backend — `VITE_API_PORT` (defaulting to 8090) provides
 * the real backend port for subdomain proxy URLs.
 */
export function serverPort(): string {
  if (import.meta.env.DEV) {
    return (import.meta.env.VITE_API_PORT as string) || '8090'
  }
  return window.location.port
}

/** Builds the URL prefix for a project-scoped API endpoint. */
export function projectUrl(projectId: string, agentType: string): string {
  return `/api/v1/projects/${projectId}/${agentType}`
}

/**
 * Error thrown by `apiFetch` for non-ok responses.
 *
 * The `code` field contains a machine-readable error code (e.g. "NAME_TAKEN",
 * "REQUIRED_FIELD") for programmatic error handling. Match on `code` instead
 * of parsing the human-readable `message`.
 */
export class ApiError extends Error {
  readonly code: string
  readonly status: number

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

/**
 * Performs a fetch request and throws on non-ok responses.
 *
 * This is the single point of error handling for all API calls. The Go server
 * returns errors as `{ "error": "message", "code": "CODE" }` — this function
 * extracts both and throws an `ApiError` so callers can display the message
 * directly or branch on the code.
 *
 * @param url - The URL to fetch.
 * @param options - Optional fetch init options.
 * @returns The raw Response object.
 * @throws ApiError with the server's error code and message, or the HTTP status text.
 */
export async function apiFetch(url: string, options?: RequestInit): Promise<Response> {
  const response = await fetch(`${API_BASE}${url}`, options)

  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`
    let code = 'UNKNOWN'
    try {
      const body = (await response.json()) as { error?: string; code?: string }
      if (body.error) {
        message = body.error
      }
      if (body.code) {
        code = body.code
      }
    } catch {
      // Response body wasn't JSON, use status text
    }
    throw new ApiError(response.status, code, message)
  }

  return response
}

/**
 * Access item API functions.
 *
 * @module
 */
import type { AccessCredential, AccessItem, AccessItemResponse, ResolvedItem } from '@/lib/types'

import { apiFetch } from './api-core'

/** Response shape from GET /api/v1/access. */
interface AccessItemListResponse {
  items: AccessItemResponse[]
}

/**
 * Fetches all access items with detection status.
 *
 * @returns An array of access items enriched with host detection results.
 */
export async function fetchAccessItems(): Promise<AccessItemResponse[]> {
  const response = await apiFetch('/api/v1/access')
  const body = (await response.json()) as AccessItemListResponse
  return body.items
}

/**
 * Fetches a single access item by ID.
 *
 * @param id - The access item identifier.
 * @returns The access item with detection status.
 */
export async function fetchAccessItem(id: string): Promise<AccessItemResponse> {
  const response = await apiFetch(`/api/v1/access/${id}`)
  return response.json() as Promise<AccessItemResponse>
}

/** Request body for creating a user-defined access item. */
interface CreateAccessItemRequest {
  label: string
  description: string
  credentials: AccessCredential[]
}

/**
 * Creates a new user-defined access item.
 *
 * @param req - The access item definition.
 * @returns The created access item with detection status.
 */
export async function createAccessItem(req: CreateAccessItemRequest): Promise<AccessItemResponse> {
  const response = await apiFetch('/api/v1/access', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  return response.json() as Promise<AccessItemResponse>
}

/** Request body for updating a user-defined access item. */
interface UpdateAccessItemRequest {
  label?: string
  description?: string
  credentials?: AccessCredential[]
}

/**
 * Updates a user-defined access item.
 *
 * @param id - The access item identifier.
 * @param req - The fields to update.
 * @returns The updated access item with detection status.
 */
export async function updateAccessItem(
  id: string,
  req: UpdateAccessItemRequest,
): Promise<AccessItemResponse> {
  const response = await apiFetch(`/api/v1/access/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  return response.json() as Promise<AccessItemResponse>
}

/**
 * Deletes a user-defined access item. Rejects deletion of built-in items.
 *
 * @param id - The access item identifier.
 */
export async function deleteAccessItem(id: string): Promise<void> {
  await apiFetch(`/api/v1/access/${id}`, { method: 'DELETE' })
}

/**
 * Resets a built-in access item to its default configuration.
 *
 * @param id - The access item identifier.
 * @returns The reset access item with detection status.
 */
export async function resetAccessItem(id: string): Promise<AccessItemResponse> {
  const response = await apiFetch(`/api/v1/access/${id}/reset`, { method: 'POST' })
  return response.json() as Promise<AccessItemResponse>
}

/** Response shape from POST /api/v1/access/resolve. */
interface ResolveAccessItemsResponse {
  items: ResolvedItem[]
}

/**
 * Resolves access items for test/preview, returning the computed injections.
 * Accepts full item objects — no DB lookup is performed server-side.
 *
 * @param items - The access items to resolve.
 * @returns An array of resolved items with per-credential injection details.
 */
export async function resolveAccessItems(items: AccessItem[]): Promise<ResolvedItem[]> {
  const response = await apiFetch('/api/v1/access/resolve', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ items }),
  })
  const body = (await response.json()) as ResolveAccessItemsResponse
  return body.items
}

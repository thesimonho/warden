/**
 * Audit log API functions.
 *
 * @module
 */
import type { AuditFilters, AuditLogEntry, AuditLogLevel, AuditSummary } from '@/lib/types'
import { API_BASE, apiFetch } from './api-core'

/** Converts audit filter fields to URLSearchParams. */
function auditFiltersToParams(filters?: AuditFilters): URLSearchParams {
  const params = new URLSearchParams()
  if (filters?.projectId) params.set('projectId', filters.projectId)
  if (filters?.worktree) params.set('worktree', filters.worktree)
  if (filters?.source) params.set('source', filters.source)
  if (filters?.level) params.set('level', filters.level)
  if (filters?.since) params.set('since', filters.since)
  if (filters?.until) params.set('until', filters.until)
  return params
}

/**
 * Fetches audit-relevant events from the server with optional filters.
 *
 * @param filters - Optional filters for container, worktree, category, time range.
 * @returns Array of event log entries matching the audit criteria.
 */
export async function fetchAuditLog(filters?: AuditFilters): Promise<AuditLogEntry[]> {
  const params = auditFiltersToParams(filters)
  if (filters?.category) params.set('category', filters.category)
  if (filters?.limit) params.set('limit', String(filters.limit))
  if (filters?.offset) params.set('offset', String(filters.offset))
  const query = params.toString()
  const path = query ? `/api/v1/audit?${query}` : '/api/v1/audit'
  const response = await apiFetch(path)
  return response.json() as Promise<AuditLogEntry[]>
}

/**
 * Fetches aggregate audit statistics.
 *
 * @param filters - Optional filters for container, worktree, time range.
 * @returns Summary with session, tool, prompt counts and top tools.
 */
export async function fetchAuditSummary(filters?: AuditFilters): Promise<AuditSummary> {
  const params = auditFiltersToParams(filters)
  const query = params.toString()
  const path = query ? `/api/v1/audit/summary?${query}` : '/api/v1/audit/summary'
  const response = await apiFetch(path)
  return response.json() as Promise<AuditSummary>
}

/**
 * Builds the URL for downloading audit log exports.
 *
 * @param format - Export format: 'csv' or 'json'.
 * @param filters - Optional filters for container, worktree, category, time range.
 * @returns URL string for the export endpoint.
 */
export function auditExportUrl(format: 'csv' | 'json', filters?: AuditFilters): string {
  const params = auditFiltersToParams(filters)
  params.set('format', format)
  if (filters?.category) params.set('category', filters.category)
  return `${API_BASE}/api/v1/audit/export?${params.toString()}`
}

/** Fetches distinct project (container) names from the audit log. */
export async function fetchAuditProjects(): Promise<string[]> {
  const response = await apiFetch('/api/v1/audit/projects')
  return response.json() as Promise<string[]>
}

/** Posts a frontend event to the audit log. */
export async function postAuditEvent(params: {
  event: string
  level?: AuditLogLevel
  message?: string
  attrs?: Record<string, unknown>
}): Promise<void> {
  await apiFetch('/api/v1/audit', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

/** Deletes audit events matching the given filters. */
export async function deleteAuditEvents(filters?: AuditFilters): Promise<void> {
  const params = auditFiltersToParams(filters)
  const query = params.toString()
  const path = query ? `/api/v1/audit?${query}` : '/api/v1/audit'
  await apiFetch(path, { method: 'DELETE' })
}

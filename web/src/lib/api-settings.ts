/**
 * Settings, defaults, and runtime API functions.
 *
 * @module
 */
import type { RuntimeInfo, ServerSettings, RuntimeDefault, ProjectTemplate } from '@/lib/types'
import { apiFetch } from './api-core'

/**
 * Fetches available container runtimes and their status.
 *
 * @returns An array of runtime info objects.
 */
export async function fetchRuntimes(): Promise<RuntimeInfo[]> {
  const response = await apiFetch('/api/v1/runtimes')
  return response.json() as Promise<RuntimeInfo[]>
}

/**
 * Fetches server-side settings.
 *
 * @returns The current server settings.
 */
export async function fetchSettings(): Promise<ServerSettings> {
  const response = await apiFetch('/api/v1/settings')
  return response.json() as Promise<ServerSettings>
}

/**
 * Updates server-side settings.
 *
 * @param settings - The settings to update.
 * @returns Whether a server restart is required.
 */
export async function updateSettings(
  settings: Partial<ServerSettings>,
): Promise<{ restartRequired: boolean }> {
  const response = await apiFetch('/api/v1/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  })
  return response.json() as Promise<{ restartRequired: boolean }>
}

/**
 * Requests a graceful server shutdown. The server responds before
 * initiating the shutdown sequence.
 */
export async function shutdownServer(): Promise<void> {
  await apiFetch('/api/v1/shutdown', { method: 'POST' })
}

/** A default mount resolved by the server. */
export interface DefaultMount {
  hostPath: string
  containerPath: string
  readOnly: boolean
  /** Restricts this mount to a specific agent type. Empty means all. */
  agentType?: string
  /** Marks this mount as mandatory for the agent to function. */
  required?: boolean
}

/** A default environment variable resolved by the server. */
export interface DefaultEnvVar {
  key: string
  value: string
}

/** Server-resolved default values for the create container form. */
export interface Defaults {
  homeDir: string
  containerHomeDir: string
  mounts: DefaultMount[]
  envVars?: DefaultEnvVar[]
  restrictedDomains?: Record<string, string[]>
  runtimes?: RuntimeDefault[]
  /** Project template loaded from .warden.json in the project directory. */
  template?: ProjectTemplate
}

/**
 * Fetches server-resolved default values for the create container form.
 * When projectPath is provided, runtime detection scans that directory.
 *
 * @param projectPath - Optional project path for runtime auto-detection.
 * @returns Default configuration values resolved on the server.
 */
export async function fetchDefaults(projectPath?: string): Promise<Defaults> {
  const url = projectPath
    ? `/api/v1/defaults?path=${encodeURIComponent(projectPath)}`
    : '/api/v1/defaults'
  const response = await apiFetch(url)
  return response.json() as Promise<Defaults>
}

/**
 * Reads a .warden.json project template from an arbitrary file path on the server.
 * Used by the "Import" button to load templates from outside the project directory.
 *
 * @param filePath - Absolute path to the .warden.json file.
 * @returns Parsed project template.
 */
export async function readProjectTemplate(filePath: string): Promise<ProjectTemplate> {
  const response = await apiFetch(`/api/v1/template?path=${encodeURIComponent(filePath)}`)
  return response.json() as Promise<ProjectTemplate>
}

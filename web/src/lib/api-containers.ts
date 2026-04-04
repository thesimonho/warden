/**
 * Container management API functions.
 *
 * @module
 */
import type {
  ContainerConfig,
  ContainerResult,
  CreateContainerRequest,
  DirEntry,
} from '@/lib/types'
import { apiFetch, projectUrl } from './api-core'

/** Result of validating a container's Warden infrastructure. */
interface ValidateContainerResult {
  valid: boolean
  missing: string[] | null
}

/**
 * Validates whether a project's container has the required Warden terminal infrastructure.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @param agentType - The CLI agent type for this project.
 * @returns Whether the container is valid and which binaries are missing.
 */
export async function validateContainer(
  projectId: string,
  agentType: string,
): Promise<ValidateContainerResult> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/container/validate`)
  return response.json() as Promise<ValidateContainerResult>
}

/**
 * Creates a new container for a project.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @param agentType - The CLI agent type for this project.
 * @param req - The container creation request.
 * @returns The container ID and name.
 */
export async function createContainer(
  projectId: string,
  agentType: string,
  req: CreateContainerRequest,
): Promise<ContainerResult> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/container`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })

  return response.json() as Promise<ContainerResult>
}

/**
 * Deletes a project's container.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @param agentType - The CLI agent type for this project.
 * @returns The container ID and name.
 */
export async function deleteContainer(
  projectId: string,
  agentType: string,
): Promise<ContainerResult> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/container`, {
    method: 'DELETE',
  })
  return response.json() as Promise<ContainerResult>
}

/**
 * Fetches the editable configuration of a project's container.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @param agentType - The CLI agent type for this project.
 * @returns The container's configuration.
 */
export async function fetchContainerConfig(
  projectId: string,
  agentType: string,
): Promise<ContainerConfig> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/container/config`)
  return response.json() as Promise<ContainerConfig>
}

/**
 * Recreates a project's container with updated configuration.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @param agentType - The CLI agent type for this project.
 * @param req - The new container configuration.
 * @returns The new container ID and name.
 */
export async function updateContainer(
  projectId: string,
  agentType: string,
  req: CreateContainerRequest,
): Promise<ContainerResult> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/container`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  return response.json() as Promise<ContainerResult>
}

/**
 * Opens a directory in the host's file manager (Finder, Explorer, etc.).
 *
 * @param path - Absolute host path to reveal.
 */
export async function revealInFileManager(path: string): Promise<void> {
  await apiFetch('/api/v1/filesystem/reveal', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  })
}

/**
 * Lists filesystem entries at the given path for the browser.
 *
 * @param dirPath - Absolute path to list entries in.
 * @param includeFiles - When true, returns files alongside directories.
 * @returns An array of filesystem entries.
 */
export async function listDirectories(dirPath: string, includeFiles = false): Promise<DirEntry[]> {
  const params = new URLSearchParams({ path: dirPath })
  if (includeFiles) {
    params.set('mode', 'file')
  }
  const response = await apiFetch(`/api/v1/filesystem/directories?${params.toString()}`)
  return response.json() as Promise<DirEntry[]>
}

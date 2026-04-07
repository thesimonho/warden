/**
 * Project management API functions.
 *
 * @module
 */
import type {
  AddProjectResponse,
  AgentType,
  BatchProjectResponse,
  Project,
  ProjectResult,
} from '@/lib/types'
import { apiFetch, projectUrl } from './api-core'

/**
 * Fetches all projects from the API.
 *
 * @returns An array of projects.
 */
export async function fetchProjects(): Promise<Project[]> {
  const response = await apiFetch('/api/v1/projects')
  return response.json() as Promise<Project[]>
}

/**
 * Adds a project to the dashboard.
 *
 * For local projects, provide projectPath. For remote projects, provide cloneURL.
 *
 * @param name - The project name.
 * @param projectPath - Absolute host path for the project directory (local projects).
 * @param agentType - The CLI agent type for this project.
 * @param cloneURL - Git repository URL to clone (remote projects).
 * @param temporary - Whether the remote workspace is ephemeral.
 * @returns The project result.
 */
export async function addProject(
  name: string,
  projectPath: string,
  agentType: AgentType,
  cloneURL?: string,
  temporary?: boolean,
): Promise<AddProjectResponse> {
  const body: Record<string, unknown> = { name, agentType }
  if (cloneURL) {
    body.cloneURL = cloneURL
    if (temporary) body.temporary = true
  } else {
    body.projectPath = projectPath
  }
  const response = await apiFetch('/api/v1/projects', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return response.json() as Promise<AddProjectResponse>
}

/**
 * Removes a project from the dashboard.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @param agentType - The CLI agent type for this project.
 * @returns The project name.
 */
export async function removeProject(projectId: string, agentType: string): Promise<ProjectResult> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}`, {
    method: 'DELETE',
  })
  return response.json() as Promise<ProjectResult>
}

/**
 * Stops a running project.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @param agentType - The CLI agent type for this project.
 * @returns The project name and container ID.
 */
export async function stopProject(projectId: string, agentType: string): Promise<ProjectResult> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/stop`, { method: 'POST' })
  return response.json() as Promise<ProjectResult>
}

/**
 * Restarts a project.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @param agentType - The CLI agent type for this project.
 * @returns The project name and container ID.
 */
export async function restartProject(projectId: string, agentType: string): Promise<ProjectResult> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/restart`, { method: 'POST' })
  return response.json() as Promise<ProjectResult>
}

/**
 * Resets all cost history for a project.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @param agentType - The CLI agent type for this project.
 */
export async function resetProjectCosts(projectId: string, agentType: string): Promise<void> {
  await apiFetch(`${projectUrl(projectId, agentType)}/costs`, { method: 'DELETE' })
}

/**
 * Purges all audit events for a project.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @param agentType - The CLI agent type for this project.
 * @returns The number of deleted events.
 */
export async function purgeProjectAudit(
  projectId: string,
  agentType: string,
): Promise<{ deleted: number }> {
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/audit`, { method: 'DELETE' })
  return response.json() as Promise<{ deleted: number }>
}

/**
 * Performs a batch operation on multiple projects.
 *
 * @param action - The operation: "stop", "restart", or "delete".
 * @param projects - The list of project targets.
 */
export async function batchProjectOperation(
  action: 'stop' | 'restart' | 'delete',
  projects: Array<{ projectId: string; agentType: string }>,
): Promise<BatchProjectResponse> {
  const response = await apiFetch('/api/v1/projects/batch', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ action, projects }),
  })
  return response.json() as Promise<BatchProjectResponse>
}

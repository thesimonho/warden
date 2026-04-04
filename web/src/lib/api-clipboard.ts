/**
 * Clipboard API functions.
 *
 * @module
 */
import type { ClipboardUploadResponse } from '@/lib/types'
import { apiFetch, projectUrl } from './api-core'

/**
 * Uploads an image to the container's clipboard staging directory for the
 * xclip shim. Used by the terminal's image paste handler — call this before
 * sending Ctrl+V to the PTY so the agent's clipboard read picks up the image.
 *
 * @param projectId - The project whose container receives the image.
 * @param agentType - The CLI agent type for this project.
 * @param file - The image blob from the browser clipboard.
 * @returns The path where the image was staged inside the container.
 */
export async function uploadClipboardImage(
  projectId: string,
  agentType: string,
  file: Blob,
): Promise<string> {
  const form = new FormData()
  form.append('file', file, 'paste.png')
  const response = await apiFetch(`${projectUrl(projectId, agentType)}/clipboard`, {
    method: 'POST',
    body: form,
  })
  const body = (await response.json()) as ClipboardUploadResponse
  return body.path
}

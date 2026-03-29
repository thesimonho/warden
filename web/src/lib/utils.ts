import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/** Abbreviates /home/<user> and /Users/<user> prefixes to ~ for display. */
export function abbreviateHomePath(path: string): string {
  return path.replace(/^\/(?:home|Users)\/[^/]+/, '~')
}

/**
 * Returns the parent directory of an absolute POSIX path.
 * Strips a trailing slash before computing. Returns '/' at the root.
 */
export function parentDir(path: string): string {
  return path.replace(/\/[^/]+\/?$/, '') || '/'
}

/** Replaces a container home directory prefix with ~ for display. */
export function containerPathToDisplay(path: string, containerHomeDir: string): string {
  if (!containerHomeDir || !path) return path
  if (path === containerHomeDir) return '~'
  if (path.startsWith(containerHomeDir + '/')) return '~' + path.slice(containerHomeDir.length)
  return path
}

/** Expands a leading ~ to the absolute container home path. */
export function containerPathToAbsolute(input: string, containerHomeDir: string): string {
  if (!containerHomeDir || !input.startsWith('~')) return input
  if (input === '~') return containerHomeDir
  if (input.startsWith('~/')) return containerHomeDir + input.slice(1)
  return input
}

/** Returns a human-readable relative time string for a Unix timestamp in seconds. */
export function relativeTime(unixSeconds: number): string {
  const diffSeconds = Math.floor(Date.now() / 1000 - unixSeconds)
  if (diffSeconds < 60) return 'just now'
  const diffMinutes = Math.floor(diffSeconds / 60)
  if (diffMinutes < 60) return `${diffMinutes}m ago`
  const diffHours = Math.floor(diffMinutes / 60)
  if (diffHours < 24) return `${diffHours}h ago`
  const diffDays = Math.floor(diffHours / 24)
  return `${diffDays}d ago`
}

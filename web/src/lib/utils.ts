import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Extracts owner/repo from a git clone URL.
 * Handles HTTPS (`https://github.com/owner/repo.git`) and
 * SSH (`git@github.com:owner/repo.git`) formats.
 * Falls back to the full URL if parsing fails.
 */
export function shortenCloneURL(url: string): string {
  const match = url.match(/(?:[/:])([^/:]+\/[^/.]+?)(?:\.git)?$/)
  return match ? match[1] : url
}

/**
 * Abbreviates /home/<user> and /Users/<user> prefixes to ~ for display.
 * Also handles Docker Desktop's /host_mnt prefix (e.g. /host_mnt/home/simon/...).
 */
export function abbreviateHomePath(path: string): string {
  return path.replace(/^(?:\/host_mnt)?\/(?:home|Users)\/[^/]+/, '~')
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
  if (path.startsWith(`${containerHomeDir}/`)) return `~${path.slice(containerHomeDir.length)}`
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

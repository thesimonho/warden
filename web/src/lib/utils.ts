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

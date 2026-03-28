/**
 * Formats a dollar amount for display.
 *
 * @param dollars - The amount in USD.
 * @returns Formatted string like "$0.42".
 */
export function formatCost(dollars: number): string {
  return `$${dollars.toFixed(2)}`
}

/**
 * Formats a duration in milliseconds to a human-readable string.
 *
 * @param ms - Duration in milliseconds.
 * @returns Formatted string like "2m 30s" or "1h 15m".
 */
export function formatDuration(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60

  if (hours > 0) {
    return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`
  }
  if (minutes > 0) {
    return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`
  }
  return `${seconds}s`
}

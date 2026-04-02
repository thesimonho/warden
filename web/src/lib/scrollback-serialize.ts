/**
 * Terminal scrollback serialization helpers.
 *
 * Wraps @xterm/addon-serialize to capture and restore the full xterm.js buffer
 * (including colors, bold, cursor position, and other SGR attributes) as a
 * single ANSI escape sequence string that can be written back to a fresh
 * Terminal instance via `terminal.write()`.
 *
 * @module
 */
import type { Terminal } from '@xterm/xterm'
import type { SerializeAddon } from '@xterm/addon-serialize'
import type { ScrollbackEntry } from '@/lib/scrollback-db'

/**
 * Builds the IndexedDB key for a terminal's scrollback buffer.
 *
 * Format: `${projectId}:${worktreeId}`. The projectId is a deterministic
 * 12-char hex hash of the host path, so the key survives container recreation.
 */
export function scrollbackKey(projectId: string, worktreeId: string): string {
  return `${projectId}:${worktreeId}`
}

/**
 * Serializes the terminal's full buffer (visible + scrollback) as an ANSI string.
 *
 * @returns The serialized ANSI string, or null if the buffer is empty.
 */
export function serializeTerminal(
  terminal: Terminal,
  serializeAddon: SerializeAddon,
): string | null {
  const data = serializeAddon.serialize({
    scrollback: terminal.options.scrollback ?? 0,
  })
  if (!data || data.length === 0) return null
  return data
}

/**
 * Restores a saved scrollback buffer into a terminal instance.
 *
 * Writes the saved ANSI string via `terminal.write()`. If the saved column
 * count differs from the current terminal, xterm.js reflows the content
 * automatically — the same behavior as resizing a terminal with history.
 */
export function restoreScrollback(terminal: Terminal, entry: ScrollbackEntry): void {
  terminal.write(entry.data)
}

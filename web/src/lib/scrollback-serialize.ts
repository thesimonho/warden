/**
 * Terminal scrollback serialization helper.
 *
 * Wraps @xterm/addon-serialize to capture the full xterm.js buffer (including
 * colors, bold, cursor position, and other SGR attributes) as a single ANSI
 * escape sequence string that can be written back to a fresh Terminal instance
 * via `terminal.write()`.
 *
 * @module
 */
import type { Terminal } from '@xterm/xterm'
import type { SerializeAddon } from '@xterm/addon-serialize'

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
  return data || null
}

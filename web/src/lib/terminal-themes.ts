import type { ITheme } from '@xterm/xterm'

/** Reads a CSS custom property from the document root as a resolved value. */
function cssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

/**
 * Builds an xterm.js theme from the active CSS custom properties.
 *
 * Terminal ANSI colors are defined in the theme CSS files (permafrost.css,
 * frostpunk.css) as `--terminal-*` variables. This keeps color palettes
 * in one place and automatically follows light/dark mode switches.
 */
export function getTerminalTheme(): ITheme {
  return {
    background: cssVar('--terminal-background'),
    foreground: cssVar('--terminal-foreground'),
    cursor: cssVar('--terminal-cursor'),
    selectionBackground: cssVar('--terminal-selection'),
    black: cssVar('--terminal-black'),
    red: cssVar('--terminal-red'),
    green: cssVar('--terminal-green'),
    yellow: cssVar('--terminal-yellow'),
    blue: cssVar('--terminal-blue'),
    magenta: cssVar('--terminal-magenta'),
    cyan: cssVar('--terminal-cyan'),
    white: cssVar('--terminal-white'),
    brightBlack: cssVar('--terminal-bright-black'),
    brightRed: cssVar('--terminal-bright-red'),
    brightGreen: cssVar('--terminal-bright-green'),
    brightYellow: cssVar('--terminal-bright-yellow'),
    brightBlue: cssVar('--terminal-bright-blue'),
    brightMagenta: cssVar('--terminal-bright-magenta'),
    brightCyan: cssVar('--terminal-bright-cyan'),
    brightWhite: cssVar('--terminal-bright-white'),
  }
}

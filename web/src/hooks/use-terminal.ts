/**
 * Terminal WebSocket client — reference implementation for browser-based PTY access.
 *
 * If you're building a web terminal that connects to a Warden worktree, copy
 * the patterns in this file. Key patterns:
 *
 * - **WebSocket protocol**: Connect to `GET /api/v1/projects/{id}/ws/{wid}`.
 *   The socket carries two frame types:
 *   - **Binary frames** — raw PTY I/O. Terminal input (keystrokes, paste) is
 *     sent as binary; terminal output (ANSI escape sequences) arrives as binary.
 *   - **Text frames** — JSON control messages. Currently only resize:
 *     `{ "type": "resize", "cols": 80, "rows": 24 }`. Send this whenever the
 *     terminal container dimensions change.
 *
 * - **Reconnection with backoff**: On close, reconnects with exponential
 *   backoff (1s → 2s → ... → 16s), up to 5 attempts. After that, status
 *   goes to `error`. The attempt counter resets on successful connection.
 *
 * - **Resize protocol**: A `ResizeObserver` watches the terminal container.
 *   On resize, we debounce (100ms), call `fitAddon.fit()` to recalculate
 *   cols/rows, then send a resize text frame to the backend. The backend
 *   forwards this to the PTY via `pty.Setsize()`.
 *
 * - **Font loading**: We wait for `document.fonts.ready` before opening the
 *   terminal so xterm.js caches correct glyph metrics. Without this, the
 *   first render may have misaligned text if the font loads late.
 *
 * For the REST API, see `api.ts`.
 * For SSE events, see `use-event-source.ts`.
 *
 * @module
 */
import { useEffect, useRef, useCallback, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SerializeAddon } from '@xterm/addon-serialize'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { getTerminalTheme } from '@/lib/terminal-themes'
import { useTerminalClipboard } from '@/hooks/use-terminal-clipboard'
import {
  saveScrollback,
  loadScrollback,
  deleteScrollback,
  scrollbackKey,
} from '@/lib/scrollback-db'
import { fetchWorktrees } from '@/lib/api'
import { serializeTerminal } from '@/lib/scrollback-serialize'
import '@fontsource/jetbrains-mono/400.css'
import '@fontsource/jetbrains-mono/600.css'

/** Connection state of the terminal WebSocket. */
export type TerminalStatus = 'connecting' | 'connected' | 'disconnected' | 'error'

/** Options for the useTerminal hook. */
interface UseTerminalOptions {
  /** Container ID (12 or 64 char hex). */
  projectId: string
  /** CLI agent type for this project. */
  agentType: string
  /** Worktree identifier. */
  worktreeId: string
  /** Whether the terminal should be active (connected). */
  isActive: boolean
  /**
   * Whether this terminal has user focus. Focused terminals flush output
   * every animation frame (~16ms); unfocused terminals flush on a slower
   * interval to reduce rendering load when many panels are visible.
   */
  isFocused?: boolean
}

/** Terminal font family with local fallbacks. */
const TERMINAL_FONT_FAMILY =
  "'JetBrains Mono', 'Cascadia Code', 'Fira Code', 'SF Mono', Menlo, Consolas, monospace"

/**
 * Waits for JetBrains Mono to be ready (loaded via Fontsource CSS imports above)
 * so xterm caches correct glyph metrics on first render.
 * Falls back gracefully — the terminal will use the next available font in the stack.
 */
const fontReady = document.fonts.ready

/** Shared encoder for terminal input — avoids allocation per keystroke. */
const textEncoder = new TextEncoder()

/** Debounce delay for resize events (ms). */
const RESIZE_DEBOUNCE_MS = 100

/** Flush interval for unfocused terminals — trades latency for lower CPU. */
const UNFOCUSED_FLUSH_MS = 200

/** Reconnect backoff: base delay doubles each attempt, capped at max. */
const RECONNECT_BASE_MS = 1000
const RECONNECT_MAX_MS = 16000
const MAX_RECONNECT_ATTEMPTS = 5

/**
 * Scrollback lines to retain in the xterm.js buffer. 10k lines keeps serialized
 * buffer sizes reasonable for IndexedDB (~1-5 MB) while capturing enough history.
 */
const SCROLLBACK_LINES = 10_000

/**
 * Builds the WebSocket URL for a terminal connection.
 *
 * In development, Vite proxies /api/* (including WS upgrades) to the Go
 * backend, so we use the current host. In production, the Go binary serves
 * both the SPA and the WebSocket endpoint on the same origin.
 */
function buildWSUrl(projectId: string, agentType: string, worktreeId: string): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/api/v1/projects/${projectId}/${agentType}/ws/${worktreeId}`
}

/**
 * Manages an xterm.js terminal instance and its WebSocket connection to the
 * Go backend's PTY proxy.
 *
 * Attaches the terminal to the provided container ref, handles resize via
 * FitAddon + ResizeObserver, and reconnects on connection loss with
 * exponential backoff.
 *
 * @returns containerRef to attach to the terminal wrapper div, status, and detach method.
 */
export function useTerminal({
  projectId,
  agentType,
  worktreeId,
  isActive,
  isFocused = true,
}: UseTerminalOptions) {
  const containerRef = useRef<HTMLDivElement>(null)
  const terminalRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const serializeAddonRef = useRef<SerializeAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectAttemptRef = useRef(0)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const resizeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const observerRef = useRef<ResizeObserver | null>(null)
  const themeObserverRef = useRef<MutationObserver | null>(null)
  const isDisposedRef = useRef(false)
  const writeBufferRef = useRef<Uint8Array[]>([])
  const flushRafRef = useRef(0)
  const throttleTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const isFocusedRef = useRef(isFocused)
  const [status, setStatus] = useState<TerminalStatus>('disconnected')

  const clipboard = useTerminalClipboard({ terminalRef, wsRef, projectId, agentType })

  // Keep focus ref in sync so the message handler (closure) sees changes.
  isFocusedRef.current = isFocused

  /** Sends a resize control message to the backend. */
  const sendResize = useCallback((cols: number, rows: number) => {
    const ws = wsRef.current
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols, rows }))
    }
  }, [])

  /**
   * Flushes buffered WebSocket output to xterm.js in a single write.
   *
   * Incoming messages are collected in writeBufferRef and flushed once per
   * animation frame. This prevents rapid output bursts (common when Claude
   * is generating) from blocking the main thread and pausing user input.
   */
  const flushWriteBuffer = useCallback(() => {
    flushRafRef.current = 0
    const terminal = terminalRef.current
    const chunks = writeBufferRef.current
    if (!terminal || chunks.length === 0) return

    // Fast path: skip allocation when only one chunk arrived this frame.
    if (chunks.length === 1) {
      terminal.write(chunks[0])
    } else {
      let totalLength = 0
      for (const chunk of chunks) {
        totalLength += chunk.byteLength
      }
      const merged = new Uint8Array(totalLength)
      let offset = 0
      for (const chunk of chunks) {
        merged.set(chunk, offset)
        offset += chunk.byteLength
      }
      terminal.write(merged)
    }
    writeBufferRef.current = []
  }, [])

  /** Fits the terminal to its container and notifies the backend. */
  const fit = useCallback(() => {
    const fitAddon = fitAddonRef.current
    const terminal = terminalRef.current
    const container = containerRef.current
    if (!fitAddon || !terminal || !container) return

    // Skip fitting when the container is hidden (display:none) — the
    // ResizeObserver fires with 0×0 dimensions which corrupts xterm's
    // column count and produces a broken narrow layout.
    if (container.offsetWidth === 0 || container.offsetHeight === 0) return

    try {
      fitAddon.fit()
      sendResize(terminal.cols, terminal.rows)
    } catch {
      // FitAddon throws if the terminal isn't attached to the DOM yet.
    }
  }, [sendResize])

  /** Connects the WebSocket and bridges it to the xterm.js instance. */
  const connect = useCallback(() => {
    const terminal = terminalRef.current
    if (!terminal || isDisposedRef.current) return

    const url = buildWSUrl(projectId, agentType, worktreeId)
    const ws = new WebSocket(url)
    ws.binaryType = 'arraybuffer'
    wsRef.current = ws
    setStatus('connecting')

    ws.addEventListener('open', () => {
      if (isDisposedRef.current) {
        ws.close()
        return
      }
      reconnectAttemptRef.current = 0
      setStatus('connected')

      // Send initial resize so the PTY matches the terminal dimensions.
      requestAnimationFrame(() => fit())
    })

    ws.addEventListener('message', (event) => {
      if (event.data instanceof ArrayBuffer) {
        writeBufferRef.current.push(new Uint8Array(event.data))
      } else if (typeof event.data === 'string') {
        writeBufferRef.current.push(textEncoder.encode(event.data))
      }
      // Focused terminals flush every animation frame for low latency.
      // Unfocused terminals are flushed by a slower setInterval instead.
      if (isFocusedRef.current && !flushRafRef.current) {
        flushRafRef.current = requestAnimationFrame(flushWriteBuffer)
      }
    })

    ws.addEventListener('close', () => {
      if (isDisposedRef.current) return

      setStatus('disconnected')

      // Reconnect with exponential backoff.
      if (reconnectAttemptRef.current < MAX_RECONNECT_ATTEMPTS) {
        const delay = Math.min(
          RECONNECT_BASE_MS * 2 ** reconnectAttemptRef.current,
          RECONNECT_MAX_MS,
        )
        reconnectAttemptRef.current += 1
        reconnectTimerRef.current = setTimeout(() => {
          if (!isDisposedRef.current) connect()
        }, delay)
      } else {
        setStatus('error')
      }
    })

    ws.addEventListener('error', () => {
      // The close event fires after error, which handles reconnection.
    })

    // Terminal input → WebSocket binary frames.
    const inputDisposable = terminal.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(textEncoder.encode(data))
      }
    })

    // Terminal binary input (paste) → WebSocket binary frames.
    const binaryDisposable = terminal.onBinary((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(textEncoder.encode(data))
      }
    })

    // Clean up xterm listeners when WebSocket closes.
    ws.addEventListener(
      'close',
      () => {
        inputDisposable.dispose()
        binaryDisposable.dispose()
      },
      { once: true },
    )
  }, [projectId, agentType, worktreeId, fit, flushWriteBuffer])

  /** Tears down the WebSocket and terminal, preventing reconnection. */
  const detach = useCallback(() => {
    isDisposedRef.current = true

    if (flushRafRef.current) {
      cancelAnimationFrame(flushRafRef.current)
      flushRafRef.current = 0
    }
    if (throttleTimerRef.current) {
      clearInterval(throttleTimerRef.current)
      throttleTimerRef.current = null
    }
    writeBufferRef.current = []

    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
    if (resizeTimerRef.current) {
      clearTimeout(resizeTimerRef.current)
      resizeTimerRef.current = null
    }
    if (observerRef.current) {
      observerRef.current.disconnect()
      observerRef.current = null
    }
    if (themeObserverRef.current) {
      themeObserverRef.current.disconnect()
      themeObserverRef.current = null
    }

    const ws = wsRef.current
    if (ws) {
      ws.close()
      wsRef.current = null
    }

    // Serialize the scrollback buffer before disposing the terminal.
    // Fire-and-forget — detach must be synchronous (React cleanup).
    const terminal = terminalRef.current
    const serializeAddon = serializeAddonRef.current
    if (terminal && serializeAddon) {
      try {
        const data = serializeTerminal(terminal, serializeAddon)
        if (data) {
          saveScrollback(projectId, worktreeId, data, terminal.cols, terminal.rows)
        }
      } catch {
        // Serialization failure is not critical.
      }
    }

    if (terminal) {
      terminal.dispose()
      terminalRef.current = null
    }

    fitAddonRef.current = null
    serializeAddonRef.current = null
    setStatus('disconnected')
  }, [projectId, worktreeId])

  // Switch flush strategy when focus changes.
  // Focused: cancel interval, let RAF handle it (scheduled per-message).
  // Unfocused: cancel pending RAF, start a periodic interval.
  useEffect(() => {
    if (!isActive) return

    if (isFocused) {
      if (throttleTimerRef.current) {
        clearInterval(throttleTimerRef.current)
        throttleTimerRef.current = null
      }
      // Drain any chunks that arrived while on the interval.
      if (writeBufferRef.current.length > 0 && !flushRafRef.current) {
        flushRafRef.current = requestAnimationFrame(flushWriteBuffer)
      }
    } else {
      if (flushRafRef.current) {
        cancelAnimationFrame(flushRafRef.current)
        flushRafRef.current = 0
      }
      throttleTimerRef.current = setInterval(flushWriteBuffer, UNFOCUSED_FLUSH_MS)
    }

    return () => {
      if (throttleTimerRef.current) {
        clearInterval(throttleTimerRef.current)
        throttleTimerRef.current = null
      }
    }
  }, [isFocused, isActive, flushWriteBuffer])

  /** Gives keyboard focus to the xterm.js instance. */
  const focus = useCallback(() => {
    terminalRef.current?.focus()
  }, [])

  // Main lifecycle effect: create terminal, connect WS, observe resizes.
  useEffect(() => {
    if (!isActive || !containerRef.current) return

    isDisposedRef.current = false
    reconnectAttemptRef.current = 0
    const container = containerRef.current

    // Local flag scoped to this effect invocation. Unlike isDisposedRef
    // (shared across runs), this stays true once cleanup fires — preventing
    // stale fontReady callbacks from operating on a disposed terminal
    // (e.g. during React Strict Mode double-mount).
    let effectCancelled = false

    // Create xterm.js instance.
    const terminal = new Terminal({
      allowProposedApi: true,
      cursorBlink: false,
      fontFamily: TERMINAL_FONT_FAMILY,
      fontSize: 12,
      fontWeight: '400',
      fontWeightBold: '600',
      scrollback: SCROLLBACK_LINES,
      smoothScrollDuration: 0,
      rescaleOverlappingGlyphs: false,
      theme: getTerminalTheme(),
    })

    const fitAddon = new FitAddon()
    terminal.loadAddon(fitAddon)
    terminal.loadAddon(new WebLinksAddon())

    const serializeAddon = new SerializeAddon()
    terminal.loadAddon(serializeAddon)

    const unicodeAddon = new Unicode11Addon()
    terminal.loadAddon(unicodeAddon)
    terminal.unicode.activeVersion = '11'

    terminalRef.current = terminal
    fitAddonRef.current = fitAddon
    serializeAddonRef.current = serializeAddon

    // Wait for JetBrains Mono to load before opening, so xterm caches
    // correct glyph metrics on first render (no FOUT or misaligned text).
    fontReady.then(() => {
      if (effectCancelled) return

      // Attach to the DOM — must happen before loading WebGL addon.
      terminal.open(container)

      // DOM renderer (default) — no WebGL or canvas addon loaded.
      // CSS zoom on the canvas surface requires DOM rendering: the browser
      // re-rasterizes real HTML text at the zoomed resolution. WebGL/canvas
      // renderers rasterize at a fixed DPR and fight the zoom, keeping text
      // the same size regardless of zoom level.

      // Scrollback: The Go proxy (internal/terminal/altscreen.go) strips
      // alternate screen escape sequences so Claude Code renders in the
      // normal buffer and xterm.js scrollback works.
      //
      // TODO: Mouse wheel scrolling in Claude Code's TUI doesn't work.
      // Claude Code enables mouse reporting, so xterm.js forwards wheel
      // events as mouse escape sequences (button 64/65), which Claude
      // interprets as input history cycling. Native terminals handle this
      // at a lower level that xterm.js doesn't expose. This is a known
      // xterm.js limitation (also affects VS Code's terminal).
      // See: https://github.com/xtermjs/xterm.js/issues/5194
      // See: https://github.com/anthropics/claude-code/issues/23581

      // OSC 52 clipboard support: applications inside the container can
      // write to the browser clipboard via the OSC 52 escape sequence.
      // Claude Code uses this in fullscreen mode (CLAUDE_CODE_NO_FLICKER=1)
      // for copy operations. Format: ESC ] 52 ; <sel> ; <base64> ST
      clipboard.registerOsc52Handler(terminal)

      // Clipboard keybindings: Ctrl+Shift+C copies selected text,
      // Ctrl+Shift+V pastes from clipboard. Plain Ctrl+C still sends ^C
      // (SIGINT) to the PTY as expected.
      terminal.attachCustomKeyEventHandler((event) => {
        if (event.type !== 'keydown') return true

        // Shift+Enter: send CSI u escape sequence so Claude Code can
        // distinguish it from plain Enter. xterm.js normally sends the
        // same CR byte for both, which breaks the "shift+enter = newline"
        // keybinding. The CSI u sequence (\e[13;2u) is the standard
        // Kitty keyboard protocol encoding for Shift+Enter.
        if (
          event.key === 'Enter' &&
          event.shiftKey &&
          !event.ctrlKey &&
          !event.altKey &&
          !event.metaKey
        ) {
          const socket = wsRef.current
          if (socket?.readyState === WebSocket.OPEN) {
            socket.send(textEncoder.encode('\x1b[13;2u'))
          }
          event.preventDefault()
          return false
        }

        const isCtrlShift = event.ctrlKey && event.shiftKey && !event.altKey && !event.metaKey

        if (isCtrlShift && event.key === 'C') {
          clipboard.copySelection()
          event.preventDefault()
          return false
        }

        if (isCtrlShift && event.key === 'V') {
          clipboard.pasteText()
          event.preventDefault()
          return false
        }

        // Plain Ctrl+V: check for image data on clipboard. If found,
        // upload to the container's xclip staging dir, then send Ctrl+V
        // so the agent reads it via the shim. For text-only clipboard,
        // send the raw Ctrl+V byte so the agent handles it.
        if (
          event.key === 'v' &&
          event.ctrlKey &&
          !event.shiftKey &&
          !event.altKey &&
          !event.metaKey
        ) {
          clipboard.pasteImageFromClipboard().then((handled) => {
            if (!handled) clipboard.sendToPty('\x16')
          })
          event.preventDefault()
          return false
        }

        return true
      })

      // Initial fit after the terminal is rendered.
      requestAnimationFrame(() => {
        if (!effectCancelled) {
          try {
            fitAddon.fit()
          } catch {
            // Ignore if not yet visible.
          }
        }
      })

      // Restore saved scrollback before connecting the WebSocket so
      // abduco's visible-screen replay appends after the historical content.
      // Fetch worktree state in parallel to check if the agent has exited —
      // stale scrollback from a finished session should not be restored.
      const sbKey = scrollbackKey(projectId, worktreeId)
      Promise.all([loadScrollback(sbKey), fetchWorktrees(projectId, agentType).catch(() => [])])
        .then(([entry, worktrees]) => {
          if (effectCancelled) return
          const wt = worktrees.find((w) => w.id === worktreeId)
          const hasAgentExited = wt != null && wt.exitCode != null
          if (entry && !hasAgentExited) {
            terminal.write(entry.data)
          } else if (entry) {
            deleteScrollback(sbKey)
          }
          connect()
        })
        .catch(() => {
          if (!effectCancelled) connect()
        })

      // Observe container resizes to refit the terminal.
      const resizeObserver = new ResizeObserver(() => {
        if (resizeTimerRef.current) clearTimeout(resizeTimerRef.current)
        resizeTimerRef.current = setTimeout(() => {
          if (!isDisposedRef.current) fit()
        }, RESIZE_DEBOUNCE_MS)
      })
      resizeObserver.observe(container)
      observerRef.current = resizeObserver

      // Sync terminal palette when the app theme changes (dark ↔ light).
      const themeObserver = new MutationObserver(() => {
        if (!isDisposedRef.current) {
          terminal.options.theme = getTerminalTheme()
        }
      })
      themeObserver.observe(document.documentElement, {
        attributes: true,
        attributeFilter: ['class'],
      })
      themeObserverRef.current = themeObserver
    })

    return () => {
      effectCancelled = true
      detach()
    }
    // We exclude `connect`, `fit`, and `detach` from deps. `connect` and `fit`
    // are stable (empty deps). `detach` depends on [projectId, worktreeId] which
    // are already in this effect's deps, so changes are covered without listing it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isActive, projectId, agentType, worktreeId])

  return {
    containerRef,
    status,
    detach,
    focus,
    fit,
    clipboard,
  }
}

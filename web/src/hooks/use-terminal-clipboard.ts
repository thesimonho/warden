/**
 * Terminal clipboard operations — reference implementation for web terminal
 * copy/paste including OSC 52 and image paste via the xclip shim.
 *
 * This hook encapsulates all clipboard interactions for a terminal instance:
 *
 * - **Text copy** via xterm.js selection or OSC 52 (for agent fullscreen mode)
 * - **Text paste** from the browser clipboard with trailing newline stripping
 * - **Image paste** via the clipboard upload API and xclip shim
 * - **Select all** with automatic copy to clipboard
 * - **OSC 52 registration** for bidirectional clipboard between browser and PTY
 *
 * The hook is designed to be composed with `useTerminal` — it needs refs to
 * the terminal instance, WebSocket, and the project ID for image uploads.
 *
 * @module
 */
import { useCallback, useRef } from 'react'
import type { Terminal } from '@xterm/xterm'
import { toast } from 'sonner'
import { uploadClipboardImage } from '@/lib/api'

/** Shared encoder — avoids allocation per clipboard operation. */
const textEncoder = new TextEncoder()

/** CSI u encoding for Ctrl+Shift+C (codepoint 99, modifier 6 = ctrl+shift+1). */
const CSI_CTRL_SHIFT_C = '\x1b[99;6u'

/** Raw Ctrl+V byte (0x16 / SYN). */
const CTRL_V_BYTE = '\x16'

/**
 * Tracks whether the next OSC 52 clipboard write was triggered by an
 * explicit Ctrl+Shift+C (so we show a toast only for that case, not
 * for copy-on-select which fires on every mouse-up in fullscreen mode).
 */
interface ExplicitCopyTracker {
  mark: () => void
  consume: () => boolean
}

function createExplicitCopyTracker(): ExplicitCopyTracker {
  let pending = false
  let timer: ReturnType<typeof setTimeout> | null = null

  return {
    mark() {
      pending = true
      if (timer) clearTimeout(timer)
      timer = setTimeout(() => {
        pending = false
        timer = null
      }, 500)
    },
    consume() {
      const was = pending
      pending = false
      if (timer) {
        clearTimeout(timer)
        timer = null
      }
      return was
    },
  }
}

/** Refs needed by the clipboard hook from the parent terminal hook. */
interface TerminalClipboardDeps {
  terminalRef: React.RefObject<Terminal | null>
  wsRef: React.RefObject<WebSocket | null>
  projectId: string
}

/**
 * Provides clipboard operations for a terminal instance. Each instance
 * gets its own explicit-copy tracker so multi-terminal layouts don't
 * interfere with each other's toast notifications.
 */
export function useTerminalClipboard({ terminalRef, wsRef, projectId }: TerminalClipboardDeps) {
  const copyTrackerRef = useRef(createExplicitCopyTracker())

  /** Sends raw bytes to the PTY via the WebSocket. */
  const sendToPty = useCallback(
    (data: string) => {
      const ws = wsRef.current
      if (ws?.readyState === WebSocket.OPEN) {
        ws.send(textEncoder.encode(data))
      }
    },
    [wsRef],
  )

  /**
   * Copies the current selection to clipboard. If xterm.js has a selection,
   * copies it directly. Otherwise sends Ctrl+Shift+C to the PTY so the
   * agent (e.g. Claude Code fullscreen mode) can handle it via OSC 52.
   */
  const copySelection = useCallback(() => {
    const terminal = terminalRef.current
    if (!terminal) return
    const selection = terminal.getSelection()
    if (selection) {
      navigator.clipboard.writeText(selection).then(() => {
        toast.success('Copied to clipboard')
      })
    } else {
      sendToPty(CSI_CTRL_SHIFT_C)
      copyTrackerRef.current.mark()
    }
  }, [terminalRef, sendToPty])

  /** Pastes text from the browser clipboard into the terminal. */
  const pasteText = useCallback(() => {
    navigator.clipboard
      .readText()
      .then((text) => {
        const trimmed = text?.replace(/[\r\n]+$/, '')
        if (trimmed) terminalRef.current?.paste(trimmed)
      })
      .catch(() => {})
  }, [terminalRef])

  /**
   * Uploads a clipboard image to the container's xclip staging directory,
   * then sends Ctrl+V to the PTY so the agent reads it via the xclip shim.
   */
  const pasteImage = useCallback(
    async (blob: Blob) => {
      try {
        await uploadClipboardImage(projectId, blob)
        sendToPty(CTRL_V_BYTE)
      } catch {
        toast.error('Failed to upload image')
      }
    },
    [projectId, sendToPty],
  )

  /**
   * Reads image data from the browser clipboard and uploads it to the
   * container. Used by both the Ctrl+V key handler and context menu.
   *
   * @returns true if an image was found and uploaded.
   */
  const pasteImageFromClipboard = useCallback(async (): Promise<boolean> => {
    try {
      const items = await navigator.clipboard.read()
      for (const item of items) {
        const imageType = item.types.find((t) => t.startsWith('image/'))
        if (imageType) {
          const blob = await item.getType(imageType)
          await pasteImage(blob)
          return true
        }
      }
    } catch {
      // Clipboard read denied or failed.
    }
    return false
  }, [pasteImage])

  /** Selects all text in the xterm.js scrollback buffer and copies it. */
  const selectAll = useCallback(() => {
    const terminal = terminalRef.current
    if (!terminal) return
    terminal.selectAll()
    const selection = terminal.getSelection()
    if (selection) {
      // xterm.js pads each line with trailing spaces to fill the terminal
      // width — strip them so the copied text is clean.
      const trimmed = selection
        .split('\n')
        .map((line) => line.trimEnd())
        .join('\n')
        .trimEnd()
      navigator.clipboard.writeText(trimmed).then(() => {
        terminal.clearSelection()
        toast.success('Copied all to clipboard')
      })
    }
  }, [terminalRef])

  /**
   * Registers an OSC 52 handler on the terminal for bidirectional clipboard
   * access. Call this once after the terminal is opened.
   *
   * OSC 52 format: `ESC ] 52 ; <sel> ; <base64> ST`
   * - Write (base64 payload): decodes and writes to browser clipboard
   * - Read (`?` payload): reads browser clipboard and responds via PTY
   */
  const registerOsc52Handler = useCallback(
    (terminal: Terminal) => {
      const tracker = copyTrackerRef.current

      terminal.parser.registerOscHandler(52, (data) => {
        const semicolonIdx = data.indexOf(';')
        if (semicolonIdx === -1) return false

        const selection = data.slice(0, semicolonIdx)
        const payload = data.slice(semicolonIdx + 1)

        if (payload === '?') {
          navigator.clipboard
            .readText()
            .then((text) => {
              const encoded = btoa(text)
              sendToPty(`\x1b]52;${selection};${encoded}\x1b\\`)
            })
            .catch(() => {})
          return true
        }

        if (payload === '') {
          navigator.clipboard.writeText('').catch(() => {})
          return true
        }

        const isExplicit = tracker.consume()
        try {
          const text = atob(payload)
          navigator.clipboard
            .writeText(text)
            .then(() => {
              if (isExplicit) toast.success('Copied to clipboard')
            })
            .catch(() => {})
        } catch {
          // Invalid base64.
        }
        return true
      })
    },
    [sendToPty],
  )

  return {
    copySelection,
    pasteText,
    pasteImageFromClipboard,
    selectAll,
    registerOsc52Handler,
    /** Sends raw bytes to the PTY. Exposed for key handlers that need PTY fallback. */
    sendToPty,
  }
}

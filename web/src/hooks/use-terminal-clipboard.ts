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

import type { Terminal } from '@xterm/xterm'
import { useCallback, useRef } from 'react'
import { toast } from 'sonner'

import { uploadClipboardImage } from '@/lib/api'

/** Shared encoder — avoids allocation per clipboard operation. */
const textEncoder = new TextEncoder()

/**
 * Converts an image blob to PNG using an offscreen canvas. Both Claude Code
 * and Codex request `image/png` from xclip, so all staged images must be PNG
 * regardless of the source format (JPEG, WebP, GIF, BMP, etc.).
 */
function convertToPng(blob: Blob): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(blob)
    const img = new Image()
    img.onload = () => {
      URL.revokeObjectURL(url)
      const canvas = document.createElement('canvas')
      canvas.width = img.naturalWidth
      canvas.height = img.naturalHeight
      const ctx = canvas.getContext('2d')
      if (!ctx) {
        reject(new Error('Failed to get canvas context'))
        return
      }
      ctx.drawImage(img, 0, 0)
      canvas.toBlob(
        (png) => (png ? resolve(png) : reject(new Error('PNG conversion failed'))),
        'image/png',
      )
    }
    img.onerror = () => {
      URL.revokeObjectURL(url)
      reject(new Error('Failed to load image for conversion'))
    }
    img.src = url
  })
}

/** CSI u encoding for Ctrl+Shift+C (codepoint 99, modifier 6 = ctrl+shift+1). */
const CSI_CTRL_SHIFT_C = '\x1b[99;6u'

/**
 * Writes text to the browser clipboard, retrying once on focus regain if the
 * initial attempt fails because the document is not focused (e.g. devtools
 * has focus). Returns true on success.
 */
async function writeClipboardText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    // "Document is not focused" — wait for focus and retry once.
    if (!document.hasFocus()) {
      return new Promise((resolve) => {
        const onFocus = () => {
          window.removeEventListener('focus', onFocus)
          navigator.clipboard.writeText(text).then(
            () => resolve(true),
            () => resolve(false),
          )
        }
        window.addEventListener('focus', onFocus, { once: true })
        // Give up after 5s if focus never returns.
        setTimeout(() => {
          window.removeEventListener('focus', onFocus)
          resolve(false)
        }, 5000)
      })
    }
    return false
  }
}

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
  agentType: string
}

/**
 * Provides clipboard operations for a terminal instance. Each instance
 * gets its own explicit-copy tracker so multi-terminal layouts don't
 * interfere with each other's toast notifications.
 */
export function useTerminalClipboard({
  terminalRef,
  wsRef,
  projectId,
  agentType,
}: TerminalClipboardDeps) {
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
   * Copies the current selection to clipboard via one of two paths:
   *
   * 1. **xterm.js selection exists** (bare bash, auth screen) — copy directly
   *    from the terminal buffer. Fast, local, no round-trip.
   *
   * 2. **No xterm.js selection** (Claude Code chat with mouse reporting) —
   *    send Ctrl+Shift+C keystroke to the PTY. The agent copies its own
   *    TUI selection and responds with an OSC 52 escape sequence, which
   *    the registered OSC 52 handler writes to the browser clipboard.
   */
  const copySelection = useCallback(() => {
    const terminal = terminalRef.current
    if (!terminal) return
    const selection = terminal.getSelection()
    if (selection) {
      writeClipboardText(selection).then((ok) => {
        if (ok) {
          toast.success('Copied to clipboard')
        } else {
          toast.error('Copy failed — clipboard access denied')
        }
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
   * then triggers the agent to read it. Non-PNG images are converted to PNG
   * first since both Claude Code and Codex expect PNG.
   *
   * Agent-specific behavior:
   * - **Claude Code**: sends Ctrl+V so the agent reads via the xclip shim
   * - **Codex**: pastes the staged file path as text, since Codex uses
   *   arboard (native X11/Wayland) instead of xclip and the container has
   *   no display server. Codex's `normalize_pasted_path` picks up the path.
   */
  const pasteImage = useCallback(
    async (blob: Blob) => {
      try {
        const png = blob.type === 'image/png' ? blob : await convertToPng(blob)
        const stagedPath = await uploadClipboardImage(projectId, agentType, png)
        if (agentType === 'codex') {
          terminalRef.current?.paste(stagedPath)
        } else {
          sendToPty(CTRL_V_BYTE)
        }
      } catch {
        toast.error('Failed to upload image')
      }
    },
    [projectId, agentType, sendToPty, terminalRef],
  )

  /**
   * Handles a native paste event, checking clipboardData for image content.
   * More reliable than `navigator.clipboard.read()` which fails to expose
   * image types on Linux. Must be called synchronously from a paste event
   * handler so `preventDefault` takes effect before the event is consumed.
   */
  const handlePasteEvent = useCallback(
    (event: ClipboardEvent): void => {
      const items = event.clipboardData?.items
      if (!items) return

      for (const item of items) {
        if (item.type.startsWith('image/')) {
          const blob = item.getAsFile()
          if (blob) {
            event.preventDefault()
            event.stopPropagation()
            pasteImage(blob).catch(() => {})
            return
          }
        }
      }
    },
    [pasteImage],
  )

  /**
   * Handles a drop event, uploading the first image file found in the
   * drag payload to the container's xclip staging directory.
   */
  const handleDropEvent = useCallback(
    (event: DragEvent): void => {
      const files = event.dataTransfer?.files
      if (!files) return

      for (const file of files) {
        if (file.type.startsWith('image/')) {
          event.preventDefault()
          event.stopPropagation()
          pasteImage(file).catch(() => {})
          return
        }
      }
    },
    [pasteImage],
  )

  /**
   * Reads image data from the browser clipboard via the async Clipboard API.
   * Used by the context menu "Paste Image" action where no native paste event
   * is available. Falls back to a toast suggesting Ctrl+V on failure, since
   * `navigator.clipboard.read()` is unreliable on Linux.
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
      toast.error('No image found in clipboard. Try Ctrl+V instead.')
    } catch {
      toast.error('Clipboard access denied. Try Ctrl+V instead.')
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
      writeClipboardText(trimmed).then((ok) => {
        if (ok) {
          terminal.clearSelection()
          toast.success('Copied all to clipboard')
        }
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
          writeClipboardText('')
          return true
        }

        const isExplicit = tracker.consume()
        try {
          const text = atob(payload)
          writeClipboardText(text).then((ok) => {
            if (ok && isExplicit) toast.success('Copied to clipboard')
          })
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
    handlePasteEvent,
    handleDropEvent,
    pasteImageFromClipboard,
    selectAll,
    registerOsc52Handler,
    /** Sends raw bytes to the PTY. Exposed for key handlers that need PTY fallback. */
    sendToPty,
  }
}

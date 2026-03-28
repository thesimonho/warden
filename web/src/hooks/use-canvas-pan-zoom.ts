import { useCallback, useEffect, useRef, useState } from 'react'
import type { PanelGeometry } from '@/lib/canvas-store'

/** Duration of animated pan/zoom transitions in milliseconds. */
export const TRANSITION_DURATION_MS = 300

/** Canvas viewport transform: translation + uniform scale. */
export interface CanvasTransform {
  /** Horizontal translation in screen pixels. */
  x: number
  /** Vertical translation in screen pixels. */
  y: number
  /** Zoom level (1 = 100%). */
  scale: number
}

const MIN_SCALE = 0.1
const MAX_SCALE = 3
const FIT_PADDING = 48

/**
 * Zoom sensitivity for Ctrl+wheel events.
 *
 * Trackpad pinch sends small deltaY values with ctrlKey=true, so
 * direct scaling (0.002) gives smooth results. Mouse wheel sends
 * larger deltaY increments (~100 per tick), so a low factor keeps
 * zoom steps gentle for both input devices.
 */
const ZOOM_SENSITIVITY = 0.002

/** Clamps scale to the allowed [MIN_SCALE, MAX_SCALE] range. */
function clampScale(s: number): number {
  return Math.min(MAX_SCALE, Math.max(MIN_SCALE, s))
}

/** Cubic ease-out curve: fast start, gentle finish. */
function easeOutCubic(t: number): number {
  return 1 - Math.pow(1 - t, 3)
}

/**
 * Hook managing pan-zoom state for an infinite canvas.
 *
 * Supports:
 * - Ctrl+left-click drag → canvas pan
 * - Ctrl+wheel → zoom (cursor-anchored)
 * - Trackpad pinch → zoom (browser sends ctrlKey=true synthetically)
 * - Trackpad two-finger scroll (deltaX ≠ 0) → pan
 * - Plain wheel over terminal panels → pass through to xterm.js scrollback
 * - "Fit all" to maximize visibility of all panels
 *
 * Middle-click is completely freed for native behavior (e.g. paste on Linux).
 *
 * Programmatic animations use requestAnimationFrame to interpolate
 * {x, y, scale} in screen-space, then derive CSS zoom + translate at
 * each frame. This avoids incorrect intermediate positions that CSS
 * transitions produce when zoom and translate are animated independently.
 */
export function useCanvasPanZoom() {
  const [transform, setTransform] = useState<CanvasTransform>({
    x: 0,
    y: 0,
    scale: 1,
  })
  const [isPanning, setIsPanning] = useState(false)

  const containerRef = useRef<HTMLDivElement>(null)
  const transformRef = useRef(transform)
  useEffect(() => {
    transformRef.current = transform
  }, [transform])
  const isPanningRef = useRef(false)
  const lastPointerRef = useRef({ x: 0, y: 0 })
  const panButtonRef = useRef<number | null>(null)
  const rafIdRef = useRef(0)

  /** Cancels any in-flight pan/zoom animation. */
  const cancelAnimation = useCallback(() => {
    if (rafIdRef.current) {
      cancelAnimationFrame(rafIdRef.current)
      rafIdRef.current = 0
    }
  }, [])

  useEffect(() => cancelAnimation, [cancelAnimation])

  /**
   * Animates transform from current value to next via requestAnimationFrame.
   *
   * Interpolates x, y, scale together in screen-space so the derived
   * CSS `zoom` + `translate(x/s, y/s)` values are correct at every frame.
   */
  const animateTo = useCallback(
    (next: CanvasTransform) => {
      cancelAnimation()

      const from = { ...transformRef.current }
      const startTime = performance.now()

      const tick = (now: number) => {
        const progress = Math.min((now - startTime) / TRANSITION_DURATION_MS, 1)
        const t = easeOutCubic(progress)

        const current: CanvasTransform = {
          x: from.x + (next.x - from.x) * t,
          y: from.y + (next.y - from.y) * t,
          scale: from.scale + (next.scale - from.scale) * t,
        }

        setTransform(current)

        if (progress < 1) {
          rafIdRef.current = requestAnimationFrame(tick)
        } else {
          rafIdRef.current = 0
        }
      }

      rafIdRef.current = requestAnimationFrame(tick)
    },
    [cancelAnimation],
  )

  /**
   * Handles wheel events for zoom and trackpad pan.
   *
   * - Ctrl+wheel (or trackpad pinch, which has ctrlKey=true): zoom
   *   towards the cursor so the point under the mouse stays fixed.
   * - Trackpad two-finger scroll (non-zero deltaX without ctrl): pan.
   * - Plain wheel over a canvas panel: pass through to xterm.js scrollback.
   * - Plain wheel over background: zoom (same as Ctrl+wheel).
   */
  const handleWheel = useCallback(
    (e: WheelEvent) => {
      const isTrackpadPan = !e.ctrlKey && e.deltaX !== 0

      if (isTrackpadPan) {
        // Two-finger scroll → pan
        e.preventDefault()
        cancelAnimation()
        setTransform((prev) => ({
          ...prev,
          x: prev.x - e.deltaX,
          y: prev.y - e.deltaY,
        }))
        return
      }

      // Without Ctrl: pass through to xterm.js when over a terminal panel,
      // otherwise zoom the canvas (background scroll = zoom).
      if (!e.ctrlKey) {
        const isOverPanel = (e.target as HTMLElement).closest?.('[data-canvas-panel]') != null
        if (isOverPanel) return
      }

      // Ctrl+wheel or trackpad pinch → zoom towards cursor.
      // stopPropagation prevents xterm.js from also scrolling scrollback.
      e.preventDefault()
      e.stopPropagation()
      cancelAnimation()

      const target = e.currentTarget as HTMLElement | null
      const rect = target?.getBoundingClientRect() ?? new DOMRect()
      const cursorX = e.clientX - rect.left
      const cursorY = e.clientY - rect.top

      setTransform((prev) => {
        const zoomFactor = 1 - e.deltaY * ZOOM_SENSITIVITY
        const newScale = clampScale(prev.scale * zoomFactor)
        const ratio = newScale / prev.scale

        return {
          scale: newScale,
          x: cursorX - (cursorX - prev.x) * ratio,
          y: cursorY - (cursorY - prev.y) * ratio,
        }
      })
    },
    [cancelAnimation],
  )

  // Attach wheel listener in capture phase with { passive: false } so
  // preventDefault() works and we fire before xterm.js's own wheel handler.
  useEffect(() => {
    const el = containerRef.current
    if (!el) return

    el.addEventListener('wheel', handleWheel, { passive: false, capture: true })
    return () => el.removeEventListener('wheel', handleWheel, { capture: true })
  }, [handleWheel])

  /**
   * Starts a pan gesture on Ctrl+left-click pointer down.
   *
   * Only Ctrl+left button (0 + ctrlKey) initiates panning, and it works
   * anywhere on the canvas (including over panels).
   */
  const handlePointerDown = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      if (!(e.button === 0 && e.ctrlKey)) return

      e.preventDefault()
      cancelAnimation()

      isPanningRef.current = true
      panButtonRef.current = e.button
      lastPointerRef.current = { x: e.clientX, y: e.clientY }
      setIsPanning(true)
      e.currentTarget.setPointerCapture(e.pointerId)
    },
    [cancelAnimation],
  )

  /** Updates translation during an active pan gesture. */
  const handlePointerMove = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    if (!isPanningRef.current) return

    const dx = e.clientX - lastPointerRef.current.x
    const dy = e.clientY - lastPointerRef.current.y
    lastPointerRef.current = { x: e.clientX, y: e.clientY }

    setTransform((prev) => ({
      ...prev,
      x: prev.x + dx,
      y: prev.y + dy,
    }))
  }, [])

  /** Ends a pan gesture. */
  const handlePointerUp = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    if (!isPanningRef.current) return
    if (panButtonRef.current !== null && e.button !== panButtonRef.current) {
      return
    }

    isPanningRef.current = false
    panButtonRef.current = null
    setIsPanning(false)
    e.currentTarget.releasePointerCapture(e.pointerId)
  }, [])

  /**
   * Computes a transform that fits all given panels within the viewport.
   *
   * Adds padding around the bounding box and centers it. If no panels
   * exist, resets to identity transform.
   */
  const fitAll = useCallback(
    (panels: { geometry: PanelGeometry }[], viewportSize: { width: number; height: number }) => {
      if (panels.length === 0) {
        setTransform({ x: 0, y: 0, scale: 1 })
        return
      }

      let minX = Infinity
      let minY = Infinity
      let maxX = -Infinity
      let maxY = -Infinity

      for (const panel of panels) {
        const { x, y, width, height } = panel.geometry
        minX = Math.min(minX, x)
        minY = Math.min(minY, y)
        maxX = Math.max(maxX, x + width)
        maxY = Math.max(maxY, y + height)
      }

      const boxWidth = maxX - minX
      const boxHeight = maxY - minY

      const availableWidth = viewportSize.width - FIT_PADDING * 2
      const availableHeight = viewportSize.height - FIT_PADDING * 2

      const scale = clampScale(Math.min(availableWidth / boxWidth, availableHeight / boxHeight))

      // Center the bounding box in the viewport
      const centerX = (minX + maxX) / 2
      const centerY = (minY + maxY) / 2

      animateTo({
        scale,
        x: viewportSize.width / 2 - centerX * scale,
        y: viewportSize.height / 2 - centerY * scale,
      })
    },
    [animateTo],
  )

  /**
   * Pans so the given panel is centered in the viewport.
   * Keeps the current zoom level.
   */
  const panTo = useCallback(
    (geometry: PanelGeometry, viewportSize: { width: number; height: number }) => {
      const prev = transformRef.current
      const centerX = geometry.x + geometry.width / 2
      const centerY = geometry.y + geometry.height / 2
      animateTo({
        scale: prev.scale,
        x: viewportSize.width / 2 - centerX * prev.scale,
        y: viewportSize.height / 2 - centerY * prev.scale,
      })
    },
    [animateTo],
  )

  /** Resets transform to identity (no pan, no zoom). */
  const resetTransform = useCallback(() => {
    setTransform({ x: 0, y: 0, scale: 1 })
  }, [])

  /**
   * Converts viewport (screen) coordinates to canvas-space coordinates.
   *
   * Useful for computing panel positions that fill the current viewport
   * (e.g. maximize). Uses a ref to avoid re-creating on every transform change.
   */
  const viewportToCanvas = useCallback(
    (screenX: number, screenY: number): { x: number; y: number } => {
      const t = transformRef.current
      return {
        x: (screenX - t.x) / t.scale,
        y: (screenY - t.y) / t.scale,
      }
    },
    [],
  )

  return {
    containerRef,
    transform,
    isPanning,
    handlePointerDown,
    handlePointerMove,
    handlePointerUp,
    fitAll,
    panTo,
    resetTransform,
    viewportToCanvas,
  }
}

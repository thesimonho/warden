import { useCallback, useMemo, useRef, useState } from 'react'
import type { CanvasPanel } from '@/lib/canvas-store'

/** Minimum drag distance (px) before a marquee becomes visible. */
const MARQUEE_THRESHOLD = 5

/** Screen-space rectangle for marquee selection. */
export interface MarqueeRect {
  startX: number
  startY: number
  currentX: number
  currentY: number
}

/** Parameters for the marquee selection hook. */
interface UseMarqueeSelectionParams {
  panels: CanvasPanel[]
  canvasRectRef: React.RefObject<DOMRect | null>
  viewportToCanvas: (x: number, y: number) => { x: number; y: number }
  selectPanels: (ids: string[]) => void
  clearSelection: () => void
  clearFocus: () => void
  handlePanPointerDown: (e: React.PointerEvent<HTMLDivElement>) => void
  handlePanPointerMove: (e: React.PointerEvent<HTMLDivElement>) => void
  handlePanPointerUp: (e: React.PointerEvent<HTMLDivElement>) => void
}

/**
 * Manages marquee (rubber-band) selection on the canvas.
 *
 * Left-drag on the canvas background starts a marquee. Moving the
 * mouse expands it and selects panels that intersect. Releasing the
 * mouse completes the selection. Clicking without dragging clears
 * selection and focus.
 *
 * Ctrl+left-drag is delegated to the pan-zoom handler instead.
 */
export function useMarqueeSelection({
  panels,
  canvasRectRef,
  viewportToCanvas,
  selectPanels,
  clearSelection,
  clearFocus,
  handlePanPointerDown,
  handlePanPointerMove,
  handlePanPointerUp,
}: UseMarqueeSelectionParams) {
  const [marquee, setMarquee] = useState<MarqueeRect | null>(null)
  const marqueeRef = useRef<MarqueeRect | null>(null)

  /** Checks if a pointer event target is on the empty canvas background. */
  const isCanvasBackground = useCallback((target: EventTarget): boolean => {
    const el = target as HTMLElement
    return !el.closest('[data-canvas-panel]') && !el.closest('[data-canvas-toolbar]')
  }, [])

  /** Selects panels intersecting the given screen-space marquee rect. */
  const selectPanelsInMarquee = useCallback(
    (mq: MarqueeRect) => {
      const canvasRect = canvasRectRef.current
      if (!canvasRect) return

      const screenLeft = Math.min(mq.startX, mq.currentX)
      const screenTop = Math.min(mq.startY, mq.currentY)
      const screenRight = Math.max(mq.startX, mq.currentX)
      const screenBottom = Math.max(mq.startY, mq.currentY)

      const topLeft = viewportToCanvas(screenLeft - canvasRect.left, screenTop - canvasRect.top)
      const bottomRight = viewportToCanvas(
        screenRight - canvasRect.left,
        screenBottom - canvasRect.top,
      )

      const intersecting = panels.filter((p) => {
        const { x, y, width, height } = p.geometry
        return (
          x < bottomRight.x && x + width > topLeft.x && y < bottomRight.y && y + height > topLeft.y
        )
      })

      selectPanels(intersecting.map((p) => p.id))
    },
    [panels, canvasRectRef, viewportToCanvas, selectPanels],
  )

  const handlePointerDown = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      if (e.button === 0 && e.ctrlKey) {
        handlePanPointerDown(e)
        return
      }

      if (e.button === 0 && isCanvasBackground(e.target)) {
        const rect: MarqueeRect = {
          startX: e.clientX,
          startY: e.clientY,
          currentX: e.clientX,
          currentY: e.clientY,
        }
        marqueeRef.current = rect
      }
    },
    [handlePanPointerDown, isCanvasBackground],
  )

  const handlePointerMove = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      handlePanPointerMove(e)

      if (marqueeRef.current) {
        const updated = {
          ...marqueeRef.current,
          currentX: e.clientX,
          currentY: e.clientY,
        }
        marqueeRef.current = updated

        const dx = Math.abs(updated.currentX - updated.startX)
        const dy = Math.abs(updated.currentY - updated.startY)
        if (dx >= MARQUEE_THRESHOLD || dy >= MARQUEE_THRESHOLD) {
          setMarquee(updated)
          selectPanelsInMarquee(updated)
        }
      }
    },
    [handlePanPointerMove, selectPanelsInMarquee],
  )

  const handlePointerUp = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      handlePanPointerUp(e)

      const mq = marqueeRef.current
      marqueeRef.current = null

      if (mq) {
        const dx = Math.abs(mq.currentX - mq.startX)
        const dy = Math.abs(mq.currentY - mq.startY)

        if (dx < MARQUEE_THRESHOLD && dy < MARQUEE_THRESHOLD) {
          clearSelection()
          clearFocus()
        }

        setMarquee(null)
        return
      }
    },
    [handlePanPointerUp, clearSelection, clearFocus],
  )

  /* eslint-disable react-hooks/refs -- canvasRectRef is stable and only read during active drag */
  /** Computed marquee overlay style (screen-space coordinates). */
  const marqueeStyle = useMemo(() => {
    if (!marquee) return undefined
    const canvasRect = canvasRectRef.current
    if (!canvasRect) return undefined

    return {
      left: Math.min(marquee.startX, marquee.currentX) - canvasRect.left,
      top: Math.min(marquee.startY, marquee.currentY) - canvasRect.top,
      width: Math.abs(marquee.currentX - marquee.startX),
      height: Math.abs(marquee.currentY - marquee.startY),
    }
  }, [marquee, canvasRectRef])
  /* eslint-enable react-hooks/refs */

  return {
    marquee,
    marqueeStyle,
    handlePointerDown,
    handlePointerMove,
    handlePointerUp,
  }
}

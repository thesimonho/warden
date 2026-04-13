import { arrayMove } from '@dnd-kit/sortable'
import { useCallback, useRef, useState } from 'react'
import { layoutGrid, layoutHorizontal, layoutVertical } from '@/lib/canvas-layout'
import type { Worktree } from '@/lib/types'
import { isWorktreeAlive } from '@/lib/types'

/** Position and size of a canvas panel. */
export interface PanelGeometry {
  x: number
  y: number
  width: number
  height: number
}

/** A panel on the canvas representing a worktree terminal. */
export interface CanvasPanel {
  /** Unique panel identifier. */
  id: string
  /** Project (container) ID this panel is connected to. */
  projectId: string
  /** CLI agent type for this project. */
  agentType: string
  /** Project display name. */
  projectName: string
  /** Worktree ID within the project. */
  worktreeId: string
  /** Git branch, if any. */
  branch?: string
  /** Panel position and size on the canvas. */
  geometry: PanelGeometry
  /** Saved geometry before maximize, for restore. */
  preMaximizeGeometry?: PanelGeometry
  /** Whether the panel is maximized. */
  isMaximized: boolean
  /** Z-index for stacking order. */
  zIndex: number
}

/** Layout arrangement type for selected panels. */
export type LayoutType = 'grid' | 'horizontal' | 'vertical'

/** Builds a unique panel ID from project and worktree IDs. */
export function buildPanelId(projectId: string, worktreeId: string): string {
  return `${projectId}-${worktreeId}`
}

const DEFAULT_WIDTH = 800
const DEFAULT_HEIGHT = 600
const PANEL_GAP = 16

/** Duration of layout animations in milliseconds. */
export const LAYOUT_ANIMATION_MS = 200

/**
 * Returns a position to the right of the focused panel, or a fallback
 * staggered from the viewport center.
 *
 * If the candidate position overlaps an existing panel, cascades
 * down-right until clear.
 */
function computeNewPanelPosition(
  existingPanels: CanvasPanel[],
  focusedPanelId: string | null,
  viewportCenter?: { x: number; y: number },
): { x: number; y: number } {
  const focused = focusedPanelId ? existingPanels.find((p) => p.id === focusedPanelId) : undefined

  let candidateX: number
  let candidateY: number

  if (focused) {
    candidateX = focused.geometry.x + focused.geometry.width + PANEL_GAP
    candidateY = focused.geometry.y
  } else if (viewportCenter) {
    candidateX = viewportCenter.x - DEFAULT_WIDTH / 2
    candidateY = viewportCenter.y - DEFAULT_HEIGHT / 2
  } else {
    candidateX = 60
    candidateY = 60
  }

  return cascadeUntilClear(existingPanels, candidateX, candidateY)
}

/** Cascades position down-right until it doesn't overlap any panel. */
function cascadeUntilClear(
  panels: CanvasPanel[],
  startX: number,
  startY: number,
): { x: number; y: number } {
  let x = startX
  let y = startY
  const step = 30

  for (let i = 0; i < 20; i++) {
    const overlaps = panels.some(
      (p) =>
        x < p.geometry.x + p.geometry.width &&
        x + DEFAULT_WIDTH > p.geometry.x &&
        y < p.geometry.y + p.geometry.height &&
        y + DEFAULT_HEIGHT > p.geometry.y,
    )
    if (!overlaps) break
    x += step
    y += step
  }

  return { x, y }
}

/**
 * Hook managing the state of all panels on the canvas.
 *
 * Provides add, remove, move, resize, maximize, focus, selection,
 * and layout operations.
 */
export function useCanvasStore() {
  const [panels, setPanels] = useState<CanvasPanel[]>([])
  const [focusedPanelId, setFocusedPanelId] = useState<string | null>(null)
  const [selectedPanelIds, setSelectedPanelIds] = useState<Set<string>>(new Set())
  const [isLayoutAnimating, setIsLayoutAnimating] = useState(false)
  const maxZRef = useRef(1)
  const animationTimerRef = useRef<ReturnType<typeof setTimeout>>(null)

  /**
   * Wraps a geometry-updating function with layout animation.
   *
   * Sets the animation flag for 300ms so CSS transitions apply to
   * panel position/size changes driven by store updates.
   */
  const withLayoutAnimation = useCallback((fn: () => void) => {
    if (animationTimerRef.current) clearTimeout(animationTimerRef.current)
    setIsLayoutAnimating(true)
    fn()
    animationTimerRef.current = setTimeout(() => {
      setIsLayoutAnimating(false)
    }, LAYOUT_ANIMATION_MS)
  }, [])

  /** Clears layout animation immediately (e.g. on drag start). */
  const clearLayoutAnimation = useCallback(() => {
    if (animationTimerRef.current) clearTimeout(animationTimerRef.current)
    setIsLayoutAnimating(false)
  }, [])

  /**
   * Adds a new panel for a project worktree.
   *
   * Places it to the right of the focused panel if one exists,
   * otherwise at the viewport center.
   *
   * @returns The geometry of the newly created panel, or null if it already existed.
   */
  const addPanel = useCallback(
    (
      params: {
        projectId: string
        agentType: string
        projectName: string
        worktreeId: string
        branch?: string
      },
      viewportCenter?: { x: number; y: number },
    ): PanelGeometry | null => {
      let result: PanelGeometry | null = null

      setPanels((prev) => {
        const alreadyExists = prev.some(
          (p) => p.projectId === params.projectId && p.worktreeId === params.worktreeId,
        )
        if (alreadyExists) return prev

        const position = computeNewPanelPosition(prev, focusedPanelId, viewportCenter)
        maxZRef.current += 1
        const newZ = maxZRef.current

        const id = buildPanelId(params.projectId, params.worktreeId)
        const geometry: PanelGeometry = {
          ...position,
          width: DEFAULT_WIDTH,
          height: DEFAULT_HEIGHT,
        }
        const panel: CanvasPanel = {
          id,
          ...params,
          geometry,
          isMaximized: false,
          zIndex: newZ,
        }
        result = geometry
        setFocusedPanelId(id)
        return [...prev, panel]
      })

      return result
    },
    [focusedPanelId],
  )

  /** Removes a panel by ID and clears focus/selection if it was targeted. */
  const removePanel = useCallback((panelId: string) => {
    setFocusedPanelId((prev) => (prev === panelId ? null : prev))
    setSelectedPanelIds((prev) => {
      if (!prev.has(panelId)) return prev
      const next = new Set(prev)
      next.delete(panelId)
      return next
    })
    setPanels((prev) => prev.filter((p) => p.id !== panelId))
  }, [])

  /** Updates panel geometry after drag or resize. */
  const updateGeometry = useCallback((panelId: string, geometry: Partial<PanelGeometry>) => {
    setPanels((prev) =>
      prev.map((p) => (p.id === panelId ? { ...p, geometry: { ...p.geometry, ...geometry } } : p)),
    )
  }, [])

  /** Brings a panel to front by giving it the highest z-index and marking it focused. */
  const bringToFront = useCallback((panelId: string) => {
    maxZRef.current += 1
    const newZ = maxZRef.current
    setFocusedPanelId(panelId)
    setPanels((prev) => prev.map((p) => (p.id === panelId ? { ...p, zIndex: newZ } : p)))
  }, [])

  /**
   * Toggles maximize state for a panel.
   *
   * When maximizing, pass the target geometry so it can be set atomically
   * with saving the pre-maximize geometry. This avoids the race where
   * a separate `updateGeometry` call overwrites the geometry before
   * `preMaximizeGeometry` is saved.
   */
  const toggleMaximize = useCallback((panelId: string, maximizeGeometry?: PanelGeometry) => {
    maxZRef.current += 1
    const newZ = maxZRef.current
    setPanels((prev) =>
      prev.map((p) => {
        if (p.id !== panelId) return p
        if (p.isMaximized) {
          return {
            ...p,
            isMaximized: false,
            geometry: p.preMaximizeGeometry ?? p.geometry,
            preMaximizeGeometry: undefined,
            zIndex: newZ,
          }
        }
        return {
          ...p,
          isMaximized: true,
          preMaximizeGeometry: p.geometry,
          geometry: maximizeGeometry ?? p.geometry,
          zIndex: newZ,
        }
      }),
    )
  }, [])

  /** Selects the given panel IDs, replacing the current selection. */
  const selectPanels = useCallback((ids: string[]) => {
    setSelectedPanelIds(new Set(ids))
  }, [])

  /** Toggles a panel in/out of the current selection. */
  const toggleSelection = useCallback((id: string) => {
    setSelectedPanelIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }, [])

  /** Clears the panel selection. */
  const clearSelection = useCallback(() => {
    setSelectedPanelIds((prev) => (prev.size === 0 ? prev : new Set()))
  }, [])

  /**
   * Applies a layout arrangement to the currently selected panels.
   *
   * Wraps the geometry updates in a layout animation so CSS
   * transitions animate panels to their new positions.
   */
  const applyLayout = useCallback(
    (layout: LayoutType) => {
      withLayoutAnimation(() => {
        setPanels((prev) => {
          const selected = prev.filter((p) => selectedPanelIds.has(p.id))
          if (selected.length < 2) return prev

          const inputs = selected.map((p) => ({
            id: p.id,
            geometry: p.geometry,
          }))

          const layoutFn =
            layout === 'grid'
              ? layoutGrid
              : layout === 'horizontal'
                ? layoutHorizontal
                : layoutVertical

          const outputs = layoutFn(inputs, PANEL_GAP)
          const outputMap = new Map(outputs.map((o) => [o.id, o.geometry]))

          return prev.map((p) => {
            const newGeo = outputMap.get(p.id)
            return newGeo ? { ...p, geometry: newGeo } : p
          })
        })
      })
    },
    [selectedPanelIds, withLayoutAnimation],
  )

  /** Sets focus to a panel without changing z-index (for grid mode). */
  const setFocusedPanel = useCallback((panelId: string) => {
    setFocusedPanelId(panelId)
  }, [])

  /** Clears focus (no panel focused). */
  const clearFocus = useCallback(() => {
    setFocusedPanelId(null)
  }, [])

  /** Moves a panel from one array position to another (for grid reordering). */
  const reorderPanels = useCallback((fromIndex: number, toIndex: number) => {
    setPanels((prev) => arrayMove(prev, fromIndex, toIndex))
  }, [])

  /**
   * Syncs canvas panels with live worktree state for a given project.
   *
   * Removes panels whose worktrees have stopped. Called by the
   * sidebar on each worktree poll cycle.
   */
  const syncWorktrees = useCallback((projectId: string, worktrees: Worktree[]) => {
    setPanels((prev) => {
      const worktreeMap = new Map(worktrees.map((wt) => [wt.id, wt]))
      let changed = false

      const updated = prev.reduce<CanvasPanel[]>((acc, panel) => {
        if (panel.projectId !== projectId) {
          acc.push(panel)
          return acc
        }

        const worktree = worktreeMap.get(panel.worktreeId)

        // Worktree gone or stopped — remove the panel.
        if (!worktree || !isWorktreeAlive(worktree)) {
          changed = true
          return acc
        }

        acc.push(panel)
        return acc
      }, [])

      return changed ? updated : prev
    })
  }, [])

  return {
    panels,
    focusedPanelId,
    selectedPanelIds,
    isLayoutAnimating,
    addPanel,
    removePanel,
    updateGeometry,
    bringToFront,
    toggleMaximize,
    syncWorktrees,
    selectPanels,
    toggleSelection,
    clearSelection,
    setFocusedPanel,
    clearFocus,
    reorderPanels,
    applyLayout,
    withLayoutAnimation,
    clearLayoutAnimation,
  }
}

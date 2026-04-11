import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Focus, Frame, LayoutGrid, ArrowRightFromLine, ArrowDownFromLine } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import ProjectSidebar, { type ViewMode } from '@/components/project/project-sidebar'
import CanvasView from '@/components/project/canvas-view'
import GridView, { type GridViewHandle } from '@/components/project/grid-view'
import { useCanvasStore, buildPanelId, LAYOUT_ANIMATION_MS } from '@/lib/canvas-store'
import { useFocusReporter } from '@/hooks/use-focus-reporter'
import { useCanvasWorktreeState } from '@/hooks/use-canvas-worktree-state'
import { useCanvasPanZoom } from '@/hooks/use-canvas-pan-zoom'
import { useMarqueeSelection } from '@/hooks/use-marquee-selection'

/** Extra buffer for React to commit the state update after the CSS animation. */
const POST_ANIMATION_BUFFER_MS = 50

/** Props for the ProjectView component. */
export interface ProjectViewProps {
  /** Project (container) ID to display. */
  projectId: string
  /** CLI agent type for this project. */
  agentType: string
  /** Called when the user picks a different project in the sidebar dropdown. */
  onProjectChange: (projectId: string, agentType: string) => void
}

/**
 * Core project UI — sidebar with view mode toggle, plus either a CSS grid
 * or an infinite canvas of terminal panels.
 *
 * This component is layout-agnostic: it fills its parent container.
 * The route page wrapper (`project-page.tsx`) handles fixed viewport
 * positioning; the workspace page embeds it in a grid cell.
 */
export default function ProjectView({ projectId, agentType, onProjectChange }: ProjectViewProps) {
  const [viewMode, setViewMode] = useState<ViewMode>('grid')

  const {
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
  } = useCanvasStore()

  const gridRef = useRef<GridViewHandle>(null)
  const panelsRef = useRef(panels)
  useEffect(() => {
    panelsRef.current = panels
  }, [panels])

  /** Syncs grid focus state with the canvas store so the sidebar highlights the focused panel. */
  const handleGridFocusChange = useCallback(
    (panelId: string, focused: boolean) => {
      if (focused) {
        setFocusedPanel(panelId)
      } else {
        clearFocus()
      }
    },
    [setFocusedPanel, clearFocus],
  )

  const activePanelIds = useMemo(() => new Set(panels.map((p) => p.id)), [panels])
  const worktreeStates = useCanvasWorktreeState(panels)

  // Report viewer focus state so the tray can suppress notifications for focused projects.
  useFocusReporter(projectId, agentType, panels)

  // ─── Stable callbacks for CanvasView (avoid re-renders on SSE) ─────
  const handleCanvasDragStop = useCallback(
    (id: string, x: number, y: number) => updateGeometry(id, { x, y }),
    [updateGeometry],
  )

  const handleCanvasResizeStop = useCallback(
    (id: string, width: number, height: number, x: number, y: number) =>
      updateGeometry(id, { width, height, x, y }),
    [updateGeometry],
  )

  // Track canvas container size for maximize and fit-all.
  const canvasRef = useRef<HTMLDivElement>(null)
  const canvasRectRef = useRef<DOMRect | null>(null)
  const [canvasSize, setCanvasSize] = useState({ width: 0, height: 0 })
  const canvasSizeRef = useRef(canvasSize)
  useEffect(() => {
    canvasSizeRef.current = canvasSize
  }, [canvasSize])

  const measureCanvas = useCallback(() => {
    if (canvasRef.current) {
      canvasRectRef.current = canvasRef.current.getBoundingClientRect()
      setCanvasSize({
        width: canvasRef.current.offsetWidth,
        height: canvasRef.current.offsetHeight,
      })
    }
  }, [])

  useEffect(() => {
    measureCanvas()
    window.addEventListener('resize', measureCanvas)
    return () => window.removeEventListener('resize', measureCanvas)
  }, [measureCanvas, viewMode])

  const {
    setContainerEl: setPanZoomEl,
    transform,
    isPanning,
    handlePointerDown: handlePanPointerDown,
    handlePointerMove: handlePanPointerMove,
    handlePointerUp: handlePanPointerUp,
    fitAll,
    panTo,
    viewportToCanvas,
  } = useCanvasPanZoom()

  const transformScaleRef = useRef(transform.scale)
  useEffect(() => {
    transformScaleRef.current = transform.scale
  }, [transform.scale])

  /** Merges canvasRef (for size measurement) and panZoomRef (for wheel listener). */
  const setCanvasRef = useCallback(
    (node: HTMLDivElement | null) => {
      canvasRef.current = node
      setPanZoomEl(node)
      if (node) measureCanvas()
    },
    [setPanZoomEl, measureCanvas],
  )

  // ─── Ctrl key tracking (for grab cursor hint) ─────────────────────
  const [isCtrlHeld, setIsCtrlHeld] = useState(false)

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Control') setIsCtrlHeld(true)
    }
    const handleKeyUp = (e: KeyboardEvent) => {
      if (e.key === 'Control') setIsCtrlHeld(false)
    }
    const handleBlur = () => setIsCtrlHeld(false)

    window.addEventListener('keydown', handleKeyDown)
    window.addEventListener('keyup', handleKeyUp)
    window.addEventListener('blur', handleBlur)
    return () => {
      window.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener('keyup', handleKeyUp)
      window.removeEventListener('blur', handleBlur)
    }
  }, [])

  // ─── Marquee selection ──────────────────────────────────────────────
  const { marquee, marqueeStyle, handlePointerDown, handlePointerMove, handlePointerUp } =
    useMarqueeSelection({
      panels,
      canvasRectRef,
      viewportToCanvas,
      selectPanels,
      clearSelection,
      clearFocus,
      handlePanPointerDown,
      handlePanPointerMove,
      handlePanPointerUp,
    })

  // ─── Panel event handlers ───────────────────────────────────────────
  const handleFitAll = useCallback(() => {
    fitAll(panelsRef.current, canvasSizeRef.current)
  }, [fitAll])

  const handleFitSelection = useCallback(() => {
    const selected = panelsRef.current.filter((p) => selectedPanelIds.has(p.id))
    if (selected.length >= 2) {
      fitAll(selected, canvasSizeRef.current)
    }
  }, [fitAll, selectedPanelIds])

  /** Applies a layout then auto-fits the selection after the animation. */
  const applyLayoutAndFit = useCallback(
    (layout: 'grid' | 'horizontal' | 'vertical') => {
      applyLayout(layout)
      setTimeout(() => {
        const selected = panelsRef.current.filter((p) => selectedPanelIds.has(p.id))
        if (selected.length >= 2) {
          fitAll(selected, canvasSizeRef.current)
        }
      }, LAYOUT_ANIMATION_MS + POST_ANIMATION_BUFFER_MS)
    },
    [applyLayout, selectedPanelIds, fitAll],
  )

  /** Focuses an existing panel — scrolls grid or pans canvas depending on view mode. */
  const handleFocusPanel = useCallback(
    (panelId: string) => {
      const panel = panelsRef.current.find((p) => p.id === panelId)
      if (!panel) return

      if (viewMode === 'grid') {
        gridRef.current?.focusPanel(panelId)
      } else {
        bringToFront(panelId)
        panTo(panel.geometry, canvasSizeRef.current)
      }
    },
    [viewMode, bringToFront, panTo],
  )

  /** Handles maximize by updating geometry in the store. */
  const handleMaximize = useCallback(
    (panelId: string) => {
      const panel = panelsRef.current.find((p) => p.id === panelId)
      if (!panel) return

      withLayoutAnimation(() => {
        if (panel.isMaximized) {
          toggleMaximize(panelId)
        } else {
          const cs = canvasSizeRef.current
          const scale = transformScaleRef.current
          const origin = viewportToCanvas(0, 0)
          toggleMaximize(panelId, {
            x: origin.x,
            y: origin.y,
            width: cs.width / scale,
            height: cs.height / scale,
          })
        }
      })
    },
    [withLayoutAnimation, viewportToCanvas, toggleMaximize],
  )

  /** Adds a panel, panning the canvas to center it or focusing the grid cell. */
  const handleAddPanel = useCallback(
    (params: {
      projectId: string
      agentType: string
      projectName: string
      worktreeId: string
      branch?: string
    }) => {
      const cs = canvasSizeRef.current
      if (viewMode === 'canvas' && cs.width > 0) {
        const center = viewportToCanvas(cs.width / 2, cs.height / 2)
        const geometry = addPanel(params, center)
        if (geometry) {
          panTo(geometry, cs)
        }
      } else {
        addPanel(params)
        // Focus the new terminal after React renders the grid cell.
        const panelId = buildPanelId(params.projectId, params.worktreeId)
        requestAnimationFrame(() => {
          gridRef.current?.focusPanel(panelId)
        })
      }
    },
    [viewMode, addPanel, viewportToCanvas, panTo],
  )

  // ─── Keyboard shortcuts (scoped to canvas container) ────────────────
  const handleCanvasKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      const tag = (e.target as HTMLElement).tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return

      const hasShortcutSelection = selectedPanelIds.size >= 2

      if (e.key === 'g' && hasShortcutSelection) {
        e.preventDefault()
        applyLayoutAndFit('grid')
      } else if (e.key === 'h' && hasShortcutSelection) {
        e.preventDefault()
        applyLayoutAndFit('horizontal')
      } else if (e.key === 'v' && hasShortcutSelection) {
        e.preventDefault()
        applyLayoutAndFit('vertical')
      } else if (e.key === 'Escape') {
        clearSelection()
      }
    },
    [selectedPanelIds.size, applyLayoutAndFit, clearSelection],
  )

  // ─── Derived state ──────────────────────────────────────────────────
  const hasSelection = selectedPanelIds.size >= 2

  return (
    <div className="flex h-full">
      <ProjectSidebar
        selectedProjectId={projectId}
        selectedAgentType={agentType}
        onProjectChange={onProjectChange}
        viewMode={viewMode}
        onViewModeChange={setViewMode}
        onAddPanel={handleAddPanel}
        onFocusPanel={handleFocusPanel}
        onRemovePanel={removePanel}
        onSyncWorktrees={syncWorktrees}
        activePanelIds={activePanelIds}
        focusedPanelId={focusedPanelId}
      />

      {viewMode === 'grid' ? (
        <GridView
          ref={gridRef}
          panels={panels}
          worktreeStates={worktreeStates}
          onRemovePanel={removePanel}
          onFocusChange={handleGridFocusChange}
          onReorder={reorderPanels}
        />
      ) : (
        /* Canvas viewport — captures pan/zoom and marquee events */
        <div
          ref={setCanvasRef}
          className="bg-muted/10 relative flex-1 overflow-hidden outline-none"
          tabIndex={-1}
          onKeyDown={handleCanvasKeyDown}
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          style={{
            touchAction: 'none',
            userSelect: marquee ? 'none' : undefined,
            WebkitUserSelect: marquee ? 'none' : undefined,
            cursor: isPanning ? 'grabbing' : isCtrlHeld ? 'grab' : undefined,
          }}
        >
          {/* Dot grid background */}
          <div
            className="pointer-events-none absolute inset-0 origin-top-left opacity-[0.10] dark:opacity-[0.05]"
            style={{
              backgroundImage: 'radial-gradient(circle, currentColor 1px, transparent 1px)',
              backgroundSize: `${24 * transform.scale}px ${24 * transform.scale}px`,
              backgroundPosition: `${transform.x}px ${transform.y}px`,
            }}
          />

          <div className="absolute inset-0" />
          {isPanning && <div className="absolute inset-0 z-99999" />}
          {marquee && <div className="absolute inset-0 z-99999 cursor-crosshair" />}

          {marqueeStyle && (
            <div
              className="border-marquee bg-marquee/15 pointer-events-none absolute z-99999 border border-dashed"
              style={marqueeStyle}
            />
          )}

          {/* Transformed canvas surface */}
          <div
            className="absolute origin-top-left"
            style={{
              transform: `translate(${transform.x / transform.scale}px, ${transform.y / transform.scale}px)`,
              zoom: transform.scale,
            }}
          >
            {panels.map((panel) => {
              const state = worktreeStates.get(panel.id)
              return (
                <CanvasView
                  key={panel.id}
                  panel={panel}
                  isActive={state?.isActive ?? false}
                  isFocused={panel.id === focusedPanelId}
                  isSelected={selectedPanelIds.has(panel.id)}
                  isLayoutAnimating={isLayoutAnimating}
                  needsInput={state?.needsInput ?? false}
                  attentionDotClass={state?.attentionDotClass}
                  attentionLabel={state?.attentionLabel}
                  stateLabel={state?.stateLabel}
                  stateDotClass={state?.stateDotClass ?? 'bg-muted-foreground/40'}
                  scale={transform.scale}
                  onRemove={removePanel}
                  onDragStop={handleCanvasDragStop}
                  onResizeStop={handleCanvasResizeStop}
                  onFocus={bringToFront}
                  onShiftClick={toggleSelection}
                  onMaximize={handleMaximize}
                  onInteractionStart={clearLayoutAnimation}
                />
              )
            })}
          </div>

          {/* Empty state */}
          {panels.length === 0 && (
            <div
              data-testid="canvas-empty-state"
              className="pointer-events-none flex h-full items-center justify-center"
            >
              <div className="text-center">
                <p className="text-muted-foreground">Select a worktree from the sidebar</p>
                <div className="text-muted-foreground/60 mt-3 space-y-1 text-sm">
                  <p>
                    <kbd className="border-border/40 rounded border px-1.5 py-0.5 text-xs">
                      Ctrl
                    </kbd>
                    {' + Drag — Pan'}
                  </p>
                  <p>
                    <kbd className="border-border/40 rounded border px-1.5 py-0.5 text-xs">
                      Ctrl
                    </kbd>
                    {' + Scroll — Zoom'}
                  </p>
                  <p>Click drag — Select panels</p>
                </div>
              </div>
            </div>
          )}

          {/* Canvas toolbar */}
          <div
            data-canvas-toolbar
            className="absolute right-4 bottom-4 z-100000 flex items-center gap-1"
          >
            <TooltipProvider>
              {hasSelection && (
                <>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        data-testid="layout-grid-button"
                        variant="secondary"
                        size="icon"
                        className="h-8 w-8 rounded-full shadow-md backdrop-blur-sm"
                        onClick={() => applyLayoutAndFit('grid')}
                        icon={LayoutGrid}
                      />
                    </TooltipTrigger>
                    <TooltipContent side="top">
                      Grid layout <kbd className="ml-1 text-xs opacity-60">G</kbd>
                    </TooltipContent>
                  </Tooltip>

                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        data-testid="layout-horizontal-button"
                        variant="secondary"
                        size="icon"
                        className="h-8 w-8 rounded-full shadow-md backdrop-blur-sm"
                        onClick={() => applyLayoutAndFit('horizontal')}
                        icon={ArrowRightFromLine}
                      />
                    </TooltipTrigger>
                    <TooltipContent side="top">
                      Horizontal layout <kbd className="ml-1 text-xs opacity-60">H</kbd>
                    </TooltipContent>
                  </Tooltip>

                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        data-testid="layout-vertical-button"
                        variant="secondary"
                        size="icon"
                        className="h-8 w-8 rounded-full shadow-md backdrop-blur-sm"
                        onClick={() => applyLayoutAndFit('vertical')}
                        icon={ArrowDownFromLine}
                      />
                    </TooltipTrigger>
                    <TooltipContent side="top">
                      Vertical layout <kbd className="ml-1 text-xs opacity-60">V</kbd>
                    </TooltipContent>
                  </Tooltip>

                  <div className="bg-border mx-1 h-5 w-px" />

                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        data-testid="fit-selection-button"
                        variant="secondary"
                        size="icon"
                        className="h-8 w-8 rounded-full shadow-md backdrop-blur-sm"
                        onClick={handleFitSelection}
                        icon={Frame}
                      />
                    </TooltipTrigger>
                    <TooltipContent side="top">Fit selection</TooltipContent>
                  </Tooltip>
                </>
              )}

              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    data-testid="fit-all-button"
                    variant="outline"
                    size="icon"
                    className="bg-background/80 h-8 w-8 rounded-full shadow-md backdrop-blur-sm"
                    onClick={handleFitAll}
                    icon={Focus}
                  />
                </TooltipTrigger>
                <TooltipContent side="left">Fit all panels</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
        </div>
      )}
    </div>
  )
}

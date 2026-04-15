import {
  closestCenter,
  DndContext,
  type DragEndEvent,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import { rectSortingStrategy, SortableContext, useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import {
  forwardRef,
  memo,
  useCallback,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from 'react'

import TerminalCard, { type TerminalCardHandle } from '@/components/project/terminal-card'
import type { PanelWorktreeState } from '@/hooks/use-canvas-worktree-state'
import { useTerminalDisconnect } from '@/hooks/use-terminal-disconnect'
import type { CanvasPanel as CanvasPanelData } from '@/lib/canvas-store'
import { worktreeDisplayName } from '@/lib/types'
import { cn } from '@/lib/utils'

/** Minimum drag distance before activation — prevents accidental drags on click. */
const POINTER_ACTIVATION = { distance: 8 } as const

/** Props for a single grid cell containing a terminal. */
interface GridCellProps {
  panel: CanvasPanelData
  isActive: boolean
  needsInput: boolean
  attentionDotClass?: string
  attentionLabel?: string
  stateLabel?: string
  stateDotClass: string
  onRemove: (panelId: string) => void
  /** Called when focus enters or leaves this cell. */
  onFocusChange: (panelId: string, focused: boolean) => void
  /** Registers this cell's terminal handle so the parent can focus it. */
  onRegister: (panelId: string, handle: TerminalCardHandle) => void
  /** Unregisters this cell's terminal handle on unmount. */
  onUnregister: (panelId: string) => void
}

/**
 * A single sortable terminal cell in the grid layout.
 *
 * Wraps a TerminalCard with dnd-kit's useSortable hook. The title bar
 * acts as the drag handle; the terminal content is untouched.
 */
function GridCellInner({
  panel,
  isActive,
  needsInput,
  attentionDotClass,
  attentionLabel,
  stateLabel,
  stateDotClass,
  onRemove,
  onFocusChange,
  onRegister,
  onUnregister,
}: GridCellProps) {
  const terminalRef = useRef<TerminalCardHandle>(null)
  const [isFocused, setIsFocused] = useState(false)

  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: panel.id,
  })

  const sortableStyle = useMemo(
    () => ({ transform: CSS.Transform.toString(transform), transition }),
    [transform, transition],
  )

  const handleFocusCapture = useCallback(() => {
    setIsFocused(true)
    onFocusChange(panel.id, true)
  }, [panel.id, onFocusChange])

  const handleBlurCapture = useCallback(() => {
    setIsFocused(false)
    onFocusChange(panel.id, false)
  }, [panel.id, onFocusChange])

  /** Register terminal handle with parent ref map for programmatic focus. */
  const setTerminalRef = useCallback(
    (handle: TerminalCardHandle | null) => {
      terminalRef.current = handle
      if (handle) {
        onRegister(panel.id, handle)
      } else {
        onUnregister(panel.id)
      }
    },
    [panel.id, onRegister, onUnregister],
  )

  const handleDisconnect = useTerminalDisconnect(
    panel.projectId,
    panel.agentType,
    panel.worktreeId,
    panel.id,
    terminalRef,
    onRemove,
  )

  /** Clicks on the overlay remove it and focus the terminal underneath. */
  const handleOverlayClick = () => {
    // Directly set focus state so the overlay disappears. We can't rely
    // solely on terminalRef.focus() triggering onFocusCapture — when the
    // changes tab is active, xterm is hidden and can't receive focus.
    setIsFocused(true)
    onFocusChange(panel.id, true)
    terminalRef.current?.focus()
  }

  return (
    <div
      ref={setNodeRef}
      data-testid={`grid-cell-${panel.worktreeId}`}
      data-panel-id={panel.id}
      className={cn(
        'border-border relative min-h-0 overflow-hidden rounded border shadow-sm',
        isDragging && 'z-50 opacity-80',
      )}
      style={sortableStyle}
      onFocusCapture={handleFocusCapture}
      onBlurCapture={handleBlurCapture}
    >
      <TerminalCard
        ref={setTerminalRef}
        projectId={panel.projectId}
        agentType={panel.agentType}
        worktreeId={panel.worktreeId}
        projectName={worktreeDisplayName(panel.worktreeId, panel.projectName)}
        branch={panel.branch}
        isActive={isActive}
        isFocused={isFocused}
        autoFocus
        stateDotClass={stateDotClass}
        stateLabel={stateLabel}
        needsInput={needsInput}
        attentionDotClass={attentionDotClass}
        attentionLabel={attentionLabel}
        onDisconnect={handleDisconnect}
        titleBarClassName="cursor-grab active:cursor-grabbing"
        dragHandleProps={{ ...attributes, ...listeners }}
      />
      {/* Transparent overlay blocks xterm wheel capture when unfocused,
          allowing the grid container to scroll. Click passes focus through.
          top-8 leaves the title bar uncovered so tab buttons and actions
          remain clickable even when the terminal is unfocused. */}
      {!isFocused && !isDragging && (
        <div className="absolute inset-0 top-8 z-10" onClick={handleOverlayClick} />
      )}
    </div>
  )
}

/** Memoized grid cell — re-renders only when its own data changes. */
const GridCell = memo(GridCellInner, (prev, next) => {
  // Only compare data props — callback props are stable (useCallback) so
  // they never trigger re-renders from parent state changes.
  return (
    prev.panel.id === next.panel.id &&
    prev.panel.branch === next.panel.branch &&
    prev.isActive === next.isActive &&
    prev.needsInput === next.needsInput &&
    prev.attentionDotClass === next.attentionDotClass &&
    prev.attentionLabel === next.attentionLabel &&
    prev.stateLabel === next.stateLabel &&
    prev.stateDotClass === next.stateDotClass
  )
})

/** Handle exposed by GridView for programmatic panel focus. */
export interface GridViewHandle {
  /** Scrolls to and focuses the terminal for the given panel. */
  focusPanel: (panelId: string) => void
}

/** Props for the grid view component. */
interface GridViewProps {
  panels: CanvasPanelData[]
  worktreeStates: Map<string, PanelWorktreeState>
  onRemovePanel: (panelId: string) => void
  /** Called when a terminal cell gains or loses focus. */
  onFocusChange: (panelId: string, focused: boolean) => void
  /** Called when the user drag-reorders panels. */
  onReorder: (fromIndex: number, toIndex: number) => void
}

/**
 * Displays connected terminals in a sortable CSS grid layout.
 *
 * Panels can be drag-reordered via the title bar. The grid auto-sizes
 * to max 2 columns with scrolling after 4 panels.
 */
const GridView = forwardRef<GridViewHandle, GridViewProps>(function GridView(
  { panels, worktreeStates, onRemovePanel, onFocusChange, onReorder },
  ref,
) {
  const terminalRefs = useRef<Map<string, TerminalCardHandle>>(new Map())

  const handleRegister = useCallback((panelId: string, handle: TerminalCardHandle) => {
    terminalRefs.current.set(panelId, handle)
  }, [])

  const handleUnregister = useCallback((panelId: string) => {
    terminalRefs.current.delete(panelId)
  }, [])

  useImperativeHandle(ref, () => ({
    focusPanel(panelId: string) {
      const cell = document.querySelector(`[data-panel-id="${panelId}"]`)
      if (cell) {
        cell.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
        terminalRefs.current.get(panelId)?.focus()
      }
    },
  }))

  // ─── Drag-and-drop ──────────────────────────────────────────────────
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: POINTER_ACTIVATION }),
    useSensor(KeyboardSensor),
  )

  const panelIds = useMemo(() => panels.map((p) => p.id), [panels])

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event
      if (!over || active.id === over.id) return

      const oldIndex = panels.findIndex((p) => p.id === active.id)
      const newIndex = panels.findIndex((p) => p.id === over.id)
      if (oldIndex !== -1 && newIndex !== -1) {
        onReorder(oldIndex, newIndex)
      }
    },
    [panels, onReorder],
  )

  // ─── Render ─────────────────────────────────────────────────────────
  if (panels.length === 0) {
    return (
      <div data-testid="grid-empty-state" className="flex flex-1 items-center justify-center">
        <div className="text-center">
          <p className="text-muted-foreground">Select a worktree from the sidebar</p>
          <div className="text-muted-foreground/60 mt-3 space-y-1 text-sm">
            <p>Drag panels to reorder</p>
            <p>
              <kbd className="border-border/40 rounded border px-1.5 py-0.5 text-xs">Ctrl</kbd>
              {' + '}
              <kbd className="border-border/40 rounded border px-1.5 py-0.5 text-xs">Shift</kbd>
              {' + '}
              <kbd className="border-border/40 rounded border px-1.5 py-0.5 text-xs">C</kbd>
              {' — Copy'}
            </p>
            <p>
              <kbd className="border-border/40 rounded border px-1.5 py-0.5 text-xs">Ctrl</kbd>
              {' + '}
              <kbd className="border-border/40 rounded border px-1.5 py-0.5 text-xs">Shift</kbd>
              {' + '}
              <kbd className="border-border/40 rounded border px-1.5 py-0.5 text-xs">V</kbd>
              {' — Paste'}
            </p>
          </div>
        </div>
      </div>
    )
  }

  const rowCount = Math.ceil(panels.length / (panels.length === 1 ? 1 : 2))
  const needsScroll = rowCount > 2

  return (
    <div data-testid="grid-view" className="relative min-h-0 flex-1">
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
        <SortableContext items={panelIds} strategy={rectSortingStrategy}>
          <div
            className={cn(
              'absolute inset-0 grid gap-2 p-2',
              panels.length === 1 ? 'grid-cols-1' : 'grid-cols-2',
              needsScroll && 'overflow-y-auto',
            )}
            style={{
              // Each row is half the container minus half the gap-2 (0.5rem).
              gridAutoRows: rowCount <= 1 ? '1fr' : 'calc(50% - 0.25rem)',
            }}
          >
            {panels.map((panel) => {
              const state = worktreeStates.get(panel.id)
              return (
                <GridCell
                  key={panel.id}
                  panel={panel}
                  isActive={state?.isActive ?? false}
                  needsInput={state?.needsInput ?? false}
                  attentionDotClass={state?.attentionDotClass}
                  attentionLabel={state?.attentionLabel}
                  stateLabel={state?.stateLabel}
                  stateDotClass={state?.stateDotClass ?? 'bg-muted-foreground/40'}
                  onRemove={onRemovePanel}
                  onFocusChange={onFocusChange}
                  onRegister={handleRegister}
                  onUnregister={handleUnregister}
                />
              )
            })}
          </div>
        </SortableContext>
      </DndContext>
    </div>
  )
})

export default GridView

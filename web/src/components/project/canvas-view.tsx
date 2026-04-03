import { memo, useCallback, useEffect, useRef, useState } from 'react'
import { Rnd, type RndResizeCallback, type RndDragCallback } from 'react-rnd'
import { Maximize2, Minimize2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { worktreeDisplayName } from '@/lib/types'
import { cn } from '@/lib/utils'
import TerminalCard, { type TerminalCardHandle } from '@/components/project/terminal-card'
import { useTerminalDisconnect } from '@/hooks/use-terminal-disconnect'
import { LAYOUT_ANIMATION_MS, type CanvasPanel as CanvasPanelData } from '@/lib/canvas-store'

/** Props for the CanvasView component. */
interface CanvasViewProps {
  panel: CanvasPanelData
  /** Whether the terminal has an active connection (connected or shell state). */
  isActive: boolean
  /** Whether the worktree needs user attention. */
  needsInput: boolean
  /** Attention dot CSS class (e.g. from notification config). */
  attentionDotClass?: string
  /** Attention label for tooltip. */
  attentionLabel?: string
  /** Whether this panel is currently focused (highest z-index). */
  isFocused: boolean
  /** Whether this panel is currently selected. */
  isSelected: boolean
  /** Whether a layout animation is in progress. */
  isLayoutAnimating: boolean
  /** Worktree state label (e.g. "Agent exited"). */
  stateLabel?: string
  /** Worktree state dot CSS class. */
  stateDotClass: string
  /** Current zoom scale of the canvas (1 = 100%). */
  scale: number
  onRemove: (panelId: string) => void
  onDragStop: (panelId: string, x: number, y: number) => void
  onResizeStop: (panelId: string, width: number, height: number, x: number, y: number) => void
  onFocus: (panelId: string) => void
  /** Called on shift+click to toggle selection. */
  onShiftClick: (panelId: string) => void
  onMaximize: (panelId: string) => void
  /** Called on drag/resize start to clear layout animation. */
  onInteractionStart: () => void
}

/** Minimum panel dimensions in pixels. */
const MIN_WIDTH = 320
const MIN_HEIGHT = 240

/**
 * A draggable, resizable panel on the canvas that embeds a TerminalCard.
 *
 * Uses controlled position/size props driven by panel.geometry in the
 * store. Layout animations are applied via CSS transitions when
 * isLayoutAnimating is true.
 *
 * The disconnect button closes the terminal viewer. The tmux session
 * and Claude continue running in the background. The worktree reappears
 * in the sidebar as reconnectable.
 */
function CanvasViewInner({
  panel,
  isActive,
  isFocused,
  isSelected,
  isLayoutAnimating,
  needsInput,
  attentionDotClass,
  attentionLabel,
  stateLabel,
  stateDotClass,
  scale,
  onRemove,
  onDragStop,
  onResizeStop,
  onFocus,
  onShiftClick,
  onMaximize,
  onInteractionStart,
}: CanvasViewProps) {
  const [isDragging, setIsDragging] = useState(false)
  const terminalRef = useRef<TerminalCardHandle>(null)

  // When the panel gains focus, give keyboard focus to the xterm instance.
  // Deferred to next frame so the browser's mousedown→mouseup focus
  // sequence completes first — otherwise the clicked element steals focus
  // back from the xterm textarea.
  useEffect(() => {
    if (!isFocused) return

    const rafId = requestAnimationFrame(() => {
      terminalRef.current?.focus()
    })
    return () => cancelAnimationFrame(rafId)
  }, [isFocused])

  const handleMouseDown = useCallback(
    (e: MouseEvent) => {
      if (e.shiftKey) {
        onShiftClick(panel.id)
      } else {
        onFocus(panel.id)
      }
    },
    [onFocus, onShiftClick, panel.id],
  )

  const handleDisconnect = useTerminalDisconnect(
    panel.projectId,
    panel.agentType,
    panel.worktreeId,
    panel.id,
    terminalRef,
    onRemove,
  )

  /** Toggles maximize via the parent callback. */
  const handleMaximize = useCallback(() => {
    onMaximize(panel.id)
  }, [panel.id, onMaximize])

  const handleInteractionStart = useCallback(() => {
    onInteractionStart()
    setIsDragging(true)
  }, [onInteractionStart])

  const handleDragStop: RndDragCallback = useCallback(
    (_e, d) => {
      setIsDragging(false)
      onDragStop(panel.id, d.x, d.y)
    },
    [panel.id, onDragStop],
  )

  const handleResizeStop: RndResizeCallback = useCallback(
    (_e, _dir, ref, _delta, position) => {
      setIsDragging(false)
      onResizeStop(panel.id, ref.offsetWidth, ref.offsetHeight, position.x, position.y)
    },
    [panel.id, onResizeStop],
  )

  const maximizeButton = (
    <Button
      data-testid="maximize-button"
      variant="ghost"
      size="icon"
      className="h-5 w-5 shrink-0"
      onClick={handleMaximize}
      title={panel.isMaximized ? 'Restore' : 'Maximize'}
      icon={panel.isMaximized ? Minimize2 : Maximize2}
    />
  )

  return (
    <Rnd
      data-testid={`canvas-panel-${panel.worktreeId}`}
      data-canvas-panel
      position={{ x: panel.geometry.x, y: panel.geometry.y }}
      size={{ width: panel.geometry.width, height: panel.geometry.height }}
      minWidth={MIN_WIDTH}
      minHeight={MIN_HEIGHT}
      dragHandleClassName="canvas-panel-handle"
      disableDragging={panel.isMaximized}
      enableResizing={!panel.isMaximized}
      style={{
        zIndex: panel.zIndex,
        contain: 'strict',
        contentVisibility: 'auto',
        containIntrinsicSize: `${panel.geometry.width}px ${panel.geometry.height}px`,
        transition: isLayoutAnimating
          ? `transform ${LAYOUT_ANIMATION_MS}ms ease-out, width ${LAYOUT_ANIMATION_MS}ms ease-out, height ${LAYOUT_ANIMATION_MS}ms ease-out`
          : undefined,
      }}
      onDragStart={handleInteractionStart}
      onDragStop={handleDragStop}
      onResizeStart={handleInteractionStart}
      onResizeStop={handleResizeStop}
      onMouseDown={handleMouseDown}
      scale={scale}
    >
      <div
        className={cn(
          'relative h-full overflow-hidden rounded border shadow-lg',
          isSelected ? 'border-marquee ring-marquee/40 ring-1' : 'border-border',
        )}
      >
        <TerminalCard
          ref={terminalRef}
          projectId={panel.projectId}
          agentType={panel.agentType}
          worktreeId={panel.worktreeId}
          projectName={worktreeDisplayName(panel.worktreeId, panel.projectName)}
          branch={panel.branch}
          isActive={isActive}
          isFocused={isFocused}
          terminalInert={!isFocused}
          stateDotClass={stateDotClass}
          stateLabel={stateLabel}
          needsInput={needsInput}
          attentionDotClass={attentionDotClass}
          attentionLabel={attentionLabel}
          onDisconnect={handleDisconnect}
          titleBarClassName={cn(
            'canvas-panel-handle',
            !panel.isMaximized && 'cursor-grab active:cursor-grabbing',
          )}
          actions={maximizeButton}
        />
        {/* Drag overlay — blocks terminal interactions during drag */}
        <div className={cn('absolute inset-0', isDragging ? 'block' : 'hidden')} />
      </div>
    </Rnd>
  )
}

/**
 * Memoized canvas view — only re-renders when its own data changes,
 * not on every parent re-render from polling.
 */
const CanvasView = memo(CanvasViewInner, (prev, next) => {
  // Only compare data props — callback props are stable (useCallback with
  // refs) so they never trigger re-renders. This prevents SSE-driven
  // worktree state updates from cascading into every canvas panel.
  return (
    prev.panel.id === next.panel.id &&
    prev.panel.zIndex === next.panel.zIndex &&
    prev.panel.branch === next.panel.branch &&
    prev.panel.isMaximized === next.panel.isMaximized &&
    prev.panel.geometry.x === next.panel.geometry.x &&
    prev.panel.geometry.y === next.panel.geometry.y &&
    prev.panel.geometry.width === next.panel.geometry.width &&
    prev.panel.geometry.height === next.panel.geometry.height &&
    prev.isActive === next.isActive &&
    prev.isFocused === next.isFocused &&
    prev.isSelected === next.isSelected &&
    prev.isLayoutAnimating === next.isLayoutAnimating &&
    prev.needsInput === next.needsInput &&
    prev.attentionDotClass === next.attentionDotClass &&
    prev.attentionLabel === next.attentionLabel &&
    prev.stateLabel === next.stateLabel &&
    prev.stateDotClass === next.stateDotClass &&
    prev.scale === next.scale
  )
})

export default CanvasView

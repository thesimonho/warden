import { forwardRef, useImperativeHandle, useState, type ReactNode } from 'react'
import {
  GitBranch,
  Unplug,
  Terminal,
  GitCompareArrows,
  Copy,
  Clipboard,
  BoxSelect,
  Image,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import { cn } from '@/lib/utils'
import { useTerminal } from '@/hooks/use-terminal'
import { ChangesView } from '@/components/project/changes-view'
import '@xterm/xterm/css/xterm.css'

/** Active tab in the terminal card. */
type TerminalTab = 'terminal' | 'changes'

/** Handle exposed by TerminalCard for parent-driven cleanup and focus. */
export interface TerminalCardHandle {
  /** Tears down the WebSocket and terminal instance. */
  detach: () => void
  /** Gives keyboard focus to the xterm.js instance. */
  focus: () => void
}

/** Props for the TerminalCard component. */
export interface TerminalCardProps {
  /** Container ID for the project. */
  projectId: string
  /** Worktree identifier within the project. */
  worktreeId: string
  /** Project display name shown in the title bar. */
  projectName: string
  /** Git branch name, if any. */
  branch?: string
  /** Whether the terminal has an active connection (connected or shell state). */
  isActive: boolean
  /** Whether this card has visual focus (highlighted title bar). */
  isFocused?: boolean
  /** Worktree state dot CSS class (e.g. 'bg-success'). */
  stateDotClass: string
  /** Worktree state label for the dot tooltip. */
  stateLabel?: string
  /** Whether the worktree needs user attention. */
  needsInput?: boolean
  /** Attention dot CSS class (e.g. from notification config). */
  attentionDotClass?: string
  /** Attention label for the dot tooltip. */
  attentionLabel?: string
  /** Called when the user clicks the disconnect button. */
  onDisconnect: () => void
  /** Extra CSS class applied to the title bar (e.g. drag handle class). */
  titleBarClassName?: string
  /** Props spread onto the title bar to make it a drag handle (e.g. dnd-kit listeners). */
  dragHandleProps?: React.HTMLAttributes<HTMLDivElement>
  /** Additional action buttons rendered after the disconnect button. */
  actions?: ReactNode
  /** When true, disables pointer events on the terminal content (not the title bar). */
  terminalInert?: boolean
  /** Extra CSS class applied to the outer container. */
  className?: string
  /** Data attribute for testing. */
  'data-testid'?: string
}

/**
 * A terminal with title bar chrome — status dot, project name, branch,
 * attention indicator, and action buttons.
 *
 * The xterm.js rendering is handled internally via the `useTerminal` hook.
 * This is the single component used by both canvas-view (wrapped in Rnd)
 * and grid-view (in a CSS grid cell).
 *
 * Layout-specific concerns (drag handles, maximize, selection rings) are
 * passed in via props — this component is layout-agnostic.
 */
const TerminalCard = forwardRef<TerminalCardHandle, TerminalCardProps>(function TerminalCard(
  {
    projectId,
    worktreeId,
    projectName,
    branch,
    isActive,
    isFocused = false,
    stateDotClass: dotClass,
    stateLabel: dotLabel,
    needsInput = false,
    attentionDotClass,
    attentionLabel,
    onDisconnect,
    titleBarClassName,
    dragHandleProps,
    actions,
    terminalInert = false,
    className,
    'data-testid': testId,
  },
  ref,
) {
  const [activeTab, setActiveTab] = useState<TerminalTab>('terminal')

  const { containerRef, detach, focus, fit, clipboard } = useTerminal({
    projectId,
    worktreeId,
    isActive,
    isFocused: isFocused && activeTab === 'terminal',
  })

  useImperativeHandle(ref, () => ({
    detach,
    focus: () => {
      // Only give keyboard focus — don't force-switch tabs. This prevents
      // parent focus events (e.g. clicking a canvas panel) from closing
      // the changes view.
      if (activeTab === 'terminal') {
        focus()
      }
    },
  }))

  return (
    <div
      data-testid={testId}
      className={cn('bg-background flex h-full flex-col overflow-hidden', className)}
    >
      {/* Title bar */}
      <div
        className={cn(
          'border-border flex shrink-0 items-center justify-between border-b px-3 py-1.5 select-none',
          isFocused ? 'bg-secondary/65 text-secondary-foreground' : 'bg-muted/50',
          titleBarClassName,
        )}
        {...dragHandleProps}
        onClick={focus}
      >
        <div className="flex items-center gap-2 overflow-hidden">
          <span
            className={cn('inline-block h-2 w-2 shrink-0 rounded-full', dotClass)}
            title={dotLabel}
          />
          <span className="truncate font-medium">{projectName}</span>
          {branch && (
            <span
              className={cn(
                'flex items-center gap-1 text-sm',
                isFocused ? 'text-secondary-foreground/60' : 'text-muted-foreground',
              )}
            >
              <GitBranch className="h-3 w-3" />
              <span className="truncate">{branch}</span>
            </span>
          )}
          {needsInput && attentionDotClass && (
            <span
              className={cn('inline-block h-2 w-2 shrink-0 rounded-full', attentionDotClass)}
              title={attentionLabel}
            />
          )}
        </div>

        <div className="flex items-center gap-1.5" onMouseDown={(e) => e.stopPropagation()}>
          {/* Tab toggle */}
          <div className="bg-muted/60 flex shrink-0 items-center rounded p-0.5">
            <button
              type="button"
              className={cn(
                'rounded px-2 py-0.5 text-sm transition-colors',
                activeTab === 'terminal'
                  ? 'bg-background text-foreground shadow-xs'
                  : 'text-muted-foreground hover:text-foreground',
              )}
              onClick={() => {
                setActiveTab('terminal')
                // Re-fit terminal on next frame after becoming visible —
                // the container was hidden (display:none) so xterm needs
                // to recalculate dimensions before focusing.
                requestAnimationFrame(() => {
                  fit()
                  focus()
                })
              }}
              title="Terminal"
            >
              <Terminal className="h-3.5 w-3.5" />
            </button>
            <button
              type="button"
              className={cn(
                'rounded px-2 py-0.5 text-sm transition-colors',
                activeTab === 'changes'
                  ? 'bg-background text-foreground shadow-xs'
                  : 'text-muted-foreground hover:text-foreground',
              )}
              onClick={() => setActiveTab('changes')}
              title="Changes"
            >
              <GitCompareArrows className="h-3.5 w-3.5" />
            </button>
          </div>

          {actions}
          <Button
            data-testid="disconnect-button"
            variant="ghost"
            size="icon"
            color="error"
            className="h-5 w-5 shrink-0"
            onClick={onDisconnect}
            title="Disconnect"
            icon={Unplug}
          />
        </div>
      </div>

      {/* Terminal content — kept mounted but hidden when Changes tab is active */}
      {isActive ? (
        <ContextMenu>
          <ContextMenuTrigger asChild>
            <div
              className={cn(
                'min-h-0 flex-1',
                terminalInert && 'pointer-events-none',
                activeTab !== 'terminal' && 'hidden',
              )}
            >
              <div
                ref={containerRef}
                data-testid="terminal-container"
                className="h-full w-full overflow-hidden border-0"
                style={{ backgroundColor: 'var(--terminal-background)' }}
              />
            </div>
          </ContextMenuTrigger>
          <ContextMenuContent className="w-64">
            <ContextMenuItem onClick={clipboard.copySelection}>
              <Copy className="size-4" />
              Copy
              <ContextMenuShortcut>Ctrl+Shift+C</ContextMenuShortcut>
            </ContextMenuItem>
            <ContextMenuItem onClick={clipboard.pasteText}>
              <Clipboard className="size-4" />
              Paste
              <ContextMenuShortcut>Ctrl+Shift+V</ContextMenuShortcut>
            </ContextMenuItem>
            <ContextMenuItem onClick={() => clipboard.pasteImageFromClipboard()}>
              <Image className="size-4" />
              Paste Image
              <ContextMenuShortcut>Ctrl+V</ContextMenuShortcut>
            </ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem onClick={clipboard.selectAll}>
              <BoxSelect className="size-4" />
              Select All
              <ContextMenuShortcut>Ctrl+Shift+A</ContextMenuShortcut>
            </ContextMenuItem>
          </ContextMenuContent>
        </ContextMenu>
      ) : (
        activeTab === 'terminal' && (
          <div
            className="flex h-full w-full items-center justify-center"
            data-testid="terminal-disconnected"
          >
            <p className="text-muted-foreground">Terminal disconnected</p>
          </div>
        )
      )}

      {/* Changes tab — only rendered when active */}
      {activeTab === 'changes' && (
        <div className="min-h-0 flex-1">
          <ChangesView projectId={projectId} worktreeId={worktreeId} />
        </div>
      )}
    </div>
  )
})

export default TerminalCard

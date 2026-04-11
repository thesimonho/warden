import { forwardRef, useImperativeHandle, useState, type ReactNode } from 'react'
import {
  GitBranch,
  Unplug,
  Bot,
  SquareTerminal,
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

/**
 * Active tab in the terminal card.
 *
 * - `'agent'` — the agent tmux session (Claude Code / Codex). Always active
 *   whenever the card is connected.
 * - `'shell'` — a plain bash session at the worktree's working directory.
 *   Backed by a separate tmux session (`warden-shell-{wid}`) that is lazily
 *   bootstrapped on first click and persists for the lifetime of the worktree.
 * - `'changes'` — the git diff view.
 */
type TerminalTab = 'agent' | 'shell' | 'changes'

/** Clipboard actions surfaced by the context menu. Matches useTerminalClipboard. */
interface PaneClipboard {
  copySelection: () => void
  pasteText: () => void
  pasteImageFromClipboard: () => void | Promise<void>
  selectAll: () => void
}

interface TerminalPaneProps {
  containerRef: React.RefObject<HTMLDivElement | null>
  clipboard: PaneClipboard
  hidden: boolean
  inert: boolean
  testId: string
}

/**
 * The xterm.js container + its context menu. Shared by both the agent and
 * shell tabs — they differ only in their containerRef, clipboard binding,
 * visibility, and testid.
 */
function TerminalPane({ containerRef, clipboard, hidden, inert, testId }: TerminalPaneProps) {
  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>
        <div className={cn('min-h-0 flex-1', inert && 'pointer-events-none', hidden && 'hidden')}>
          <div
            ref={containerRef}
            data-testid={testId}
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
  )
}

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
  /** CLI agent type for this project. */
  agentType: string
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
  /**
   * When true, gives keyboard focus to xterm after it attaches to the DOM.
   * Solves the race where focus is requested before xterm.open() completes.
   */
  autoFocus?: boolean
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
    agentType,
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
    autoFocus = false,
    terminalInert = false,
    className,
    'data-testid': testId,
  },
  ref,
) {
  const [activeTab, setActiveTab] = useState<TerminalTab>('agent')
  // Lazy: don't spin up the shell tmux session until the user actually
  // opens the shell tab once. Once flipped it never flips back — the shell
  // stays alive in the background so returning to the tab is instant.
  const [hasOpenedShell, setHasOpenedShell] = useState(false)

  const { containerRef, detach, focus, fit, clipboard } = useTerminal({
    projectId,
    agentType,
    worktreeId,
    isActive,
    isFocused: isFocused && activeTab === 'agent',
    autoFocus,
    mode: 'agent',
  })

  const {
    containerRef: shellContainerRef,
    detach: shellDetach,
    focus: shellFocus,
    fit: shellFit,
    clipboard: shellClipboard,
  } = useTerminal({
    projectId,
    agentType,
    worktreeId,
    // Gate shell activation on the user having opened the tab at least once.
    // This avoids eagerly bootstrapping a shell tmux session for every
    // terminal card on app load.
    isActive: isActive && hasOpenedShell,
    isFocused: isFocused && activeTab === 'shell',
    autoFocus: false,
    mode: 'shell',
  })

  useImperativeHandle(ref, () => ({
    detach: () => {
      detach()
      shellDetach()
    },
    focus: () => {
      // Only give keyboard focus — don't force-switch tabs. This prevents
      // parent focus events (e.g. clicking a canvas panel) from closing
      // the changes view.
      if (activeTab === 'agent') {
        focus()
      } else if (activeTab === 'shell') {
        shellFocus()
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
          {/* Tab toggle: agent, shell, changes */}
          <div className="bg-muted/60 flex shrink-0 items-center rounded p-0.5">
            <button
              type="button"
              className={cn(
                'rounded px-2 py-0.5 text-sm transition-colors',
                activeTab === 'agent'
                  ? 'bg-background text-foreground shadow-xs'
                  : 'text-muted-foreground hover:text-foreground',
              )}
              onClick={() => {
                setActiveTab('agent')
                // Re-fit terminal on next frame after becoming visible —
                // the container was hidden (display:none) so xterm needs
                // to recalculate dimensions before focusing.
                requestAnimationFrame(() => {
                  fit()
                  focus()
                })
              }}
              title="Agent"
              data-testid="tab-agent"
            >
              <Bot className="h-3.5 w-3.5" />
            </button>
            <button
              type="button"
              className={cn(
                'rounded px-2 py-0.5 text-sm transition-colors',
                activeTab === 'shell'
                  ? 'bg-background text-foreground shadow-xs'
                  : 'text-muted-foreground hover:text-foreground',
              )}
              onClick={() => {
                setActiveTab('shell')
                // Lazy-activate the shell tmux session the first time the
                // user opens this tab. All subsequent clicks are free.
                setHasOpenedShell(true)
                requestAnimationFrame(() => {
                  shellFit()
                  shellFocus()
                })
              }}
              title="Terminal"
              data-testid="tab-shell"
            >
              <SquareTerminal className="h-3.5 w-3.5" />
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
              data-testid="tab-changes"
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

      {/* Both terminals stay mounted so tab switching reattaches to live sessions. */}
      {isActive ? (
        <TerminalPane
          containerRef={containerRef}
          clipboard={clipboard}
          hidden={activeTab !== 'agent'}
          inert={terminalInert}
          testId="terminal-container"
        />
      ) : (
        activeTab === 'agent' && (
          <div
            className="flex h-full w-full items-center justify-center"
            data-testid="terminal-disconnected"
          >
            <p className="text-muted-foreground">Terminal disconnected</p>
          </div>
        )
      )}

      {isActive && hasOpenedShell ? (
        <TerminalPane
          containerRef={shellContainerRef}
          clipboard={shellClipboard}
          hidden={activeTab !== 'shell'}
          inert={terminalInert}
          testId="shell-terminal-container"
        />
      ) : (
        activeTab === 'shell' && (
          <div
            className="flex h-full w-full items-center justify-center"
            data-testid="shell-terminal-disconnected"
          >
            <p className="text-muted-foreground">
              {isActive ? 'Starting shell…' : 'Terminal disconnected'}
            </p>
          </div>
        )
      )}

      {activeTab === 'changes' && (
        <div className="min-h-0 flex-1">
          <ChangesView projectId={projectId} agentType={agentType} worktreeId={worktreeId} />
        </div>
      )}
    </div>
  )
})

export default TerminalCard

import {
  FolderOpen,
  GitBranch,
  Info,
  Plus,
  RefreshCw,
  RotateCcw,
  Square,
  Trash2,
  Unplug,
} from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { toast } from 'sonner'

import NewWorktreeDialog from '@/components/project/new-worktree-dialog'
import RemoveWorktreeDialog from '@/components/project/remove-worktree-dialog'
import { Button } from '@/components/ui/button'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import { Separator } from '@/components/ui/separator'
import { cleanupWorktrees } from '@/lib/api'
import { buildPanelId } from '@/lib/canvas-store'
import { getAttentionConfig } from '@/lib/notification-config'
import type { Worktree } from '@/lib/types'
import { hasActiveTerminal, worktreeDisplayName, worktreeStateIndicator } from '@/lib/types'
import { cn } from '@/lib/utils'

/** Returns a display label for a worktree — project name for main, ID otherwise. */
function worktreeLabel(worktree: Worktree, projectName: string): string {
  return worktreeDisplayName(worktree.id, projectName)
}

/** Props for the worktree list component. */
interface WorktreeListProps {
  projectId: string
  agentType: string
  projectName: string
  isGitRepo: boolean
  worktrees: Worktree[]
  isLoading: boolean
  onWorktreeCreated: (worktreeId: string) => void
  onRefreshComplete?: () => void
  /** Assigns a group label to each worktree. When provided, items are partitioned into sections. */
  groupBy?: (worktree: Worktree) => string
  /** Controls the order in which groups appear. Only used when `groupBy` is set. */
  groupOrder?: string[]
  /** Set of panel IDs currently on the canvas, used to show "On Canvas" vs connect icon. */
  activePanelIds: Set<string>
  /** ID of the currently focused panel. */
  focusedPanelId: string | null
  /** ID of the worktree currently being connected. */
  connectingId: string | null
  onAdd: (worktree: Worktree) => void
  onFocus: (worktree: Worktree) => void
  onDisconnect: (worktreeId: string) => void
  onStop: (worktreeId: string) => void
  onReset: (worktreeId: string) => void
  onRemove: (worktreeId: string) => void
  /** Opens a worktree's host directory in the system file manager. */
  onReveal?: (worktree: Worktree) => void
  /** Controlled open state for the new worktree dialog (parent owns the state). */
  newDialogOpen?: boolean
  /** Callback when the new worktree dialog open state changes. */
  onNewDialogOpenChange?: (open: boolean) => void
  /** Called when the "New" button is clicked. Parent can gate this (e.g. budget check). */
  onRequestNewWorktree?: () => void
}

/** A named group of worktrees for sectioned rendering. */
interface WorktreeGroup {
  label: string
  items: Worktree[]
}

/**
 * Partitions worktrees into ordered groups.
 *
 * Preserves the original array order within each group. All groups
 * in groupOrder are included even if empty, so headers always render.
 */
function partitionWorktrees(
  worktrees: Worktree[],
  groupBy: (worktree: Worktree) => string,
  groupOrder: string[],
): WorktreeGroup[] {
  const grouped = new Map<string, Worktree[]>()

  for (const wt of worktrees) {
    const label = groupBy(wt)
    const existing = grouped.get(label)
    if (existing) {
      existing.push(wt)
    } else {
      grouped.set(label, [wt])
    }
  }

  const orderedLabels = [
    ...groupOrder.filter((label) => grouped.has(label)),
    ...[...grouped.keys()].filter((label) => !groupOrder.includes(label)),
  ]

  return orderedLabels.map((label) => ({ label, items: grouped.get(label) ?? [] }))
}

/**
 * Worktree list with header, items, and dialogs.
 *
 * Header contains the "New Worktree" button and a refresh button.
 * Each worktree renders as a row with status dot, branch, and
 * connect/focus actions. Right-click context menu provides
 * Disconnect and Remove.
 */
export default function WorktreeList({
  projectId,
  agentType,
  projectName,
  isGitRepo,
  worktrees,
  isLoading,
  onWorktreeCreated,
  onRefreshComplete,
  groupBy,
  groupOrder = [],
  activePanelIds,
  focusedPanelId,
  connectingId,
  onAdd,
  onFocus,
  onDisconnect,
  onStop,
  onReset,
  onRemove,
  onReveal,
  newDialogOpen,
  onNewDialogOpenChange,
  onRequestNewWorktree,
}: WorktreeListProps) {
  // Use controlled state from parent when provided, otherwise manage internally.
  const [internalDialogOpen, setInternalDialogOpen] = useState(false)
  const isDialogOpen = newDialogOpen ?? internalDialogOpen
  const setIsDialogOpen = onNewDialogOpenChange ?? setInternalDialogOpen

  const [isRefreshing, setIsRefreshing] = useState(false)

  const groups = useMemo(() => {
    if (!groupBy) return null
    return partitionWorktrees(worktrees, groupBy, groupOrder)
  }, [worktrees, groupBy, groupOrder])

  /** Whether to render section headers. */
  const shouldShowHeaders = groups !== null

  /** Cleans up orphaned worktree directories and refreshes the list. */
  const handleRefresh = useCallback(async () => {
    setIsRefreshing(true)
    try {
      const result = await cleanupWorktrees(projectId, agentType)
      const count = result.removed?.length ?? 0
      if (count > 0) {
        toast.success(`Removed ${count} unused worktree${count > 1 ? 's' : ''}`)
      }
      onRefreshComplete?.()
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error'
      toast.error('Failed to refresh worktrees', { description: message })
    } finally {
      setIsRefreshing(false)
    }
  }, [projectId, agentType, onRefreshComplete])

  const renderItem = (wt: Worktree) => {
    const panelId = buildPanelId(projectId, wt.id)
    const isOnCanvas = activePanelIds.has(panelId)

    return (
      <WorktreeRow
        key={wt.id}
        worktree={wt}
        projectName={projectName}
        isOnCanvas={isOnCanvas}
        isFocused={panelId === focusedPanelId}
        isConnecting={connectingId === wt.id}
        onAdd={() => onAdd(wt)}
        onFocus={() => onFocus(wt)}
        onDisconnect={() => onDisconnect(wt.id)}
        onStop={() => onStop(wt.id)}
        onReset={() => onReset(wt.id)}
        onRemove={() => onRemove(wt.id)}
        onReveal={onReveal ? () => onReveal(wt) : undefined}
      />
    )
  }

  return (
    <>
      {/* Header */}
      {isGitRepo ? (
        <div className="flex items-center justify-between p-2">
          <Button
            data-testid="new-worktree-button"
            size="sm"
            onClick={onRequestNewWorktree ?? (() => setIsDialogOpen(true))}
            icon={Plus}
          >
            New
          </Button>
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground text-sm">
              {worktrees.length} {worktrees.length === 1 ? 'worktree' : 'worktrees'}
            </span>
            <Button
              variant="ghost"
              size="icon"
              className="text-muted-foreground h-5 w-5"
              disabled={isRefreshing}
              onClick={handleRefresh}
              icon={RefreshCw}
              loading={isRefreshing}
            />
          </div>
        </div>
      ) : (
        <div className="text-muted-foreground flex items-center gap-1.5 border-b p-2 text-sm">
          <Info className="h-3.5 w-3.5 shrink-0" />
          Non-git project
        </div>
      )}

      {/* List */}
      <div data-testid="worktree-list" className="min-w-0 flex-1 space-y-0.5 overflow-y-auto p-2">
        {isLoading && (
          <p className="text-muted-foreground px-2 py-1 text-sm">Loading worktrees...</p>
        )}

        {!isLoading && worktrees.length === 0 && (
          <p className="text-muted-foreground px-2 py-1 text-sm">No worktrees found.</p>
        )}

        {shouldShowHeaders
          ? groups.map((group, groupIndex) => (
              <div key={group.label}>
                {groupIndex > 0 && <Separator className="my-4" />}
                <p
                  className={`text-muted-foreground px-2 pb-1 text-xs font-medium tracking-wider uppercase ${groupIndex === 0 ? 'pt-2' : 'pt-1'}`}
                >
                  {group.label}
                </p>
                {group.items.map(renderItem)}
              </div>
            ))
          : worktrees.map(renderItem)}
      </div>

      {isGitRepo && (
        <NewWorktreeDialog
          projectId={projectId}
          agentType={agentType}
          open={isDialogOpen}
          onOpenChange={setIsDialogOpen}
          onCreated={onWorktreeCreated}
        />
      )}
    </>
  )
}

// ---------------------------------------------------------------------------
// WorktreeRow
// ---------------------------------------------------------------------------

/** Props for a single worktree row. */
interface WorktreeRowProps {
  worktree: Worktree
  projectName: string
  isOnCanvas: boolean
  isFocused: boolean
  isConnecting: boolean
  onAdd: () => void
  onFocus: () => void
  onDisconnect: () => void
  onStop: () => void
  onReset: () => void
  onRemove: () => void
  onReveal?: () => void
}

/**
 * A single worktree entry with status dot, branch, and connect icon.
 *
 * Left-click connects or focuses the worktree panel.
 * Right-click opens a context menu with Disconnect and Remove actions.
 */
function WorktreeRow({
  worktree,
  projectName,
  isOnCanvas,
  isFocused,
  isConnecting,
  onAdd,
  onFocus,
  onDisconnect,
  onStop,
  onReset,
  onRemove,
  onReveal,
}: WorktreeRowProps) {
  const [showRemoveDialog, setShowRemoveDialog] = useState(false)
  const [showResetDialog, setShowResetDialog] = useState(false)

  const stateInfo = worktreeStateIndicator[worktree.state]
  const attentionDot = worktree.needsInput ? getAttentionConfig(worktree.notificationType) : null

  const textClass = attentionDot?.textClass ?? stateInfo.textClass
  const statusLabel = attentionDot?.label ?? stateInfo.label
  const label = worktreeLabel(worktree, projectName)
  const isActive = hasActiveTerminal(worktree)
  const isMain = worktree.id === 'main'
  const canDisconnect = isActive
  const canStop = isActive
  const canRemove = !isMain && worktree.state !== 'connected'

  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger asChild>
          <div
            data-testid={`worktree-row-${worktree.id}`}
            role="button"
            tabIndex={isConnecting ? -1 : 0}
            className={cn(
              'hover:bg-muted flex w-full min-w-0 items-center gap-2 rounded px-2 py-1.5 text-left',
              !isConnecting && 'cursor-pointer',
              isConnecting && 'animate-pulse',
              isFocused && 'bg-selected',
            )}
            aria-disabled={isConnecting}
            onClick={isConnecting ? undefined : isOnCanvas ? onFocus : onAdd}
            onKeyDown={(e) => {
              if ((e.key === 'Enter' || e.key === ' ') && !isConnecting) {
                if (isOnCanvas) onFocus()
                else onAdd()
              }
            }}
          >
            <div className="min-w-0 flex-1">
              <p className="truncate font-medium">{label}</p>
              {worktree.branch && (
                <p className="text-muted-foreground flex min-w-0 items-center gap-0.5">
                  <GitBranch className="h-3 w-3 shrink-0" />
                  <span className="truncate">{worktree.branch}</span>
                </p>
              )}
              <p className={cn('flex items-center gap-1 text-sm lowercase', textClass)}>
                <span
                  className={cn(
                    'inline-block h-1.5 w-1.5 shrink-0 rounded-full',
                    attentionDot?.dotClass ?? stateInfo.dotClass,
                  )}
                />
                {statusLabel}
              </p>
            </div>
            {isConnecting && (
              <span className="text-muted-foreground ml-auto text-sm">Connecting...</span>
            )}
          </div>
        </ContextMenuTrigger>
        <ContextMenuContent>
          {onReveal && (
            <>
              <ContextMenuItem onClick={onReveal}>
                <FolderOpen className="h-4 w-4" />
                Reveal in File Manager
              </ContextMenuItem>
              <ContextMenuSeparator />
            </>
          )}
          <ContextMenuItem disabled={!canDisconnect} onClick={onDisconnect}>
            <Unplug className="h-4 w-4" />
            <div>
              <div>Disconnect</div>
              <div className="text-muted-foreground text-xs font-normal">
                Close viewer, agent runs in background
              </div>
            </div>
          </ContextMenuItem>
          <ContextMenuItem disabled={!canStop} onClick={onStop}>
            <Square className="h-4 w-4" />
            <div>
              <div>Stop</div>
              <div className="text-muted-foreground text-xs font-normal">
                Stop agent from running in the background
              </div>
            </div>
          </ContextMenuItem>
          {isMain ? (
            <ContextMenuItem
              onClick={() => setShowResetDialog(true)}
              className="text-error focus:text-error"
            >
              <RotateCcw className="h-4 w-4" />
              <div>
                <div>Reset</div>
                <div className="text-muted-foreground text-xs font-normal">
                  Clear all history and start fresh
                </div>
              </div>
            </ContextMenuItem>
          ) : (
            <ContextMenuItem
              disabled={!canRemove}
              onClick={() => setShowRemoveDialog(true)}
              className="text-error focus:text-error"
            >
              <Trash2 className="h-4 w-4" />
              <div>
                <div>Delete</div>
                <div className="text-muted-foreground text-xs font-normal">
                  Remove this worktree and all its history
                </div>
              </div>
            </ContextMenuItem>
          )}
        </ContextMenuContent>
      </ContextMenu>

      <RemoveWorktreeDialog
        open={showResetDialog}
        label={label}
        variant="reset"
        onOpenChange={setShowResetDialog}
        onConfirm={onReset}
      />
      <RemoveWorktreeDialog
        open={showRemoveDialog}
        label={label}
        onOpenChange={setShowRemoveDialog}
        onConfirm={onRemove}
      />
    </>
  )
}

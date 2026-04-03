import { useCallback, useEffect, useMemo, useState } from 'react'
import { LayoutGrid, Scan } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { toast } from 'sonner'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { connectTerminal, disconnectTerminal, killWorktreeProcess, removeWorktree } from '@/lib/api'
import { formatCost } from '@/lib/cost'
import { buildPanelId } from '@/lib/canvas-store'
import { useProjects } from '@/hooks/use-projects'
import { useRevealInFileManager } from '@/hooks/use-reveal-in-file-manager'
import { useWorktrees } from '@/hooks/use-worktrees'
import { workspaceMount, type Worktree, type WorkspaceMount } from '@/lib/types'
import WorktreeList from '@/components/project/worktree-list'

/** Projects change infrequently — poll less aggressively than worktree state. */
const PROJECT_POLL_INTERVAL_MS = 10_000

/** Callback signature for adding a worktree panel to the canvas. */
interface OnAddPanel {
  (params: {
    projectId: string
    agentType: string
    projectName: string
    worktreeId: string
    branch?: string
  }): void
}

/** Group ordering for the worktree list — panels on canvas first. */
const CANVAS_GROUP_ORDER = ['Displayed', 'Available']

/** Callback to sync canvas panels with live worktree state. */
interface OnSyncWorktrees {
  (projectId: string, worktrees: Worktree[]): void
}

/** Set of panel IDs already on the canvas, for disabling duplicate adds. */
type ActivePanelIds = Set<string>

/** The two display modes for the workspace area. */
export type ViewMode = 'grid' | 'canvas'

/** Props for the project sidebar. */
interface ProjectSidebarProps {
  /** Currently selected project ID, driven by the URL. */
  selectedProjectId: string
  /** CLI agent type for the selected project. */
  selectedAgentType: string
  /** Called when the user picks a different project in the dropdown. */
  onProjectChange: (projectId: string, agentType: string) => void
  /** Current display mode for the workspace area. */
  viewMode: ViewMode
  /** Called when the user switches between grid and canvas modes. */
  onViewModeChange: (mode: ViewMode) => void
  onAddPanel: OnAddPanel
  onFocusPanel: (panelId: string) => void
  /** Removes a panel from the canvas by its panel ID. */
  onRemovePanel: (panelId: string) => void
  onSyncWorktrees: OnSyncWorktrees
  activePanelIds: ActivePanelIds
  /** ID of the currently focused panel on the canvas. */
  focusedPanelId: string | null
}

/**
 * Sidebar with a project dropdown selector and worktree list beneath.
 *
 * The selected project is controlled by the parent via URL params.
 * Clicking a worktree adds its terminal as a panel on the canvas,
 * auto-connecting if needed.
 */
export default function ProjectSidebar({
  selectedProjectId,
  selectedAgentType,
  onProjectChange,
  viewMode,
  onViewModeChange,
  onAddPanel,
  onFocusPanel,
  onRemovePanel,
  onSyncWorktrees,
  activePanelIds,
  focusedPanelId,
}: ProjectSidebarProps) {
  const { projects, isLoading } = useProjects(PROJECT_POLL_INTERVAL_MS)
  const runningProjects = useMemo(() => projects.filter((p) => p.state === 'running'), [projects])

  // Fall back to the first running project when the current selection is invalid.
  useEffect(() => {
    if (runningProjects.length === 0) return
    const isValid = runningProjects.some(
      (p) => p.projectId === selectedProjectId && p.agentType === selectedAgentType,
    )
    if (!isValid) {
      onProjectChange(runningProjects[0].projectId, runningProjects[0].agentType)
    }
  }, [runningProjects, selectedProjectId, selectedAgentType, onProjectChange])

  const selectedProject = runningProjects.find(
    (p) => p.projectId === selectedProjectId && p.agentType === selectedAgentType,
  )

  return (
    <div
      data-testid="project-sidebar"
      className="border-border bg-muted/30 flex h-full w-72 shrink-0 flex-col gap-2 overflow-hidden border-r"
    >
      {/* View mode toggle */}
      <div className="px-3 pt-3">
        <Tabs value={viewMode} onValueChange={(v) => onViewModeChange(v as ViewMode)}>
          <TabsList className="w-full">
            <TabsTrigger value="grid">
              <LayoutGrid className="h-3.5 w-3.5" />
              Grid
            </TabsTrigger>
            <TabsTrigger value="canvas" className="relative">
              <Scan className="h-3.5 w-3.5" />
              Canvas
              <Badge
                variant="secondary"
                className="absolute -top-2 -right-2 rounded px-1 py-0 text-xs"
              >
                beta
              </Badge>
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      <div className="px-3">
        {isLoading ? (
          <p className="text-muted-foreground py-1 text-center text-sm">Loading...</p>
        ) : runningProjects.length === 0 ? (
          <p className="text-muted-foreground py-1 text-center text-sm">No running projects</p>
        ) : (
          <div className="space-y-1">
            <label className="text-muted-foreground text-sm font-medium">Project</label>
            <Select
              value={`${selectedProjectId}:${selectedAgentType}`}
              onValueChange={(compound) => {
                const [id, ...rest] = compound.split(':')
                const agentType = rest.join(':')
                onProjectChange(id, agentType)
              }}
            >
              <SelectTrigger data-testid="project-select" className="h-8 text-sm">
                <SelectValue placeholder="Select a project" />
              </SelectTrigger>
              <SelectContent>
                {runningProjects.map((project) => (
                  <SelectItem
                    key={`${project.projectId}:${project.agentType}`}
                    value={`${project.projectId}:${project.agentType}`}
                  >
                    {project.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
      </div>

      <Separator className="my-2" />

      <div className="min-h-0 min-w-0 flex-1 overflow-y-auto">
        {selectedProject && (
          <ProjectWorktreeList
            key={`${selectedProject.projectId}:${selectedProject.agentType}`}
            projectId={selectedProject.projectId}
            agentType={selectedProject.agentType}
            projectName={selectedProject.name}
            isGitRepo={selectedProject.isGitRepo}
            mount={workspaceMount(selectedProject)}
            totalCost={selectedProject.totalCost}
            costBudget={selectedProject.costBudget}
            onAddPanel={onAddPanel}
            onFocusPanel={onFocusPanel}
            onRemovePanel={onRemovePanel}
            onSyncWorktrees={onSyncWorktrees}
            activePanelIds={activePanelIds}
            focusedPanelId={focusedPanelId}
          />
        )}
      </div>
    </div>
  )
}

/** Props for the project-specific worktree list. */
interface ProjectWorktreeListProps {
  projectId: string
  agentType: string
  projectName: string
  isGitRepo: boolean
  /** Host↔container path mapping for the workspace mount. */
  mount?: WorkspaceMount
  /** Aggregate cost across all worktrees in USD. */
  totalCost: number
  /** Effective cost budget in USD (0 = unlimited). */
  costBudget: number
  onAddPanel: OnAddPanel
  onFocusPanel: (panelId: string) => void
  onRemovePanel: (panelId: string) => void
  onSyncWorktrees: OnSyncWorktrees
  activePanelIds: ActivePanelIds
  focusedPanelId: string | null
}

/** Lists worktrees for a project, with an add button for each. */
function ProjectWorktreeList({
  projectId,
  agentType,
  projectName,
  isGitRepo,
  mount,
  totalCost,
  costBudget,
  onAddPanel,
  onFocusPanel,
  onRemovePanel,
  onSyncWorktrees,
  activePanelIds,
  focusedPanelId,
}: ProjectWorktreeListProps) {
  const { worktrees, isLoading, refetch } = useWorktrees(projectId, agentType)
  const [connectingId, setConnectingId] = useState<string | null>(null)

  const isOverBudget = costBudget > 0 && totalCost > costBudget
  const [budgetPendingAction, setBudgetPendingAction] = useState<(() => void) | null>(null)

  /**
   * Gates an action behind a budget confirmation dialog when the project
   * has exceeded its cost budget. Executes immediately otherwise.
   */
  const gateWithBudget = useCallback(
    (action: () => void) => {
      if (isOverBudget) {
        setBudgetPendingAction(() => action)
      } else {
        action()
      }
    },
    [isOverBudget],
  )

  /** Confirms the pending budget-gated action. */
  const handleBudgetConfirm = useCallback(() => {
    budgetPendingAction?.()
    setBudgetPendingAction(null)
  }, [budgetPendingAction])

  /** Cancels the pending budget-gated action. */
  const handleBudgetCancel = useCallback(() => {
    setBudgetPendingAction(null)
  }, [])

  // Control the new worktree dialog from here so we can gate it with budget check.
  const [isNewDialogOpen, setIsNewDialogOpen] = useState(false)

  /** Opens the new worktree dialog, gated by budget check. */
  const handleRequestNewWorktree = useCallback(() => {
    gateWithBudget(() => setIsNewDialogOpen(true))
  }, [gateWithBudget])

  // Sync canvas panels whenever worktree state changes.
  useEffect(() => {
    if (worktrees.length > 0) {
      onSyncWorktrees(projectId, worktrees)
    }
  }, [worktrees, projectId, onSyncWorktrees])

  /**
   * Adds a worktree panel to the canvas, auto-connecting if the
   * worktree is stopped or in background state.
   */
  const handleAddPanel = useCallback(
    async (worktree: Worktree) => {
      const shouldConnect = worktree.state === 'stopped' || worktree.state === 'background'

      if (shouldConnect) {
        setConnectingId(worktree.id)
        try {
          await connectTerminal(projectId, agentType, worktree.id)
          onAddPanel({
            projectId,
            agentType,
            projectName,
            worktreeId: worktree.id,
            branch: worktree.branch,
          })
          refetch()
        } catch (err) {
          const message = err instanceof Error ? err.message : 'Unknown error'
          toast.error('Failed to connect terminal', { description: message })
        } finally {
          setConnectingId(null)
        }
      } else {
        onAddPanel({
          projectId,
          agentType,
          projectName,
          worktreeId: worktree.id,
          branch: worktree.branch,
        })
      }
    },
    [projectId, agentType, projectName, onAddPanel, refetch],
  )

  /**
   * Reconnects a background worktree that already has a panel on the canvas.
   *
   * Calls the connect API to push a terminal_connected event (transitioning
   * the backend state from background → connected), then focuses the panel.
   */
  const handleReconnectAndFocus = useCallback(
    async (worktree: Worktree) => {
      const panelId = buildPanelId(projectId, worktree.id)
      setConnectingId(worktree.id)
      try {
        await connectTerminal(projectId, agentType, worktree.id)
        onFocusPanel(panelId)
        refetch()
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error'
        toast.error('Failed to reconnect terminal', { description: message })
      } finally {
        setConnectingId(null)
      }
    },
    [projectId, agentType, onFocusPanel, refetch],
  )

  /** Disconnects the terminal viewer for a worktree. */
  const handleDisconnect = useCallback(
    async (worktreeId: string) => {
      try {
        await disconnectTerminal(projectId, agentType, worktreeId)
        const panelId = buildPanelId(projectId, worktreeId)
        onRemovePanel(panelId)
        refetch()
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error'
        toast.error('Failed to disconnect terminal', { description: message })
      }
    },
    [projectId, agentType, onRemovePanel, refetch],
  )

  /** Stops the agent process for a worktree (kills tmux session + children). */
  const handleStop = useCallback(
    async (worktreeId: string) => {
      try {
        await killWorktreeProcess(projectId, agentType, worktreeId)
        const panelId = buildPanelId(projectId, worktreeId)
        onRemovePanel(panelId)
        refetch()
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error'
        toast.error('Failed to stop worktree', { description: message })
      }
    },
    [projectId, agentType, onRemovePanel, refetch],
  )

  /** Removes a worktree entirely (git worktree remove + cleanup). */
  const handleRemove = useCallback(
    async (worktreeId: string) => {
      try {
        await removeWorktree(projectId, agentType, worktreeId)
        const panelId = buildPanelId(projectId, worktreeId)
        onRemovePanel(panelId)
        refetch()
        toast.success('Worktree removed')
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error'
        toast.error('Failed to remove worktree', { description: message })
        refetch()
      }
    },
    [projectId, agentType, onRemovePanel, refetch],
  )

  const handleReveal = useRevealInFileManager(mount)

  /** Called when a worktree is created from the dialog. */
  const handleWorktreeCreated = useCallback(
    (worktreeId: string) => {
      refetch()
      toast.success('Worktree created', {
        description: `Created ${worktreeId}`,
      })
    },
    [refetch],
  )

  /** Classifies a worktree as "On Canvas" or "Available" for grouped rendering. */
  const groupByCanvas = useCallback(
    (wt: Worktree) => {
      const panelId = buildPanelId(projectId, wt.id)
      return activePanelIds.has(panelId) ? 'Displayed' : 'Available'
    },
    [projectId, activePanelIds],
  )

  const groupOrder = CANVAS_GROUP_ORDER

  /** Handles focus or reconnect depending on worktree state. */
  const handleFocusOrReconnect = useCallback(
    (wt: Worktree) => {
      if (wt.state === 'background') {
        handleReconnectAndFocus(wt)
      } else {
        const panelId = buildPanelId(projectId, wt.id)
        onFocusPanel(panelId)
      }
    },
    [projectId, onFocusPanel, handleReconnectAndFocus],
  )

  return (
    <>
      <WorktreeList
        projectId={projectId}
        agentType={agentType}
        projectName={projectName}
        isGitRepo={isGitRepo}
        worktrees={worktrees}
        isLoading={isLoading}
        onWorktreeCreated={handleWorktreeCreated}
        onRefreshComplete={refetch}
        groupBy={groupByCanvas}
        groupOrder={groupOrder}
        activePanelIds={activePanelIds}
        focusedPanelId={focusedPanelId}
        connectingId={connectingId}
        onAdd={handleAddPanel}
        onFocus={handleFocusOrReconnect}
        onDisconnect={handleDisconnect}
        onStop={handleStop}
        onRemove={handleRemove}
        onReveal={handleReveal ?? undefined}
        newDialogOpen={isNewDialogOpen}
        onNewDialogOpenChange={setIsNewDialogOpen}
        onRequestNewWorktree={handleRequestNewWorktree}
      />

      <AlertDialog
        open={budgetPendingAction !== null}
        onOpenChange={(open) => {
          if (!open) handleBudgetCancel()
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Budget exceeded</AlertDialogTitle>
            <AlertDialogDescription>
              This project has spent {formatCost(totalCost)} of its {formatCost(costBudget)} budget.
              Creating a new worktree will start a new Claude session and incur additional costs.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={handleBudgetCancel}>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleBudgetConfirm}>Continue anyway</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

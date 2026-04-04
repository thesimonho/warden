import { useNavigate } from 'react-router-dom'
import {
  Square,
  Loader2,
  FolderOpen,
  Terminal,
  FolderCog,
  Pencil,
  Layers,
  ShieldCheck,
  ShieldOff,
  RotateCcw,
  Play,
  Trash2,
} from 'lucide-react'
import type { AgentType, Project } from '@/lib/types'
import type { InstallStatus } from '@/hooks/use-projects'
import { formatCost } from '@/lib/cost'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { abbreviateHomePath, cn } from '@/lib/utils'
import StatusBadge from '@/components/home/status-badge'
import AgentStatusIndicator from '@/components/home/agent-status-indicator'
import { AgentIcon } from '@/components/ui/agent-icons'

/** Props for the ProjectCard component. */
interface ProjectCardProps {
  project: Project
  onStop: (id: string, agentType: AgentType) => void
  onRestart: (id: string, agentType: AgentType) => void
  onRemove: (project: Project) => void
  onEdit: (project: Project) => void
  /** When true, clicking the card toggles selection instead of navigating. */
  isSelected?: boolean
  onToggleSelect?: (id: string, agentType: AgentType) => void
  /** Whether a stop action is in flight for this project. */
  isStopPending?: boolean
  /** Whether a restart action is in flight for this project. */
  isRestartPending?: boolean
  /** Whether the "prevent restart" budget enforcement action is enabled. */
  budgetActionPreventStart?: boolean
  /** Agent CLI or runtime installation status for this project. */
  installStatus?: InstallStatus
}

/**
 * Displays a project container's info with status and action buttons.
 *
 * In selectable mode, clicking the card toggles selection and the View button
 * is replaced by a visual selection indicator.
 *
 * @param props.project - The project to display.
 * @param props.onStop - Callback when the stop button is clicked.
 * @param props.onRestart - Callback when the restart button is clicked.
 * @param props.isSelected - Whether the card is currently selected.
 * @param props.onToggleSelect - Callback when the card selection is toggled.
 */

export default function ProjectCard({
  project,
  onStop,
  onRestart,
  onRemove,
  onEdit,
  isSelected = false,
  onToggleSelect,
  isStopPending = false,
  isRestartPending = false,
  budgetActionPreventStart = false,
  installStatus,
}: ProjectCardProps) {
  const navigate = useNavigate()
  const isRunning = project.state === 'running'
  const isNotFound = !project.hasContainer
  const isOverBudget =
    budgetActionPreventStart && project.costBudget > 0 && project.totalCost > project.costBudget

  const handleCardClick = (e: React.MouseEvent) => {
    // Don't toggle selection when clicking interactive elements inside the card.
    if ((e.target as HTMLElement).closest('button, a')) return
    onToggleSelect?.(project.projectId, project.agentType)
  }

  if (isNotFound) {
    return (
      <Card data-testid={`project-card-${project.name}`} className="opacity-75">
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="flex items-center gap-2 text-xl font-semibold">
            <AgentIcon type={project.agentType} className="h-4 w-4 shrink-0" />
            {project.name}
          </CardTitle>
          <StatusBadge state="no container" />
        </CardHeader>
        <CardContent className="space-y-1.5">
          {project.hostPath && (
            <span className="text-muted-foreground flex items-center gap-1 text-sm">
              <FolderOpen className="h-3 w-3 shrink-0" />
              {abbreviateHomePath(project.hostPath)}
            </span>
          )}
          {project.totalCost > 0.001 && <CostBadge project={project} />}
        </CardContent>
        <CardFooter className="gap-2">
          <Button size="sm" variant="default" onClick={() => onEdit(project)} icon={Play}>
            Create Container
          </Button>
          <div className="ml-auto flex items-center gap-1">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  size="sm"
                  variant="ghost"
                  color="error"
                  onClick={() => onRemove(project)}
                  icon={FolderCog}
                />
              </TooltipTrigger>
              <TooltipContent>Manage</TooltipContent>
            </Tooltip>
          </div>
        </CardFooter>
      </Card>
    )
  }

  return (
    <Card
      data-testid={`project-card-${project.name}`}
      className={cn('cursor-pointer rounded transition-all', isSelected && 'ring-primary ring-2')}
      onClick={handleCardClick}
    >
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="flex items-center gap-4 text-xl font-semibold">
          {project.name || project.projectId}
          <div className="flex items-center gap-2">
            <AgentIcon type={project.agentType} className="h-4 w-4 shrink-0" />
            {project.agentVersion && (
              <span className="text-muted-foreground text-xs font-light">
                ({project.agentVersion})
              </span>
            )}
          </div>
        </CardTitle>
        <div className="flex items-center gap-2">
          {installStatus ? (
            <span className="text-muted-foreground flex items-center gap-1.5 text-sm">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              {installStatus.message}
            </span>
          ) : isRunning ? (
            <div className="flex gap-0">
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    data-testid="restart-button"
                    size="sm"
                    variant="ghost"
                    color="muted"
                    onClick={() => onRestart(project.projectId, project.agentType)}
                    disabled={isStopPending || isRestartPending || isOverBudget}
                    icon={isRestartPending ? Loader2 : RotateCcw}
                    loading={isRestartPending}
                  />
                </TooltipTrigger>
                <TooltipContent>
                  {isOverBudget ? 'Budget exceeded — increase budget to restart' : 'Restart'}
                </TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    data-testid="stop-button"
                    size="sm"
                    variant="ghost"
                    color="error"
                    onClick={() => onStop(project.projectId, project.agentType)}
                    disabled={isStopPending || isRestartPending}
                    icon={isStopPending ? Loader2 : Square}
                    loading={isStopPending}
                  />
                </TooltipTrigger>
                <TooltipContent>Stop</TooltipContent>
              </Tooltip>
            </div>
          ) : (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  data-testid="start-button"
                  size="sm"
                  variant="ghost"
                  color="success"
                  onClick={() => onRestart(project.projectId, project.agentType)}
                  disabled={isStopPending || isRestartPending || isOverBudget}
                  icon={isRestartPending ? Loader2 : Play}
                  loading={isRestartPending}
                />
              </TooltipTrigger>
              <TooltipContent>
                {isOverBudget ? 'Budget exceeded — increase budget to start' : 'Start'}
              </TooltipContent>
            </Tooltip>
          )}

          {!installStatus && <StatusBadge data-testid="status-badge" state={project.state} />}
        </div>
      </CardHeader>

      <CardContent className="flex-1 space-y-1.5">
        {project.type && <p className="text-muted-foreground">{project.type}</p>}
        <div className="flex items-center justify-between">
          <p className="text-muted-foreground text-sm">{project.status}</p>
          {(project.totalCost > 0.001 || project.costBudget > 0) && <CostBadge project={project} />}
        </div>
        <div className="flex flex-wrap justify-between gap-x-3 gap-y-1 pt-1">
          {project.mountedDir && (
            <span className="text-muted-foreground flex items-center gap-1 text-sm">
              <FolderOpen className="h-3 w-3 shrink-0" />
              {abbreviateHomePath(project.mountedDir)}
            </span>
          )}
          {project.sshPort && (
            <span className="text-muted-foreground flex items-center gap-1 text-sm">
              <Terminal className="h-3 w-3 shrink-0" />:{project.sshPort}
            </span>
          )}
          {project.networkMode === 'restricted' && (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="text-warning flex items-center gap-1 text-sm">
                  <ShieldCheck className="h-3 w-3 shrink-0" />
                  Restricted
                </span>
              </TooltipTrigger>
              <TooltipContent>Network Mode</TooltipContent>
            </Tooltip>
          )}
          {project.networkMode === 'none' && (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="text-error flex items-center gap-1 text-sm">
                  <ShieldOff className="h-3 w-3 shrink-0" />
                  No Network
                </span>
              </TooltipTrigger>
              <TooltipContent>Network Mode</TooltipContent>
            </Tooltip>
          )}
        </div>
        <div className="flex flex-wrap justify-between">
          {isRunning && project.activeWorktreeCount > 0 && (
            <div className="flex flex-wrap gap-x-3 gap-y-1 pt-1">
              <span className="text-muted-foreground flex items-center gap-1 text-sm">
                <Layers className="h-3 w-3 shrink-0" />
                {project.activeWorktreeCount} active worktree
                {project.activeWorktreeCount !== 1 ? 's' : ''}
              </span>
            </div>
          )}
          {isRunning && (
            <AgentStatusIndicator
              status={project.agentStatus}
              needsInput={project.needsInput}
              notificationType={project.notificationType}
            />
          )}
        </div>
      </CardContent>

      <CardFooter className="gap-2">
        <Button
          data-testid="view-button"
          size="sm"
          variant="default"
          onClick={() => navigate(`/projects/${project.projectId}/${project.agentType}`)}
          disabled={!isRunning || isOverBudget}
        >
          View
        </Button>

        <div className="ml-auto flex items-center gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                data-testid="edit-button"
                size="sm"
                variant="ghost"
                color="warning"
                onClick={() => onEdit(project)}
                icon={Pencil}
              />
            </TooltipTrigger>
            <TooltipContent>Edit</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                data-testid="delete-button"
                size="sm"
                variant="ghost"
                color="error"
                onClick={() => onRemove(project)}
                icon={Trash2}
              />
            </TooltipTrigger>
            <TooltipContent>Delete</TooltipContent>
          </Tooltip>
        </div>
      </CardFooter>
    </Card>
  )
}

/** Fraction of budget at which the warning color activates. */
const BUDGET_WARNING_THRESHOLD = 0.8

/** Displays cost with optional budget progress and color coding. */
function CostBadge({ project }: { project: Project }) {
  const hasBudget = project.costBudget > 0
  const isOverBudget = hasBudget && project.totalCost > project.costBudget
  const isNearBudget =
    hasBudget && project.totalCost > project.costBudget * BUDGET_WARNING_THRESHOLD

  const colorClass = isOverBudget
    ? 'text-error'
    : isNearBudget
      ? 'text-warning'
      : 'text-muted-foreground'

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className={`flex cursor-default items-center gap-1 font-mono text-sm ${colorClass}`}>
          {formatCost(project.totalCost)}
          {hasBudget && (
            <span className="text-muted-foreground">/ {formatCost(project.costBudget)}</span>
          )}
        </span>
      </TooltipTrigger>
      <TooltipContent>
        {project.isEstimatedCost ? 'Estimated' : 'Actual'} project cost
      </TooltipContent>
    </Tooltip>
  )
}

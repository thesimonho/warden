import type { AgentType, Project } from '@/lib/types'
import ProjectCard from '@/components/home/project-card'
import { Skeleton } from '@/components/ui/skeleton'

/** Props for the ProjectGrid component. */
interface ProjectGridProps {
  projects: Project[]
  isLoading: boolean
  onStop: (id: string, agentType: AgentType) => void
  onRestart: (id: string, agentType: AgentType) => void
  onRemove: (project: Project) => void
  onEdit: (project: Project) => void
  isSelectable?: boolean
  selectedIds?: Set<string>
  onToggleSelect?: (id: string, agentType: AgentType) => void
  /** Compound keys (projectId:agentType) of projects with a stop action in flight. */
  pendingStopIds?: Set<string>
  /** Compound keys (projectId:agentType) of projects with a restart action in flight. */
  pendingRestartIds?: Set<string>
  /** Whether the "prevent restart" budget enforcement action is enabled. */
  budgetActionPreventStart?: boolean
}

/**
 * Displays a responsive grid of project cards with loading and empty states.
 *
 * @param props.projects - The projects to display.
 * @param props.isLoading - Whether the initial load is in progress.
 * @param props.onStop - Callback when a project's stop button is clicked.
 * @param props.onRestart - Callback when a project's restart button is clicked.
 * @param props.isSelectable - Whether cards are in selection mode.
 * @param props.selectedIds - Set of currently selected project IDs.
 * @param props.onToggleSelect - Callback when a project's selection is toggled.
 * @param props.pendingStopIds - IDs of projects with a stop action in flight.
 * @param props.pendingRestartIds - IDs of projects with a restart action in flight.
 */
export default function ProjectGrid({
  projects,
  isLoading,
  onStop,
  onRestart,
  onRemove,
  onEdit,
  isSelectable = false,
  selectedIds = new Set(),
  onToggleSelect,
  pendingStopIds = new Set(),
  pendingRestartIds = new Set(),
  budgetActionPreventStart = false,
}: ProjectGridProps) {
  if (isLoading) {
    return (
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-48 rounded" />
        ))}
      </div>
    )
  }

  if (projects.length === 0) {
    return (
      <div className="flex h-48 items-center justify-center rounded border border-dashed">
        <p className="text-muted-foreground">
          No projects yet. Click <strong>Add Project</strong> to get started.
        </p>
      </div>
    )
  }

  return (
    <div
      data-testid="project-grid"
      className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"
    >
      {projects.map((project) => {
        const key = `${project.projectId}:${project.agentType}`
        return (
          <ProjectCard
            key={key}
            project={project}
            onStop={onStop}
            onRestart={onRestart}
            onRemove={onRemove}
            onEdit={onEdit}
            isSelectable={isSelectable}
            isSelected={selectedIds.has(key)}
            onToggleSelect={onToggleSelect}
            isStopPending={pendingStopIds.has(key)}
            isRestartPending={pendingRestartIds.has(key)}
            budgetActionPreventStart={budgetActionPreventStart}
          />
        )
      })}
    </div>
  )
}

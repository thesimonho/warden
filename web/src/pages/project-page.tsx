import { useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import ProjectView from '@/components/project/project-view'
import { useEventSource } from '@/hooks/use-event-source'
import type { BudgetContainerStoppedEvent } from '@/lib/types'

/**
 * Route wrapper for the project view at `/projects/:id/:agentType`.
 *
 * Reads the project ID and agent type from URL params, handles navigation
 * on project change, and renders the core ProjectView in a fixed viewport
 * layout. Automatically redirects to home when budget enforcement stops
 * the container the user is currently viewing.
 */
export default function ProjectPage() {
  const { id: projectId, agentType } = useParams<{ id: string; agentType: string }>()
  const navigate = useNavigate()

  const handleProjectChange = useCallback(
    (newProjectId: string, newAgentType: string) => {
      navigate(`/projects/${newProjectId}/${newAgentType}`, { replace: true })
    },
    [navigate],
  )

  /** Redirect to home when budget enforcement stops the current project's container. */
  const handleBudgetContainerStopped = useCallback(
    (event: BudgetContainerStoppedEvent) => {
      if (event.projectId !== projectId) return

      toast.error(
        `Container stopped — budget exceeded ($${event.totalCost.toFixed(2)} / $${event.budget.toFixed(2)})`,
      )
      navigate('/', { replace: true })
    },
    [projectId, navigate],
  )

  useEventSource({ onBudgetContainerStopped: handleBudgetContainerStopped })

  return (
    <div className="fixed inset-0 top-14">
      <ProjectView
        projectId={projectId ?? ''}
        agentType={agentType ?? ''}
        onProjectChange={handleProjectChange}
      />
    </div>
  )
}

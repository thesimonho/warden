import { useCallback, useMemo } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { useProjects } from '@/hooks/use-projects'
import ProjectView from '@/components/project/project-view'

/** Parsed workspace panel entry — project ID paired with its agent type. */
interface WorkspaceEntry {
  projectId: string
  agentType: string
}

/** Encodes a workspace entry as "projectId:agentType" for the query string. */
function encodeEntry(entry: WorkspaceEntry): string {
  return `${entry.projectId}:${entry.agentType}`
}

/** Decodes a "projectId:agentType" token from the query string. */
function decodeEntry(token: string): WorkspaceEntry | null {
  const [projectId, agentType] = token.split(':')
  return projectId && agentType ? { projectId, agentType } : null
}

/**
 * Workspace page showing multiple project views in a grid.
 *
 * Each cell is a full ProjectView with its own sidebar, grid/canvas
 * toggle, and terminal panels. Project entries are read from the `ids`
 * query parameter as comma-separated "projectId:agentType" pairs.
 *
 * Example: /workspace?ids=abc123:claude-code,def456:codex
 */
export default function WorkspacePage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const { projects } = useProjects()

  const entries = useMemo(
    () =>
      (searchParams.get('ids')?.split(',').filter(Boolean) ?? [])
        .map(decodeEntry)
        .filter((e): e is WorkspaceEntry => e !== null),
    [searchParams],
  )

  const projectByKey = useMemo(
    () => new Map(projects.map((p) => [`${p.projectId}:${p.agentType}`, p])),
    [projects],
  )
  const selectedProjects = useMemo(
    () =>
      entries.flatMap((entry) => {
        const p = projectByKey.get(encodeEntry(entry))
        return p ? [p] : []
      }),
    [entries, projectByKey],
  )

  /** Updates a single entry in the URL while preserving the others. */
  const handleProjectChange = useCallback(
    (index: number, newProjectId: string, newAgentType: string) => {
      const updated = [...entries]
      updated[index] = { projectId: newProjectId, agentType: newAgentType }
      setSearchParams({ ids: updated.map(encodeEntry).join(',') }, { replace: true })
    },
    [entries, setSearchParams],
  )

  return (
    <div className="fixed inset-0 top-14 flex flex-col">
      <div className="flex shrink-0 items-center gap-3 border-b px-4 py-2">
        <Link
          to="/"
          className="text-muted-foreground hover:text-foreground inline-flex items-center gap-1"
        >
          <ArrowLeft className="h-4 w-4" />
          Back
        </Link>
        <h2>Workspace</h2>
        <span className="text-muted-foreground text-sm">
          {selectedProjects.length} project{selectedProjects.length !== 1 ? 's' : ''}
        </span>
      </div>

      {selectedProjects.length === 0 ? (
        <div className="flex flex-1 items-center justify-center">
          <p className="text-muted-foreground">No projects found for the selected IDs.</p>
        </div>
      ) : (
        <div className="grid min-h-0 flex-1 grid-cols-1 gap-1 lg:grid-cols-2">
          {selectedProjects.map((project, index) => (
            <div
              key={`${project.projectId}:${project.agentType}`}
              className="min-h-0 overflow-hidden border-b lg:border-r"
            >
              <ProjectView
                projectId={project.projectId}
                agentType={project.agentType}
                onProjectChange={(newId, newAgent) => handleProjectChange(index, newId, newAgent)}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

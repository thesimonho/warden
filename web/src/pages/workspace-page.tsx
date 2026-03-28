import { useCallback, useMemo } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { useProjects } from '@/hooks/use-projects'
import ProjectView from '@/components/project/project-view'

/**
 * Workspace page showing multiple project views in a grid.
 *
 * Each cell is a full ProjectView with its own sidebar, grid/canvas
 * toggle, and terminal panels. Project IDs are read from the `ids`
 * query parameter (comma-separated).
 *
 * Example: /workspace?ids=abc123,def456
 */
export default function WorkspacePage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const { projects } = useProjects()

  const ids = useMemo(
    () => searchParams.get('ids')?.split(',').filter(Boolean) ?? [],
    [searchParams],
  )

  const projectById = useMemo(() => new Map(projects.map((p) => [p.projectId, p])), [projects])
  const selectedProjects = useMemo(
    () =>
      ids.flatMap((id) => {
        const p = projectById.get(id)
        return p ? [p] : []
      }),
    [ids, projectById],
  )

  /** Updates a single project ID in the URL while preserving the others. */
  const handleProjectChange = useCallback(
    (index: number, newProjectId: string) => {
      const updated = [...ids]
      updated[index] = newProjectId
      setSearchParams({ ids: updated.join(',') }, { replace: true })
    },
    [ids, setSearchParams],
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
            <div key={project.projectId} className="min-h-0 overflow-hidden border-b lg:border-r">
              <ProjectView
                projectId={project.projectId}
                onProjectChange={(newId) => handleProjectChange(index, newId)}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

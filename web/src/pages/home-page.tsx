import { useCallback, useEffect, useState } from 'react'
import type React from 'react'
import { useNavigate, useOutletContext } from 'react-router-dom'
import { LayoutGrid, Square, RotateCw, X, Loader2, Plus, RefreshCw, Box } from 'lucide-react'
import { toast } from 'sonner'
import { useNotifications } from '@/hooks/use-notifications'
import { useProjects } from '@/hooks/use-projects'
import { useRecentWorkspaces } from '@/hooks/use-recent-workspaces'
import { ApiError, stopProject, restartProject, fetchProjects } from '@/lib/api'
import type { Project } from '@/lib/types'
import { Button } from '@/components/ui/button'
import CostDashboard from '@/components/home/cost-dashboard'
import ProjectGrid from '@/components/home/project-grid'
import RecentWorkspaces from '@/components/home/recent-workspaces'
import AddProjectDialog, { type CreateForProject } from '@/components/home/add-project-dialog'
import ManageProjectDialog from '@/components/home/manage-project-dialog'
import StaleMountsDialog from '@/components/home/stale-mounts-dialog'
import type { LayoutContext } from '@/components/layout'

/** Home page displaying all managed project containers in a grid. */
export default function HomePage() {
  const navigate = useNavigate()
  const { settings, budgetActionPreventStart } = useOutletContext<LayoutContext>()
  const { projects, isLoading, isRefreshing, error, refetch } = useProjects()
  const { recentWorkspaces, addWorkspace } = useRecentWorkspaces()
  useNotifications(projects, settings.notificationsEnabled)
  const [isSelectMode, setIsSelectMode] = useState(false)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [pendingStopIds, setPendingStopIds] = useState<Set<string>>(new Set())
  const [pendingRestartIds, setPendingRestartIds] = useState<Set<string>>(new Set())
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Project | null>(null)
  const [projectToManage, setProjectToManage] = useState<Project | null>(null)
  const [createForProject, setCreateForProject] = useState<CreateForProject | null>(null)
  const [staleMountsProject, setStaleMountsProject] = useState<{
    id: string
    name: string
  } | null>(null)

  const handleStop = useCallback(
    async (id: string) => {
      setPendingStopIds((prev) => new Set([...prev, id]))
      try {
        await stopProject(id)
        toast.success('Project stopped')
        refetch()
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Failed to stop project')
      } finally {
        setPendingStopIds((prev) => {
          const next = new Set(prev)
          next.delete(id)
          return next
        })
      }
    },
    [refetch],
  )

  const handleRestart = useCallback(
    async (id: string) => {
      setPendingRestartIds((prev) => new Set([...prev, id]))
      try {
        await restartProject(id)
        // Wait briefly for the container to either stabilize or crash
        await new Promise((r) => setTimeout(r, 2000))
        const updated = await fetchProjects()
        const project = updated.find((p) => p.projectId === id)
        if (project?.state === 'running') {
          toast.success('Project started')
        } else {
          toast.error(`Project failed to start (${project?.status ?? 'unknown'})`)
        }
        refetch()
      } catch (err) {
        if (err instanceof ApiError && err.code === 'STALE_MOUNTS') {
          const match = projects.find((p) => p.projectId === id)
          setStaleMountsProject({ id, name: match?.name ?? id })
        } else {
          toast.error(err instanceof Error ? err.message : 'Failed to restart project')
        }
      } finally {
        setPendingRestartIds((prev) => {
          const next = new Set(prev)
          next.delete(id)
          return next
        })
      }
    },
    [projects, refetch],
  )

  const handleRemove = useCallback((project: Project) => {
    setProjectToManage(project)
  }, [])

  const handleEdit = useCallback((project: Project) => {
    if (!project.hasContainer) {
      setCreateForProject({
        projectId: project.projectId,
        name: project.name,
        hostPath: project.hostPath,
      })
    } else {
      setEditTarget(project)
    }
  }, [])

  const handleToggleSelect = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }, [])

  const handleEnterSelectMode = () => {
    setIsSelectMode(true)
    setSelectedIds(new Set())
  }

  const handleCancelSelectMode = () => {
    setIsSelectMode(false)
    setSelectedIds(new Set())
  }

  const handleOpenWorkspace = useCallback(
    (ids: string[]) => {
      addWorkspace(ids)
      navigate(`/workspace?ids=${ids.join(',')}`)
    },
    [addWorkspace, navigate],
  )

  const handleOpenSelected = () => {
    if (selectedIds.size === 0) return
    handleOpenWorkspace([...selectedIds])
  }

  const handleBulkAction = useCallback(
    async (
      action: (id: string) => Promise<unknown>,
      setPending: React.Dispatch<React.SetStateAction<Set<string>>>,
      label: string,
    ) => {
      const ids = [...selectedIds]
      setPending((prev) => new Set([...prev, ...ids]))
      try {
        const results = await Promise.allSettled(ids.map(action))
        const failed = results.filter((r) => r.status === 'rejected')
        if (failed.length > 0) {
          toast.error(
            `Failed to ${label} ${failed.length} project${failed.length !== 1 ? 's' : ''}`,
          )
        } else {
          toast.success(
            `${ids.length} project${ids.length !== 1 ? 's' : ''} ${label}${label.endsWith('e') ? 'd' : 'ed'}`,
          )
        }
        refetch()
      } finally {
        setPending((prev) => {
          const next = new Set(prev)
          ids.forEach((id) => next.delete(id))
          return next
        })
      }
    },
    [selectedIds, refetch],
  )

  const handleStopSelected = useCallback(
    () => handleBulkAction(stopProject, setPendingStopIds, 'stop'),
    [handleBulkAction],
  )

  const handleRestartSelected = useCallback(
    () => handleBulkAction(restartProject, setPendingRestartIds, 'restart'),
    [handleBulkAction],
  )

  useEffect(() => {
    if (error) toast.error(error)
  }, [error])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Box className="text-muted-foreground h-5 w-5" />
          <h2>Projects</h2>
        </div>
        <div className="flex items-center gap-3">
          {projects.length > 0 && (
            <span className="text-muted-foreground flex items-center gap-1.5">
              {isRefreshing && <RefreshCw className="h-3 w-3 animate-spin" />}
              {projects.length} container{projects.length !== 1 ? 's' : ''}
            </span>
          )}
          {!isSelectMode && projects.length > 0 && (
            <Button
              data-testid="select-mode-button"
              size="sm"
              variant="outline"
              onClick={handleEnterSelectMode}
              icon={LayoutGrid}
            >
              Select
            </Button>
          )}
          {!isSelectMode && (
            <Button
              data-testid="add-project-button"
              size="sm"
              onClick={() => setIsAddDialogOpen(true)}
              icon={Plus}
            >
              Add Project
            </Button>
          )}
        </div>
      </div>

      <CostDashboard projects={projects} />

      <RecentWorkspaces
        recentWorkspaces={recentWorkspaces}
        projects={projects}
        onOpen={handleOpenWorkspace}
      />

      <ProjectGrid
        projects={projects}
        isLoading={isLoading}
        onStop={handleStop}
        onRestart={handleRestart}
        onRemove={handleRemove}
        onEdit={handleEdit}
        isSelectable={isSelectMode}
        selectedIds={selectedIds}
        onToggleSelect={handleToggleSelect}
        pendingStopIds={pendingStopIds}
        pendingRestartIds={pendingRestartIds}
        budgetActionPreventStart={budgetActionPreventStart}
      />

      {isSelectMode && (
        <div className="bg-background fixed bottom-6 left-1/2 flex -translate-x-1/2 items-center gap-3 rounded border px-5 py-3 shadow-lg">
          <span className="text-muted-foreground">{selectedIds.size} selected</span>
          <Button
            size="sm"
            variant="success"
            onClick={handleOpenSelected}
            disabled={
              selectedIds.size === 0 || pendingStopIds.size > 0 || pendingRestartIds.size > 0
            }
            icon={LayoutGrid}
          >
            Open in Workspace
          </Button>
          <Button
            size="sm"
            variant="error"
            onClick={handleStopSelected}
            disabled={
              selectedIds.size === 0 || pendingStopIds.size > 0 || pendingRestartIds.size > 0
            }
            icon={pendingStopIds.size > 0 ? Loader2 : Square}
            loading={pendingStopIds.size > 0}
          >
            Stop
          </Button>
          <Button
            size="sm"
            variant="warning"
            onClick={handleRestartSelected}
            disabled={
              selectedIds.size === 0 || pendingStopIds.size > 0 || pendingRestartIds.size > 0
            }
            icon={pendingRestartIds.size > 0 ? Loader2 : RotateCw}
            loading={pendingRestartIds.size > 0}
          >
            Restart
          </Button>
          <Button size="sm" variant="ghost" onClick={handleCancelSelectMode} icon={X}>
            Cancel
          </Button>
        </div>
      )}

      <AddProjectDialog
        open={isAddDialogOpen}
        onOpenChange={setIsAddDialogOpen}
        onProjectAdded={refetch}
      />

      <AddProjectDialog
        open={editTarget !== null}
        onOpenChange={(open) => {
          if (!open) setEditTarget(null)
        }}
        onProjectAdded={refetch}
        editProjectId={editTarget?.projectId}
        editIsRunning={editTarget?.state === 'running'}
      />

      <AddProjectDialog
        open={createForProject !== null}
        onOpenChange={(open) => {
          if (!open) setCreateForProject(null)
        }}
        onProjectAdded={refetch}
        createForProject={createForProject}
      />

      <ManageProjectDialog
        open={projectToManage !== null}
        onOpenChange={(open) => {
          if (!open) setProjectToManage(null)
        }}
        project={projectToManage}
        onComplete={refetch}
      />

      <StaleMountsDialog
        open={staleMountsProject !== null}
        onOpenChange={(open) => {
          if (!open) setStaleMountsProject(null)
        }}
        project={staleMountsProject}
        onRecreated={() => {
          setStaleMountsProject(null)
          refetch()
        }}
      />
    </div>
  )
}

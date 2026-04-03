import { useCallback, useEffect, useState } from 'react'
import type React from 'react'
import { useNavigate, useOutletContext } from 'react-router-dom'
import { Square, Play, RotateCw, X, Loader2, Plus, RefreshCw, Box } from 'lucide-react'
import { toast } from 'sonner'
import { useNotifications } from '@/hooks/use-notifications'
import { useProjects } from '@/hooks/use-projects'
import {
  ApiError,
  stopProject,
  restartProject,
  fetchProjects,
  fetchSettings,
  fetchDefaults,
  addProject,
  createContainer,
} from '@/lib/api'
import { restrictedDomains } from '@/lib/domain-groups'
import type { AgentType, ServerSettings } from '@/lib/types'
import type { Project } from '@/lib/types'
import { Button } from '@/components/ui/button'
import CostDashboard from '@/components/home/cost-dashboard'
import ProjectGrid from '@/components/home/project-grid'
import AddProjectDialog, { type CreateForProject } from '@/components/home/add-project-dialog'
import ManageProjectDialog from '@/components/home/manage-project-dialog'
import StaleMountsDialog from '@/components/home/stale-mounts-dialog'
import type { LayoutContext } from '@/components/layout'

/** Home page displaying all managed project containers in a grid. */
export default function HomePage() {
  const navigate = useNavigate()
  const { settings, budgetActionPreventStart } = useOutletContext<LayoutContext>()
  const { projects, isLoading, isRefreshing, error, refetch } = useProjects()
  useNotifications(projects, settings.notificationsEnabled)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [pendingStopIds, setPendingStopIds] = useState<Set<string>>(new Set())
  const [pendingRestartIds, setPendingRestartIds] = useState<Set<string>>(new Set())
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Project | null>(null)
  const [projectToManage, setProjectToManage] = useState<Project | null>(null)
  const [createForProject, setCreateForProject] = useState<CreateForProject | null>(null)
  const [staleMountsProject, setStaleMountsProject] = useState<{
    id: string
    agentType: string
    name: string
  } | null>(null)
  const [serverSettings, setServerSettings] = useState<ServerSettings | null>(null)

  // Fetch server settings in dev mode for the quick-add buttons.
  useEffect(() => {
    if (import.meta.env.DEV) {
      fetchSettings()
        .then(setServerSettings)
        .catch(() => {})
    }
  }, [])

  /** Builds a compound key for uniquely identifying a project across agent types. */
  const compoundKey = (id: string, agentType: string) => `${id}:${agentType}`

  const handleStop = useCallback(
    async (id: string, agentType: AgentType) => {
      const key = compoundKey(id, agentType)
      setPendingStopIds((prev) => new Set([...prev, key]))
      try {
        await stopProject(id, agentType)
        toast.success('Project stopped')
        refetch()
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Failed to stop project')
      } finally {
        setPendingStopIds((prev) => {
          const next = new Set(prev)
          next.delete(key)
          return next
        })
      }
    },
    [refetch],
  )

  const handleRestart = useCallback(
    async (id: string, agentType: AgentType) => {
      const key = compoundKey(id, agentType)
      setPendingRestartIds((prev) => new Set([...prev, key]))
      try {
        await restartProject(id, agentType)
        // Wait briefly for the container to either stabilize or crash
        await new Promise((r) => setTimeout(r, 2000))
        const updated = await fetchProjects()
        const project = updated.find((p) => p.projectId === id && p.agentType === agentType)
        if (project?.state === 'running') {
          toast.success('Project started')
        } else {
          toast.error(`Project failed to start (${project?.status ?? 'unknown'})`)
        }
        refetch()
      } catch (err) {
        if (err instanceof ApiError && err.code === 'STALE_MOUNTS') {
          const match = projects.find((p) => p.projectId === id && p.agentType === agentType)
          setStaleMountsProject({
            id,
            agentType,
            name: match?.name ?? id,
          })
        } else {
          toast.error(err instanceof Error ? err.message : 'Failed to restart project')
        }
      } finally {
        setPendingRestartIds((prev) => {
          const next = new Set(prev)
          next.delete(key)
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

  const handleToggleSelect = useCallback((id: string, agentType: AgentType) => {
    const key = compoundKey(id, agentType)
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(key)) {
        next.delete(key)
      } else {
        next.add(key)
      }
      return next
    })
  }, [])

  const handleClearSelection = () => {
    setSelectedIds(new Set())
  }

  /** Parses a compound key back into its project ID and agent type. */
  const parseKey = (key: string): [string, AgentType] => {
    const [id, agentType] = key.split(':')
    return [id, agentType as AgentType]
  }

  const handleBulkAction = useCallback(
    async (
      action: (id: string, agentType: AgentType) => Promise<unknown>,
      setPending: React.Dispatch<React.SetStateAction<Set<string>>>,
      label: string,
    ) => {
      const keys = [...selectedIds]
      setPending((prev) => new Set([...prev, ...keys]))
      try {
        const results = await Promise.allSettled(
          keys.map((key) => {
            const [id, agentType] = parseKey(key)
            return action(id, agentType)
          }),
        )
        const failed = results.filter((r) => r.status === 'rejected')
        if (failed.length > 0) {
          toast.error(
            `Failed to ${label} ${failed.length} project${failed.length !== 1 ? 's' : ''}`,
          )
        } else {
          toast.success(
            `${keys.length} project${keys.length !== 1 ? 's' : ''} ${label}${label.endsWith('e') ? 'd' : 'ed'}`,
          )
        }
        refetch()
      } finally {
        setPending((prev) => {
          const next = new Set(prev)
          keys.forEach((key) => next.delete(key))
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

  const handleQuickAdd = useCallback(
    async (agentType: AgentType) => {
      if (!serverSettings?.workingDirectory) return
      try {
        const defaults = await fetchDefaults()
        const mounts = defaults.mounts
          .filter((m) => !m.agentType || m.agentType === agentType)
          .map(({ hostPath, containerPath, readOnly }) => ({ hostPath, containerPath, readOnly }))

        const result = await addProject('warden', serverSettings.workingDirectory, agentType)
        await createContainer(result.projectId, agentType, {
          name: `warden-${agentType}`,
          image: '',
          projectPath: serverSettings.workingDirectory,
          agentType,
          skipPermissions: true,
          networkMode: 'restricted',
          allowedDomains: [...restrictedDomains],
          mounts,
        })
        toast.success(`${agentType} project created`)
        refetch()
        navigate(`/projects/${result.projectId}/${agentType}`)
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Failed to quick-add project')
      }
    },
    [serverSettings?.workingDirectory, refetch, navigate],
  )

  useEffect(() => {
    if (error) toast.error(error)
  }, [error])

  // Clear selection when clicking outside cards anywhere on the page.
  useEffect(() => {
    if (selectedIds.size === 0) return

    const handleClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement
      if (target.closest('[data-testid^="project-card-"], button, a, [role="dialog"]')) return
      setSelectedIds(new Set())
    }

    document.addEventListener('click', handleClick)
    return () => document.removeEventListener('click', handleClick)
  }, [selectedIds.size])

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
          {import.meta.env.DEV && serverSettings?.workingDirectory && (
            <>
              <Button
                size="sm"
                variant="outline"
                color="warning"
                onClick={() => handleQuickAdd('claude-code')}
                icon={Plus}
              >
                Claude (dev)
              </Button>
              <Button
                size="sm"
                variant="outline"
                color="warning"
                onClick={() => handleQuickAdd('codex')}
                icon={Plus}
              >
                Codex (dev)
              </Button>
            </>
          )}
          <Button
            data-testid="add-project-button"
            size="sm"
            onClick={() => setIsAddDialogOpen(true)}
            icon={Plus}
          >
            Add Project
          </Button>
        </div>
      </div>

      <CostDashboard projects={projects} />

      <ProjectGrid
        projects={projects}
        isLoading={isLoading}
        onStop={handleStop}
        onRestart={handleRestart}
        onRemove={handleRemove}
        onEdit={handleEdit}
        selectedIds={selectedIds}
        onToggleSelect={handleToggleSelect}
        pendingStopIds={pendingStopIds}
        pendingRestartIds={pendingRestartIds}
        budgetActionPreventStart={budgetActionPreventStart}
      />

      {selectedIds.size > 0 && (
        <div className="bg-background fixed bottom-6 left-1/2 flex -translate-x-1/2 items-center gap-3 rounded border px-5 py-3 shadow-lg">
          <span className="text-muted-foreground">{selectedIds.size} selected</span>
          <Button
            size="sm"
            variant="success"
            onClick={handleRestartSelected}
            disabled={pendingStopIds.size > 0 || pendingRestartIds.size > 0}
            icon={pendingRestartIds.size > 0 ? Loader2 : Play}
            loading={pendingRestartIds.size > 0}
          >
            Start
          </Button>
          <Button
            size="sm"
            variant="error"
            onClick={handleStopSelected}
            disabled={pendingStopIds.size > 0 || pendingRestartIds.size > 0}
            icon={pendingStopIds.size > 0 ? Loader2 : Square}
            loading={pendingStopIds.size > 0}
          >
            Stop
          </Button>
          <Button
            size="sm"
            variant="warning"
            onClick={handleRestartSelected}
            disabled={pendingStopIds.size > 0 || pendingRestartIds.size > 0}
            icon={pendingRestartIds.size > 0 ? Loader2 : RotateCw}
            loading={pendingRestartIds.size > 0}
          >
            Restart
          </Button>
          <Button size="sm" variant="ghost" onClick={handleClearSelection} icon={X}>
            Clear
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
        editAgentType={editTarget?.agentType}
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

import { useCallback, useEffect, useState } from 'react'
import { AlertTriangle, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import type { CheckNameResult, ContainerConfig, CreateContainerRequest } from '@/lib/types'
import { DEFAULT_AGENT_TYPE } from '@/lib/types'
import {
  addProject,
  checkContainerName,
  createContainer,
  fetchContainerConfig,
  updateContainer,
} from '@/lib/api'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import ProjectConfigForm, {
  type ProjectConfigFormData,
} from '@/components/home/project-config-form'

/** Pre-fill data for creating a container on an existing no-container project. */
export interface CreateForProject {
  projectId: string
  name: string
  hostPath: string
}

/** Props for the AddProjectDialog component. */
interface AddProjectDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Called after a project is added or created. */
  onProjectAdded: () => void
  /** When set, opens the dialog directly in edit mode for this project. */
  editProjectId?: string | null
  /** Agent type for the project being edited. */
  editAgentType?: string | null
  /** When set, opens in create mode for an existing project that has no container. */
  createForProject?: CreateForProject | null
}

/** Pending confirmation state when a container name is already taken. */
interface PendingConfirm {
  formData: ProjectConfigFormData
  payload: CreateContainerRequest
  checkResult: CheckNameResult
}

/**
 * Dialog for creating or editing project containers.
 *
 * In create mode, checks container name availability before creating anything.
 * If a Warden-managed container already exists, shows a confirmation prompt.
 * When `editProjectId` is provided, opens directly in edit mode.
 * When `createForProject` is provided, skips project registration and
 * creates a container for the existing project.
 */
export default function AddProjectDialog({
  open,
  onOpenChange,
  onProjectAdded,
  editProjectId = null,
  editAgentType = null,
  createForProject = null,
}: AddProjectDialogProps) {
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [editConfig, setEditConfig] = useState<ContainerConfig | null>(null)
  const [isLoadingConfig, setIsLoadingConfig] = useState(false)
  const [pendingConfirm, setPendingConfirm] = useState<PendingConfirm | null>(null)
  const isEditMode = editProjectId !== null

  /** Reset state when dialog opens/closes or edit target changes. */
  useEffect(() => {
    if (!open) {
      setFormError(null)
      setEditConfig(null)
      setIsLoadingConfig(false)
      setPendingConfirm(null)
      return
    }

    if (editProjectId && editAgentType) {
      setIsLoadingConfig(true)
      setFormError(null)
      fetchContainerConfig(editProjectId, editAgentType)
        .then(setEditConfig)
        .catch((err) => {
          setFormError(err instanceof Error ? err.message : 'Failed to load config')
        })
        .finally(() => setIsLoadingConfig(false))
    }
  }, [open, editProjectId, editAgentType])

  /** Builds the container request payload from form data. */
  const buildPayload = (data: ProjectConfigFormData): CreateContainerRequest => ({
    name: data.name,
    image: data.image,
    projectPath: data.projectPath,
    cloneURL: data.cloneURL,
    temporary: data.temporary,
    agentType: data.agentType,
    envVars: data.envVars,
    mounts: data.mounts,
    skipPermissions: data.skipPermissions,
    networkMode: data.networkMode,
    allowedDomains: data.allowedDomains,
    costBudget: data.costBudget,
    enabledAccessItems: data.enabledAccessItems,
    enabledRuntimes: data.enabledRuntimes,
    forwardedPorts: data.forwardedPorts,
  })

  /** Creates the project and/or container. Called after name check passes. */
  const executeCreate = useCallback(
    async (data: ProjectConfigFormData, payload: CreateContainerRequest) => {
      if (createForProject) {
        await createContainer(createForProject.projectId, data.agentType, payload)
        toast.success('Container created')
      } else {
        await addProject(
          data.name,
          data.projectPath,
          data.agentType,
          data.cloneURL,
          data.temporary,
          payload,
        )
        toast.success('Project created')
      }
      onProjectAdded()
      onOpenChange(false)
    },
    [createForProject, onProjectAdded, onOpenChange],
  )

  /** Handles form submission — checks name availability before creating. */
  const handleFormSubmit = useCallback(
    async (data: ProjectConfigFormData) => {
      setIsSubmitting(true)
      setFormError(null)
      setPendingConfirm(null)

      const payload = buildPayload(data)

      try {
        if (isEditMode && editProjectId && editAgentType) {
          const result = await updateContainer(editProjectId, editAgentType, payload)
          toast.success(result.recreated ? 'Container recreated' : 'Container updated')
          onProjectAdded()
          onOpenChange(false)
          return
        }

        // Pre-flight: check if the container name is available before creating anything.
        const check = await checkContainerName(data.name)
        if (!check.available) {
          if (check.managed) {
            setPendingConfirm({ formData: data, payload, checkResult: check })
            return
          }
          setFormError(
            `Container name "${data.name}" is in use by a non-Warden container — remove it manually or choose a different name`,
          )
          return
        }

        await executeCreate(data, payload)
      } catch (err) {
        setFormError(
          err instanceof Error
            ? err.message
            : `Failed to ${isEditMode ? 'update' : 'create'} project`,
        )
      } finally {
        setIsSubmitting(false)
      }
    },
    [isEditMode, editProjectId, editAgentType, onProjectAdded, onOpenChange, executeCreate],
  )

  /** Replaces the existing container after user confirmation. */
  const handleReplace = useCallback(async () => {
    if (!pendingConfirm) return
    setIsSubmitting(true)
    setFormError(null)
    try {
      await executeCreate(pendingConfirm.formData, {
        ...pendingConfirm.payload,
        forceReplace: true,
      })
    } catch (err) {
      setPendingConfirm(null)
      setFormError(err instanceof Error ? err.message : 'Failed to replace container')
    } finally {
      setIsSubmitting(false)
    }
  }, [pendingConfirm, executeCreate])

  const title = isEditMode
    ? 'Edit Container'
    : createForProject
      ? 'Create Container'
      : 'Create Project'
  const description = isEditMode
    ? 'Update the container configuration.'
    : createForProject
      ? `Create a container for ${createForProject.name}.`
      : 'Configure a new project with a container.'

  /** Initial values for create-for-existing mode (pre-fill name and path). */
  const createForExistingDefaults: ContainerConfig | undefined = createForProject
    ? {
        name: createForProject.name,
        projectPath: createForProject.hostPath,
        image: '',
        agentType: DEFAULT_AGENT_TYPE,
        skipPermissions: false,
        networkMode: 'full',
        costBudget: 0,
      }
    : undefined

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="overflow-hidden sm:max-w-5xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        {/* Fixed height so the dialog doesn't jump when switching between steps.
            flex-col so the form's internal scroll area (flex-1 + overflow-y-auto) works. */}
        <div className="flex h-[70vh] flex-col p-2">
          {isLoadingConfig && (
            <div className="flex items-center justify-center py-16">
              <Loader2 className="text-muted-foreground h-5 w-5 animate-spin" />
            </div>
          )}

          {!isLoadingConfig && isEditMode && !editConfig && formError && (
            <div className="border-error/50 bg-error/10 text-error rounded border p-3">
              {formError}
            </div>
          )}

          {pendingConfirm && (
            <div className="flex flex-col items-center justify-center gap-4 py-16">
              <AlertTriangle className="text-warning h-10 w-10" />
              <div className="max-w-md space-y-2 text-center">
                <p className="font-medium">A Warden container already exists with this name</p>
                <p className="text-muted-foreground text-sm">
                  The existing container is {pendingConfirm.checkResult.state ?? 'unknown'}.
                  Replace it to continue, or go back to choose a different name.
                </p>
              </div>
              <div className="flex gap-2">
                <Button variant="ghost" onClick={() => setPendingConfirm(null)}>
                  Go Back
                </Button>
                <Button variant="error" onClick={handleReplace} loading={isSubmitting}>
                  Replace & Continue
                </Button>
              </div>
            </div>
          )}

          {!pendingConfirm && !isLoadingConfig && (!isEditMode || editConfig) && (
            <div className="flex min-h-0 flex-1 flex-col">
              {isEditMode && (
                <p className="text-muted-foreground mb-4 text-sm">
                  Some changes may require the container to be recreated.
                </p>
              )}
              <ProjectConfigForm
                mode={isEditMode ? 'edit' : 'create'}
                initialValues={editConfig ?? createForExistingDefaults}
                onSubmit={handleFormSubmit}
                isSubmitting={isSubmitting}
                error={formError}
              />
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

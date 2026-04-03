import { useCallback, useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import type { ContainerConfig } from '@/lib/types'
import { DEFAULT_AGENT_TYPE } from '@/lib/types'
import { addProject, createContainer, fetchContainerConfig, updateContainer } from '@/lib/api'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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

/**
 * Dialog for creating or editing project containers.
 *
 * In create mode, shows the project config form. Creating a project first
 * registers it via `addProject`, then creates its container.
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

  const isEditMode = editProjectId !== null

  /** Reset state when dialog opens/closes or edit target changes. */
  useEffect(() => {
    if (!open) {
      setFormError(null)
      setEditConfig(null)
      setIsLoadingConfig(false)
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

  /** Handles form submission for create, edit, and create-for-existing modes. */
  const handleFormSubmit = useCallback(
    async (data: ProjectConfigFormData) => {
      setIsSubmitting(true)
      setFormError(null)
      try {
        const payload = {
          name: data.name,
          image: data.image,
          projectPath: data.projectPath,
          agentType: data.agentType,
          envVars: data.envVars,
          mounts: data.mounts,
          skipPermissions: data.skipPermissions,
          networkMode: data.networkMode,
          allowedDomains: data.allowedDomains,
          costBudget: data.costBudget,
          enabledAccessItems: data.enabledAccessItems,
        }

        if (isEditMode && editProjectId && editAgentType) {
          await updateContainer(editProjectId, editAgentType, payload)
          toast.success('Container updated')
        } else if (createForProject) {
          await createContainer(createForProject.projectId, data.agentType, payload)
          toast.success('Container created')
        } else {
          const result = await addProject(data.name, data.projectPath, data.agentType)
          const projectId = result.projectId
          await createContainer(projectId, data.agentType, payload)
          toast.success('Project created')
        }
        onProjectAdded()
        onOpenChange(false)
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
    [isEditMode, editProjectId, editAgentType, createForProject, onProjectAdded, onOpenChange],
  )

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

        <div className="max-h-[70vh] overflow-y-auto px-1">
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

          {!isLoadingConfig && (!isEditMode || editConfig) && (
            <>
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
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

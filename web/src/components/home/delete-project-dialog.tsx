import { AlertTriangle, Loader2, Trash2 } from 'lucide-react'
import { useCallback, useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { deleteContainer, removeProject, resetProjectCosts } from '@/lib/api'
import type { Project } from '@/lib/types'

/** Props for the DeleteProjectDialog component. */
interface DeleteProjectDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  project: Project | null
  /** Called after any management action completes. */
  onComplete: () => void
}

/**
 * Management dialog with three independent destructive actions.
 *
 * Each action is an unchecked checkbox — nothing is assumed. The user
 * explicitly opts into each operation.
 *
 * @param props.project - The project being managed.
 * @param props.onComplete - Called after operations finish so the caller can refetch.
 */
export default function DeleteProjectDialog({
  open,
  onOpenChange,
  project,
  onComplete,
}: DeleteProjectDialogProps) {
  const [removeFromWarden, setRemoveFromWarden] = useState(false)
  const [shouldDeleteContainer, setShouldDeleteContainer] = useState(false)
  const [resetCosts, setResetCosts] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)

  const hasContainer = project?.hasContainer ?? false
  const hasAnyAction = removeFromWarden || shouldDeleteContainer || resetCosts

  const resetState = useCallback(() => {
    setRemoveFromWarden(false)
    setShouldDeleteContainer(false)
    setResetCosts(false)
  }, [])

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (isSubmitting) return
      if (!nextOpen) resetState()
      onOpenChange(nextOpen)
    },
    [isSubmitting, resetState, onOpenChange],
  )

  const handleConfirm = useCallback(async () => {
    if (!project || !hasAnyAction) return

    setIsSubmitting(true)
    const errors: string[] = []

    // Delete container first so no orphaned container remains if a later step fails.
    if (shouldDeleteContainer && hasContainer) {
      try {
        await deleteContainer(project.projectId, project.agentType)
      } catch {
        errors.push('delete container')
      }
    }

    // Cost reset is independent.
    if (resetCosts) {
      try {
        await resetProjectCosts(project.projectId, project.agentType)
      } catch {
        errors.push('reset costs')
      }
    }

    // Remove from Warden last so cost cleanup can resolve the project row.
    if (removeFromWarden) {
      try {
        await removeProject(project.projectId, project.agentType)
      } catch {
        errors.push('remove project')
      }
    }

    setIsSubmitting(false)

    if (errors.length > 0) {
      toast.error(`Failed to: ${errors.join(', ')}`)
      return
    }

    toast.success('Project updated')
    resetState()
    onOpenChange(false)
    onComplete()
  }, [
    project,
    hasAnyAction,
    shouldDeleteContainer,
    hasContainer,
    resetCosts,
    removeFromWarden,
    resetState,
    onOpenChange,
    onComplete,
  ])

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Delete Project Items</DialogTitle>
          <DialogDescription>
            Select <strong>{project?.name}</strong> items to delete.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <ActionCheckbox
            id="remove-from-warden"
            checked={removeFromWarden}
            onCheckedChange={setRemoveFromWarden}
            disabled={isSubmitting}
            label="Remove from Warden"
            description="Untrack this project. Does not affect the container or its data."
          />

          <ActionCheckbox
            id="delete-container"
            checked={shouldDeleteContainer}
            onCheckedChange={setShouldDeleteContainer}
            disabled={!hasContainer || isSubmitting}
            label="Docker container"
            description={
              hasContainer ? 'Stop and permanently remove the container.' : 'No container exists.'
            }
          />

          <ActionCheckbox
            id="reset-costs"
            checked={resetCosts}
            onCheckedChange={setResetCosts}
            disabled={isSubmitting}
            label="Cost history"
            description="Clear all tracked cost data for this project."
          />

          {removeFromWarden && !shouldDeleteContainer && hasContainer && (
            <div className="border-warning/50 bg-warning/10 flex items-start gap-2 rounded border p-3">
              <AlertTriangle className="text-warning mt-0.5 h-4 w-4 shrink-0" />
              <p className="text-warning text-sm">
                The container will remain on disk. Re-add the same directory to reconnect.
              </p>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => handleOpenChange(false)} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button
            variant="error"
            onClick={handleConfirm}
            disabled={!hasAnyAction || isSubmitting}
            icon={isSubmitting ? Loader2 : Trash2}
            loading={isSubmitting}
          >
            {isSubmitting ? 'Processing…' : 'Delete'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** Reusable checkbox row for a management action. */
function ActionCheckbox({
  id,
  checked,
  onCheckedChange,
  disabled,
  label,
  description,
}: {
  id: string
  checked: boolean
  onCheckedChange: (checked: boolean) => void
  disabled: boolean
  label: string
  description: React.ReactNode
}) {
  return (
    <div className="flex items-center gap-3">
      <Checkbox
        id={id}
        checked={checked}
        onCheckedChange={(v) => onCheckedChange(v === true)}
        disabled={disabled}
      />
      <div className="space-y-0.5">
        <label htmlFor={id} className="cursor-pointer leading-none font-medium">
          {label}
        </label>
        <p className="text-muted-foreground text-sm">{description}</p>
      </div>
    </div>
  )
}

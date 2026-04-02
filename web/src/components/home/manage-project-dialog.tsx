import { useCallback, useState } from 'react'
import { AlertTriangle, FolderCog, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { deleteContainer, removeProject, resetProjectCosts, purgeProjectAudit } from '@/lib/api'
import { deleteProjectScrollback } from '@/lib/scrollback-db'
import type { Project } from '@/lib/types'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

/** Props for the ManageProjectDialog component. */
interface ManageProjectDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  project: Project | null
  /** Called after any management action completes. */
  onComplete: () => void
}

/**
 * Returns the text the user must type to confirm audit purge.
 *
 * Uses the project name so the confirmation is project-specific.
 */
function purgeConfirmation(name: string): string {
  return name
}

/**
 * Management dialog with four independent destructive actions.
 *
 * Each action is an unchecked checkbox — nothing is assumed. The user
 * explicitly opts into each operation. "Purge audit history" requires
 * type-to-confirm to prevent accidental data loss.
 *
 * @param props.project - The project being managed.
 * @param props.onComplete - Called after operations finish so the caller can refetch.
 */
export default function ManageProjectDialog({
  open,
  onOpenChange,
  project,
  onComplete,
}: ManageProjectDialogProps) {
  const [removeFromWarden, setRemoveFromWarden] = useState(false)
  const [shouldDeleteContainer, setShouldDeleteContainer] = useState(false)
  const [resetCosts, setResetCosts] = useState(false)
  const [purgeAudit, setPurgeAudit] = useState(false)
  const [purgeConfirmText, setPurgeConfirmText] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const hasContainer = project?.hasContainer ?? false
  const hasAnyAction = removeFromWarden || shouldDeleteContainer || resetCosts || purgeAudit
  const confirmWord = project ? purgeConfirmation(project.name) : ''
  const isPurgeConfirmed = !purgeAudit || purgeConfirmText === confirmWord

  const resetState = useCallback(() => {
    setRemoveFromWarden(false)
    setShouldDeleteContainer(false)
    setResetCosts(false)
    setPurgeAudit(false)
    setPurgeConfirmText('')
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
    if (!project || !hasAnyAction || !isPurgeConfirmed) return

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

    // Cost reset and audit purge are independent — run in parallel.
    const dataCleanup: Promise<unknown>[] = []
    if (resetCosts) dataCleanup.push(resetProjectCosts(project.projectId, project.agentType))
    if (purgeAudit) dataCleanup.push(purgeProjectAudit(project.projectId, project.agentType))
    const results = await Promise.allSettled(dataCleanup)
    const dataLabels = [resetCosts && 'reset costs', purgeAudit && 'purge audit'].filter(Boolean)
    results.forEach((r, i) => {
      if (r.status === 'rejected') errors.push(dataLabels[i] as string)
    })

    // Remove from Warden last so cost/audit cleanup can resolve the project row.
    if (removeFromWarden) {
      try {
        await removeProject(project.projectId, project.agentType)
        void deleteProjectScrollback(project.projectId)
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
    isPurgeConfirmed,
    shouldDeleteContainer,
    hasContainer,
    resetCosts,
    purgeAudit,
    removeFromWarden,
    resetState,
    onOpenChange,
    onComplete,
  ])

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Manage Project</DialogTitle>
          <DialogDescription>
            Select actions to perform on <strong>{project?.name}</strong>.
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
            label="Delete container"
            description={
              hasContainer ? 'Stop and permanently remove the container.' : 'No container exists.'
            }
          />

          <ActionCheckbox
            id="reset-costs"
            checked={resetCosts}
            onCheckedChange={setResetCosts}
            disabled={isSubmitting}
            label="Reset cost history"
            description="Clear all tracked cost data for this project."
          />

          <ActionCheckbox
            id="purge-audit"
            checked={purgeAudit}
            onCheckedChange={setPurgeAudit}
            disabled={isSubmitting}
            label="Purge audit history"
            description="Permanently delete all audit events for this project."
          />

          <div
            className={cn(
              'ml-7 grid transition-all duration-200 ease-out',
              purgeAudit ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0',
            )}
          >
            <div className="overflow-hidden">
              <div className="space-y-2 pb-1">
                <p className="text-error text-sm">
                  This is irreversible. Type <strong>{confirmWord}</strong> to confirm.
                </p>
                <Input
                  value={purgeConfirmText}
                  onChange={(e) => setPurgeConfirmText(e.target.value)}
                  placeholder={confirmWord}
                  disabled={isSubmitting}
                  className="max-w-48"
                />
              </div>
            </div>
          </div>

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
            disabled={!hasAnyAction || !isPurgeConfirmed || isSubmitting}
            icon={isSubmitting ? Loader2 : undefined}
            loading={isSubmitting}
          >
            {isSubmitting ? 'Processing…' : 'Confirm'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** Icon used for the manage button on project cards. */
export { FolderCog as ManageIcon }

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

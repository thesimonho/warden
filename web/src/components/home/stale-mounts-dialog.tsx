import { Loader2, RefreshCw } from 'lucide-react'
import { useCallback, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { fetchContainerConfig, updateContainer } from '@/lib/api'

/** Props for the StaleMountsDialog component. */
interface StaleMountsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The project that failed to start due to stale mounts. */
  project: { id: string; agentType: string; name: string } | null
  /** Called after successful recreation to refresh the project list. */
  onRecreated: () => void
}

/**
 * Dialog shown when a container start is blocked by stale bind mounts.
 *
 * This happens when a dotfile manager (Nix Home Manager, GNU Stow) changes
 * symlink targets after the container was created. The only recovery is to
 * recreate the container so it picks up the new symlink targets.
 */
export default function StaleMountsDialog({
  open,
  onOpenChange,
  project,
  onRecreated,
}: StaleMountsDialogProps) {
  const [isRecreating, setIsRecreating] = useState(false)

  const handleRecreate = useCallback(async () => {
    if (!project) return
    setIsRecreating(true)
    try {
      const config = await fetchContainerConfig(project.id, project.agentType)
      await updateContainer(project.id, project.agentType, {
        name: config.name,
        image: config.image,
        projectPath: config.projectPath,
        mounts: config.mounts,
        envVars: config.envVars,
        networkMode: config.networkMode,
        allowedDomains: config.allowedDomains,
        skipPermissions: config.skipPermissions,
        costBudget: config.costBudget,
        enabledAccessItems: config.enabledAccessItems,
      })
      toast.success('Container recreated')
      onRecreated()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to recreate container')
    } finally {
      setIsRecreating(false)
    }
  }, [project, onRecreated])

  const handleOpenChange = (nextOpen: boolean) => {
    if (isRecreating) return
    onOpenChange(nextOpen)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{project?.name} cannot start</DialogTitle>
          <DialogDescription>Bind mounts reference outdated targets.</DialogDescription>
        </DialogHeader>

        <p className="text-muted-foreground text-sm">
          Recreating the container will resolve the mount targets and create fresh bind mounts.
          Worktree data will be preserved in your project directory.
        </p>

        <DialogFooter>
          <Button variant="ghost" onClick={() => handleOpenChange(false)} disabled={isRecreating}>
            Dismiss
          </Button>
          <Button
            variant="success"
            onClick={handleRecreate}
            disabled={isRecreating}
            icon={isRecreating ? Loader2 : RefreshCw}
            loading={isRecreating}
          >
            {isRecreating ? 'Recreating…' : 'Recreate Container'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

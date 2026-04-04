import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

/** Props for the RemoveWorktreeDialog component. */
interface RemoveWorktreeDialogProps {
  open: boolean
  label: string
  /** Controls the dialog copy. Defaults to "delete" (remove worktree from disk). */
  variant?: 'delete' | 'reset'
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
}

/**
 * Confirmation dialog shown before removing or resetting a worktree.
 *
 * In "delete" mode (default), warns that `git worktree remove` will run.
 * In "reset" mode, warns that all history will be cleared but the worktree is preserved.
 */
export default function RemoveWorktreeDialog({
  open,
  label,
  variant = 'delete',
  onOpenChange,
  onConfirm,
}: RemoveWorktreeDialogProps) {
  const isReset = variant === 'reset'

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {isReset ? <>Reset &quot;{label}&quot;?</> : <>Delete worktree &quot;{label}&quot;?</>}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {isReset
              ? 'This will stop the agent, clear session data, and start fresh. The worktree and audit history are preserved.'
              : 'This will run git worktree remove and delete the working directory. Any uncommitted changes in this worktree will be lost.'}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            className="bg-error text-error-foreground hover:bg-error/90"
          >
            {isReset ? 'Reset' : 'Delete'}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

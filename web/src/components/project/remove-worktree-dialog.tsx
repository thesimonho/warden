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
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
}

/**
 * Confirmation dialog shown before removing a worktree.
 *
 * Explains that `git worktree remove` will run and uncommitted changes
 * will be lost, while the branch itself is preserved.
 */
export default function RemoveWorktreeDialog({
  open,
  label,
  onOpenChange,
  onConfirm,
}: RemoveWorktreeDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete worktree &quot;{label}&quot;?</AlertDialogTitle>
          <AlertDialogDescription>
            This will run <code>git worktree remove</code> and delete the working directory. Any
            uncommitted changes in this worktree will be lost.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            className="bg-error text-error-foreground hover:bg-error/90"
          >
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { createWorktree } from '@/lib/api'

/** Props for the NewWorktreeDialog component. */
interface NewWorktreeDialogProps {
  projectId: string
  agentType: string
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (worktreeId: string) => void
}

/**
 * Dialog for creating a new git worktree with an attached terminal.
 *
 * The name is used as both the worktree directory name and the initial
 * branch name. A terminal is automatically connected after creation.
 *
 * @param props.projectId - The project to create the worktree in.
 * @param props.open - Whether the dialog is open.
 * @param props.onOpenChange - Callback to control dialog visibility.
 * @param props.onCreated - Callback invoked with the worktree ID on success.
 */
export default function NewWorktreeDialog({
  projectId,
  agentType,
  open,
  onOpenChange,
  onCreated,
}: NewWorktreeDialogProps) {
  const [name, setName] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) {
      setName('')
      setError(null)
    }
  }, [open])

  /**
   * Sanitizes a worktree name for use as a git branch/directory name.
   * Replaces invalid characters with hyphens and collapses runs of hyphens.
   */
  const sanitizeName = (input: string): string => {
    return (
      input
        // Replace whitespace and git-invalid characters with hyphens.
        // eslint-disable-next-line no-control-regex -- intentional: sanitizes git ref name rules
        .replace(/[\s~^:?*\[\]\\@{}\x00-\x1f\x7f]+/g, '-')
        // Collapse consecutive dots (invalid in git refs).
        .replace(/\.{2,}/g, '.')
        // Collapse consecutive hyphens from substitution.
        .replace(/-{2,}/g, '-')
        // Strip leading dots and hyphens.
        .replace(/^[.-]+/, '')
    )
  }

  /** Validates the sanitized name before submission. */
  const validateName = (input: string): string | null => {
    if (!input) return 'Worktree name is required'
    if (input.endsWith('.lock') || input.endsWith('.')) return 'Name cannot end with .lock or a dot'
    return null
  }

  const handleSubmit = async () => {
    const trimmedName = name.trim()
    const validationError = validateName(trimmedName)
    if (validationError) {
      setError(validationError)
      return
    }

    setIsSubmitting(true)
    setError(null)

    try {
      const response = await createWorktree(projectId, agentType, trimmedName)
      setName('')
      onOpenChange(false)
      onCreated(response.worktreeId)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create worktree'
      setError(message)
      toast.error('Failed to create worktree', { description: message })
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !isSubmitting) {
      e.preventDefault()
      handleSubmit()
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>New Worktree</DialogTitle>
          <DialogDescription>
            Create an isolated working directory for independent work. A new git branch will be
            created automatically.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-1.5">
          <Input
            data-testid="worktree-name-input"
            placeholder="Worktree name (e.g. feature-auth)"
            value={name}
            onChange={(e) => {
              setName(sanitizeName(e.target.value))
              if (error) setError(null)
            }}
            onKeyDown={handleKeyDown}
            disabled={isSubmitting}
            autoFocus
          />
          <p className="text-muted-foreground text-sm">
            Letters, numbers, hyphens, and underscores only.
          </p>
        </div>

        {error && <p className="text-error">{error}</p>}

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button
            data-testid="create-worktree-button"
            onClick={handleSubmit}
            disabled={isSubmitting || !name.trim()}
          >
            {isSubmitting ? 'Creating...' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

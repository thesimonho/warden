import { ExternalLink, X } from 'lucide-react'

import { cn } from '@/lib/utils'

/** Base styling shared by all port chip variants. */
const baseClasses = cn(
  'inline-flex items-center gap-1 rounded p-1',
  'bg-secondary text-secondary-foreground',
  'font-mono text-xs',
)

/** Props for a removable port chip (used in the config form). */
interface RemovablePortChipProps {
  port: number
  disabled?: boolean
  onRemove: () => void
}

/** A port chip with an X button for removal. */
export function RemovablePortChip({ port, disabled, onRemove }: RemovablePortChipProps) {
  return (
    <span className={baseClasses}>
      :{port}
      <button
        type="button"
        onClick={onRemove}
        disabled={disabled}
        className={cn(
          'rounded-full',
          'hover:bg-muted-foreground/20 transition-colors',
          'cursor-pointer disabled:opacity-50',
        )}
      >
        <X className="h-3 w-3" />
      </button>
    </span>
  )
}

/** Props for a linkable port chip (used on project cards). */
interface LinkPortChipProps {
  port: number
  href: string
}

/** A port chip that links to the proxy URL in a new tab. */
export function LinkPortChip({ port, href }: LinkPortChipProps) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      title={`Open port ${port} in new tab`}
      data-testid={`port-chip-${port}`}
      onClick={(e) => e.stopPropagation()}
      className={cn(baseClasses, 'hover:bg-secondary/80 transition-colors')}
    >
      :{port}
      <ExternalLink className="h-3 w-3" />
    </a>
  )
}

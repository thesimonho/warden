import type { ClaudeStatus, NotificationType } from '@/lib/types'
import { getAttentionConfig } from '@/lib/notification-config'
import { cn } from '@/lib/utils'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

/** Props for the ClaudeStatusIndicator component. */
interface ClaudeStatusIndicatorProps {
  status: ClaudeStatus
  /** When true, overrides the display to show an attention alert. */
  needsInput?: boolean
  /** The specific type of attention needed, for context-specific labels. */
  notificationType?: NotificationType
}

/** Maps Claude status to display properties. */
const statusConfig: Record<ClaudeStatus, { label: string; dotClass: string }> = {
  working: { label: 'Working', dotClass: 'bg-warning animate-pulse' },
  idle: { label: 'Idle', dotClass: 'bg-muted-foreground/40' },
  unknown: { label: 'Status Unknown', dotClass: 'bg-muted-foreground/20' },
}

/**
 * Displays the Claude Code process status as a labeled dot indicator.
 * When needsInput is true, overrides the status display with a
 * context-specific attention alert based on the notification type.
 *
 * @param props.status - The current Claude status for the container.
 * @param props.needsInput - Whether Claude is blocked waiting for user attention.
 * @param props.notificationType - The specific kind of attention needed.
 */
export default function ClaudeStatusIndicator({
  status,
  needsInput,
  notificationType,
}: ClaudeStatusIndicatorProps) {
  const config = needsInput ? getAttentionConfig(notificationType) : statusConfig[status]

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className="text-muted-foreground flex items-center gap-1.5 text-sm">
          <span className={cn('inline-block h-2 w-2 rounded-full', config.dotClass)} />
          <span>{config.label}</span>
        </div>
      </TooltipTrigger>
      <TooltipContent>Agent Status</TooltipContent>
    </Tooltip>
  )
}

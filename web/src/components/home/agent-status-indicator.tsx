import type { AgentStatus, NotificationType } from '@/lib/types'
import { getAttentionConfig } from '@/lib/notification-config'
import { cn } from '@/lib/utils'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

/** Props for the AgentStatusIndicator component. */
interface AgentStatusIndicatorProps {
  status: AgentStatus
  /** When true, overrides the display to show an attention alert. */
  needsInput?: boolean
  /** The specific type of attention needed, for context-specific labels. */
  notificationType?: NotificationType
}

/** Maps agent status to display properties. */
const statusConfig: Record<AgentStatus, { label: string; dotClass: string }> = {
  working: { label: 'Working', dotClass: 'bg-warning animate-pulse' },
  idle: { label: 'Idle', dotClass: 'bg-muted-foreground/40' },
  unknown: { label: 'Status Unknown', dotClass: 'bg-muted-foreground/20' },
}

/**
 * Displays the agent CLI process status as a labeled dot indicator.
 * When needsInput is true, overrides the status display with a
 * context-specific attention alert based on the notification type.
 *
 * @param props.status - The current agent status for the container.
 * @param props.needsInput - Whether the agent is blocked waiting for user attention.
 * @param props.notificationType - The specific kind of attention needed.
 */
export default function AgentStatusIndicator({
  status,
  needsInput,
  notificationType,
}: AgentStatusIndicatorProps) {
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

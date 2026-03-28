import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

/** Props for the StatusBadge component. */
interface StatusBadgeProps {
  state: string
  'data-testid'?: string
}

/** Maps Docker container states to badge visual variants. */
const stateVariantMap: Record<string, 'default' | 'success' | 'warning' | 'error' | 'outline'> = {
  running: 'success',
  paused: 'warning',
  exited: 'error',
  restarting: 'outline',
  dead: 'error',
  created: 'outline',
  'no container': 'outline',
}

/**
 * Displays a color-coded badge for a Docker container state.
 *
 * @param props.state - The Docker container state string.
 */
export default function StatusBadge({ state, 'data-testid': testId }: StatusBadgeProps) {
  const variant = stateVariantMap[state] ?? 'outline'

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Badge variant={variant} className="capitalize" data-testid={testId}>
          {state}
        </Badge>
      </TooltipTrigger>
      <TooltipContent>Container Status</TooltipContent>
    </Tooltip>
  )
}

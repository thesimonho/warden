import type { NotificationType } from '@/lib/types'

/** Per-notification-type display and messaging metadata. */
interface NotificationConfig {
  /** Short label for status indicators (e.g. "Need Permission"). */
  label: string
  /** Tailwind classes for the attention dot indicator. */
  dotClass: string
  /** Tailwind text color class matching the dot color. */
  textClass: string
  /** Browser notification body text. */
  message: string
}

/**
 * Single source of truth for notification type display properties.
 *
 * `auth_success` is included for completeness but is not treated as an
 * attention-requiring state — see `isAttentionType`.
 */
const notificationConfig: Record<NotificationType, NotificationConfig> = {
  permission_prompt: {
    label: 'Need Permission',
    dotClass: 'bg-warning animate-pulse',
    textClass: 'text-warning',
    message: 'A worktree needs tool approval.',
  },
  elicitation_dialog: {
    label: 'Need Answer',
    dotClass: 'bg-error animate-pulse',
    textClass: 'text-error',
    message: 'Claude is asking a question.',
  },
  idle_prompt: {
    label: 'Need Input',
    dotClass: 'bg-info animate-pulse',
    textClass: 'text-info',
    message: 'Claude is done and waiting for the next prompt.',
  },
  auth_success: {
    label: 'Auth complete',
    dotClass: 'bg-muted-foreground/20',
    textClass: 'text-muted-foreground',
    message: 'Authentication completed.',
  },
}

/** Default display config when notification type is unknown. */
const defaultConfig: NotificationConfig = {
  label: 'Need Input',
  dotClass: 'bg-error animate-pulse',
  textClass: 'text-error',
  message: 'A worktree is waiting for your response.',
}

/**
 * Returns the display config for a given notification type.
 * Falls back to a generic "needs input" config for unknown types.
 */
export function getAttentionConfig(notificationType?: NotificationType): NotificationConfig {
  return (notificationType && notificationConfig[notificationType]) || defaultConfig
}

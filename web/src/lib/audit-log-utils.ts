import {
  Bot,
  Bug,
  Coins,
  type LucideIcon,
  MessageSquare,
  Play,
  Settings2,
  Terminal,
} from 'lucide-react'
import { toast } from 'sonner'

import type { AuditCategory, AuditLogEntry, AuditLogLevel } from '@/lib/types'

// --- Category metadata (single source of truth) ---

/** Category display info used by filter tabs, badges, and table columns. */
export interface CategoryMeta {
  id: AuditCategory
  label: string
  icon: LucideIcon
  /** Tailwind text class for colored text. */
  textStyle: string
  /** Tailwind classes for badge styling (text + border). */
  badgeStyle: string
}

/** All audit categories with display metadata in canonical order. */
export const AUDIT_CATEGORIES: CategoryMeta[] = [
  {
    id: 'session',
    label: 'Sessions',
    icon: Play,
    textStyle: 'text-(--category-session)',
    badgeStyle: 'text-(--category-session) border-(--category-session)/30',
  },
  {
    id: 'agent',
    label: 'Agent',
    icon: Bot,
    textStyle: 'text-(--category-agent)',
    badgeStyle: 'text-(--category-agent) border-(--category-agent)/30',
  },
  {
    id: 'prompt',
    label: 'Prompts',
    icon: MessageSquare,
    textStyle: 'text-(--category-prompt)',
    badgeStyle: 'text-(--category-prompt) border-(--category-prompt)/30',
  },
  {
    id: 'config',
    label: 'Config',
    icon: Settings2,
    textStyle: 'text-(--category-config)',
    badgeStyle: 'text-(--category-config) border-(--category-config)/30',
  },
  {
    id: 'budget',
    label: 'Budget',
    icon: Coins,
    textStyle: 'text-(--category-budget)',
    badgeStyle: 'text-(--category-budget) border-(--category-budget)/30',
  },
  {
    id: 'system',
    label: 'System',
    icon: Terminal,
    textStyle: 'text-muted-foreground',
    badgeStyle: 'text-muted-foreground border-muted-foreground/30',
  },
  {
    id: 'debug',
    label: 'Debug',
    icon: Bug,
    textStyle: 'text-(--category-debug)',
    badgeStyle: 'text-(--category-debug) border-(--category-debug)/30',
  },
]

/** Category sort order derived from canonical array position. */
export const categoryOrder: Record<string, number> = Object.fromEntries(
  AUDIT_CATEGORIES.map(({ id }, i) => [id, i]),
)

// --- Level metadata ---

/** Level CSS variable names for filter tab coloring. */
export const levelColorVar: Record<AuditLogLevel, string> = {
  info: '--color-info',
  warn: '--color-warning',
  error: '--color-error',
}

/** All severity levels in display order. */
export const ALL_LEVELS = ['info', 'warn', 'error'] as const satisfies readonly AuditLogLevel[]

/**
 * Formats an ISO timestamp to a compact local date+time string.
 *
 * @param iso - ISO 8601 timestamp.
 * @param includeMs - Whether to include milliseconds (default: false).
 */
export function formatTimestamp(iso: string, includeMs = false): string {
  const date = new Date(iso)
  return date.toLocaleString('en-US', {
    month: '2-digit',
    day: '2-digit',
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    ...(includeMs ? { fractionalSecondDigits: 3 as const } : {}),
  })
}

/**
 * Formats an ISO timestamp to a full date + time string.
 *
 * @param iso - ISO 8601 timestamp.
 * @param includeMs - Whether to include milliseconds (default: false).
 */
export function formatFullTimestamp(iso: string, includeMs = false): string {
  const date = new Date(iso)
  return date.toLocaleString('en-US', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    ...(includeMs ? { fractionalSecondDigits: 3 as const } : {}),
  })
}

/** Copies an audit log entry as JSON to clipboard. */
export async function copyEntry(entry: AuditLogEntry): Promise<void> {
  await navigator.clipboard.writeText(JSON.stringify(entry, null, 2))
  toast.success('Entry copied')
}

/** Builds a stable key for an audit log entry. */
export function entryKey(entry: AuditLogEntry): string {
  return String(entry.id)
}

/** Converts a snake_case event name to a human-readable label. */
export function eventLabel(event: string): string {
  return event.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

/** Fields known to carry JSON-encoded strings from Claude Code hooks. */
const JSON_STRING_FIELDS = new Set(['toolInput'])

/**
 * Prepares event data for human-readable display in the expanded row detail.
 * Shallow — only processes top-level string values.
 *
 * - Parses known JSON-encoded string fields (e.g. `toolInput`) into objects.
 * - Converts literal `\n` sequences in string values to real newlines.
 */
export function formatDataForDisplay(data: Record<string, unknown>): Record<string, unknown> {
  const result: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(data)) {
    if (typeof value !== 'string') {
      result[key] = value
      continue
    }
    // Parse known JSON string fields into objects for readable display.
    if (JSON_STRING_FIELDS.has(key) && (value.startsWith('{') || value.startsWith('['))) {
      try {
        result[key] = JSON.parse(value)
        continue
      } catch {
        // Not valid JSON — fall through to string handling.
      }
    }
    // Convert literal \n sequences to real newlines for display.
    result[key] = value.includes('\\n') ? value.replace(/\\n/g, '\n') : value
  }
  return result
}

/** Extracts a display message from an event entry. */
export function entryMessage(entry: AuditLogEntry): string {
  const data = entry.data

  // For user_prompt events, prefer data.prompt over msg — the msg field
  // may contain a [bash] prefix baked in by the Go backend for TUI display,
  // but the web UI renders bash indicators visually via promptSource().
  if (entry.event === 'user_prompt' && data?.prompt) {
    // Collapse newlines/carriage returns to spaces for single-line table display.
    // CSS truncate handles overflow at column width — no JS length limit needed.
    return (data.prompt as string).replace(/[\r\n]+/g, ' ')
  }

  if (entry.msg) return entry.msg
  if (!data) return ''

  if (entry.event === 'tool_use' && data.toolName) return data.toolName as string
  if (entry.event === 'session_start' && data.model) return `Model: ${data.model as string}`
  if (entry.event === 'session_end' && data.reason) return `Reason: ${data.reason as string}`
  if (entry.event === 'tool_use_failure') {
    return [data.toolName as string, data.error as string].filter(Boolean).join(': ')
  }
  if (entry.event === 'stop_failure' && data.error) return data.error as string
  if (entry.event === 'permission_request' && data.toolName) return data.toolName as string
  if ((entry.event === 'subagent_start' || entry.event === 'subagent_stop') && data.agentType) {
    return data.agentType as string
  }
  if (entry.event === 'task_completed' && data.taskSubject) return data.taskSubject as string
  if (entry.event === 'config_change' && data.source) {
    return `${data.source as string}${data.filePath ? `: ${data.filePath as string}` : ''}`
  }
  if (entry.event === 'instructions_loaded' && data.filePath) return data.filePath as string
  if (
    (entry.event === 'elicitation' || entry.event === 'elicitation_result') &&
    data.mcpServerName
  ) {
    return data.mcpServerName as string
  }
  return ''
}

/** Returns the prompt source for a user_prompt entry, if set. */
export function promptSource(entry: AuditLogEntry): 'bash' | 'bash_output' | undefined {
  if (entry.event !== 'user_prompt' || !entry.data) return undefined
  const source = entry.data.promptSource as string | undefined
  if (source === 'bash' || source === 'bash_output') return source
  return undefined
}

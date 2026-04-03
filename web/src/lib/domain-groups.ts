import type { AgentType } from '@/lib/types'

/**
 * Returns the default restricted domains for an agent type from the
 * server-provided defaults. Falls back to an empty list if not available.
 *
 * @param restrictedDomains - The map from the defaults API response.
 * @param agentType - The agent type to look up.
 */
export function getRestrictedDomains(
  restrictedDomains: Record<string, string[]> | undefined,
  agentType: AgentType,
): readonly string[] {
  return restrictedDomains?.[agentType] ?? []
}

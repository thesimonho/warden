/**
 * Pure functions for merging project template values with server defaults.
 *
 * These are extracted from the form component so they can be unit-tested
 * without React rendering. The form calls these helpers and applies the
 * results to state.
 *
 * @module
 */
import type { AgentType, ProjectTemplate, RuntimeDefault } from '@/lib/types'

/**
 * Computes runtime toggle state from server defaults and an optional template.
 *
 * Without a template, runtimes are enabled based on detection + alwaysEnabled.
 * With a template, the template's runtime list takes precedence, but
 * alwaysEnabled runtimes remain enabled regardless.
 */
export function resolveRuntimeToggles(
  runtimeDefaults: RuntimeDefault[],
  template?: ProjectTemplate | null,
): Record<string, boolean> {
  const toggles: Record<string, boolean> = {}

  if (template?.runtimes) {
    const templateSet = new Set(template.runtimes)
    for (const r of runtimeDefaults) {
      toggles[r.id] = templateSet.has(r.id) || r.alwaysEnabled
    }
  } else {
    for (const r of runtimeDefaults) {
      toggles[r.id] = r.alwaysEnabled || r.detected
    }
  }

  return toggles
}

/**
 * Computes the env var entries contributed by enabled runtimes.
 * These are shown as read-only entries in the form and stripped
 * before submission (managed via WARDEN_ENABLED_RUNTIMES instead).
 */
export function resolveRuntimeEnvVars(
  runtimeDefaults: RuntimeDefault[],
  toggles: Record<string, boolean>,
): { key: string; value: string }[] {
  const entries: { key: string; value: string }[] = []
  for (const r of runtimeDefaults) {
    if (toggles[r.id] && r.envVars) {
      for (const [key, value] of Object.entries(r.envVars)) {
        entries.push({ key, value })
      }
    }
  }
  return entries
}

/**
 * Merges runtime-contributed domains into a base domain list.
 * Mirrors the server-side `mergeRuntimeDomains` logic so the form UI
 * shows the full set of domains the container will actually receive.
 */
export function mergeRuntimeDomains(
  baseDomains: string[],
  runtimeDefaults: RuntimeDefault[],
  toggles: Record<string, boolean>,
): string[] {
  const existing = new Set(baseDomains)
  const merged = [...baseDomains]
  for (const r of runtimeDefaults) {
    if (toggles[r.id] && r.domains) {
      for (const d of r.domains) {
        if (!existing.has(d)) {
          existing.add(d)
          merged.push(d)
        }
      }
    }
  }
  return merged
}

/**
 * Resolves the allowed domains for a given agent type, preferring template
 * overrides when the template uses restricted network mode.
 *
 * Returns undefined when no override applies (caller should use server defaults).
 */
export function resolveTemplateDomains(
  template: ProjectTemplate | null | undefined,
  agentType: AgentType,
): string[] | undefined {
  if (!template || template.networkMode !== 'restricted') return undefined
  return template.agents?.[agentType]?.allowedDomains
}

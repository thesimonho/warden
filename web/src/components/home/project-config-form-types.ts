import type { AgentType, Mount, NetworkMode } from '@/lib/types'
import type { DefaultMount } from '@/lib/api'

/** Identifies each step of the project config form. */
export type FormStep = 'general' | 'environment' | 'network' | 'advanced'

/** Ordered list of form steps for navigation. */
export const FORM_STEPS: FormStep[] = ['general', 'environment', 'network', 'advanced']

/** Human-readable labels for each form step. */
export const FORM_STEP_LABELS: Record<FormStep, string> = {
  general: 'General',
  environment: 'Environment',
  network: 'Network',
  advanced: 'Advanced',
}

/** Badge state for a form step tab. */
export type StepBadge = 'required' | 'configured' | 'empty'

/** A single key-value pair for environment variables. */
export interface EnvVarEntry {
  key: string
  value: string
}

/** Props for the ProjectConfigForm component. */
export interface ProjectConfigFormProps {
  /** Whether the form is for creating or editing a container. */
  mode: 'create' | 'edit'
  /** Initial values to populate the form (used in edit mode). */
  initialValues?: import('@/lib/types').ContainerConfig
  /** Called when the form is submitted with valid data. */
  onSubmit: (data: ProjectConfigFormData) => void
  /** Whether the form submission is in progress. */
  isSubmitting: boolean
  /** External error message to display. */
  error?: string | null
}

/** Data shape emitted by the form on submit. */
export interface ProjectConfigFormData {
  name: string
  image: string
  projectPath: string
  agentType: AgentType
  envVars?: Record<string, string>
  mounts?: Mount[]
  skipPermissions: boolean
  networkMode: NetworkMode
  allowedDomains?: string[]
  costBudget?: number
  enabledAccessItems?: string[]
  enabledRuntimes?: string[]
  forwardedPorts?: number[]
}

/** Default container image for new projects. */
export const DEFAULT_IMAGE = 'ghcr.io/thesimonho/warden:latest'

/** Returns true if a default mount belongs to the given agent type. */
export function isMountForAgent(m: DefaultMount, type: AgentType): boolean {
  if (m.agentType) return m.agentType === type
  return true // non-agent mount, always include
}

/** Returns the required default mount for the given agent type, if any. */
export function findRequiredMount(
  defaults: DefaultMount[],
  type: AgentType,
): DefaultMount | undefined {
  return defaults.find((m) => m.required && isMountForAgent(m, type))
}

/**
 * Returns a mount list with the required agent config mount prepended if missing.
 * Returns the input array unchanged (same reference) when already present.
 */
export function withRequiredMount(
  currentMounts: Mount[],
  defaults: DefaultMount[],
  type: AgentType,
): Mount[] {
  const required = findRequiredMount(defaults, type)
  if (!required) return currentMounts
  const hasIt = currentMounts.some((m) => m.containerPath === required.containerPath)
  if (hasIt) return currentMounts
  return [
    {
      hostPath: required.hostPath,
      containerPath: required.containerPath,
      readOnly: required.readOnly,
    },
    ...currentMounts,
  ]
}

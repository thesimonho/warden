import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Info } from 'lucide-react'
import { toast } from 'sonner'
import type {
  AccessItemResponse,
  AgentType,
  Mount,
  NetworkMode,
  ProjectTemplate,
  RuntimeDefault,
} from '@/lib/types'
import type { DefaultMount } from '@/lib/api'
import { agentTypeLabels, agentTypeOptions, DEFAULT_AGENT_TYPE } from '@/lib/types'
import { fetchAccessItems, fetchDefaults, fetchSettings, validateProjectTemplate } from '@/lib/api'
import {
  resolveRuntimeToggles,
  resolveRuntimeEnvVars,
  resolveTemplateDomains,
  mergeRuntimeDomains,
} from '@/lib/template'
import { containerPathToDisplay, containerPathToAbsolute } from '@/lib/utils'
import { getRestrictedDomains } from '@/lib/domain-groups'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { AgentIcon } from '@/components/ui/agent-icons'
import DirectoryBrowser from '@/components/ui/directory-browser'
import type {
  EnvVarEntry,
  FormStep,
  ProjectConfigFormProps,
  StepBadge,
} from './project-config-form-types'
import {
  DEFAULT_IMAGE,
  FORM_STEPS,
  isMountForAgent,
  findRequiredMount,
  withRequiredMount,
} from './project-config-form-types'
import {
  FormField,
  BindMountsField,
  EnvVarsField,
  ForwardedPortsField,
} from './project-config-form-fields'
import { StepTabs, StepFooter } from './project-config-form-steps'

export type { ProjectConfigFormData } from './project-config-form-types'

/**
 * Reusable form for creating or editing a project container.
 *
 * In edit mode, name and project path are read-only since they can't be
 * changed after creation. The submit button text adapts to the mode.
 *
 * @param props.mode - Whether creating or editing.
 * @param props.initialValues - Pre-populated values for edit mode.
 * @param props.onSubmit - Handler for form submission.
 * @param props.isSubmitting - Whether a submission is in progress.
 */
export default function ProjectConfigForm({
  mode,
  initialValues,
  onSubmit,
  isSubmitting,
  error: externalError,
}: ProjectConfigFormProps) {
  const [agentType, setAgentType] = useState<AgentType>(
    initialValues?.agentType ?? DEFAULT_AGENT_TYPE,
  )
  const [name, setName] = useState(initialValues?.name ?? '')
  const [projectPath, setProjectPath] = useState(initialValues?.projectPath ?? '')
  const [image, setImage] = useState(initialValues?.image || DEFAULT_IMAGE)
  const [mounts, setMounts] = useState<Mount[]>(initialValues?.mounts ?? [])
  const [skipPermissions, setSkipPermissions] = useState(initialValues?.skipPermissions ?? false)
  const [networkMode, setNetworkMode] = useState<NetworkMode>(
    initialValues?.networkMode ?? 'restricted',
  )
  const [allowedDomains, setAllowedDomains] = useState(
    () => initialValues?.allowedDomains?.join('\n') ?? '',
  )
  const [envVars, setEnvVars] = useState<EnvVarEntry[]>(() => {
    if (!initialValues?.envVars) return []
    return Object.entries(initialValues.envVars).map(([key, value]) => ({
      key,
      value,
    }))
  })
  const [costBudget, setCostBudget] = useState(
    initialValues?.costBudget && initialValues.costBudget > 0
      ? String(initialValues.costBudget)
      : '',
  )
  const [forwardedPorts, setForwardedPorts] = useState<number[]>(
    initialValues?.forwardedPorts ?? [],
  )
  const [accessItems, setAccessItems] = useState<AccessItemResponse[]>([])
  const [accessToggles, setAccessToggles] = useState<Record<string, boolean>>({})
  const [runtimeDefaults, setRuntimeDefaults] = useState<RuntimeDefault[]>([])
  const [runtimeToggles, setRuntimeToggles] = useState<Record<string, boolean>>({})
  const [error, setError] = useState<string | null>(null)
  const [currentStep, setCurrentStep] = useState<FormStep>('general')
  const [homeDir, setHomeDir] = useState('')
  const [containerHomeDir, setContainerHomeDir] = useState('')
  const [requiredContainerPath, setRequiredContainerPath] = useState<string | null>(null)
  const importInputRef = useRef<HTMLInputElement>(null)
  const defaultsLoaded = useRef(false)
  const defaultMountsRef = useRef<DefaultMount[]>([])
  const restrictedDomainsRef = useRef<Record<string, string[]>>({})
  const templateRef = useRef<ProjectTemplate | null>(null)

  /**
   * Applies a project template to the form state.
   * Only used in create mode — edit mode uses initialValues from the DB.
   */
  const applyTemplate = useCallback(
    (
      tmpl: ProjectTemplate,
      currentAgentType: AgentType,
      runtimes: RuntimeDefault[] = runtimeDefaults,
      currentToggles?: Record<string, boolean>,
    ) => {
      templateRef.current = tmpl
      if (tmpl.image) setImage(tmpl.image)
      if (tmpl.skipPermissions !== undefined) setSkipPermissions(tmpl.skipPermissions)
      if (tmpl.networkMode) setNetworkMode(tmpl.networkMode)
      if (tmpl.costBudget !== undefined && tmpl.costBudget > 0) {
        setCostBudget(String(tmpl.costBudget))
      }

      if (tmpl.forwardedPorts) setForwardedPorts(tmpl.forwardedPorts)

      // Apply runtime toggles — template runtimes override detection.
      // Also sync env vars since they're derived from which runtimes are enabled.
      let toggles = currentToggles
      if (tmpl.runtimes) {
        toggles = resolveRuntimeToggles(runtimes, tmpl)
        setRuntimeToggles(toggles)
        setEnvVars(resolveRuntimeEnvVars(runtimes, toggles))
      }

      // Apply agent-specific domains with runtime domains merged in.
      // If the template has agent-specific domain overrides, use those as
      // the base. Otherwise fall back to server defaults so that runtime
      // domains (e.g. Go's proxy.golang.org) still get merged in.
      const templateDomains = resolveTemplateDomains(tmpl, currentAgentType)
      if (templateDomains && toggles) {
        setAllowedDomains(mergeRuntimeDomains(templateDomains, runtimes, toggles).join('\n'))
      } else if (templateDomains) {
        setAllowedDomains(templateDomains.join('\n'))
      } else if (toggles && tmpl.runtimes) {
        const serverDomains = [
          ...getRestrictedDomains(restrictedDomainsRef.current, currentAgentType),
        ]
        setAllowedDomains(mergeRuntimeDomains(serverDomains, runtimes, toggles).join('\n'))
      }
    },
    [runtimeDefaults],
  )

  /** Fetches server-resolved defaults and access items on first render. */
  useEffect(() => {
    if (defaultsLoaded.current) return
    defaultsLoaded.current = true

    fetchDefaults(initialValues?.projectPath || projectPath || undefined)
      .then((defaults) => {
        if (defaults.homeDir) {
          setHomeDir(defaults.homeDir)
        }
        if (defaults.containerHomeDir) {
          setContainerHomeDir(defaults.containerHomeDir)
        }
        if (defaults.mounts?.length > 0) {
          defaultMountsRef.current = defaults.mounts
          const req = findRequiredMount(defaults.mounts, agentType)
          setRequiredContainerPath(req?.containerPath ?? null)
        }
        if (defaults.restrictedDomains) {
          restrictedDomainsRef.current = defaults.restrictedDomains
        }
        const runtimes = defaults.runtimes ?? []
        if (runtimes.length > 0) {
          setRuntimeDefaults(runtimes)
        }

        // Compute runtime toggles for both create and edit mode.
        // In create mode these feed into domain and env var resolution below.
        let rToggles: Record<string, boolean> = {}
        if (mode === 'create') {
          rToggles = resolveRuntimeToggles(runtimes)
          setRuntimeToggles(rToggles)
          setEnvVars(resolveRuntimeEnvVars(runtimes, rToggles))
        } else if (runtimes.length > 0) {
          const enabled = new Set(initialValues?.enabledRuntimes ?? [])
          for (const r of runtimes) {
            rToggles[r.id] = r.alwaysEnabled || enabled.has(r.id)
          }
          setRuntimeToggles(rToggles)
          // Re-populate runtime env vars (stripped at save time, not in DB).
          const runtimeEnvs = resolveRuntimeEnvVars(runtimes, rToggles)
          if (runtimeEnvs.length > 0) {
            setEnvVars((prev) => {
              const existing = new Set(prev.map((e) => e.key))
              const toAdd = runtimeEnvs.filter((e) => !existing.has(e.key))
              return [...prev, ...toAdd]
            })
          }
        }

        if (mode === 'create') {
          if (defaults.mounts?.length > 0) {
            setMounts(defaults.mounts.filter((m) => isMountForAgent(m, agentType)))
          }
          if (defaults.restrictedDomains && !initialValues?.allowedDomains) {
            const baseDomains = [...getRestrictedDomains(defaults.restrictedDomains, agentType)]
            setAllowedDomains(mergeRuntimeDomains(baseDomains, runtimes, rToggles).join('\n'))
          }
          // Apply template after setting defaults so template overrides take effect.
          if (defaults.template) {
            applyTemplate(defaults.template, agentType, runtimes, rToggles)
          }
        } else if (defaults.mounts?.length > 0) {
          // Ensure the required agent config mount is present (covers
          // projects created before this mount was mandatory).
          setMounts((prev) => withRequiredMount(prev, defaults.mounts, agentType))
        }
      })
      .catch(() => {})

    fetchAccessItems()
      .then((items) => {
        setAccessItems(items)

        if (mode === 'create') {
          // Enable all detected access items by default.
          const toggles: Record<string, boolean> = {}
          for (const item of items) {
            toggles[item.id] = item.detection.available
          }
          setAccessToggles(toggles)
        } else {
          // Edit mode: read enabled access items from stored config.
          const enabled = new Set(initialValues?.enabledAccessItems ?? [])
          const toggles: Record<string, boolean> = {}
          for (const item of items) {
            toggles[item.id] = item.detection.available && enabled.has(item.id)
          }
          setAccessToggles(toggles)
        }
      })
      .catch(() => {})

    if (mode === 'create') {
      fetchSettings()
        .then((settings) => {
          if (settings.defaultProjectBudget > 0) {
            setCostBudget(String(settings.defaultProjectBudget))
          }
        })
        .catch(() => {})
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- initialValues is stable across renders; only mode determines create/edit behavior
  }, [mode])

  /** Re-fetches defaults when project path changes in create mode to pick up .warden.json. */
  useEffect(() => {
    if (mode !== 'create' || !defaultsLoaded.current || !projectPath) return

    let stale = false
    fetchDefaults(projectPath)
      .then((defaults) => {
        if (stale) return
        if (defaults.restrictedDomains) {
          restrictedDomainsRef.current = defaults.restrictedDomains
        }
        if (defaults.runtimes) {
          setRuntimeDefaults(defaults.runtimes)
        }
        const runtimes = defaults.runtimes ?? []
        if (defaults.template) {
          // Template handles its own toggle/envvar/domain resolution.
          applyTemplate(defaults.template, agentType, runtimes)
        } else {
          templateRef.current = null
          // No template — resolve from detection and merge runtime domains.
          const toggles = resolveRuntimeToggles(runtimes)
          setRuntimeToggles(toggles)
          setEnvVars(resolveRuntimeEnvVars(runtimes, toggles))
          if (defaults.restrictedDomains) {
            const baseDomains = [...getRestrictedDomains(defaults.restrictedDomains, agentType)]
            setAllowedDomains(mergeRuntimeDomains(baseDomains, runtimes, toggles).join('\n'))
          }
        }
      })
      .catch(() => {})
    return () => {
      stale = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-fetch when projectPath changes
  }, [projectPath])

  /** Updates agent type, re-filters default mounts, and updates allowed domains. */
  const handleAgentTypeChange = (newType: AgentType) => {
    setAgentType(newType)
    const req = findRequiredMount(defaultMountsRef.current, newType)
    setRequiredContainerPath(req?.containerPath ?? null)
    if (mode === 'create' && defaultMountsRef.current.length > 0) {
      setMounts(defaultMountsRef.current.filter((m) => isMountForAgent(m, newType)))
    }
    if (mode === 'create') {
      const templateDomains = resolveTemplateDomains(templateRef.current, newType)
      const baseDomains = templateDomains ?? [
        ...getRestrictedDomains(restrictedDomainsRef.current, newType),
      ]
      setAllowedDomains(
        mergeRuntimeDomains(baseDomains, runtimeDefaults, runtimeToggles).join('\n'),
      )
    }
  }

  /**
   * Toggles a runtime on/off and updates domains and env vars accordingly.
   * When enabled: adds the runtime's domains to the allowed list and its
   * env vars to the env var list (read-only). When disabled: removes them.
   */
  const handleRuntimeToggle = (runtimeId: string, enabled: boolean) => {
    const runtime = runtimeDefaults.find((r) => r.id === runtimeId)
    if (!runtime || runtime.alwaysEnabled) return

    setRuntimeToggles((prev) => ({ ...prev, [runtimeId]: enabled }))

    // Update allowed domains when in restricted mode.
    if (networkMode === 'restricted' && runtime.domains.length > 0) {
      setAllowedDomains((prev) => {
        const currentDomains = prev
          .split('\n')
          .map((d) => d.trim())
          .filter(Boolean)
        if (enabled) {
          const toAdd = runtime.domains.filter((d) => !currentDomains.includes(d))
          return [...currentDomains, ...toAdd].join('\n')
        }
        const domainSet = new Set(runtime.domains)
        return currentDomains.filter((d) => !domainSet.has(d)).join('\n')
      })
    }

    // Update env vars — add runtime env vars as read-only entries on enable,
    // remove them on disable.
    if (runtime.envVars && Object.keys(runtime.envVars).length > 0) {
      setEnvVars((prev) => {
        if (enabled) {
          const existing = new Set(prev.map((e) => e.key))
          const toAdd = Object.entries(runtime.envVars)
            .filter(([key]) => !existing.has(key))
            .map(([key, value]) => ({ key, value }))
          return [...prev, ...toAdd]
        }
        const runtimeKeys = new Set(Object.keys(runtime.envVars))
        return prev.filter((e) => !runtimeKeys.has(e.key))
      })
    }
  }

  /** Set of env var keys contributed by enabled runtimes (read-only in the form). */
  const runtimeEnvKeys = useMemo(() => {
    const keys = new Set<string>()
    for (const r of runtimeDefaults) {
      if (runtimeToggles[r.id] && r.envVars) {
        for (const key of Object.keys(r.envVars)) {
          keys.add(key)
        }
      }
    }
    return keys
  }, [runtimeDefaults, runtimeToggles])

  /** Adds a blank env var row. */
  const handleAddEnvVar = () => {
    setEnvVars((prev) => [...prev, { key: '', value: '' }])
  }

  /** Updates a single env var entry by index. */
  const handleUpdateEnvVar = (index: number, field: 'key' | 'value', newValue: string) => {
    setEnvVars((prev) =>
      prev.map((entry, i) => (i === index ? { ...entry, [field]: newValue } : entry)),
    )
  }

  /** Removes an env var row by index. */
  const handleRemoveEnvVar = (index: number) => {
    setEnvVars((prev) => prev.filter((_, i) => i !== index))
  }

  /** Validates form fields and returns an error message or null. */
  const validate = (): string | null => {
    if (!name.trim()) return 'Container name is required'
    if (!projectPath.trim()) return 'Project directory is required'
    if (!image.trim()) return 'Image is required'
    if (networkMode === 'restricted') {
      const hasDomains = allowedDomains.split('\n').some((d) => d.trim())
      if (!hasDomains) return 'Restricted mode requires at least one allowed domain'
    }
    return null
  }

  const handleSubmit = () => {
    const validationError = validate()
    if (validationError) {
      setError(validationError)
      return
    }

    setError(null)

    const envMap: Record<string, string> = {}
    for (const entry of envVars) {
      const trimmedKey = entry.key.trim()
      if (trimmedKey && entry.value) {
        envMap[trimmedKey] = entry.value
      }
    }

    const validMounts = mounts.filter((m) => m.hostPath.trim() && m.containerPath.trim())
    const parsedDomains =
      networkMode === 'restricted'
        ? allowedDomains
            .split('\n')
            .map((d) => d.trim())
            .filter(Boolean)
        : undefined
    const enabledIds = accessItems.filter((item) => accessToggles[item.id]).map((item) => item.id)
    const enabledRuntimeIds = runtimeDefaults.filter((r) => runtimeToggles[r.id]).map((r) => r.id)

    // Strip runtime-contributed env vars from the user env map — they're
    // managed by the container install script via WARDEN_ENABLED_RUNTIMES.
    for (const key of runtimeEnvKeys) {
      delete envMap[key]
    }

    onSubmit({
      name: name.trim(),
      image: image.trim(),
      projectPath: projectPath.trim(),
      agentType,
      envVars: Object.keys(envMap).length > 0 ? envMap : undefined,
      mounts: validMounts.length > 0 ? validMounts : undefined,
      skipPermissions,
      networkMode,
      allowedDomains: parsedDomains,
      costBudget: parseFloat(costBudget) || 0,
      enabledAccessItems: enabledIds.length > 0 ? enabledIds : undefined,
      enabledRuntimes: enabledRuntimeIds.length > 0 ? enabledRuntimeIds : undefined,
      forwardedPorts: forwardedPorts.length > 0 ? forwardedPorts : undefined,
    })
  }

  const containerToDisplay = useCallback(
    (path: string) => containerPathToDisplay(path, containerHomeDir),
    [containerHomeDir],
  )

  const containerToAbsolute = useCallback(
    (input: string) => containerPathToAbsolute(input, containerHomeDir),
    [containerHomeDir],
  )

  /** User-visible mounts with their original indices (for safe mutation of the mounts array). */
  const visibleMounts = useMemo(() => mounts.map((mount, index) => ({ mount, index })), [mounts])

  /** User-visible env vars with their original indices. */
  const visibleEnvVars = useMemo(() => envVars.map((entry, index) => ({ entry, index })), [envVars])

  const isEditMode = mode === 'edit'
  const isValid = useMemo(
    () => validate() === null,
    // eslint-disable-next-line react-hooks/exhaustive-deps -- validate is not memoized; listing its captured fields explicitly
    [name, projectPath, image, networkMode, allowedDomains],
  )
  const displayError = error ?? externalError

  /** Summary text and badge indicator for each step tab, derived from current form state. */
  const { stepSummaries, stepBadges } = useMemo(() => {
    // General
    const generalSetup = !name.trim() || !projectPath.trim()
    const generalSummary = generalSetup
      ? 'Setup required'
      : `${agentTypeLabels[agentType]}, ${name}`

    // Environment
    const enabledRuntimeCount = runtimeDefaults.filter((r) => runtimeToggles[r.id]).length
    const enabledAccessCount = accessItems.filter((item) => accessToggles[item.id]).length
    const envParts: string[] = []
    if (runtimeDefaults.length === 0) {
      envParts.push('Detecting...')
    } else {
      envParts.push(`${enabledRuntimeCount} runtime${enabledRuntimeCount !== 1 ? 's' : ''}`)
    }
    if (enabledAccessCount > 0) {
      envParts.push(`${enabledAccessCount} access`)
    }
    const environmentSummary = envParts.join(', ')
    const environmentConfigured = runtimeDefaults.length > 0 || accessItems.length > 0

    // Network
    const portsSuffix =
      forwardedPorts.length > 0
        ? `, ${forwardedPorts.length} port${forwardedPorts.length !== 1 ? 's' : ''}`
        : ''
    const networkSummary =
      (networkMode === 'full'
        ? 'Full access'
        : networkMode === 'restricted'
          ? 'Restricted'
          : 'No network') + portsSuffix

    // Advanced
    const hasCustomImage = image.trim() !== DEFAULT_IMAGE
    const hasUserMounts = mounts.some((m) => m.containerPath !== requiredContainerPath)
    const hasUserEnvVars = envVars.some((e) => e.key.trim() && !runtimeEnvKeys.has(e.key))
    const advancedConfigured = hasCustomImage || hasUserMounts || hasUserEnvVars
    const advancedParts: string[] = []
    if (hasCustomImage) advancedParts.push('Custom image')
    if (hasUserMounts) advancedParts.push('Mounts')
    if (hasUserEnvVars) advancedParts.push('Env vars')
    const advancedSummary = advancedParts.length > 0 ? advancedParts.join(', ') : 'Defaults applied'

    const summaries: Record<FormStep, string> = {
      general: generalSummary,
      environment: environmentSummary,
      network: networkSummary,
      advanced: advancedSummary,
    }

    const badges: Record<FormStep, StepBadge> = {
      general: generalSetup ? 'required' : 'configured',
      environment: environmentConfigured ? 'configured' : 'empty',
      network: 'configured',
      advanced: advancedConfigured ? 'configured' : 'empty',
    }

    return { stepSummaries: summaries, stepBadges: badges }
  }, [
    name,
    projectPath,
    agentType,
    runtimeDefaults,
    runtimeToggles,
    accessItems,
    accessToggles,
    networkMode,
    image,
    mounts,
    requiredContainerPath,
    envVars,
    runtimeEnvKeys,
    forwardedPorts,
  ])

  /** Navigate to the previous step. */
  const handleStepBack = () => {
    const idx = FORM_STEPS.indexOf(currentStep)
    if (idx > 0) setCurrentStep(FORM_STEPS[idx - 1])
  }

  /** Navigate to the next step. */
  const handleStepNext = () => {
    const idx = FORM_STEPS.indexOf(currentStep)
    if (idx < FORM_STEPS.length - 1) setCurrentStep(FORM_STEPS[idx + 1])
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <StepTabs
        currentStep={currentStep}
        onStepChange={setCurrentStep}
        summaries={stepSummaries}
        badges={stepBadges}
      />

      {/* Scrollable step content */}
      <div className="min-h-0 flex-1 overflow-y-auto px-2 py-6">
        {currentStep === 'general' && (
          <div className="space-y-8">
            <FormField label="Agent" required>
              <Select
                value={agentType}
                onValueChange={(val) => handleAgentTypeChange(val as AgentType)}
                disabled={isSubmitting || isEditMode}
              >
                <SelectTrigger className="w-full">
                  <SelectValue>
                    <span className="flex items-center gap-2">
                      <AgentIcon type={agentType} className="h-4 w-4 shrink-0" />
                      {agentTypeLabels[agentType]}
                    </span>
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {agentTypeOptions.map((type) => (
                    <SelectItem key={type} value={type}>
                      <span className="flex items-center gap-2">
                        <AgentIcon type={type} className="h-4 w-4 shrink-0" />
                        {agentTypeLabels[type]}
                      </span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {isEditMode && (
                <p className="text-muted-foreground text-sm">
                  Agent type cannot be changed after creation.
                </p>
              )}
            </FormField>

            <FormField label="Container Name" required>
              <Input
                placeholder="my-project"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={isSubmitting || isEditMode}
              />
              {isEditMode && (
                <p className="text-muted-foreground text-sm">
                  Name and project directory cannot be changed after creation.
                </p>
              )}
            </FormField>

            <FormField
              label="Project Directory"
              description="Host directory to bind-mount into the container."
              required
            >
              <DirectoryBrowser
                value={projectPath}
                onChange={(newPath) => {
                  if (newPath === projectPath) return
                  setProjectPath(newPath)
                }}
                disabled={isSubmitting || isEditMode}
                defaultPath={homeDir}
              />
            </FormField>

            <FormField
              label="Project Budget"
              description="Auto-pauses agents when exceeded. Leave empty for unlimited."
            >
              <div className="flex items-center gap-2">
                <span className="text-muted-foreground text-sm">$</span>
                <Input
                  type="number"
                  min={0}
                  step={0.01}
                  placeholder="Use default"
                  value={costBudget}
                  onChange={(e) => setCostBudget(e.target.value)}
                  disabled={isSubmitting}
                  className="w-32"
                />
              </div>
            </FormField>

            <div className="flex items-center justify-between rounded border p-3">
              <div className="space-y-0.5">
                <label htmlFor="skip-permissions-toggle" className="font-medium">
                  Skip permission prompts
                </label>
                <p className="text-muted-foreground text-sm">
                  Auto-approve all actions (
                  <code>
                    {agentType === 'codex'
                      ? '--dangerously-bypass-approvals-and-sandbox'
                      : '--dangerously-skip-permissions'}
                  </code>
                  ).
                </p>
              </div>
              <Switch
                id="skip-permissions-toggle"
                checked={skipPermissions}
                onCheckedChange={setSkipPermissions}
                disabled={isSubmitting}
              />
            </div>
          </div>
        )}

        {currentStep === 'environment' && (
          <div className="space-y-8">
            <div className="space-y-2">
              <label className="font-medium">Runtimes</label>
              <p className="text-muted-foreground text-sm">
                Language runtimes to install in the container.
              </p>
              {runtimeDefaults.map((runtime) => (
                <label key={runtime.id} className="flex items-start gap-2 py-1">
                  <Checkbox
                    checked={runtimeToggles[runtime.id] ?? false}
                    onCheckedChange={(checked) => handleRuntimeToggle(runtime.id, checked === true)}
                    disabled={isSubmitting || runtime.alwaysEnabled}
                    className="mt-0.5"
                  />
                  <div className="flex flex-col gap-0.5">
                    <span className="text-sm">
                      {runtime.label}
                      {runtime.alwaysEnabled && (
                        <span className="text-muted-foreground ml-1 text-xs">(required)</span>
                      )}
                      {!runtime.alwaysEnabled && runtime.detected && (
                        <span className="text-muted-foreground ml-1 text-xs">(detected)</span>
                      )}
                    </span>
                    <span className="text-muted-foreground text-xs">{runtime.description}</span>
                  </div>
                </label>
              ))}
            </div>

            <div className="space-y-2">
              <label className="font-medium">Access</label>
              <p className="text-muted-foreground text-sm">
                Passthrough access items to containers.
              </p>
              {accessItems.length === 0 && (
                <p className="text-muted-foreground text-sm">No access items configured.</p>
              )}
              {accessItems.map((item) => {
                const isDetected = item.detection.available
                return (
                  <label key={item.id} className="flex items-start gap-2 py-1">
                    <Checkbox
                      checked={accessToggles[item.id] ?? false}
                      onCheckedChange={(checked) =>
                        setAccessToggles((prev) => ({ ...prev, [item.id]: checked === true }))
                      }
                      disabled={isSubmitting || !isDetected}
                      className="mt-0.5"
                    />
                    <div className="flex flex-col gap-0.5">
                      <span className="flex items-center gap-1.5 text-sm">
                        <span
                          className={`inline-block h-2 w-2 shrink-0 rounded-full ${isDetected ? 'bg-success' : 'bg-muted-foreground/40'}`}
                        />
                        <span className={isDetected ? '' : 'text-muted-foreground'}>
                          {item.label}
                          {!isDetected && ' (unavailable)'}
                        </span>
                      </span>
                      {item.description && (
                        <span className="text-muted-foreground text-xs">{item.description}</span>
                      )}
                    </div>
                  </label>
                )
              })}
            </div>
          </div>
        )}

        {currentStep === 'network' && (
          <div className="space-y-2">
            <div className="space-y-0.5">
              <label className="font-medium">
                <span className="flex items-center gap-1.5">
                  Network
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Info className="text-muted-foreground h-3.5 w-3.5" />
                    </TooltipTrigger>
                    <TooltipContent side="right" className="max-w-64">
                      <p>
                        Controls outbound network access from the container. Restricted allows only
                        specified domains. None blocks all outbound traffic.
                      </p>
                    </TooltipContent>
                  </Tooltip>
                </span>
              </label>
            </div>
            <div className="flex gap-2">
              {(['full', 'restricted', 'none'] as const).map((m) => (
                <Button
                  key={m}
                  type="button"
                  size="sm"
                  variant={networkMode === m ? 'secondary' : 'outline'}
                  onClick={() => setNetworkMode(m)}
                  disabled={isSubmitting}
                  className="flex-1 capitalize"
                >
                  {m}
                </Button>
              ))}
            </div>
            {networkMode === 'full' && (
              <p className="text-muted-foreground text-sm">
                Unrestricted outbound access. The container can reach any host on the internet.
              </p>
            )}
            {networkMode === 'restricted' && (
              <div className="space-y-1.5">
                <label className="text-muted-foreground text-sm">
                  Allowed domains (one per line)
                </label>
                <Textarea
                  value={allowedDomains}
                  onChange={(e) => setAllowedDomains(e.target.value)}
                  disabled={isSubmitting}
                  rows={9}
                  className="font-mono"
                  placeholder="*.anthropic.com&#10;*.github.com&#10;registry.npmjs.org"
                />
                <p className="text-muted-foreground text-sm">
                  Wildcard patterns (e.g. *.github.com) match all subdomains dynamically.
                </p>
              </div>
            )}
            {networkMode === 'none' && (
              <p className="text-muted-foreground text-sm">
                All outbound traffic will be blocked. Package installs and API calls will not work.
              </p>
            )}

            <div className="mt-6 space-y-2 border-t pt-6">
              <label className="block font-medium">
                <span className="flex items-center gap-1.5">
                  Forwarded Ports
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Info className="text-muted-foreground h-3.5 w-3.5" />
                    </TooltipTrigger>
                    <TooltipContent side="right" className="max-w-64">
                      <p>
                        Container ports exposed via reverse proxy. Use for dev servers (Vite,
                        Next.js, etc.) — supports HTTP and WebSocket for HMR.
                      </p>
                    </TooltipContent>
                  </Tooltip>
                </span>
              </label>
              <ForwardedPortsField
                ports={forwardedPorts}
                isSubmitting={isSubmitting}
                onAdd={(port) => setForwardedPorts((prev) => [...prev, port])}
                onRemove={(index) =>
                  setForwardedPorts((prev) => prev.filter((_, i) => i !== index))
                }
              />
            </div>
          </div>
        )}

        {currentStep === 'advanced' && (
          <div className="space-y-8">
            <FormField
              label={
                <span className="flex items-center gap-1.5">
                  Image
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Info className="text-muted-foreground h-3.5 w-3.5" />
                    </TooltipTrigger>
                    <TooltipContent side="right" className="max-w-64">
                      <p>
                        Specify a custom image if you&apos;ve built one with project-specific
                        dependencies (e.g. FROM ghcr.io/thesimonho/warden).
                      </p>
                    </TooltipContent>
                  </Tooltip>
                </span>
              }
            >
              <Input
                value={image}
                onChange={(e) => setImage(e.target.value)}
                disabled={isSubmitting}
              />
            </FormField>

            <BindMountsField
              visibleMounts={visibleMounts}
              containerHomeDir={containerHomeDir}
              homeDir={homeDir}
              requiredContainerPath={requiredContainerPath}
              isSubmitting={isSubmitting}
              containerToDisplay={containerToDisplay}
              containerToAbsolute={containerToAbsolute}
              onMountsChange={setMounts}
            />

            <EnvVarsField
              visibleEnvVars={visibleEnvVars}
              runtimeEnvKeys={runtimeEnvKeys}
              isSubmitting={isSubmitting}
              onAdd={handleAddEnvVar}
              onUpdate={handleUpdateEnvVar}
              onRemove={handleRemoveEnvVar}
            />
          </div>
        )}
      </div>

      {/* Hidden file input for template import — outside step content so ref persists */}
      {mode === 'create' && (
        <input
          ref={importInputRef}
          type="file"
          accept=".json,application/json"
          className="hidden"
          onChange={(e) => {
            const file = e.target.files?.[0]
            if (!file) return
            file
              .text()
              .then((text) => validateProjectTemplate(text))
              .then((tmpl) => {
                applyTemplate(tmpl, agentType, runtimeDefaults, runtimeToggles)
                toast.success('Template imported')
              })
              .catch((err: unknown) => {
                const message = err instanceof Error ? err.message : 'Failed to import template'
                toast.error(message)
              })
            // Reset so the same file can be re-selected
            e.target.value = ''
          }}
        />
      )}

      <StepFooter
        currentStep={currentStep}
        onBack={handleStepBack}
        onNext={handleStepNext}
        onSubmit={handleSubmit}
        isValid={isValid}
        isSubmitting={isSubmitting}
        mode={mode}
        onImport={mode === 'create' ? () => importInputRef.current?.click() : undefined}
        error={displayError}
      />
    </div>
  )
}

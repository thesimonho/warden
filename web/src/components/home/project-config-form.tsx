import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ArrowRight, Info, Loader2, Plus, Trash2 } from 'lucide-react'
import type { ContainerConfig, Mount, NetworkMode } from '@/lib/types'
import { fetchDefaults, fetchSettings } from '@/lib/api'
import type { MountPreset } from '@/lib/api'
import { restrictedDomains } from '@/lib/domain-groups'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import DirectoryBrowser from '@/components/home/directory-browser'

/** A single key-value pair for environment variables. */
interface EnvVarEntry {
  key: string
  value: string
}

/** Props for the ProjectConfigForm component. */
interface ProjectConfigFormProps {
  /** Whether the form is for creating or editing a container. */
  mode: 'create' | 'edit'
  /** Initial values to populate the form (used in edit mode). */
  initialValues?: ContainerConfig
  /** Called when the form is submitted with valid data. */
  onSubmit: (data: ProjectConfigFormData) => void
  /** Whether the form submission is in progress. */
  isSubmitting: boolean
  /** Whether the form is disabled (e.g. container is running). */
  disabled?: boolean
  /** External error message to display. */
  error?: string | null
}

/** Data shape emitted by the form on submit. */
export interface ProjectConfigFormData {
  name: string
  image: string
  projectPath: string
  envVars?: Record<string, string>
  mounts?: Mount[]
  skipPermissions: boolean
  networkMode: NetworkMode
  allowedDomains?: string[]
  costBudget?: number
  enabledPresets?: string[]
}

/** Default container image for new projects. */
const DEFAULT_IMAGE = 'ghcr.io/thesimonho/warden:latest'

/** Default allowed domains for restricted network mode. */
const DEFAULT_ALLOWED_DOMAINS = restrictedDomains.join('\n')

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
  disabled = false,
  error: externalError,
}: ProjectConfigFormProps) {
  const [name, setName] = useState(initialValues?.name ?? '')
  const [projectPath, setProjectPath] = useState(initialValues?.projectPath ?? '')
  const [image, setImage] = useState(initialValues?.image ?? DEFAULT_IMAGE)
  const [mounts, setMounts] = useState<Mount[]>(initialValues?.mounts ?? [])
  const [skipPermissions, setSkipPermissions] = useState(initialValues?.skipPermissions ?? false)
  const [networkMode, setNetworkMode] = useState<NetworkMode>(
    initialValues?.networkMode ?? 'restricted',
  )
  const [allowedDomains, setAllowedDomains] = useState(
    () => initialValues?.allowedDomains?.join('\n') ?? DEFAULT_ALLOWED_DOMAINS,
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
  const [presets, setPresets] = useState<MountPreset[]>([])
  const [presetToggles, setPresetToggles] = useState<Record<string, boolean>>({})
  const [error, setError] = useState<string | null>(null)
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [homeDir, setHomeDir] = useState('')
  const [containerHomeDir, setContainerHomeDir] = useState('')
  const defaultsLoaded = useRef(false)

  /** Fetches server-resolved defaults on first render. */
  useEffect(() => {
    if (defaultsLoaded.current) return
    defaultsLoaded.current = true

    fetchDefaults()
      .then((defaults) => {
        if (defaults.homeDir) {
          setHomeDir(defaults.homeDir)
        }
        if (defaults.containerHomeDir) {
          setContainerHomeDir(defaults.containerHomeDir)
        }
        if (defaults.presets?.length) {
          setPresets(defaults.presets)

          if (mode === 'create') {
            // Enable all available presets by default.
            const toggles: Record<string, boolean> = {}
            for (const preset of defaults.presets) {
              toggles[preset.id] = preset.available
            }
            setPresetToggles(toggles)
          } else {
            // Edit mode: read enabled presets from stored config.
            const enabled = new Set(initialValues?.enabledPresets ?? [])
            const toggles: Record<string, boolean> = {}
            for (const preset of defaults.presets) {
              toggles[preset.id] = preset.available && enabled.has(preset.id)
            }
            setPresetToggles(toggles)
          }
        }
        if (mode === 'create') {
          if (defaults.mounts?.length > 0) {
            setMounts(defaults.mounts)
          }
          if (defaults.envVars?.length) {
            setEnvVars(defaults.envVars)
          }
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

    // Merge enabled preset mounts/envVars into the flat lists.
    const allMounts = [...mounts.filter((m) => !presetContainerPaths.has(m.containerPath))]
    for (const preset of presets) {
      if (presetToggles[preset.id]) {
        for (const pm of preset.mounts) {
          allMounts.push({
            hostPath: pm.hostPath,
            containerPath: pm.containerPath,
            readOnly: pm.readOnly,
          })
        }
        if (preset.envVars) {
          for (const ev of preset.envVars) {
            const trimmedKey = ev.key.trim()
            if (trimmedKey && !envMap[trimmedKey]) {
              envMap[trimmedKey] = ev.value
            }
          }
        }
      }
    }

    const validMounts = allMounts.filter((m) => m.hostPath.trim() && m.containerPath.trim())
    const parsedDomains =
      networkMode === 'restricted'
        ? allowedDomains
            .split('\n')
            .map((d) => d.trim())
            .filter(Boolean)
        : undefined
    const enabledPresetIds = presets.filter((p) => presetToggles[p.id]).map((p) => p.id)

    onSubmit({
      name: name.trim(),
      image: image.trim(),
      projectPath: projectPath.trim(),
      envVars: Object.keys(envMap).length > 0 ? envMap : undefined,
      mounts: validMounts.length > 0 ? validMounts : undefined,
      skipPermissions,
      networkMode,
      allowedDomains: parsedDomains,
      costBudget: parseFloat(costBudget) || 0,
      enabledPresets: enabledPresetIds.length > 0 ? enabledPresetIds : undefined,
    })
  }

  /** Replaces the container home directory prefix with ~ for display. */
  const containerToDisplay = useCallback(
    (path: string) => {
      if (!containerHomeDir || !path) return path
      if (path === containerHomeDir) return '~'
      if (path.startsWith(containerHomeDir + '/')) return '~' + path.slice(containerHomeDir.length)
      return path
    },
    [containerHomeDir],
  )

  /** Expands a leading ~ to the container home directory. */
  const containerToAbsolute = useCallback(
    (input: string) => {
      if (!containerHomeDir || !input.startsWith('~')) return input
      if (input === '~') return containerHomeDir
      if (input.startsWith('~/')) return containerHomeDir + input.slice(1)
      return input
    },
    [containerHomeDir],
  )

  /** Container paths that belong to any preset (used to filter the bind mounts grid). */
  const presetContainerPaths = useMemo(() => {
    const paths = new Set<string>()
    for (const preset of presets) {
      for (const m of preset.mounts) {
        paths.add(m.containerPath)
      }
    }
    return paths
  }, [presets])

  /** User-visible mounts with their original indices (for safe mutation of the mounts array). */
  const visibleMounts = useMemo(
    () =>
      mounts
        .map((mount, index) => ({ mount, index }))
        .filter(({ mount }) => !presetContainerPaths.has(mount.containerPath)),
    [mounts, presetContainerPaths],
  )

  /** Env var keys that belong to any preset. */
  const presetEnvKeys = useMemo(() => {
    const keys = new Set<string>()
    for (const preset of presets) {
      for (const ev of preset.envVars ?? []) {
        keys.add(ev.key)
      }
    }
    return keys
  }, [presets])

  /** User-visible env vars with their original indices. */
  const visibleEnvVars = useMemo(
    () =>
      envVars
        .map((entry, index) => ({ entry, index }))
        .filter(({ entry }) => !presetEnvKeys.has(entry.key)),
    [envVars, presetEnvKeys],
  )

  const isEditMode = mode === 'edit'
  const isValid = useMemo(
    () => validate() === null,
    // eslint-disable-next-line react-hooks/exhaustive-deps -- validate is not memoized; listing its captured fields explicitly
    [name, projectPath, image, networkMode, allowedDomains],
  )
  const displayError = error ?? externalError

  return (
    <div className="space-y-8">
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

      <div className="flex items-center justify-between rounded border p-3">
        <div className="space-y-0.5">
          <label htmlFor="skip-permissions-toggle" className="font-medium">
            Skip permission prompts
          </label>
          <p className="text-muted-foreground text-sm">
            Auto-approve all actions (<code>--dangerously-skip-permissions</code>).
          </p>
        </div>
        <Switch
          id="skip-permissions-toggle"
          checked={skipPermissions}
          onCheckedChange={setSkipPermissions}
          disabled={isSubmitting || disabled}
        />
      </div>

      <FormField
        label="Project budget"
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
            disabled={isSubmitting || disabled}
            className="w-32"
          />
        </div>
      </FormField>

      <div className="space-y-2">
        <div className="space-y-0.5">
          <label className="font-medium">
            <span className="flex items-center gap-1.5">
              Network Access
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="text-muted-foreground h-3.5 w-3.5" />
                </TooltipTrigger>
                <TooltipContent side="right" className="max-w-64">
                  <p>
                    Controls outbound network access from the container. Restricted uses iptables to
                    allow only specified domains. None blocks all outbound traffic. Domain IPs are
                    resolved at container start — restart to pick up DNS changes. Requires container
                    recreate to change.
                  </p>
                </TooltipContent>
              </Tooltip>
            </span>
          </label>
        </div>
        <div className="flex gap-2">
          {(['full', 'restricted', 'none'] as const).map((mode) => (
            <Button
              key={mode}
              type="button"
              size="sm"
              variant={networkMode === mode ? 'default' : 'outline'}
              onClick={() => setNetworkMode(mode)}
              disabled={isSubmitting || disabled}
              className="flex-1 capitalize"
            >
              {mode === 'full' ? 'Full' : mode === 'restricted' ? 'Restricted' : 'None'}
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
            <label className="text-muted-foreground text-sm">Allowed domains (one per line)</label>
            <Textarea
              value={allowedDomains}
              onChange={(e) => setAllowedDomains(e.target.value)}
              disabled={isSubmitting || disabled}
              rows={9}
              className="font-mono"
              placeholder="*.anthropic.com&#10;*.github.com&#10;registry.npmjs.org"
            />
            <p className="text-muted-foreground text-sm">
              Wildcard patterns (e.g. *.github.com) resolve the base domain at container start.
            </p>
          </div>
        )}
        {networkMode === 'none' && (
          <p className="text-muted-foreground text-sm">
            All outbound traffic will be blocked. Package installs and API calls will not work.
          </p>
        )}
      </div>

      <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
        <CollapsibleTrigger asChild>
          <Button type="button" variant="ghost" size="sm" className="w-full justify-between">
            Advanced
            <svg
              className={`h-4 w-4 transition-transform ${advancedOpen ? 'rotate-180' : ''}`}
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="m6 9 6 6 6-6" />
            </svg>
          </Button>
        </CollapsibleTrigger>
        <CollapsibleContent className="space-y-4 pt-2">
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
              disabled={isSubmitting || disabled}
            />
          </FormField>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="font-medium">
                <span className="flex items-center gap-1.5">
                  Bind Mounts
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Info className="text-muted-foreground h-3.5 w-3.5" />
                    </TooltipTrigger>
                    <TooltipContent side="right" className="max-w-64">
                      <p>
                        Mount host directories into the container. Toggle passthrough presets below
                        to include common infrastructure mounts automatically.
                      </p>
                    </TooltipContent>
                  </Tooltip>
                </span>
              </label>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={() =>
                  setMounts((prev) => [
                    ...prev,
                    { hostPath: '', containerPath: '', readOnly: true },
                  ])
                }
                disabled={isSubmitting || disabled}
                icon={Plus}
              >
                Add
              </Button>
            </div>

            {presets.length > 0 && (
              <div className="flex items-center gap-3">
                <span className="text-muted-foreground text-sm">Passthrough:</span>
                {presets.map((preset) => (
                  <label key={preset.id} className="flex items-center gap-1.5 text-sm">
                    <Checkbox
                      checked={presetToggles[preset.id] ?? false}
                      onCheckedChange={(checked) =>
                        setPresetToggles((prev) => ({ ...prev, [preset.id]: checked === true }))
                      }
                      disabled={isSubmitting || disabled || !preset.available}
                    />
                    <span className={preset.available ? '' : 'text-muted-foreground'}>
                      {preset.label}
                    </span>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Info className="text-muted-foreground h-3 w-3" />
                      </TooltipTrigger>
                      <TooltipContent side="top" className="max-w-72">
                        <p>
                          {preset.description}
                          {!preset.available && ' (not detected on host)'}
                        </p>
                      </TooltipContent>
                    </Tooltip>
                  </label>
                ))}
              </div>
            )}

            {visibleMounts.length === 0 && (
              <p className="text-muted-foreground text-sm">No additional bind mounts configured.</p>
            )}
            {visibleMounts.length > 0 && (
              <div className="grid grid-cols-[1fr_auto_1fr_auto_auto] items-center gap-x-2 gap-y-2">
                <span className="text-muted-foreground text-sm font-medium">Host</span>
                <span />
                <span className="text-muted-foreground flex items-center gap-1 text-sm font-medium">
                  Container
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Info className="text-muted-foreground h-3 w-3" />
                    </TooltipTrigger>
                    <TooltipContent side="top" className="max-w-64">
                      ~ expands to {containerHomeDir || '/home/<user>'}. If using a custom image
                      with a different user, enter absolute paths instead.
                    </TooltipContent>
                  </Tooltip>
                </span>
                <span />
                <span />
                {visibleMounts.map(({ mount, index: mountIndex }) => (
                  <Fragment key={mountIndex}>
                    <DirectoryBrowser
                      value={mount.hostPath}
                      onChange={(val) =>
                        setMounts((prev) =>
                          prev.map((m, i) => (i === mountIndex ? { ...m, hostPath: val } : m)),
                        )
                      }
                      disabled={isSubmitting || disabled}
                      defaultPath={homeDir}
                      placeholder="/host/path"
                      mode="file"
                    />
                    <ArrowRight className="text-muted-foreground h-3.5 w-3.5 shrink-0" />
                    <Input
                      placeholder="/container/path"
                      value={containerToDisplay(mount.containerPath)}
                      onChange={(e) => {
                        const absolutePath = containerToAbsolute(e.target.value)
                        setMounts((prev) =>
                          prev.map((m, i) =>
                            i === mountIndex ? { ...m, containerPath: absolutePath } : m,
                          ),
                        )
                      }}
                      className="font-mono text-sm"
                      disabled={isSubmitting || disabled}
                    />
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          type="button"
                          size="sm"
                          variant={mount.readOnly ? 'ghost' : 'secondary'}
                          onClick={() =>
                            setMounts((prev) =>
                              prev.map((m, i) =>
                                i === mountIndex ? { ...m, readOnly: !m.readOnly } : m,
                              ),
                            )
                          }
                          disabled={isSubmitting || disabled}
                          className="shrink-0 px-2 font-mono text-sm"
                        >
                          {mount.readOnly ? 'RO' : 'RW'}
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>{mount.readOnly ? 'Read-only' : 'Read-write'}</TooltipContent>
                    </Tooltip>
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      onClick={() => setMounts((prev) => prev.filter((_, i) => i !== mountIndex))}
                      disabled={isSubmitting || disabled}
                      className="shrink-0 px-2"
                      icon={Trash2}
                    />
                  </Fragment>
                ))}
              </div>
            )}
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="font-medium">Environment Variables</label>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={handleAddEnvVar}
                disabled={isSubmitting || disabled}
                icon={Plus}
              >
                Add
              </Button>
            </div>
            {visibleEnvVars.length === 0 && (
              <p className="text-muted-foreground text-sm">No environment variables configured.</p>
            )}
            {visibleEnvVars.map(({ entry, index }) => (
              <div key={index} className="flex items-center gap-2">
                <Input
                  placeholder="KEY"
                  value={entry.key}
                  onChange={(e) => handleUpdateEnvVar(index, 'key', e.target.value)}
                  className="flex-1 font-mono text-sm"
                  disabled={isSubmitting || disabled}
                />
                <Input
                  placeholder="value"
                  value={entry.value}
                  onChange={(e) => handleUpdateEnvVar(index, 'value', e.target.value)}
                  className="flex-1 font-mono text-sm"
                  type={
                    entry.key.includes('KEY') ||
                    entry.key.includes('SECRET') ||
                    entry.key.includes('TOKEN')
                      ? 'password'
                      : 'text'
                  }
                  disabled={isSubmitting || disabled}
                />
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  onClick={() => handleRemoveEnvVar(index)}
                  disabled={isSubmitting || disabled}
                  className="shrink-0 px-2"
                  icon={Trash2}
                />
              </div>
            ))}
          </div>
        </CollapsibleContent>
      </Collapsible>

      {displayError && <p className="text-error">{displayError}</p>}

      <div className="flex justify-end">
        <Button onClick={handleSubmit} disabled={isSubmitting || disabled || !isValid}>
          {isSubmitting && !disabled ? (
            <>
              <Loader2 className="animate-spin" />
              {isEditMode ? 'Saving...' : 'Creating...'}
            </>
          ) : isEditMode ? (
            'Save Changes'
          ) : (
            'Create'
          )}
        </Button>
      </div>
    </div>
  )
}

/** Props for FormField. */
interface FormFieldProps {
  label: React.ReactNode
  description?: string
  required?: boolean
  children: React.ReactNode
}

/** Simple labelled form field wrapper. */
function FormField({ label, description, required, children }: FormFieldProps) {
  return (
    <div className="space-y-1.5">
      <label className="font-medium">
        {label}
        {required && <span className="text-error ml-0.5">*</span>}
      </label>
      {description && <p className="text-muted-foreground text-sm">{description}</p>}
      {children}
    </div>
  )
}

import { ArrowRight, Info, Plus, Trash2 } from 'lucide-react'
import type React from 'react'
import { Fragment, useState } from 'react'

import { Button } from '@/components/ui/button'
import DirectoryBrowser from '@/components/ui/directory-browser'
import { Input } from '@/components/ui/input'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import type { Mount } from '@/lib/types'

import { RemovablePortChip } from './port-chip'
import type { EnvVarEntry } from './project-config-form-types'

/** Props for FormField. */
interface FormFieldProps {
  label: React.ReactNode
  description?: string
  required?: boolean
  children: React.ReactNode
}

/** Simple labelled form field wrapper. */
export function FormField({ label, description, required, children }: FormFieldProps) {
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

/** Props for the BindMountsField component. */
interface BindMountsFieldProps {
  /** Current list of mounts with their original indices. */
  visibleMounts: { mount: Mount; index: number }[]
  /** Container home directory for tilde expansion display. */
  containerHomeDir: string
  /** Host home directory for the directory browser default path. */
  homeDir: string
  /** Container path that cannot be removed (required agent config mount). */
  requiredContainerPath: string | null
  /** Whether the form is currently submitting. */
  isSubmitting: boolean
  /** Converts an absolute container path to a display string (tilde notation). */
  containerToDisplay: (path: string) => string
  /** Converts a display string back to an absolute container path. */
  containerToAbsolute: (input: string) => string
  /** Updates the mounts array. */
  onMountsChange: React.Dispatch<React.SetStateAction<Mount[]>>
}

/**
 * Editable list of bind mounts for the project config form.
 *
 * Each row shows host path, container path, read-only toggle, and a
 * delete button. Required mounts (e.g. agent config) are non-removable.
 */
export function BindMountsField({
  visibleMounts,
  containerHomeDir,
  homeDir,
  requiredContainerPath,
  isSubmitting,
  containerToDisplay,
  containerToAbsolute,
  onMountsChange,
}: BindMountsFieldProps) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <label className="font-medium">
          <span className="flex items-center gap-1.5">
            Bind Mounts
            <Tooltip>
              <TooltipTrigger asChild>
                <Info className="text-muted-foreground h-3.5 w-3.5" />
              </TooltipTrigger>
              <TooltipContent side="right">
                <p>Mount host directories into the container.</p>
              </TooltipContent>
            </Tooltip>
          </span>
        </label>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          onClick={() =>
            onMountsChange((prev) => [...prev, { hostPath: '', containerPath: '', readOnly: true }])
          }
          disabled={isSubmitting}
          icon={Plus}
        >
          Add
        </Button>
      </div>

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
                ~ expands to {containerHomeDir || '/home/<user>'}. If using a custom image with a
                different user, enter absolute paths instead.
              </TooltipContent>
            </Tooltip>
          </span>
          <span />
          <span />
          {visibleMounts.map(({ mount, index: mountIndex }) => {
            const isRequired = mount.containerPath === requiredContainerPath
            return (
              <Fragment key={mountIndex}>
                <DirectoryBrowser
                  value={mount.hostPath}
                  onChange={(val) =>
                    onMountsChange((prev) =>
                      prev.map((m, i) => (i === mountIndex ? { ...m, hostPath: val } : m)),
                    )
                  }
                  disabled={isSubmitting}
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
                    onMountsChange((prev) =>
                      prev.map((m, i) =>
                        i === mountIndex ? { ...m, containerPath: absolutePath } : m,
                      ),
                    )
                  }}
                  className="font-mono text-sm"
                  disabled={isSubmitting || isRequired}
                />
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      size="sm"
                      variant={mount.readOnly ? 'ghost' : 'secondary'}
                      onClick={() =>
                        onMountsChange((prev) =>
                          prev.map((m, i) =>
                            i === mountIndex ? { ...m, readOnly: !m.readOnly } : m,
                          ),
                        )
                      }
                      disabled={isSubmitting}
                      className="shrink-0 px-2 font-mono text-sm"
                    >
                      {mount.readOnly ? 'RO' : 'RW'}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{mount.readOnly ? 'Read-only' : 'Read-write'}</TooltipContent>
                </Tooltip>
                {isRequired ? (
                  <span className="text-muted-foreground shrink-0 px-2 text-xs">Required</span>
                ) : (
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      onMountsChange((prev) => prev.filter((_, i) => i !== mountIndex))
                    }
                    disabled={isSubmitting}
                    className="shrink-0 px-2"
                    icon={Trash2}
                  />
                )}
              </Fragment>
            )
          })}
        </div>
      )}
    </div>
  )
}

/** Props for the EnvVarsField component. */
interface EnvVarsFieldProps {
  /** Current list of env var entries with their original indices. */
  visibleEnvVars: { entry: EnvVarEntry; index: number }[]
  /** Set of env var keys managed by runtimes (displayed as read-only). */
  runtimeEnvKeys: Set<string>
  /** Whether the form is currently submitting. */
  isSubmitting: boolean
  /** Adds a blank env var row. */
  onAdd: () => void
  /** Updates a single env var entry by index. */
  onUpdate: (index: number, field: 'key' | 'value', newValue: string) => void
  /** Removes an env var row by index. */
  onRemove: (index: number) => void
}

/**
 * Editable list of environment variables for the project config form.
 *
 * Each row shows a key input, value input (password-masked for secrets),
 * and a delete button. Runtime-managed env vars are read-only.
 */
export function EnvVarsField({
  visibleEnvVars,
  runtimeEnvKeys,
  isSubmitting,
  onAdd,
  onUpdate,
  onRemove,
}: EnvVarsFieldProps) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <label className="font-medium">Environment Variables</label>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          onClick={onAdd}
          disabled={isSubmitting}
          icon={Plus}
        >
          Add
        </Button>
      </div>
      {visibleEnvVars.length === 0 && (
        <p className="text-muted-foreground text-sm">No environment variables configured.</p>
      )}
      {visibleEnvVars.map(({ entry, index }) => {
        const isRuntimeManaged = runtimeEnvKeys.has(entry.key)
        return (
          <div key={index} className="flex items-center gap-2">
            <Input
              placeholder="KEY"
              value={entry.key}
              onChange={(e) => onUpdate(index, 'key', e.target.value)}
              className="flex-1 font-mono text-sm"
              disabled={isSubmitting || isRuntimeManaged}
            />
            <Input
              placeholder="value"
              value={entry.value}
              onChange={(e) => onUpdate(index, 'value', e.target.value)}
              className="flex-1 font-mono text-sm"
              type={
                entry.key.includes('KEY') ||
                entry.key.includes('SECRET') ||
                entry.key.includes('TOKEN')
                  ? 'password'
                  : 'text'
              }
              disabled={isSubmitting || isRuntimeManaged}
            />
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={() => onRemove(index)}
              disabled={isSubmitting || isRuntimeManaged}
              className="shrink-0 px-2"
              icon={Trash2}
            />
          </div>
        )
      })}
    </div>
  )
}

/** Props for the ForwardedPortsField component. */
interface ForwardedPortsFieldProps {
  /** Current list of forwarded port numbers. */
  ports: number[]
  /** Whether the form is currently submitting. */
  isSubmitting: boolean
  /** Adds a validated port to the list. */
  onAdd: (port: number) => void
  /** Removes a port by index. */
  onRemove: (index: number) => void
}

/**
 * Editable list of forwarded ports for the project config form.
 *
 * Always-visible input field with inline validation. Type a port number
 * and press Enter to add it as a chip. Each port is exposed via Warden's
 * reverse proxy with HTTP and WebSocket support (for dev server HMR).
 */
export function ForwardedPortsField({
  ports,
  isSubmitting,
  onAdd,
  onRemove,
}: ForwardedPortsFieldProps) {
  const [inputValue, setInputValue] = useState('')
  const [error, setError] = useState('')

  const validate = (value: string): number | null => {
    const trimmed = value.trim()
    if (!trimmed) return null
    const parsed = parseInt(trimmed, 10)
    if (Number.isNaN(parsed) || parsed < 1 || parsed > 65535) {
      setError('Port must be 1-65535')
      return null
    }
    if (ports.includes(parsed)) {
      setError('Port already added')
      return null
    }
    setError('')
    return parsed
  }

  const handleSubmit = () => {
    const port = validate(inputValue)
    if (port !== null) {
      onAdd(port)
      setInputValue('')
    }
  }

  return (
    <div>
      <div className="flex items-center gap-2">
        <Input
          type="number"
          min={1}
          max={65535}
          placeholder="port number"
          value={inputValue}
          onChange={(e) => {
            setInputValue(e.target.value)
            if (error) setError('')
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              handleSubmit()
            }
          }}
          className="w-48 font-mono text-sm"
          disabled={isSubmitting}
        />
      </div>

      {error && <p className="text-error mt-1.5 text-xs">{error}</p>}

      {ports.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1.5">
          {ports.map((port, index) => (
            <RemovablePortChip
              key={port}
              port={port}
              disabled={isSubmitting}
              onRemove={() => onRemove(index)}
            />
          ))}
        </div>
      )}
    </div>
  )
}

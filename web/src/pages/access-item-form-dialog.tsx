import { useEffect, useState } from 'react'
import { containerPathToDisplay, containerPathToAbsolute } from '@/lib/utils'
import {
  Loader2,
  Plus,
  Trash2,
  FlaskConical,
  RotateCcw,
  CircleCheck,
  CircleMinus,
  Save,
} from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { createAccessItem, updateAccessItem, resetAccessItem, resolveAccessItems } from '@/lib/api'
import DirectoryBrowser from '@/components/ui/directory-browser'
import type {
  AccessItemResponse,
  AccessCredential,
  AccessSource,
  AccessInjection,
  AccessSourceType,
  AccessInjectionType,
  ResolvedItem,
} from '@/lib/types'

// --- Credential Form Helpers ---

/** Creates a blank source entry. */
function emptySource(): AccessSource {
  return { type: 'env', value: '' }
}

/** Creates a blank injection entry. */
function emptyInjection(): AccessInjection {
  return { type: 'env', key: '' }
}

/** Creates a blank credential entry. */
function emptyCredential(): AccessCredential {
  return { label: '', sources: [emptySource()], injections: [emptyInjection()] }
}

/** Human-readable labels for source types. */
const SOURCE_TYPE_LABELS: Record<AccessSourceType, string> = {
  env: 'Env Var',
  file: 'File',
  socket: 'Socket',
  command: 'Command',
}

/** Human-readable labels for injection types. */
const INJECTION_TYPE_LABELS: Record<AccessInjectionType, string> = {
  env: 'Env Var',
  mount_file: 'Mount File',
  mount_socket: 'Mount Socket',
}

// --- Source Row ---

/** Editable row for a single source entry. */
function SourceRow({
  source,
  onChange,
  onRemove,
  isOnly,
  homeDir,
}: {
  source: AccessSource
  onChange: (updated: AccessSource) => void
  onRemove: () => void
  isOnly: boolean
  homeDir: string
}) {
  const useBrowser = source.type === 'file' || source.type === 'socket'

  return (
    <div className="flex items-center gap-2">
      <Select
        value={source.type}
        onValueChange={(val) => onChange({ ...source, type: val as AccessSourceType })}
      >
        <SelectTrigger className="w-40 shrink-0 text-left">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {Object.entries(SOURCE_TYPE_LABELS).map(([type, label]) => (
            <SelectItem key={type} value={type}>
              {label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {useBrowser ? (
        <div className="flex-1">
          <DirectoryBrowser
            value={source.value}
            onChange={(val) => onChange({ ...source, value: val })}
            defaultPath={homeDir}
            mode="file"
            placeholder={source.type === 'socket' ? '/run/agent.sock' : '~/.ssh/config'}
          />
        </div>
      ) : (
        <Input
          placeholder={source.type === 'env' ? 'VAR_NAME' : 'command to run'}
          value={source.value}
          onChange={(e) => onChange({ ...source, value: e.target.value })}
          className="flex-1 font-mono text-sm"
        />
      )}
      {!isOnly && (
        <Button
          type="button"
          size="sm"
          variant="ghost"
          color="error"
          onClick={onRemove}
          icon={Trash2}
        />
      )}
    </div>
  )
}

// --- Injection Row ---

/** Editable row for a single injection entry. */
function InjectionRow({
  injection,
  onChange,
  onRemove,
  isOnly,
  containerHomeDir,
}: {
  injection: AccessInjection
  onChange: (updated: AccessInjection) => void
  onRemove: () => void
  isOnly: boolean
  containerHomeDir: string
}) {
  const isMount = injection.type === 'mount_file' || injection.type === 'mount_socket'

  return (
    <div className="flex items-center gap-2">
      <Select
        value={injection.type}
        onValueChange={(val) => onChange({ ...injection, type: val as AccessInjectionType })}
      >
        <SelectTrigger className="w-40 shrink-0 text-left">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {Object.entries(INJECTION_TYPE_LABELS).map(([type, label]) => (
            <SelectItem key={type} value={type}>
              {label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        placeholder={injection.type === 'env' ? 'VAR_NAME' : isMount ? '~/path' : '/container/path'}
        value={isMount ? containerPathToDisplay(injection.key, containerHomeDir) : injection.key}
        onChange={(e) =>
          onChange({
            ...injection,
            key: isMount
              ? containerPathToAbsolute(e.target.value, containerHomeDir)
              : e.target.value,
          })
        }
        className="flex-1 font-mono text-sm"
      />
      {!isOnly && (
        <Button
          type="button"
          size="sm"
          variant="ghost"
          color="error"
          onClick={onRemove}
          icon={Trash2}
        />
      )}
    </div>
  )
}

// --- Credential Builder ---

/** Editable section for a single credential within an access item. */
function CredentialBuilder({
  credential,
  onChange,
  onRemove,
  isOnly,
  homeDir,
  containerHomeDir,
}: {
  credential: AccessCredential
  onChange: (updated: AccessCredential) => void
  onRemove: () => void
  isOnly: boolean
  homeDir: string
  containerHomeDir: string
}) {
  const updateSource = (index: number, updated: AccessSource) => {
    const next = credential.sources.map((s, i) => (i === index ? updated : s))
    onChange({ ...credential, sources: next })
  }

  const removeSource = (index: number) => {
    onChange({ ...credential, sources: credential.sources.filter((_, i) => i !== index) })
  }

  const addSource = () => {
    onChange({ ...credential, sources: [...credential.sources, emptySource()] })
  }

  const updateInjection = (index: number, updated: AccessInjection) => {
    const next = credential.injections.map((inj, i) => (i === index ? updated : inj))
    onChange({ ...credential, injections: next })
  }

  const removeInjection = (index: number) => {
    onChange({ ...credential, injections: credential.injections.filter((_, i) => i !== index) })
  }

  const addInjection = () => {
    onChange({ ...credential, injections: [...credential.injections, emptyInjection()] })
  }

  return (
    <div className="border-border bg-content-1 space-y-3 rounded border p-3">
      <div className="flex items-center justify-between">
        <Input
          placeholder="Credential label"
          value={credential.label}
          onChange={(e) => onChange({ ...credential, label: e.target.value })}
          className="max-w-64"
        />
        {!isOnly && (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            color="error"
            onClick={onRemove}
            icon={Trash2}
          />
        )}
      </div>

      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <label className="text-muted-foreground text-xs font-medium uppercase">Sources</label>
          <Button type="button" size="sm" variant="ghost" onClick={addSource} icon={Plus}>
            Add
          </Button>
        </div>
        {credential.sources.map((source, i) => (
          <SourceRow
            key={i}
            source={source}
            onChange={(s) => updateSource(i, s)}
            onRemove={() => removeSource(i)}
            isOnly={credential.sources.length === 1}
            homeDir={homeDir}
          />
        ))}
      </div>

      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <label className="text-muted-foreground text-xs font-medium uppercase">Injections</label>
          <Button type="button" size="sm" variant="ghost" onClick={addInjection} icon={Plus}>
            Add
          </Button>
        </div>
        {credential.injections.map((injection, i) => (
          <InjectionRow
            key={i}
            injection={injection}
            onChange={(inj) => updateInjection(i, inj)}
            onRemove={() => removeInjection(i)}
            isOnly={credential.injections.length === 1}
            containerHomeDir={containerHomeDir}
          />
        ))}
      </div>
    </div>
  )
}

// --- Create/Edit Dialog ---

/** Dialog for creating or editing an access item. */
export default function AccessItemFormDialog({
  open,
  onOpenChange,
  editItem,
  onSaved,
  homeDir,
  containerHomeDir,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  editItem: AccessItemResponse | null
  onSaved: () => void
  homeDir: string
  containerHomeDir: string
}) {
  const [label, setLabel] = useState('')
  const [description, setDescription] = useState('')
  const [credentials, setCredentials] = useState<AccessCredential[]>([emptyCredential()])
  const [isSaving, setIsSaving] = useState(false)
  const [isResetting, setIsResetting] = useState(false)
  const [isTesting, setIsTesting] = useState(false)
  const [testResult, setTestResult] = useState<ResolvedItem | null>(null)

  const isEditMode = editItem !== null
  const isBuiltIn = editItem?.builtIn ?? false

  /** Reset form when dialog opens with an item or fresh. */
  useEffect(() => {
    if (!open) return
    setTestResult(null)
    if (editItem) {
      setLabel(editItem.label)
      setDescription(editItem.description)
      setCredentials(
        editItem.credentials.length > 0
          ? structuredClone(editItem.credentials)
          : [emptyCredential()],
      )
    } else {
      setLabel('')
      setDescription('')
      setCredentials([emptyCredential()])
    }
  }, [open, editItem])

  const updateCredential = (index: number, updated: AccessCredential) => {
    setCredentials((prev) => prev.map((c, i) => (i === index ? updated : c)))
  }

  const removeCredential = (index: number) => {
    setCredentials((prev) => prev.filter((_, i) => i !== index))
  }

  const addCredential = () => {
    setCredentials((prev) => [...prev, emptyCredential()])
  }

  /** Validates the form and returns an error or null. */
  const validate = (): string | null => {
    if (!label.trim()) return 'Label is required'
    for (const cred of credentials) {
      if (!cred.label.trim()) return 'Each credential needs a label'
      if (cred.sources.length === 0) return 'Each credential needs at least one source'
      if (cred.injections.length === 0) return 'Each credential needs at least one injection'
      for (const src of cred.sources) {
        if (!src.value.trim()) return 'Source values cannot be empty'
      }
      for (const inj of cred.injections) {
        if (!inj.key.trim()) return 'Injection keys cannot be empty'
      }
    }
    return null
  }

  /** Resets a built-in item to its default configuration. */
  const handleReset = async () => {
    if (!editItem) return
    setIsResetting(true)
    try {
      await resetAccessItem(editItem.id)
      toast.success(`Reset "${editItem.label}" to default`)
      onSaved()
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to reset access item')
    } finally {
      setIsResetting(false)
    }
  }

  /** Tests the current form state by resolving it server-side. */
  const handleTest = async () => {
    const validationError = validate()
    if (validationError) {
      toast.error(validationError)
      return
    }
    setIsTesting(true)
    setTestResult(null)
    try {
      const item = {
        id: editItem?.id ?? '',
        label,
        description,
        method: editItem?.method ?? 'transport',
        credentials,
        builtIn: editItem?.builtIn ?? false,
      }
      const resolved = await resolveAccessItems([item])
      setTestResult(resolved[0] ?? null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to test access item')
    } finally {
      setIsTesting(false)
    }
  }

  const handleSave = async () => {
    const validationError = validate()
    if (validationError) {
      toast.error(validationError)
      return
    }

    setIsSaving(true)
    try {
      if (isEditMode) {
        await updateAccessItem(editItem.id, {
          label: label.trim(),
          description: description.trim(),
          credentials,
        })
        toast.success('Access item updated')
      } else {
        await createAccessItem({
          label: label.trim(),
          description: description.trim(),
          credentials,
        })
        toast.success('Access item created')
      }
      onSaved()
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save access item')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>{isEditMode ? 'Edit Access Item' : 'Create Access Item'}</DialogTitle>
          <DialogDescription>
            {isEditMode
              ? 'Update the credential passthrough configuration.'
              : 'Define a custom credential passthrough from host to container.'}
          </DialogDescription>
        </DialogHeader>

        <div className="max-h-[60vh] space-y-4 overflow-y-auto px-1">
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Label</label>
            <Input
              placeholder="My Credentials"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
            />
          </div>

          <div className="space-y-1.5">
            <label className="text-sm font-medium">Description</label>
            <Textarea
              placeholder="What this access item provides..."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={2}
            />
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="text-sm font-medium">Credentials</label>
              <Button type="button" size="sm" variant="ghost" onClick={addCredential} icon={Plus}>
                Add Credential
              </Button>
            </div>
            <div className="space-y-4">
              {credentials.map((cred, i) => (
                <CredentialBuilder
                  key={i}
                  credential={cred}
                  onChange={(updated) => updateCredential(i, updated)}
                  onRemove={() => removeCredential(i)}
                  isOnly={credentials.length === 1}
                  homeDir={homeDir}
                  containerHomeDir={containerHomeDir}
                />
              ))}
            </div>
          </div>

          {/* Test resolution — available in both create and edit mode */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="text-sm font-medium">Test Resolution</label>
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={handleTest}
                disabled={isTesting}
                icon={isTesting ? Loader2 : FlaskConical}
                loading={isTesting}
              >
                Test
              </Button>
            </div>
            {testResult && (
              <div className="space-y-4">
                {testResult.credentials.map((cred, i) => (
                  <div
                    key={i}
                    className="bg-content-1 border-border space-y-1.5 rounded border p-2"
                  >
                    <div className="flex items-center gap-2">
                      {cred.resolved ? (
                        <CircleCheck className="text-success h-3.5 w-3.5" />
                      ) : (
                        <CircleMinus className="text-muted-foreground h-3.5 w-3.5" />
                      )}
                      <span className="text-sm font-medium">{cred.label}</span>
                      {cred.sourceMatched && (
                        <Badge variant="outline" className="font-mono text-xs">
                          {cred.sourceMatched}
                        </Badge>
                      )}
                    </div>
                    {cred.error && <p className="text-error text-sm">{cred.error}</p>}
                    {cred.injections && cred.injections.length > 0 && (
                      <div className="space-y-2">
                        {cred.injections.map((inj, j) => (
                          <div
                            key={j}
                            className="flex items-center gap-2 rounded font-mono text-xs"
                          >
                            <Badge variant="secondary" className="text-xs uppercase">
                              {inj.type.split('_').join(' ')}
                            </Badge>
                            <span className="text-muted-foreground">{inj.key}</span>
                            <span className="text-muted-foreground">=</span>
                            <span className="truncate">{inj.value}</span>
                            {inj.readOnly && (
                              <Badge variant="outline" className="text-xs">
                                RO
                              </Badge>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                    {!cred.resolved && !cred.error && (
                      <p className="text-muted-foreground text-xs">Not detected on host</p>
                    )}
                  </div>
                ))}
              </div>
            )}
            {!testResult && !isTesting && (
              <p className="text-muted-foreground text-xs">
                Click Test to preview what will be injected into containers.
              </p>
            )}
          </div>
        </div>

        <DialogFooter>
          {isBuiltIn && (
            <Button
              className="mr-auto"
              variant="outline"
              onClick={handleReset}
              disabled={isResetting || isSaving}
              icon={isResetting ? Loader2 : RotateCcw}
              loading={isResetting}
            >
              Reset to Default
            </Button>
          )}
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            onClick={handleSave}
            disabled={isSaving}
            icon={isSaving ? Loader2 : Save}
            loading={isSaving}
          >
            {isEditMode ? 'Save' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

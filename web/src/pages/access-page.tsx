import { useCallback, useEffect, useState } from 'react'
import { containerPathToDisplay, containerPathToAbsolute } from '@/lib/utils'
import {
  KeyRound,
  Loader2,
  Plus,
  Pencil,
  Trash2,
  FlaskConical,
  RefreshCw,
  RotateCcw,
  CircleCheck,
  CircleMinus,
  Save,
} from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
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
import {
  fetchAccessItems,
  fetchDefaults,
  createAccessItem,
  updateAccessItem,
  deleteAccessItem,
  resetAccessItem,
  resolveAccessItems,
} from '@/lib/api'
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
function AccessItemFormDialog({
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

  /** Tests the current item by calling resolve on the saved version. */
  const handleTest = async () => {
    if (!editItem) return
    setIsTesting(true)
    setTestResult(null)
    try {
      const resolved = await resolveAccessItems([editItem.id])
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

          {/* Test results — only available for saved items */}
          {isEditMode && (
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
          )}
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

// --- Delete Confirmation Dialog ---

/** Confirmation dialog for deleting a user-defined access item. */
function DeleteAccessItemDialog({
  open,
  onOpenChange,
  item,
  onDeleted,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  item: AccessItemResponse | null
  onDeleted: () => void
}) {
  const [isDeleting, setIsDeleting] = useState(false)

  const handleDelete = async () => {
    if (!item) return
    setIsDeleting(true)
    try {
      await deleteAccessItem(item.id)
      toast.success(`Deleted "${item.label}"`)
      onDeleted()
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete access item')
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Delete Access Item</DialogTitle>
          <DialogDescription>
            Are you sure you want to delete <strong>{item?.label}</strong>? This cannot be undone.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant="error"
            onClick={handleDelete}
            disabled={isDeleting}
            icon={isDeleting ? Loader2 : Trash2}
            loading={isDeleting}
          >
            Delete
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// --- Access Item Card ---

/** Renders a single access item as a card with detection status and actions. */
function AccessItemCard({
  item,
  onEdit,
  onDelete,
}: {
  item: AccessItemResponse
  onEdit: () => void
  onDelete: () => void
}) {
  const isAvailable = item.detection.available

  return (
    <Card>
      <CardHeader>
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-2">
            <div
              className={`h-2.5 w-2.5 shrink-0 rounded-full ${isAvailable ? 'bg-success' : 'bg-muted-foreground/40'}`}
            />
            <CardTitle className="text-base">{item.label}</CardTitle>
            {item.builtIn && (
              <Badge variant="secondary" className="ml-2 text-xs">
                Built-in
              </Badge>
            )}
          </div>
          <div className="flex items-center gap-1">
            <Button size="sm" variant="ghost" color="warning" onClick={onEdit} icon={Pencil} />
            {!item.builtIn && (
              <Button size="sm" variant="ghost" color="error" onClick={onDelete} icon={Trash2} />
            )}
          </div>
        </div>
        {item.description && <CardDescription>{item.description}</CardDescription>}
      </CardHeader>

      <CardContent>
        <div className="space-y-1.5">
          {item.detection.credentials.map((cred, i) => (
            <div key={i} className="flex items-center gap-2 text-sm">
              {cred.available ? (
                <CircleCheck className="text-success h-3.5 w-3.5 shrink-0" />
              ) : (
                <CircleMinus className="text-muted-foreground h-3.5 w-3.5 shrink-0" />
              )}
              <span className={cred.available ? '' : 'text-muted-foreground'}>{cred.label}</span>
              {cred.sourceMatched && (
                <span className="text-muted-foreground font-mono text-xs">
                  ({cred.sourceMatched})
                </span>
              )}
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

// --- Page ---

/** Access page showing all credential passthrough items with detection and management. */
export default function AccessPage() {
  const [items, setItems] = useState<AccessItemResponse[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [homeDir, setHomeDir] = useState('')
  const [containerHomeDir, setContainerHomeDir] = useState('')

  // Fetch host/container home dirs for the directory browser and ~ expansion.
  useEffect(() => {
    fetchDefaults()
      .then((defaults) => {
        if (defaults.homeDir) setHomeDir(defaults.homeDir)
        if (defaults.containerHomeDir) setContainerHomeDir(defaults.containerHomeDir)
      })
      .catch(() => {})
  }, [])

  // Form dialog state
  const [isFormOpen, setIsFormOpen] = useState(false)
  const [editItem, setEditItem] = useState<AccessItemResponse | null>(null)

  // Delete dialog state
  const [isDeleteOpen, setIsDeleteOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<AccessItemResponse | null>(null)

  const loadItems = useCallback(async () => {
    setIsLoading(true)
    try {
      const data = await fetchAccessItems()
      setItems(data)
    } catch {
      toast.error('Failed to load access items')
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    loadItems()
  }, [loadItems])

  const handleCreate = () => {
    setEditItem(null)
    setIsFormOpen(true)
  }

  const handleEdit = (item: AccessItemResponse) => {
    setEditItem(item)
    setIsFormOpen(true)
  }

  const handleDelete = (item: AccessItemResponse) => {
    setDeleteTarget(item)
    setIsDeleteOpen(true)
  }

  const availableCount = items.filter((i) => i.detection.available).length

  return (
    <div className="-m-6 flex h-[calc(100vh-57px)] flex-col">
      {/* Page header */}
      <header className="flex items-center justify-between border-b px-4 py-3">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <KeyRound className="text-muted-foreground h-5 w-5" />
            <h2>Access</h2>
          </div>
          {!isLoading && (
            <span className="text-muted-foreground text-sm">
              {availableCount} of {items.length} detected
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            icon={isLoading ? Loader2 : RefreshCw}
            loading={isLoading}
            onClick={loadItems}
          >
            Refresh
          </Button>
          <Button size="sm" icon={Plus} onClick={handleCreate}>
            Create
          </Button>
        </div>
      </header>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4">
        {isLoading && items.length === 0 && (
          <div className="flex items-center justify-center py-16">
            <Loader2 className="text-muted-foreground h-5 w-5 animate-spin" />
          </div>
        )}

        {!isLoading && items.length === 0 && (
          <div className="text-muted-foreground flex flex-col items-center gap-2 py-16">
            <KeyRound className="h-8 w-8" />
            <p>No access items configured.</p>
            <Button size="sm" variant="outline" icon={Plus} onClick={handleCreate}>
              Create one
            </Button>
          </div>
        )}

        {items.length > 0 && (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((item) => (
              <AccessItemCard
                key={item.id}
                item={item}
                onEdit={() => handleEdit(item)}
                onDelete={() => handleDelete(item)}
              />
            ))}
          </div>
        )}
      </div>

      {/* Dialogs */}
      <AccessItemFormDialog
        open={isFormOpen}
        onOpenChange={setIsFormOpen}
        editItem={editItem}
        onSaved={loadItems}
        homeDir={homeDir}
        containerHomeDir={containerHomeDir}
      />
      <DeleteAccessItemDialog
        open={isDeleteOpen}
        onOpenChange={setIsDeleteOpen}
        item={deleteTarget}
        onDeleted={loadItems}
      />
    </div>
  )
}

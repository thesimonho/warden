import {
  CircleCheck,
  CircleMinus,
  KeyRound,
  Loader2,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
} from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
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
import { deleteAccessItem, fetchAccessItems, fetchDefaults } from '@/lib/api'
import type { AccessItemResponse } from '@/lib/types'
import AccessItemFormDialog from '@/pages/access-item-form-dialog'

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

import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  RefreshCw,
  Download,
  Trash2,
  Loader2,
  ChevronDown,
  ShieldCheck,
  Search,
  X,
} from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { AuditLogTable } from '@/components/audit-log-table'
import { useInterval } from '@/hooks/use-interval'
import { ActivityTimeline } from '@/components/activity-timeline'
import { fetchAuditLog, fetchAuditSummary, auditExportUrl, deleteAuditEvents } from '@/lib/api'
import { cn } from '@/lib/utils'
import { formatCost } from '@/lib/cost'
import { DAY } from '@/lib/activity-buckets'
import {
  AUDIT_CATEGORIES,
  ALL_LEVELS,
  levelColorVar,
  formatFullTimestamp,
} from '@/lib/audit-log-utils'
import { readStoredSet, writeStoredSet } from '@/lib/storage'
import type {
  AuditLogEntry,
  AuditLogLevel,
  AuditCategory,
  AuditSummary,
  AuditFilters,
} from '@/lib/types'

// --- Constants ---

/** Auto-refresh interval options in seconds. */
const AUTO_REFRESH_OPTIONS = [
  { label: '10s', seconds: 10 },
  { label: '30s', seconds: 30 },
  { label: '1m', seconds: 60 },
  { label: '5m', seconds: 300 },
] as const

/** Timeline loads all entries (no time filter) so the full graph is visible. */
const TIMELINE_FILTERS: AuditFilters = {}

/** Toggles a value in a Set, returning a new Set. */
function toggleSetValue<T>(set: Set<T>, value: T): Set<T> {
  const next = new Set(set)
  if (next.has(value)) next.delete(value)
  else next.add(value)
  return next
}

// --- Components ---

/** A multi-select filter toggle with count badge. */
function FilterTab({
  label,
  count,
  active,
  onClick,
  colorVar,
}: {
  label: string
  count: number
  active: boolean
  onClick: () => void
  colorVar?: string
}) {
  return (
    <button
      type="button"
      style={colorVar ? ({ '--tab-color': `var(${colorVar})` } as React.CSSProperties) : undefined}
      className={cn(
        'flex cursor-pointer items-center gap-1.5 rounded px-2.5 py-1 text-sm capitalize transition-colors',
        active && 'bg-muted',
      )}
      onClick={onClick}
    >
      <span className="text-(--tab-color)">{label}</span>
      <span className="text-muted-foreground text-xs">{count}</span>
    </button>
  )
}

/** A single metric display in the summary dashboard. */
function SummaryCard({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-muted-foreground text-xs tracking-wide uppercase">{label}</span>
      <span className="text-lg font-semibold">{value}</span>
    </div>
  )
}

/** Text the user must type to confirm audit deletion. */
const DELETE_CONFIRMATION = 'delete'

/** Delete audit events dialog with scoping options and type-to-confirm. */
function DeleteAuditDialog({
  open,
  onOpenChange,
  projectNames,
  onDeleted,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  projectNames: string[]
  onDeleted: () => void
}) {
  const [project, setProject] = useState('all')
  const [category, setCategory] = useState<AuditCategory | 'all'>('all')
  const [age, setAge] = useState<'all' | '7' | '30' | '90'>('all')
  const [confirmText, setConfirmText] = useState('')
  const [isDeleting, setIsDeleting] = useState(false)

  const isConfirmed = confirmText === DELETE_CONFIRMATION

  const handleDelete = async () => {
    if (!isConfirmed) return

    const filters: AuditFilters = {}
    if (project !== 'all') filters.projectId = project
    if (category !== 'all') filters.category = category
    if (age !== 'all') {
      filters.until = new Date(Date.now() - Number(age) * DAY).toISOString()
    }

    setIsDeleting(true)
    try {
      await deleteAuditEvents(filters)
      toast.success('Audit events deleted')
      setConfirmText('')
      onOpenChange(false)
      onDeleted()
    } catch {
      toast.error('Failed to delete audit events')
    } finally {
      setIsDeleting(false)
    }
  }

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) setConfirmText('')
    onOpenChange(nextOpen)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete Audit Events</DialogTitle>
          <DialogDescription>
            Choose which events to delete. This cannot be undone.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4 py-2">
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Project</label>
            <Select value={project} onValueChange={setProject}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All projects</SelectItem>
                {projectNames.map((name) => (
                  <SelectItem key={name} value={name}>
                    {name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Category</label>
            <Select value={category} onValueChange={(v) => setCategory(v as AuditCategory | 'all')}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All categories</SelectItem>
                {AUDIT_CATEGORIES.map(({ id, label }) => (
                  <SelectItem key={id} value={id}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Age</label>
            <Select value={age} onValueChange={(v) => setAge(v as 'all' | '7' | '30' | '90')}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">Any age</SelectItem>
                <SelectItem value="7">Older than 7 days</SelectItem>
                <SelectItem value="30">Older than 30 days</SelectItem>
                <SelectItem value="90">Older than 90 days</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-1.5">
            <p className="text-error text-sm">
              Type <strong>{DELETE_CONFIRMATION}</strong> to confirm.
            </p>
            <input
              type="text"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder={DELETE_CONFIRMATION}
              disabled={isDeleting}
              className="border-input bg-background placeholder:text-muted-foreground/40 focus:border-ring focus:ring-ring/50 h-8 max-w-48 rounded border px-2.5 text-sm transition-colors outline-none focus:ring-[2px]"
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={() => handleOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant="error"
            icon={isDeleting ? Loader2 : Trash2}
            loading={isDeleting}
            onClick={handleDelete}
            disabled={!isConfirmed || isDeleting}
          >
            Delete
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// --- Storage keys ---

const STORAGE_KEY_CATEGORIES = 'audit:categories'
const STORAGE_KEY_LEVELS = 'audit:levels'

// --- Page ---

/** Unified audit log page with timeline, summary, multi-select filters, and export. */
export default function AuditPage() {
  const [entries, setEntries] = useState<AuditLogEntry[]>([])
  const [allEntries, setAllEntries] = useState<AuditLogEntry[]>([])
  const [summary, setSummary] = useState<AuditSummary | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [isDeleteOpen, setIsDeleteOpen] = useState(false)
  const [activeCategories, setActiveCategories] = useState<Set<AuditCategory>>(() =>
    readStoredSet(
      STORAGE_KEY_CATEGORIES,
      AUDIT_CATEGORIES.filter((c) => c.id !== 'debug').map((c) => c.id),
    ),
  )
  const [activeLevels, setActiveLevels] = useState<Set<AuditLogLevel>>(() =>
    readStoredSet(STORAGE_KEY_LEVELS, ALL_LEVELS),
  )
  const [globalFilter, setGlobalFilter] = useState('')
  const [filteredCount, setFilteredCount] = useState(0)
  const [since, setSince] = useState<string | undefined>()
  const [until, setUntil] = useState<string | undefined>()
  const [autoRefreshSeconds, setAutoRefreshSeconds] = useState<number | null>(null)
  const [isRefreshMenuOpen, setIsRefreshMenuOpen] = useState(false)

  /** Server-side filters (time range only — category/level/search are client-side). */
  const currentFilters = useMemo(
    (): AuditFilters => ({
      since,
      until,
    }),
    [since, until],
  )

  /** Filter helper: returns entries matching the active categories. */
  const filterByCategory = useCallback(
    (items: AuditLogEntry[]) => {
      if (activeCategories.size === AUDIT_CATEGORIES.length) return items
      return items.filter((e) => activeCategories.has(e.category ?? 'system'))
    },
    [activeCategories],
  )

  /** Entries filtered by active categories (client-side). */
  const categoryFilteredEntries = useMemo(
    () => filterByCategory(entries),
    [entries, filterByCategory],
  )

  /** All entries filtered by active categories (for the timeline graph). */
  const timelineEntries = useMemo(
    () => filterByCategory(allEntries),
    [allEntries, filterByCategory],
  )

  // --- Data fetching ---

  const loadTableData = useCallback(async (filters: AuditFilters) => {
    setIsLoading(true)
    try {
      const [entriesData, summaryData] = await Promise.all([
        fetchAuditLog(filters),
        fetchAuditSummary({
          projectId: filters.projectId,
          since: filters.since,
          until: filters.until,
        }),
      ])
      setEntries(entriesData)
      setSummary(summaryData)
    } catch {
      toast.error('Failed to load audit log')
    } finally {
      setIsLoading(false)
    }
  }, [])

  const loadTimelineData = useCallback(async (filters: AuditFilters) => {
    try {
      setAllEntries(await fetchAuditLog(filters))
    } catch {
      /* timeline is non-critical */
    }
  }, [])

  useEffect(() => {
    loadTableData(currentFilters)
  }, [loadTableData, currentFilters])

  useEffect(() => {
    loadTimelineData(TIMELINE_FILTERS)
  }, [loadTimelineData])

  // --- Handlers ---

  const handleRangeChange = useCallback((newSince?: string, newUntil?: string) => {
    setSince(newSince)
    setUntil(newUntil)
  }, [])

  /** Refreshes table and timeline data. */
  const refreshAllData = useCallback(() => {
    loadTableData(currentFilters)
    loadTimelineData(TIMELINE_FILTERS)
  }, [loadTableData, loadTimelineData, currentFilters])

  useInterval(refreshAllData, autoRefreshSeconds ? autoRefreshSeconds * 1000 : null)

  // --- Counts for filter badges ---

  const categoryCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const { id } of AUDIT_CATEGORIES) counts[id] = 0
    for (const e of entries) {
      const cat = e.category ?? 'system'
      counts[cat] = (counts[cat] || 0) + 1
    }
    return counts
  }, [entries])

  const levelCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const level of ALL_LEVELS) counts[level] = 0
    for (const e of categoryFilteredEntries) {
      counts[e.level] = (counts[e.level] || 0) + 1
    }
    return counts
  }, [categoryFilteredEntries])

  const projectNames = useMemo(
    () => [...new Set(entries.map((e) => e.containerName).filter(Boolean) as string[])],
    [entries],
  )

  return (
    <div className="-m-6 flex h-[calc(100vh-57px)] flex-col">
      {/* Page header */}
      <header className="flex items-center justify-between border-b px-4 py-3">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <ShieldCheck className="text-muted-foreground h-5 w-5" />
            <h2>Audit Log</h2>
          </div>
          {!isLoading && (
            <span className="text-muted-foreground text-sm">
              {filteredCount === entries.length
                ? `${entries.length} entries`
                : `${filteredCount} of ${entries.length} entries`}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <div className="flex items-center">
            <Button
              variant="outline"
              size="sm"
              icon={isLoading ? Loader2 : RefreshCw}
              loading={isLoading || !!autoRefreshSeconds}
              onClick={refreshAllData}
              className="rounded-r-none border-r-0"
            >
              {AUTO_REFRESH_OPTIONS.find((o) => o.seconds === autoRefreshSeconds)?.label ??
                'Refresh'}
            </Button>
            <Popover open={isRefreshMenuOpen} onOpenChange={setIsRefreshMenuOpen}>
              <PopoverTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  className="rounded-l-none px-1.5"
                  aria-label="Auto-refresh options"
                >
                  <ChevronDown className="size-3" />
                </Button>
              </PopoverTrigger>
              <PopoverContent align="end" className="w-36 p-1">
                {autoRefreshSeconds && (
                  <button
                    className="hover:bg-accent w-full rounded px-2 py-1.5 text-left text-sm font-medium"
                    onClick={() => {
                      setAutoRefreshSeconds(null)
                      setIsRefreshMenuOpen(false)
                    }}
                  >
                    Off
                  </button>
                )}
                {AUTO_REFRESH_OPTIONS.map((option) => (
                  <button
                    key={option.seconds}
                    className={cn(
                      'w-full rounded px-2 py-1.5 text-left text-sm',
                      autoRefreshSeconds === option.seconds
                        ? 'bg-accent font-medium'
                        : 'hover:bg-accent',
                    )}
                    onClick={() => {
                      setAutoRefreshSeconds(option.seconds)
                      setIsRefreshMenuOpen(false)
                    }}
                  >
                    Every {option.label}
                  </button>
                ))}
              </PopoverContent>
            </Popover>
          </div>
          <Button
            variant="outline"
            size="sm"
            icon={Download}
            onClick={() => window.open(auditExportUrl('csv', currentFilters), '_blank')}
            disabled={entries.length === 0}
          >
            CSV
          </Button>
          <Button
            variant="outline"
            size="sm"
            icon={Download}
            onClick={() => window.open(auditExportUrl('json', currentFilters), '_blank')}
            disabled={entries.length === 0}
          >
            JSON
          </Button>
          <Button
            variant="outline"
            size="sm"
            color="error"
            icon={Trash2}
            onClick={() => setIsDeleteOpen(true)}
            disabled={entries.length === 0}
          >
            Delete
          </Button>
        </div>
      </header>

      {/* Summary dashboard */}
      {summary && (
        <div className="border-b px-4 py-3">
          <div className="flex flex-wrap gap-6">
            <SummaryCard label="Sessions" value={summary.totalSessions} />
            <SummaryCard label="Tool Uses" value={summary.totalToolUses} />
            <SummaryCard label="Prompts" value={summary.totalPrompts} />
            <SummaryCard label="Total Cost" value={formatCost(summary.totalCostUsd)} />
            <SummaryCard label="Projects" value={summary.uniqueProjects} />
            <SummaryCard label="Worktrees" value={summary.uniqueWorktrees} />
            {summary.topTools.length > 0 && (
              <div className="flex flex-col gap-0.5">
                <span className="text-muted-foreground text-xs tracking-wide uppercase">
                  Top Tools
                </span>
                <div className="flex flex-wrap gap-1">
                  {summary.topTools.slice(0, 5).map((tool) => (
                    <Badge key={tool.name} variant="outline" className="font-mono text-xs">
                      {tool.name} ({tool.count})
                    </Badge>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Activity timeline */}
      <div className="border-b px-4 py-2">
        <ActivityTimeline
          entries={timelineEntries}
          since={since}
          until={until}
          onRangeChange={handleRangeChange}
        />
      </div>

      {/* Filter bar */}
      <div className="flex flex-wrap items-center gap-4 border-b px-4 py-2">
        {/* Category filters (multi-select) */}
        <div className="flex items-center gap-1">
          <span className="text-muted-foreground/60 mr-1 text-xs tracking-wide uppercase">
            Category
          </span>
          <button
            type="button"
            className={cn(
              'flex cursor-pointer items-center gap-1.5 rounded px-2.5 py-1 text-sm transition-colors',
              activeCategories.size === AUDIT_CATEGORIES.length && 'bg-muted font-medium',
            )}
            onClick={() => {
              const allSelected = activeCategories.size === AUDIT_CATEGORIES.length
              const next = allSelected
                ? new Set<AuditCategory>()
                : new Set(AUDIT_CATEGORIES.map((c) => c.id))
              setActiveCategories(next)
              writeStoredSet(STORAGE_KEY_CATEGORIES, next)
            }}
          >
            All
          </button>
          {AUDIT_CATEGORIES.map(({ id, label, icon: Icon, textStyle }) => {
            const isActive = activeCategories.has(id)
            return (
              <button
                key={id}
                type="button"
                className={cn(
                  'flex cursor-pointer items-center gap-1.5 rounded px-2.5 py-1 text-sm transition-colors',
                  textStyle,
                  isActive && 'bg-muted',
                )}
                onClick={() => {
                  const next = toggleSetValue(activeCategories, id)
                  setActiveCategories(next)
                  writeStoredSet(STORAGE_KEY_CATEGORIES, next)
                }}
              >
                <Icon className="h-3 w-3" />
                {label}
                <span className="text-muted-foreground text-xs">{categoryCounts[id] ?? 0}</span>
              </button>
            )
          })}
        </div>

        <div className="bg-border h-5 w-px" />

        {/* Level filters (multi-select with counts) */}
        <div className="flex items-center gap-1">
          <span className="text-muted-foreground/60 mr-1 text-xs tracking-wide uppercase">
            Level
          </span>
          {ALL_LEVELS.map((level) => (
            <FilterTab
              key={level}
              label={level}
              count={levelCounts[level]}
              active={activeLevels.has(level)}
              onClick={() => {
                const next = toggleSetValue(activeLevels, level)
                setActiveLevels(next)
                writeStoredSet(STORAGE_KEY_LEVELS, next)
              }}
              colorVar={levelColorVar[level]}
            />
          ))}
        </div>

        <div className="bg-border h-5 w-px" />

        {/* Global search */}
        <div className="relative flex items-center gap-1">
          <span className="text-muted-foreground/60 mr-1 text-xs tracking-wide uppercase">
            Search
          </span>
          <div className="relative">
            <Search className="text-muted-foreground/50 pointer-events-none absolute top-1/2 left-2 h-3 w-3 -translate-y-1/2" />
            <input
              type="text"
              value={globalFilter}
              onChange={(e) => setGlobalFilter(e.target.value)}
              placeholder="Filter events..."
              className="border-input bg-muted/30 placeholder:text-muted-foreground/40 focus:border-ring focus:ring-ring/50 h-7 w-52 rounded border py-0.5 pr-7 pl-7 text-xs transition-colors outline-none focus:ring-[2px]"
            />
            {globalFilter && (
              <button
                type="button"
                onClick={() => setGlobalFilter('')}
                className="text-muted-foreground hover:text-foreground absolute top-1/2 right-1.5 -translate-y-1/2 rounded p-0.5"
                title="Clear search"
              >
                <X className="h-3 w-3" />
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Audit log table */}
      <AuditLogTable
        entries={entries}
        isLoading={isLoading && entries.length === 0}
        activeCategories={activeCategories}
        activeLevels={activeLevels}
        globalFilter={globalFilter}
        onFilteredCountChange={setFilteredCount}
      />

      {/* Footer */}
      {entries.length > 0 && (
        <footer className="text-muted-foreground border-t px-4 py-2 text-xs">
          <span>
            Newest: {formatFullTimestamp(entries[0].ts)} — Oldest:{' '}
            {formatFullTimestamp(entries[entries.length - 1].ts)}
          </span>
        </footer>
      )}

      <DeleteAuditDialog
        open={isDeleteOpen}
        onOpenChange={setIsDeleteOpen}
        projectNames={projectNames}
        onDeleted={refreshAllData}
      />
    </div>
  )
}

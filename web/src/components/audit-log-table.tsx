import { compareItems, type RankingInfo, rankItem, rankings } from '@tanstack/match-sorter-utils'
import {
  type ColumnDef,
  type ColumnFiltersState,
  type ColumnSizingState,
  type ExpandedState,
  type FilterFn,
  flexRender,
  getCoreRowModel,
  getExpandedRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  type SortingFn,
  type SortingState,
  sortingFns,
  useReactTable,
} from '@tanstack/react-table'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

declare module '@tanstack/react-table' {
  interface FilterFns {
    fuzzy: FilterFn<unknown>
  }
  interface FilterMeta {
    itemRank: RankingInfo
  }
}

import { useVirtualizer } from '@tanstack/react-virtual'
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  ChevronDown,
  ChevronRight,
  Copy,
  Loader2,
  SquareTerminal,
} from 'lucide-react'
import { AgentIcon } from '@/components/ui/agent-icons'
import { Badge } from '@/components/ui/badge'
import {
  AUDIT_CATEGORIES,
  categoryOrder,
  copyEntry,
  entryKey,
  entryMessage,
  eventLabel,
  formatDataForDisplay,
  formatTimestamp,
  promptSource,
} from '@/lib/audit-log-utils'
import { readStorage, writeStorage } from '@/lib/storage'
import type { AgentType, AuditCategory, AuditLogEntry, AuditLogLevel } from '@/lib/types'
import { cn } from '@/lib/utils'

// --- Styles ---

/** Severity level badge styles. */
const levelStyles: Record<AuditLogLevel, string> = {
  info: 'bg-info/15 text-info border-info/30',
  warn: 'bg-warning/15 text-warning border-warning/30',
  error: 'bg-error/15 text-error border-error/30',
}

/** Badge style and uppercase label derived from shared category metadata. */
const categoryBadgeStyles: Record<string, string> = Object.fromEntries(
  AUDIT_CATEGORIES.map(({ id, badgeStyle }) => [id, badgeStyle]),
)
const categoryLabels: Record<string, string> = Object.fromEntries(
  AUDIT_CATEGORIES.map(({ id, label }) => [id, label.toUpperCase()]),
)

// --- localStorage persistence ---

const STORAGE_SORTING = 'audit:table:sorting'
const STORAGE_SIZING = 'audit:table:sizing'

// --- Fuzzy filter ---

/** Fuzzy filter using match-sorter-utils. Requires at least a substring (CONTAINS) match. */
const fuzzyFilter: FilterFn<AuditLogEntry> = (row, columnId, value, addMeta) => {
  const itemRank = rankItem(row.getValue(columnId), value, {
    threshold: rankings.CONTAINS,
  })
  addMeta({ itemRank })
  return itemRank.passed
}

/** Sorts by fuzzy match rank, falling back to alphanumeric when ranks are equal. */
const fuzzySort: SortingFn<AuditLogEntry> = (rowA, rowB, columnId) => {
  const metaA = rowA.columnFiltersMeta[columnId]
  const metaB = rowB.columnFiltersMeta[columnId]
  const dir = metaA?.itemRank && metaB?.itemRank ? compareItems(metaA.itemRank, metaB.itemRank) : 0
  return dir === 0 ? sortingFns.alphanumeric(rowA, rowB, columnId) : dir
}

// --- Column definitions ---

const columns: ColumnDef<AuditLogEntry, unknown>[] = [
  {
    id: 'search',
    accessorFn: (row) =>
      [
        row.event,
        row.projectId,
        row.agentType,
        row.containerName,
        row.worktree,
        row.msg,
        row.category,
        entryMessage(row),
      ]
        .filter(Boolean)
        .join(' '),
    header: '',
    enableSorting: false,
    enableResizing: false,
    enableHiding: false,
    enableGlobalFilter: false,
    filterFn: 'fuzzy',
    sortingFn: fuzzySort,
    size: 0,
    minSize: 0,
    maxSize: 0,
    cell: () => null,
  },
  {
    id: 'expand',
    header: '',
    size: 28,
    minSize: 28,
    maxSize: 28,
    enableSorting: false,
    enableResizing: false,
    enableHiding: false,
    enableGlobalFilter: false,
    cell: ({ row }) =>
      row.getCanExpand() ? (
        <span className="text-muted-foreground flex items-center justify-center">
          {row.getIsExpanded() ? (
            <ChevronDown className="h-4 w-4" />
          ) : (
            <ChevronRight className="h-4 w-4" />
          )}
        </span>
      ) : null,
  },
  {
    accessorKey: 'level',
    header: 'Level',
    size: 75,
    minSize: 60,
    sortingFn: (rowA, rowB) => {
      const order: Record<string, number> = { error: 2, warn: 1, info: 0 }
      return (
        (order[rowA.getValue<string>('level')] ?? 0) - (order[rowB.getValue<string>('level')] ?? 0)
      )
    },
    sortDescFirst: true,
    filterFn: (row, _columnId, filterValue: Set<string>) => filterValue.has(row.getValue('level')),
    cell: ({ getValue }) => {
      const level = getValue<AuditLogLevel>()
      return (
        <Badge
          className={`${levelStyles[level]} w-12 justify-center rounded border px-1 py-0 font-mono text-xs uppercase`}
        >
          {level}
        </Badge>
      )
    },
  },

  {
    accessorKey: 'ts',
    header: 'Timestamp',
    size: 130,
    minSize: 120,
    sortingFn: 'datetime',
    sortDescFirst: true,
    cell: ({ getValue }) => (
      <span className="text-muted-foreground">{formatTimestamp(getValue<string>())}</span>
    ),
  },
  {
    accessorKey: 'agentType',
    header: 'Agent',
    size: 70,
    sortUndefined: 'last',
    cell: ({ getValue }) => {
      const at = getValue<AgentType | undefined>()
      if (!at) return null
      return <AgentIcon type={at} className="h-4 w-4" />
    },
  },
  {
    accessorKey: 'projectId',
    header: 'Project ID',
    size: 110,
    minSize: 90,
    sortUndefined: 'last',
    cell: ({ getValue }) => (
      <span className="text-muted-foreground/60 truncate font-mono text-xs">
        {getValue<string>() ?? ''}
      </span>
    ),
  },
  {
    accessorKey: 'containerName',
    header: 'Name',
    minSize: 120,
    sortUndefined: 'last',
    cell: ({ getValue }) => (
      <span className="text-muted-foreground truncate">{getValue<string>() ?? ''}</span>
    ),
  },
  {
    accessorKey: 'worktree',
    header: 'Worktree',
    minSize: 120,
    sortUndefined: 'last',
    cell: ({ getValue }) => {
      const wt = getValue<string>()
      return <span className="text-muted-foreground/70 truncate">{wt ? `[${wt}]` : ''}</span>
    },
  },
  {
    id: 'category',
    accessorFn: (row) => row.category ?? 'system',
    header: 'Category',
    size: 100,
    minSize: 80,
    sortingFn: (rowA, rowB) =>
      (categoryOrder[rowA.getValue<string>('category')] ?? 7) -
      (categoryOrder[rowB.getValue<string>('category')] ?? 7),
    filterFn: (row, _columnId, filterValue: Set<string>) =>
      filterValue.has(row.getValue('category')),
    cell: ({ getValue }) => {
      const category = getValue<AuditCategory>()
      const badgeStyle = categoryBadgeStyles[category] ?? categoryBadgeStyles.system
      const label = categoryLabels[category] ?? category.toUpperCase()
      return (
        <Badge
          variant="outline"
          className={`${badgeStyle} w-18 justify-center rounded px-1 py-0 font-mono text-xs`}
        >
          {label}
        </Badge>
      )
    },
  },
  {
    accessorKey: 'event',
    header: 'Event',
    minSize: 220,
    cell: ({ getValue }) => (
      <span className="text-muted-foreground font-semibold">{eventLabel(getValue<string>())}</span>
    ),
  },
  {
    id: 'message',
    accessorFn: (row) => entryMessage(row),
    header: 'Message',
    enableSorting: false,
    meta: { flex: true },
    cell: ({ getValue, row }) => {
      const source = promptSource(row.original)
      if (source) {
        return (
          <span className="flex min-w-0 items-center gap-1.5">
            <SquareTerminal className="text-muted-foreground h-3.5 w-3.5 shrink-0" />
            <span className="text-foreground/50 truncate font-mono">{getValue<string>()}</span>
          </span>
        )
      }
      return <span className="text-foreground/80 truncate">{getValue<string>()}</span>
    },
  },
  {
    id: 'actions',
    header: '',
    size: 32,
    minSize: 32,
    maxSize: 32,
    enableSorting: false,
    enableResizing: false,
    enableHiding: false,
    enableGlobalFilter: false,
    cell: ({ row }) => (
      <button
        type="button"
        className="text-muted-foreground/0 group-hover:text-muted-foreground hover:text-foreground! p-0.5 transition-colors"
        onClick={(e) => {
          e.stopPropagation()
          copyEntry(row.original)
        }}
        title="Copy as JSON"
      >
        <Copy className="h-4 w-4 cursor-pointer" />
      </button>
    ),
  },
]

// --- Expanded row detail ---

/** Renders the expanded detail panel for a row. Bash prompts get a
 *  pre-formatted text block; other events show raw JSON data/attrs. */
function ExpandedRowDetail({ entry }: { entry: AuditLogEntry }) {
  const source = promptSource(entry)
  const prompt = entry.data?.prompt as string | undefined

  return (
    <td
      colSpan={999}
      className="border-border/30 animate-in fade-in slide-in-from-top-1 w-full border-t pr-2 pb-2 pl-16 duration-150"
    >
      {source && prompt ? (
        <pre className="text-muted-foreground overflow-x-auto text-xs leading-relaxed whitespace-pre select-text">
          {cleanTerminalOutput(prompt)}
        </pre>
      ) : (
        <pre className="text-muted-foreground text-xs leading-relaxed wrap-break-word whitespace-pre-wrap select-text">
          {entry.data && Object.keys(entry.data).length > 0 && (
            <span>
              <span className="text-foreground/60">data: </span>
              {JSON.stringify(formatDataForDisplay(entry.data), null, 2)}
              {'\n'}
            </span>
          )}
          {entry.attrs && Object.keys(entry.attrs).length > 0 && (
            <span>
              <span className="text-foreground/60">attrs: </span>
              {JSON.stringify(formatDataForDisplay(entry.attrs), null, 2)}
            </span>
          )}
        </pre>
      )}
    </td>
  )
}

/** Cleans terminal output for display. Handles carriage returns (\r)
 *  used by CLI tools like curl for progress bar overwrites — keeps
 *  only the final content after the last \r on each line. */
function cleanTerminalOutput(text: string): string {
  return text
    .split('\n')
    .map((line) => {
      const lastCR = line.lastIndexOf('\r')
      return lastCR >= 0 ? line.substring(lastCR + 1) : line
    })
    .join('\n')
}

// --- Sort indicator ---

/** Renders a sort direction icon for a column header. */
function SortIcon({ direction }: { direction: false | 'asc' | 'desc' }) {
  if (direction === 'asc') return <ArrowUp className="h-3 w-3" />
  if (direction === 'desc') return <ArrowDown className="h-3 w-3" />
  return <ArrowUpDown className="text-muted-foreground/40 h-3 w-3" />
}

// --- Component ---

interface AuditLogTableProps {
  /** All entries (unfiltered by category/level — the table handles that). */
  entries: AuditLogEntry[]
  /** Whether the initial data load is in progress. */
  isLoading: boolean
  /** Active category filter values. */
  activeCategories: Set<AuditCategory>
  /** Active level filter values. */
  activeLevels: Set<AuditLogLevel>
  /** Global fuzzy search query. */
  globalFilter: string
  /** Called when the filtered row count changes (for displaying in the page header). */
  onFilteredCountChange?: (count: number) => void
}

/** Virtualized audit log table with sorting, resizing, filtering, and expandable rows. */
export function AuditLogTable({
  entries,
  isLoading,
  activeCategories,
  activeLevels,
  globalFilter,
  onFilteredCountChange,
}: AuditLogTableProps) {
  // Derive column filters from external filter state.
  const columnFilters = useMemo<ColumnFiltersState>(() => {
    const filters: ColumnFiltersState = [
      { id: 'category', value: activeCategories },
      { id: 'level', value: activeLevels },
    ]
    if (globalFilter) {
      filters.push({ id: 'search', value: globalFilter })
    }
    return filters
  }, [activeCategories, activeLevels, globalFilter])

  const [sorting, setSorting] = useState<SortingState>(() => readStorage(STORAGE_SORTING, []))
  const [columnSizing, setColumnSizing] = useState<ColumnSizingState>(() =>
    readStorage(STORAGE_SIZING, {}),
  )
  const [expanded, setExpanded] = useState<ExpandedState>({})

  const handleSortingChange = useCallback(
    (updater: SortingState | ((prev: SortingState) => SortingState)) => {
      setSorting((prev) => {
        const next = typeof updater === 'function' ? updater(prev) : updater
        writeStorage(STORAGE_SORTING, next)
        return next
      })
    },
    [],
  )

  /** Debounced persistence for column sizing — avoids writing on every mousemove frame. */
  const sizingWriteTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => {
    return () => {
      if (sizingWriteTimer.current) clearTimeout(sizingWriteTimer.current)
    }
  }, [])

  const handleSizingChange = useCallback(
    (updater: ColumnSizingState | ((prev: ColumnSizingState) => ColumnSizingState)) => {
      setColumnSizing((prev) => {
        const next = typeof updater === 'function' ? updater(prev) : updater
        if (sizingWriteTimer.current) clearTimeout(sizingWriteTimer.current)
        sizingWriteTimer.current = setTimeout(() => writeStorage(STORAGE_SIZING, next), 300)
        return next
      })
    },
    [],
  )

  const table = useReactTable({
    data: entries,
    columns,
    filterFns: { fuzzy: fuzzyFilter },
    state: { columnFilters, sorting, columnSizing, expanded },
    onSortingChange: handleSortingChange,
    onColumnSizingChange: handleSizingChange,
    onExpandedChange: setExpanded,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getExpandedRowModel: getExpandedRowModel(),
    getRowCanExpand: (row) =>
      (row.original.data != null && Object.keys(row.original.data).length > 0) ||
      (row.original.attrs != null && Object.keys(row.original.attrs).length > 0),
    getRowId: (row) => entryKey(row),
    columnResizeMode: 'onChange',
  })

  const { rows } = table.getRowModel()

  useEffect(() => {
    onFilteredCountChange?.(rows.length)
  }, [rows.length, onFilteredCountChange])

  /** Track mousedown position to distinguish clicks from text-selection drags. */
  const mouseDownPos = useRef<{ x: number; y: number } | null>(null)
  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    mouseDownPos.current = { x: e.clientX, y: e.clientY }
  }, [])
  const isDrag = useCallback((e: React.MouseEvent) => {
    if (!mouseDownPos.current) return false
    const dx = e.clientX - mouseDownPos.current.x
    const dy = e.clientY - mouseDownPos.current.y
    return Math.abs(dx) > 4 || Math.abs(dy) > 4
  }, [])

  const scrollContainerRef = useRef<HTMLDivElement>(null)
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => 33,
    overscan: 5,
    measureElement:
      typeof window !== 'undefined' && !navigator.userAgent.includes('Firefox')
        ? (element) => element?.getBoundingClientRect().height
        : undefined,
  })

  /** Re-measure all rows when expansion state changes so positions update. */
  useEffect(() => {
    virtualizer.measure()
  }, [virtualizer])

  if (isLoading) {
    return (
      <div className="text-muted-foreground flex flex-1 items-center justify-center py-12">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        Loading...
      </div>
    )
  }

  if (entries.length === 0) {
    return (
      <div className="text-muted-foreground flex flex-1 items-center justify-center py-12">
        No audit events recorded yet. Enable audit logging in Settings to start capturing activity.
      </div>
    )
  }

  return (
    <div ref={scrollContainerRef} className="flex-1 overflow-auto select-text">
      {/* CSS grid layout on table elements is required for virtualization — see TanStack Table virtualization guide */}
      <table className="grid">
        <thead className="bg-background sticky top-0 z-1 grid border-b">
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id} className="flex w-full">
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  className="text-muted-foreground relative flex h-9 items-center px-2 text-left text-sm font-medium"
                  style={
                    (header.column.columnDef.meta as Record<string, unknown>)?.flex
                      ? { flex: 1, minWidth: header.column.columnDef.minSize }
                      : { width: header.getSize() }
                  }
                >
                  {header.isPlaceholder ? null : (
                    <div
                      className={cn(
                        'flex items-center gap-1.5',
                        header.column.getCanSort() && 'cursor-pointer select-none',
                      )}
                      onClick={header.column.getToggleSortingHandler()}
                    >
                      {flexRender(header.column.columnDef.header, header.getContext())}
                      {header.column.getCanSort() && (
                        <SortIcon direction={header.column.getIsSorted()} />
                      )}
                    </div>
                  )}
                  {header.column.getCanResize() && (
                    <div
                      onMouseDown={header.getResizeHandler()}
                      onTouchStart={header.getResizeHandler()}
                      onDoubleClick={() => header.column.resetSize()}
                      className={cn(
                        'absolute top-0 -right-1 h-full w-2 cursor-col-resize touch-none select-none',
                        header.column.getIsResizing() ? 'bg-primary' : 'hover:bg-primary/50',
                      )}
                    />
                  )}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody
          className="relative grid font-mono text-xs"
          style={{ height: `${virtualizer.getTotalSize()}px` }}
        >
          {virtualizer.getVirtualItems().map((virtualRow) => {
            const row = rows[virtualRow.index]
            return (
              <tr
                key={row.id}
                ref={(node) => virtualizer.measureElement(node)}
                data-index={virtualRow.index}
                className={cn(
                  'group border-border/30 hover:bg-muted/30 absolute flex w-full flex-wrap border-b',
                  row.getCanExpand() && 'cursor-pointer',
                )}
                onMouseDown={handleMouseDown}
                onClick={
                  row.getCanExpand()
                    ? (e) => {
                        if (isDrag(e)) return
                        row.toggleExpanded()
                      }
                    : undefined
                }
                style={{ transform: `translateY(${virtualRow.start}px)` }}
              >
                {row.getVisibleCells().map((cell) => (
                  <td
                    key={cell.id}
                    className="flex items-center overflow-hidden px-2 py-1.5"
                    style={
                      (cell.column.columnDef.meta as Record<string, unknown>)?.flex
                        ? { flex: 1, minWidth: cell.column.columnDef.minSize }
                        : { width: cell.column.getSize() }
                    }
                  >
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
                {row.getIsExpanded() && <ExpandedRowDetail entry={row.original} />}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

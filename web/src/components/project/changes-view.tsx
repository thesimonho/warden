import { useState, useMemo } from 'react'
import { DiffView, DiffModeEnum } from '@git-diff-view/react'
import '@git-diff-view/react/styles/diff-view.css'
import { RefreshCw, ChevronRight, FileText, FilePlus, FileMinus, FileEdit } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'
import { useDiff } from '@/hooks/use-diff'
import { useTheme } from '@/hooks/use-theme'
import type { DiffFileSummary } from '@/lib/types'

/** Props for the ChangesView component. */
interface ChangesViewProps {
  projectId: string
  worktreeId: string
}

/** Status badge variant and label for a file change status. */
const statusConfig: Record<
  DiffFileSummary['status'],
  { label: string; variant: 'success' | 'warning' | 'error' | 'secondary'; icon: React.ElementType }
> = {
  added: { label: 'A', variant: 'success', icon: FilePlus },
  modified: { label: 'M', variant: 'warning', icon: FileEdit },
  deleted: { label: 'D', variant: 'error', icon: FileMinus },
  renamed: { label: 'R', variant: 'secondary', icon: FileText },
}

/**
 * Splits a raw unified diff into a map of file path to per-file diff chunk.
 * Matches both `b/<path>` (added/modified/renamed) and `a/<path>` (deleted)
 * in the diff header to handle all change types.
 */
function buildFileDiffMap(rawDiff: string, files: DiffFileSummary[]): Map<string, string> {
  const chunks = rawDiff.split(/^(?=diff --git )/m)
  const map = new Map<string, string>()

  for (const file of files) {
    const chunk = chunks.find((c) => c.includes(`b/${file.path}`) || c.includes(`a/${file.path}`))
    if (chunk) {
      map.set(file.path, chunk)
    }
  }

  return map
}

/**
 * Displays uncommitted changes for a worktree — file list with +/- stats
 * and click-to-expand per-file diffs.
 *
 * All files are collapsed by default so the user can scan the file list
 * first, then drill into specific files.
 */
export function ChangesView({ projectId, worktreeId }: ChangesViewProps) {
  const { diff, isLoading, error, refetch } = useDiff(projectId, worktreeId, true)
  const [expandedFiles, setExpandedFiles] = useState<Set<string>>(new Set())
  const { resolvedTheme } = useTheme()
  const isDark = resolvedTheme === 'dark'

  // Split rawDiff once — shared across all FileEntry components.
  const fileDiffMap = useMemo(
    () => (diff ? buildFileDiffMap(diff.rawDiff, diff.files) : new Map<string, string>()),
    [diff],
  )

  /** Toggle a file's expanded state. */
  function toggleFile(path: string) {
    setExpandedFiles((prev) => {
      const next = new Set(prev)
      if (next.has(path)) {
        next.delete(path)
      } else {
        next.add(path)
      }
      return next
    })
  }

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <RefreshCw className="text-muted-foreground h-5 w-5 animate-spin" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2">
        <p className="text-error text-sm">{error}</p>
        <Button variant="outline" size="sm" icon={RefreshCw} onClick={refetch}>
          Retry
        </Button>
      </div>
    )
  }

  if (!diff || diff.files.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2">
        <p className="text-muted-foreground text-sm">No uncommitted changes</p>
        <Button variant="outline" size="sm" icon={RefreshCw} onClick={refetch}>
          Refresh
        </Button>
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="border-border flex shrink-0 items-center justify-between border-b px-3 py-1.5">
        <div className="flex items-center gap-2 text-sm">
          <span>
            {diff.files.length} file{diff.files.length !== 1 ? 's' : ''} changed
          </span>
          <span className="text-success">+{diff.totalAdditions}</span>
          <span className="text-error">-{diff.totalDeletions}</span>
          {diff.truncated && <Badge variant="warning">Diff truncated</Badge>}
        </div>
        <Button
          variant="ghost"
          size="icon-xs"
          icon={RefreshCw}
          loading={isLoading}
          onClick={refetch}
          title="Refresh diff"
        />
      </div>

      {/* File list */}
      <ScrollArea className="min-h-0 flex-1">
        <div className="divide-border divide-y">
          {diff.files.map((file) => (
            <FileEntry
              key={file.path}
              file={file}
              isExpanded={expandedFiles.has(file.path)}
              fileDiff={fileDiffMap.get(file.path)}
              isDark={isDark}
              onToggle={() => toggleFile(file.path)}
            />
          ))}
        </div>
      </ScrollArea>
    </div>
  )
}

/** Props for a single file entry in the changes list. */
interface FileEntryProps {
  file: DiffFileSummary
  isExpanded: boolean
  fileDiff: string | undefined
  isDark: boolean
  onToggle: () => void
}

/** A single file row with expandable inline diff. */
function FileEntry({ file, isExpanded, fileDiff, isDark, onToggle }: FileEntryProps) {
  const config = statusConfig[file.status]

  return (
    <div>
      <button
        type="button"
        className="hover:bg-accent/50 flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm"
        onClick={onToggle}
      >
        <ChevronRight
          className={cn(
            'text-muted-foreground h-3.5 w-3.5 shrink-0 transition-transform',
            isExpanded && 'rotate-90',
          )}
        />
        <config.icon className="text-muted-foreground h-3.5 w-3.5 shrink-0" />
        <span className="min-w-0 flex-1 truncate font-mono text-sm">
          {file.path}
          {file.oldPath && <span className="text-muted-foreground"> (from {file.oldPath})</span>}
        </span>
        {!file.isBinary && (
          <span className="flex shrink-0 items-center gap-1.5 font-mono text-sm">
            <span className="text-success">+{file.additions}</span>
            <span className="text-error">-{file.deletions}</span>
          </span>
        )}
        <Badge variant={config.variant} className="text-xxs shrink-0">
          {config.label}
        </Badge>
      </button>

      {isExpanded && !file.isBinary && (
        <FileDiffPanel fileDiff={fileDiff} filePath={file.path} isDark={isDark} />
      )}
      {isExpanded && file.isBinary && (
        <div className="bg-muted/30 text-muted-foreground px-6 py-3 text-sm">
          Binary file — diff not available
        </div>
      )}
    </div>
  )
}

/** Props for the inline diff panel. */
interface FileDiffPanelProps {
  fileDiff: string | undefined
  filePath: string
  isDark: boolean
}

/** Renders the inline unified diff for a single file using @git-diff-view/react. */
function FileDiffPanel({ fileDiff, filePath, isDark }: FileDiffPanelProps) {
  if (!fileDiff) {
    return (
      <div className="bg-muted/30 text-muted-foreground px-6 py-3 text-sm">
        Diff content not available
      </div>
    )
  }

  // Pass the entire file diff chunk as a single hunk — the library
  // handles internal hunk splitting correctly.
  return (
    <div className="border-border overflow-hidden border-t text-sm">
      <DiffView
        data={{
          newFile: { fileName: filePath, content: '' },
          oldFile: { fileName: filePath, content: '' },
          hunks: [fileDiff],
        }}
        diffViewMode={DiffModeEnum.Unified}
        diffViewTheme={isDark ? 'dark' : 'light'}
        diffViewHighlight
        diffViewWrap
        diffViewFontSize={12}
      />
    </div>
  )
}

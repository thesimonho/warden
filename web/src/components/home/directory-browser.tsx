import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ChevronRight, CornerLeftUp, Folder, FolderOpen, Loader2, X } from 'lucide-react'
import { listDirectories } from '@/lib/api'
import type { DirEntry } from '@/lib/types'

/** Props for the DirectoryBrowser component. */
interface DirectoryBrowserProps {
  /** Current selected path (always absolute). */
  value: string
  /** Callback when the user commits a directory (always absolute). */
  onChange: (path: string) => void
  /** Whether the browser is disabled. */
  disabled?: boolean
  /** User's home directory — used as fallback browse path. */
  defaultPath?: string
  /** Placeholder text shown when no path is selected. */
  placeholder?: string
}

/** Starting path for the directory browser. */
const ROOT_DIR = '/'

/**
 * Fuzzy-finder style directory picker.
 *
 * Displays a split input: a non-editable prefix showing the current directory,
 * and an editable filter input for searching subdirectories. The dropdown lists
 * children of the current directory filtered by case-insensitive contains match.
 *
 * Backspace when the filter is empty navigates up one directory level.
 * Enter on a highlighted item navigates into that directory.
 * Enter with no highlight commits the current directory path.
 * Escape reverts to the value from when the field was focused.
 *
 * Internal state is decoupled from the parent — `onChange` only fires on
 * explicit commit, not during browsing or filtering.
 */
export default function DirectoryBrowser({
  value,
  onChange,
  disabled,
  defaultPath,
  placeholder = '/path/to/directory',
}: DirectoryBrowserProps) {
  const fallbackPath = defaultPath || ROOT_DIR

  /** The directory currently being browsed (absolute path). */
  const [browseDir, setBrowseDir] = useState(() => value || fallbackPath)
  /** Filter text typed by the user to narrow the directory listing. */
  const [filter, setFilter] = useState('')
  /** Snapshot of the committed value when the dropdown opened, for Escape revert. */
  const [savedValue, setSavedValue] = useState(value)
  /** Raw directory listing from the API for browseDir. */
  const [directories, setDirectories] = useState<DirEntry[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [isOpen, setIsOpen] = useState(false)
  const [highlightIndex, setHighlightIndex] = useState(-1)
  const [openAbove, setOpenAbove] = useState(false)

  const containerRef = useRef<HTMLDivElement>(null)
  const filterRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const lastFetchedDir = useRef('')

  /** Sync browseDir from prop when dropdown is closed (external updates). */
  useEffect(() => {
    if (!isOpen && value) {
      setBrowseDir(value)
    }
  }, [value, isOpen])

  /** Update browseDir when defaultPath arrives asynchronously. */
  useEffect(() => {
    if (defaultPath && !value && !isOpen && browseDir === ROOT_DIR) {
      setBrowseDir(defaultPath)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-run when defaultPath changes
  }, [defaultPath])

  /** Filter directories by case-insensitive contains match. */
  const filteredDirectories = useMemo(() => {
    if (!filter) return directories
    const needle = filter.toLowerCase()
    return directories.filter((dir) => dir.name.toLowerCase().includes(needle))
  }, [directories, filter])

  /** Fetches directory listing for the given path. */
  const fetchDirectories = useCallback(async (path: string) => {
    setIsLoading(true)
    try {
      const dirs = await listDirectories(path)
      setDirectories(dirs)
      lastFetchedDir.current = path
    } catch {
      setDirectories([])
      lastFetchedDir.current = path
    } finally {
      setIsLoading(false)
    }
  }, [])

  /** Fetch when browseDir changes while dropdown is open. */
  useEffect(() => {
    if (!isOpen) return
    if (browseDir === lastFetchedDir.current) return
    fetchDirectories(browseDir)
  }, [browseDir, isOpen, fetchDirectories])

  /** Immediate fetch on open. */
  useEffect(() => {
    if (isOpen) {
      lastFetchedDir.current = ''
      fetchDirectories(browseDir)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- fetch once on open, not on browseDir/fetchDirectories changes
  }, [isOpen])

  /** Commits the current browseDir as the selected value. */
  const commitValue = useCallback(() => {
    if (browseDir !== value) {
      onChange(browseDir)
    }
    setIsOpen(false)
    setHighlightIndex(-1)
    setFilter('')
  }, [browseDir, value, onChange])

  /** Reverts to the saved value and closes. */
  const revertAndClose = useCallback(() => {
    setBrowseDir(savedValue || fallbackPath)
    setIsOpen(false)
    setHighlightIndex(-1)
    setFilter('')
  }, [savedValue, fallbackPath])

  /** Navigates into a subdirectory. */
  const navigateInto = useCallback((dir: DirEntry) => {
    setBrowseDir(dir.path)
    setFilter('')
    setHighlightIndex(-1)
    requestAnimationFrame(() => filterRef.current?.focus())
  }, [])

  /** Navigates up to the parent directory. */
  const navigateUp = useCallback(() => {
    setBrowseDir((prev) => {
      if (prev === '/') return prev
      const parent = prev.replace(/\/[^/]+\/?$/, '') || '/'
      return parent
    })
    setFilter('')
    setHighlightIndex(-1)
  }, [])

  /** Clears the selection and resets to the fallback path. */
  const clearValue = useCallback(() => {
    onChange('')
    setBrowseDir(fallbackPath)
    setIsOpen(false)
    setHighlightIndex(-1)
    setFilter('')
  }, [onChange, fallbackPath])

  /** Close dropdown and commit when clicking outside. */
  useEffect(() => {
    if (!isOpen) return

    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        if (browseDir !== value) {
          onChange(browseDir)
        }
        setIsOpen(false)
        setHighlightIndex(-1)
        setFilter('')
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [isOpen, browseDir, onChange, value])

  /** Scroll highlighted item into view. */
  useEffect(() => {
    if (highlightIndex < 0 || !listRef.current) return
    const items = listRef.current.querySelectorAll('[data-dir-item]')
    items[highlightIndex]?.scrollIntoView({ block: 'nearest' })
  }, [highlightIndex])

  /**
   * Handles keyboard navigation in the filter input.
   *
   * Highlight indices: -1 = nothing, 0 = .. row (when present),
   * dirOffset..totalItems-1 = directory entries.
   */
  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    switch (e.key) {
      case 'Backspace': {
        if (filter === '') {
          e.preventDefault()
          navigateUp()
        }
        break
      }
      case 'ArrowDown': {
        e.preventDefault()
        setHighlightIndex((prev) => (prev < totalItems - 1 ? prev + 1 : 0))
        break
      }
      case 'ArrowUp': {
        e.preventDefault()
        setHighlightIndex((prev) => (prev > 0 ? prev - 1 : totalItems - 1))
        break
      }
      case 'Enter': {
        e.preventDefault()
        if (highlightIndex >= 0 && highlightIndex < totalItems) {
          if (canNavigateUp && highlightIndex === 0) {
            navigateUp()
          } else {
            navigateInto(filteredDirectories[highlightIndex - dirOffset])
          }
        } else {
          commitValue()
        }
        break
      }
      case 'Escape': {
        e.preventDefault()
        revertAndClose()
        break
      }
    }
  }

  /** Opens the dropdown and snapshots the current value. */
  const handleOpen = () => {
    if (disabled) return
    setSavedValue(value)
    if (!browseDir || browseDir === ROOT_DIR) {
      setBrowseDir(fallbackPath)
    }

    setIsOpen(true)

    requestAnimationFrame(() => {
      if (containerRef.current) {
        const rect = containerRef.current.getBoundingClientRect()
        const spaceBelow = window.innerHeight - rect.bottom
        const dropdownHeight = 320
        setOpenAbove(spaceBelow < dropdownHeight && rect.top > spaceBelow)
      }
      filterRef.current?.focus()
    })
  }

  /** Whether the parent directory row should be shown (not at root). */
  const canNavigateUp = browseDir !== '/'

  /** Total number of selectable items in the dropdown (.. row + directories). */
  const totalItems = (canNavigateUp ? 1 : 0) + filteredDirectories.length

  /** Index offset: directory items start after the .. row when present. */
  const dirOffset = canNavigateUp ? 1 : 0

  return (
    <div ref={containerRef} className="relative flex-1">
      {/* Closed state: show committed value as a clickable field */}
      {!isOpen && (
        <div className="border-input focus-within:border-ring focus-within:ring-ring/50 dark:bg-input/30 flex h-9 w-full items-center rounded border bg-transparent shadow-xs transition-[color,box-shadow] focus-within:ring-[3px]">
          <button
            type="button"
            onClick={handleOpen}
            disabled={disabled}
            className="flex min-w-0 flex-1 cursor-text items-center px-3 font-mono text-sm disabled:cursor-not-allowed disabled:opacity-50"
          >
            {value ? (
              <span className="truncate">{value}</span>
            ) : (
              <span className="text-muted-foreground truncate">{placeholder}</span>
            )}
          </button>
          {value && !disabled && (
            <button
              type="button"
              onClick={clearValue}
              className="text-error hover:text-error/80 shrink-0 px-2"
              title="Clear selection"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      )}

      {/* Open state: split input with static prefix + editable filter */}
      {isOpen && (
        <>
          <div className="border-ring ring-ring/50 dark:bg-input/30 flex h-9 w-full items-center rounded border bg-transparent shadow-xs ring-[3px] transition-[color,box-shadow]">
            <div className="flex min-w-0 flex-1 items-center px-3 font-mono text-sm">
              <span className="text-muted-foreground shrink-0">
                {browseDir}
                {browseDir !== '/' && '/'}
              </span>
              <input
                ref={filterRef}
                type="text"
                value={filter}
                onChange={(e) => {
                  setFilter(e.target.value)
                  setHighlightIndex(-1)
                }}
                onKeyDown={handleKeyDown}
                className="min-w-0 flex-1 bg-transparent outline-none"
                autoComplete="off"
                spellCheck={false}
              />
            </div>
            <button
              type="button"
              onClick={clearValue}
              className="text-error hover:text-error/80 shrink-0 px-2"
              title="Clear selection"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>

          <div
            className={`bg-popover absolute left-0 z-50 w-md rounded border shadow-md ${openAbove ? 'bottom-full mb-1' : 'top-full mt-1'}`}
          >
            <div ref={listRef} className="max-h-72 overflow-y-auto overscroll-contain">
              {/* Parent directory row */}
              {canNavigateUp && (
                <button
                  type="button"
                  data-dir-item
                  onClick={navigateUp}
                  onMouseEnter={() => setHighlightIndex(0)}
                  onMouseLeave={() => setHighlightIndex(-1)}
                  className={`text-muted-foreground flex w-full items-center gap-2 px-3 py-1.5 text-left ${
                    highlightIndex === 0 ? 'bg-accent text-accent-foreground' : ''
                  }`}
                >
                  <CornerLeftUp className="h-3.5 w-3.5 shrink-0" />
                  <span className="text-sm">..</span>
                </button>
              )}

              {isLoading && (
                <div className="flex items-center justify-center py-4">
                  <Loader2 className="text-muted-foreground h-4 w-4 animate-spin" />
                </div>
              )}

              {!isLoading && filteredDirectories.length === 0 && (
                <p className="text-muted-foreground px-3 py-2 text-sm">
                  {directories.length > 0 ? 'No matches' : 'No subdirectories'}
                </p>
              )}

              {!isLoading &&
                filteredDirectories.map((dir, index) => {
                  const itemIndex = index + dirOffset
                  const isHighlighted = itemIndex === highlightIndex
                  return (
                    <button
                      key={dir.path}
                      type="button"
                      data-dir-item
                      onClick={() => navigateInto(dir)}
                      onMouseEnter={() => setHighlightIndex(itemIndex)}
                      onMouseLeave={() => setHighlightIndex(-1)}
                      className={`group flex w-full items-center gap-2 px-3 py-1.5 text-left ${
                        isHighlighted ? 'bg-accent text-accent-foreground' : ''
                      }`}
                    >
                      <Folder
                        className={`text-muted-foreground h-3.5 w-3.5 shrink-0 ${
                          isHighlighted ? 'hidden' : 'group-hover:hidden'
                        }`}
                      />
                      <FolderOpen
                        className={`text-muted-foreground h-3.5 w-3.5 shrink-0 ${
                          isHighlighted ? 'block' : 'hidden group-hover:block'
                        }`}
                      />
                      <span className="min-w-0 flex-1 truncate">{dir.name}</span>
                      <ChevronRight className="text-muted-foreground h-3.5 w-3.5 shrink-0" />
                    </button>
                  )
                })}
            </div>
          </div>
        </>
      )}
    </div>
  )
}

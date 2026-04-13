import { Search, X } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'

/** Props for the project name filter. */
export interface ProjectFilterProps {
  projectNames: string[]
  activeProject: string
  onSelect: (project: string) => void
}

/**
 * Text input with dropdown autocomplete for filtering by project name.
 *
 * Shows matching project names as the user types. Selecting a name
 * applies a server-side filter; clearing the input removes it.
 */
export function ProjectFilter({ projectNames, activeProject, onSelect }: ProjectFilterProps) {
  const [query, setQuery] = useState(activeProject)
  const [isOpen, setIsOpen] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  /** Sync query when activeProject changes externally (e.g. from localStorage). */
  useEffect(() => {
    setQuery(activeProject)
  }, [activeProject])

  /** Case-insensitive substring match. */
  const matches = useMemo(() => {
    if (!query) return projectNames
    const lower = query.toLowerCase()
    return projectNames.filter((name) => name.toLowerCase().includes(lower))
  }, [projectNames, query])

  /** Selects a project and closes the dropdown. */
  const selectProject = (name: string) => {
    setQuery(name)
    setIsOpen(false)
    onSelect(name)
  }

  /** Clears the project filter. */
  const clearFilter = () => {
    setQuery('')
    setIsOpen(false)
    onSelect('')
  }

  /** Commits the current query as a filter on blur or Enter. */
  const commitQuery = () => {
    if (query === '') {
      onSelect('')
    } else if (projectNames.includes(query)) {
      onSelect(query)
    } else {
      setQuery(activeProject)
    }
    setIsOpen(false)
  }

  /** Close dropdown on outside click. */
  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsOpen(false)
        setQuery(activeProject)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [activeProject])

  return (
    <div ref={containerRef} className="relative flex items-center gap-1">
      <span className="text-muted-foreground/60 mr-1 text-xs tracking-wide uppercase">Project</span>
      <div className="relative">
        <Search className="text-muted-foreground/50 pointer-events-none absolute top-1/2 left-2 h-3 w-3 -translate-y-1/2" />
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setIsOpen(true)
          }}
          onFocus={() => setIsOpen(true)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              if (matches.length === 1) {
                selectProject(matches[0])
              } else {
                commitQuery()
              }
            }
            if (e.key === 'Escape') {
              setQuery(activeProject)
              setIsOpen(false)
              inputRef.current?.blur()
            }
          }}
          placeholder="All projects"
          className="border-input bg-muted/30 placeholder:text-muted-foreground/40 focus:border-ring focus:ring-ring/50 h-7 w-44 rounded border py-0.5 pr-7 pl-7 text-xs transition-colors outline-none focus:ring-[2px]"
        />
        {activeProject && (
          <button
            type="button"
            onClick={clearFilter}
            className="text-muted-foreground hover:text-foreground absolute top-1/2 right-1.5 -translate-y-1/2 rounded p-0.5"
            title="Clear project filter"
          >
            <X className="h-3 w-3" />
          </button>
        )}
      </div>
      {isOpen && matches.length > 0 && (
        <div className="bg-popover border-border absolute top-full left-0 z-50 mt-1 max-h-48 w-56 overflow-y-auto rounded border shadow-md">
          {matches.map((name) => (
            <button
              key={name}
              type="button"
              className={`hover:bg-muted w-full px-3 py-1.5 text-left text-xs ${
                name === activeProject ? 'bg-muted font-medium' : ''
              }`}
              onMouseDown={(e) => {
                e.preventDefault()
                selectProject(name)
              }}
            >
              {name}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

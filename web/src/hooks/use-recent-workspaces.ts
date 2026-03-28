import { useCallback, useState } from 'react'

const SESSION_KEY = 'recent-workspaces'
const MAX_ENTRIES = 5

/** A saved workspace entry in sessionStorage. */
export interface RecentWorkspace {
  /** Sorted project IDs that make up this workspace. */
  ids: string[]
  savedAt: number
}

/** Reads recent workspaces from sessionStorage, returning an empty array on parse failure. */
function readFromStorage(): RecentWorkspace[] {
  try {
    const raw = sessionStorage.getItem(SESSION_KEY)
    return raw ? (JSON.parse(raw) as RecentWorkspace[]) : []
  } catch {
    return []
  }
}

/** Writes recent workspaces to sessionStorage. */
function writeToStorage(entries: RecentWorkspace[]): void {
  sessionStorage.setItem(SESSION_KEY, JSON.stringify(entries))
}

/** Normalizes an ID list to a sorted, deduped key for comparison. */
function toKey(ids: string[]): string {
  return [...new Set(ids)].sort().join(',')
}

/** Return type for the useRecentWorkspaces hook. */
interface UseRecentWorkspacesResult {
  recentWorkspaces: RecentWorkspace[]
  addWorkspace: (ids: string[]) => void
}

/**
 * Manages a session-scoped list of recently opened workspaces.
 *
 * Persists to sessionStorage so workspaces survive navigation within a tab
 * but are cleared when the tab is closed. Deduplicates by ID set and caps
 * at 5 entries (most recent first).
 *
 * @returns The current list and a function to record a new workspace visit.
 */
export function useRecentWorkspaces(): UseRecentWorkspacesResult {
  const [recentWorkspaces, setRecentWorkspaces] = useState<RecentWorkspace[]>(readFromStorage)

  const addWorkspace = useCallback((ids: string[]) => {
    if (ids.length === 0) return
    const key = toKey(ids)
    const existing = readFromStorage().filter((entry) => toKey(entry.ids) !== key)
    const updated = [{ ids: [...new Set(ids)].sort(), savedAt: Date.now() }, ...existing].slice(
      0,
      MAX_ENTRIES,
    )
    writeToStorage(updated)
    setRecentWorkspaces(updated)
  }, [])

  return { recentWorkspaces, addWorkspace }
}

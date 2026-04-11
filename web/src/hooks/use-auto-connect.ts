/**
 * Auto-connects worktrees specified via URL query parameters.
 *
 * When a project page is loaded with `?worktrees=wid1,wid2`, this hook
 * waits for the worktree list to load, then triggers `onAddPanel` for
 * each matching worktree. The query parameter is consumed once and
 * removed from the URL.
 *
 * Used by the system tray and desktop notifications to deep-link into
 * specific worktrees that need attention.
 *
 * @module
 */
import { useEffect, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'
import type { Worktree } from '@/lib/types'

/** Options for the auto-connect hook. */
interface UseAutoConnectOptions {
  /** Available worktrees from the API. */
  worktrees: Worktree[]
  /** Whether the worktree list is still loading. */
  isLoading: boolean
  /** Callback to add and connect a worktree panel. */
  onAddPanel: (worktree: Worktree) => void
}

/**
 * Reads `?worktrees=` from the URL, waits for the worktree list to load,
 * then auto-connects each matching worktree by calling `onAddPanel`.
 *
 * The query parameter is cleared after processing to prevent re-triggering
 * on subsequent renders.
 */
export function useAutoConnect({ worktrees, isLoading, onAddPanel }: UseAutoConnectOptions): void {
  const [searchParams, setSearchParams] = useSearchParams()
  const processedRef = useRef(false)
  const onAddPanelRef = useRef(onAddPanel)

  // Keep callback ref current without re-triggering the effect.
  useEffect(() => {
    onAddPanelRef.current = onAddPanel
  }, [onAddPanel])

  useEffect(() => {
    // Wait until worktrees have actually loaded — isLoading may flip
    // to false before the startTransition in useWorktrees commits the
    // worktree array, so we also check for a non-empty list.
    if (isLoading || worktrees.length === 0 || processedRef.current) return

    const raw = searchParams.get('worktrees')
    if (!raw) {
      processedRef.current = true
      return
    }

    processedRef.current = true
    const ids = new Set(raw.split(',').filter(Boolean))
    const toConnect = worktrees.filter((wt) => ids.has(wt.id))

    for (const wt of toConnect) {
      onAddPanelRef.current(wt)
    }

    // Clear the query param so it doesn't re-trigger.
    setSearchParams(
      (prev) => {
        prev.delete('worktrees')
        return prev
      },
      { replace: true },
    )
  }, [worktrees, isLoading, searchParams, setSearchParams])
}

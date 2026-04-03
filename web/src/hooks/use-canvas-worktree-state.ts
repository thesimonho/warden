import { useCallback, useEffect, useRef, useState } from 'react'
import { fetchWorktrees } from '@/lib/api'
import {
  deriveStateLabel,
  deriveWorktreeStateFromEvent,
  hasActiveTerminal,
  worktreeStateIndicator,
} from '@/lib/types'
import type { CanvasPanel } from '@/lib/canvas-store'
import type { WorktreeState, WorktreeStateEvent } from '@/lib/types'
import { getAttentionConfig } from '@/lib/notification-config'
import { useEventSource, SSE_POLL_INTERVAL_MS } from '@/hooks/use-event-source'

/** Display state for a canvas panel derived from its worktree. */
export interface PanelWorktreeState {
  /** The worktree's current state enum (connected, shell, background, disconnected). */
  worktreeState: WorktreeState
  /** Whether the terminal has an active connection (connected or shell state). */
  isActive: boolean
  needsInput: boolean
  attentionDotClass?: string
  attentionLabel?: string
  stateLabel?: string
  stateDotClass: string
}

/**
 * Polls worktree state for all panels on the canvas and derives
 * display indicators (status dot, attention dot, state label).
 *
 * Groups panels by project to minimize API calls — one fetch per
 * unique project, regardless of how many panels it has.
 *
 * SSE worktree_state events update attention indicators in real-time
 * between polls.
 *
 * @param panels - The current canvas panels.
 * @returns A map from panel ID to its display state.
 */
export function useCanvasWorktreeState(panels: CanvasPanel[]): Map<string, PanelWorktreeState> {
  const [stateMap, setStateMap] = useState<Map<string, PanelWorktreeState>>(new Map())
  const panelsRef = useRef(panels)
  useEffect(() => {
    panelsRef.current = panels
  }, [panels])

  useEffect(() => {
    let cancelled = false

    const poll = async () => {
      const currentPanels = panelsRef.current
      if (currentPanels.length === 0) {
        setStateMap((prev) => (prev.size === 0 ? prev : new Map()))
        return
      }

      // Group panels by project+agentType to batch API calls.
      const projectKeys = [...new Set(currentPanels.map((p) => `${p.projectId}:${p.agentType}`))]
      const nextMap = new Map<string, PanelWorktreeState>()

      await Promise.all(
        projectKeys.map(async (key) => {
          const [projectId, agentType] = key.split(':')
          try {
            const worktrees = await fetchWorktrees(projectId, agentType)
            const worktreeMap = new Map(worktrees.map((wt) => [wt.id, wt]))

            for (const panel of currentPanels) {
              if (panel.projectId !== projectId || panel.agentType !== agentType) continue
              const wt = worktreeMap.get(panel.worktreeId)

              if (!wt) {
                nextMap.set(panel.id, {
                  worktreeState: 'disconnected',
                  isActive: false,
                  needsInput: false,
                  stateDotClass: 'bg-muted-foreground/40',
                  stateLabel: 'Unknown',
                })
                continue
              }

              const attention = wt.needsInput ? getAttentionConfig(wt.notificationType) : null

              nextMap.set(panel.id, {
                worktreeState: wt.state,
                isActive: hasActiveTerminal(wt),
                needsInput: wt.needsInput ?? false,
                attentionDotClass: attention?.dotClass,
                attentionLabel: attention?.label,
                stateLabel: deriveStateLabel(wt.state, wt.exitCode),
                stateDotClass: worktreeStateIndicator[wt.state].dotClass,
              })
            }
          } catch {
            // If a project fetch fails, leave its panels with stale state.
          }
        }),
      )

      if (!cancelled) {
        setStateMap((prev) => {
          // Carry forward previous state for panels whose project fetch
          // failed. Without this, a transient API error would wipe the
          // panel's state and flip isActive to false (via the ?? false
          // fallback), causing an active terminal to show "disconnected".
          for (const panel of currentPanels) {
            if (!nextMap.has(panel.id)) {
              const prevState = prev.get(panel.id)
              if (prevState) {
                nextMap.set(panel.id, prevState)
              }
            }
          }

          if (prev.size !== nextMap.size) return nextMap
          for (const [id, next] of nextMap) {
            const current = prev.get(id)
            if (!current) return nextMap
            if (
              current.worktreeState !== next.worktreeState ||
              current.isActive !== next.isActive ||
              current.needsInput !== next.needsInput ||
              current.stateDotClass !== next.stateDotClass ||
              current.attentionDotClass !== next.attentionDotClass ||
              current.stateLabel !== next.stateLabel
            ) {
              return nextMap
            }
          }
          return prev
        })
      }
    }

    if (panelsRef.current.length === 0) return

    // Seed optimistic state for newly added panels. Panels are only created
    // when a user connects a terminal, so isActive: true is the correct default.
    // This ensures TerminalPanel receives isActive=true immediately, before the
    // first poll resolves, so it opens the WebSocket without waiting.
    setStateMap((prev) => {
      let changed = false
      const next = new Map(prev)
      for (const panel of panelsRef.current) {
        if (!next.has(panel.id)) {
          next.set(panel.id, {
            worktreeState: 'connected',
            isActive: true,
            needsInput: false,
            stateDotClass: worktreeStateIndicator.connected.dotClass,
            stateLabel: undefined,
          })
          changed = true
        }
      }
      return changed ? next : prev
    })

    poll()
    const interval = setInterval(poll, SSE_POLL_INTERVAL_MS)
    return () => {
      cancelled = true
      clearInterval(interval)
    }
    // panels.length as dependency is intentional: panelsRef keeps the latest
    // panel list, so the interval always polls the current set without
    // restarting unnecessarily.
  }, [panels.length])

  /** Applies a worktree_state SSE event to canvas panel state. */
  const handleWorktreeState = useCallback((event: WorktreeStateEvent) => {
    const currentPanels = panelsRef.current
    if (currentPanels.length === 0) return

    // Find panels matching this event.
    const matchingPanels = currentPanels.filter(
      (p) =>
        p.projectId === event.projectId &&
        p.agentType === event.agentType &&
        p.worktreeId === event.worktreeId,
    )
    if (matchingPanels.length === 0) return

    const attention = event.needsInput ? getAttentionConfig(event.notificationType) : null

    setStateMap((prev) => {
      let changed = false
      const next = new Map(prev)

      for (const panel of matchingPanels) {
        const current = prev.get(panel.id)
        if (!current) continue

        const derivedState = deriveWorktreeStateFromEvent(event, current.worktreeState)
        const nextIsActive = hasActiveTerminal({ state: derivedState })
        const nextStateDotClass = worktreeStateIndicator[derivedState].dotClass
        const nextStateLabel = deriveStateLabel(derivedState, event.exitCode)
        const nextNeedsInput = event.needsInput
        const nextAttentionDotClass = attention?.dotClass
        const nextAttentionLabel = attention?.label

        const isSame =
          current.worktreeState === derivedState &&
          current.isActive === nextIsActive &&
          current.needsInput === nextNeedsInput &&
          current.attentionDotClass === nextAttentionDotClass &&
          current.attentionLabel === nextAttentionLabel &&
          current.stateLabel === nextStateLabel
        if (isSame) continue

        changed = true
        next.set(panel.id, {
          ...current,
          worktreeState: derivedState,
          isActive: nextIsActive,
          needsInput: nextNeedsInput,
          attentionDotClass: nextAttentionDotClass,
          attentionLabel: nextAttentionLabel,
          stateLabel: nextStateLabel,
          stateDotClass: nextStateDotClass,
        })
      }

      return changed ? next : prev
    })
  }, [])

  useEventSource({ onWorktreeState: handleWorktreeState })

  return stateMap
}

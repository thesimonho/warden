/**
 * SSE (Server-Sent Events) client — reference implementation for real-time updates.
 *
 * If you're building a frontend that needs live worktree state, cost updates,
 * or attention notifications, copy the patterns in this file. Key patterns:
 *
 * - **Singleton connection**: All components share one EventSource via a
 *   module-level ref-counted singleton (`acquireSource` / `releaseSource`).
 *   The connection opens on first subscriber mount and closes when the last
 *   one unmounts. This avoids duplicate TCP connections.
 *
 * - **Exponential backoff reconnection**: When the connection drops, we
 *   schedule reconnection with exponential backoff (1s → 2s → 4s → ... → 30s).
 *   The attempt counter resets on successful connection. New EventSource
 *   instances are created on reconnect, and event listeners are re-attached.
 *
 * - **Typed event parsing**: SSE events arrive as named events
 *   (`worktree_state`, `project_state`, `worktree_list_changed`) with JSON
 *   `data` fields. The `makeSSEHandler` helper parses JSON and forwards to
 *   a ref callback, so handler identity changes don't cause re-subscriptions.
 *
 * - **Ref-based callbacks**: Handlers are stored in refs and updated on every
 *   render. The SSE listener reads from the ref, so the effect never needs to
 *   re-subscribe when a callback changes. This is the standard React pattern
 *   for "latest callback in a stable effect".
 *
 * For the REST API, see `api.ts`.
 * For terminal WebSocket, see `use-terminal.ts`.
 *
 * @module
 */
import type React from 'react'
import { useEffect, useRef, useState } from 'react'

import type {
  BudgetContainerStoppedEvent,
  BudgetExceededEvent,
  ContainerStateChangedEvent,
  ProjectStateEvent,
  ViewerFocusEvent,
  WorktreeListChangedEvent,
  WorktreeStateEvent,
} from '@/lib/types'

/** Connection status of the SSE stream. */
export type EventSourceStatus = 'connecting' | 'open' | 'closed' | 'server_stopped'

// ---------------------------------------------------------------------------
// Module-level singleton — all hooks share one TCP connection.
// Multiple components (project cards, sidebar, canvas) all need real-time
// state. Rather than each opening its own EventSource, we ref-count a single
// connection and multiplex event listeners onto it.
// ---------------------------------------------------------------------------

let sharedSource: EventSource | null = null
let refCount = 0
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let reconnectAttempt = 0
let serverStopped = false

/**
 * Reduced polling interval for hooks that receive real-time SSE updates.
 * Polling serves as a safety net for crash recovery and initial hydration.
 */
export const SSE_POLL_INTERVAL_MS = 15_000

/** Maximum backoff delay in milliseconds. */
const MAX_BACKOFF_MS = 30_000

/** Base backoff delay in milliseconds. */
const BASE_BACKOFF_MS = 1_000

/** Returns exponential backoff delay clamped to MAX_BACKOFF_MS. */
function backoffDelay(attempt: number): number {
  const delay = BASE_BACKOFF_MS * 2 ** attempt
  return Math.min(delay, MAX_BACKOFF_MS)
}

/** Status listeners notified when connection state changes. */
const statusListeners = new Set<(status: EventSourceStatus) => void>()

/** Notifies all registered status listeners. */
function broadcastStatus(status: EventSourceStatus): void {
  for (const listener of statusListeners) {
    listener(status)
  }
}

/** Creates a new EventSource and wires up reconnection logic. */
function createSource(): EventSource {
  const source = new EventSource('/api/v1/events')

  source.addEventListener('open', () => {
    reconnectAttempt = 0
    broadcastStatus('open')
  })

  // server_shutdown is broadcast by the server before it stops.
  // Set the flag so we show "server_stopped" instead of reconnecting.
  source.addEventListener('server_shutdown', () => {
    serverStopped = true
  })

  source.addEventListener('error', () => {
    // EventSource auto-closes on error. Clean up and schedule reconnect.
    source.close()
    sharedSource = null

    // If the server sent a shutdown event, don't reconnect.
    if (serverStopped) {
      broadcastStatus('server_stopped')
      return
    }

    broadcastStatus('closed')

    if (refCount > 0) {
      const delay = backoffDelay(reconnectAttempt)
      reconnectAttempt++
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null
        if (refCount > 0) {
          broadcastStatus('connecting')
          sharedSource = createSource()
        }
      }, delay)
    }
  })

  return source
}

/**
 * Acquires a reference to the shared EventSource singleton.
 * Creates the connection on first use.
 */
function acquireSource(): EventSource {
  refCount++
  if (!sharedSource) {
    broadcastStatus('connecting')
    sharedSource = createSource()
  }
  return sharedSource
}

/**
 * Releases a reference to the shared EventSource.
 * Closes the connection when no hooks are subscribed.
 */
function releaseSource(): void {
  refCount--
  if (refCount <= 0) {
    refCount = 0
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (sharedSource) {
      sharedSource.close()
      sharedSource = null
    }
    reconnectAttempt = 0
    serverStopped = false
    broadcastStatus('closed')
  }
}

// ---------------------------------------------------------------------------
// Public hooks
// ---------------------------------------------------------------------------

/** Callback for worktree_state SSE events. */
export type WorktreeStateHandler = (event: WorktreeStateEvent) => void

/** Callback for project_state SSE events. */
export type ProjectStateHandler = (event: ProjectStateEvent) => void

/** Callback for worktree_list_changed SSE events. */
export type WorktreeListChangedHandler = (event: WorktreeListChangedEvent) => void

/** Callback for budget_exceeded SSE events. */
export type BudgetExceededHandler = (event: BudgetExceededEvent) => void

/** Callback for budget_container_stopped SSE events. */
export type BudgetContainerStoppedHandler = (event: BudgetContainerStoppedEvent) => void

/** Payload for runtime_status SSE events. */
export interface RuntimeStatusEvent {
  projectId: string
  agentType?: string
  containerName: string
  /** "installing" or "installed". */
  phase: 'installing' | 'installed'
  runtimeId: string
  runtimeLabel: string
}

/** Callback for runtime_status SSE events. */
export type RuntimeStatusHandler = (event: RuntimeStatusEvent) => void

/** Payload for agent_status SSE events. */
export interface AgentStatusEvent {
  projectId: string
  agentType?: string
  containerName: string
  /** "installing" or "installed". */
  phase: 'installing' | 'installed'
  version: string
}

/** Callback for agent_status SSE events. */
export type AgentStatusHandler = (event: AgentStatusEvent) => void

/** Callback for container_state_changed SSE events. */
export type ContainerStateChangedHandler = (event: ContainerStateChangedEvent) => void

/** Callback for viewer_focus SSE events. */
export type ViewerFocusHandler = (event: ViewerFocusEvent) => void

/** Options for subscribing to SSE events. */
interface UseEventSourceOptions {
  /** Handler for worktree_state events. */
  onWorktreeState?: WorktreeStateHandler
  /** Handler for project_state events. */
  onProjectState?: ProjectStateHandler
  /** Handler for worktree_list_changed events. */
  onWorktreeListChanged?: WorktreeListChangedHandler
  /** Handler for budget_exceeded events. */
  onBudgetExceeded?: BudgetExceededHandler
  /** Handler for budget_container_stopped events. */
  onBudgetContainerStopped?: BudgetContainerStoppedHandler
  /** Handler for runtime_status events (runtime install progress). */
  onRuntimeStatus?: RuntimeStatusHandler
  /** Handler for agent_status events (agent CLI install progress). */
  onAgentStatus?: AgentStatusHandler
  /** Handler for container_state_changed events (container lifecycle). */
  onContainerStateChanged?: ContainerStateChangedHandler
  /** Handler for viewer_focus events (viewer focus state changes). */
  onViewerFocus?: ViewerFocusHandler
}

/** Creates a MessageEvent handler that parses JSON and forwards to a ref callback. */
function makeSSEHandler<T>(ref: React.RefObject<((data: T) => void) | undefined>) {
  return (e: MessageEvent) => {
    try {
      ref.current?.(JSON.parse(e.data) as T)
    } catch {
      // Ignore malformed events.
    }
  }
}

/**
 * Subscribes to the shared SSE connection and dispatches typed events.
 *
 * Multiple components calling this hook share a single EventSource
 * connection. The connection is created on first mount and closed when
 * the last subscriber unmounts.
 *
 * @param options - Event handlers for worktree_state and project_state.
 * @returns The current connection status.
 */
export function useEventSource(options: UseEventSourceOptions): EventSourceStatus {
  const [status, setStatus] = useState<EventSourceStatus>('connecting')
  const onWorktreeStateRef = useRef(options.onWorktreeState)
  const onProjectStateRef = useRef(options.onProjectState)
  const onWorktreeListChangedRef = useRef(options.onWorktreeListChanged)
  const onBudgetExceededRef = useRef(options.onBudgetExceeded)
  const onBudgetContainerStoppedRef = useRef(options.onBudgetContainerStopped)
  const onRuntimeStatusRef = useRef(options.onRuntimeStatus)
  const onAgentStatusRef = useRef(options.onAgentStatus)
  const onContainerStateChangedRef = useRef(options.onContainerStateChanged)
  const onViewerFocusRef = useRef(options.onViewerFocus)

  // Keep refs current without re-subscribing.
  useEffect(() => {
    onWorktreeStateRef.current = options.onWorktreeState
    onProjectStateRef.current = options.onProjectState
    onWorktreeListChangedRef.current = options.onWorktreeListChanged
    onBudgetExceededRef.current = options.onBudgetExceeded
    onBudgetContainerStoppedRef.current = options.onBudgetContainerStopped
    onRuntimeStatusRef.current = options.onRuntimeStatus
    onAgentStatusRef.current = options.onAgentStatus
    onContainerStateChangedRef.current = options.onContainerStateChanged
    onViewerFocusRef.current = options.onViewerFocus
  }, [
    options.onWorktreeState,
    options.onProjectState,
    options.onWorktreeListChanged,
    options.onBudgetExceeded,
    options.onBudgetContainerStopped,
    options.onRuntimeStatus,
    options.onAgentStatus,
    options.onContainerStateChanged,
    options.onViewerFocus,
  ])

  useEffect(() => {
    const source = acquireSource()

    // Register status listener.
    statusListeners.add(setStatus)

    // Sync initial status from the external EventSource — legitimate effect-to-state sync.
    if (source.readyState === EventSource.OPEN) setStatus('open')
    else if (source.readyState === EventSource.CLOSED) setStatus('closed')
    else setStatus('connecting')

    const handleWorktreeState = makeSSEHandler(onWorktreeStateRef)
    const handleProjectState = makeSSEHandler(onProjectStateRef)
    const handleWorktreeListChanged = makeSSEHandler(onWorktreeListChangedRef)
    const handleBudgetExceeded = makeSSEHandler(onBudgetExceededRef)
    const handleBudgetContainerStopped = makeSSEHandler(onBudgetContainerStoppedRef)
    const handleRuntimeStatus = makeSSEHandler(onRuntimeStatusRef)
    const handleAgentStatus = makeSSEHandler(onAgentStatusRef)
    const handleContainerStateChanged = makeSSEHandler(onContainerStateChangedRef)
    const handleViewerFocus = makeSSEHandler(onViewerFocusRef)

    // We need to listen on the current source AND any future reconnected sources.
    // Since sharedSource can change on reconnect, we add listeners at the module level.
    // Use a helper that attaches to whatever sharedSource exists.
    const attach = (src: EventSource) => {
      src.addEventListener('worktree_state', handleWorktreeState)
      src.addEventListener('project_state', handleProjectState)
      src.addEventListener('worktree_list_changed', handleWorktreeListChanged)
      src.addEventListener('budget_exceeded', handleBudgetExceeded)
      src.addEventListener('budget_container_stopped', handleBudgetContainerStopped)
      src.addEventListener('runtime_status', handleRuntimeStatus)
      src.addEventListener('agent_status', handleAgentStatus)
      src.addEventListener('container_state_changed', handleContainerStateChanged)
      src.addEventListener('viewer_focus', handleViewerFocus)
    }

    const detach = (src: EventSource) => {
      src.removeEventListener('worktree_state', handleWorktreeState)
      src.removeEventListener('project_state', handleProjectState)
      src.removeEventListener('worktree_list_changed', handleWorktreeListChanged)
      src.removeEventListener('budget_exceeded', handleBudgetExceeded)
      src.removeEventListener('budget_container_stopped', handleBudgetContainerStopped)
      src.removeEventListener('runtime_status', handleRuntimeStatus)
      src.removeEventListener('agent_status', handleAgentStatus)
      src.removeEventListener('container_state_changed', handleContainerStateChanged)
      src.removeEventListener('viewer_focus', handleViewerFocus)
    }

    attach(source)

    // Track the source we attached to so we can re-attach on reconnect.
    let currentSource = source

    // Poll for reconnections — the status listener will tell us when it happens.
    const onStatusChange = (newStatus: EventSourceStatus) => {
      setStatus(newStatus)
      if (newStatus === 'connecting' || newStatus === 'open') {
        if (sharedSource && sharedSource !== currentSource) {
          detach(currentSource)
          attach(sharedSource)
          currentSource = sharedSource
        }
      }
    }

    // Replace the simple setStatus listener with our enhanced one.
    statusListeners.delete(setStatus)
    statusListeners.add(onStatusChange)

    return () => {
      statusListeners.delete(onStatusChange)
      detach(currentSource)
      releaseSource()
    }
  }, [])

  return status
}

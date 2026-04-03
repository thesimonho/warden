import { describe, it, expect } from 'vitest'
import {
  deriveWorktreeStateFromEvent,
  hasActiveTerminal,
  isConnectedWorktree,
  isWorktreeAlive,
} from '@/lib/types'
import type { WorktreeState, WorktreeStateEvent } from '@/lib/types'

describe('hasActiveTerminal', () => {
  it('returns true for connected worktrees', () => {
    expect(hasActiveTerminal({ state: 'connected' })).toBe(true)
  })

  it('returns true for shell worktrees', () => {
    expect(hasActiveTerminal({ state: 'shell' })).toBe(true)
  })

  it('returns false for disconnected worktrees', () => {
    expect(hasActiveTerminal({ state: 'stopped' })).toBe(false)
  })

  it('returns true for background worktrees', () => {
    expect(hasActiveTerminal({ state: 'background' })).toBe(true)
  })

  it('returns false for all non-active states', () => {
    const inactiveStates: WorktreeState[] = ['stopped']
    for (const state of inactiveStates) {
      expect(hasActiveTerminal({ state })).toBe(false)
    }
  })
})

describe('isConnectedWorktree', () => {
  it('returns true for connected worktrees', () => {
    expect(isConnectedWorktree({ state: 'connected' })).toBe(true)
  })

  it('returns false for shell worktrees', () => {
    expect(isConnectedWorktree({ state: 'shell' })).toBe(false)
  })

  it('returns false for disconnected worktrees', () => {
    expect(isConnectedWorktree({ state: 'stopped' })).toBe(false)
  })

  it('returns false for background worktrees', () => {
    expect(isConnectedWorktree({ state: 'background' })).toBe(false)
  })
})

describe('isWorktreeAlive', () => {
  it('returns true for connected worktrees', () => {
    expect(isWorktreeAlive({ state: 'connected' })).toBe(true)
  })

  it('returns true for shell worktrees', () => {
    expect(isWorktreeAlive({ state: 'shell' })).toBe(true)
  })

  it('returns true for background worktrees', () => {
    expect(isWorktreeAlive({ state: 'background' })).toBe(true)
  })

  it('returns false for disconnected worktrees', () => {
    expect(isWorktreeAlive({ state: 'stopped' })).toBe(false)
  })
})

describe('deriveWorktreeStateFromEvent', () => {
  const baseEvent: WorktreeStateEvent = {
    projectId: 'abc123def456',
    containerName: 'proj-1',
    worktreeId: 'main',
    needsInput: false,
    sessionActive: true,
  }

  it('uses event.state when present', () => {
    const event = { ...baseEvent, state: 'background' as const }
    expect(deriveWorktreeStateFromEvent(event, 'connected')).toBe('background')
  })

  it('transitions connected → shell when session ends', () => {
    const event = { ...baseEvent, sessionActive: false }
    expect(deriveWorktreeStateFromEvent(event, 'connected')).toBe('shell')
  })

  it('preserves current state when session is active and no push state', () => {
    expect(deriveWorktreeStateFromEvent(baseEvent, 'connected')).toBe('connected')
  })

  it('preserves non-connected state when session ends without push state', () => {
    const event = { ...baseEvent, sessionActive: false }
    expect(deriveWorktreeStateFromEvent(event, 'background')).toBe('background')
  })

  it('prefers event.state over session heuristic', () => {
    const event = { ...baseEvent, sessionActive: false, state: 'connected' as const }
    expect(deriveWorktreeStateFromEvent(event, 'connected')).toBe('connected')
  })
})

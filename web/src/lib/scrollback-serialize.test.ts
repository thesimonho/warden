import { describe, it, expect } from 'vitest'
import { scrollbackKey } from '@/lib/scrollback-serialize'

describe('scrollbackKey', () => {
  it('produces projectId:worktreeId format', () => {
    expect(scrollbackKey('abc123def456', 'main')).toBe('abc123def456:main')
  })

  it('handles worktree IDs with special characters', () => {
    expect(scrollbackKey('abc123def456', 'feature-branch.v2')).toBe(
      'abc123def456:feature-branch.v2',
    )
  })
})

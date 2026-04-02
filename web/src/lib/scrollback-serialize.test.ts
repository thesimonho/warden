import { describe, it, expect } from 'vitest'
import { serializeTerminal } from '@/lib/scrollback-serialize'

describe('serializeTerminal', () => {
  it('returns null for empty serialize result', () => {
    const mockAddon = { serialize: () => '' }
    const mockTerminal = { options: { scrollback: 100 } }
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    expect(serializeTerminal(mockTerminal as any, mockAddon as any)).toBeNull()
  })

  it('returns the ANSI string when buffer has content', () => {
    const ansi = '\x1b[32mhello\x1b[0m'
    const mockAddon = { serialize: () => ansi }
    const mockTerminal = { options: { scrollback: 100 } }
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    expect(serializeTerminal(mockTerminal as any, mockAddon as any)).toBe(ansi)
  })
})

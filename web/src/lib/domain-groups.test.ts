import { describe, expect, it } from 'vitest'

import { getRestrictedDomains } from '@/lib/domain-groups'

describe('getRestrictedDomains', () => {
  const serverDomains: Record<string, string[]> = {
    'claude-code': [
      '*.anthropic.com',
      '*.github.com',
      '*.githubusercontent.com',
      'registry.npmjs.org',
    ],
    codex: [
      '*.openai.com',
      '*.chatgpt.com',
      '*.github.com',
      '*.githubusercontent.com',
      'registry.npmjs.org',
    ],
  }

  it('returns claude-code domains for claude-code', () => {
    const domains = getRestrictedDomains(serverDomains, 'claude-code')
    expect(domains).toContain('*.anthropic.com')
    expect(domains).not.toContain('*.openai.com')
  })

  it('returns codex domains for codex', () => {
    const domains = getRestrictedDomains(serverDomains, 'codex')
    expect(domains).toContain('*.openai.com')
    expect(domains).not.toContain('*.anthropic.com')
  })

  it('includes shared domains for both agent types', () => {
    for (const agentType of ['claude-code', 'codex'] as const) {
      const domains = getRestrictedDomains(serverDomains, agentType)
      expect(domains).toContain('*.github.com')
      expect(domains).toContain('registry.npmjs.org')
    }
  })

  it('returns empty array when domains map is undefined', () => {
    expect(getRestrictedDomains(undefined, 'claude-code')).toEqual([])
  })

  it('returns empty array for unknown agent type', () => {
    // oxlint-disable-next-line @typescript-eslint/no-explicit-any -- testing invalid input
    expect(getRestrictedDomains(serverDomains, 'unknown' as any)).toEqual([])
  })
})

import { describe, it, expect } from 'vitest'
import { restrictedDomains } from '@/lib/domain-groups'

describe('restrictedDomains', () => {
  it('has at least one domain', () => {
    expect(restrictedDomains.length).toBeGreaterThan(0)
  })

  it('has no duplicate domains', () => {
    const unique = new Set(restrictedDomains)
    expect(unique.size).toBe(restrictedDomains.length)
  })

  it('all domains are non-empty trimmed strings', () => {
    for (const domain of restrictedDomains) {
      expect(domain).toBeTruthy()
      expect(domain).toBe(domain.trim())
    }
  })

  it('includes anthropic wildcard', () => {
    expect(restrictedDomains).toContain('*.anthropic.com')
  })

  it('includes github wildcard', () => {
    expect(restrictedDomains).toContain('*.github.com')
  })
})

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { abbreviateHomePath, relativeTime } from '@/lib/utils'

const FIXED_NOW = new Date('2024-06-15T12:00:00Z')
const FIXED_NOW_SECONDS = Math.floor(FIXED_NOW.getTime() / 1000)

describe('abbreviateHomePath', () => {
  it('abbreviates /home/<user> to ~', () => {
    expect(abbreviateHomePath('/home/simon/Projects/warden')).toBe('~/Projects/warden')
  })

  it('abbreviates /Users/<user> to ~', () => {
    expect(abbreviateHomePath('/Users/simon/Projects/warden')).toBe('~/Projects/warden')
  })

  it('strips Docker Desktop /host_mnt prefix before abbreviating', () => {
    expect(abbreviateHomePath('/host_mnt/home/simon/Projects/warden')).toBe('~/Projects/warden')
    expect(abbreviateHomePath('/host_mnt/Users/simon/Projects/warden')).toBe('~/Projects/warden')
  })

  it('leaves other paths unchanged', () => {
    expect(abbreviateHomePath('/var/lib/data')).toBe('/var/lib/data')
    expect(abbreviateHomePath('/host_mnt/var/lib/data')).toBe('/host_mnt/var/lib/data')
  })
})

describe('relativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(FIXED_NOW)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns "just now" for timestamps within the last 59 seconds', () => {
    expect(relativeTime(FIXED_NOW_SECONDS)).toBe('just now')
    expect(relativeTime(FIXED_NOW_SECONDS - 1)).toBe('just now')
    expect(relativeTime(FIXED_NOW_SECONDS - 59)).toBe('just now')
  })

  it('returns minutes for 1–59 minutes ago', () => {
    expect(relativeTime(FIXED_NOW_SECONDS - 60)).toBe('1m ago')
    expect(relativeTime(FIXED_NOW_SECONDS - 90)).toBe('1m ago')
    expect(relativeTime(FIXED_NOW_SECONDS - 3599)).toBe('59m ago')
  })

  it('returns hours for 1–23 hours ago', () => {
    expect(relativeTime(FIXED_NOW_SECONDS - 3600)).toBe('1h ago')
    expect(relativeTime(FIXED_NOW_SECONDS - 7200)).toBe('2h ago')
    expect(relativeTime(FIXED_NOW_SECONDS - 86399)).toBe('23h ago')
  })

  it('returns days for 24+ hours ago', () => {
    expect(relativeTime(FIXED_NOW_SECONDS - 86400)).toBe('1d ago')
    expect(relativeTime(FIXED_NOW_SECONDS - 86400 * 7)).toBe('7d ago')
  })
})

import { describe, it, expect } from 'vitest'
import { formatCost, formatDuration } from '@/lib/cost'

describe('formatCost', () => {
  it('formats zero cost', () => {
    expect(formatCost(0)).toBe('$0.00')
  })

  it('formats positive cost to two decimal places', () => {
    expect(formatCost(0.42)).toBe('$0.42')
    expect(formatCost(1.005)).toBe('$1.00')
    expect(formatCost(12.999)).toBe('$13.00')
  })
})

describe('formatDuration', () => {
  it('formats seconds only', () => {
    expect(formatDuration(0)).toBe('0s')
    expect(formatDuration(5_000)).toBe('5s')
    expect(formatDuration(59_000)).toBe('59s')
  })

  it('formats minutes and seconds', () => {
    expect(formatDuration(60_000)).toBe('1m')
    expect(formatDuration(90_000)).toBe('1m 30s')
    expect(formatDuration(150_000)).toBe('2m 30s')
  })

  it('formats hours and minutes', () => {
    expect(formatDuration(3_600_000)).toBe('1h')
    expect(formatDuration(4_500_000)).toBe('1h 15m')
    expect(formatDuration(7_200_000)).toBe('2h')
  })
})

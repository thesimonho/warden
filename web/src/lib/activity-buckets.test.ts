import { describe, expect, it } from 'vitest'

import { type ActivityBucket, bucketEventsByCategory, chooseBucketWidth } from './activity-buckets'
import type { AuditCategory, AuditLogEntry } from './types'

/** Creates a minimal event entry at the given ISO timestamp with the given category. */
function entry(
  ts: string,
  category: AuditCategory = 'agent',
  source: AuditLogEntry['source'] = 'agent',
): AuditLogEntry {
  return {
    id: 0,
    ts,
    source,
    level: 'info',
    event: 'test',
    msg: '',
    worktree: '',
    category,
  }
}

/** Sums a single category across all buckets. */
const sumCategory = (buckets: ActivityBucket[], key: keyof ActivityBucket): number =>
  buckets.reduce((sum: number, b) => sum + (b[key] as number), 0)

const HOUR = 3_600_000
const DAY = 86_400_000

describe('chooseBucketWidth', () => {
  it('returns 1 hour for spans under 24 hours', () => {
    expect(chooseBucketWidth(12 * HOUR)).toBe(HOUR)
    expect(chooseBucketWidth(23 * HOUR)).toBe(HOUR)
  })

  it('returns 6 hours for spans of 1–7 days', () => {
    expect(chooseBucketWidth(2 * DAY)).toBe(6 * HOUR)
    expect(chooseBucketWidth(7 * DAY)).toBe(6 * HOUR)
  })

  it('returns 1 day for spans of 1–4 weeks', () => {
    expect(chooseBucketWidth(10 * DAY)).toBe(DAY)
    expect(chooseBucketWidth(28 * DAY)).toBe(DAY)
  })

  it('returns 1 week for spans of 1–6 months', () => {
    expect(chooseBucketWidth(60 * DAY)).toBe(7 * DAY)
    expect(chooseBucketWidth(180 * DAY)).toBe(7 * DAY)
  })

  it('returns 1 month (30 days) for spans over 6 months', () => {
    expect(chooseBucketWidth(200 * DAY)).toBe(30 * DAY)
    expect(chooseBucketWidth(365 * DAY)).toBe(30 * DAY)
  })

  it('returns 1 hour for zero or negative spans', () => {
    expect(chooseBucketWidth(0)).toBe(HOUR)
    expect(chooseBucketWidth(-1000)).toBe(HOUR)
  })
})

describe('bucketEventsByCategory', () => {
  it('returns empty result for empty input', () => {
    const result = bucketEventsByCategory([])
    expect(result.buckets).toEqual([])
    expect(result.bucketWidthMs).toBe(HOUR)
  })

  it('pads a single event to at least 4 buckets', () => {
    const result = bucketEventsByCategory([entry('2026-03-25T10:00:00Z', 'agent')])
    expect(result.buckets.length).toBeGreaterThanOrEqual(4)
    expect(sumCategory(result.buckets, 'agent')).toBe(1)
    expect(sumCategory(result.buckets, 'session')).toBe(0)
  })

  it('counts events by category within the same bucket', () => {
    const events = [
      entry('2026-03-25T10:00:00Z', 'agent'),
      entry('2026-03-25T10:15:00Z', 'agent'),
      entry('2026-03-25T10:30:00Z', 'session'),
      entry('2026-03-25T10:35:00Z', 'prompt'),
      entry('2026-03-25T10:45:00Z', 'config'),
      entry('2026-03-25T10:50:00Z', 'system'),
    ]
    const result = bucketEventsByCategory(events)
    expect(sumCategory(result.buckets, 'agent')).toBe(2)
    expect(sumCategory(result.buckets, 'session')).toBe(1)
    expect(sumCategory(result.buckets, 'prompt')).toBe(1)
    expect(sumCategory(result.buckets, 'config')).toBe(1)
    expect(sumCategory(result.buckets, 'system')).toBe(1)
  })

  it('uses 1-hour buckets for short time spans', () => {
    const events = [entry('2026-03-25T10:00:00Z', 'agent'), entry('2026-03-25T13:00:00Z', 'agent')]
    const result = bucketEventsByCategory(events)
    expect(result.bucketWidthMs).toBe(HOUR)
    expect(result.buckets.length).toBe(4)
  })

  it('uses 6-hour buckets for multi-day spans', () => {
    const events = [entry('2026-03-20T10:00:00Z', 'agent'), entry('2026-03-23T10:00:00Z', 'agent')]
    const result = bucketEventsByCategory(events)
    expect(result.bucketWidthMs).toBe(6 * HOUR)
    // 3 days = 72 hours → 12 six-hour buckets + 1 = 13
    expect(result.buckets.length).toBeGreaterThanOrEqual(12)
  })

  it('uses daily buckets for multi-week spans', () => {
    const events = [entry('2026-03-01T10:00:00Z', 'agent'), entry('2026-03-20T10:00:00Z', 'agent')]
    const result = bucketEventsByCategory(events)
    expect(result.bucketWidthMs).toBe(DAY)
  })

  it('uses weekly buckets for multi-month spans', () => {
    const events = [entry('2026-01-01T10:00:00Z', 'agent'), entry('2026-03-20T10:00:00Z', 'agent')]
    const result = bucketEventsByCategory(events)
    expect(result.bucketWidthMs).toBe(7 * DAY)
  })

  it('fills gaps with zero-count buckets', () => {
    const events = [entry('2026-03-25T10:00:00Z', 'agent'), entry('2026-03-25T13:00:00Z', 'agent')]
    const result = bucketEventsByCategory(events)
    expect(result.buckets.length).toBe(4)

    const middleBuckets = result.buckets.slice(1, -1)
    for (const bucket of middleBuckets) {
      const total =
        bucket.session +
        bucket.agent +
        bucket.prompt +
        bucket.config +
        bucket.budget +
        bucket.system +
        bucket.debug
      expect(total).toBe(0)
    }
  })

  it('buckets are sorted by time ascending', () => {
    const events = [
      entry('2026-03-25T14:00:00Z', 'agent'),
      entry('2026-03-25T10:00:00Z', 'session'),
      entry('2026-03-25T12:00:00Z', 'prompt'),
    ]
    const result = bucketEventsByCategory(events)
    for (let i = 1; i < result.buckets.length; i++) {
      expect(result.buckets[i].time).toBeGreaterThan(result.buckets[i - 1].time)
    }
  })

  it('skips events without a category', () => {
    const events: AuditLogEntry[] = [
      {
        id: 0,
        ts: '2026-03-25T10:00:00Z',
        source: 'agent',
        level: 'info',
        event: 'test',
        msg: '',
        worktree: '',
      },
      entry('2026-03-25T10:30:00Z', 'agent'),
    ]
    const result = bucketEventsByCategory(events)
    expect(sumCategory(result.buckets, 'agent')).toBe(1)
  })

  it('preserves total event count across all bucket widths', () => {
    // Events spanning 2 months → weekly buckets
    const events = [
      entry('2026-01-10T10:00:00Z', 'agent'),
      entry('2026-01-15T14:00:00Z', 'session'),
      entry('2026-02-01T08:00:00Z', 'prompt'),
      entry('2026-02-20T16:00:00Z', 'config'),
      entry('2026-03-10T12:00:00Z', 'system'),
    ]
    const result = bucketEventsByCategory(events)
    const totalEvents =
      sumCategory(result.buckets, 'agent') +
      sumCategory(result.buckets, 'session') +
      sumCategory(result.buckets, 'prompt') +
      sumCategory(result.buckets, 'config') +
      sumCategory(result.buckets, 'system')
    expect(totalEvents).toBe(5)
  })
})

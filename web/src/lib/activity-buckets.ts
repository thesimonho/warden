import type { AuditCategory, AuditLogEntry } from './types'

export const HOUR = 3_600_000
export const DAY = 86_400_000
const WEEK = 7 * DAY
const MONTH = 30 * DAY

/** All category keys in canonical order. */
export const CATEGORY_KEYS: AuditCategory[] = [
  'session',
  'agent',
  'prompt',
  'config',
  'budget',
  'system',
  'debug',
]

/** A single time bucket with event counts per category. */
export interface ActivityBucket {
  /** Bucket start time as ISO string. */
  timestamp: string
  /** Bucket start time as epoch ms (for recharts numeric axis). */
  time: number
  /** Event counts per category. */
  session: number
  agent: number
  prompt: number
  config: number
  budget: number
  system: number
  debug: number
}

/** Result of bucketing events, including the resolved bucket width. */
export interface BucketResult {
  /** Time-sorted buckets with gap-filling. */
  buckets: ActivityBucket[]
  /** The bucket width in milliseconds used for this dataset. */
  bucketWidthMs: number
}

/**
 * Picks a bucket width that keeps the total bar count in a reasonable
 * range (~24–180 bars) regardless of the time span.
 *
 * | Time span   | Bucket width |
 * |-------------|-------------|
 * | < 24h       | 1 hour      |
 * | 1–7 days    | 6 hours     |
 * | 1–4 weeks   | 1 day       |
 * | 1–6 months  | 1 week      |
 * | 6+ months   | 1 month     |
 */
export function chooseBucketWidth(spanMs: number): number {
  if (spanMs <= DAY) return HOUR
  if (spanMs <= 7 * DAY) return 6 * HOUR
  if (spanMs <= 28 * DAY) return DAY
  if (spanMs <= 180 * DAY) return WEEK
  return MONTH
}

/** Creates an empty bucket at the given timestamp. */
function emptyBucket(timeMs: number): ActivityBucket {
  return {
    timestamp: new Date(timeMs).toISOString(),
    time: timeMs,
    session: 0,
    agent: 0,
    prompt: 0,
    config: 0,
    budget: 0,
    system: 0,
    debug: 0,
  }
}

/**
 * Groups events into adaptive time buckets with counts per category.
 *
 * Bucket width is chosen automatically based on the time span of the
 * data (see {@link chooseBucketWidth}). Single-pass over the entries
 * to parse timestamps, find the time range, and count events per bucket.
 * Gaps between events are filled with zero-count buckets so the timeline
 * is continuous.
 */
export function bucketEventsByCategory(entries: readonly AuditLogEntry[]): BucketResult {
  if (entries.length === 0) return { buckets: [], bucketWidthMs: HOUR }

  // First pass: parse timestamps and find time range. Cache parsed values
  // to avoid re-creating Date objects in the counting pass.
  let minTime = Infinity
  let maxTime = -Infinity
  const parsed: { timeMs: number; category: AuditCategory }[] = []

  for (const entry of entries) {
    const category = entry.category
    if (!category) continue
    const timeMs = new Date(entry.ts).getTime()
    if (timeMs < minTime) minTime = timeMs
    if (timeMs > maxTime) maxTime = timeMs
    parsed.push({ timeMs, category })
  }

  if (parsed.length === 0) return { buckets: [], bucketWidthMs: HOUR }

  const bucketWidthMs = chooseBucketWidth(maxTime - minTime)

  // Second pass: count events per bucket using cached timestamps.
  const countMap = new Map<number, Record<AuditCategory, number>>()

  for (const { timeMs, category } of parsed) {
    const bucketStart = Math.floor(timeMs / bucketWidthMs) * bucketWidthMs
    let counts = countMap.get(bucketStart)
    if (!counts) {
      counts = { session: 0, agent: 0, prompt: 0, config: 0, budget: 0, system: 0, debug: 0 }
      countMap.set(bucketStart, counts)
    }
    if (category in counts) {
      counts[category]++
    }
  }

  // Build the full bucket array, filling gaps with zero-count buckets.
  // Pad with empty buckets so the brush always has room to move.
  const MIN_BUCKETS = 4
  let firstBucketStart = Math.floor(minTime / bucketWidthMs) * bucketWidthMs
  let lastBucketStart = Math.floor(maxTime / bucketWidthMs) * bucketWidthMs

  const dataSpan = (lastBucketStart - firstBucketStart) / bucketWidthMs + 1
  if (dataSpan < MIN_BUCKETS) {
    const padding = Math.ceil((MIN_BUCKETS - dataSpan) / 2)
    firstBucketStart -= padding * bucketWidthMs
    lastBucketStart += padding * bucketWidthMs
  }

  const buckets: ActivityBucket[] = []

  for (let t = firstBucketStart; t <= lastBucketStart; t += bucketWidthMs) {
    const counts = countMap.get(t)
    buckets.push(counts ? { ...emptyBucket(t), ...counts } : emptyBucket(t))
  }

  return { buckets, bucketWidthMs }
}

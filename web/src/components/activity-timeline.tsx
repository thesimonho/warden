import { useCallback, useEffect, useMemo, useRef } from 'react'
import { Bar, BarChart, Brush, CartesianGrid, XAxis, YAxis } from 'recharts'
import { X } from 'lucide-react'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { Button } from '@/components/ui/button'
import {
  bucketEventsByCategory,
  CATEGORY_KEYS,
  HOUR,
  DAY,
  type ActivityBucket,
} from '@/lib/activity-buckets'
import type { AuditLogEntry } from '@/lib/types'

/** Chart color config mapping categories to theme CSS variables. */
const chartConfig = {
  session: { label: 'Session', color: 'var(--category-session)' },
  agent: { label: 'Agent', color: 'var(--category-agent)' },
  prompt: { label: 'Prompt', color: 'var(--category-prompt)' },
  config: { label: 'Config', color: 'var(--category-config)' },
  budget: { label: 'Budget', color: 'var(--category-budget)' },
  system: { label: 'System', color: 'var(--category-system)' },
  debug: { label: 'Debug', color: 'var(--category-debug)' },
} satisfies ChartConfig

/** Category keys in visual stack order (bottom to top). */
const STACK_ORDER = [...CATEGORY_KEYS].reverse()

/**
 * Formats a bucket timestamp for the brush tick labels.
 * Shows time for hourly buckets, date for larger intervals.
 */
function formatBrushTick(timeMs: number, bucketWidthMs: number): string {
  if (bucketWidthMs < DAY) {
    return new Date(timeMs).toLocaleTimeString(undefined, {
      hour: 'numeric',
      minute: '2-digit',
      hour12: true,
    })
  }
  return new Date(timeMs).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
  })
}

/**
 * Formats a timestamp for the main X-axis.
 * Shows date + time for sub-day buckets, date-only for day+ buckets.
 */
function formatDateTick(timeMs: number, bucketWidthMs: number): string {
  if (bucketWidthMs < DAY) {
    return new Date(timeMs).toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      hour12: true,
    })
  }
  return new Date(timeMs).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
  })
}

/**
 * Formats a timestamp for the tooltip header.
 * Shows a range label appropriate to the bucket width.
 */
function formatTooltipLabel(timeMs: number, bucketWidthMs: number): string {
  const start = new Date(timeMs)
  if (bucketWidthMs <= HOUR) {
    return start.toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
      hour12: true,
    })
  }
  if (bucketWidthMs < DAY) {
    const end = new Date(timeMs + bucketWidthMs)
    const fmt = (d: Date) =>
      d.toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        hour: 'numeric',
        hour12: true,
      })
    return `${fmt(start)} – ${fmt(end)}`
  }
  if (bucketWidthMs === DAY) {
    return start.toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    })
  }
  // Week or month buckets: show date range.
  const end = new Date(timeMs + bucketWidthMs - DAY)
  const fmt = (d: Date) => d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  return `${fmt(start)} – ${fmt(end)}`
}

interface ActivityTimelineProps {
  /** All events to visualize (unfiltered by time). */
  entries: readonly AuditLogEntry[]
  /** Currently selected start time (ISO string). */
  since?: string
  /** Currently selected end time (ISO string). */
  until?: string
  /** Called when user drags the brush to select a new time range. */
  onRangeChange: (since: string | undefined, until: string | undefined) => void
}

/**
 * Stacked bar chart showing event density over time with a draggable
 * brush for selecting a time range. Bucket width adapts automatically
 * to the data's time span so the chart stays readable at any scale.
 */
export function ActivityTimeline({ entries, since, until, onRangeChange }: ActivityTimelineProps) {
  const { buckets, bucketWidthMs } = useMemo(() => bucketEventsByCategory(entries), [entries])

  /** Debounce timer ref for brush changes. */
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Clean up debounce timer on unmount.
  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [])

  /** Converts brush indices to ISO timestamps and calls onRangeChange (debounced). */
  const handleBrushChange = useCallback(
    (brushState: { startIndex?: number; endIndex?: number }) => {
      if (debounceRef.current) clearTimeout(debounceRef.current)

      debounceRef.current = setTimeout(() => {
        const { startIndex, endIndex } = brushState
        if (startIndex == null || endIndex == null || buckets.length === 0) return

        const isFullRange = startIndex === 0 && endIndex === buckets.length - 1
        if (isFullRange) {
          onRangeChange(undefined, undefined)
          return
        }

        const sinceTs = buckets[startIndex].timestamp
        const untilTs = new Date(buckets[endIndex].time + bucketWidthMs).toISOString()
        onRangeChange(sinceTs, untilTs)
      }, 300)
    },
    [buckets, bucketWidthMs, onRangeChange],
  )

  /** Computes controlled brush indices from since/until props.
   *  Always returns indices (defaults to full range) so the brush stays controlled. */
  const brushIndices = useMemo(() => {
    if (buckets.length === 0) return { startIndex: 0, endIndex: 0 }

    let startIndex = 0
    let endIndex = buckets.length - 1

    if (since) {
      const sinceMs = new Date(since).getTime()
      const idx = buckets.findIndex((b: ActivityBucket) => b.time >= sinceMs)
      if (idx >= 0) startIndex = idx
    }

    if (until) {
      const untilMs = new Date(until).getTime()
      for (let i = buckets.length - 1; i >= 0; i--) {
        if (buckets[i].time < untilMs) {
          endIndex = i
          break
        }
      }
    }

    return { startIndex, endIndex }
  }, [since, until, buckets])

  const hasSelection = since != null || until != null

  return (
    <div className="relative flex flex-col">
      {hasSelection && (
        <Button
          variant="ghost"
          size="sm"
          icon={X}
          className="text-muted-foreground absolute top-0 right-0 z-10 h-5 px-1.5 text-xs"
          onClick={() => onRangeChange(undefined, undefined)}
        >
          Clear selection
        </Button>
      )}

      {buckets.length === 0 ? (
        <div className="text-muted-foreground flex h-26 items-center justify-center text-xs">
          No activity for the selected filters
        </div>
      ) : (
        <ChartContainer config={chartConfig} className="activity-timeline aspect-auto h-25 w-full">
          <BarChart
            data={buckets}
            margin={{ top: 0, right: 0, bottom: 0, left: 0 }}
            barCategoryGap={1}
          >
            <CartesianGrid vertical={false} strokeDasharray="3 3" />
            <XAxis
              dataKey="time"
              type="number"
              domain={['dataMin', 'dataMax']}
              scale="time"
              tickFormatter={(v: number) => formatDateTick(v, bucketWidthMs)}
              tick={{ fontSize: 10 }}
              tickLine={false}
              axisLine={false}
            />
            <YAxis hide />
            <ChartTooltip
              position={{ y: -120 }}
              allowEscapeViewBox={{ x: true, y: true }}
              content={
                <ChartTooltipContent
                  className="bg-content-2"
                  labelFormatter={(_, payload) => {
                    const bucket = payload[0]?.payload as ActivityBucket | undefined
                    return bucket ? formatTooltipLabel(bucket.time, bucketWidthMs) : ''
                  }}
                />
              }
            />
            {STACK_ORDER.map((category) => (
              <Bar
                key={category}
                dataKey={category}
                stackId="category"
                fill={`var(--color-${category})`}
                radius={category === 'session' ? [1, 1, 0, 0] : undefined}
                isAnimationActive={false}
              />
            ))}
            <Brush
              dataKey="time"
              height={20}
              stroke="var(--border)"
              fill="var(--background)"
              tickFormatter={(v: number) => formatBrushTick(v, bucketWidthMs)}
              onChange={handleBrushChange}
              startIndex={brushIndices.startIndex}
              endIndex={brushIndices.endIndex}
            />
          </BarChart>
        </ChartContainer>
      )}
    </div>
  )
}

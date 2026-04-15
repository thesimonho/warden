import { Activity, DollarSign, Layers } from 'lucide-react'
import { useMemo } from 'react'

import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { formatCost } from '@/lib/cost'
import type { Project } from '@/lib/types'

/** Props for the CostDashboard component. */
interface CostDashboardProps {
  projects: readonly Project[]
}

/**
 * Displays aggregate worktree and cost statistics across all projects.
 *
 * Shows running project count, active worktrees, and aggregate cost.
 * Always visible so the dashboard layout is consistent.
 *
 * @param props.projects - All projects to aggregate stats from.
 */
export default function CostDashboard({ projects }: CostDashboardProps) {
  const stats = useMemo(() => {
    const runningProjects = projects.filter((p) => p.state === 'running')
    const totalActiveWorktrees = runningProjects.reduce((sum, p) => sum + p.activeWorktreeCount, 0)
    const totalCost = projects.reduce((sum, p) => sum + p.totalCost, 0)
    return {
      runningCount: runningProjects.length,
      totalActiveWorktrees,
      totalCost,
    }
  }, [projects])

  return (
    <div className="grid grid-cols-3 gap-4">
      <StatCard
        icon={<Activity className="text-muted-foreground h-4 w-4" />}
        label="Running Projects"
        value={String(stats.runningCount)}
      />
      <StatCard
        icon={<Layers className="text-muted-foreground h-4 w-4" />}
        label="Active Worktrees"
        value={String(stats.totalActiveWorktrees)}
      />
      <Tooltip>
        <TooltipTrigger asChild>
          <div>
            <StatCard
              icon={<DollarSign className="text-muted-foreground h-4 w-4" />}
              label="Total Cost"
              value={formatCost(stats.totalCost)}
              isMono
            />
          </div>
        </TooltipTrigger>
        <TooltipContent>Estimated API cost across all running projects</TooltipContent>
      </Tooltip>
    </div>
  )
}

/** Props for StatCard. */
interface StatCardProps {
  icon: React.ReactNode
  label: string
  value: string
  isMono?: boolean
}

/** A single stat display card. */
function StatCard({ icon, label, value, isMono = false }: StatCardProps) {
  return (
    <div className="bg-card rounded border p-4">
      <div className="text-muted-foreground flex items-center gap-2">
        {icon}
        {label}
      </div>
      <p className={`mt-1 text-2xl font-semibold ${isMono ? 'font-mono' : ''}`}>{value}</p>
    </div>
  )
}

import { useMemo } from 'react'
import { DollarSign, Layers, Activity } from 'lucide-react'
import type { Project } from '@/lib/types'
import { formatCost } from '@/lib/cost'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

/** Props for the CostDashboard component. */
interface CostDashboardProps {
  projects: readonly Project[]
}

/**
 * Displays aggregate worktree and cost statistics across all projects.
 *
 * Only renders when there are running projects with active worktrees or costs.
 * Shows total active worktrees, running projects count, and aggregate cost.
 *
 * @param props.projects - All projects to aggregate stats from.
 */
export default function CostDashboard({ projects }: CostDashboardProps) {
  const stats = useMemo(() => {
    const runningProjects = projects.filter((p) => p.state === 'running')
    const totalActiveWorktrees = runningProjects.reduce((sum, p) => sum + p.activeWorktreeCount, 0)
    const totalCost = runningProjects.reduce((sum, p) => sum + p.totalCost, 0)
    return {
      runningCount: runningProjects.length,
      totalActiveWorktrees,
      totalCost,
    }
  }, [projects])

  const hasContent =
    stats.runningCount > 0 && (stats.totalActiveWorktrees > 0 || stats.totalCost > 0)

  if (!hasContent) return null

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

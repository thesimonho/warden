import { useMemo, useState } from 'react'
import { LayoutGrid, ChevronRight } from 'lucide-react'
import type { Project } from '@/lib/types'
import type { RecentWorkspace } from '@/hooks/use-recent-workspaces'
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from '@/components/ui/collapsible'

/** Props for the RecentWorkspaces component. */
interface RecentWorkspacesProps {
  recentWorkspaces: RecentWorkspace[]
  projects: Project[]
  onOpen: (ids: string[]) => void
}

/**
 * Displays a collapsible list of recently opened workspaces (hidden by default).
 *
 * Each entry shows the names of the projects in that workspace. IDs that no
 * longer exist in the current project list are shown as their raw ID so the
 * entry remains identifiable.
 *
 * @param props.recentWorkspaces - The saved workspace entries to display.
 * @param props.projects - The current project list used to resolve names from IDs.
 * @param props.onOpen - Callback when a workspace entry is opened.
 */
export default function RecentWorkspaces({
  recentWorkspaces,
  projects,
  onOpen,
}: RecentWorkspacesProps) {
  const [isOpen, setIsOpen] = useState(false)

  const projectNameById = useMemo(
    () => new Map(projects.map((p) => [p.id, p.name || p.id])),
    [projects],
  )

  if (recentWorkspaces.length === 0) return null

  const resolveName = (id: string) => projectNameById.get(id) ?? id

  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen}>
      <CollapsibleTrigger className="text-muted-foreground hover:text-foreground flex cursor-pointer items-center gap-1.5">
        <ChevronRight
          className="h-3.5 w-3.5 transition-transform duration-150"
          style={{ transform: isOpen ? 'rotate(90deg)' : 'rotate(0deg)' }}
        />
        Recent Workspaces
      </CollapsibleTrigger>
      <CollapsibleContent className="mt-2 flex flex-wrap gap-2">
        {recentWorkspaces.map((workspace) => {
          const label = workspace.ids.map(resolveName).join(', ')
          return (
            <button
              key={workspace.savedAt}
              onClick={() => onOpen(workspace.ids)}
              className="bg-card hover:bg-accent flex cursor-pointer items-center gap-2 rounded border px-3 py-2 transition-colors"
            >
              <LayoutGrid className="text-muted-foreground h-3.5 w-3.5 shrink-0" />
              <span className="max-w-[320px] truncate">{label}</span>
            </button>
          )
        })}
      </CollapsibleContent>
    </Collapsible>
  )
}

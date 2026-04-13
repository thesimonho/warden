import { Monitor, Moon, Sun } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import type { ThemePreference } from '@/hooks/use-theme'
import { cn } from '@/lib/utils'

/** Props for the ThemeToggle component. */
interface ThemeToggleProps {
  preference: ThemePreference
  onChange: (preference: ThemePreference) => void
}

const SEGMENTS: { value: ThemePreference; icon: typeof Sun; label: string }[] = [
  { value: 'light', icon: Sun, label: 'Light' },
  { value: 'system', icon: Monitor, label: 'System' },
  { value: 'dark', icon: Moon, label: 'Dark' },
]

/**
 * Three-state segmented toggle for theme selection.
 *
 * Shows all three options (light, system, dark) with a sliding
 * background indicator on the active segment.
 *
 * @param props.preference - The current theme preference.
 * @param props.onChange - Callback when a segment is clicked.
 */
export default function ThemeToggle({ preference, onChange }: ThemeToggleProps) {
  const activeIndex = SEGMENTS.findIndex((s) => s.value === preference)

  return (
    <div className="bg-muted relative flex rounded p-0.5" role="radiogroup" aria-label="Theme">
      {/* Sliding indicator — same size as a button, translated by index */}
      <div
        className="bg-background absolute inset-y-0.5 w-7 rounded shadow-sm transition-transform duration-200 ease-out"
        style={{ left: '2px', transform: `translateX(${activeIndex * 100}%)` }}
      />

      {SEGMENTS.map(({ value, icon: Icon, label }) => (
        <Tooltip key={value}>
          <TooltipTrigger asChild>
            <button
              type="button"
              role="radio"
              aria-checked={preference === value}
              aria-label={label}
              onClick={() => onChange(value)}
              className={cn(
                'relative z-10 flex h-6 w-7 cursor-pointer items-center justify-center rounded transition-colors',
                preference === value
                  ? 'text-foreground'
                  : 'text-muted-foreground hover:text-foreground/70',
              )}
            >
              <Icon
                className={cn(
                  'size-3.5',
                  value === 'light' && 'text-warning',
                  value === 'dark' && 'text-info',
                )}
              />
            </button>
          </TooltipTrigger>
          <TooltipContent>{label}</TooltipContent>
        </Tooltip>
      ))}
    </div>
  )
}

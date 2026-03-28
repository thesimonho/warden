import { useCallback, useEffect, useState } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Info, Loader2, Settings as SettingsIcon } from 'lucide-react'
import { Separator } from '@/components/ui/separator'
import { type Settings } from '@/lib/settings'
import { fetchRuntimes, fetchSettings, updateSettings } from '@/lib/api'
import type { AuditLogMode, RuntimeInfo } from '@/lib/types'

/** Props for the SettingsDialog component. */
interface SettingsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  settings: Settings
  onSettingsChange: (settings: Settings) => void
  auditLogMode: AuditLogMode
  onAuditLogModeChange: (mode: AuditLogMode) => void
}

/**
 * Modal dialog for configuring dashboard preferences.
 *
 * Changes are applied immediately and persisted to localStorage via
 * the onSettingsChange callback. Server-side settings (like runtime)
 * are fetched and updated via the API.
 *
 * @param props.settings - Current settings values.
 * @param props.onSettingsChange - Called with updated settings on any change.
 */
export default function SettingsDialog({
  open,
  onOpenChange,
  settings,
  onSettingsChange,
  auditLogMode,
  onAuditLogModeChange,
}: SettingsDialogProps) {
  const [runtimes, setRuntimes] = useState<RuntimeInfo[]>([])
  const [currentRuntime, setCurrentRuntime] = useState<string>('')
  const [restartNeeded, setRestartNeeded] = useState(false)
  const [isAuditLogModePending, setIsAuditLogModePending] = useState(false)
  const [defaultBudget, setDefaultBudget] = useState('')
  const [budgetActions, setBudgetActions] = useState({
    budgetActionWarn: true,
    budgetActionStopWorktrees: false,
    budgetActionStopContainer: false,
    budgetActionPreventStart: false,
  })

  /** Loads runtime availability and current setting when dialog opens. */
  const loadServerData = useCallback(async () => {
    try {
      const [runtimeList, serverSettings] = await Promise.all([fetchRuntimes(), fetchSettings()])
      setRuntimes(runtimeList)
      setDefaultBudget(
        serverSettings.defaultProjectBudget > 0 ? String(serverSettings.defaultProjectBudget) : '',
      )
      setBudgetActions({
        budgetActionWarn: serverSettings.budgetActionWarn,
        budgetActionStopWorktrees: serverSettings.budgetActionStopWorktrees,
        budgetActionStopContainer: serverSettings.budgetActionStopContainer,
        budgetActionPreventStart: serverSettings.budgetActionPreventStart,
      })

      // If the saved runtime is unavailable, fall back to the first available one.
      const savedRuntime = serverSettings.runtime
      const isSavedAvailable = runtimeList.some((rt) => rt.name === savedRuntime && rt.available)
      if (isSavedAvailable) {
        setCurrentRuntime(savedRuntime)
      } else {
        const firstAvailable = runtimeList.find((rt) => rt.available)
        if (firstAvailable) {
          setCurrentRuntime(firstAvailable.name)
          await updateSettings({ runtime: firstAvailable.name })
        }
      }
      setRestartNeeded(false)
    } catch {
      // Silently ignore — runtimes section won't render
    }
  }, [])

  useEffect(() => {
    if (open) {
      loadServerData()
    }
  }, [open, loadServerData])

  /** Toggles notifications, requesting browser permission if enabling. */
  const handleNotificationsToggle = async () => {
    const enabling = !settings.notificationsEnabled
    if (enabling && 'Notification' in window && Notification.permission === 'default') {
      const permission = await Notification.requestPermission()
      if (permission !== 'granted') return
    }
    onSettingsChange({ ...settings, notificationsEnabled: enabling })
  }

  /** Updates the container runtime setting on the server. */
  const handleRuntimeChange = async (runtime: RuntimeInfo['name']) => {
    try {
      const result = await updateSettings({ runtime })
      setCurrentRuntime(runtime)
      if (result.restartRequired) {
        setRestartNeeded(true)
      }
    } catch {
      // Ignore — user can retry
    }
  }

  /** Cycles audit log mode: off → standard → detailed → off. */
  const handleAuditLogModeChange = async (mode: AuditLogMode) => {
    setIsAuditLogModePending(true)
    try {
      await updateSettings({ auditLogMode: mode })
      onAuditLogModeChange(mode)
    } catch {
      // Ignore — user can retry
    } finally {
      setIsAuditLogModePending(false)
    }
  }

  /** Returns whether a runtime option should be disabled. */
  const isRuntimeDisabled = (rt: RuntimeInfo): boolean => !rt.available

  /** Returns whether a runtime was detected via binary but has no active socket. */
  const isSocketInactive = (rt: RuntimeInfo): boolean => rt.available && !rt.socketPath

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl" aria-describedby={undefined}>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <SettingsIcon className="text-muted-foreground h-5 w-5" />
            Settings
          </DialogTitle>
        </DialogHeader>

        <div className="max-h-[70vh] space-y-6 overflow-y-auto py-2">
          {runtimes.length > 0 && (
            <div className="space-y-2">
              <p className="font-medium">Container runtime</p>
              <p className="text-muted-foreground text-sm">
                Select which container engine to use. Requires a restart to take effect.
              </p>
              <RadioGroup
                value={currentRuntime}
                onValueChange={(v) => handleRuntimeChange(v as RuntimeInfo['name'])}
              >
                {runtimes.map((rt) => (
                  <label
                    key={rt.name}
                    className={`flex items-center gap-2 ${
                      isRuntimeDisabled(rt) ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'
                    }`}
                  >
                    <RadioGroupItem value={rt.name} disabled={isRuntimeDisabled(rt)} />
                    <span className="capitalize">{rt.name}</span>
                    {isRuntimeDisabled(rt) && (
                      <span className="text-muted-foreground text-sm">(not detected)</span>
                    )}
                    {rt.available && rt.version && (
                      <span className="text-muted-foreground text-sm">v{rt.version}</span>
                    )}
                  </label>
                ))}
              </RadioGroup>
              {runtimes.some((rt) => rt.name === currentRuntime && isSocketInactive(rt)) && (
                <p className="text-warning text-sm">
                  No active socket found. Start the service first, e.g.{' '}
                  <code className="bg-muted text-foreground rounded px-1 py-0.5 font-mono text-sm">
                    systemctl --user enable --now {currentRuntime}.socket
                  </code>
                </p>
              )}
              {restartNeeded && (
                <p className="text-warning text-sm">
                  Restart the dashboard to apply the runtime change.
                </p>
              )}
            </div>
          )}

          {runtimes.length > 0 && <Separator />}

          <div className="space-y-2">
            <p className="font-medium">Browser notifications</p>
            <p className="text-muted-foreground text-sm">Get notified when agents need input.</p>
            <label className="flex cursor-pointer items-center gap-2">
              <Checkbox
                checked={settings.notificationsEnabled}
                onCheckedChange={handleNotificationsToggle}
              />
              <span>Enable notifications</span>
            </label>
          </div>

          <Separator />

          <div className="space-y-2">
            <p className="font-medium">Default budget</p>
            <p className="text-muted-foreground text-sm">
              Per-project cost limit (can be overridden per project).
            </p>
            <div className="flex items-center gap-2">
              <span className="text-muted-foreground text-sm">$</span>
              <Input
                type="number"
                min={0}
                step={0.01}
                placeholder="Unlimited"
                value={defaultBudget}
                onChange={(e) => setDefaultBudget(e.target.value)}
                onBlur={async () => {
                  const value = parseFloat(defaultBudget) || 0
                  try {
                    await updateSettings({ defaultProjectBudget: value })
                  } catch {
                    // Ignore — user can retry
                  }
                }}
                className="w-32"
              />
            </div>
          </div>

          <div className="space-y-2">
            <p className="font-small">When budget is exceeded...</p>
            {(
              [
                { key: 'budgetActionWarn', label: 'Show a warning' },
                { key: 'budgetActionStopWorktrees', label: 'Stop worktrees' },
                { key: 'budgetActionStopContainer', label: 'Stop container' },
                { key: 'budgetActionPreventStart', label: 'Prevent restart' },
              ] as const
            ).map(({ key, label }) => (
              <label key={key} className="flex cursor-pointer items-center gap-2">
                <Checkbox
                  checked={budgetActions[key]}
                  onCheckedChange={async (checked) => {
                    const value = checked === true
                    setBudgetActions((prev) => ({ ...prev, [key]: value }))
                    try {
                      await updateSettings({ [key]: value })
                    } catch {
                      setBudgetActions((prev) => ({ ...prev, [key]: !value }))
                    }
                  }}
                />
                <span>{label}</span>
              </label>
            ))}
          </div>

          <Separator />

          <div className="space-y-3">
            <p className="font-medium">Audit logging</p>
            <RadioGroup
              value={auditLogMode}
              onValueChange={(value) => handleAuditLogModeChange(value as AuditLogMode)}
              disabled={isAuditLogModePending}
              className="space-y-1"
            >
              <label className="flex cursor-pointer items-center gap-2">
                <RadioGroupItem value="off" />
                <span>Off</span>
              </label>
              <label className="flex cursor-pointer items-center gap-2">
                <RadioGroupItem value="standard" />
                <span>Standard</span>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Info className="text-muted-foreground h-3.5 w-3.5" />
                  </TooltipTrigger>
                  <TooltipContent>Sessions, worktree lifecycle, system events</TooltipContent>
                </Tooltip>
              </label>
              <label className="flex cursor-pointer items-center gap-2">
                <RadioGroupItem value="detailed" />
                <span>Detailed</span>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Info className="text-muted-foreground h-3.5 w-3.5" />
                  </TooltipTrigger>
                  <TooltipContent>
                    Standard plus tool use, user prompts, permissions, config changes, and debug
                  </TooltipContent>
                </Tooltip>
              </label>
            </RadioGroup>
            {isAuditLogModePending && (
              <Loader2 className="text-muted-foreground h-4 w-4 animate-spin" />
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}

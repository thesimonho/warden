import { useCallback, useEffect, useState } from 'react'
import { Link, Outlet } from 'react-router-dom'
import { toast, Toaster } from 'sonner'
import { Box, KeyRound, Settings, ShieldCheck } from 'lucide-react'
import { useTheme } from '@/hooks/use-theme'
import { loadSettings, saveSettings, type Settings as DashboardSettings } from '@/lib/settings'
import { fetchSettings } from '@/lib/api'
import ThemeToggle from '@/components/theme-toggle'
import SettingsDialog from '@/components/settings-dialog'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import type { AuditLogMode } from '@/lib/types'

/** Context passed to child routes via useOutletContext. */
export interface LayoutContext {
  settings: DashboardSettings
  /** Whether the "prevent restart" budget enforcement action is enabled. */
  budgetActionPreventStart: boolean
}

/**
 * App shell layout with a branded sticky header and routed content area.
 *
 * Owns theme and settings state so the header controls stay in sync
 * without a separate context provider.
 */
export default function Layout() {
  const { preference, setPreference, resolvedTheme } = useTheme()
  const [settings, setSettings] = useState<DashboardSettings>(loadSettings)
  const [isSettingsOpen, setIsSettingsOpen] = useState(false)
  const [auditLogMode, setAuditLogMode] = useState<AuditLogMode>('off')
  const [budgetActionPreventStart, setBudgetActionPreventStart] = useState(false)
  const [serverVersion, setServerVersion] = useState('')

  /** Seed server-derived state without requiring settings dialog to be opened. */
  useEffect(() => {
    fetchSettings()
      .then((serverSettings) => {
        setAuditLogMode(serverSettings.auditLogMode)
        setBudgetActionPreventStart(serverSettings.budgetActionPreventStart)
        setServerVersion(serverSettings.version)

        // Notify once per CLI version change so the user knows containers
        // will get a new CLI on next recreate.
        const versions = [
          { key: 'claude-code', label: 'Claude Code', version: serverSettings.claudeCodeVersion },
          { key: 'codex', label: 'Codex', version: serverSettings.codexVersion },
        ]
        for (const { key, label, version } of versions) {
          if (!version) continue
          const storageKey = `cli-version:${key}`
          const prev = localStorage.getItem(storageKey)
          localStorage.setItem(storageKey, version)
          if (prev && prev !== version) {
            toast.info(`${label} updated to ${version}`, {
              description: `Previously ${prev}. New containers will use this version.`,
              duration: Infinity,
              dismissible: true,
            })
          }
        }
      })
      .catch(() => {})
  }, [])

  const handleSettingsChange = useCallback((updated: DashboardSettings) => {
    setSettings(updated)
    saveSettings(updated)
  }, [])

  return (
    <div className="bg-background text-foreground min-h-screen">
      <header className="bg-background/80 sticky top-0 z-10 border-b backdrop-blur-sm">
        <div className="flex items-center justify-between px-6 py-3">
          <div className="flex items-center gap-2">
            <Link to="/" className="flex items-center gap-2.5 transition-opacity hover:opacity-75">
              <img src="/logo.svg" alt="Warden" className="h-5 dark:invert" />
            </Link>
            <Button size="sm" variant="ghost" className="dark:brightness-60" asChild>
              <Link to="https://github.com/thesimonho/warden" target="_blank">
                <img src="/github-icon.webp" alt="GitHub" className="h-5 dark:invert" />
              </Link>
            </Button>
            {serverVersion && (
              <span className="text-muted-foreground text-xs">{serverVersion}</span>
            )}
          </div>

          <div className="flex items-center gap-1">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button data-testid="projects-button" size="sm" variant="ghost" asChild>
                  <Link to="/">
                    <Box />
                  </Link>
                </Button>
              </TooltipTrigger>
              <TooltipContent>Projects</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button data-testid="access-button" size="sm" variant="ghost" asChild>
                  <Link to="/access">
                    <KeyRound />
                  </Link>
                </Button>
              </TooltipTrigger>
              <TooltipContent>Access</TooltipContent>
            </Tooltip>
            {auditLogMode !== 'off' && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button data-testid="audit-button" size="sm" variant="ghost" asChild>
                    <Link to="/audit">
                      <ShieldCheck />
                    </Link>
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Audit Log</TooltipContent>
              </Tooltip>
            )}
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  data-testid="settings-button"
                  size="sm"
                  variant="ghost"
                  onClick={() => setIsSettingsOpen(true)}
                  icon={Settings}
                />
              </TooltipTrigger>
              <TooltipContent>Settings</TooltipContent>
            </Tooltip>
            <ThemeToggle preference={preference} onChange={setPreference} />
          </div>
        </div>
      </header>
      <main className="p-6">
        <Outlet context={{ settings, budgetActionPreventStart } satisfies LayoutContext} />
      </main>

      <SettingsDialog
        open={isSettingsOpen}
        onOpenChange={(open) => {
          setIsSettingsOpen(open)
          if (!open) {
            // Refresh server settings that affect other pages.
            fetchSettings()
              .then((s) => {
                setBudgetActionPreventStart(s.budgetActionPreventStart)
                setAuditLogMode(s.auditLogMode)
              })
              .catch(() => {})
          }
        }}
        settings={settings}
        onSettingsChange={handleSettingsChange}
        auditLogMode={auditLogMode}
        onAuditLogModeChange={setAuditLogMode}
      />
      <Toaster theme={resolvedTheme} position="bottom-right" visibleToasts={3} expand richColors />
    </div>
  )
}

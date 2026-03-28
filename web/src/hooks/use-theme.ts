import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from 'react'

/** The user's stored preference — includes 'system' for OS-following mode. */
export type ThemePreference = 'light' | 'system' | 'dark'

/** The resolved appearance applied to the page. */
export type ResolvedTheme = 'light' | 'dark'

const STORAGE_KEY = 'theme'
const PREFERENCE_CYCLE: ThemePreference[] = ['light', 'system', 'dark']
const SYSTEM_DARK_QUERY = '(prefers-color-scheme: dark)'

/** Reads the persisted theme preference, defaulting to system. */
function resolveInitialPreference(): ThemePreference {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored === 'light' || stored === 'system' || stored === 'dark') return stored
  } catch {
    // localStorage unavailable (e.g. SSR or restricted context)
  }
  return 'system'
}

/** Subscribes to OS color-scheme changes via matchMedia. */
function subscribeToSystemTheme(callback: () => void) {
  const mql = window.matchMedia(SYSTEM_DARK_QUERY)
  mql.addEventListener('change', callback)
  return () => mql.removeEventListener('change', callback)
}

/** Returns the current OS dark-mode preference. */
function getSystemSnapshot(): boolean {
  return window.matchMedia(SYSTEM_DARK_QUERY).matches
}

/** SSR fallback — assume dark for developer-facing tool. */
function getServerSnapshot(): boolean {
  return true
}

/**
 * Manages light/system/dark theme with localStorage persistence.
 *
 * When set to 'system', follows the OS `prefers-color-scheme` setting and
 * reacts live to changes. Applies/removes the `dark` class on `<html>` and
 * persists the preference.
 *
 * @returns `preference` — the stored setting, `resolvedTheme` — the active
 * appearance, and `cycleTheme` — advances to the next mode.
 */
export function useTheme() {
  const [preference, setPreference] = useState<ThemePreference>(resolveInitialPreference)

  const isSystemDark = useSyncExternalStore(
    subscribeToSystemTheme,
    getSystemSnapshot,
    getServerSnapshot,
  )

  const resolvedTheme: ResolvedTheme = useMemo(() => {
    if (preference === 'system') return isSystemDark ? 'dark' : 'light'
    return preference
  }, [preference, isSystemDark])

  useEffect(() => {
    document.documentElement.classList.toggle('dark', resolvedTheme === 'dark')
    try {
      localStorage.setItem(STORAGE_KEY, preference)
    } catch {
      // localStorage unavailable
    }
  }, [preference, resolvedTheme])

  const cycleTheme = useCallback(() => {
    setPreference((current) => {
      const currentIndex = PREFERENCE_CYCLE.indexOf(current)
      const nextIndex = (currentIndex + 1) % PREFERENCE_CYCLE.length
      return PREFERENCE_CYCLE[nextIndex]
    })
  }, [])

  return { preference, setPreference, resolvedTheme, cycleTheme }
}

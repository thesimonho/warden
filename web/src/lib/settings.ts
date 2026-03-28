/** Dashboard user preferences persisted to localStorage. */
export interface Settings {
  /** Whether browser notifications are enabled for worktree events. */
  notificationsEnabled: boolean
}

export const SETTINGS_KEY = 'settings'

export const DEFAULT_SETTINGS: Settings = {
  notificationsEnabled: false,
}

/**
 * Loads settings from localStorage, merging with defaults so new keys
 * are always present even if absent from stored data.
 */
export function loadSettings(): Settings {
  try {
    const raw = localStorage.getItem(SETTINGS_KEY)
    if (!raw) return { ...DEFAULT_SETTINGS }
    return { ...DEFAULT_SETTINGS, ...(JSON.parse(raw) as Partial<Settings>) }
  } catch {
    return { ...DEFAULT_SETTINGS }
  }
}

/** Persists settings to localStorage. */
export function saveSettings(settings: Settings): void {
  localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings))
}

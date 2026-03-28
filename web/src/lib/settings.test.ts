import { describe, it, expect, beforeEach } from 'vitest'
import { loadSettings, saveSettings, DEFAULT_SETTINGS, SETTINGS_KEY } from '@/lib/settings'

describe('loadSettings', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('returns defaults when nothing is stored', () => {
    expect(loadSettings()).toEqual(DEFAULT_SETTINGS)
  })

  it('returns stored value merged with defaults', () => {
    localStorage.setItem(SETTINGS_KEY, JSON.stringify({ notificationsEnabled: true }))
    expect(loadSettings()).toEqual({
      notificationsEnabled: true,
    })
  })

  it('fills in missing keys with defaults', () => {
    localStorage.setItem(SETTINGS_KEY, JSON.stringify({}))
    expect(loadSettings()).toEqual(DEFAULT_SETTINGS)
  })

  it('returns defaults when stored JSON is malformed', () => {
    localStorage.setItem(SETTINGS_KEY, 'not-json{{{')
    expect(loadSettings()).toEqual(DEFAULT_SETTINGS)
  })
})

describe('saveSettings', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('persists settings to localStorage', () => {
    const settings = { ...DEFAULT_SETTINGS, notificationsEnabled: true }
    saveSettings(settings)
    expect(JSON.parse(localStorage.getItem(SETTINGS_KEY)!)).toEqual(settings)
  })

  it('round-trips through loadSettings', () => {
    const settings = { ...DEFAULT_SETTINGS, notificationsEnabled: true }
    saveSettings(settings)
    expect(loadSettings()).toEqual(settings)
  })

  it('overwrites previously stored settings', () => {
    saveSettings({ ...DEFAULT_SETTINGS, notificationsEnabled: false })
    saveSettings({ ...DEFAULT_SETTINGS, notificationsEnabled: true })
    expect(loadSettings()).toEqual({
      ...DEFAULT_SETTINGS,
      notificationsEnabled: true,
    })
  })
})
